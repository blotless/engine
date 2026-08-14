package homoglyph

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// lookalikes maps Cyrillic/Greek letters commonly used in mixed-script identifiers to Latin.
var lookalikes = map[rune]rune{
	'\u0430': 'a', '\u0435': 'e', '\u043e': 'o', '\u0440': 'p', '\u0441': 'c', '\u0443': 'y', '\u0445': 'x',
	'\u0456': 'i', '\u0458': 'j', '\u0455': 's', '\u0501': 'd', '\u0261': 'g',
	'\u0410': 'A', '\u0412': 'B', '\u0415': 'E', '\u041a': 'K', '\u041c': 'M', '\u041d': 'H', '\u041e': 'O',
	'\u0420': 'P', '\u0421': 'C', '\u0422': 'T', '\u0425': 'X', '\u0406': 'I',
	'\u0391': 'A', '\u0392': 'B', '\u0395': 'E', '\u0396': 'Z', '\u0397': 'H', '\u0399': 'I', '\u039a': 'K',
	'\u039c': 'M', '\u039d': 'N', '\u039f': 'O', '\u03a1': 'P', '\u03a4': 'T', '\u03a5': 'Y', '\u03a7': 'X',
	'\u03b1': 'a', '\u03b2': 'b', '\u03b5': 'e', '\u03b9': 'i', '\u03ba': 'k', '\u03bd': 'v', '\u03bf': 'o',
	'\u03c1': 'p', '\u03c4': 't', '\u03c5': 'y', '\u03c7': 'x', '\u03b3': 'y',
}

// ASCII folds mixed-script lookalikes to a Latin identifier.
func ASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < utf8.RuneSelf && isASCIIIdent(r):
			b.WriteByte(byte(r))
		default:
			if m, ok := lookalikes[r]; ok {
				b.WriteRune(m)
			}
		}
	}
	out := b.String()
	if out == "" {
		return "ident"
	}
	first, _ := utf8.DecodeRuneInString(out)
	if !unicode.IsLetter(first) && first != '_' {
		return "x" + out
	}
	return out
}

func isASCIIIdent(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// UniqueASCII returns ASCII(s) not present in taken, then records it in taken.
func UniqueASCII(s string, taken map[string]struct{}) string {
	base := ASCII(s)
	if _, used := taken[base]; !used {
		taken[base] = struct{}{}
		return base
	}
	for i := 2; ; i++ {
		cand := base + itoa(i)
		if _, used := taken[cand]; !used {
			taken[cand] = struct{}{}
			return cand
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
