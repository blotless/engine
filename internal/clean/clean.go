package clean

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/text/unicode/norm"

	"github.com/blotless/engine/internal/astgo"
	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/classify"
	"github.com/blotless/engine/internal/detect/homoglyph"
	"github.com/blotless/engine/ports"
)

// Planner applies strip/normalize replacements from end to start.
type Planner struct {
	Gofmt bool
	NFKC  bool
}

func (p Planner) CanClean(f domain.Finding) bool {
	switch f.Clean {
	case domain.CleanStrip, domain.CleanNormalize:
		return true
	case domain.CleanRewrite:
		return f.Replacement != ""
	default:
		return false
	}
}

func (p Planner) Plan(u domain.Unit, findings []domain.Finding) (domain.Patch, error) {
	patch := domain.Patch{Path: u.Path, Original: append([]byte(nil), u.Bytes...)}
	findings = fillReplacements(u, findings)
	var writable []domain.Finding
	for _, f := range findings {
		if f.Span.File != u.Path && f.Span.File != "" {
			// still allow if same unit
		}
		if !p.CanClean(f) {
			patch.Skipped = append(patch.Skipped, f)
			continue
		}
		if f.Clean == domain.CleanNormalize && f.Replacement == "" {
			patch.Skipped = append(patch.Skipped, f)
			continue
		}
		if f.Span.Start < 0 || f.Span.End > len(u.Bytes) || f.Span.End <= f.Span.Start {
			patch.Skipped = append(patch.Skipped, f)
			continue
		}
		writable = append(writable, f)
	}
	sort.Slice(writable, func(i, j int) bool {
		if writable[i].Span.Start != writable[j].Span.Start {
			return writable[i].Span.Start < writable[j].Span.Start
		}
		return writable[i].Span.End > writable[j].Span.End
	})
	var kept []domain.Finding
	lastEnd := -1
	for _, f := range writable {
		if f.Span.Start < lastEnd {
			patch.Skipped = append(patch.Skipped, f)
			continue
		}
		kept = append(kept, f)
		lastEnd = f.Span.End
	}
	updated := append([]byte(nil), u.Bytes...)
	for i := len(kept) - 1; i >= 0; i-- {
		f := kept[i]
		updated = concat(updated[:f.Span.Start], []byte(f.Replacement), updated[f.Span.End:])
	}
	if p.NFKC && classify.TextPayload(u) {
		if n := norm.NFKC.Bytes(updated); !bytes.Equal(n, updated) {
			updated = n
		}
	}
	if p.Gofmt && u.Language == domain.LangGo && !bytes.Equal(updated, u.Bytes) {
		formatted, err := format.Source(updated)
		if err == nil {
			updated = formatted
		}
	}
	patch.Updated = updated
	patch.Applied = kept
	return patch, nil
}

func fillReplacements(u domain.Unit, fs []domain.Finding) []domain.Finding {
	taken := map[string]struct{}{}
	mapping := map[string]string{}
	if u.Language == domain.LangGo {
		if f, err := astgo.Parse(u.Path, u.Bytes); err == nil {
			f.WalkIdents(func(id astgo.Ident) {
				taken[id.Name] = struct{}{}
			})
		}
	}
	out := make([]domain.Finding, 0, len(fs))
	for _, finding := range fs {
		orig := finding.Evidence
		if orig == "" {
			orig = finding.Symbol
		}
		switch finding.RuleID {
		case "homoglyph.ident", "homoglyph.yaml_key", "homoglyph.json_key":
			if finding.Replacement == "" && orig != "" && homoglyph.MixedScript(orig) {
				repl, ok := mapping[orig]
				if !ok {
					delete(taken, orig)
					repl = homoglyph.UniqueASCII(orig, taken)
					mapping[orig] = repl
				}
				finding.Replacement = repl
				finding.Clean = domain.CleanNormalize
			}
		}
		out = append(out, finding)
	}
	return out
}

func concat(a, b, c []byte) []byte {
	out := make([]byte, 0, len(a)+len(b)+len(c))
	out = append(out, a...)
	out = append(out, b...)
	out = append(out, c...)
	return out
}

// Apply writes patch.Updated atomically. backupExt may be empty.
func Apply(patch domain.Patch, backupExt string) error {
	if !patch.Changed() {
		return nil
	}
	dir := filepath.Dir(patch.Path)
	if backupExt != "" {
		if err := os.WriteFile(patch.Path+backupExt, patch.Original, 0o644); err != nil {
			return fmt.Errorf("clean: backup %s: %w", patch.Path, err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".blot-*")
	if err != nil {
		return fmt.Errorf("clean: temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(patch.Updated); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("clean: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("clean: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	info, err := os.Stat(patch.Path)
	if err == nil {
		_ = os.Chmod(tmpName, info.Mode())
	}
	if err := os.Rename(tmpName, patch.Path); err != nil {
		return fmt.Errorf("clean: rename: %w", err)
	}
	return nil
}

var _ ports.Cleaner = Planner{}
