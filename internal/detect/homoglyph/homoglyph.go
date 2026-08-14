package homoglyph

import (
	"unicode"
	"unicode/utf8"
)

// MixedScript reports whether s contains both Latin and a lookalike script.
func MixedScript(s string) bool {
	hasLatin, hasOther := false, false
	for _, r := range s {
		if unicode.Is(unicode.Latin, r) {
			hasLatin = true
			continue
		}
		if unicode.Is(unicode.Cyrillic, r) || unicode.Is(unicode.Greek, r) {
			hasOther = true
		}
	}
	return hasLatin && hasOther
}

// LineCol returns 1-based line/col for a byte offset.
func LineCol(src []byte, off int) (line, col int) {
	line, col = 1, 1
	i := 0
	for i < off && i < len(src) {
		r, size := utf8.DecodeRune(src[i:])
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
		i += size
	}
	return line, col
}
