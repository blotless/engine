package layerb

import (
	"testing"

	"github.com/blotless/engine/domain"
	"github.com/blotless/engine/internal/rules"
)

func TestSkipsUnmarked(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	u := domain.Unit{Path: "README.md", Bytes: []byte("hello world this is a long enough human readme that would otherwise be paraphrased if it were marked as AI generated content for watermark reduction purposes only.")}
	if fs := Plan(u, nil, false, ""); len(fs) != 0 {
		t.Fatalf("unmarked: %#v", fs)
	}
}

func TestForceUnmarked(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 0, 400)
	for len(body) < 300 {
		body = append(body, []byte("This is ordinary prose without Layer A marks. ")...)
	}
	fs := Plan(domain.Unit{Path: "notes.md", Bytes: body}, nil, true, "")
	if len(fs) != 1 || fs[0].RuleID != "statwm.layer_b_prose" {
		t.Fatalf("force: %#v", fs)
	}
}

func TestForceSkipsImages(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	fs := Plan(domain.Unit{Path: "a.png", Bytes: []byte("x"), Kind: domain.KindImage}, nil, true, "")
	if len(fs) != 0 {
		t.Fatalf("image: %#v", fs)
	}
}

func TestProseWhenMarked(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 0, 400)
	for len(body) < 300 {
		body = append(body, []byte("This assistant wrote a long paragraph about the API client. ")...)
	}
	fs := Plan(domain.Unit{Path: "notes.md", Bytes: body}, []domain.Finding{{
		RuleID: "stamp.co_authored_by_ai",
		Family: domain.FamilyStamp,
	}}, false, "humanize")
	if len(fs) != 1 || fs[0].RuleID != "statwm.layer_b_prose" || fs[0].Clean != domain.CleanRewrite || fs[0].Kind != "humanize" {
		t.Fatalf("%#v", fs)
	}
}

func TestCommentsWhenMarked(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	src := []byte(`package p
// New creates an API client that talks to OpenProject using a long enough comment.
func New() {}
`)
	fs := Plan(domain.Unit{Path: "a.go", Bytes: src}, []domain.Finding{{
		RuleID: "stamp.co_authored_by_ai",
		Family: domain.FamilyStamp,
	}}, false, "")
	if len(fs) != 2 {
		t.Fatalf("%#v", fs)
	}
	if fs[0].RuleID != "statwm.ast_transform" || fs[1].RuleID != "statwm.layer_b_comment" {
		t.Fatalf("%#v", fs)
	}
}

func TestSurveyRecommended(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 0, 400)
	for len(body) < 300 {
		body = append(body, []byte("This assistant wrote a long paragraph about the API client. ")...)
	}
	u := domain.Unit{Path: "notes.md", RelPath: "notes.md", Bytes: body}
	rec, opt := Survey([]domain.Unit{u}, map[string][]domain.Finding{
		"notes.md": {{Family: domain.FamilyStamp}},
	})
	if len(rec) != 1 || rec[0] != "notes.md" || len(opt) != 0 {
		t.Fatalf("rec=%v opt=%v", rec, opt)
	}
	rec, opt = Survey([]domain.Unit{u}, nil)
	if len(rec) != 0 || len(opt) != 1 {
		t.Fatalf("unmarked rec=%v opt=%v", rec, opt)
	}
}

func TestStrengthAliases(t *testing.T) {
	if _, err := rules.Load(); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 0, 400)
	for len(body) < 300 {
		body = append(body, []byte("This assistant wrote a long paragraph about the API client. ")...)
	}
	u := domain.Unit{Path: "notes.md", Bytes: body}
	marked := []domain.Finding{{RuleID: "stamp.co_authored_by_ai", Family: domain.FamilyStamp}}
	for _, tc := range []struct {
		in   string
		kind string
	}{
		{"backtranslate", "backtranslate"},
		{"structural", "structural"},
		{"unknown", "paraphrase"},
	} {
		fs := Plan(u, marked, false, tc.in)
		if len(fs) != 1 || fs[0].Kind != tc.kind {
			t.Fatalf("%s: %#v", tc.in, fs)
		}
	}
	code := Plan(u, marked, false, "code")
	if len(code) != 1 || code[0].Kind != "paraphrase" {
		t.Fatalf("code: %#v", code)
	}
}
