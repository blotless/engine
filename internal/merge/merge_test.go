package merge

import (
	"testing"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/ports"
)

func TestProtectAndDedup(t *testing.T) {
	f := domain.Finding{
		RuleID: "unicode.zwsp",
		Span:   domain.Span{File: "a.go", Start: 5, End: 8, Line: 1, Col: 2},
		Clean:  domain.CleanStrip,
	}
	dup := f
	protect := []ports.ProtectSpan{{
		Span:   domain.Span{File: "a.go", Start: 0, End: 10},
		Reason: "string_literal",
	}}
	out := Findings([]domain.Finding{f, dup}, protect, false, nil)
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Clean != domain.CleanReportOnly || !out[0].Protect {
		t.Fatalf("%+v", out[0])
	}
	out2 := Findings([]domain.Finding{f}, protect, true, nil)
	if out2[0].Clean != domain.CleanStrip {
		t.Fatalf("aggressive: %+v", out2[0])
	}
	out3 := Findings([]domain.Finding{f}, nil, false, map[string]struct{}{"unicode.zwsp": {}})
	if len(out3) != 0 {
		t.Fatal("disabled")
	}

	c2pa := domain.Finding{
		RuleID: "c2pa.unstructured",
		Family: domain.FamilyC2PA,
		Span:   domain.Span{File: "a.go", Start: 5, End: 8, Line: 1, Col: 2},
		Clean:  domain.CleanStrip,
	}
	out4 := Findings([]domain.Finding{c2pa}, protect, false, nil)
	if len(out4) != 1 || out4[0].Clean != domain.CleanStrip || out4[0].Protect {
		t.Fatalf("c2pa should still strip inside strings: %+v", out4)
	}
}

func TestInvalidSpanSkipped(t *testing.T) {
	out := Findings([]domain.Finding{{
		RuleID: "unicode.zwsp",
		Span:   domain.Span{File: "a.go", Start: -1, End: 2},
	}}, nil, false, nil)
	if len(out) != 0 {
		t.Fatalf("%#v", out)
	}
}

func TestProtectUnicodeTags(t *testing.T) {
	f := domain.Finding{
		RuleID: "unicode.tags",
		Span:   domain.Span{File: "a.go", Start: 1, End: 2},
		Clean:  domain.CleanStrip,
	}
	protect := []ports.ProtectSpan{{Span: domain.Span{File: "a.go", Start: 0, End: 10}, Reason: "string"}}
	out := Findings([]domain.Finding{f}, protect, false, nil)
	if len(out) != 1 || out[0].Protect {
		t.Fatalf("tags not protectable: %+v", out[0])
	}
}

func TestSortOrder(t *testing.T) {
	fs := []domain.Finding{
		{RuleID: "b", Span: domain.Span{File: "z.go", Start: 1, End: 2}},
		{RuleID: "a", Span: domain.Span{File: "a.go", Start: 5, End: 8}},
	}
	out := Findings(fs, nil, false, nil)
	if out[0].Span.File != "a.go" || out[1].Span.File != "z.go" {
		t.Fatalf("%#v", out)
	}
}
