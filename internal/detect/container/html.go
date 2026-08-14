package container

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

var (
	metaTagRe  = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	jsonLDRe   = regexp.MustCompile(`(?is)<script\b[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>.*?</script>`)
	dataAIRe   = regexp.MustCompile(`(?i)\sdata-ai[\w-]*\s*=\s*["'][^"']*["']`)
	attrPairRe = regexp.MustCompile(`(?i)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
)

func scanHTML(u domain.Unit) ([]domain.Finding, error) {
	var out []domain.Finding
	src := u.Bytes
	for _, idx := range metaTagRe.FindAllIndex(src, -1) {
		tag := src[idx[0]:idx[1]]
		if isCMSGenerator(tag) {
			continue
		}
		if !htmlAITag(tag) {
			continue
		}
		out = append(out, rules.Apply(domain.Finding{
			Span:     spanFor(u, idx[0], idx[1]),
			Evidence: clip(string(tag), 80),
		}, "container.html_meta"))
	}
	for _, idx := range jsonLDRe.FindAllIndex(src, -1) {
		blob := src[idx[0]:idx[1]]
		if !jsonLDAI(blob) {
			continue
		}
		out = append(out, rules.Apply(domain.Finding{
			Span:     spanFor(u, idx[0], idx[1]),
			Evidence: "json-ld provenance",
		}, "container.html_meta"))
	}
	for _, idx := range dataAIRe.FindAllIndex(src, -1) {
		out = append(out, rules.Apply(domain.Finding{
			Span:     spanFor(u, idx[0], idx[1]),
			Evidence: clip(string(src[idx[0]:idx[1]]), 80),
		}, "container.html_meta"))
	}
	return out, nil
}

func htmlAITag(tag []byte) bool {
	if bytes.Contains(bytes.ToLower(tag), []byte("c2pa")) || bytes.Contains(bytes.ToLower(tag), []byte("contentcredential")) {
		return true
	}
	if aiMetaName.Match(tag) {
		return true
	}
	return looksAI(tag)
}

func jsonLDAI(blob []byte) bool {
	if aiMetaName.Match(blob) {
		return true
	}
	low := bytes.ToLower(blob)
	return bytes.Contains(low, []byte("digitalsourcetype")) ||
		bytes.Contains(low, []byte("trainedalgorithmicmedia")) ||
		bytes.Contains(low, []byte("softwareagent")) ||
		bytes.Contains(low, []byte("c2pa"))
}

// isCMSGenerator implements WR PR 42: lowercase attribute names before lookup.
func isCMSGenerator(tag []byte) bool {
	attrs := parseAttrs(tag)
	name := strings.ToLower(attrs["name"])
	if name == "" {
		name = strings.ToLower(attrs["property"])
	}
	if name != "generator" {
		return false
	}
	if generatorAI.MatchString(attrs["content"]) || generatorAI.Match(tag) {
		return false
	}
	return true
}

func parseAttrs(tag []byte) map[string]string {
	out := map[string]string{}
	for _, m := range attrPairRe.FindAllSubmatch(tag, -1) {
		name := strings.ToLower(string(m[1]))
		val := ""
		switch {
		case len(m[2]) > 0:
			val = string(m[2])
		case len(m[3]) > 0:
			val = string(m[3])
		default:
			val = string(m[4])
		}
		out[name] = val
	}
	return out
}
