package clean

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/blotless/engine/domain"
)

func TestPlanAndApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	orig := []byte("abXcd")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	u := domain.Unit{Path: path, Bytes: orig, Language: domain.LangText}
	p, err := Planner{}.Plan(u, []domain.Finding{{
		Clean: domain.CleanStrip,
		Span:  domain.Span{File: path, Start: 2, End: 3},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.Updated, []byte("abcd")) {
		t.Fatalf("%q", p.Updated)
	}
	if err := Apply(p, ".bak"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, []byte("abcd")) {
		t.Fatalf("wrote %q", got)
	}
	bak, _ := os.ReadFile(path + ".bak")
	if !bytes.Equal(bak, orig) {
		t.Fatalf("backup %q", bak)
	}
}

func TestApplyNoChange(t *testing.T) {
	p := domain.Patch{Path: filepath.Join(t.TempDir(), "x.txt"), Original: []byte("a"), Updated: []byte("a")}
	if err := Apply(p, ".bak"); err != nil {
		t.Fatal(err)
	}
}

func TestCanClean(t *testing.T) {
	p := Planner{}
	if !p.CanClean(domain.Finding{Clean: domain.CleanStrip}) {
		t.Fatal("strip")
	}
	if !p.CanClean(domain.Finding{Clean: domain.CleanRewrite, Replacement: "x"}) {
		t.Fatal("rewrite with replacement")
	}
	if p.CanClean(domain.Finding{Clean: domain.CleanRewrite}) {
		t.Fatal("rewrite without replacement")
	}
	if p.CanClean(domain.Finding{Clean: domain.CleanReportOnly}) {
		t.Fatal("report only")
	}
}

func TestPlanRenamesHomoglyphIdent(t *testing.T) {
	src := []byte("package p\n\nfunc sc\u0430n() {}\n")
	u := domain.Unit{Path: "a.go", Bytes: src, Language: domain.LangGo}
	p, err := Planner{Gofmt: true}.Plan(u, []domain.Finding{{
		RuleID:   "homoglyph.ident",
		Clean:    domain.CleanNormalize,
		Evidence: "sc\u0430n",
		Symbol:   "sc\u0430n",
		Span:     domain.Span{File: "a.go", Start: bytes.Index(src, []byte("sc")), End: bytes.Index(src, []byte("sc")) + len("sc\u0430n")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(p.Updated, []byte("func scan()")) {
		t.Fatalf("%q", p.Updated)
	}
	if bytes.Contains(p.Updated, []byte("\u0430")) {
		t.Fatalf("cyrillic remains: %q", p.Updated)
	}
}

func TestPlanNFKC(t *testing.T) {
	src := []byte("\ufb01le")
	u := domain.Unit{Path: "a.txt", Bytes: src, Language: domain.LangText, Kind: domain.KindText}
	p, err := Planner{NFKC: true}.Plan(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(p.Updated) != "file" {
		t.Fatalf("nfkc=%q", p.Updated)
	}
}

func TestPlanOverlapSkip(t *testing.T) {
	u := domain.Unit{Path: "a.txt", Bytes: []byte("abcdef"), Language: domain.LangText}
	p, err := Planner{}.Plan(u, []domain.Finding{
		{Clean: domain.CleanStrip, Span: domain.Span{File: "a.txt", Start: 1, End: 4}},
		{Clean: domain.CleanStrip, Span: domain.Span{File: "a.txt", Start: 2, End: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Skipped) == 0 {
		t.Fatalf("%#v", p)
	}
}

func TestApplyBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	orig := []byte("before")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	p := domain.Patch{Path: path, Original: orig, Updated: []byte("after")}
	if err := Apply(p, ".bak"); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil || !bytes.Equal(bak, orig) {
		t.Fatalf("bak=%q err=%v", bak, err)
	}
}

func TestPlanSkipsInvalidSpan(t *testing.T) {
	u := domain.Unit{Path: "a.txt", Bytes: []byte("abc"), Language: domain.LangText}
	p, err := Planner{}.Plan(u, []domain.Finding{{
		Clean: domain.CleanStrip,
		Span:  domain.Span{File: "a.txt", Start: 5, End: 6},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Skipped) != 1 || p.Changed() {
		t.Fatalf("%#v", p)
	}
}

func TestApplyMissingDir(t *testing.T) {
	err := Apply(domain.Patch{
		Path:    filepath.Join(t.TempDir(), "missing", "a.txt"),
		Updated: []byte("x"),
	}, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPlanRewriteReplacement(t *testing.T) {
	u := domain.Unit{Path: "a.txt", Bytes: []byte("hello world"), Language: domain.LangText}
	p, err := Planner{}.Plan(u, []domain.Finding{{
		Clean:       domain.CleanRewrite,
		Replacement: "hi universe",
		Span:        domain.Span{File: "a.txt", Start: 0, End: 11},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if string(p.Updated) != "hi universe" {
		t.Fatalf("%q", p.Updated)
	}
}
