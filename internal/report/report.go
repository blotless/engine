package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/ports"
	"gopkg.in/yaml.v3"
)

const Disclaimer = "Layer A: Unicode/stamps. Files: C2PA/EXIF/XMP/doc props (auto-strip). Layer B: best-effort paraphrase against token-sampling watermarks (no vendor-key cert)."

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// JSON writes a machine-readable report.
func JSON(w io.Writer, r ports.Report) error {
	r.Disclaimer = Disclaimer
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// YAML writes the same report as YAML.
func YAML(w io.Writer, r ports.Report) error {
	r.Disclaimer = Disclaimer
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(r)
}

// Table writes a human-readable report grouped by file.
func Table(w io.Writer, r ports.Report) error {
	color := useColor(w)
	if len(r.Findings) == 0 {
		fmt.Fprintf(w, "No findings in %d file(s).\n", r.FilesScanned)
		return writeSummary(w, r, color)
	}
	if !r.Streamed {
		last := ""
		for _, f := range flattenFindings(r.Findings) {
			last = WriteFinding(w, f, color, last)
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w)
	}

	var certain, likely, heuristic int
	for _, f := range r.Findings {
		switch f.Confidence {
		case domain.ConfidenceCertain:
			certain++
		case domain.ConfidenceLikely:
			likely++
		default:
			heuristic++
		}
	}
	fmt.Fprintf(w, "%d files scanned, %d skipped  text=%d  images=%d  docs=%d\n%d findings   certain=%d  likely=%d  heuristic=%d\n",
		r.FilesScanned, r.FilesSkipped, r.TextFiles, r.ImageFiles, r.DocFiles, len(r.Findings), certain, likely, heuristic)
	if r.After != nil {
		if len(r.Leftover) == 0 {
			fmt.Fprintln(w, "leftover after clean: none")
		} else {
			fmt.Fprintf(w, "leftover after clean: %d\n", len(r.Leftover))
			last := ""
			for _, f := range flattenFindings(r.Leftover) {
				last = WriteFinding(w, f, color, last)
			}
			fmt.Fprintln(w)
		}
	}
	return writeSummary(w, r, color)
}

// WriteFinding displays a single finding. lastFile is the preceding file header; the result is
// value should be passed back on the next call.
func WriteFinding(w io.Writer, f domain.Finding, color bool, lastFile string) string {
	if f.Span.File != lastFile {
		if lastFile != "" {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, paint(color, ansiBold+ansiCyan, f.Span.File))
		lastFile = f.Span.File
	}
	loc := fmt.Sprintf("%d:%d", f.Span.Line, f.Span.Col)
	fmt.Fprintf(w, "  %-5s  %-8s  %s  %s\n", layerTag(f), loc, f.RuleID, paint(color, confColor(f.Confidence), string(f.Confidence)))
	if f.Function != "" || f.Symbol != "" || f.Kind != "" {
		var bits []string
		if f.Function != "" {
			bits = append(bits, "func "+f.Function)
		}
		if f.Symbol != "" {
			bits = append(bits, "symbol "+f.Symbol)
		}
		if f.Kind != "" {
			bits = append(bits, f.Kind)
		}
		fmt.Fprintf(w, "            %s\n", paint(color, ansiCyan, strings.Join(bits, "  ·  ")))
	}
	if snip := compactMsg(f.Snippet, 100); snip != "" {
		fmt.Fprintf(w, "            %s\n", paint(color, ansiDim, snip))
	} else if msg := compactMsg(f.Message, 96); msg != "" {
		fmt.Fprintf(w, "            %s\n", paint(color, ansiDim, msg))
	}
	return lastFile
}

// ColorEnabled reports whether w is a color TTY.
func ColorEnabled(w io.Writer) bool { return useColor(w) }

func flattenFindings(fs []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, g := range groupByFile(fs) {
		out = append(out, g.findings...)
	}
	return out
}

func writeSummary(w io.Writer, r ports.Report, color bool) error {
	writeLayers(w, r, color)
	fmt.Fprintf(w, "AI trace  %s\n", paint(color, scoreColor(r.Score.Percent), scoreLine(r.Score, r.After)))
	fmt.Fprintf(w, "likely agent  %s\n", paint(color, ansiCyan, agentLine(r.Score, r.After)))
	if r.Score.Confidence != "" && r.Score.Confidence != domain.ConfidenceNone {
		fmt.Fprintf(w, "origin confidence  %s\n", r.Score.Confidence)
	}
	for i, ev := range r.Score.Evidence {
		if i >= 3 {
			break
		}
		fmt.Fprintf(w, "  evidence  %s\n", ev)
	}
	if r.After == nil {
		fmt.Fprintln(w, "next  blotless clean . --write")
		fmt.Fprintln(w, "      blotless clean . --write --layer-b   # AST transform (+ agent/LLM for prose)")
		fmt.Fprintln(w, "      blotless clean . --write --llm=ollama --layer-b")
	}
	if len(r.Patches) == 0 {
		return nil
	}
	mode := "dry-run"
	if !r.DryRun {
		mode = "written"
	}
	_, err := fmt.Fprintf(w, "patched %d file(s)  %s\n", len(r.Patches), mode)
	return err
}

func writeLayers(w io.Writer, r ports.Report, color bool) {
	var a, files, b, bOpt int
	var bFiles []string
	if r.Layers != nil {
		a, files, b = r.Layers.A, r.Layers.Files, r.Layers.B
		bOpt = r.Layers.BOptional
		bFiles = r.Layers.BFiles
	} else if r.DetectorHits != nil {
		a = r.DetectorHits["unicode"] + r.DetectorHits["stamp"] + r.DetectorHits["astgo"] + r.DetectorHits["c2pa"]
		files = r.DetectorHits["container"]
	}
	fmt.Fprintln(w, paint(color, ansiBold, "layers"))
	fmt.Fprintf(w, "  A      Unicode / stamps     %d hits   verifiable\n", a)
	fmt.Fprintf(w, "  Files  C2PA / metadata      %d hits   verifiable\n", files)
	switch {
	case b > 0:
		fmt.Fprintf(w, "  B      statistical text     %d file(s) eligible  no public detector\n", b)
		writeBFiles(w, bFiles)
	case bOpt > 0:
		fmt.Fprintf(w, "  B      statistical text     %d file(s) optional (--layer-b)  no public detector\n", bOpt)
	default:
		fmt.Fprintln(w, "  B      statistical text     none     no public detector")
	}
}

func writeBFiles(w io.Writer, files []string) {
	const capN = 8
	n := len(files)
	if n > capN {
		n = capN
	}
	for _, f := range files[:n] {
		fmt.Fprintf(w, "         %s\n", f)
	}
	if extra := len(files) - n; extra > 0 {
		fmt.Fprintf(w, "         … +%d more\n", extra)
	}
}

func layerTag(f domain.Finding) string {
	if strings.HasPrefix(f.RuleID, "statwm.") {
		return "B"
	}
	if strings.HasPrefix(f.RuleID, "container.") {
		return "Files"
	}
	switch f.Family {
	case domain.FamilyC2PA, domain.FamilyMetadata:
		return "Files"
	default:
		return "A"
	}
}

func scoreLine(s domain.Score, after *domain.Score) string {
	if after == nil {
		return fmt.Sprintf("%d%%", s.Percent)
	}
	return fmt.Sprintf("%d%% → %d%%", s.Percent, after.Percent)
}

func agentLine(s domain.Score, after *domain.Score) string {
	label := s.Label
	if label == "" {
		label = "none"
	}
	if after == nil {
		return label
	}
	next := after.Label
	if next == "" {
		next = "none"
	}
	if label == next {
		return label
	}
	return label + " → " + next
}

func scoreColor(percent int) string {
	switch {
	case percent >= 50:
		return ansiBold + ansiRed
	case percent >= 20:
		return ansiYellow
	default:
		return ansiDim
	}
}

type fileGroup struct {
	file     string
	findings []domain.Finding
}

func groupByFile(fs []domain.Finding) []fileGroup {
	var order []string
	idx := map[string]int{}
	var groups []fileGroup
	for _, f := range fs {
		file := f.Span.File
		i, ok := idx[file]
		if !ok {
			idx[file] = len(groups)
			order = append(order, file)
			groups = append(groups, fileGroup{file: file})
			i = len(groups) - 1
		}
		groups[i].findings = append(groups[i].findings, f)
	}
	_ = order
	return groups
}

func compactMsg(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max-1]) + "..."
}

