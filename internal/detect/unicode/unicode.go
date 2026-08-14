package unicode

import (
	"context"
	"unicode"
	"unicode/utf8"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/classify"
	"github.com/blotless/engine/internal/rules"
	"github.com/blotless/engine/ports"
)

const (
	zwsp        = '\u200B'
	zwnj        = '\u200C'
	zwj         = '\u200D'
	wordJoiner  = '\u2060'
	bom         = '\uFEFF'
	softHyphen  = '\u00AD'
	nbsp        = '\u00A0'
	nnbsp       = '\u202F'
	enSpace     = '\u2002'
	emSpace     = '\u2003'
	figureSpace = '\u2007'
	thinSpace   = '\u2009'
	hairSpace   = '\u200A'
	vsStart     = '\uFE00'
	vsEnd       = '\uFE0F'
	vsSupStart  = '\U000E0100'
	vsSupEnd    = '\U000E01EF'
	tagLang     = '\U000E0001'
	tagSpace    = '\U000E0020'
	tagCancel   = '\U000E007F'
	bidiStart   = '\u202A'
	bidiEnd     = '\u202E'
	bidiIsoS    = '\u2066'
	bidiIsoE    = '\u2069'
	vs16        = '\uFE0F'
	vs15        = '\uFE0E'
)

// Detector finds invisible and confusable Unicode code points.
type Detector struct {
	Aggressive bool
}

func (d Detector) ID() string { return "unicode" }

func (d Detector) Families() []domain.Family { return []domain.Family{domain.FamilyUnicode} }

func (d Detector) Scan(_ context.Context, u domain.Unit) ([]domain.Finding, error) {
	if !classify.TextPayload(u) {
		return nil, nil
	}
	src := u.Bytes
	rtl := containsRTL(src)

	var out []domain.Finding
	line, col := 1, 1
	i := 0
	runes := make([]rune, 0, 8)
	offs := make([]int, 0, 8)
	cols := make([]int, 0, 8)
	lines := make([]int, 0, 8)

	for i < len(src) {
		r, size := utf8.DecodeRune(src[i:])
		runes = append(runes, r)
		offs = append(offs, i)
		cols = append(cols, col)
		lines = append(lines, line)
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
		i += size
	}

	for idx, r := range runes {
		start := offs[idx]
		end := len(src)
		if idx+1 < len(offs) {
			end = offs[idx+1]
		}
		span := domain.Span{File: u.Path, Start: start, End: end, Line: lines[idx], Col: cols[idx]}

		switch {
		case r == zwsp:
			out = append(out, hit("unicode.zwsp", span, r, ""))
		case r == zwnj:
			if !d.Aggressive && scriptJoiner(runes, idx) {
				continue
			}
			out = append(out, hit("unicode.zwnj", span, r, ""))
		case r == zwj:
			if emojiZWJ(runes, idx) {
				continue
			}
			out = append(out, hit("unicode.zwj", span, r, ""))
		case r == wordJoiner:
			out = append(out, hit("unicode.word_joiner", span, r, ""))
		case r == bom:
			out = append(out, hit("unicode.bom", span, r, ""))
		case r == softHyphen:
			out = append(out, hit("unicode.soft_hyphen", span, r, ""))
		case r == nbsp:
			out = append(out, hit("unicode.nbsp", span, r, " "))
		case r == nnbsp:
			out = append(out, hit("unicode.nnbsp", span, r, " "))
		case r == figureSpace:
			out = append(out, hit("unicode.figure_space", span, r, " "))
		case r == thinSpace:
			out = append(out, hit("unicode.thin_space", span, r, " "))
		case r == hairSpace:
			out = append(out, hit("unicode.hair_space", span, r, " "))
		case r == emSpace:
			out = append(out, hit("unicode.em_space", span, r, " "))
		case r == enSpace:
			out = append(out, hit("unicode.en_space", span, r, " "))
		case r >= bidiStart && r <= bidiEnd || r >= bidiIsoS && r <= bidiIsoE || r == '\u200E' || r == '\u200F' || r == '\u061C':
			f := hit("unicode.bidi", span, r, "")
			if d.Aggressive || !rtl {
				f.Clean = domain.CleanStrip
			}
			out = append(out, f)
		case r >= vsStart && r <= vsEnd:
			if emojiVS(runes, idx) && !d.Aggressive {
				continue
			}
			out = append(out, hit("unicode.vs", span, r, ""))
		case r >= vsSupStart && r <= vsSupEnd:
			out = append(out, hit("unicode.vs_supplement", span, r, ""))
		case r == tagLang || (r >= tagSpace && r <= tagCancel):
			if !d.Aggressive && flagTag(runes, idx) {
				continue
			}
			out = append(out, hit("unicode.tags", span, r, ""))
		case extraSpace[r]:
			out = append(out, hit("unicode.space_homoglyph", span, r, " "))
		case formatInvisible[r]:
			out = append(out, hit("unicode.format", span, r, ""))
		case unicode.Is(unicode.Cf, r) && !orthoCf[r]:
			out = append(out, hit("unicode.other_cf", span, r, ""))
		case d.Aggressive:
			if repl, ok := latinConfusable[r]; ok && latinNeighbor(runes, idx) {
				out = append(out, hit("unicode.confusable", span, r, string(repl)))
			}
		}
	}
	return out, nil
}

