//go:build !llm

package llm

import "testing"

func TestNativeRequiresTag(t *testing.T) {
	_, err := New(Config{Mode: "native"})
	if err == nil {
		t.Fatal("expected error without -tags llm")
	}
}
