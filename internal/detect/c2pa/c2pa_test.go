package c2pa

import (
	"context"
	"strings"
	"testing"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

func TestStructuredAndHTML(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := []byte("# " + manifestBegin() + " https://example/m.c2pa " + manifestEnd() + "\n" +
		`<script type="application/` + `c2pa">{}</script>` + "\n" +
		`<link rel="c2pa-` + `manifest" href="m.c2pa">` + "\n")
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "a.md", Bytes: src})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"c2pa.structured":  false,
		"c2pa.html_script": false,
		"c2pa.html_link":   false,
	}
	for _, f := range fs {
		want[f.RuleID] = true
	}
	for id, ok := range want {
		if !ok {
			t.Fatalf("missing %s in %#v", id, fs)
		}
	}
}

func TestUnstructuredRun(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("hello")
	for i := 0; i < 20; i++ {
		b.WriteRune('\uFE00' + rune(i%16))
	}
	fs, err := Detector{}.Scan(context.Background(), domain.Unit{Path: "t.txt", Bytes: []byte(b.String())})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range fs {
		if f.RuleID == "c2pa.unstructured" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing unstructured: %#v", fs)
	}
}

func TestDetectorMeta(t *testing.T) {
	d := Detector{}
	if d.ID() != "c2pa" || len(d.Families()) == 0 {
		t.Fatalf("id=%s", d.ID())
	}
}
