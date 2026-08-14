package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/classify"
	"github.com/blotless/engine/internal/clean"
	astdet "github.com/blotless/engine/internal/detect/ast"
	"github.com/blotless/engine/internal/detect/c2pa"
	"github.com/blotless/engine/internal/detect/container"
	"github.com/blotless/engine/internal/detect/stamp"
	unicodedet "github.com/blotless/engine/internal/detect/unicode"
	"github.com/blotless/engine/internal/enrich"
	"github.com/blotless/engine/internal/layerb"
	"github.com/blotless/engine/internal/llm"
	"github.com/blotless/engine/internal/merge"
	"github.com/blotless/engine/internal/redact"
	"github.com/blotless/engine/internal/report"
	"github.com/blotless/engine/internal/rules"
	"github.com/blotless/engine/internal/score"
	"github.com/blotless/engine/internal/walk"
	"github.com/blotless/engine/ports"
)

// RuleInfo is a public view of an embedded rule spec.
type RuleInfo struct {
	ID          string `json:"id" yaml:"id"`
	Family      string `json:"family" yaml:"family"`
	Severity    string `json:"severity" yaml:"severity"`
	Confidence  string `json:"confidence" yaml:"confidence"`
	Clean       string `json:"clean" yaml:"clean"`
	Description string `json:"description" yaml:"description"`
}

const DefaultMaxFileBytes = 8 << 20

// LLMConfig selects an optional language-model adapter.
type LLMConfig struct {
	Mode     string
	Model    string
	Endpoint string
	Timeout  time.Duration
	APIKey   string
}

// Config is the CLI-facing engine configuration. Cobra/Viper must not leak in.
type Config struct {
	Paths          []string
	Include        []string
	Exclude        []string
	Aggressive     bool
	FailOn         domain.Confidence
	LLM            LLMConfig
	Write          bool
	Backup         string
	MaxFileBytes   int64
	DisabledRules  []string
	Gofmt          bool
	LayerB         bool
	LayerBStrength string
	NFKC           bool
	ForceText      bool
	ForceKind      string // auto|text|image|container
	Logger         *slog.Logger
	Progress       ports.Progress
}

// Result is the outcome of Scan or Clean.
type Result struct {
	FilesScanned int
	FilesSkipped int
	Findings     []domain.Finding
	Patches      []domain.Patch
	DryRun       bool
	Score        domain.Score
	After        *domain.Score
	Leftover     []domain.Finding
	Streamed     bool
	TextFiles    int            `json:"text_files,omitempty" yaml:"text_files,omitempty"`
	ImageFiles   int            `json:"image_files,omitempty" yaml:"image_files,omitempty"`
	DocFiles     int            `json:"doc_files,omitempty" yaml:"doc_files,omitempty"`
	DetectorHits map[string]int `json:"detector_hits,omitempty" yaml:"detector_hits,omitempty"`
	LayerA       int            `json:"layer_a"`
	LayerFiles   int            `json:"layer_files"`
	LayerB       int            `json:"layer_b"`
	LayerBOpt    int            `json:"layer_b_optional,omitempty"`
	LayerBFiles  []string       `json:"layer_b_files,omitempty"`
}

// ShouldFail reports whether findings meet FailOn.
func (r Result) ShouldFail(failOn domain.Confidence) bool {
	for _, f := range r.Findings {
		if f.Confidence.AtLeast(failOn) {
			return true
		}
	}
	return false
}

// Engine orchestrates walk, detect, merge, and clean.
type Engine struct {
	cfg       Config
	walker    ports.Walker
	llm       ports.LLM
	detectors []ports.Detector
	protector ports.Protector
	planner   clean.Planner
	log       *slog.Logger
}

