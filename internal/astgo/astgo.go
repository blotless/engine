// Package astgo parses Go source and yields byte-offset spans for comments,
// identifiers, and string literals. It has no knowledge of blotless rules.
package astgo

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// File is a parsed Go source unit with a file set for offset mapping.
type File struct {
	Name string
	Src  []byte
	Fset *token.FileSet
	AST  *ast.File
}

// Span is a UTF-8 byte range in the original source.
type Span struct {
	Start int
	End   int
	Line  int
	Col   int
}

// Comment is a single comment with its source span.
type Comment struct {
	Text string
	Span Span
	Doc  bool
}

// Ident is a Go identifier with its source span.
type Ident struct {
	Name string
	Span Span
}

// Literal is a string or rune literal with raw source text and decoded value.
type Literal struct {
	Kind  token.Token
	Raw   string
	Value string
	Span  Span
}

// Parse parses src as a Go file. name is used in error messages and positions.
func Parse(name string, src []byte) (*File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("astgo: parse %s: %w", name, err)
	}
	return &File{Name: name, Src: src, Fset: fset, AST: f}, nil
}

// SpanFor maps a token range to byte offsets in src.
func (f *File) SpanFor(start, end token.Pos) Span {
	p := f.Fset.Position(start)
	e := f.Fset.Position(end)
	return Span{
		Start: p.Offset,
		End:   e.Offset,
		Line:  p.Line,
		Col:   p.Column,
	}
}

// WalkComments calls fn for every comment in the file, including doc comments.
func (f *File) WalkComments(fn func(Comment)) {
	if f.AST.Comments == nil {
		return
	}
	doc := map[*ast.Comment]struct{}{}
	if f.AST.Doc != nil {
		for _, c := range f.AST.Doc.List {
			doc[c] = struct{}{}
		}
	}
	for _, g := range f.AST.Comments {
		for _, c := range g.List {
			_, isDoc := doc[c]
			end := token.Pos(int(c.Pos()) + len(c.Text))
			fn(Comment{
				Text: c.Text,
				Span: f.SpanFor(c.Pos(), end),
				Doc:  isDoc,
			})
		}
	}
}

// WalkIdents calls fn for every identifier, including package and blank names.
func (f *File) WalkIdents(fn func(Ident)) {
	ast.Inspect(f.AST, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		fn(Ident{
			Name: id.Name,
			Span: f.SpanFor(id.Pos(), id.End()),
		})
		return true
	})
}

// WalkStrings calls fn for every string or rune literal.
func (f *File) WalkStrings(fn func(Literal)) {
	ast.Inspect(f.AST, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		if lit.Kind != token.STRING && lit.Kind != token.CHAR {
			return true
		}
		fn(Literal{
			Kind:  lit.Kind,
			Raw:   lit.Value,
			Value: lit.Value,
			Span:  f.SpanFor(lit.Pos(), lit.End()),
		})
		return true
	})
}

// GoGenerateSpans returns spans of //go:generate directive comments.
func (f *File) GoGenerateSpans() []Span {
	var out []Span
	f.WalkComments(func(c Comment) {
		text := c.Text
		if strings.HasPrefix(text, "//go:generate") || strings.HasPrefix(text, "// go:generate") {
			out = append(out, c.Span)
		}
	})
	return out
}

// StringSpans returns spans of string and rune literals.
func (f *File) StringSpans() []Span {
	var out []Span
	f.WalkStrings(func(l Literal) {
		out = append(out, l.Span)
	})
	return out
}
