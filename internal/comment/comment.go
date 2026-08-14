package comment

import (
	"bytes"
)

// Line returns the comment portion of a source line and its byte offset.
// It understands //, #, --, <!--, /* and block-comment continuation (* ...).
// Code and string contents are disregarded, including "https://" and quoted stamps.
func Line(line []byte) (off int, region []byte, ok bool) {
	i := 0
	for i < len(line) && isWS(line[i]) {
		i++
	}
	if i >= len(line) {
		return 0, nil, false
	}
	rest := line[i:]
	switch {
	case bytes.HasPrefix(rest, []byte("//")):
		return i, rest, true
	case bytes.HasPrefix(rest, []byte("<!--")):
		return i, rest, true
	case bytes.HasPrefix(rest, []byte("/*")):
		return i, rest, true
	case bytes.HasPrefix(rest, []byte("--")):
		return i, rest, true
	case rest[0] == '#':
		return i, rest, true
	case rest[0] == '*' && (len(rest) == 1 || rest[1] == ' ' || rest[1] == '*' || rest[1] == '/'):
		return i, rest, true
	}
	if j := indexCommentToken(line, []byte("//")); j >= 0 {
		return j, line[j:], true
	}
	if j := indexCommentToken(line, []byte("<!--")); j >= 0 {
		return j, line[j:], true
	}
	if j := indexHashComment(line); j >= 0 {
		return j, line[j:], true
	}
	return 0, nil, false
}

func indexCommentToken(line, tok []byte) int {
	start := 0
	for start < len(line) {
		i := bytes.Index(line[start:], tok)
		if i < 0 {
			return -1
		}
		i += start
		if i > 0 && line[i-1] == ':' && tok[0] == '/' {
			start = i + 1
			continue
		}
		if i == 0 || isWS(line[i-1]) {
			return i
		}
		start = i + 1
	}
	return -1
}

func indexHashComment(line []byte) int {
	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		if i == 0 || isWS(line[i-1]) {
			return i
		}
	}
	return -1
}

func isWS(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r'
}

// Texts returns comment bodies of at least min bytes.
func Texts(src []byte, min int) []string {
	var out []string
	start := 0
	for start <= len(src) {
		rel := bytes.IndexByte(src[start:], '\n')
		end := len(src)
		next := len(src)
		if rel >= 0 {
			end = start + rel
			next = end + 1
		}
		_, region, ok := Line(src[start:end])
		if ok {
			t := string(bytes.TrimSpace(region))
			t = trimCommentMarks(t)
			if len(t) >= min {
				out = append(out, t)
			}
		}
		if rel < 0 {
			break
		}
		start = next
	}
	return out
}

func trimCommentMarks(s string) string {
	s = string(bytes.TrimSpace([]byte(s)))
	for _, p := range []string{"//", "<!--", "/*", "--", "#", "*"} {
		if len(s) >= len(p) && s[:len(p)] == p {
			s = s[len(p):]
			s = string(bytes.TrimSpace([]byte(s)))
		}
	}
	if n := len(s); n >= 3 && s[n-3:] == "-->" {
		s = string(bytes.TrimSpace([]byte(s[:n-3])))
	}
	if n := len(s); n >= 2 && s[n-2:] == "*/" {
		s = string(bytes.TrimSpace([]byte(s[:n-2])))
	}
	return s
}
