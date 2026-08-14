//go:build llm

package llm

import "testing"

func TestNativeConstructs(t *testing.T) {
	n, err := New(Config{Mode: "native", Endpoint: "http://127.0.0.1:8080"})
	if err != nil {
		t.Fatal(err)
	}
	if n.Name() != "native" {
		t.Fatalf("name=%s", n.Name())
	}
}
