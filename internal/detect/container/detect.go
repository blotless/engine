package container

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
	"github.com/blotless/engine/ports"
)

// Detector strips C2PA / EXIF / XMP / document provenance (Files layer).
type Detector struct{}

func (Detector) ID() string { return "container" }

func (Detector) Families() []domain.Family {
	return []domain.Family{domain.FamilyC2PA, domain.FamilyMetadata}
}

func (Detector) Scan(ctx context.Context, u domain.Unit) ([]domain.Finding, error) {
	switch formatOf(u) {
	case "png":
		return scanPNG(u)
	case "jpeg":
		return scanJPEG(u)
	case "webp":
		return scanWebP(u)
	case "html":
		return scanHTML(u)
	case "markdown":
		return scanMarkdown(u)
	case "svg":
		return scanSVG(u)
	case "pdf":
		return scanPDF(ctx, u)
	case "docx":
		return scanDOCX(u)
	case "odt":
		return scanODT(u)
	default:
		return nil, nil
	}
}

func formatOf(u domain.Unit) string {
	ext := strings.ToLower(filepath.Ext(u.Path))
	raw := u.Bytes
	switch {
	case ext == ".png" || bytes.HasPrefix(raw, pngSig):
		return "png"
	case ext == ".jpg" || ext == ".jpeg" || bytes.HasPrefix(raw, jpegSOI):
		return "jpeg"
	case ext == ".webp" || isWebP(raw):
		return "webp"
	case ext == ".svg":
		return "svg"
	case ext == ".html" || ext == ".htm":
		return "html"
	case ext == ".md" || ext == ".markdown" || ext == ".mdx":
		return "markdown"
	case ext == ".pdf" || bytes.HasPrefix(raw, []byte("%PDF")):
		return "pdf"
	case ext == ".docx":
		return "docx"
	case ext == ".odt":
		return "odt"
	}
	if bytes.HasPrefix(raw, []byte("PK")) {
		if bytes.Contains(raw[:min(len(raw), 64<<10)], []byte("word/document.xml")) {
			return "docx"
		}
		if bytes.Contains(raw[:min(len(raw), 64<<10)], []byte("content.xml")) {
			return "odt"
		}
	}
	if len(raw) > 0 && (raw[0] == '<' || bytes.HasPrefix(bytes.TrimLeft(raw, " \t\r\n"), []byte("<"))) {
		head := bytes.ToLower(raw[:min(len(raw), 512)])
		if bytes.Contains(head, []byte("<svg")) {
			return "svg"
		}
	}
	return ""
}

func applyWhole(u domain.Unit, id, evidence string, cleaned []byte) []domain.Finding {
	if bytes.Equal(cleaned, u.Bytes) {
		return nil
	}
	f := rules.Apply(wholeFile(u, id, evidence, string(cleaned)), id)
	f.Replacement = string(cleaned)
	return []domain.Finding{f}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ ports.Detector = Detector{}
