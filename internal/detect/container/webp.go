package container

import (
	"encoding/binary"

	"github.com/blotless/engine/domain"
)

func isWebP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

type webpChunk struct {
	fourcc  []byte
	payload []byte
	padding []byte
}

func scanWebP(u domain.Unit) ([]domain.Finding, error) {
	cleaned, labels, err := stripWebP(u.Bytes)
	if err != nil || labels == "" {
		return nil, nil
	}
	return applyWhole(u, "container.webp_metadata", labels, cleaned), nil
}

func webpChunks(data []byte) (chunks []webpChunk, notes []string) {
	if !isWebP(data) {
		return nil, []string{"not a WebP"}
	}
	declared := binary.LittleEndian.Uint32(data[4:8])
	if int(declared)+8 != len(data) {
		notes = append(notes, "RIFF size mismatch")
	}
	pos := 12
	for pos+8 <= len(data) {
		fourcc := data[pos : pos+4]
		length := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		start := pos + 8
		end := start + length
		padded := end + (length & 1)
		if padded > len(data) {
			notes = append(notes, "truncated WebP chunk "+string(fourcc))
			break
		}
		chunks = append(chunks, webpChunk{
			fourcc:  append([]byte(nil), fourcc...),
			payload: data[start:end],
			padding: data[end:padded],
		})
		pos = padded
	}
	if pos != len(data) && !hasTrunc(notes) {
		notes = append(notes, "trailing WebP bytes")
	}
	return chunks, notes
}

func hasTrunc(notes []string) bool {
	for _, n := range notes {
		if len(n) >= 9 && n[:9] == "truncated" {
			return true
		}
	}
	return false
}

func stripWebP(data []byte) ([]byte, string, error) {
	chunks, notes := webpChunks(data)
	if len(chunks) == 0 && len(notes) == 1 && notes[0] == "not a WebP" {
		return nil, "", errNotWebP
	}
	if len(notes) > 0 {
		return nil, "", errMalformedWebP
	}
	var dropped []string
	var kept []webpChunk
	removedFlags := byte(0)
	metaFlags := map[string]byte{"ICCP": 0x20, "EXIF": 0x08, "XMP ": 0x04}
	for _, c := range chunks {
		name := string(c.fourcc)
		drop := name == "C2PA" || name == "c2pa"
		if name == "EXIF" || name == "XMP " {
			drop = drop || looksAI(c.payload)
		} else if name == "ICCP" && looksAI(c.payload) {
			drop = true
		}
		if drop {
			dropped = append(dropped, name)
			removedFlags |= metaFlags[name]
			continue
		}
		kept = append(kept, c)
	}
	if len(dropped) == 0 {
		return data, "", nil
	}
	body := append([]byte(nil), []byte("WEBP")...)
	for _, c := range kept {
		payload := c.payload
		if string(c.fourcc) == "VP8X" && len(payload) >= 1 && removedFlags != 0 {
			payload = append([]byte{payload[0] & ^removedFlags}, payload[1:]...)
		}
		body = append(body, c.fourcc...)
		var ln [4]byte
		binary.LittleEndian.PutUint32(ln[:], uint32(len(payload)))
		body = append(body, ln[:]...)
		body = append(body, payload...)
		if len(payload)&1 == 1 {
			if len(c.padding) == 1 {
				body = append(body, c.padding...)
			} else {
				body = append(body, 0)
			}
		}
	}
	var sz [4]byte
	binary.LittleEndian.PutUint32(sz[:], uint32(len(body)))
	out := append([]byte("RIFF"), sz[:]...)
	out = append(out, body...)
	return out, "WebP " + join(dropped), nil
}

type webpErr string

func (e webpErr) Error() string { return string(e) }

const errNotWebP webpErr = "not WebP"
const errMalformedWebP webpErr = "malformed WebP"
