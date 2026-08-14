package c2pa

import (
	"context"
	"regexp"
	"unicode/utf8"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/classify"
	"github.com/blotless/engine/internal/rules"
	"github.com/blotless/engine/ports"
)

const (
	vsStart    = '\uFE00'
	vsEnd      = '\uFE0F'
	vsSupStart = '\U000E0100'
	vsSupEnd   = '\U000E01EF'
	minVSRun   = 16
)

var (
	// Split armour/HTML so this source does not contain a full match (clean would strip the detector).
	structuredRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(manifestBegin()) + `.*?` + regexp.QuoteMeta(manifestEnd()))
	scriptRe     = regexp.MustCompile(`(?is)<script\b[^>]*type\s*=\s*["']application/` + `c2pa["'][^>]*>.*?</script>`)
	linkRe       = regexp.MustCompile(`(?is)<link\b[^>]*rel\s*=\s*["']c2pa-` + `manifest["'][^>]*/?>`)
)

func manifestBegin() string { return "-----BEGIN " + "C2PA MANIFEST-----" }
func manifestEnd() string   { return "-----END " + "C2PA MANIFEST-----" }

// Detector finds C2PA 2.4 text embeddings (A.7, A.8, A.9).
type Detector struct{}

func (Detector) ID() string { return "c2pa" }

func (Detector) Families() []domain.Family { return []domain.Family{domain.FamilyC2PA} }

func (Detector) Scan(_ context.Context, u domain.Unit) ([]domain.Finding, error) {
	if !classify.TextPayload(u) {
		return nil, nil
	}
	var out []domain.Finding
	out = append(out, findRegex(u, structuredRe, "c2pa.structured")...)
	out = append(out, findRegex(u, scriptRe, "c2pa.html_script")...)
	out = append(out, findRegex(u, linkRe, "c2pa.html_link")...)
	out = append(out, findVSRuns(u)...)
	return out, nil
}

func findRegex(u domain.Unit, re *regexp.Regexp, id string) []domain.Finding {
	var out []domain.Finding
	idxs := re.FindAllIndex(u.Bytes, -1)
	for _, idx := range idxs {
		if idx[1] <= idx[0] {
			continue
		}
		span := spanFor(u, idx[0], idx[1])
		ev := string(u.Bytes[idx[0]:min(idx[1], idx[0]+80)])
		out = append(out, rules.Apply(domain.Finding{
			Span:     span,
			Evidence: ev,
		}, id))
	}
	return out
}

func findVSRuns(u domain.Unit) []domain.Finding {
	var out []domain.Finding
	src := u.Bytes
	i := 0
	for i < len(src) {
		r, size := utf8.DecodeRune(src[i:])
		if !isVS(r) {
			i += size
			continue
		}
		start := i
		count := 0
		for i < len(src) {
			r, size = utf8.DecodeRune(src[i:])
			if !isVS(r) {
				break
			}
			count++
			i += size
		}
		if count >= minVSRun {
			span := spanFor(u, start, i)
			out = append(out, rules.Apply(domain.Finding{
				Span:     span,
				Evidence: "variation-selector run",
			}, "c2pa.unstructured"))
		}
	}
	return out
}

func isVS(r rune) bool {
	return (r >= vsStart && r <= vsEnd) || (r >= vsSupStart && r <= vsSupEnd)
}

func spanFor(u domain.Unit, start, end int) domain.Span {
	line, col := 1, 1
	i := 0
	for i < start && i < len(u.Bytes) {
		r, size := utf8.DecodeRune(u.Bytes[i:])
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
		i += size
	}
	return domain.Span{File: u.Path, Start: start, End: end, Line: line, Col: col}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ ports.Detector = Detector{}
