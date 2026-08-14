package llm

import (
	"context"
	"testing"

	"github.com/blotless/engine/ports"
)

func TestNoop(t *testing.T) {
	n, err := New(Config{Mode: "off"})
	if err != nil {
		t.Fatal(err)
	}
	if n.Name() != "noop" {
		t.Fatal(n.Name())
	}
	out, err := n.Classify(context.Background(), ports.ClassifyIn{Fragment: "x"})
	if err != nil || out.Rationale != "" {
		t.Fatalf("%v %#v", err, out)
	}
	rw, err := n.Rewrite(context.Background(), ports.RewriteIn{Fragment: "x"})
	if err != nil || rw.Rewrite != "" {
		t.Fatalf("%v %#v", err, rw)
	}
	if err := n.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewUnknownMode(t *testing.T) {
	_, err := New(Config{Mode: "gpt"})
	if err == nil {
		t.Fatal("expected error")
	}
	n, err := New(Config{})
	if err != nil || n.Name() != "noop" {
		t.Fatalf("%v %#v", err, n)
	}
}

func TestParseRewriteRawAndJSON(t *testing.T) {
	got := parseRewrite([]byte(`{"rewrite":"hello there","rationale":"x"}`))
	if got.Rewrite != "hello there" {
		t.Fatalf("%#v", got)
	}
	got = parseRewrite([]byte("rewritten prose only"))
	if got.Rewrite != "rewritten prose only" {
		t.Fatalf("%#v", got)
	}
	got = parseRewrite([]byte("```\nplain rewrite\n```"))
	if got.Rewrite != "plain rewrite" {
		t.Fatalf("%#v", got)
	}
}
