//go:build llm

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/blotless/engine/ports"
)

// Native talks to a local llama.cpp OpenAI-compatible server.
// In-process GGUF load can be added behind the same port without changing callers.
type Native struct {
	cfg Config
}

func newNative(cfg Config) (ports.LLM, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://127.0.0.1:8080"
	}
	if cfg.Model == "" {
		cfg.Model = os.Getenv("BLOTLESS_LLM_MODEL")
	}
	return Native{cfg: cfg}, nil
}

func (n Native) Name() string { return "native" }

func (n Native) Classify(ctx context.Context, in ports.ClassifyIn) (ports.ClassifyOut, error) {
	raw, err := n.chat(ctx, `Classify if SOURCE CODE is AI-assistant generated. Ordinary human code => confidence none, rationale not_ai. Use the style fingerprint. Return JSON keys rule_suggestion, rationale, confidence (likely|heuristic|none), agent (cursor|claude|copilot|chatgpt|gemini|none). Name agent only from an explicit marker in the fragment; otherwise none.
File: `+in.File+`
Style fingerprint: `+in.Context+`
Fragment:
`+in.Fragment)
	if err != nil {
		return ports.ClassifyOut{}, err
	}
	var out ports.ClassifyOut
	if err := json.Unmarshal(raw, &out); err != nil {
		return ports.ClassifyOut{}, nil
	}
	return out, nil
}

func (n Native) Rewrite(ctx context.Context, in ports.RewriteIn) (ports.RewriteOut, error) {
	raw, err := n.chat(ctx, rewritePrompt(in))
	if err != nil {
		return ports.RewriteOut{}, err
	}
	return parseRewrite(raw), nil
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
}

func (n Native) chat(ctx context.Context, prompt string) ([]byte, error) {
	body, err := json.Marshal(chatReq{
		Model:    n.cfg.Model,
		Stream:   false,
		Messages: []chatMsg{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(n.cfg.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.APIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("native llm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("native llm: status %d: %s", resp.StatusCode, b)
	}
	var decoded chatResp
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("native llm: empty choices")
	}
	return []byte(strings.TrimSpace(decoded.Choices[0].Message.Content)), nil
}

func (Native) Close() error { return nil }

var _ ports.LLM = Native{}
