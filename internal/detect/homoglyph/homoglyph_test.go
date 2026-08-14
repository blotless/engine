package homoglyph

import "testing"

func TestMixedScript(t *testing.T) {
	if MixedScript("scan") {
		t.Fatal("latin only")
	}
	if MixedScript("\u0441\u043a\u0430\u043d") {
		t.Fatal("cyrillic only")
	}
	if !MixedScript("sc\u0430n") {
		t.Fatal("mixed")
	}
}

func TestASCII(t *testing.T) {
	if got := ASCII("sc\u0430n"); got != "scan" {
		t.Fatalf("%q", got)
	}
	taken := map[string]struct{}{"scan": {}}
	if got := UniqueASCII("sc\u0430n", taken); got != "scan2" {
		t.Fatalf("%q", got)
	}
}

func TestLineCol(t *testing.T) {
	src := []byte("ab\ncd")
	line, col := LineCol(src, 0)
	if line != 1 || col != 1 {
		t.Fatalf("%d:%d", line, col)
	}
	line, col = LineCol(src, 4)
	if line != 2 || col != 2 {
		t.Fatalf("%d:%d", line, col)
	}
}
