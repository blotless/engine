# Architecture — blotless/engine

This repo is the **detect / clean library**. CLI and AST are separate:

```
cli  →  engine  →  ast
```

## Pipeline

```
walk → classify → detect → protect → merge → score
clean ← plan ← ast.Transform / optional LLM ← layerb.Plan
```

| Package | Role |
|---------|------|
| `internal/walk` | Enumerate files |
| `internal/classify` | Kind / language / decode |
| `internal/detect/*` | Unicode, stamp, origin, c2pa, container, Go AST homoglyphs |
| `internal/layerb` | Eligibility + `statwm.ast_transform` |
| `internal/score` | Percent, agent, confidence, evidence |
| `internal/clean` | Strip/normalize planner |

Detectors take `domain.Unit` bytes only — **no FS**.

## Config hooks for WASM

```go
Config{
    LayerB:  true,
    AstWASM: map[string]string{"rust": "./plugin.wasm"},
    AstExt:  map[string]string{".zig": "zig"},
}
```

`New` calls `ast.RegisterWASM` / `ast.MapExt`. Plugin contract: [ast ABI](https://github.com/blotless/ast/blob/main/ABI.md).

## Related

- [README.md](../README.md)
- [CONTRIBUTING.md](../CONTRIBUTING.md)
- [ROADMAP.md](../ROADMAP.md)
