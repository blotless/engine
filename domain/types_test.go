package domain

import "testing"

func TestConfidenceAtLeast(t *testing.T) {
	if ConfidenceCertain.AtLeast(ConfidenceNone) {
		t.Fatal("none should not fail")
	}
	if !ConfidenceCertain.AtLeast(ConfidenceCertain) {
		t.Fatal("certain vs certain")
	}
	if !ConfidenceCertain.AtLeast(ConfidenceLikely) {
		t.Fatal("certain should meet likely")
	}
	if ConfidenceHeuristic.AtLeast(ConfidenceLikely) {
		t.Fatal("heuristic should not meet likely")
	}
	if !ConfidenceHeuristic.AtLeast("any") {
		t.Fatal("any")
	}
	if ConfidenceCertain.Rank() <= ConfidenceLikely.Rank() || Confidence("").Rank() != 0 {
		t.Fatal("rank")
	}
}

func TestSpanOverlaps(t *testing.T) {
	a := Span{File: "a.go", Start: 0, End: 10}
	b := Span{File: "a.go", Start: 5, End: 15}
	c := Span{File: "b.go", Start: 5, End: 15}
	if !a.Overlaps(b) || !b.Overlaps(a) {
		t.Fatal("expected overlap")
	}
	if a.Overlaps(c) {
		t.Fatal("different files")
	}
	if !a.Contains(Span{File: "a.go", Start: 1, End: 2}) {
		t.Fatal("contains")
	}
	if a.Contains(Span{File: "b.go", Start: 1, End: 2}) {
		t.Fatal("contains other file")
	}
	if !a.Valid() || (Span{Start: 0, End: 0}).Valid() || (Span{Start: -1, End: 2}).Valid() {
		t.Fatal("valid")
	}
}

func TestFindingKey(t *testing.T) {
	f := Finding{RuleID: "unicode.zwsp", Span: Span{File: "a.txt", Start: 1, End: 2}}
	if f.Key() != "a.txt:1:2:unicode.zwsp" {
		t.Fatalf("key=%q", f.Key())
	}
}

func TestPatchChanged(t *testing.T) {
	same := Patch{Original: []byte("ab"), Updated: []byte("ab")}
	if same.Changed() {
		t.Fatal("same")
	}
	if !(Patch{Original: []byte("a"), Updated: []byte("ab")}).Changed() {
		t.Fatal("len")
	}
	if !(Patch{Original: []byte("ab"), Updated: []byte("ac")}).Changed() {
		t.Fatal("byte")
	}
	if (Patch{}).Changed() {
		t.Fatal("empty")
	}
}

func TestCleanWritable(t *testing.T) {
	if !CleanStrip.Writable() || CleanReportOnly.Writable() {
		t.Fatal("writable")
	}
}
