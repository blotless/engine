package enrich

import (
	"strings"

	"github.com/blotless/engine/internal/astgo"
	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/snippet"
)

// Findings adds snippet, function, symbol, and kind to each hit.
func Findings(u domain.Unit, fs []domain.Finding) []domain.Finding {
	var goFile *astgo.File
	if u.Language == domain.LangGo {
		goFile, _ = astgo.Parse(u.Path, u.Bytes)
	}
	out := make([]domain.Finding, 0, len(fs))
	for _, f := range fs {
		if f.Snippet == "" && f.Span.Start >= 0 && f.Span.Start <= len(u.Bytes) {
			if u.Kind == domain.KindImage || u.Kind == domain.KindContainer {
				f.Snippet = f.Evidence
			} else {
				end := f.Span.End
				if end < f.Span.Start || end > len(u.Bytes) {
					end = f.Span.Start
				}
				f.Snippet = snippet.Extract(u.Bytes, f.Span.Start, end)
			}
		}
		if goFile != nil {
			ctx := goFile.ContextAt(f.Span.Start)
			if f.Function == "" {
				f.Function = ctx.Function
			}
			if f.Symbol == "" {
				f.Symbol = ctx.Symbol
			}
			if f.Kind == "" {
				f.Kind = ctx.Kind
			}
		}
		if f.Symbol == "" && looksLikeSymbol(f.Evidence) {
			f.Symbol = f.Evidence
		}
		out = append(out, f)
	}
	return out
}

func looksLikeSymbol(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	if strings.ContainsAny(s, " \t\n") {
		return false
	}
	return true
}
