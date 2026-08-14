package rules

import (
	"fmt"
	"os"
	"sync"

	"github.com/blotless/engine/domain"
	"gopkg.in/yaml.v3"
)

// Spec is a versioned rule definition.
type Spec struct {
	ID          string               `yaml:"id" json:"id"`
	Family      domain.Family        `yaml:"family" json:"family"`
	Severity    domain.Severity      `yaml:"severity" json:"severity"`
	Confidence  domain.Confidence    `yaml:"confidence" json:"confidence"`
	Clean       domain.CleanStrategy `yaml:"clean" json:"clean"`
	Description string               `yaml:"description" json:"description"`
}

type fileDoc struct {
	Rules []Spec `yaml:"rules"`
}

var (
	once    sync.Once
	catalog []Spec
	byID    map[string]Spec
	loadErr error
)

// Load reads the embedded v1 pack. Safe to call many times.
func Load() ([]Spec, error) {
	once.Do(func() {
		byID = map[string]Spec{}
		entries, err := packFS.ReadDir("v1")
		if err != nil {
			loadErr = fmt.Errorf("rules: read pack: %w", err)
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			b, err := packFS.ReadFile("v1/" + e.Name())
			if err != nil {
				loadErr = fmt.Errorf("rules: read %s: %w", e.Name(), err)
				return
			}
			var doc fileDoc
			if err := yaml.Unmarshal(b, &doc); err != nil {
				loadErr = fmt.Errorf("rules: parse %s: %w", e.Name(), err)
				return
			}
			for _, s := range doc.Rules {
				if s.ID == "" {
					loadErr = fmt.Errorf("rules: empty id in %s", e.Name())
					return
				}
				if _, ok := byID[s.ID]; ok {
					loadErr = fmt.Errorf("rules: duplicate id %s", s.ID)
					return
				}
				byID[s.ID] = s
				catalog = append(catalog, s)
			}
		}
	})
	return catalog, loadErr
}

// Lookup returns a spec by id.
func Lookup(id string) (Spec, bool) {
	if _, err := Load(); err != nil {
		return Spec{}, false
	}
	s, ok := byID[id]
	return s, ok
}

// MustLookup returns a spec or prints to stderr and returns a fallback.
func MustLookup(id string) Spec {
	s, ok := Lookup(id)
	if !ok {
		fmt.Fprintf(os.Stderr, "blotless: unknown rule id %s\n", id)
		return Spec{
			ID:         id,
			Family:     domain.FamilyUnicode,
			Severity:   domain.SeverityWarning,
			Confidence: domain.ConfidenceCertain,
			Clean:      domain.CleanStrip,
		}
	}
	return s
}

// Apply copies spec fields onto a finding.
func Apply(f domain.Finding, id string) domain.Finding {
	s := MustLookup(id)
	f.RuleID = s.ID
	f.Family = s.Family
	f.Severity = s.Severity
	f.Confidence = s.Confidence
	if f.Clean == "" {
		f.Clean = s.Clean
	}
	if f.Message == "" {
		f.Message = s.Description
	}
	return f
}
