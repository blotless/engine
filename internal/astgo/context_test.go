package astgo

import "testing"

func TestContextAt(t *testing.T) {
	src := []byte(`package p

func Hello() string {
	x := "hi"
	return x
}

func (t *T) Run() {}
`)
	f, err := Parse("ctx.go", src)
	if err != nil {
		t.Fatal(err)
	}
	helloOff := 0
	f.WalkIdents(func(id Ident) {
		if id.Name == "Hello" {
			helloOff = id.Span.Start
		}
	})
	ctx := f.ContextAt(helloOff)
	if ctx.Function != "Hello" || ctx.Symbol != "Hello" {
		t.Fatalf("%+v", ctx)
	}

	xOff := 0
	f.WalkIdents(func(id Ident) {
		if id.Name == "x" && xOff == 0 {
			xOff = id.Span.Start
		}
	})
	in := f.ContextAt(xOff)
	if in.Function != "Hello" || in.Symbol != "x" {
		t.Fatalf("x: %+v", in)
	}

	runOff := 0
	f.WalkIdents(func(id Ident) {
		if id.Name == "Run" {
			runOff = id.Span.Start
		}
	})
	m := f.ContextAt(runOff)
	if m.Function != "*T.Run" {
		t.Fatalf("method: %+v", m)
	}

	strOff := 0
	f.WalkStrings(func(l Literal) { strOff = l.Span.Start + 1 })
	s := f.ContextAt(strOff)
	if s.Kind != "string" || s.Function != "Hello" {
		t.Fatalf("string: %+v", s)
	}
}
