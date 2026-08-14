package redact

import (
	"regexp"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]+?-----END [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*\S+`),
	regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,2}`),
}

// Secrets replaces high-risk tokens before any LLM call.
func Secrets(s string) string {
	out := s
	for _, re := range patterns {
		out = re.ReplaceAllString(out, "[REDACTED]")
	}
	return out
}
