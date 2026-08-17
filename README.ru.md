<h1 align="center">blotless/engine</h1>

<p align="center">
  <strong>Движок detect / clean</strong><br>
  Layer A · Layer B (AST / LLM) · C2PA · origin · Zero CGO
</p>

<p align="center">
  <b>Язык:</b> <a href="README.md">English</a> | Русский
</p>

---

Отдельный репозиторий. CLI — [blotless/cli](https://github.com/blotless/cli), трансформы — [blotless/ast](https://github.com/blotless/ast).

```bash
go get github.com/blotless/engine
```

```go
eng, _ := engine.New(engine.Config{
    Paths:   []string{"."},
    LayerB:  true,
    AstWASM: map[string]string{"rust": "./blotless_rust_transform.wasm"},
})
res, _ := eng.Scan(context.Background())
```

WASM-модуль: [ast README](https://github.com/blotless/ast/blob/main/README.ru.md), пример [wasm-rust](https://github.com/blotless/ast/tree/main/examples/wasm-rust).

Документы: [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), [ROADMAP.md](ROADMAP.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).  
Полное описание — в [README.md](README.md).
