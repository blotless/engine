package astgo

import (
	"go/token"
	"strings"
	"testing"
)

func TestParseWalk(t *testing.T) {
	src := []byte(`package p

// doc comment
func Hello() string {
	x := "hi"
	return x
}

//go:generate echo hi
`)
	f, err := Parse("walk.go", src)
	if err != nil {
		t.Fatal(err)
	}

	var comments []Comment
	f.WalkComments(func(c Comment) { comments = append(comments, c) })
	if len(comments) < 2 {
		t.Fatalf("comments=%d", len(comments))
	}

	var idents []Ident
	f.WalkIdents(func(id Ident) { idents = append(idents, id) })
	foundHello := false
	for _, id := range idents {
		if id.Name == "Hello" {
			foundHello = true
			got := string(src[id.Span.Start:id.Span.End])
			if got != "Hello" {
				t.Fatalf("ident span %q", got)
			}
		}
	}
	if !foundHello {
		t.Fatal("Hello ident not found")
	}

	var lits []Literal
	f.WalkStrings(func(l Literal) { lits = append(lits, l) })
	if len(lits) != 1 || lits[0].Kind != token.STRING {
		t.Fatalf("lits=%v", lits)
	}
	if !strings.Contains(lits[0].Raw, "hi") {
		t.Fatalf("raw=%s", lits[0].Raw)
	}

	gens := f.GoGenerateSpans()
	if len(gens) != 1 {
		t.Fatalf("gogenerate=%d", len(gens))
	}
	got := string(src[gens[0].Start:gens[0].End])
	if !strings.Contains(got, "go:generate") {
		t.Fatalf("generate span %q", got)
	}

	spans := f.StringSpans()
	if len(spans) != 1 {
		t.Fatalf("string spans=%d", len(spans))
	}
}

func TestRuneAndGenerateSpace(t *testing.T) {
	src := []byte("package p\nconst r = 'x'\n// go:generate echo\n")
	f, err := Parse("rune.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	f.WalkStrings(func(l Literal) {
		n++
		if l.Kind != token.CHAR {
			t.Fatalf("kind=%v", l.Kind)
		}
	})
	if n != 1 {
		t.Fatalf("lits=%d", n)
	}
	if len(f.GoGenerateSpans()) != 1 {
		t.Fatal("generate")
	}
}

func TestContextAtComment(t *testing.T) {
	src := []byte("package p\n// hello there\n")
	f, err := Parse("c.go", src)
	if err != nil {
		t.Fatal(err)
	}
	off := 0
	f.WalkComments(func(c Comment) {
		if strings.Contains(c.Text, "hello") {
			off = c.Span.Start + 3
		}
	})
	ctx := f.ContextAt(off)
	if ctx.Kind != "comment" {
		t.Fatalf("%+v", ctx)
	}
	code := f.ContextAt(0)
	if code.Kind != "code" && code.Kind != "ident" {
		t.Fatalf("pkg: %+v", code)
	}
}

func TestParseError(t *testing.T) {
	_, err := Parse("bad.go", []byte("not go"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestContextAtMethodReceiver(t *testing.T) {
	src := []byte("package p\n\ntype T struct{}\nfunc (p *T) Work() {}\n")
	f, err := Parse("m.go", src)
	if err != nil {
		t.Fatal(err)
	}
	off := strings.Index(string(src), "Work")
	ctx := f.ContextAt(off)
	if ctx.Function != "*T.Work" || ctx.Symbol != "Work" {
		t.Fatalf("%+v", ctx)
	}
}
