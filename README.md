<h1 align="center">blotless/engine</h1>

<p align="center">
  <strong>Detection and cleaning engine for blotless</strong><br>
  Layer A Unicode, Layer B survey, C2PA / file metadata. Zero CGO.
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

**engine** is the library of the [blotless](https://github.com/blotless) ecosystem — walk, classify, detect, merge, score, and plan patches. The CLI in [blotless/cli](https://github.com/blotless/cli) is a thin Cobra wrapper around `engine.Scan` / `engine.Clean`.

Detectors receive a `domain.Unit` (already-loaded bytes). They do not read the filesystem.

### Key Features

| Category | Capabilities |
|----------|--------------|
| **Layer A** | Invisible Unicode, exotic spaces, bidi, tags, VS, other Cf, Latin confusables |
| **Layer B** | Eligibility survey; `engine/rewrite` hook; optional Ollama / openai (`paraphrase` / `humanize` / `code` / `backtranslate` / `structural`) |
| **Files** | C2PA / EXIF / XMP in PNG, JPEG, WebP, SVG, PDF, DOCX, ODT, HTML, Markdown |
| **Web audit** | `engine/webaudit` — SSRF-safe sitemap fetch |
| **Go source** | Homoglyph idents; string / `go:generate` protect spans via `internal/astgo` |
| **Stamps** | AI co-author / generator phrases in comments only |
| **Clean** | Strip / normalize / rewrite; optional NFKC; `gofmt` for Go |
| **Build** | `CGO_ENABLED=0`, Go 1.26+. Pre-release: `task preflight` ([Taskfile.yml](Taskfile.yml)) |

---

## Installation

```bash
go get github.com/blotless/engine
```

**Requirements:** Go 1.26+, `CGO_ENABLED=0`.

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
	})
	if err != nil {
		panic(err)
	}
	defer eng.Close()

	res, err := eng.Scan(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Printf("findings=%d score=%d\n", len(res.Findings), res.Score.Percent)
}
```

---

## Architecture

```
walk → classify → detect → protect → merge → score
                              │
clean ← plan ← optional LLM ← Layer B survey
```

Public surface: `engine.Config`, `engine.Scan`, `engine.Clean`, `engine.Rules`, `domain`, `ports`.

---

## Ecosystem

| Project | Description |
|---------|-------------|
| [blotless/engine](https://github.com/blotless/engine) | **Detection / clean library (this repo)** |
| [blotless/cli](https://github.com/blotless/cli) | Command-line interface |
| [blotless/skills](https://github.com/blotless/skills) | Agent skill package (calls `blotless` on PATH) |

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<p align="center">
  <strong>blotless</strong> — inspect first, then clean
</p>
