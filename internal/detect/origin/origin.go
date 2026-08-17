// Package origin detects soft AI-origin heuristics (not SynthID verification).
package origin

import (
	"bytes"
	"context"
	"regexp"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/classify"
	"github.com/blotless/engine/internal/comment"
	"github.com/blotless/engine/internal/rules"
	"github.com/blotless/engine/ports"
)

type lineRule struct {
	id  string
	re  *regexp.Regexp
	max int // max hits per file
}

var lineRules = []lineRule{
	{id: "origin.named.claude", re: regexp.MustCompile(`(?i)\b(claude|anthropic)\b`), max: 2},
	{id: "origin.named.chatgpt", re: regexp.MustCompile(`(?i)\b(chatgpt|chat\s*gpt)\b`), max: 2},
	{id: "origin.named.gemini", re: regexp.MustCompile(`(?i)\b(gemini|google\s+bard)\b`), max: 2},
	{id: "origin.named.copilot", re: regexp.MustCompile(`(?i)\b(github\s+)?copilot\b`), max: 2},
	{id: "origin.named.cursor", re: regexp.MustCompile(`(?i)\bcursor\s+(agent|tab|composer)\b`), max: 2},
	{id: "origin.style.boilerplate", re: regexp.MustCompile(`(?i)(sure,?\s+here'?s|as an ai|i'?d be happy to|let me know if you need|hope this helps|of course!?\s+here)`), max: 3},
}

// Detector finds soft origin fingerprints in comments and short prose lines.
type Detector struct{}

func (Detector) ID() string { return "origin" }

func (Detector) Families() []domain.Family { return []domain.Family{domain.FamilyHeuristic} }

func (Detector) Scan(_ context.Context, u domain.Unit) ([]domain.Finding, error) {
	if !classify.TextPayload(u) {
		return nil, nil
	}
	counts := map[string]int{}
	var out []domain.Finding
	start := 0
	lineNo := 1
	for start <= len(u.Bytes) {
		rel := bytes.IndexByte(u.Bytes[start:], '\n')
		end := len(u.Bytes)
		next := len(u.Bytes)
		if rel >= 0 {
			end = start + rel
			next = end + 1
		}
		line := u.Bytes[start:end]
		cOff, region, inComment := comment.Line(line)
		scan := region
		off := cOff
		if !inComment {
			// also scan short markdown/prose lines (not whole code lines)
			trim := bytes.TrimSpace(line)
			if len(trim) == 0 || trim[0] == '{' || trim[0] == '[' {
				if rel < 0 {
					break
				}
				start = next
				lineNo++
				continue
			}
			scan = line
			off = 0
		}
		for _, lr := range lineRules {
			if counts[lr.id] >= lr.max {
				continue
			}
			if !lr.re.Match(scan) {
				continue
			}
			abs := start + off
			spanEnd := start + len(line)
			if next < spanEnd {
				spanEnd = next
			}
			ev := string(scan)
			if len(ev) > 120 {
				ev = ev[:117] + "..."
			}
			out = append(out, rules.Apply(domain.Finding{
				Span:     domain.Span{File: u.Path, Start: abs, End: spanEnd, Line: lineNo, Col: off + 1},
				Message:  "origin hint: " + lr.id,
				Evidence: ev,
			}, lr.id))
			counts[lr.id]++
		}
		if rel < 0 {
			break
		}
		start = next
		lineNo++
	}
	return out, nil
}

var _ ports.Detector = Detector{}
