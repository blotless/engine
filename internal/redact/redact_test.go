package redact

import "testing"

func TestSecrets(t *testing.T) {
	in := "key AKIAIOSFODNN7EXAMPLE and password: hunter2"
	got := Secrets(in)
	if got == in {
		t.Fatal("expected redaction")
	}
	if !contains(got, "[REDACTED]") {
		t.Fatalf("got %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (stringIndex(s, sub) >= 0))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
