package ports

import "testing"

func TestReportAndLayers(t *testing.T) {
	var r Report
	if r.FilesScanned != 0 || r.Layers != nil {
		t.Fatalf("%#v", r)
	}
	r.Layers = &Layers{A: 1, Files: 2, B: 3, Note: "agent"}
	if r.Layers.A+r.Layers.Files+r.Layers.B != 6 {
		t.Fatal("layers")
	}
	in := RewriteIn{Fragment: "hello", Strength: "paraphrase"}
	if in.Strength != "paraphrase" {
		t.Fatal("rewrite")
	}
	var m FileMeta
	if m.Path != "" {
		t.Fatal("meta")
	}
}
