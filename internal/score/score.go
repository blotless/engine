package score

import (
	"regexp"
	"sort"
	"strings"

	"github.com/blotless/engine/domain"
)

type agentSpec struct {
	id    string
	label string
	re    *regexp.Regexp
}

var agents = []agentSpec{
	{id: "copilot", label: "GitHub Copilot", re: regexp.MustCompile(`(?i)\b(github copilot|copilot)\b`)},
	{id: "chatgpt", label: "ChatGPT", re: regexp.MustCompile(`(?i)\b(chatgpt|chat gpt)\b`)},
	{id: "cursor", label: "Cursor", re: regexp.MustCompile(`(?i)\bcursor\b`)},
	{id: "claude", label: "Claude", re: regexp.MustCompile(`(?i)\b(claude|anthropic)\b`)},
	{id: "codex", label: "Codex", re: regexp.MustCompile(`(?i)\bcodex\b`)},
	{id: "openai", label: "OpenAI", re: regexp.MustCompile(`(?i)\bopenai\b`)},
	{id: "gemini", label: "Gemini", re: regexp.MustCompile(`(?i)\b(gemini|bard)\b`)},
	{id: "devin", label: "Devin", re: regexp.MustCompile(`(?i)\bdevin\b`)},
	{id: "windsurf", label: "Windsurf", re: regexp.MustCompile(`(?i)\bwindsurf\b`)},
	{id: "aider", label: "Aider", re: regexp.MustCompile(`(?i)\baider\b`)},
	{id: "tabnine", label: "Tabnine", re: regexp.MustCompile(`(?i)\btabnine\b`)},
	{id: "cody", label: "Cody", re: regexp.MustCompile(`(?i)\b(sourcegraph|cody)\b`)},
}

var familyCap = map[domain.Family]int{
	domain.FamilyUnicode:   48,
	domain.FamilyC2PA:      36,
	domain.FamilyStamp:     40,
	domain.FamilyHomoglyph: 36,
	domain.FamilyAST:       36,
	domain.FamilyHeuristic: 72,
	domain.FamilyMetadata:  24,
}

// Compute estimates an AI-trace percent and the closest named agent.
func Compute(findings []domain.Finding, filesScanned int) domain.Score {
	if len(findings) == 0 {
		return domain.Score{Percent: 0, Agent: "none", Label: "none", Confidence: domain.ConfidenceNone}
	}

	files := map[string]struct{}{}
	familyRaw := map[domain.Family]int{}
	votes := map[string]int{}
	stampVotes := map[string]int{}
	hintVotes := map[string]int{}
	var evidence []string
	forensic, heuristic := false, false

	for _, f := range findings {
		if strings.HasPrefix(f.RuleID, "statwm.") {
			continue
		}
		files[f.Span.File] = struct{}{}
		familyRaw[f.Family] += weight(f)
		switch f.Family {
		case domain.FamilyUnicode, domain.FamilyC2PA, domain.FamilyStamp, domain.FamilyHomoglyph, domain.FamilyAST, domain.FamilyMetadata:
			forensic = true
		case domain.FamilyHeuristic:
			heuristic = true
		}
		if id, n, hard := agentVote(f); id != "" {
			votes[id] += n
			if hard {
				stampVotes[id] += n
			} else {
				hintVotes[id] += n
			}
			if len(evidence) < 5 {
				ev := f.RuleID
				if f.Evidence != "" {
					ev = f.RuleID + ": " + trimEv(f.Evidence)
				}
				evidence = append(evidence, ev)
			}
		}
	}

	if len(files) == 0 {
		return domain.Score{Percent: 0, Agent: "none", Label: "none", Confidence: domain.ConfidenceNone}
	}

	raw := 0
	for fam, n := range familyRaw {
		if capAt, ok := familyCap[fam]; ok && n > capAt {
			n = capAt
		}
		raw += n
	}

	scanned := filesScanned
	if scanned < 1 {
		scanned = 1
	}
	density := (len(files) * 15) / scanned
	if density > 15 {
		density = 15
	}
	percent := raw + density
	if percent > 100 {
		percent = 100
	}

	agent, label, conf := pickAgents(stampVotes, hintVotes, forensic, heuristic)
	return domain.Score{
		Percent:    percent,
		Agent:      agent,
		Label:      label,
		Agents:     nonempty(votes),
		Confidence: conf,
		Evidence:   evidence,
	}
}

func trimEv(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

func weight(f domain.Finding) int {
	base := 4
	switch f.Family {
	case domain.FamilyUnicode:
		base = 8
	case domain.FamilyC2PA:
		base = 18
	case domain.FamilyStamp:
		base = 16
	case domain.FamilyHomoglyph, domain.FamilyAST:
		base = 14
	case domain.FamilyHeuristic:
		base = 7
	case domain.FamilyMetadata:
		base = 5
	}
	switch f.Confidence {
	case domain.ConfidenceCertain:
		return base
	case domain.ConfidenceLikely:
		return max(3, base*6/10)
	default:
		return max(1, base*3/10)
	}
}

func agentVote(f domain.Finding) (string, int, bool) {
	text := strings.Join([]string{f.Evidence, f.Message, f.RuleID}, " ")
	switch f.Family {
	case domain.FamilyStamp, domain.FamilyC2PA, domain.FamilyMetadata:
		if id := matchAgentText(text); id != "" {
			n := 3
			if f.Confidence == domain.ConfidenceCertain {
				n = 10
			}
			return id, n, true
		}
	case domain.FamilyHeuristic:
		if id := matchAgentText(text); id != "" {
			n := 2
			if strings.HasPrefix(f.RuleID, "origin.named.") {
				n = 4
			}
			return id, n, false
		}
	}
	return "", 0, false
}

func matchAgentText(text string) string {
	for _, a := range agents {
		if a.re.MatchString(text) {
			return a.id
		}
	}
	return ""
}

func pickAgents(stamp, hint map[string]int, forensic, heuristic bool) (string, string, domain.Confidence) {
	if id, label := namedFrom(stamp); id != "" {
		return id, label, domain.ConfidenceLikely
	}
	if id, label := namedFrom(hint); id != "" {
		return id, label, domain.ConfidenceHeuristic
	}
	if forensic {
		return "unknown", "unknown (forensic marks)", domain.ConfidenceLikely
	}
	if heuristic {
		return "generic", "likely AI-generated", domain.ConfidenceHeuristic
	}
	return "none", "none", domain.ConfidenceNone
}

func namedFrom(votes map[string]int) (string, string) {
	type hit struct {
		id    string
		label string
		n     int
	}
	var hits []hit
	for _, a := range agents {
		if n := votes[a.id]; n > 0 {
			hits = append(hits, hit{id: a.id, label: a.label, n: n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].n > hits[j].n })
	if len(hits) == 1 {
		return hits[0].id, hits[0].label
	}
	if len(hits) > 1 {
		labels := make([]string, len(hits))
		for i, h := range hits {
			labels[i] = h.label
		}
		return hits[0].id, strings.Join(labels, ", ")
	}
	return "", ""
}

func nonempty(m map[string]int) map[string]int {
	if len(m) == 0 {
		return nil
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
