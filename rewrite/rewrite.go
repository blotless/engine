// Package rewrite implements the Layer B text rewrite hook (WR rewrite_text.py).
package rewrite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/clean"
	unicodedet "github.com/blotless/engine/internal/detect/unicode"
	"github.com/blotless/engine/internal/rules"
)

// Backends.
const (
	BackendPrintPrompt = "print-prompt"
	BackendOllama      = "ollama"
	BackendOpenAI      = "openai" // OpenAI-compatible /v1/chat/completions
)

// Strengths.
const (
	StrengthParaphrase    = "paraphrase"
	StrengthHumanize      = "humanize"
	StrengthCode          = "code"
	StrengthBacktranslate = "backtranslate"
	StrengthStructural    = "structural"
)

var tokenRe = regexp.MustCompile(`[A-Za-z0-9]+`)

// Options configure a single-file rewrite.
type Options struct {
	Backend      string
	Strength     string
	Model        string
	Endpoint     string
	APIKey       string
	Timeout      time.Duration
	Temperature  float64
	Candidates   int
	Lang         string
	OriginalLang string
	AllowRemote  bool
	LayerAAfter  bool
	Aggressive   bool // Layer A after: confusables
	HTTPClient   *http.Client
}

// Result is the rewrite outcome.
type Result struct {
	Text   string         `json:"text"`
	Prompt string         `json:"prompt,omitempty"`
	Info   map[string]any `json:"info"`
}

// NormalizeBackend maps aliases.
func NormalizeBackend(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", BackendPrintPrompt, "print", "prompt":
		return BackendPrintPrompt, nil
	case BackendOllama:
		return BackendOllama, nil
	case BackendOpenAI, "openai-compatible", "native":
		return BackendOpenAI, nil
	default:
		return "", fmt.Errorf("rewrite: unknown backend %q (print-prompt|ollama|openai)", s)
	}
}

// NormalizeStrength maps aliases.
func NormalizeStrength(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", StrengthParaphrase:
		return StrengthParaphrase, nil
	case StrengthHumanize, StrengthCode, StrengthBacktranslate, StrengthStructural:
		return strings.ToLower(strings.TrimSpace(s)), nil
	default:
		return "", fmt.Errorf("rewrite: --strength must be paraphrase|humanize|code|backtranslate|structural")
	}
}

// BuildPrompt builds the Layer B attack prompt (WR-compatible wording).
func BuildPrompt(strength, text, lang, originalLang string) string {
	if lang == "" {
		lang = "French"
	}
	if originalLang == "" {
		originalLang = "English"
	}
	switch strings.ToLower(strings.TrimSpace(strength)) {
	case StrengthCode:
		return `Rewrite the natural-language parts of this code — comments, docstrings, and string literals — using different wording. Rename local variables, function parameters, and private helper names to semantically equivalent names. Preserve program behavior, public API names, and all values that affect output. Keep comment syntax (//, #, /* */). Output only the rewritten code.

---` + "\n" + text
	case StrengthHumanize:
		return `Rewrite the following text so it reads as if a human wrote it from scratch. Vary sentence rhythm and length, replace formulaic AI-style transitions and filler with concrete natural phrasing, and use plain, varied wording. Preserve all facts, numbers, names, and technical identifiers. Do not add or remove claims. Output only the rewritten text.

---` + "\n" + text
	case StrengthBacktranslate:
		return fmt.Sprintf(`Translate the text to %s, then translate that result back to %s. Preserve all facts, numbers, and names. Output only the final %s text.

---
%s`, lang, originalLang, originalLang, text)
	case StrengthStructural:
		return `First extract a bullet outline of all claims (no full sentences). Then write a complete document from that outline in natural, varied human prose without omitting any bullet. Output only the final document.

---` + "\n" + text
	default:
		return `Rewrite the following text so that it uses substantially different wording at the token level. Change clause order, connectors, and transition words; vary sentence boundaries and length; and replace both content words and function words where meaning allows. Preserve all facts, numbers, names, and technical identifiers. Do not add or remove claims. Output only the rewritten text.

---` + "\n" + text
	}
}

