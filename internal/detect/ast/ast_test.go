package ast

import (
	"context"
	"strings"
	"testing"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

func TestMixedIdent(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := []byte("package p\nfunc sc\u0430n() {}\n")
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{
		Path: "a.go", Bytes: src, Language: domain.LangGo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].RuleID != "homoglyph.ident" {
		t.Fatalf("%#v", fs)
	}
}

func TestProtectSpans(t *testing.T) {
	src := []byte("package p\nvar s = \"x\"\n//go:generate echo\n")
	ps, err := Detector{}.Protect(context.Background(), domain.Unit{
		Path: "a.go", Bytes: src, Language: domain.LangGo,
	})
	if err != nil {
		t.Fatal(err)
	}
	var str, gen bool
	for _, p := range ps {
		if p.Reason == "string_literal" {
			str = true
		}
		if p.Reason == "go_generate" {
			gen = true
		}
	}
	if !str || !gen {
		t.Fatalf("str=%v gen=%v %#v", str, gen, ps)
	}
}

func TestSkipNonGo(t *testing.T) {
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{
		Path: "a.txt", Bytes: []byte("func sc\u0430n() {}"), Language: domain.LangText,
	})
	if err != nil || len(fs) != 0 {
		t.Fatalf("scan %#v %v", fs, err)
	}
	ps, err := Detector{}.Protect(context.Background(), domain.Unit{
		Path: "a.txt", Bytes: []byte(`var s = "x"`), Language: domain.LangText,
	})
	if err != nil || len(ps) != 0 {
		t.Fatalf("protect %#v %v", ps, err)
	}
	if CommentFragments(domain.Unit{Language: domain.LangText, Bytes: []byte("// x")}) != nil {
		t.Fatal("fragments")
	}
}

func TestParseErrorIsSkip(t *testing.T) {
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{
		Path: "bad.go", Bytes: []byte("not go"), Language: domain.LangGo,
	})
	if err != nil || len(fs) != 0 {
		t.Fatalf("%#v %v", fs, err)
	}
}

func TestCommentFragmentsAndProtectOverlap(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := []byte("package p\n// " + strings.Repeat("word ", 20) + "\nvar s = \"x\"\n")
	u := domain.Unit{Path: "a.go", Bytes: src, Language: domain.LangGo}
	frags := CommentFragments(u)
	if len(frags) != 1 {
		t.Fatalf("frags=%q", frags)
	}
	ps, err := Detector{}.Protect(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	hit, reason := OverlapsProtect(domain.Finding{Span: domain.Span{File: "a.go", Start: 0, End: 1}}, ps)
	if hit {
		t.Fatalf("package clause should not overlap strings: %s", reason)
	}
	var strStart int
	for _, p := range ps {
		if p.Reason == "string_literal" {
			strStart = p.Span.Start
			break
		}
	}
	hit, reason = OverlapsProtect(domain.Finding{Span: domain.Span{File: "a.go", Start: strStart, End: strStart + 1}}, ps)
	if !hit || reason != "string_literal" {
		t.Fatalf("hit=%v reason=%s", hit, reason)
	}
}

func TestDetectorMeta(t *testing.T) {
	d := Detector{}
	if d.ID() != "astgo" || len(d.Families()) == 0 {
		t.Fatalf("id=%s families=%d", d.ID(), len(d.Families()))
	}
}
