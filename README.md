<p align="center">
  <strong>Detection and cleaning engine for blotless</strong><br>
  Layer A Unicode · Layer B survey + AST/LLM · C2PA / files · origin heuristics<br>
  Zero CGO
</p>

<h1 align="center">blotless/engine</h1>

<p align="center">
  Walk, classify, detect, merge, score, plan patches — embeddable library
</p>

<p align="center">
  <b>Language:</b> English | <a href="README.ru.md">Русский</a>
</p>

<p align="center">
  <a href="https://github.com/blotless/engine/actions/workflows/ci.yml"><img src="https://github.com/blotless/engine/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/blotless/engine"><img src="https://pkg.go.dev/badge/github.com/blotless/engine.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/blotless/engine"><img src="https://goreportcard.com/badge/github.com/blotless/engine" alt="Go Report Card"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/blotless/engine/releases"><img src="https://img.shields.io/github/v/release/blotless/engine" alt="Latest Release"></a>
  <a href="https://github.com/blotless/engine"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version"></a>
</p>

---

## Overview

**engine** is the library of the [blotless](https://github.com/blotless) ecosystem. The CLI in [blotless/cli](https://github.com/blotless/cli) is a thin Cobra wrapper around `engine.Scan` / `engine.Clean`. Language transforms use [blotless/ast](https://github.com/blotless/ast). These are **separate GitHub repositories**.

Detectors receive a `domain.Unit` (already-loaded bytes). They do not read the filesystem.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Layer A** | Invisible Unicode, exotic spaces, bidi, tags, VS, other Cf, Latin confusables |
| **Origin** | Soft fingerprints → `Score.Agent` / `Confidence` / `Evidence` (not SynthID verify) |
| **Layer B** | Eligibility survey; `ast.Transform`; optional Ollama / openai rewrite |
| **Files** | C2PA / EXIF / XMP in PNG, JPEG, WebP, SVG, PDF, DOCX, ODT, HTML, Markdown |
| **Web audit** | `engine/webaudit` — SSRF-safe sitemap fetch |
| **Go source** | Homoglyph idents; protect spans via `blotless/ast/golang` |
| **Stamps** | AI co-author / generator phrases in comments |
| **Clean** | Strip / normalize / rewrite; optional NFKC; `gofmt` for Go |
| **Build** | `CGO_ENABLED=0`, Go 1.26+; `task preflight` |

---

## Installation

```bash
go get github.com/blotless/engine
```

**Requirements:** Go 1.26+, `CGO_ENABLED=0`. Depends on [`github.com/blotless/ast`](https://github.com/blotless/ast).

---

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	"github.com/blotless/engine"
)

func main() {
	eng, err := engine.New(engine.Config{
		Paths:      []string{"."},
		Aggressive: true,
		LayerB:     true,
	})
	if err != nil {
		panic(err)
	}
	defer eng.Close()

	res, err := eng.Scan(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Printf("findings=%d score=%d%% agent=%s conf=%s\n",
		len(res.Findings), res.Score.Percent, res.Score.Agent, res.Score.Confidence)
}
```

### Clean with a WASM language plugin

Plugin tutorial: [blotless/ast — WASM](https://github.com/blotless/ast#add-a-language-via-wasm-no-go-required). Example crate: [examples/wasm-rust](https://github.com/blotless/ast/tree/main/examples/wasm-rust).

```go
eng, err := engine.New(engine.Config{
    Paths:   []string{"."},
    Write:   true,
    LayerB:  true,
    AstWASM: map[string]string{
        "rust": "./blotless_rust_transform.wasm", // also maps .rs
    },
    // AstExt: map[string]string{".zig": "zig"},
})
if err != nil {
    panic(err)
}
defer eng.Close()
_, err = eng.Clean(context.Background())
```

Equivalent CLI: `blotless clean . --write --layer-b --ast-wasm rust=./blotless_rust_transform.wasm`

---

## Architecture

```
walk → classify → detect → protect → merge → score
                              │
clean ← plan ← ast.Transform / optional LLM ← Layer B survey
```

Public surface: `engine.Config`, `engine.Scan`, `engine.Clean`, `engine.Rules`, `domain`, `ports`.

Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Ecosystem

| Project | Description |
|---------|-------------|
| [blotless/engine](https://github.com/blotless/engine) | **This repo** |
| [blotless/ast](https://github.com/blotless/ast) | AST transform + WASM |
| [blotless/cli](https://github.com/blotless/cli) | CLI |
| [blotless/skills](https://github.com/blotless/skills) | Agent skills |

---

## Development

```bash
git clone https://github.com/blotless/engine
cd engine
CGO_ENABLED=0 go test ./...
task preflight
```

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- [SECURITY.md](SECURITY.md)
- [ROADMAP.md](ROADMAP.md)

---

## Disclaimer

Layer B is best-effort. Origin scores are heuristic. Do not claim certified human authorship.

---

## License

MIT License — see [LICENSE](LICENSE).

---

<p align="center"><strong>blotless</strong> — inspect first, then clean</p>
