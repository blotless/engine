package classify

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/blotless/engine/domain"
)

const sniffSize = 8192

var (
	pngSig  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	jpegSOI = []byte{0xff, 0xd8}
	pdfSig  = []byte("%PDF")
	riff    = []byte("RIFF")
	webp    = []byte("WEBP")
	zipSig  = []byte("PK")
)

// LanguageFromPath maps a file extension to a language.
func LanguageFromPath(path string) domain.Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return domain.LangGo
	case ".yaml", ".yml":
		return domain.LangYAML
	case ".json":
		return domain.LangJSON
	case ".md", ".markdown", ".mdx":
		return domain.LangMarkdown
	case ".html", ".htm", ".xml", ".svg":
		return domain.LangHTML
	case ".png", ".jpg", ".jpeg", ".webp":
		return domain.LangImage
	case ".pdf", ".docx", ".odt":
		return domain.LangContainer
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx",
		".c", ".h", ".cc", ".cpp", ".cxx", ".hpp",
		".cs", ".java", ".kt", ".kts", ".scala", ".swift",
		".rs", ".php", ".css", ".scss":
		return domain.LangText
	case ".py", ".rb", ".pl", ".sh", ".bash", ".zsh",
		".toml", ".ini", ".cfg", ".conf", ".tf", ".lua", ".sql", ".r":
		return domain.LangText
	case ".txt":
		return domain.LangText
	default:
		return domain.LangText
	}
}

// Identifies if buf appears to be a binary file (indicating a NUL byte in the display window).
func IsBinary(buf []byte) bool {
	n := len(buf)
	if n > sniffSize {
		n = sniffSize
	}
	return bytes.IndexByte(buf[:n], 0) >= 0
}

// KindOf classifies a file for the three-layer pipeline.
// skip is true for unknown binaries that must not be decoded as text.
func KindOf(path string, raw []byte) (kind domain.Kind, skip bool) {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case isImageExt(ext) || isImageMagic(raw):
		return domain.KindImage, false
	case isBinContainerExt(ext) || isBinContainerMagic(raw):
		return domain.KindContainer, false
	case IsBinary(raw):
		return "", true
	default:
		return domain.KindText, false
	}
}

func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

func isBinContainerExt(ext string) bool {
	switch ext {
	case ".pdf", ".docx", ".odt":
		return true
	default:
		return false
	}
}

func isImageMagic(raw []byte) bool {
	if bytes.HasPrefix(raw, pngSig) || bytes.HasPrefix(raw, jpegSOI) {
		return true
	}
	return len(raw) >= 12 && bytes.Equal(raw[:4], riff) && bytes.Equal(raw[8:12], webp)
}

func isBinContainerMagic(raw []byte) bool {
	if bytes.HasPrefix(raw, pdfSig) {
		return true
	}
	if !bytes.HasPrefix(raw, zipSig) {
		return false
	}
	// Cheap sniff: OOXML / ODF names appear as stored paths in the local headers.
	head := raw
	if len(head) > 64<<10 {
		head = head[:64<<10]
	}
	return bytes.Contains(head, []byte("word/document.xml")) ||
		bytes.Contains(head, []byte("content.xml")) && bytes.Contains(head, []byte("meta.xml"))
}

// TextPayload reports whether Unicode / stamp / text-C2PA detectors should run.
func TextPayload(u domain.Unit) bool {
	if u.Kind == domain.KindImage || u.Kind == domain.KindContainer {
		return false
	}
	return u.Language != domain.LangImage && u.Language != domain.LangContainer
}

// Decode inspects BOM and charset. Bytes are kept as-is (UTF-8 assumed).
func Decode(path, rel string, raw []byte) domain.Unit {
	kind, _ := KindOf(path, raw)
	lang := LanguageFromPath(path)
	switch kind {
	case domain.KindImage:
		lang = domain.LangImage
	case domain.KindContainer:
		lang = domain.LangContainer
	}
	u := domain.Unit{
		Path:     path,
		RelPath:  rel,
		Bytes:    raw,
		Charset:  "utf-8",
		Language: lang,
		Kind:     kind,
	}
	if u.Kind == "" {
		u.Kind = domain.KindText
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		u.HasBOM = true
	}
	if kind == domain.KindText && !utf8.Valid(raw) {
		u.Charset = "unknown"
	}
	return u
}
