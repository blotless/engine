package ast

import (
	"context"
	"strings"

	"github.com/blotless/engine/internal/astgo"
	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/detect/homoglyph"
	"github.com/blotless/engine/internal/rules"
	"github.com/blotless/engine/ports"
)

// Detector finds mixed-script identifiers in Go files.
type Detector struct{}

func (Detector) ID() string { return "astgo" }

func (Detector) Families() []domain.Family {
	return []domain.Family{domain.FamilyHomoglyph, domain.FamilyAST}
}

func (Detector) Scan(_ context.Context, u domain.Unit) ([]domain.Finding, error) {
	if u.Language != domain.LangGo {
		return nil, nil
	}
	f, err := astgo.Parse(u.Path, u.Bytes)
	if err != nil {
		return nil, nil
	}
	var out []domain.Finding
	f.WalkIdents(func(id astgo.Ident) {
		if !homoglyph.MixedScript(id.Name) {
			return
		}
		out = append(out, rules.Apply(domain.Finding{
			Span: domain.Span{
				File:  u.Path,
				Start: id.Span.Start,
				End:   id.Span.End,
				Line:  id.Span.Line,
				Col:   id.Span.Col,
			},
			Evidence: id.Name,
			Symbol:   id.Name,
			Kind:     "ident",
		}, "homoglyph.ident"))
	})
	return out, nil
}

// Protect marks string literals and go:generate comments.
func (Detector) Protect(_ context.Context, u domain.Unit) ([]ports.ProtectSpan, error) {
	if u.Language != domain.LangGo {
		return nil, nil
	}
	f, err := astgo.Parse(u.Path, u.Bytes)
	if err != nil {
		return nil, nil
	}
	var out []ports.ProtectSpan
	for _, s := range f.StringSpans() {
		out = append(out, ports.ProtectSpan{
			Span:   domain.Span{File: u.Path, Start: s.Start, End: s.End, Line: s.Line, Col: s.Col},
			Reason: "string_literal",
		})
	}
	for _, s := range f.GoGenerateSpans() {
		out = append(out, ports.ProtectSpan{
			Span:   domain.Span{File: u.Path, Start: s.Start, End: s.End, Line: s.Line, Col: s.Col},
			Reason: "go_generate",
		})
	}
	return out, nil
}

func OverlapsProtect(f domain.Finding, ps []ports.ProtectSpan) (bool, string) {
	for _, p := range ps {
		if p.Span.Overlaps(f.Span) || p.Span.Contains(f.Span) {
			return true, p.Reason
		}
	}
	return false, ""
}

func CommentFragments(u domain.Unit) []string {
	if u.Language != domain.LangGo {
		return nil
	}
	f, err := astgo.Parse(u.Path, u.Bytes)
	if err != nil {
		return nil
	}
	var out []string
	f.WalkComments(func(c astgo.Comment) {
		t := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		t = strings.TrimSuffix(t, "*/")
		t = strings.TrimSpace(t)
		if len(t) >= 80 {
			out = append(out, t)
		}
	})
	return out
}

var _ ports.Detector = Detector{}
var _ ports.Protector = Detector{}
