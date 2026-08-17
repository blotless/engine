// Package layerb plans best-effort rewrites against token-sampling watermarks
// (Kirchenbauer / SynthID-Text). Inspect reports eligibility; clean rewrites.
package layerb

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/blotless/ast"
	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/comment"
	"github.com/blotless/engine/internal/rules"
)

const minProseRunes = 280
const minCommentRunes = 80

// Plan emits rewrite findings for token-sampling watermark reduction.
// By default only files already marked by Layer A / Files are rewritten.
// force=true runs Layer B even when A found nothing (explicit --layer-b).
func Plan(u domain.Unit, existing []domain.Finding, force bool, strength string) []domain.Finding {
	if u.Kind == domain.KindImage || u.Kind == domain.KindContainer {
		return nil
	}
	if !force && !aiMarked(existing) {
		return nil
	}
	strength = normalizeStrength(strength)
	var out []domain.Finding
	if lang, ok := ast.LangFromPath(u.Path); ok && ast.HasDriver(lang) {
		out = append(out, rules.Apply(domain.Finding{
			Span:    domain.Span{File: u.Path, Start: 0, End: len(u.Bytes), Line: 1, Col: 1},
			Kind:    "structural",
			Message: "Layer B AST transform (" + string(lang) + ")",
		}, "statwm.ast_transform"))
	}
	if proseFile(u.Path) && utf8.RuneCount(u.Bytes) >= minProseRunes {
		end := len(u.Bytes)
		kind := strength
		if kind == "code" {
			kind = "paraphrase"
		}
		out = append(out, rules.Apply(domain.Finding{
			Span:    domain.Span{File: u.Path, Start: 0, End: end, Line: 1, Col: 1},
			Kind:    kind,
			Message: "Layer B " + kind + " (token-level rewrite)",
		}, "statwm.layer_b_prose"))
		return out
	}
	out = append(out, commentFindings(u, strength)...)
	return out
}

func normalizeStrength(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "humanize":
		return "humanize"
	case "code":
		return "code"
	case "backtranslate":
		return "backtranslate"
	case "structural":
		return "structural"
	default:
		return "paraphrase"
	}
}

func aiMarked(fs []domain.Finding) bool {
	for _, f := range fs {
		switch f.Family {
		case domain.FamilyStamp, domain.FamilyC2PA, domain.FamilyUnicode, domain.FamilyMetadata, domain.FamilyHeuristic:
			return true
		}
	}
	return false
}

func proseFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".txt", ".html", ".htm", ".rst", ".adoc":
		return true
	default:
		return false
	}
}

func commentFindings(u domain.Unit, strength string) []domain.Finding {
	var out []domain.Finding
	start := 0
	line := 1
	for start <= len(u.Bytes) {
		rel := indexByte(u.Bytes[start:], '\n')
		end := len(u.Bytes)
		next := len(u.Bytes)
		if rel >= 0 {
			end = start + rel
			next = end + 1
		}
		off, region, ok := comment.Line(u.Bytes[start:end])
		if ok && !skipComment(region) && utf8.RuneCount(region) >= minCommentRunes {
			abs := start + off
			kind := "code"
			if strength == "humanize" || strength == "paraphrase" {
				kind = strength
			}
			out = append(out, rules.Apply(domain.Finding{
				Span:    domain.Span{File: u.Path, Start: abs, End: start + len(u.Bytes[start:end]), Line: line, Col: off + 1},
				Kind:    kind,
				Message: "Layer B comment " + kind,
			}, "statwm.layer_b_comment"))
		}
		if rel < 0 {
			break
		}
		start = next
		line++
	}
	return out
}

// Survey lists files eligible for Layer B. Recommended = already Layer A/Files
// marked. Optional = clean prose/comments that would rewrite only with --layer-b.
func Survey(list []domain.Unit, byFile map[string][]domain.Finding) (recommended, optional []string) {
	for _, u := range list {
		label := u.RelPath
		if label == "" {
			label = u.Path
		}
		existing := byFile[u.Path]
		if len(Plan(u, existing, false, "")) > 0 {
			recommended = append(recommended, label)
			continue
		}
		if len(Plan(u, existing, true, "")) > 0 {
			optional = append(optional, label)
		}
	}
	return recommended, optional
}

func skipComment(region []byte) bool {
	s := strings.TrimSpace(string(region))
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "go:") || strings.HasPrefix(s, "nolint") ||
		strings.HasPrefix(s, "export ") || strings.HasPrefix(s, "spdx-") ||
		strings.HasPrefix(strings.ToLower(s), "copyright")
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}
