package merge

import (
	"sort"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/ports"
)

// Findings dedups overlapping identical rules and applies protect spans.
func Findings(in []domain.Finding, protect []ports.ProtectSpan, aggressive bool, disabled map[string]struct{}) []domain.Finding {
	var out []domain.Finding
	seen := map[string]struct{}{}
	for _, f := range in {
		if _, skip := disabled[f.RuleID]; skip {
			continue
		}
		if !f.Span.Valid() {
			continue
		}
		if !aggressive {
			if ok, reason := overlaps(f, protect); ok && protectable(f) {
				f.Clean = domain.CleanReportOnly
				f.Protect = true
				f.ProtectKind = reason
			}
		}
		k := f.Key()
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Span.File != out[j].Span.File {
			return out[i].Span.File < out[j].Span.File
		}
		if out[i].Span.Start != out[j].Span.Start {
			return out[i].Span.Start < out[j].Span.Start
		}
		if out[i].Span.End != out[j].Span.End {
			return out[i].Span.End > out[j].Span.End
		}
		return out[i].RuleID < out[j].RuleID
	})
	return out
}

func overlaps(f domain.Finding, ps []ports.ProtectSpan) (bool, string) {
	for _, p := range ps {
		if p.Span.Contains(f.Span) || p.Span.Overlaps(f.Span) {
			return true, p.Reason
		}
	}
	return false, ""
}

func protectable(f domain.Finding) bool {
	if f.Family == domain.FamilyC2PA || f.Family == domain.FamilyMetadata {
		return false
	}
	switch f.RuleID {
	case "unicode.tags", "unicode.vs_supplement":
		return false
	default:
		return true
	}
}