func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func confColor(c domain.Confidence) string {
	switch c {
	case domain.ConfidenceCertain:
		return ansiBold + ansiRed
	case domain.ConfidenceLikely:
		return ansiYellow
	default:
		return ansiDim
	}
}

func paint(enabled bool, code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

func sarifMessage(f domain.Finding) string {
	var bits []string
	if f.Function != "" {
		bits = append(bits, "func "+f.Function)
	}
	if f.Symbol != "" {
		bits = append(bits, "symbol "+f.Symbol)
	}
	if f.Snippet != "" {
		bits = append(bits, f.Snippet)
	} else if f.Message != "" {
		bits = append(bits, f.Message)
	}
	return strings.Join(bits, " · ")
}

func sarifLevel(s domain.Severity) string {
	switch s {
	case domain.SeverityError:
		return "error"
	case domain.SeverityWarning:
		return "warning"
	default:
		return "note"
	}
}

type sarifRule struct {
	ID               string `json:"id"`
	ShortDescription struct {
		Text string `json:"text"`
	} `json:"shortDescription"`
}

// SARIF writes SARIF 2.1.0 for CI.
func SARIF(w io.Writer, r ports.Report, toolVersion string) error {
	seen := map[string]string{}
	var rules []sarifRule
	for _, f := range r.Findings {
		if _, ok := seen[f.RuleID]; ok {
			continue
		}
		seen[f.RuleID] = f.Message
		item := sarifRule{ID: f.RuleID}
		item.ShortDescription.Text = f.Message
		rules = append(rules, item)
	}
	results := make([]map[string]any, 0, len(r.Findings))
	for _, f := range r.Findings {
		results = append(results, map[string]any{
			"ruleId":  f.RuleID,
			"level":   sarifLevel(f.Severity),
			"message": map[string]string{"text": sarifMessage(f)},
			"locations": []map[string]any{
				{
					"physicalLocation": map[string]any{
						"artifactLocation": map[string]string{"uri": f.Span.File},
						"region": map[string]any{
							"startLine":   f.Span.Line,
							"startColumn": f.Span.Col,
							"byteOffset":  f.Span.Start,
							"byteLength":  f.Span.End - f.Span.Start,
							"snippet":     map[string]string{"text": f.Snippet},
						},
					},
				},
			},
			"properties": map[string]string{
				"function": f.Function,
				"symbol":   f.Symbol,
				"kind":     f.Kind,
			},
		})
	}
	props := map[string]any{
		"aiTracePercent": r.Score.Percent,
		"likelyAgent":    r.Score.Label,
	}
	if r.Score.Confidence != "" {
		props["originConfidence"] = string(r.Score.Confidence)
	}
	if len(r.Score.Evidence) > 0 {
		props["originEvidence"] = r.Score.Evidence
	}
	if r.After != nil {
		props["afterPercent"] = r.After.Percent
		props["afterAgent"] = r.After.Label
	}
	doc := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"runs": []any{
			map[string]any{
				"tool": map[string]any{
					"driver": map[string]any{
						"name":           "blotless",
						"informationUri": "https://github.com/blotless/blotless",
						"version":        toolVersion,
						"rules":          rules,
					},
				},
				"results":    results,
				"properties": props,
			},
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// Write dispatches a format name.
func Write(w io.Writer, format string, r ports.Report, version string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "table":
		return Table(w, r)
	case "json":
		return JSON(w, r)
	case "yaml", "yml":
		return YAML(w, r)
	case "sarif":
		return SARIF(w, r, version)
	default:
		return fmt.Errorf("report: unknown format %q (table|json|yaml|sarif)", format)
	}
}
