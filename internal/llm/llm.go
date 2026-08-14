package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blotless/engine/ports"
)

// DefaultOllamaModel is pulled when --llm-model is empty.
const DefaultOllamaModel = "qwen2.5-coder:3b"

// Config selects an LLM adapter.
type Config struct {
	Mode        string
	Model       string
	Endpoint    string
	Timeout     time.Duration
	PullTimeout time.Duration
	APIKey      string
	Logger      *slog.Logger
}

// New returns noop, ollama, or native depending on mode.
func New(cfg Config) (ports.LLM, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" || mode == "off" || mode == "noop" {
		return Noop{}, nil
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.PullTimeout <= 0 {
		cfg.PullTimeout = 10 * time.Minute
	}
	switch mode {
	case "ollama":
		return newOllama(cfg)
	case "native":
		return newNative(cfg)
	default:
		return nil, fmt.Errorf("llm: unknown mode %q", cfg.Mode)
	}
}

// Noop never classifies or rewrites.
type Noop struct{}

func (Noop) Name() string { return "noop" }

func (Noop) Classify(context.Context, ports.ClassifyIn) (ports.ClassifyOut, error) {
	return ports.ClassifyOut{}, nil
}

func (Noop) Rewrite(context.Context, ports.RewriteIn) (ports.RewriteOut, error) {
	return ports.RewriteOut{}, nil
}

func (Noop) Close() error { return nil }

var _ ports.LLM = Noop{}
