package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/ports"
)

func TestFormats(t *testing.T) {
	r := ports.Report{
		FilesScanned: 2,
		Score:        domain.Score{Percent: 41, Agent: "cursor", Label: "Cursor"},
		Findings: []domain.Finding{
			{
				RuleID:     "unicode.zwsp",
				Severity:   domain.SeverityWarning,
				Confidence: domain.ConfidenceCertain,
				Message:    "Zero-width space (U+200B)",
				Snippet:    "hello<U+200B>world",
				Function:   "Hello",
				Symbol:     "s",
				Kind:       "string",
				Span:       domain.Span{File: "a.txt", Line: 1, Col: 2, Start: 1, End: 4},
			},
			{
				RuleID:     "stamp.ai_boilerplate",
				Severity:   domain.SeverityInfo,
				Confidence: domain.ConfidenceLikely,
				Message:    "Likely AI-assistant boilerplate prose",
				Span:       domain.Span{File: "a.txt", Line: 4, Col: 1, Start: 10, End: 20},
			},
		},
	}
	var table, js, yml, sarif bytes.Buffer
	if err := Write(&table, "table", r, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := Write(&js, "json", r, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := Write(&yml, "yaml", r, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := Write(&sarif, "sarif", r, "dev"); err != nil {
		t.Fatal(err)
	}
	got := table.String()
	if !strings.Contains(got, "func Hello") || !strings.Contains(got, "hello<U+200B>world") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "AI trace") || !strings.Contains(got, "likely agent") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "layers") || !strings.Contains(got, "Unicode / stamps") {
		t.Fatal(got)
	}
	if strings.Contains(got, "FILE\tLINE") {
		t.Fatal("old table header still present")
	}
	if !strings.Contains(js.String(), `"function": "Hello"`) {
		t.Fatal(js.String())
	}
	if !strings.Contains(yml.String(), "snippet: hello<U+200B>world") {
		t.Fatal(yml.String())
	}
	if !strings.Contains(sarif.String(), `"version": "2.1.0"`) {
		t.Fatal(sarif.String())
	}
}

func TestEmptyTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "table", ports.Report{FilesScanned: 3}, "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No findings") {
		t.Fatal(buf.String())
	}
	if !strings.Contains(buf.String(), "layers") {
		t.Fatal(buf.String())
	}
	if !strings.Contains(buf.String(), "AI trace  0%") {
		t.Fatal(buf.String())
	}
}

func TestAfterScoreTable(t *testing.T) {
	after := domain.Score{Percent: 0, Agent: "none", Label: "none"}
	r := ports.Report{
		FilesScanned: 1,
		Score:        domain.Score{Percent: 41, Agent: "cursor", Label: "Cursor"},
		After:        &after,
		Leftover: []domain.Finding{{
			RuleID:     "stamp.ai_boilerplate",
			Confidence: domain.ConfidenceLikely,
			Span:       domain.Span{File: "a.go", Line: 2, Col: 1, Start: 4, End: 8},
		}},
		Patches: []domain.Patch{{Path: "a.txt"}},
		DryRun:  true,
		Findings: []domain.Finding{{
			RuleID:     "unicode.zwsp",
			Confidence: domain.ConfidenceCertain,
			Span:       domain.Span{File: "a.txt", Line: 1, Col: 1, Start: 0, End: 1},
		}},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "table", r, "dev"); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "41% → 0%") || !strings.Contains(got, "Cursor → none") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "patched 1 file(s)  dry-run") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "leftover after clean: 1") || !strings.Contains(got, "stamp.ai_boilerplate") {
		t.Fatal(got)
	}
}

func TestMultiAgentTable(t *testing.T) {
	r := ports.Report{
		FilesScanned: 2,
		Score:        domain.Score{Percent: 40, Agent: "cursor", Label: "Cursor, Claude"},
		Findings: []domain.Finding{{
			RuleID:     "stamp.co_authored_by_ai",
			Confidence: domain.ConfidenceCertain,
			Span:       domain.Span{File: "a.go", Line: 1, Col: 1, Start: 0, End: 1},
		}},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "table", r, "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "likely agent  Cursor, Claude") {
		t.Fatal(buf.String())
	}
}

func TestWriteFindingColor(t *testing.T) {
	var buf bytes.Buffer
	f := domain.Finding{
		RuleID:     "unicode.zwsp",
		Severity:   domain.SeverityWarning,
		Confidence: domain.ConfidenceCertain,
		Message:    "Zero-width space",
		Snippet:    "a<U+200B>b",
		Span:       domain.Span{File: "a.txt", Line: 1, Col: 1},
	}
	last := WriteFinding(&buf, f, true, "")
	if last != "a.txt" || buf.Len() == 0 {
		t.Fatalf("last=%q buf=%q", last, buf.String())
	}
	last = WriteFinding(&buf, f, true, "a.txt")
	if last != "a.txt" {
		t.Fatalf("same file header repeat: %q", last)
	}
}

