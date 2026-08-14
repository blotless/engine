package snippet

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxRunes = 120

// Extract returns the line around start/end with invisible runes made visible.
func Extract(src []byte, start, end int) string {
	if len(src) == 0 {
		return ""
	}
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if end < start {
		end = start
	}
	lineStart := start
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(src) && src[lineEnd] != '\n' && src[lineEnd] != '\r' {
		lineEnd++
	}
	return Escape(src[lineStart:lineEnd])
}

// Escape renders invisible and confusable format chars as <U+XXXX>.
func Escape(src []byte) string {
	var b strings.Builder
	n := 0
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if n >= maxRunes {
			b.WriteString("...")
			break
		}
		switch {
		case r == '\t':
			b.WriteString(`\t`)
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, "<0x%02X>", src[i])
		case invisible(r):
			if r <= 0xFFFF {
				fmt.Fprintf(&b, "<U+%04X>", r)
			} else {
				fmt.Fprintf(&b, "<U+%06X>", r)
			}
		default:
			b.WriteRune(r)
		}
		n++
		i += size
	}
	return strings.TrimRight(b.String(), " \t")
}

func invisible(r rune) bool {
	switch r {
	case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF', '\u00AD',
		'\u00A0', '\u202F', '\u2002', '\u2003', '\u2007', '\u2009', '\u200A':
		return true
	}
	if r >= '\u202A' && r <= '\u202E' {
		return true
	}
	if r >= '\u2066' && r <= '\u2069' {
		return true
	}
	if r >= '\uFE00' && r <= '\uFE0F' {
		return true
	}
	if r >= '\U000E0100' && r <= '\U000E01EF' {
		return true
	}
	if r == '\U000E0001' || (r >= '\U000E0020' && r <= '\U000E007F') {
		return true
	}
	return unicode.Is(unicode.Cf, r) && r != '\n' && r != '\r'
}
