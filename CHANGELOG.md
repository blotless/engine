# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-08-17

### Added

- Origin heuristics (`internal/detect/origin`); `Score.Confidence` and `Evidence`.
- Layer B offline AST transform via [`github.com/blotless/ast`](https://github.com/blotless/ast) (`statwm.ast_transform`).
- `Config.AstWASM` / `Config.AstExt` for WASM language plugins.
- Contributor docs: CONTRIBUTING, SECURITY, ROADMAP, docs/ARCHITECTURE.

### Changed

- Go parse helpers moved from `internal/astgo` to `github.com/blotless/ast/golang`.
- deps: golang.org/x/text v0.21.0 → v0.41.0; wazero v1.9.0 → v1.12.0 (via ast).

## [0.1.0] - 2026-08-14

### Added

- Layer A Unicode / stamps; Files C2PA/metadata; Layer B survey + LLM rewrite hooks.
