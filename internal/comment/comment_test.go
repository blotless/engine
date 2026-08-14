package comment

import "testing"

func TestLineSkipsStringsAndURLs(t *testing.T) {
	cases := []struct {
		line string
		ok   bool
		want string
	}{
		{`// Co-authored-by: Cursor`, true, `// Co-authored-by: Cursor`},
		{`# Sure, here's a helper`, true, `# Sure, here's a helper`},
		{`    -- Hope this helps!`, true, `-- Hope this helps!`},
		{`var s = "Sure, here's a helper"`, false, ""},
		{`var s = "// Sure, here's a helper"`, false, ""},
		{`u := "https://example.com/Sure, here's"`, false, ""},
		{`x := 1 // Sure, here's a helper`, true, `// Sure, here's a helper`},
		{`foo: bar # inline yaml`, true, `# inline yaml`},
		{` * Co-authored-by: Claude`, true, `* Co-authored-by: Claude`},
		{`*item := next()`, false, ""},
		{`<!-- Co-authored-by: Claude -->`, true, `<!-- Co-authored-by: Claude -->`},
		{`/* stamp */`, true, `/* stamp */`},
		{`SELECT 1 -- Hope this helps!`, false, ""},
		{``, false, ""},
		{`   `, false, ""},
		{`x := 1 <!-- leftover -->`, true, `<!-- leftover -->`},
	}
	for _, tc := range cases {
		_, region, ok := Line([]byte(tc.line))
		if ok != tc.ok {
			t.Fatalf("%q ok=%v want %v", tc.line, ok, tc.ok)
		}
		if ok && string(region) != tc.want {
			t.Fatalf("%q region=%q want %q", tc.line, region, tc.want)
		}
	}
}

func TestTexts(t *testing.T) {
	src := []byte("// short\n// this comment is long enough to keep\n# also a long enough hash comment here\n")
	got := Texts(src, 20)
	if len(got) != 2 {
		t.Fatalf("got=%q", got)
	}
	if got[0] != "this comment is long enough to keep" {
		t.Fatalf("got[0]=%q", got[0])
	}
	empty := Texts(nil, 1)
	if len(empty) != 0 {
		t.Fatalf("empty=%q", empty)
	}
}
