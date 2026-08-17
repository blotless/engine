# blotless/engine roadmap

## Current State

Shipped **v0.2.0** (2026-08-17).

Layer A, Files/C2PA, origin heuristics, Layer B survey, AST via ast, optional LLM rewrite.

## Near term

- [ ] Origin false-positive review (e.g. `cursor` in CSS)
- [ ] Policy packs (enable/disable rule sets)
- [ ] Coverage floor for `internal/detect/origin`
- [ ] More container formats

## Out of scope

Pixel/audio SynthID; language parsers (those live in [ast](https://github.com/blotless/ast)).