// New constructs an engine from config. LLM adapters are wired here.
func New(cfg Config) (*Engine, error) {
	if _, err := rules.Load(); err != nil {
		return nil, err
	}
	if cfg.MaxFileBytes <= 0 {
		cfg.MaxFileBytes = DefaultMaxFileBytes
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	cfg.Gofmt = true
	adapter, err := llm.New(llm.Config{
		Mode:     cfg.LLM.Mode,
		Model:    cfg.LLM.Model,
		Endpoint: cfg.LLM.Endpoint,
		Timeout:  cfg.LLM.Timeout,
		APIKey:   cfg.LLM.APIKey,
		Logger:   cfg.Logger,
	})
	if err != nil {
		return nil, err
	}
	astDet := astdet.Detector{}
	dets := []ports.Detector{
		unicodedet.Detector{Aggressive: cfg.Aggressive},
		c2pa.Detector{},
		stamp.Detector{},
		container.Detector{},
		astDet,
	}
	return &Engine{
		cfg:       cfg,
		walker:    walk.Walker{Opts: walk.Options{Include: cfg.Include, Exclude: cfg.Exclude, MaxFileBytes: cfg.MaxFileBytes}},
		llm:       adapter,
		detectors: dets,
		protector: astDet,
		planner:   clean.Planner{Gofmt: cfg.Gofmt, NFKC: cfg.NFKC},
		log:       cfg.Logger,
	}, nil
}

// Close stops a managed Ollama process if this engine started it.
func (e *Engine) Close() error {
	if e == nil || e.llm == nil {
		return nil
	}
	return e.llm.Close()
}

// Scan walks paths and returns findings. It never writes files.
func (e *Engine) Scan(ctx context.Context) (*Result, error) {
	return e.run(ctx, false)
}

// Clean plans patches and optionally writes them.
func (e *Engine) Clean(ctx context.Context) (*Result, error) {
	res, err := e.run(ctx, true)
	if err != nil {
		return res, err
	}
	if !e.cfg.Write {
		res.DryRun = true
		return res, nil
	}
	for _, p := range res.Patches {
		if err := clean.Apply(p, e.cfg.Backup); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (e *Engine) run(ctx context.Context, planClean bool) (*Result, error) {
	disabled := map[string]struct{}{}
	for _, id := range e.cfg.DisabledRules {
		disabled[id] = struct{}{}
	}
	res := &Result{}

	e.op("walk", "")
	var list []domain.Unit
	err := e.walker.Walk(ctx, e.cfg.Paths, func(ctx context.Context, meta ports.FileMeta) error {
		raw, err := os.ReadFile(meta.Path)
		if err != nil {
			return err
		}
		_, skip := classify.KindOf(meta.Path, raw)
		if skip && !e.cfg.ForceText {
			res.FilesSkipped++
			return nil
		}
		u := classify.Decode(meta.Path, meta.RelPath, raw)
		if skip && e.cfg.ForceText {
			u.Kind = domain.KindText
		}
		u = applyForceKind(u, e.cfg.ForceKind)
		list = append(list, u)
		return nil
	})
	if err != nil {
		return res, err
	}
	for _, u := range list {
		switch u.Kind {
		case domain.KindImage:
			res.ImageFiles++
		case domain.KindContainer:
			res.DocFiles++
		default:
			res.TextFiles++
		}
	}
	res.FilesScanned = len(list)
	e.op("walk", fmt.Sprintf("%d files  text=%d  images=%d  docs=%d", len(list), res.TextFiles, res.ImageFiles, res.DocFiles))

	all, byFile, _, detHits, err := e.analyze(ctx, list, disabled, analyzeOpts{live: e.cfg.Progress != nil})
	if err != nil {
		return res, err
	}
	if e.cfg.Progress != nil {
		e.cfg.Progress.Finish()
		res.Streamed = true
	}
	res.DetectorHits = detHits
	for _, d := range e.detectors {
		e.op(d.ID(), fmt.Sprintf("%d hits", detHits[d.ID()]))
	}
	res.Findings = all
	res.LayerA = detHits["unicode"] + detHits["stamp"] + detHits["astgo"] + detHits["c2pa"]
	res.LayerFiles = detHits["container"]
	rec, opt := layerb.Survey(list, byFile)
	res.LayerB = len(rec)
	res.LayerBOpt = len(opt)
	res.LayerBFiles = rec
	if len(rec) > 0 {
		e.op("layer-b", fmt.Sprintf("%d files eligible", len(rec)))
	} else if len(opt) > 0 {
		e.op("layer-b", fmt.Sprintf("%d files optional (--layer-b)", len(opt)))
	} else {
		e.op("layer-b", "none eligible")
	}
	res.Score = score.Compute(all, res.FilesScanned)
	e.op("score", fmt.Sprintf("%d%%  %s", res.Score.Percent, res.Score.Label))

	if planClean {
		if e.cfg.LayerB {
			e.op("layer-b", "force (even if Layer A is clean)")
		}
		e.op("clean", "")
		liveLLM := e.llm != nil && e.llm.Name() != "noop"
		changed := 0
		for _, u := range list {
			fs := append([]domain.Finding(nil), byFile[u.Path]...)
			bHits := layerb.Plan(u, fs, e.cfg.LayerB, e.cfg.LayerBStrength)
			if liveLLM {
				fs = append(fs, bHits...)
				fs = e.rewrite(ctx, u, fs)
			} else if len(bHits) > 0 {
				for i := range bHits {
					bHits[i].Clean = domain.CleanReportOnly
					bHits[i].Message = "Layer B: rewrite in the agent, or blotless clean --llm=ollama --layer-b"
				}
				fs = append(fs, bHits...)
			}
			p, err := e.planner.Plan(u, fs)
			if err != nil {
				return res, err
			}
			if p.Changed() {
				res.Patches = append(res.Patches, p)
				changed++
			}
		}
		if e.cfg.Write {
			e.op("clean", fmt.Sprintf("write %d files", changed))
		} else {
			e.op("clean", fmt.Sprintf("dry-run %d files", changed))
		}

		afterList := patchedUnits(list, res.Patches)
		afterFindings, _, _, _, err := e.analyze(ctx, afterList, disabled, analyzeOpts{})
		if err != nil {
			return res, err
		}
		after := score.Compute(afterFindings, res.FilesScanned)
		res.After = &after
		res.Leftover = afterFindings
		e.op("verify", fmt.Sprintf("%d%%  %s", after.Percent, after.Label))
	}
	return res, nil
}

type analyzeOpts struct {
	live bool
}

func (e *Engine) analyze(ctx context.Context, list []domain.Unit, disabled map[string]struct{}, opts analyzeOpts) ([]domain.Finding, map[string][]domain.Finding, map[string]domain.Unit, map[string]int, error) {
	detHits := map[string]int{}
	var all []domain.Finding
	byFile := map[string][]domain.Finding{}
	units := map[string]domain.Unit{}
	total := len(list)

	for i, u := range list {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, nil, err
		}
		var hits []domain.Finding
		for _, d := range e.detectors {
			label := u.RelPath
			if label == "" {
				label = u.Path
			}
			if opts.live && e.cfg.Progress != nil {
				e.cfg.Progress.File(i+1, total, label, d.ID())
			}
			fs, err := d.Scan(ctx, u)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("%s: %w", d.ID(), err)
			}
			detHits[d.ID()] += len(fs)
			hits = append(hits, fs...)
		}
		var protect []ports.ProtectSpan
		var err error
		if e.protector != nil {
			protect, err = e.protector.Protect(ctx, u)
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
		merged := merge.Findings(hits, protect, e.cfg.Aggressive, disabled)
		merged = enrich.Findings(u, merged)
		if opts.live && e.cfg.Progress != nil {
			for _, f := range merged {
				e.cfg.Progress.Finding(f)
			}
		}
		if len(merged) == 0 {
			continue
		}
		byFile[u.Path] = merged
		units[u.Path] = u
		all = append(all, merged...)
	}
	return all, byFile, units, detHits, nil
}

func patchedUnits(list []domain.Unit, patches []domain.Patch) []domain.Unit {
	byPath := map[string][]byte{}
	for _, p := range patches {
		if p.Changed() {
			byPath[p.Path] = p.Updated
		}
	}
	if len(byPath) == 0 {
		return list
	}
	out := make([]domain.Unit, len(list))
	for i, u := range list {
		out[i] = u
		if b, ok := byPath[u.Path]; ok {
			out[i].Bytes = b
		}
	}
	return out
}

func (e *Engine) op(name, detail string) {
	if detail == "" {
		e.log.Info(name)
		return
	}
	e.log.Info(name, "detail", detail)
}

func (e *Engine) rewrite(ctx context.Context, u domain.Unit, fs []domain.Finding) []domain.Finding {
	out := make([]domain.Finding, 0, len(fs))
	for _, f := range fs {
		if f.Clean != domain.CleanRewrite {
			out = append(out, f)
			continue
		}
		frag := ""
		if f.Span.Valid() && f.Span.End <= len(u.Bytes) {
			frag = string(u.Bytes[f.Span.Start:f.Span.End])
		}
		rw, err := e.llm.Rewrite(ctx, ports.RewriteIn{
			Fragment: redact.Secrets(frag),
			Strength: f.Kind,
		})
		if err != nil || rw.Rewrite == "" {
			if f.RuleID == "stamp.ai_boilerplate" {
				f.Clean = domain.CleanStrip
				f.Replacement = ""
			} else {
				f.Clean = domain.CleanReportOnly
			}
			out = append(out, f)
			continue
		}
		f.Replacement = rw.Rewrite
		out = append(out, f)
	}
	return out
}

// Report writes findings using format (table|json|yaml|sarif).
func Report(w io.Writer, format string, res *Result, version string) error {
	rep := ports.Report{
		FilesScanned: res.FilesScanned,
		FilesSkipped: res.FilesSkipped,
		Findings:     res.Findings,
		Patches:      res.Patches,
		DryRun:       res.DryRun,
		Score:        res.Score,
		After:        res.After,
		Leftover:     res.Leftover,
		Streamed:     res.Streamed,
		TextFiles:    res.TextFiles,
		ImageFiles:   res.ImageFiles,
		DocFiles:     res.DocFiles,
		DetectorHits: res.DetectorHits,
		Layers: &ports.Layers{
			A:         res.LayerA,
			Files:     res.LayerFiles,
			B:         res.LayerB,
			BOptional: res.LayerBOpt,
			BFiles:    res.LayerBFiles,
			Note:      "Layer B cannot be detected without a vendor key; listed files are eligible for rewrite.",
		},
	}
	return report.Write(w, format, rep, version)
}

// WriteFinding prints one live finding. Pass lastFile from the previous call.
func WriteFinding(w io.Writer, f domain.Finding, lastFile string) string {
	return report.WriteFinding(w, f, report.ColorEnabled(w), lastFile)
}

// Rules returns the embedded catalog.
func Rules() ([]RuleInfo, error) {
	specs, err := rules.Load()
	if err != nil {
		return nil, err
	}
	out := make([]RuleInfo, 0, len(specs))
	for _, s := range specs {
		out = append(out, RuleInfo{
			ID:          s.ID,
			Family:      string(s.Family),
			Severity:    string(s.Severity),
			Confidence:  string(s.Confidence),
			Clean:       string(s.Clean),
			Description: s.Description,
		})
	}
	return out, nil
}

// Explain returns one rule spec.
func Explain(id string) (RuleInfo, error) {
	s, ok := rules.Lookup(id)
	if !ok {
		return RuleInfo{}, fmt.Errorf("unknown rule %s", id)
	}
	return RuleInfo{
		ID:          s.ID,
		Family:      string(s.Family),
		Severity:    string(s.Severity),
		Confidence:  string(s.Confidence),
		Clean:       string(s.Clean),
		Description: s.Description,
	}, nil
}

func applyForceKind(u domain.Unit, force string) domain.Unit {
	switch strings.ToLower(strings.TrimSpace(force)) {
	case "text":
		u.Kind = domain.KindText
		if u.Language == domain.LangImage || u.Language == domain.LangContainer {
			u.Language = domain.LangText
		}
	case "image":
		u.Kind = domain.KindImage
		u.Language = domain.LangImage
	case "container":
		u.Kind = domain.KindContainer
		u.Language = domain.LangContainer
	}
	return u
}
