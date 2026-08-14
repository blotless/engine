package webaudit

import (
	"bytes"
	"compress/gzip"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestGuessKind(t *testing.T) {
	cases := []struct {
		url, ct string
		data    []byte
		want    string
	}{
		{"https://x/a.png", "", []byte{0x89, 'P', 'N', 'G'}, "png"},
		{"https://x/a", "", []byte("%PDF-1.4"), "pdf"},
		{"https://x/", "text/html; charset=utf-8", []byte("<html>hi"), "html"},
		{"https://x/a.jpeg", "image/jpeg", nil, "jpeg"},
		{"https://x/a.svg", "image/svg+xml", nil, "svg"},
		{"https://x/a.pdf", "application/pdf", nil, "pdf"},
		{"https://x/a.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil, "docx"},
		{"https://x/a.odt", "application/vnd.oasis.opendocument.text", nil, "odt"},
		{"https://x/a.md", "text/markdown", nil, "markdown"},
		{"https://x/a.txt", "text/plain", nil, "text"},
		{"https://x/page.htm", "", []byte("<html><body>x</body></html>"), "html"},
		{"https://x/x", "", []byte("  <svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), "svg"},
		{"https://x/x", "", []byte("plain prose"), "text"},
	}
	for _, tc := range cases {
		if got := GuessKind(tc.url, tc.data, tc.ct); got != tc.want {
			t.Fatalf("%s ct=%q: got %q want %q", tc.url, tc.ct, got, tc.want)
		}
	}
}

func TestExtForKind(t *testing.T) {
	cases := map[string]string{
		"png": ".png", "jpeg": ".jpg", "svg": ".svg", "pdf": ".pdf",
		"docx": ".docx", "odt": ".odt", "html": ".html", "markdown": ".md",
		"text": ".txt", "other": ".txt",
	}
	for kind, want := range cases {
		if got := ExtForKind(kind); got != want {
			t.Fatalf("%s: got %q want %q", kind, got, want)
		}
	}
}

func TestParseOriginRejects(t *testing.T) {
	cases := []string{
		"file:///etc/passwd",
		"http://user:pass@evil.com/",
		"ftp://example.com/",
		"http:///no-host",
	}
	for _, raw := range cases {
		if _, err := parseOrigin(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestParseOriginDefaults(t *testing.T) {
	httpO, err := parseOrigin("http://Example.COM/path")
	if err != nil {
		t.Fatal(err)
	}
	if httpO.scheme != "http" || httpO.host != "example.com" || httpO.port != 80 {
		t.Fatalf("%+v", httpO)
	}
	httpsO, err := parseOrigin("https://example.com:8443/x")
	if err != nil {
		t.Fatal(err)
	}
	if httpsO.port != 8443 {
		t.Fatalf("%+v", httpsO)
	}
}

func TestOriginAllowedUpgrade(t *testing.T) {
	httpO := origin{scheme: "http", host: "example.com", port: 80}
	httpsO := origin{scheme: "https", host: "example.com", port: 443}
	if !originAllowed(httpsO, httpO) {
		t.Fatal("upgrade")
	}
	other := origin{scheme: "https", host: "evil.com", port: 443}
	if originAllowed(other, httpO) {
		t.Fatal("cross")
	}
	if !originAllowed(httpO, httpO) {
		t.Fatal("same")
	}
}

func TestParseSitemap(t *testing.T) {
	xml := `<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
</urlset>`
	kind, locs, err := parseSitemap([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if kind != "urlset" || len(locs) != 2 {
		t.Fatalf("%s %#v", kind, locs)
	}

	index := `<?xml version="1.0"?><sitemapindex><sitemap><loc>https://example.com/sub.xml</loc></sitemap></sitemapindex>`
	kind, locs, err = parseSitemap([]byte(index))
	if err != nil || kind != "sitemapindex" || len(locs) != 1 {
		t.Fatalf("index: kind=%s locs=%v err=%v", kind, locs, err)
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(xml))
	_ = zw.Close()
	kind, locs, err = parseSitemap(gz.Bytes())
	if err != nil || kind != "urlset" || len(locs) != 2 {
		t.Fatalf("gzip: kind=%s locs=%v err=%v", kind, locs, err)
	}

	if _, _, err := parseSitemap([]byte{0x1f, 0x8b, 0x00, 0x00}); err == nil {
		t.Fatal("expected bad gzip error")
	}

	kind, locs, err = parseSitemap([]byte("<not>valid</not>"))
	if err != nil || kind != "urlset" {
		t.Fatalf("fallback kind=%s err=%v locs=%v", kind, err, locs)
	}
}

func TestParseSitemapTooLarge(t *testing.T) {
	big := make([]byte, maxSitemapBytes+1)
	big[0], big[1] = '<', 'u'
	if _, _, err := parseSitemap(big); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("%v", err)
	}
}

func TestResolvePublicLoopback(t *testing.T) {
	_, err := resolvePublic(origin{scheme: "http", host: "127.0.0.1", port: 80})
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("%v", err)
	}
	_, err = resolvePublic(origin{scheme: "http", host: "10.0.0.1", port: 80})
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("private: %v", err)
	}
}

func TestResolvePublicExample(t *testing.T) {
	addrs, err := resolvePublic(origin{scheme: "https", host: "example.com", port: 443})
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) == 0 {
		t.Fatal("no addrs")
	}
}

func TestIsGlobalIP(t *testing.T) {
	if isGlobalIP(nil) {
		t.Fatal("nil")
	}
	if isGlobalIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("loopback")
	}
	if !isGlobalIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public")
	}
}

func TestValidatedTargetCrossOrigin(t *testing.T) {
	allowed := origin{scheme: "https", host: "example.com", port: 443}
	_, _, err := validatedTarget("https://evil.com/", &allowed)
	if err == nil || !strings.Contains(err.Error(), "cross-origin") {
		t.Fatalf("%v", err)
	}
}

func TestResolveURLAndPath(t *testing.T) {
	if got := resolveURL("https://example.com/a/b", "../c"); got != "https://example.com/c" {
		t.Fatalf("%q", got)
	}
	if got := resolveURL("%%%", "x"); got != "x" {
		t.Fatalf("%q", got)
	}
	if got := urlPath("https://example.com/dir/page.html"); got != "/dir/page.html" {
		t.Fatalf("%q", got)
	}
	if got := urlPath("not-a-url"); got != "not-a-url" {
		t.Fatalf("%q", got)
	}
}

func TestCollectRequiresInput(t *testing.T) {
	_, _, err := Collect(context.Background(), Options{})
	if err == nil || !strings.Contains(err.Error(), "sitemap") {
		t.Fatalf("%v", err)
	}
}

func TestCollectNoSitemapOnBase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := Collect(ctx, Options{Base: "https://example.com", Timeout: 15 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "no sitemap") {
		t.Fatalf("%v", err)
	}
}

func TestFetchURLRejectsLoopback(t *testing.T) {
	_, _, err := FetchURL(context.Background(), nil, "http://127.0.0.1:1/", time.Second, 1024, nil)
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("%v", err)
	}
}

func TestFetchURLExample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, ct, err := FetchURL(ctx, nil, "https://example.com/", 15*time.Second, DefaultMaxBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || !strings.Contains(strings.ToLower(ct), "html") {
		t.Fatalf("len=%d ct=%q", len(data), ct)
	}
	if GuessKind("https://example.com/", data, ct) != "html" {
		t.Fatal("kind")
	}
}

func TestCollectFromPublicSitemap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_, urls, err := Collect(ctx, Options{
		Sitemap:  "https://www.sitemaps.org/sitemap.xml",
		Timeout:  20 * time.Second,
		MaxPages: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) == 0 {
		t.Fatal("expected urls from public sitemap")
	}
}

func TestParseOriginBadURL(t *testing.T) {
	if _, err := parseOrigin("://bad"); err == nil {
		t.Fatal("expected bad url error")
	}
}

func TestCollectDefaults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := Collect(ctx, Options{Sitemap: "https://www.sitemaps.org/sitemap.xml", MaxPages: 1})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPinnedDialerConnects(t *testing.T) {
	o, addrs, err := validatedTarget("https://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	dial := pinnedDialer(o, addrs, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := dial(ctx, "tcp", "example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestDiscoverSitemapFromBase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got, err := discoverSitemap(ctx, nil, "https://www.sitemaps.org", DefaultTimeout, DefaultMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected sitemap URL from base discovery")
	}
}