func hit(id string, span domain.Span, r rune, repl string) domain.Finding {
	f := rules.Apply(domain.Finding{
		Span:        span,
		Evidence:    "U+" + runeHex(r),
		Replacement: repl,
	}, id)
	return f
}

func runeHex(r rune) string {
	const hex = "0123456789ABCDEF"
	if r <= 0xFFFF {
		return string([]byte{
			hex[(r>>12)&0xF], hex[(r>>8)&0xF], hex[(r>>4)&0xF], hex[r&0xF],
		})
	}
	out := make([]byte, 0, 6)
	for shift := 20; shift >= 0; shift -= 4 {
		out = append(out, hex[(r>>shift)&0xF])
	}
	i := 0
	for i < len(out)-1 && out[i] == '0' {
		i++
	}
	return string(out[i:])
}

func emojiZWJ(runes []rune, idx int) bool {
	prev := prevNonVS(runes, idx-1)
	next := nextNonVS(runes, idx+1)
	return isEmojiContext(prev) && isEmojiContext(next)
}

func emojiVS(runes []rune, idx int) bool {
	r := runes[idx]
	if r != vs15 && r != vs16 {
		return false
	}
	prev := prevNonVS(runes, idx-1)
	return isEmojiContext(prev)
}

func prevNonVS(runes []rune, idx int) rune {
	for i := idx; i >= 0; i-- {
		r := runes[i]
		if r >= vsStart && r <= vsEnd {
			continue
		}
		if r >= 0x1F3FB && r <= 0x1F3FF {
			continue
		}
		return r
	}
	return 0
}

func nextNonVS(runes []rune, idx int) rune {
	for i := idx; i < len(runes); i++ {
		r := runes[i]
		if r >= vsStart && r <= vsEnd {
			continue
		}
		if r >= 0x1F3FB && r <= 0x1F3FF {
			continue
		}
		return r
	}
	return 0
}

func isEmojiContext(r rune) bool {
	if r == 0 {
		return false
	}
	if r == vs15 || r == vs16 {
		return true
	}
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return true
	}
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x2B00 && r <= 0x2BFF:
		return true
	case r >= 0x1F1E6 && r <= 0x1F1FF:
		return true
	case r >= 0x1F004 && r <= 0x1F0FF:
		return true
	default:
		return false
	}
}

func scriptJoiner(runes []rune, idx int) bool {
	prev := prevNonVS(runes, idx-1)
	return prev > 0x7F && unicode.IsLetter(prev)
}

func flagTag(runes []rune, idx int) bool {
	return isEmojiContext(prevNonVS(runes, idx-1))
}

var extraSpace = map[rune]bool{
	'\u1680': true, '\u2000': true, '\u2001': true, '\u2004': true, '\u2005': true,
	'\u2006': true, '\u2008': true, '\u205F': true, '\u3000': true,
}

var formatInvisible = map[rune]bool{
	'\u034F': true, '\u115F': true, '\u1160': true, '\u17B4': true, '\u17B5': true,
	'\u180B': true, '\u180C': true, '\u180D': true, '\u180E': true,
	'\u2061': true, '\u2062': true, '\u2063': true, '\u2064': true,
	'\u206A': true, '\u206B': true, '\u206C': true, '\u206D': true, '\u206E': true, '\u206F': true,
	'\uFFF9': true, '\uFFFA': true, '\uFFFB': true,
}

func containsRTL(src []byte) bool {
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if unicode.Is(unicode.Hebrew, r) || unicode.Is(unicode.Arabic, r) || unicode.Is(unicode.Syriac, r) {
			return true
		}
		i += size
	}
	return false
}

var _ ports.Detector = Detector{}
