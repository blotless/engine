package score

import (
	"strings"
	"testing"

	"github.com/blotless/engine/domain"
)

func TestEmpty(t *testing.T) {
	s := Compute(nil, 3)
	if s.Percent != 0 || s.Agent != "none" {
		t.Fatalf("%+v", s)
	}
}

func TestCursorStamp(t *testing.T) {
	s := Compute([]domain.Finding{{
		RuleID:     "stamp.co_authored_by_ai",
		Family:     domain.FamilyStamp,
		Confidence: domain.ConfidenceCertain,
		Evidence:   "Co-authored-by: Cursor <cursor@example.com>",
		Span:       domain.Span{File: "a.go", Start: 0, End: 10},
	}}, 1)
	if s.Percent < 20 {
		t.Fatalf("percent=%d", s.Percent)
	}
	if s.Agent != "cursor" || s.Label != "Cursor" {
		t.Fatalf("%+v", s)
	}
}

func TestUnicodeUnknownAgent(t *testing.T) {
	s := Compute([]domain.Finding{{
		RuleID:     "unicode.zwsp",
		Family:     domain.FamilyUnicode,
		Confidence: domain.ConfidenceCertain,
		Span:       domain.Span{File: "a.txt", Start: 0, End: 3},
	}}, 1)
	if s.Percent < 10 {
		t.Fatalf("percent=%d", s.Percent)
	}
	if s.Agent != "unknown" {
		t.Fatalf("%+v", s)
	}
}

func TestMetadataNamesAgent(t *testing.T) {
	s := Compute([]domain.Finding{{
		RuleID:     "container.html_meta",
		Family:     domain.FamilyMetadata,
		Confidence: domain.ConfidenceCertain,
		Evidence:   `<meta name="generator" content="Claude">`,
		Span:       domain.Span{File: "a.html", Start: 0, End: 40},
	}}, 1)
	if s.Percent == 0 {
		t.Fatalf("percent=%d", s.Percent)
	}
	if s.Agent != "claude" {
		t.Fatalf("%+v", s)
	}
}

func TestCursorNotOverriddenByOtherHint(t *testing.T) {
	s := Compute([]domain.Finding{
		{
			RuleID:     "stamp.co_authored_by_ai",
			Family:     domain.FamilyStamp,
			Confidence: domain.ConfidenceCertain,
			Evidence:   "Co-authored-by: Cursor <cursor@example.com>",
			Span:       domain.Span{File: "a.go", Start: 0, End: 10},
		},
		{
			RuleID:     "unicode.zwsp",
			Family:     domain.FamilyUnicode,
			Confidence: domain.ConfidenceCertain,
			Span:       domain.Span{File: "a.go", Start: 20, End: 23},
		},
	}, 1)
	if s.Agent != "cursor" || s.Label != "Cursor" {
		t.Fatalf("%+v", s)
	}
}

func TestMultipleStampAgents(t *testing.T) {
	s := Compute([]domain.Finding{
		{
			RuleID:     "stamp.co_authored_by_ai",
			Family:     domain.FamilyStamp,
			Confidence: domain.ConfidenceCertain,
			Evidence:   "Co-authored-by: Cursor <cursor@example.com>",
			Span:       domain.Span{File: "a.go", Start: 0, End: 10},
		},
		{
			RuleID:     "stamp.co_authored_by_ai",
			Family:     domain.FamilyStamp,
			Confidence: domain.ConfidenceCertain,
			Evidence:   "Co-authored-by: Claude <claude@example.com>",
			Span:       domain.Span{File: "b.go", Start: 0, End: 10},
		},
	}, 2)
	if s.Agent != "cursor" && s.Agent != "claude" {
		t.Fatalf("primary=%s", s.Agent)
	}
	if !strings.Contains(s.Label, "Cursor") || !strings.Contains(s.Label, "Claude") {
		t.Fatalf("label=%q", s.Label)
	}
	if s.Agents["cursor"] == 0 || s.Agents["claude"] == 0 {
		t.Fatalf("votes=%v", s.Agents)
	}
}

func TestStatwmNotScored(t *testing.T) {
	s := Compute([]domain.Finding{{
		RuleID:     "statwm.layer_b_prose",
		Family:     domain.FamilyHeuristic,
		Confidence: domain.ConfidenceHeuristic,
		Span:       domain.Span{File: "a.md", Start: 0, End: 10},
	}}, 1)
	if s.Percent != 0 || s.Agent != "none" {
		t.Fatalf("%+v", s)
	}
}

func TestForensicUnknown(t *testing.T) {
	s := Compute([]domain.Finding{{
		RuleID:     "unicode.zwsp",
		Family:     domain.FamilyUnicode,
		Confidence: domain.ConfidenceLikely,
		Span:       domain.Span{File: "a.txt", Start: 0, End: 1},
	}}, 1)
	if s.Agent != "unknown" {
		t.Fatalf("%+v", s)
	}
}

func TestHeuristicGeneric(t *testing.T) {
	s := Compute([]domain.Finding{{
		RuleID:     "heuristic.test",
		Family:     domain.FamilyHeuristic,
		Confidence: domain.ConfidenceHeuristic,
		Span:       domain.Span{File: "a.go", Start: 0, End: 1},
	}}, 1)
	if s.Agent != "generic" {
		t.Fatalf("%+v", s)
	}
}
