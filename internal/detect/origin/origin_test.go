package origin_test

import (
	"context"
	"testing"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/detect/origin"
	"github.com/blotless/engine/internal/rules"
)

func TestOriginClaudeComment(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	u := domain.Unit{
		Path: "x.go", Kind: domain.KindText, Language: domain.LangGo,
		Bytes: []byte("// refined with Claude for clarity\npackage p\n"),
	}
	fs, err := origin.Detector{}.Scan(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) == 0 {
		t.Fatal("expected hit")
	}
	if fs[0].RuleID != "origin.named.claude" || fs[0].Family != domain.FamilyHeuristic {
		t.Fatalf("%+v", fs[0])
	}
}
