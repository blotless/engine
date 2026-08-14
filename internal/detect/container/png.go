package container

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/blotless/engine/domain"
)

var pngSig = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

func scanPNG(u domain.Unit) ([]domain.Finding, error) {
	cleaned, labels, ok := stripPNG(u.Bytes)
	if !ok || len(labels) == 0 {
		return nil, nil
	}
	return applyWhole(u, "container.png_metadata", labels, cleaned), nil
}

func stripPNG(data []byte) (out []byte, labels string, ok bool) {
	if !bytesHasPrefix(data, pngSig) {
		return nil, "", false
	}
	var dropped []string
	out = append([]byte(nil), pngSig...)
	pos := 8
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		ctype := data[pos+4 : pos+8]
		start := pos + 8
		end := start + length
		if end+4 > len(data) || length < 0 {
			return nil, "", false
		}
		payload := data[start:end]
		pos = end + 4
		name := string(ctype)
		drop := false
		switch {
		case name == "caBX" || name == "juMB" || name == "jumb" || len(name) >= 2 && name[:2] == "c2":
			drop = true
		case name == "eXIf" || name == "tEXt" || name == "zTXt" || name == "iTXt":
			drop = looksAI(payload)
		case looksC2PA(append(ctype, payload...)) && !keepPNG(name):
			drop = true
		}
		if drop {
			dropped = append(dropped, name)
			if h := hitLabels(payload); h != "" {
				dropped[len(dropped)-1] = name + " (" + h + ")"
			}
		} else {
			out = append(out, data[start-8:end+4]...)
		}
		if name == "IEND" {
			break
		}
	}
	if len(dropped) == 0 {
		return data, "", true
	}
	return out, "PNG " + join(dropped), true
}

func keepPNG(name string) bool {
	switch name {
	case "IHDR", "IDAT", "IEND", "PLTE", "tRNS", "gAMA", "pHYs", "sRGB", "cHRM", "iCCP":
		return true
	default:
		return false
	}
}

func bytesHasPrefix(b, pfx []byte) bool {
	if len(b) < len(pfx) {
		return false
	}
	for i, c := range pfx {
		if b[i] != c {
			return false
		}
	}
	return true
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func pngChunk(typ string, payload []byte) []byte {
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	copy(hdr[4:8], typ)
	crc := crc32.NewIEEE()
	_, _ = crc.Write([]byte(typ))
	_, _ = crc.Write(payload)
	var c [4]byte
	binary.BigEndian.PutUint32(c[:], crc.Sum32())
	out := append(hdr[:], payload...)
	return append(out, c[:]...)
}
