package container

import (
	"encoding/binary"

	"github.com/blotless/engine/domain"
)

var jpegSOI = []byte{0xff, 0xd8}

func scanJPEG(u domain.Unit) ([]domain.Finding, error) {
	cleaned, labels, ok := stripJPEG(u.Bytes)
	if !ok || labels == "" {
		return nil, nil
	}
	return applyWhole(u, "container.jpeg_metadata", labels, cleaned), nil
}

func stripJPEG(data []byte) (out []byte, labels string, ok bool) {
	if !bytesHasPrefix(data, jpegSOI) {
		return nil, "", false
	}
	var dropped []string
	out = append([]byte(nil), jpegSOI...)
	i := 2
	n := len(data)
	for i < n {
		if data[i] != 0xFF {
			out = append(out, data[i:]...)
			break
		}
		for i < n && data[i] == 0xFF {
			i++
		}
		if i >= n {
			break
		}
		marker := data[i]
		i++
		if marker == 0xD9 {
			out = append(out, 0xFF, 0xD9)
			break
		}
		if marker == 0xD8 {
			continue
		}
		if marker >= 0xD0 && marker <= 0xD7 {
			out = append(out, 0xFF, marker)
			continue
		}
		if marker == 0xDA {
			out = append(out, 0xFF, 0xDA)
			out = append(out, data[i:]...)
			break
		}
		if i+2 > n {
			return nil, "", false
		}
		seglen := int(binary.BigEndian.Uint16(data[i : i+2]))
		if seglen < 2 || i+seglen > n {
			return nil, "", false
		}
		payload := data[i+2 : i+seglen]
		next := i + seglen
		drop := false
		keep := true
		if marker >= 0xE0 && marker <= 0xEF {
			if marker == 0xEB { // APP11 — JUMBF/C2PA
				drop = true
				dropped = append(dropped, "APP11")
			} else if looksAI(payload) {
				drop = true
				dropped = append(dropped, "APP"+itoa(int(marker-0xE0)))
			}
			keep = !drop
		} else if marker == 0xFE && looksAI(payload) {
			drop = true
			dropped = append(dropped, "COM")
			keep = false
		}
		if keep && !drop {
			out = append(out, 0xFF, marker)
			out = append(out, data[i:i+seglen]...)
		}
		i = next
	}
	if len(dropped) == 0 {
		return data, "", true
	}
	return out, "JPEG " + join(dropped), true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
