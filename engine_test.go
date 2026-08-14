package engine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blotless/engine"
	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/clean"
	unicodedet "github.com/blotless/engine/internal/detect/unicode"
	"github.com/blotless/engine/internal/rules"
)

func TestScanAndCleanUnicode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	original := []byte("hello\u200bworld\u00a0x")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := engine.Config{Paths: []string{dir}, FailOn: domain.ConfidenceCertain}
	eng, err := engine.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) < 2 {
		t.Fatalf("findings=%d", len(res.Findings))
	}
	if !res.ShouldFail(domain.ConfidenceCertain) {
		t.Fatal("expected fail-on certain")
	}

	eng2, err := engine.New(engine.Config{Paths: []string{dir}, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	cleaned, err := eng2.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cleaned.Patches) == 0 {
		t.Fatal("expected patches")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("helloworld x")) {
		t.Fatalf("cleaned=%q", got)
	}

	eng3, err := engine.New(engine.Config{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	again, err := eng3.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Findings) != 0 {
		t.Fatalf("not idempotent: %#v", again.Findings)
	}
}

func TestCleanRescoresAfter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	src := []byte("hello\u200bworld\n// Co-authored-by: Cursor <cursor@example.com>\n")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Score.Percent == 0 {
		t.Fatal("expected before score")
	}
	if res.Score.Agent != "cursor" {
		t.Fatalf("agent=%s", res.Score.Agent)
	}
	if res.After == nil {
		t.Fatal("expected after score")
	}
	if res.After.Percent >= res.Score.Percent {
		t.Fatalf("after=%d before=%d", res.After.Percent, res.Score.Percent)
	}
	if res.After.Percent != 0 {
		t.Fatalf("after percent=%d findings remain", res.After.Percent)
	}
}

func TestCleanRenamesGoHomoglyph(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	src := []byte("package p\n\nfunc sc\u0430n() {}\n")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{path}, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("func scan()")) {
		t.Fatalf("rename failed: %q patches=%d findings=%d", got, len(res.Patches), len(res.Findings))
	}
	if bytes.Contains(got, []byte("\u0430")) {
		t.Fatalf("cyrillic remains: %q", got)
	}
}

func TestCleanIdempotentProperty(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := []byte("ab\u200bcd\u00a0ef\ufeff")
	u := domain.Unit{Path: "x.txt", Bytes: src, Language: domain.LangText}
	fs, err := unicodedet.Detector{}.Scan(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	p1, err := clean.Planner{}.Plan(u, fs)
	if err != nil {
		t.Fatal(err)
	}
	u2 := u
	u2.Bytes = p1.Updated
	fs2, err := unicodedet.Detector{}.Scan(context.Background(), u2)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := clean.Planner{}.Plan(u2, fs2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p1.Updated, p2.Updated) {
		t.Fatalf("not idempotent %q vs %q", p1.Updated, p2.Updated)
	}
}

func TestProtectGoString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	src := "package main\nvar s = \"hi\u200b\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{path}, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Contains(got, []byte(`\u200b`)) && !bytes.Contains(got, []byte("\u200b")) {
		t.Fatalf("string content was stripped: %q findings=%v", got, res.Findings)
	}
	stripped := false
	for _, f := range res.Findings {
		if f.RuleID == "unicode.zwsp" && f.Clean == domain.CleanStrip && !f.Protect {
			stripped = true
		}
	}
	if stripped {
		t.Fatal("zwsp inside string should be protected")
	}
}

func TestJSONReport(t *testing.T) {
	var buf bytes.Buffer
	err := engine.Report(&buf, "json", &engine.Result{FilesScanned: 1}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "files_scanned") {
		t.Fatalf("%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"percent"`) {
		t.Fatalf("%s", buf.String())
	}
}

func TestRulesCatalog(t *testing.T) {
	list, err := engine.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 10 {
		t.Fatalf("rules=%d", len(list))
	}
	if _, err := engine.Explain("unicode.zwsp"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Explain("container.png_metadata"); err != nil {
		t.Fatal(err)
	}
}

func TestExcludeSkipsTestStamps(t *testing.T) {
	dir := t.TempDir()
	src := []byte("package p\n// Co-authored-by: Cursor <cursor@example.com>\n")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}, Exclude: []string{"*_test.go"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("excluded tests still flagged: %#v", res.Findings)
	}
}

func TestScanAndCleanPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mark.png")
	data := testPNGWithAIText()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesScanned != 1 {
		t.Fatalf("scanned=%d skipped=%d", res.FilesScanned, res.FilesSkipped)
	}
	if res.ImageFiles != 1 || res.TextFiles != 0 {
		t.Fatalf("coverage text=%d images=%d", res.TextFiles, res.ImageFiles)
	}
	if res.DetectorHits["container"] < 1 {
		t.Fatalf("hits=%v", res.DetectorHits)
	}
	found := false
	for _, f := range res.Findings {
		if f.RuleID == "container.png_metadata" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing png finding: %#v", res.Findings)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("Claude")) {
		t.Fatal("claude remains")
	}
	if res.After == nil || res.After.Percent != 0 {
		t.Fatalf("after=%v", res.After)
	}
}

