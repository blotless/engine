package domain

import "fmt"

// Family groups detection algorithms.
type Family string

const (
	FamilyUnicode   Family = "unicode"
	FamilyC2PA      Family = "c2pa"
	FamilyStamp     Family = "stamp"
	FamilyHomoglyph Family = "homoglyph"
	FamilyAST       Family = "ast"
	FamilyMetadata  Family = "metadata"
	FamilyHeuristic Family = "heuristic"
)

// Severity is a finding impact class.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Confidence is how sure a detector is.
type Confidence string

const (
	ConfidenceCertain   Confidence = "certain"
	ConfidenceLikely    Confidence = "likely"
	ConfidenceHeuristic Confidence = "heuristic"
)

// Rank returns a higher number for stronger confidence.
func (c Confidence) Rank() int {
	switch c {
	case ConfidenceCertain:
		return 3
	case ConfidenceLikely:
		return 2
	case ConfidenceHeuristic:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether c meets the fail-on threshold.
func (c Confidence) AtLeast(min Confidence) bool {
	if min == "" || min == ConfidenceNone {
		return false
	}
	if min == "any" {
		return true
	}
	return c.Rank() >= min.Rank()
}

// ConfidenceNone disables fail-on.
const ConfidenceNone Confidence = "none"

// CleanStrategy says what a cleaner may do.
type CleanStrategy string

const (
	CleanStrip      CleanStrategy = "strip"
	CleanNormalize  CleanStrategy = "normalize"
	CleanRewrite    CleanStrategy = "rewrite"
	CleanReportOnly CleanStrategy = "report_only"
)

// Writable reports whether the strategy may mutate bytes.
func (s CleanStrategy) Writable() bool {
	return s == CleanStrip || s == CleanNormalize || s == CleanRewrite
}

// Language is a classified source kind.
type Language string

const (
	LangUnknown   Language = ""
	LangGo        Language = "go"
	LangYAML      Language = "yaml"
	LangJSON      Language = "json"
	LangMarkdown  Language = "markdown"
	LangHTML      Language = "html"
	LangText      Language = "text"
	LangImage     Language = "image"
	LangContainer Language = "container"
)

// Kind is the blot pipeline class for a file (watermarks-remover layers).
type Kind string

const (
	KindText      Kind = "text"
	KindImage     Kind = "image"
	KindContainer Kind = "container"
)

// Span is a UTF-8 byte range inside a file.
type Span struct {
	File  string `json:"file" yaml:"file"`
	Start int    `json:"start" yaml:"start"`
	End   int    `json:"end" yaml:"end"`
	Line  int    `json:"line" yaml:"line"`
	Col   int    `json:"col" yaml:"col"`
}

// Overlaps reports whether two spans share bytes in the same file.
func (s Span) Overlaps(o Span) bool {
	if s.File != o.File {
		return false
	}
	return s.Start < o.End && o.Start < s.End
}

// Contains reports whether o is fully inside s.
func (s Span) Contains(o Span) bool {
	if s.File != o.File {
		return false
	}
	return s.Start <= o.Start && o.End <= s.End
}

// Valid reports whether the span has a non-empty range.
func (s Span) Valid() bool {
	return s.Start >= 0 && s.End > s.Start
}

// Finding is one detector hit.
type Finding struct {
	RuleID      string        `json:"rule_id" yaml:"rule_id"`
	Family      Family        `json:"family" yaml:"family"`
	Severity    Severity      `json:"severity" yaml:"severity"`
	Confidence  Confidence    `json:"confidence" yaml:"confidence"`
	Span        Span          `json:"span" yaml:"span"`
	Message     string        `json:"message" yaml:"message"`
	Evidence    string        `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Snippet     string        `json:"snippet,omitempty" yaml:"snippet,omitempty"`
	Symbol      string        `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Function    string        `json:"function,omitempty" yaml:"function,omitempty"`
	Kind        string        `json:"kind,omitempty" yaml:"kind,omitempty"`
	Clean       CleanStrategy `json:"clean" yaml:"clean"`
	Replacement string        `json:"replacement,omitempty" yaml:"replacement,omitempty"`
	Protect     bool          `json:"protect,omitempty" yaml:"protect,omitempty"`
	ProtectKind string        `json:"protect_kind,omitempty" yaml:"protect_kind,omitempty"`
}

// Key is used for dedup.
func (f Finding) Key() string {
	return fmt.Sprintf("%s:%d:%d:%s", f.Span.File, f.Span.Start, f.Span.End, f.RuleID)
}

// Unit is a decoded file ready for detectors. Detectors must not read the FS.
type Unit struct {
	Path     string   `json:"path"`
	RelPath  string   `json:"rel_path,omitempty"`
	Bytes    []byte   `json:"-"`
	Charset  string   `json:"charset,omitempty"`
	Language Language `json:"language,omitempty"`
	Kind     Kind     `json:"kind,omitempty"`
	HasBOM   bool     `json:"has_bom,omitempty"`
}

// Patch is a planned mutation of a single file.
type Patch struct {
	Path     string    `json:"path" yaml:"path"`
	Original []byte    `json:"-" yaml:"-"`
	Updated  []byte    `json:"-" yaml:"-"`
	Applied  []Finding `json:"applied,omitempty" yaml:"applied,omitempty"`
	Skipped  []Finding `json:"skipped,omitempty" yaml:"skipped,omitempty"`
}

// Changed reports whether Updated differs from Original.
func (p Patch) Changed() bool {
	if len(p.Original) != len(p.Updated) {
		return true
	}
	for i := range p.Original {
		if p.Original[i] != p.Updated[i] {
			return true
		}
	}
	return false
}

// Score is an aggregate forensic AI-trace estimate for a scan.
type Score struct {
	Percent    int            `json:"percent" yaml:"percent"`
	Agent      string         `json:"agent" yaml:"agent"`
	Label      string         `json:"label" yaml:"label"`
	Agents     map[string]int `json:"agents,omitempty" yaml:"agents,omitempty"`
	Confidence Confidence     `json:"confidence,omitempty" yaml:"confidence,omitempty"`
	Evidence   []string       `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}
