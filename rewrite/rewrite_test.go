package rewrite

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildPromptStrengths(t *testing.T) {
	text := "Hello world."
	cases := []struct {
		strength string
		want     string
	}{
		{StrengthParaphrase, "token level"},
		{StrengthHumanize, "human wrote"},
		{StrengthCode, "public API"},
		{StrengthBacktranslate, "Translate the text to French"},
		{StrengthStructural, "bullet outline"},
	}
	for _, tc := range cases {
		p := BuildPrompt(tc.strength, text, "", "")
		if !strings.Contains(p, tc.want) {
			t.Fatalf("%s: missing %q in %s", tc.strength, tc.want, p)
		}
		if !strings.Contains(p, text) {
			t.Fatalf("%s: missing text", tc.strength)
		}
	}
}

func TestLexicalDivergenceAndSelect(t *testing.T) {
	orig := "the quick brown fox jumps over the lazy dog"
	near := "the quick brown fox jumps over the lazy dog"
	far := "a speedy auburn canine leaps above a sleepy hound"
	if LexicalDivergence(orig, near) > 0.1 {
		t.Fatalf("near=%v", LexicalDivergence(orig, near))
	}
	if LexicalDivergence(orig, far) < 0.5 {
		t.Fatalf("far=%v", LexicalDivergence(orig, far))
	}
	best, scores := SelectCandidate(orig, []string{near, far})
	if best != far {
		t.Fatalf("best=%q scores=%v", best, scores)
	}
}

func TestCheckRemote(t *testing.T) {
	if err := CheckRemote("http://127.0.0.1:11434", false); err != nil {
		t.Fatal(err)
	}
	if err := CheckRemote("http://example.com:11434", false); err == nil {
		t.Fatal("expected deny")
	}
	if err := CheckRemote("http://example.com:11434", true); err != nil {
		t.Fatal(err)
	}
	if err := CheckRemote("file:///etc/passwd", false); err == nil {
		t.Fatal("scheme")
	}
}

func TestRunPrintPrompt(t *testing.T) {
	res, err := Run(context.Background(), "Hello world.", Options{
		Backend:  BackendPrintPrompt,
		Strength: StrengthParaphrase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "token level") || !strings.Contains(res.Text, "Hello world.") {
		t.Fatalf("%s", res.Text)
	}
	if res.Info["mode"] != BackendPrintPrompt {
		t.Fatalf("%v", res.Info)
	}
}

func TestRunRequiresModel(t *testing.T) {
	_, err := Run(context.Background(), "x", Options{Backend: BackendOllama})
	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("%v", err)
	}
}

func TestNormalize(t *testing.T) {
	b, err := NormalizeBackend("native")
	if err != nil || b != BackendOpenAI {
		t.Fatalf("%q %v", b, err)
	}
	_, err = NormalizeStrength("nope")
	if err == nil {
		t.Fatal("expected strength error")
	}
}

func TestLayerAAfter(t *testing.T) {
	src := "hello\u200bworld"
	res, err := Run(context.Background(), src, Options{
		Backend:     BackendPrintPrompt,
		LayerAAfter: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// print-prompt returns the prompt, not scrubbed body — Layer A after only applies to model output.
	if !strings.Contains(res.Text, "\u200b") && !strings.Contains(res.Prompt, "hello") {
		t.Fatalf("%s", res.Text)
	}
	cleaned, stats, err := scrubLayerA(context.Background(), src, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cleaned, "\u200b") {
		t.Fatalf("zwsp remains: %q %#v", cleaned, stats)
	}
}

func TestRunOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": "rewritten text"},
		})
	}))
	t.Cleanup(srv.Close)

	res, err := Run(context.Background(), "original text here", Options{
		Backend:  BackendOllama,
		Model:    "test",
		Endpoint: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "rewritten text" || res.Info["mode"] != "rewritten" {
		t.Fatalf("%q %#v", res.Text, res.Info)
	}
}

func TestRunOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk-") {
			t.Fatalf("auth=%q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "openai rewrite"}}},
		})
	}))
	t.Cleanup(srv.Close)

	res, err := Run(context.Background(), "hello world", Options{
		Backend:  BackendOpenAI,
		Model:    "gpt-test",
		Endpoint: srv.URL,
		APIKey:   "sk-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "openai rewrite" {
		t.Fatalf("%q", res.Text)
	}
}

func TestRunMultipleCandidates(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		text := "candidate one matches original closely"
		if n > 1 {
			text = "a wholly divergent paraphrase with fresh vocabulary"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": text},
		})
	}))
	t.Cleanup(srv.Close)

	res, err := Run(context.Background(), "alpha beta gamma delta epsilon", Options{
		Backend:    BackendOllama,
		Model:      "test",
		Endpoint:   srv.URL,
		Candidates: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text == "" || res.Info["candidate_scores"] == nil {
		t.Fatalf("text=%q info=%v", res.Text, res.Info)
	}
}

func TestRunOllamaErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, strings.Repeat("x", 300))
	}))
	t.Cleanup(srv.Close)

	_, err := Run(context.Background(), "x", Options{
		Backend:  BackendOllama,
		Model:    "test",
		Endpoint: srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("%v", err)
	}
}

func TestRunLayerAAfterModelOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": "clean\u200btext"},
		})
	}))
	t.Cleanup(srv.Close)

	res, err := Run(context.Background(), "src", Options{
		Backend:     BackendOllama,
		Model:       "test",
		Endpoint:    srv.URL,
		LayerAAfter: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, "\u200b") {
		t.Fatalf("layer A after failed: %q", res.Text)
	}
}

func TestNormalizeBackendAliases(t *testing.T) {
	for _, in := range []string{"", "print", "prompt", "native", "openai-compatible"} {
		if _, err := NormalizeBackend(in); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
	}
	if _, err := NormalizeBackend("bogus"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLexicalDivergenceEdgeCases(t *testing.T) {
	if LexicalDivergence("", "") != 0 {
		t.Fatal("both empty")
	}
	if LexicalDivergence("a", "") != 1 {
		t.Fatal("one empty")
	}
}

func TestNormalizeStrengthAll(t *testing.T) {
	for _, s := range []string{"humanize", "code", "backtranslate", "structural", ""} {
		got, err := NormalizeStrength(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if got == "" {
			t.Fatalf("%s", s)
		}
	}
}

func TestCheckRemoteLoopbackVariants(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]"} {
		if err := CheckRemote("http://"+host+":8080", false); err != nil {
			t.Fatalf("%s: %v", host, err)
		}
	}
	if err := CheckRemote("http://", false); err == nil {
		t.Fatal("missing host")
	}
}
