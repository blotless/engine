<h1 align="center">blotless/engine</h1>

<p align="center">
  <strong>Движок детекции и очистки blotless</strong><br>
  Layer A Unicode, Layer B survey, C2PA / метаданные. Zero CGO.
</p>

<p align="center">
  <b>Язык:</b> <a href="README.md">English</a> | Русский
</p>

<p align="center">
  <a href="https://github.com/blotless/engine/actions/workflows/ci.yml"><img src="https://github.com/blotless/engine/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/blotless/engine"><img src="https://pkg.go.dev/badge/github.com/blotless/engine.svg" alt="Go Reference"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/blotless/engine"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version"></a>
</p>

---

## Обзор

**engine** — библиотека экосистемы [blotless](https://github.com/blotless): walk → classify → detect → merge → score → plan. CLI ([blotless/cli](https://github.com/blotless/cli)) — тонкая обёртка над `engine.Scan` / `engine.Clean`.

Детекторы получают `domain.Unit` (байты уже загружены) и **не** читают ФС сами.

### Возможности

| Категория | Что умеет |
|----------|-----------|
| **Layer A** | Невидимый Unicode, exotic spaces, bidi, tags, VS, Cf, confusables |
| **Layer B** | Survey eligibility; `rewrite` hook; ollama / openai |
| **Files** | C2PA / EXIF / XMP: PNG, JPEG, WebP, SVG, PDF, DOCX, ODT, HTML, MD |
| **Web** | `webaudit` — SSRF-safe sitemap |
| **Go** | Homoglyph idents; protect строк / `go:generate` (`internal/astgo`) |
| **Сборка** | `CGO_ENABLED=0`, Go 1.26+ |

---

## Установка

```bash
go get github.com/blotless/engine
```

**Требования:** Go 1.26+, `CGO_ENABLED=0`.

---

## Быстрый старт

```go
eng, err := engine.New(engine.Config{Paths: []string{"."}, Aggressive: true})
if err != nil { panic(err) }
defer eng.Close()
res, err := eng.Scan(context.Background())
```

Публичный API: `Config`, `Scan`, `Clean`, `Rules`, `domain`, `ports`.

---

## Экосистема

| Проект | Описание |
|--------|----------|
| [blotless/engine](https://github.com/blotless/engine) | **Библиотека (этот репозиторий)** |
| [blotless/cli](https://github.com/blotless/cli) | CLI |
| [blotless/skills](https://github.com/blotless/skills) | Скилл агента |

---

## Лицензия

MIT — см. [LICENSE](LICENSE).