func TestStreamedTable(t *testing.T) {
	var buf bytes.Buffer
	r := ports.Report{
		FilesScanned: 1,
		Streamed:     true,
		Findings: []domain.Finding{{
			RuleID: "unicode.zwsp",
			Span:   domain.Span{File: "a.txt", Line: 1, Col: 1, Start: 0, End: 1},
		}},
	}
	if err := Write(&buf, "table", r, "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "1 files scanned") {
		t.Fatal(buf.String())
	}
}

func TestUnknownFormat(t *testing.T) {
	err := Write(&bytes.Buffer{}, "xml", ports.Report{}, "dev")
	if err == nil {
		t.Fatal("expected format error")
	}
}

func TestReportScoreColors(t *testing.T) {
	var low, mid, high bytes.Buffer
	for i, pct := range []int{10, 30, 60} {
		r := ports.Report{FilesScanned: 1, Score: domain.Score{Percent: pct, Agent: "none", Label: "none"}}
		w := &low
		if i == 1 {
			w = &mid
		}
		if i == 2 {
			w = &high
		}
		if err := Write(w, "table", r, "dev"); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(low.String(), "10%") || !strings.Contains(high.String(), "60%") {
		t.Fatalf("low=%s high=%s", low.String(), high.String())
	}
}

func TestYAMLReport(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "yaml", ports.Report{
		FilesScanned: 1,
		Findings: []domain.Finding{{
			RuleID: "unicode.zwsp",
			Span:   domain.Span{File: "a.txt", Line: 1, Col: 1, Start: 0, End: 1},
		}},
	}, "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "files_scanned:") {
		t.Fatal(buf.String())
	}
}

func TestTableLayerTags(t *testing.T) {
	r := ports.Report{
		FilesScanned: 2,
		Findings: []domain.Finding{
			{RuleID: "c2pa.unstructured", Family: domain.FamilyC2PA, Confidence: domain.ConfidenceCertain, Span: domain.Span{File: "a.go", Line: 1, Col: 1, Start: 0, End: 1}},
			{RuleID: "statwm.layer_b_prose", Confidence: domain.ConfidenceHeuristic, Span: domain.Span{File: "b.md", Line: 1, Col: 1, Start: 0, End: 1}},
		},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "table", r, "dev"); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Files") || !strings.Contains(got, "B") {
		t.Fatal(got)
	}
}

func TestJSONWithLayers(t *testing.T) {
	r := ports.Report{
		FilesScanned: 1,
		Layers:       &ports.Layers{A: 1, Files: 2, B: 3, BFiles: []string{"a.md"}},
	}
	var buf bytes.Buffer
	if err := Write(&buf, "json", r, "dev"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"a": 1`) || !strings.Contains(buf.String(), "a.md") {
		t.Fatal(buf.String())
	}
}

func TestSARIFMinimal(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "sarif", ports.Report{FilesScanned: 0}, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "blotless") {
		t.Fatal(buf.String())
	}
}

func TestWriteFindingNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	WriteFinding(&buf, domain.Finding{
		RuleID: "unicode.zwsp",
		Span:   domain.Span{File: "a.txt", Line: 1, Col: 1},
	}, false, "")
}

func TestWriteSummaryBFilesCap(t *testing.T) {
	files := make([]string, 12)
	for i := range files {
		files[i] = fmt.Sprintf("f%d.md", i)
	}
	var buf bytes.Buffer
	if err := writeSummary(&buf, ports.Report{
		Layers: &ports.Layers{B: 12, BFiles: files},
		Score:  domain.Score{Percent: 10, Label: "none"},
	}, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "f0.md") || !strings.Contains(got, "+4 more") {
		t.Fatal(got)
	}
}

func TestWriteSummaryDetectorHits(t *testing.T) {
	var buf bytes.Buffer
	if err := writeSummary(&buf, ports.Report{
		DetectorHits: map[string]int{"unicode": 2, "stamp": 1, "astgo": 0, "c2pa": 0, "container": 3},
		Score:        domain.Score{Percent: 5, Label: "none"},
	}, false); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "3 hits") {
		t.Fatal(got)
	}
}

func TestWriteSummaryBOptional(t *testing.T) {
	var buf bytes.Buffer
	if err := writeSummary(&buf, ports.Report{
		Layers: &ports.Layers{BOptional: 2},
		Score:  domain.Score{Percent: 0},
	}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "optional") {
		t.Fatal(buf.String())
	}
}

func TestColorEnabledBuffer(t *testing.T) {
	if ColorEnabled(&bytes.Buffer{}) {
		t.Fatal("buffer should not enable color")
	}
}

func TestWriteFindingLongMessage(t *testing.T) {
	long := strings.Repeat("word ", 50)
	var buf bytes.Buffer
	WriteFinding(&buf, domain.Finding{
		Message: long,
		Span:    domain.Span{File: "a.txt", Line: 1, Col: 1},
	}, false, "")
	if !strings.Contains(buf.String(), "...") {
		t.Fatal(buf.String())
	}
}

func TestCompactMsg(t *testing.T) {
	if got := compactMsg("", 10); got != "" {
		t.Fatalf("%q", got)
	}
	short := compactMsg("one two", 10)
	if short != "one two" {
		t.Fatalf("%q", short)
	}
	long := compactMsg(strings.Repeat("x ", 60), 20)
	if !strings.HasSuffix(long, "...") {
		t.Fatalf("%q", long)
	}
}
