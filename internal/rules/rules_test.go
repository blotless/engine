package rules

import (
	"testing"

	"github.com/blotless/engine/domain"
)

func TestLoad(t *testing.T) {
	list, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("empty catalog")
	}
	if _, ok := Lookup("unicode.zwsp"); !ok {
		t.Fatal("missing unicode.zwsp")
	}
	if _, ok := Lookup("unicode.confusable"); !ok {
		t.Fatal("missing unicode.confusable")
	}
	if _, ok := Lookup("does.not.exist"); ok {
		t.Fatal("unexpected hit")
	}
}

func TestApplyAndMustLookup(t *testing.T) {
	f := Apply(domain.Finding{Message: "keep"}, "unicode.zwsp")
	if f.RuleID != "unicode.zwsp" || f.Family == "" || f.Message != "keep" {
		t.Fatalf("%#v", f)
	}
	unknown := MustLookup("no.such.rule")
	if unknown.ID != "no.such.rule" || unknown.Clean != domain.CleanStrip {
		t.Fatalf("%#v", unknown)
	}
	filled := Apply(domain.Finding{}, "no.such.rule")
	if filled.RuleID != "no.such.rule" || filled.Clean != domain.CleanStrip {
		t.Fatalf("%#v", filled)
	}
}

func TestApplyPreservesClean(t *testing.T) {
	f := Apply(domain.Finding{Clean: domain.CleanReportOnly}, "unicode.zwsp")
	if f.Clean != domain.CleanReportOnly {
		t.Fatalf("%s", f.Clean)
	}
}

func TestApplyFillsMessage(t *testing.T) {
	f := Apply(domain.Finding{}, "unicode.zwsp")
	if f.Message == "" {
		t.Fatal("expected rule description")
	}
}

func TestLookupManyRules(t *testing.T) {
	list, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 20 {
		t.Fatalf("rules=%d", len(list))
	}
}