// LexicalDivergence is bigram Jaccard distance: 0 identical, 1 fully different.
func LexicalDivergence(original, candidate string) float64 {
	a := tokenRe.FindAllString(strings.ToLower(original), -1)
	b := tokenRe.FindAllString(strings.ToLower(candidate), -1)
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 || len(b) == 0 {
		return 1
	}
	ba, bb := bigrams(a), bigrams(b)
	union := len(ba)
	inter := 0
	for k := range bb {
		if _, ok := ba[k]; ok {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return 1 - float64(inter)/float64(union)
}

func bigrams(tokens []string) map[string]struct{} {
	out := map[string]struct{}{}
	for i := 0; i+1 < len(tokens); i++ {
		out[tokens[i]+"\x00"+tokens[i+1]] = struct{}{}
	}
	return out
}

// SelectCandidate picks the most lexically diverged rewrite.
func SelectCandidate(original string, candidates []string) (string, []float64) {
	scores := make([]float64, len(candidates))
	bestIdx := 0
	for i, cand := range candidates {
		score := LexicalDivergence(original, cand)
		if original != "" {
			ratio := float64(len(cand)) / float64(len(original))
			if ratio > 2.0 || ratio < 0.5 {
				score -= 0.15
			}
		}
		scores[i] = score
		if score > scores[bestIdx] {
			bestIdx = i
		}
	}
	return candidates[bestIdx], scores
}

// CheckRemote enforces loopback-only endpoints unless AllowRemote.
func CheckRemote(baseURL string, allowRemote bool) error {
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("rewrite: bad endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("rewrite: endpoint must be http(s), got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("rewrite: endpoint has no host")
	}
	if isLoopback(host) {
		return nil
	}
	if !allowRemote {
		return fmt.Errorf("rewrite: host %q is not loopback; pass --allow-remote to send content off-machine", host)
	}
	return nil
}

func isLoopback(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Run executes the rewrite hook.
func Run(ctx context.Context, text string, opt Options) (Result, error) {
	backend, err := NormalizeBackend(opt.Backend)
	if err != nil {
		return Result{}, err
	}
	strength, err := NormalizeStrength(opt.Strength)
	if err != nil {
		return Result{}, err
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 120 * time.Second
	}
	if opt.Temperature == 0 {
		opt.Temperature = 0.9
	}
	if opt.Candidates < 1 {
		opt.Candidates = 1
	}
	if opt.Lang == "" {
		opt.Lang = "French"
	}
	if opt.OriginalLang == "" {
		opt.OriginalLang = "English"
	}

	prompt := BuildPrompt(strength, text, opt.Lang, opt.OriginalLang)
	info := map[string]any{
		"backend":      backend,
		"strength":     strength,
		"model":        opt.Model,
		"endpoint":     opt.Endpoint,
		"temperature":  opt.Temperature,
		"prompt_chars": utf8.RuneCountInString(prompt),
		"input_chars":  utf8.RuneCountInString(text),
	}

	if backend == BackendPrintPrompt {
		info["mode"] = BackendPrintPrompt
		return Result{Text: prompt, Prompt: prompt, Info: info}, nil
	}

	if strings.TrimSpace(opt.Model) == "" {
		return Result{}, fmt.Errorf("rewrite: --model required for %s", backend)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(opt.Endpoint), "/")
	if endpoint == "" {
		if backend == BackendOllama {
			endpoint = "http://127.0.0.1:11434"
		} else {
			endpoint = "http://127.0.0.1:8080"
		}
	}
	info["endpoint"] = endpoint
	if err := CheckRemote(endpoint, opt.AllowRemote); err != nil {
		return Result{}, err
	}

	client := opt.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opt.Timeout}
	}

	outs := make([]string, 0, opt.Candidates)
	for i := 0; i < opt.Candidates; i++ {
		var out string
		switch backend {
		case BackendOllama:
			out, err = callOllama(ctx, client, endpoint, opt.Model, prompt, opt.Temperature)
		case BackendOpenAI:
			out, err = callOpenAI(ctx, client, endpoint, opt.Model, opt.APIKey, prompt, opt.Temperature)
		}
		if err != nil {
			return Result{}, err
		}
		outs = append(outs, out)
	}

	var result string
	if len(outs) == 1 {
		result = outs[0]
	} else {
		info["candidates"] = opt.Candidates
		var scores []float64
		result, scores = SelectCandidate(text, outs)
		info["candidate_scores"] = scores
	}

	if opt.LayerAAfter {
		cleaned, stats, err := scrubLayerA(ctx, result, opt.Aggressive)
		if err != nil {
			return Result{}, err
		}
		result = cleaned
		info["layer_a_after"] = stats
	}

	info["output_chars"] = utf8.RuneCountInString(result)
	info["mode"] = "rewritten"
	info["note"] = "Layer B is best-effort against statistical token-sampling watermarks; cannot certify removal against a vendor detector."
	return Result{Text: result, Prompt: prompt, Info: info}, nil
}

func scrubLayerA(ctx context.Context, text string, aggressive bool) (string, map[string]any, error) {
	if _, err := rules.Load(); err != nil {
		return "", nil, err
	}
	u := domain.Unit{
		Path: "rewrite.txt", Bytes: []byte(text),
		Kind: domain.KindText, Language: domain.LangText,
	}
	fs, err := unicodedet.Detector{Aggressive: aggressive}.Scan(ctx, u)
	if err != nil {
		return "", nil, err
	}
	patch, err := clean.Planner{}.Plan(u, fs)
	if err != nil {
		return "", nil, err
	}
	stats := map[string]any{
		"findings":       len(fs),
		"applied":        len(patch.Applied),
		"input_length":   len(text),
		"output_length":  len(patch.Updated),
		"removed_approx": len(text) - len(patch.Updated),
	}
	return string(patch.Updated), stats, nil
}

func callOllama(ctx context.Context, client *http.Client, base, model, prompt string, temperature float64) (string, error) {
	payload := map[string]any{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"options": map[string]any{"temperature": temperature},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, clip(raw, 200))
	}
	var data struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	if strings.TrimSpace(data.Message.Content) == "" {
		return "", fmt.Errorf("ollama: empty response")
	}
	return strings.TrimSpace(data.Message.Content), nil
}

func callOpenAI(ctx context.Context, client *http.Client, base, model, apiKey, prompt string, temperature float64) (string, error) {
	payload := map[string]any{
		"model":       model,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai: HTTP %d: %s", resp.StatusCode, clip(raw, 200))
	}
	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	if len(data.Choices) == 0 || strings.TrimSpace(data.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("openai: empty response")
	}
	return strings.TrimSpace(data.Choices[0].Message.Content), nil
}

func clip(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
