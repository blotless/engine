package unicode

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

func TestScanInvisible(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "hello\u200bworld\u00a0x"
	d := Detector{}
	fs, err := d.Scan(context.Background(), domain.Unit{Path: "a.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("findings=%d %#v", len(fs), fs)
	}
	ids := fs[0].RuleID + "," + fs[1].RuleID
	if !strings.Contains(ids, "unicode.zwsp") || !strings.Contains(ids, "unicode.nbsp") {
		t.Fatalf("ids=%s", ids)
	}
	if fs[1].Replacement != " " && fs[0].Replacement != " " {
		t.Fatal("nbsp should normalize to space")
	}
}

func TestSkipEmojiZWJ(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	// family: man + ZWJ + woman + ZWJ + girl (simplified emoji sequence)
	src := "ok \U0001F468\u200D\U0001F469\u200D\U0001F467 end"
	d := Detector{}
	fs, err := d.Scan(context.Background(), domain.Unit{Path: "e.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.RuleID == "unicode.zwj" {
			t.Fatalf("emoji ZWJ flagged: %+v", f)
		}
	}
}

func TestBOMAndTags(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "\ufeffhi\U000E0020"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	var bom, tag bool
	for _, f := range fs {
		if f.RuleID == "unicode.bom" {
			bom = true
		}
		if f.RuleID == "unicode.tags" {
			tag = true
		}
	}
	if !bom || !tag {
		t.Fatalf("bom=%v tag=%v findings=%v", bom, tag, fs)
	}
}

func TestBidiRTLReportOnly(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "שלום\u202e"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "he.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "unicode.bidi" {
			found = true
			if f.Clean != domain.CleanReportOnly {
				t.Fatalf("expected report_only, got %s", f.Clean)
			}
		}
	}
	if !found {
		t.Fatal("missing bidi")
	}
	fs2, _ := Detector{Aggressive: true}.Scan(context.Background(), domain.Unit{Path: "he.txt", Bytes: []byte(src)})
	for _, f := range fs2 {
		if f.RuleID == "unicode.bidi" && f.Clean != domain.CleanStrip {
			t.Fatalf("aggressive should strip, got %s", f.Clean)
		}
	}
}

func TestFormatAndSpaceHomoglyph(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\u034Fb\u3000c"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	var format, space bool
	for _, f := range fs {
		if f.RuleID == "unicode.format" {
			format = true
		}
		if f.RuleID == "unicode.space_homoglyph" && f.Replacement == " " {
			space = true
		}
	}
	if !format || !space {
		t.Fatalf("format=%v space=%v %#v", format, space, fs)
	}
}

func TestArabicZWNJKept(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "می\u200cروم"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "fa.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.RuleID == "unicode.zwnj" {
			t.Fatalf("orthographic ZWNJ flagged: %+v", f)
		}
	}
}

func TestOtherCf(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\U0001D173b"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "unicode.other_cf" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing other_cf: %#v", fs)
	}
}

func TestOrthoCfKept(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "x\u0600y"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "ar.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.RuleID == "unicode.other_cf" {
			t.Fatalf("orthographic Cf flagged: %+v", f)
		}
	}
}

func TestAggressiveConfusable(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "sc\u0430n" // Latin sc + Cyrillic a + Latin n
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.RuleID == "unicode.confusable" {
			t.Fatal("confusable without --aggressive")
		}
	}
	fs, err = Detector{Aggressive: true}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "unicode.confusable" && f.Replacement == "a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing confusable: %#v", fs)
	}
}

func TestCyrillicWordNotConfusable(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "\u0441\u043a\u0430\u043d"
	fs, err := Detector{Aggressive: true}.Scan(context.Background(), domain.Unit{Path: "ru.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.RuleID == "unicode.confusable" {
			t.Fatalf("real Cyrillic flagged: %+v", f)
		}
	}
}

func TestDetectorMeta(t *testing.T) {
	d := Detector{}
	if d.ID() != "unicode" || len(d.Families()) == 0 {
		t.Fatalf("id=%s families=%d", d.ID(), len(d.Families()))
	}
}

func TestEmojiVSNotFlagged(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "ok \U0001F3A8\uFE0F end"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "e.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.RuleID == "unicode.vs" {
			t.Fatalf("emoji VS flagged: %+v", f)
		}
	}
}

func TestSkipImageUnit(t *testing.T) {
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{
		Path: "a.png", Bytes: []byte{0x89, 'P', 'N', 'G'}, Kind: domain.KindImage,
	})
	if err != nil || len(fs) != 0 {
		t.Fatalf("%#v %v", fs, err)
	}
}

func TestZWNJAggressive(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\u200cb"
	fs, err := Detector{Aggressive: true}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "unicode.zwnj" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%#v", fs)
	}
}

func TestWordJoinerAndThinSpace(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\u2060b\u2009c"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	var wj, thin bool
	for _, f := range fs {
		if f.RuleID == "unicode.word_joiner" {
			wj = true
		}
		if f.RuleID == "unicode.thin_space" {
			thin = true
		}
	}
	if !wj || !thin {
		t.Fatalf("wj=%v thin=%v %#v", wj, thin, fs)
	}
}

func TestHairSpaceAndEmSpace(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\u200ah\u2003b"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	var hair, em bool
	for _, f := range fs {
		if f.RuleID == "unicode.hair_space" {
			hair = true
		}
		if f.RuleID == "unicode.em_space" {
			em = true
		}
	}
	if !hair || !em {
		t.Fatalf("hair=%v em=%v %#v", hair, em, fs)
	}
}

func TestSpaceVariants(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		src, rule string
	}{
		{"a\u2007b", "unicode.figure_space"},
		{"a\u2002b", "unicode.en_space"},
		{"a\u2003b", "unicode.em_space"},
		{"a\u00adb", "unicode.soft_hyphen"},
	}
	for _, tc := range cases {
		fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(tc.src)})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range fs {
			if f.RuleID == tc.rule {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: %#v", tc.rule, fs)
		}
	}
}

func TestNNBSP(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\u202fb"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "unicode.nnbsp" && f.Replacement == " " {
			found = true
		}
	}
	if !found {
		t.Fatalf("%#v", fs)
	}
}

func TestVSSupplement(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := "a\U000E0100b"
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "unicode.vs_supplement" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%#v", fs)
	}
}

func FuzzScanNoPanic(f *testing.F) {
	f.Add([]byte("hello\u200b"))
	f.Add([]byte("\ufeff"))
	f.Add([]byte("👨‍👩‍👧"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if !utf8.Valid(b) && len(b) > 1<<20 {
			t.Skip()
		}
		_, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "f.txt", Bytes: b})
		if err != nil {
			t.Fatal(err)
		}
	})
}
