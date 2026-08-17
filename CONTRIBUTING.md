# Contributing to blotless/engine

Thank you for contributing to the detection/clean library.

---

## Requirements

- **Go 1.26+**, **`CGO_ENABLED=0`**, **`GOTOOLCHAIN=local`**
- Depends on [blotless/ast](https://github.com/blotless/ast)

```bash
git clone https://github.com/blotless/engine
cd engine
export CGO_ENABLED=0 GOTOOLCHAIN=local
go test ./...
task preflight
```

---

## Workflow

```bash
git checkout -b feat/your-feature
gofmt -w .
go test ./...
git commit -m "feat(origin): add copilot comment cue"
```

PR: https://github.com/blotless/engine/compare

### Checklist

- [ ] Tests for new detectors/rules
- [ ] Detectors must **not** read the filesystem (`domain.Unit` only)
- [ ] YAML rule ids unique under `internal/rules/v1/`
- [ ] No `Co-authored-by:` trailers

Scopes: `unicode`, `stamp`, `origin`, `c2pa`, `layerb`, `score`, `clean`, `llm`.

**Author:** `lkmavi <zikmanv@icloud.com>` unless agreed.

Do not add heavy ML adapters or raw exec plugins. WASM belongs in **ast**.

---

## Structure

```
engine/
├── engine.go           # Scan / Clean / Config
├── domain/             # Finding, Score, Unit
├── internal/detect/    # unicode, stamp, origin, c2pa, …
├── internal/layerb/
└── internal/rules/v1/
```

---

[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) · [LICENSE](LICENSE)
