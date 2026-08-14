package ports

import (
	"context"
	"io"

	"github.com/blotless/engine/domain"
)

// FileMeta is a walked filesystem entry.
type FileMeta struct {
	Path    string
	RelPath string
	Size    int64
}

// WalkFunc is called for each candidate file.
type WalkFunc func(ctx context.Context, meta FileMeta) error

// Walker enumerates files. Git-aware walkers can replace the FS walker later.
type Walker interface {
	Walk(ctx context.Context, roots []string, fn WalkFunc) error
}

// Detector scans one already-loaded unit.
type Detector interface {
	ID() string
	Families() []domain.Family
	Scan(ctx context.Context, u domain.Unit) ([]domain.Finding, error)
}

// Protector optionally marks spans that must not be mutated unless aggressive.
type Protector interface {
	Protect(ctx context.Context, u domain.Unit) ([]ProtectSpan, error)
}

// ProtectSpan is a region that defaults to report-only cleaning.
type ProtectSpan struct {
	Span   domain.Span
	Reason string
}

// Cleaner plans byte-level mutations for a unit.
type Cleaner interface {
	CanClean(domain.Finding) bool
	Plan(u domain.Unit, findings []domain.Finding) (domain.Patch, error)
}

// ClassifyIn is a fragment sent to an LLM.
type ClassifyIn struct {
	File     string
	Fragment string
	Context  string
}

// ClassifyOut is schema-validated LLM output.
type ClassifyOut struct {
	RuleSuggestion string `json:"rule_suggestion"`
	Rationale      string `json:"rationale"`
	Confidence     string `json:"confidence"`
	Agent          string `json:"agent,omitempty"`
}

// RewriteIn asks the LLM to paraphrase a fragment.
type RewriteIn struct {
	Fragment string
	Context  string
	Strength string
}

// RewriteOut is schema-validated LLM rewrite output.
type RewriteOut struct {
	Rewrite   string `json:"rewrite"`
	Rationale string `json:"rationale"`
}

// LLM is optional. The default implementation is a noop.
type LLM interface {
	Name() string
	Classify(ctx context.Context, in ClassifyIn) (ClassifyOut, error)
	Rewrite(ctx context.Context, in RewriteIn) (RewriteOut, error)
	Close() error
}

// Reporter writes a scan result.
type Reporter interface {
	Write(w io.Writer, res Report) error
}

// Report is the formatter view of a scan.
type Report struct {
	FilesScanned int              `json:"files_scanned" yaml:"files_scanned"`
	FilesSkipped int              `json:"files_skipped" yaml:"files_skipped"`
	Findings     []domain.Finding `json:"findings" yaml:"findings"`
	Patches      []domain.Patch   `json:"patches,omitempty" yaml:"patches,omitempty"`
	DryRun       bool             `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	Score        domain.Score     `json:"score" yaml:"score"`
	After        *domain.Score    `json:"after,omitempty" yaml:"after,omitempty"`
	Leftover     []domain.Finding `json:"leftover,omitempty" yaml:"leftover,omitempty"`
	Disclaimer   string           `json:"disclaimer,omitempty" yaml:"disclaimer,omitempty"`
	Streamed     bool             `json:"-" yaml:"-"`
	TextFiles    int              `json:"text_files,omitempty" yaml:"text_files,omitempty"`
	ImageFiles   int              `json:"image_files,omitempty" yaml:"image_files,omitempty"`
	DocFiles     int              `json:"doc_files,omitempty" yaml:"doc_files,omitempty"`
	DetectorHits map[string]int   `json:"detector_hits,omitempty" yaml:"detector_hits,omitempty"`
	Layers       *Layers          `json:"layers,omitempty" yaml:"layers,omitempty"`
}

// Layers is the three-channel inspect summary (WR A / Files / B).
type Layers struct {
	A         int      `json:"a"`
	Files     int      `json:"files"`
	B         int      `json:"b"`
	BOptional int      `json:"b_optional,omitempty" yaml:"b_optional,omitempty"`
	BFiles    []string `json:"b_files,omitempty" yaml:"b_files,omitempty"`
	Note      string   `json:"note,omitempty" yaml:"note,omitempty"`
}

// Progress receives live scan updates. Calls are sequential.
type Progress interface {
	File(done, total int, path, detector string)
	Finding(domain.Finding)
	Finish()
}
