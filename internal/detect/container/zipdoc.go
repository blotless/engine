package container

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

const maxZipBytes = 128 << 20

var (
	dcCreatorRe   = regexp.MustCompile(`(?is)(<dc:creator[^>]*>)(.*?)(</dc:creator>)`)
	lastModByRe   = regexp.MustCompile(`(?is)(<cp:lastModifiedBy[^>]*>)(.*?)(</cp:lastModifiedBy>)`)
	applicationRe = regexp.MustCompile(`(?is)(<Application[^>]*>)(.*?)(</Application>)`)
	appVersionRe  = regexp.MustCompile(`(?is)(<AppVersion[^>]*>)(.*?)(</AppVersion>)`)
	odtGenRe      = regexp.MustCompile(`(?is)<meta:generator\b[^>]*>.*?</meta:generator\s*>`)
	odtCreatorRe  = regexp.MustCompile(`(?is)<dc:creator\b[^>]*>.*?</dc:creator\s*>`)
	ctOverrideRe  = regexp.MustCompile(`<Override\b[^>]*PartName="/customXml/[^"]*"[^>]*/>`)
)

func scanDOCX(u domain.Unit) ([]domain.Finding, error) {
	cleaned, labels, err := rewriteZip(u.Bytes, cleanDOCXPart)
	if err != nil || labels == "" {
		return nil, nil
	}
	f := rules.Apply(wholeFile(u, "container.docx_metadata", labels, string(cleaned)), "container.docx_metadata")
	f.Replacement = string(cleaned)
	return []domain.Finding{f}, nil
}

func scanODT(u domain.Unit) ([]domain.Finding, error) {
	cleaned, labels, err := rewriteZip(u.Bytes, cleanODTPart)
	if err != nil || labels == "" {
		return nil, nil
	}
	f := rules.Apply(wholeFile(u, "container.odt_metadata", labels, string(cleaned)), "container.odt_metadata")
	f.Replacement = string(cleaned)
	return []domain.Finding{f}, nil
}

func rewriteZip(data []byte, part func(name string, raw []byte, actions *[]string) ([]byte, bool)) ([]byte, string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", err
	}
	var budget int64
	var actions []string
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		budget += int64(f.UncompressedSize64)
		if budget > maxZipBytes {
			_ = zw.Close()
			return nil, "", errZipBudget
		}
		rc, err := f.Open()
		if err != nil {
			_ = zw.Close()
			return nil, "", err
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			_ = zw.Close()
			return nil, "", err
		}
		raw, keep := part(f.Name, raw, &actions)
		if !keep {
			continue
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			_ = zw.Close()
			return nil, "", err
		}
		if _, err := w.Write(raw); err != nil {
			_ = zw.Close()
			return nil, "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	if len(actions) == 0 {
		return data, "", nil
	}
	return buf.Bytes(), join(uniq(actions)), nil
}

func cleanDOCXPart(name string, raw []byte, actions *[]string) ([]byte, bool) {
	if strings.HasPrefix(name, "customXml/") {
		*actions = append(*actions, "drop "+name)
		return nil, false
	}
	if name == "[Content_Types].xml" {
		n := 0
		raw = ctOverrideRe.ReplaceAllFunc(raw, func(_ []byte) []byte {
			n++
			return nil
		})
		if n > 0 {
			*actions = append(*actions, "content-types")
		}
		return raw, true
	}
	if !strings.HasPrefix(name, "docProps/") {
		return raw, true
	}
	text := string(raw)
	orig := text
	text = scrubXMLField(text, dcCreatorRe, "dc:creator", actions)
	text = scrubXMLField(text, lastModByRe, "lastModifiedBy", actions)
	text = scrubXMLField(text, applicationRe, "Application", actions)
	text = scrubXMLField(text, appVersionRe, "AppVersion", actions)
	if strings.HasSuffix(name, "custom.xml") && (looksAI(raw) || aiMetaName.Match(raw)) {
		*actions = append(*actions, "drop "+name)
		return nil, false
	}
	if text != orig {
		return []byte(text), true
	}
	return raw, true
}

func scrubXMLField(text string, re *regexp.Regexp, label string, actions *[]string) string {
	return re.ReplaceAllStringFunc(text, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		inner := sub[2]
		if generatorAI.MatchString(inner) || aiMetaName.MatchString(inner) {
			*actions = append(*actions, label)
			return sub[1] + sub[3]
		}
		return m
	})
}

func cleanODTPart(name string, raw []byte, actions *[]string) ([]byte, bool) {
	if name != "meta.xml" {
		if looksAI(raw) && name != "content.xml" && name != "styles.xml" && name != "mimetype" && name != "META-INF/manifest.xml" {
			*actions = append(*actions, "drop "+name)
			return nil, false
		}
		return raw, true
	}
	text := string(raw)
	n := 0
	text = odtGenRe.ReplaceAllStringFunc(text, func(_ string) string {
		n++
		*actions = append(*actions, "generator")
		return ""
	})
	text = odtCreatorRe.ReplaceAllStringFunc(text, func(m string) string {
		if aiMetaName.MatchString(m) || generatorAI.MatchString(m) {
			*actions = append(*actions, "creator")
			return ""
		}
		return m
	})
	_ = n
	return []byte(text), true
}

type zipErr string

func (e zipErr) Error() string { return string(e) }

const errZipBudget zipErr = "zip decompressed size exceeds cap"
