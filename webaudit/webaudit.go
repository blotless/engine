// Package webaudit audits URLs from a sitemap (WR audit_website.py), SSRF-safe.
package webaudit

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMaxBytes = 4 << 20
	DefaultTimeout  = 15 * time.Second
	DefaultMaxPages = 200
	maxSitemapBytes = 64 << 20
	maxRedirects    = 5
	userAgent       = "blotless-audit-web/1.0"
)

// Options configure a website audit.
type Options struct {
	Sitemap    string
	Base       string
	MaxPages   int
	Timeout    time.Duration
	MaxBytes   int
	HTTPClient *http.Client
}

// FileResult is one inspected remote asset (caller fills findings).
type FileResult struct {
	URL   string `json:"url"`
	Kind  string `json:"kind"`
	Bytes int    `json:"bytes"`
	Error string `json:"error,omitempty"`
	Raw   []byte `json:"-"`
}

// Report is the audit aggregate shell (engine fills scan results).
type Report struct {
	Sitemap       string       `json:"sitemap"`
	Base          string       `json:"base,omitempty"`
	URLsCollected int          `json:"urls_collected"`
	URLsScanned   int          `json:"urls_scanned"`
	Failures      []Failure    `json:"urls_failed,omitempty"`
	Files         []FileResult `json:"files"`
}

// Failure records a fetch/inspect error.
type Failure struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type origin struct {
	scheme string
	host   string
	port   int
}

// Collect downloads sitemap URLs (same-origin, public IPs only).
func Collect(ctx context.Context, opt Options) (sitemapURL string, urls []string, err error) {
	if opt.MaxPages <= 0 {
		opt.MaxPages = DefaultMaxPages
	}
	if opt.Timeout <= 0 {
		opt.Timeout = DefaultTimeout
	}
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = DefaultMaxBytes
	}
	client := opt.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opt.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}

	sitemapURL = strings.TrimSpace(opt.Sitemap)
	if sitemapURL == "" {
		base := strings.TrimSpace(opt.Base)
		if base == "" {
			return "", nil, fmt.Errorf("provide --sitemap URL or --base URL")
		}
		sitemapURL, err = discoverSitemap(ctx, client, base, opt.Timeout, opt.MaxBytes)
		if err != nil {
			return "", nil, err
		}
		if sitemapURL == "" {
			return "", nil, fmt.Errorf("no sitemap found for %s", base)
		}
	}
	urls, err = collectURLs(ctx, client, sitemapURL, opt.Timeout, opt.MaxBytes, opt.MaxPages)
	return sitemapURL, urls, err
}

// FetchURL downloads one URL with SSRF protections; returns body and content-type.
func FetchURL(ctx context.Context, client *http.Client, rawURL string, timeout time.Duration, maxBytes int, allowed *origin) ([]byte, string, error) {
	if client == nil {
		client = &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}
	current := rawURL
	var expected *origin = allowed
	for redirect := 0; redirect <= maxRedirects; redirect++ {
		o, addrs, err := validatedTarget(current, expected)
		if err != nil {
			return nil, "", err
		}
		if expected == nil {
			expected = &o
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Connection", "close")
		// Pin via custom dialer
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.DialContext = pinnedDialer(o, addrs, timeout)
		c := *client
		c.Transport = tr
		c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		resp, err := c.Do(req)
		if err != nil {
			return nil, "", err
		}
		ct := resp.Header.Get("Content-Type")
		if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 || resp.StatusCode == 308 {
			loc := resp.Header.Get("Location")
			_ = resp.Body.Close()
			if loc == "" {
				return nil, "", fmt.Errorf("redirect without Location")
			}
			if redirect >= maxRedirects {
				return nil, "", fmt.Errorf("too many redirects")
			}
			current = resolveURL(current, loc)
			continue
		}
		defer resp.Body.Close()
		limited := io.LimitReader(resp.Body, int64(maxBytes)+1)
		data, err := io.ReadAll(limited)
		if err != nil {
			return nil, "", err
		}
		if len(data) > maxBytes {
			return nil, "", fmt.Errorf("exceeds %d bytes", maxBytes)
		}
		return data, ct, nil
	}
	return nil, "", fmt.Errorf("too many redirects")
}

// GuessKind classifies downloaded bytes (WR guess_kind).
func GuessKind(rawURL string, data []byte, contentType string) string {
	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	ct = strings.TrimSpace(ct)
	switch {
	case strings.Contains(ct, "html"):
		return "html"
	case ct == "image/png":
		return "png"
	case ct == "image/jpeg":
		return "jpeg"
	case strings.Contains(ct, "svg"):
		return "svg"
	case ct == "application/pdf":
		return "pdf"
	case strings.Contains(ct, "wordprocessingml"):
		return "docx"
	case strings.Contains(ct, "opendocument.text"):
		return "odt"
	case strings.Contains(ct, "markdown"):
		return "markdown"
	case ct == "text/plain":
		return "text"
	}
	path := strings.ToLower(urlPath(rawURL))
	for _, pair := range []struct{ ext, kind string }{
		{".png", "png"}, {".jpg", "jpeg"}, {".jpeg", "jpeg"}, {".svg", "svg"},
		{".pdf", "pdf"}, {".docx", "docx"}, {".odt", "odt"}, {".html", "html"},
		{".htm", "html"}, {".md", "markdown"}, {".markdown", "markdown"}, {".txt", "text"},
	} {
		if strings.HasSuffix(path, pair.ext) {
			return pair.kind
		}
	}
	if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		return "png"
	}
	if bytes.HasPrefix(data, []byte{0xff, 0xd8}) {
		return "jpeg"
	}
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return "pdf"
	}
	head := data
	if len(head) > 500 {
		head = head[:500]
	}
	low := bytes.ToLower(head)
	if bytes.Contains(low, []byte("svg")) && bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("<")) {
		return "svg"
	}
	if bytes.Contains(low, []byte("<html")) || bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("<")) {
		return "html"
	}
	return "text"
}

