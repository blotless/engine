package snippet

import (
	"strings"
	"testing"
)

func TestExtractInvisible(t *testing.T) {
	src := []byte("hello\u200bworld")
	got := Extract(src, 5, 8)
	if got != "hello<U+200B>world" {
		t.Fatalf("%q", got)
	}
}

func TestExtractLine(t *testing.T) {
	src := []byte("aaa\nfunc Hello() {}\nbbb")
	got := Extract(src, 9, 14)
	if got != "func Hello() {}" {
		t.Fatalf("%q", got)
	}
}

func TestExtractEmptyAndBounds(t *testing.T) {
	if Extract(nil, 0, 1) != "" {
		t.Fatal("empty")
	}
	got := Extract([]byte("ab"), -1, 99)
	if got != "ab" {
		t.Fatalf("%q", got)
	}
	got = Extract([]byte("ab"), 2, 1)
	if got != "ab" {
		t.Fatalf("swap %q", got)
	}
}

func TestEscapeTabAndInvalid(t *testing.T) {
	got := Escape([]byte("a\tb\xffc"))
	if !strings.Contains(got, `\t`) || !strings.Contains(got, "<0xFF>") {
		t.Fatalf("%q", got)
	}
}

func TestEscapeTruncates(t *testing.T) {
	src := []byte(strings.Repeat("x", 200))
	got := Escape(src)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("len=%d", len(got))
	}
}

func TestEscapeFormatInvisible(t *testing.T) {
	got := Escape([]byte("a\u061Cb"))
	if !strings.Contains(got, "<U+") {
		t.Fatalf("%q", got)
	}
}
