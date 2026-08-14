package container

import (
	"bytes"
	"regexp"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

var (
	svgMetaRe = regexp.MustCompile(`(?is)<metadata\b[^>]*>.*?</metadata\s*>`)
	svgXMPRe  = regexp.MustCompile(`(?is)<x:xmpmeta\b[^>]*>.*?</x:xmpmeta\s*>`)
	svgCmtRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	svgGenRe  = regexp.MustCompile(`(?i)\s(inkscape:version|sodipodi:docname|generator)\s*=\s*"[^"]*"`)
)

func scanSVG(u domain.Unit) ([]domain.Finding, error) {
	var out []domain.Finding
	for _, re := range []*regexp.Regexp{svgMetaRe, svgXMPRe} {
		for _, idx := range re.FindAllIndex(u.Bytes, -1) {
			blob := u.Bytes[idx[0]:idx[1]]
			if !svgProvenance(blob) {
				continue
			}
			out = append(out, rules.Apply(domain.Finding{
				Span:     spanFor(u, idx[0], idx[1]),
				Evidence: clip(string(blob), 80),
			}, "container.svg_metadata"))
		}
	}
	for _, idx := range svgCmtRe.FindAllIndex(u.Bytes, -1) {
		if !aiMetaName.Match(u.Bytes[idx[0]:idx[1]]) {
			continue
		}
		out = append(out, rules.Apply(domain.Finding{
			Span:     spanFor(u, idx[0], idx[1]),
			Evidence: "svg comment",
		}, "container.svg_metadata"))
	}
	if len(out) == 0 {
		for _, idx := range svgGenRe.FindAllIndex(u.Bytes, -1) {
			blob := u.Bytes[idx[0]:idx[1]]
			if !generatorAI.Match(blob) && !aiMetaName.Match(blob) {
				continue
			}
			out = append(out, rules.Apply(domain.Finding{
				Span:     spanFor(u, idx[0], idx[1]),
				Evidence: clip(string(blob), 80),
			}, "container.svg_metadata"))
		}
	}
	return out, nil
}

func svgProvenance(blob []byte) bool {
	if looksAI(blob) || aiMetaName.Match(blob) {
		return true
	}
	low := bytes.ToLower(blob)
	return bytes.Contains(low, []byte("xmp")) || bytes.Contains(low, []byte("rdf:")) ||
		bytes.Contains(low, []byte("c2pa"))
}