// ExtForKind maps kind to a temp file extension.
func ExtForKind(kind string) string {
	switch kind {
	case "png":
		return ".png"
	case "jpeg":
		return ".jpg"
	case "svg":
		return ".svg"
	case "pdf":
		return ".pdf"
	case "docx":
		return ".docx"
	case "odt":
		return ".odt"
	case "html":
		return ".html"
	case "markdown":
		return ".md"
	default:
		return ".txt"
	}
}

func discoverSitemap(ctx context.Context, client *http.Client, base string, timeout time.Duration, maxBytes int) (string, error) {
	base = strings.TrimRight(base, "/")
	o, err := parseOrigin(base)
	if err != nil {
		return "", err
	}
	for _, candidate := range []string{base + "/sitemap.xml", base + "/sitemap_index.xml"} {
		data, _, err := FetchURL(ctx, client, candidate, timeout, maxBytes, &o)
		if err != nil {
			continue
		}
		if _, _, err := parseSitemap(data); err == nil {
			return candidate, nil
		}
	}
	data, _, err := FetchURL(ctx, client, base+"/robots.txt", timeout, 1<<20, &o)
	if err != nil {
		return "", nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			candidate := strings.TrimSpace(line[len("sitemap:"):])
			co, err := parseOrigin(candidate)
			if err != nil {
				return "", err
			}
			if !originAllowed(co, o) {
				return "", fmt.Errorf("cross-origin sitemap is not allowed: %s", candidate)
			}
			return candidate, nil
		}
	}
	return "", nil
}

