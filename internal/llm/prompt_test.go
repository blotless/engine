package llm

import (
	"strings"
	"testing"

	"github.com/blotless/engine/ports"
)

func TestRewritePromptStrength(t *testing.T) {
	in := ports.RewriteIn{Fragment: "Hello world."}
	p := rewritePrompt(in)
	if !strings.Contains(p, "token level") {
		t.Fatalf("paraphrase: %s", p)
	}
	in.Strength = "humanize"
	p = rewritePrompt(in)
	if !strings.Contains(p, "human wrote") {
		t.Fatalf("humanize: %s", p)
	}
	in.Strength = "code"
	p = rewritePrompt(in)
	if !strings.Contains(p, "public API") {
		t.Fatalf("code: %s", p)
	}
}
