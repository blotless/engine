//go:build !llm

package llm

import (
	"fmt"

	"github.com/blotless/engine/ports"
)

func newNative(cfg Config) (ports.LLM, error) {
	return nil, fmt.Errorf("llm: native adapter requires rebuilding with -tags llm")
}
