package classify

import (
	"testing"

	"github.com/blotless/engine/domain"
)

func TestLanguageAndBinary(t *testing.T) {
	if LanguageFromPath("a.go") != "go" {
		t.Fatal("go")
	}
	if LanguageFromPath("a.yaml") != "yaml" {
		t.Fatal("yaml")
	}
	if LanguageFromPath("a.png") != domain.LangImage {
		t.Fatal("png lang")
	}
	if !IsBinary([]byte("a\x00b")) {
		t.Fatal("nul")
	}
	u := Decode("a.go", "a.go", []byte{0xEF, 0xBB, 0xBF, 'p'})
	if !u.HasBOM {
		t.Fatal("bom")
	}
}

func TestKindOfMedia(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0, 0, 0, 0)
	kind, skip := KindOf("x.png", png)
	if skip || kind != domain.KindImage {
		t.Fatalf("png kind=%s skip=%v", kind, skip)
	}
	kind, skip = KindOf("x.bin", []byte("a\x00b"))
	if !skip {
		t.Fatalf("unknown binary should skip, kind=%s", kind)
	}
	kind, skip = KindOf("note.txt", []byte("hello"))
	if skip || kind != domain.KindText {
		t.Fatalf("text kind=%s skip=%v", kind, skip)
	}
	pdf := []byte("%PDF-1.4\n%\x00")
	kind, skip = KindOf("a.pdf", pdf)
	if skip || kind != domain.KindContainer {
		t.Fatalf("pdf kind=%s skip=%v", kind, skip)
	}
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0}
	kind, skip = KindOf("x.jpg", jpeg)
	if skip || kind != domain.KindImage {
		t.Fatalf("jpeg kind=%s skip=%v", kind, skip)
	}
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	kind, skip = KindOf("x.webp", webp)
	if skip || kind != domain.KindImage {
		t.Fatalf("webp kind=%s skip=%v", kind, skip)
	}
	kind, skip = KindOf("doc.docx", []byte("PK\x03\x04word/document.xml rest"))
	if skip || kind != domain.KindContainer {
		t.Fatalf("docx kind=%s skip=%v", kind, skip)
	}
	kind, skip = KindOf("plain.zip", []byte("PK\x03\x04readme.txt"))
	if skip || kind == domain.KindContainer {
		t.Fatalf("plain zip should not be a document container, kind=%s skip=%v", kind, skip)
	}
}

func TestTextPayload(t *testing.T) {
	if TextPayload(domain.Unit{Kind: domain.KindImage}) {
		t.Fatal("image")
	}
	if TextPayload(domain.Unit{Kind: domain.KindContainer}) {
		t.Fatal("container")
	}
	if !TextPayload(domain.Unit{Kind: domain.KindText, Language: domain.LangHTML}) {
		t.Fatal("html")
	}
}

func TestLanguageFromPathMore(t *testing.T) {
	if LanguageFromPath("a.json") != domain.LangJSON {
		t.Fatal("json")
	}
	if LanguageFromPath("a.md") != domain.LangMarkdown {
		t.Fatal("md")
	}
	if LanguageFromPath("a.html") != domain.LangHTML {
		t.Fatal("html")
	}
	if LanguageFromPath("a.pdf") != domain.LangContainer {
		t.Fatal("pdf")
	}
}

func TestDecodeInvalidUTF8(t *testing.T) {
	u := Decode("a.txt", "a.txt", []byte{0xff, 0xfe, 'x'})
	if u.Charset != "unknown" {
		t.Fatalf("charset=%s", u.Charset)
	}
}

func TestIsImageMagic(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0)
	if !isImageMagic(png) {
		t.Fatal("png magic")
	}
	if isImageMagic([]byte("nope")) {
		t.Fatal("not image")
	}
}

func TestBinContainerMagic(t *testing.T) {
	docx := []byte("PK\x03\x04word/document.xml")
	if !isBinContainerMagic(docx) {
		t.Fatal("docx magic")
	}
	odt := []byte("PK\x03\x04content.xml" + string(make([]byte, 100)) + "meta.xml")
	if !isBinContainerMagic(odt) {
		t.Fatal("odt magic")
	}
}

func TestIsBinaryNullInMiddle(t *testing.T) {
	if !IsBinary([]byte("hello\x00world")) {
		t.Fatal("nul byte")
	}
}

func TestDecodePaths(t *testing.T) {
	u := Decode("x.unknown", "x.unknown", []byte("data"))
	if u.Language != domain.LangText {
		t.Fatalf("lang=%s", u.Language)
	}
}