func TestScanLooksAtGoAndImages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("hello\u200bworld"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mark.png"), testPNGWithAIText(), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.TextFiles != 2 || res.ImageFiles != 1 {
		t.Fatalf("text=%d images=%d scanned=%d", res.TextFiles, res.ImageFiles, res.FilesScanned)
	}
	if res.DetectorHits["unicode"] < 1 {
		t.Fatalf("unicode not run on text: %v", res.DetectorHits)
	}
	if res.DetectorHits["container"] < 1 {
		t.Fatalf("container not run on png: %v", res.DetectorHits)
	}
	var goHit, pngHit, txtHit bool
	for _, f := range res.Findings {
		switch filepath.Base(f.Span.File) {
		case "a.go":
			goHit = true
		case "mark.png":
			pngHit = f.RuleID == "container.png_metadata"
			if strings.Contains(f.Snippet, "<0x89>PNG") {
				t.Fatalf("binary snippet: %q", f.Snippet)
			}
			if !strings.Contains(strings.ToLower(f.Evidence+f.Snippet), "claude") {
				t.Fatalf("expected claude in evidence %q snippet %q", f.Evidence, f.Snippet)
			}
		case "note.txt":
			txtHit = f.RuleID == "unicode.zwsp"
		}
	}
	if goHit {
		t.Fatal("clean Go file should not be a finding")
	}
	if !pngHit || !txtHit {
		t.Fatalf("png=%v txt=%v findings=%v", pngHit, txtHit, res.Findings)
	}
}

func TestScanAndCleanHTMLMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.html")
	src := []byte(`<html><head><meta NAME="GENERATOR" CONTENT="WordPress 6.4"><meta name="generator" content="Claude"></head><body>ok</body></html>`)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("WordPress")) {
		t.Fatalf("cms generator stripped: %s", got)
	}
	if bytes.Contains(got, []byte("Claude")) {
		t.Fatalf("claude remains: %s", got)
	}
	if len(res.Patches) == 0 {
		t.Fatal("expected patch")
	}
}

func TestSkipUnknownBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), []byte("a\x00b\x00c"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesScanned != 1 || res.FilesSkipped != 1 {
		t.Fatalf("scanned=%d skipped=%d", res.FilesScanned, res.FilesSkipped)
	}
}

func testPNGWithAIText() []byte {
	ihdr := make([]byte, 13)
	ihdr[8], ihdr[9] = 8, 2
	ihdr[3], ihdr[7] = 1, 1
	out := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	out = append(out, testPNGChunk("IHDR", ihdr)...)
	out = append(out, testPNGChunk("tEXt", []byte("Comment\x00Generated by Claude"))...)
	out = append(out, testPNGChunk("IEND", nil)...)
	return out
}

func testPNGChunk(typ string, payload []byte) []byte {
	var hdr [8]byte
	hdr[0] = byte(len(payload) >> 24)
	hdr[1] = byte(len(payload) >> 16)
	hdr[2] = byte(len(payload) >> 8)
	hdr[3] = byte(len(payload))
	copy(hdr[4:8], typ)
	crc := crc32.NewIEEE()
	_, _ = crc.Write([]byte(typ))
	_, _ = crc.Write(payload)
	sum := crc.Sum32()
	var c [4]byte
	c[0] = byte(sum >> 24)
	c[1] = byte(sum >> 16)
	c[2] = byte(sum >> 8)
	c[3] = byte(sum)
	out := append(hdr[:], payload...)
	return append(out, c[:]...)
}

func TestEngineClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": "qwen2.5-coder:3b"}},
			})
		}
	}))
	t.Cleanup(srv.Close)

	eng, err := engine.New(engine.Config{
		Paths: []string{t.TempDir()},
		LLM:   engine.LLMConfig{Mode: "ollama", Endpoint: srv.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFinding(t *testing.T) {
	var buf bytes.Buffer
	f := domain.Finding{
		RuleID:  "unicode.zwsp",
		Message: "Zero-width space",
		Snippet: "a<U+200B>b",
		Span:    domain.Span{File: "a.txt", Line: 1, Col: 1},
	}
	last := engine.WriteFinding(&buf, f, "")
	if last != "a.txt" || !strings.Contains(buf.String(), "unicode.zwsp") {
		t.Fatalf("last=%q buf=%q", last, buf.String())
	}
}

func TestExplainUnknown(t *testing.T) {
	if _, err := engine.Explain("no.such.rule"); err == nil {
		t.Fatal("expected error")
	}
}

func TestShouldFailHeuristic(t *testing.T) {
	res := engine.Result{Findings: []domain.Finding{{
		Confidence: domain.ConfidenceHeuristic,
	}}}
	if res.ShouldFail(domain.ConfidenceLikely) {
		t.Fatal("heuristic should not fail on likely")
	}
	if !res.ShouldFail(domain.ConfidenceHeuristic) {
		t.Fatal("heuristic should fail on heuristic")
	}
}

func TestForceKindScan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "img.png")
	if err := os.WriteFile(path, testPNGWithAIText(), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}, ForceKind: "text", ForceText: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.TextFiles != 1 || res.ImageFiles != 0 {
		t.Fatalf("text=%d images=%d", res.TextFiles, res.ImageFiles)
	}
}

func TestCleanWithLLMRewrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": "qwen2.5-coder:3b"}},
			})
		case "/api/generate":
			_, _ = io.WriteString(w, `{"response":"rewritten prose without ai markers"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	prose := strings.Repeat("This paragraph mentions AI assistance and needs rewriting. ", 8)
	src := prose + "\n// Co-authored-by: Cursor <cursor@example.com>\n"
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{
		Paths:  []string{dir},
		LLM:    engine.LLMConfig{Mode: "ollama", Endpoint: srv.URL},
		LayerB: true,
		Write:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Patches) == 0 {
		t.Fatalf("findings=%d patches=%d", len(res.Findings), len(res.Patches))
	}
}

func TestForceKindImageAndContainer(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "img.png")
	pdfPath := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(pngPath, testPNGWithAIText(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4\n1 0 obj\n<< /Producer (Claude) >>\nendobj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	engImg, err := engine.New(engine.Config{Paths: []string{pngPath}, ForceKind: "image"})
	if err != nil {
		t.Fatal(err)
	}
	resImg, err := engImg.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resImg.ImageFiles != 1 {
		t.Fatalf("images=%d", resImg.ImageFiles)
	}
	engDoc, err := engine.New(engine.Config{Paths: []string{pdfPath}, ForceKind: "container"})
	if err != nil {
		t.Fatal(err)
	}
	resDoc, err := engDoc.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resDoc.DocFiles != 1 {
		t.Fatalf("docs=%d", resDoc.DocFiles)
	}
}

func TestCleanDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	orig := []byte("hello\u200b")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{Paths: []string{dir}, Write: false})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || len(res.Patches) == 0 {
		t.Fatalf("dry=%v patches=%d", res.DryRun, len(res.Patches))
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, orig) {
		t.Fatalf("dry-run modified file: orig=%q got=%q", orig, got)
	}
}

func TestNewNativeUnavailable(t *testing.T) {
	_, err := engine.New(engine.Config{
		Paths: []string{t.TempDir()},
		LLM:   engine.LLMConfig{Mode: "native"},
	})
	if err == nil || !strings.Contains(err.Error(), "native") {
		t.Fatalf("%v", err)
	}
}

func TestCleanLLMRewriteFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": "qwen2.5-coder:3b"}},
			})
		case "/api/generate":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	prose := strings.Repeat("Layer B eligible prose with enough length for rewrite. ", 8)
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte(prose+"\n// Co-authored-by: Cursor <x>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Config{
		Paths:  []string{dir},
		LLM:    engine.LLMConfig{Mode: "ollama", Endpoint: srv.URL},
		LayerB: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	res, err := eng.Clean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) == 0 {
		t.Fatal("expected findings")
	}
}

func TestScanProgress(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\u200b"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &fakeProgress{}
	eng, err := engine.New(engine.Config{Paths: []string{dir}, Progress: p})
	if err != nil {
		t.Fatal(err)
	}
	res, err := eng.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Streamed || p.files == 0 || p.findings == 0 {
		t.Fatalf("streamed=%v files=%d findings=%d", res.Streamed, p.files, p.findings)
	}
}

func TestShouldFailNone(t *testing.T) {
	if (engine.Result{}).ShouldFail(domain.ConfidenceCertain) {
		t.Fatal("empty result")
	}
}

func TestEngineCloseNil(t *testing.T) {
	var eng *engine.Engine
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
}

type fakeProgress struct {
	files, findings int
}

func (p *fakeProgress) File(done, total int, path, detector string) { p.files++ }
func (p *fakeProgress) Finding(domain.Finding)                      { p.findings++ }
func (p *fakeProgress) Finish()                                     {}
