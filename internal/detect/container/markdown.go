package container

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

var frontRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)
var fmKeyRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*:`)

func scanMarkdown(u domain.Unit) ([]domain.Finding, error) {
	m := frontRe.FindSubmatchIndex(u.Bytes)
	if m == nil {
		return nil, nil
	}
	block := u.Bytes[m[2]:m[3]]
	cleaned, dropped := cleanFrontMatter(block)
	if len(dropped) == 0 {
		return nil, nil
	}
	cleaned = bytes.Trim(cleaned, "\n")
	var repl []byte
	if len(bytes.TrimSpace(cleaned)) == 0 {
		repl = nil
	} else {
		repl = []byte("---\n")
		repl = append(repl, cleaned...)
		repl = append(repl, []byte("\n---\n")...)
	}
	return []domain.Finding{rules.Apply(domain.Finding{
		Span:        spanFor(u, m[0], m[1]),
		Evidence:    join(dropped),
		Replacement: string(repl),
	}, "container.md_front_matter")}, nil
}

func cleanFrontMatter(block []byte) (kept []byte, dropped []string) {
	lines := bytes.Split(block, []byte("\n"))
	var out [][]byte
	dropping := false
	for _, line := range lines {
		if len(line) == 0 {
			if !dropping {
				out = append(out, line)
			}
			continue
		}
		stripped := bytes.TrimSpace(line)
		if len(stripped) == 0 || bytes.HasPrefix(stripped, []byte("#")) {
			if !dropping {
				out = append(out, line)
			}
			continue
		}
		if line[0] == ' ' || line[0] == '\t' || line[0] == '-' {
			if !dropping {
				out = append(out, line)
			}
			continue
		}
		km := fmKeyRe.FindSubmatch(line)
		if km == nil {
			dropping = false
			out = append(out, line)
			continue
		}
		key := string(km[1])
		val := ""
		if i := bytes.IndexByte(line, ':'); i >= 0 && i+1 < len(line) {
			val = string(line[i+1:])
		}
		if dropFrontKey(key, val) {
			dropped = append(dropped, key)
			dropping = true
			continue
		}
		dropping = false
		out = append(out, line)
	}
	if len(dropped) == 0 {
		return block, nil
	}
	return bytes.Join(out, []byte("\n")), dropped
}

func dropFrontKey(key, val string) bool {
	lk := strings.ToLower(key)
	if _, ok := aiFrontKeys[lk]; ok {
		return true
	}
	if aiMetaName.MatchString(key) {
		return true
	}
	return aiMetaName.MatchString(val)
}
