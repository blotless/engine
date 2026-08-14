package container

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

var (
	xmpPacketRe = regexp.MustCompile(`(?is)<\?xpacket begin.*?<\?xpacket end[^?]*\?>`)
	pdfInfoRe   = regexp.MustCompile(`(?i)/(Producer|Creator)\s*\((?:\\.|[^\\)])*\)`)
	pdfStreamRe = regexp.MustCompile(`(?s)stream\r?\n.*?endstream`)
)

func scanPDF(ctx context.Context, u domain.Unit) ([]domain.Finding, error) {
	cleaned, labels, changed := stripPDF(ctx, u.Bytes)
	if !changed {
		return nil, nil
	}
	f := rules.Apply(wholeFile(u, "container.pdf_metadata", labels, string(cleaned)), "container.pdf_metadata")
	f.Replacement = string(cleaned)
	return []domain.Finding{f}, nil
}

func stripPDF(ctx context.Context, data []byte) (out []byte, labels string, changed bool) {
	var actions []string
	out = append([]byte(nil), data...)
	out = xmpPacketRe.ReplaceAllFunc(out, func(pkt []byte) []byte {
		if !looksAI(pkt) {
			return pkt
		}
		actions = append(actions, "xmp")
		return bytes.Repeat([]byte(" "), len(pkt))
	})
	out = pdfInfoRe.ReplaceAllFunc(out, func(m []byte) []byte {
		if !generatorAI.Match(m) && !aiMetaName.Match(m) {
			return m
		}
		actions = append(actions, "info")
		return bytes.Repeat([]byte(" "), len(m))
	})
	if len(actions) == 0 && !pdfStructuredAI(data) {
		return data, "", false
	}
	if len(actions) == 0 {
		return data, "", false
	}
	out = maybeQPDF(ctx, out, &actions)
	return out, "PDF " + join(uniq(actions)), true
}

func pdfStructuredAI(data []byte) bool {
	noStreams := pdfStreamRe.ReplaceAll(data, []byte("stream endstream"))
	xmp := bytes.Join(xmpPacketRe.FindAll(data, -1), []byte("\n"))
	return looksAI(noStreams) || looksAI(xmp)
}

func maybeQPDF(ctx context.Context, data []byte, actions *[]string) []byte {
	qpdf, err := exec.LookPath("qpdf")
	if err != nil {
		*actions = append(*actions, "no-qpdf")
		return data
	}
	dir, err := os.MkdirTemp("", "blot-pdf-*")
	if err != nil {
		return data
	}
	defer func() { _ = os.RemoveAll(dir) }()
	in := filepath.Join(dir, "in.pdf")
	outp := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return data
	}
	cmd := exec.CommandContext(ctx, qpdf, "--linearize", in, outp)
	if err := cmd.Run(); err != nil {
		*actions = append(*actions, "qpdf-failed")
		return data
	}
	b, err := os.ReadFile(outp)
	if err != nil || len(b) == 0 {
		return data
	}
	*actions = append(*actions, "qpdf")
	return b
}

func uniq(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
