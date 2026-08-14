package unicode

import "unicode"

// latinConfusable maps lookalikes used as stealth carriers in otherwise Latin text.
// Aggressive mode only; skipped when neighbors are not Latin (real Cyrillic/Greek).
var latinConfusable = map[rune]rune{
	'\u0430': 'a', '\u0435': 'e', '\u043e': 'o', '\u0440': 'p', '\u0441': 'c', '\u0443': 'y', '\u0445': 'x',
	'\u0456': 'i', '\u0458': 'j', '\u0455': 's',
	'\u0410': 'A', '\u0412': 'B', '\u0415': 'E', '\u041a': 'K', '\u041c': 'M', '\u041d': 'H', '\u041e': 'O',
	'\u0420': 'P', '\u0421': 'C', '\u0422': 'T', '\u0425': 'X', '\u0406': 'I', '\u0408': 'J',
	'\u03bf': 'o', '\u03b1': 'a', '\u03bd': 'v', '\u03c1': 'p', '\u03c4': 't', '\u03c5': 'u', '\u03c7': 'x', '\u03b9': 'i',
	'\u0391': 'A', '\u0392': 'B', '\u0395': 'E', '\u0396': 'Z', '\u0397': 'H', '\u0399': 'I', '\u039a': 'K',
	'\u039c': 'M', '\u039d': 'N', '\u039f': 'O', '\u03a1': 'P', '\u03a4': 'T', '\u03a5': 'Y', '\u03a7': 'X',
}

func init() {
	for i := 0; i < 26; i++ {
		latinConfusable[rune('\uFF21'+i)] = rune('A' + i)
		latinConfusable[rune('\uFF41'+i)] = rune('a' + i)
	}
	for i := 0; i < 10; i++ {
		latinConfusable[rune('\uFF10'+i)] = rune('0' + i)
	}
}

// Orthographic Cf that must stay in Arabic/Syriac (and related) text.
var orthoCf = map[rune]bool{
	'\u0600': true, '\u0601': true, '\u0602': true, '\u0603': true,
	'\u0604': true, '\u0605': true, '\u06DD': true, '\u070F': true, '\u08E2': true,
}

func latinNeighbor(runes []rune, idx int) bool {
	for i := idx - 1; i >= 0; i-- {
		r := runes[i]
		if unicode.IsLetter(r) {
			return unicode.Is(unicode.Latin, r)
		}
	}
	for i := idx + 1; i < len(runes); i++ {
		r := runes[i]
		if unicode.IsLetter(r) {
			return unicode.Is(unicode.Latin, r)
		}
	}
	return false
}