func collectURLs(ctx context.Context, client *http.Client, sitemapURL string, timeout time.Duration, maxBytes, maxPages int) ([]string, error) {
	o, err := parseOrigin(sitemapURL)
	if err != nil {
		return nil, err
	}
	var urls []string
	seen := map[string]struct{}{}
	var recurse func(string, int) error
	recurse = func(u string, depth int) error {
		if len(urls) >= maxPages || depth > 3 {
			return nil
		}
		data, _, err := FetchURL(ctx, client, u, timeout, maxBytes, &o)
		if err != nil {
			return err
		}
		kind, locs, err := parseSitemap(data)
		if err != nil {
			return err
		}
		if kind == "sitemapindex" {
			for _, loc := range locs {
				co, err := parseOrigin(loc)
				if err != nil {
					return err
				}
				if !originAllowed(co, o) {
					return fmt.Errorf("cross-origin sitemap URL is not allowed: %s", loc)
				}
				if _, ok := seen[loc]; ok {
					continue
				}
				seen[loc] = struct{}{}
				if err := recurse(loc, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		for _, loc := range locs {
			co, err := parseOrigin(loc)
			if err != nil {
				return err
			}
			if !originAllowed(co, o) {
				return fmt.Errorf("cross-origin sitemap URL is not allowed: %s", loc)
			}
			if _, ok := seen[loc]; ok {
				continue
			}
			seen[loc] = struct{}{}
			urls = append(urls, loc)
			if len(urls) >= maxPages {
				break
			}
		}
		return nil
	}
	if err := recurse(sitemapURL, 0); err != nil {
		return nil, err
	}
	return urls, nil
}

func parseSitemap(data []byte) (kind string, locs []string, err error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "", nil, err
		}
		defer gr.Close()
		data, err = io.ReadAll(io.LimitReader(gr, maxSitemapBytes+1))
		if err != nil {
			return "", nil, err
		}
	}
	if len(data) > maxSitemapBytes {
		return "", nil, fmt.Errorf("sitemap exceeds %d bytes", maxSitemapBytes)
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var root struct {
		XMLName xml.Name
		Locs    []string `xml:"url>loc"`
		Maps    []string `xml:"sitemap>loc"`
	}
	// Manual walk for namespaces
	type node struct {
		XMLName xml.Name
		Content []byte `xml:",innerxml"`
	}
	_ = root
	locs = nil
	kind = ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		local := se.Name.Local
		if kind == "" && (local == "urlset" || local == "sitemapindex") {
			kind = local
		}
		if local == "loc" {
			var loc string
			if err := dec.DecodeElement(&loc, &se); err != nil {
				return "", nil, err
			}
			loc = strings.TrimSpace(loc)
			if loc != "" {
				locs = append(locs, loc)
			}
		}
	}
	if kind == "" {
		kind = "urlset"
	}
	return kind, locs, nil
}

func parseOrigin(raw string) (origin, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return origin{}, err
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return origin{}, fmt.Errorf("unsupported URL scheme: %s", scheme)
	}
	if u.User != nil {
		return origin{}, fmt.Errorf("credentials in URLs are not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return origin{}, fmt.Errorf("URL has no hostname")
	}
	port := u.Port()
	p := 0
	if port != "" {
		fmt.Sscanf(port, "%d", &p)
	} else if scheme == "https" {
		p = 443
	} else {
		p = 80
	}
	return origin{scheme: scheme, host: strings.ToLower(strings.TrimRight(host, ".")), port: p}, nil
}

func originAllowed(candidate, expected origin) bool {
	if candidate == expected {
		return true
	}
	return expected.scheme == "http" && expected.port == 80 &&
		candidate.scheme == "https" && candidate.port == 443 &&
		candidate.host == expected.host
}

func validatedTarget(rawURL string, expected *origin) (origin, []string, error) {
	o, err := parseOrigin(rawURL)
	if err != nil {
		return origin{}, nil, err
	}
	if expected != nil && !originAllowed(o, *expected) {
		return origin{}, nil, fmt.Errorf("cross-origin URL is not allowed: %s://%s:%d", o.scheme, o.host, o.port)
	}
	addrs, err := resolvePublic(o)
	if err != nil {
		return origin{}, nil, err
	}
	return o, addrs, nil
}

func resolvePublic(o origin) ([]string, error) {
	host := strings.Split(o.host, "%")[0]
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			// WR uses is_global — reject private/loopback
			if !isGlobalIP(ip) {
				return nil, fmt.Errorf("refusing non-public address for %s: %s", o.host, ip)
			}
		}
		if !isGlobalIP(ip) {
			return nil, fmt.Errorf("refusing non-public address for %s: %s", o.host, ip)
		}
		return []string{ip.String()}, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve hostname %s: %w", host, err)
	}
	var out []string
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			ip = v4
		}
		if !isGlobalIP(ip) {
			return nil, fmt.Errorf("refusing non-public address for %s: %s", host, ip)
		}
		s := ip.String()
		dup := false
		for _, x := range out {
			if x == s {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("hostname resolved to no IP addresses: %s", host)
	}
	return out, nil
}

func isGlobalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// Reject CGNAT / documentation ranges roughly via IsPrivate already for RFC1918
	return ip.IsGlobalUnicast()
}

func pinnedDialer(o origin, addrs []string, timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		var last error
		d := net.Dialer{Timeout: timeout}
		for _, a := range addrs {
			c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(a, fmt.Sprintf("%d", o.port)))
			if err == nil {
				return c, nil
			}
			last = err
		}
		if last != nil {
			return nil, last
		}
		return nil, fmt.Errorf("no validated address for %s", o.host)
	}
}

func resolveURL(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

func urlPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Path
}
