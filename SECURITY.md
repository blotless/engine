# Security Policy

## Supported versions

Latest `github.com/blotless/engine` release and `main`.

## Reporting

**Do not** open a public issue.

1. Email: **zikmanv@icloud.com**
2. https://github.com/blotless/engine/security/advisories/new

Include version/SHA, impact, and a minimal file that triggers the bug.

**Initial response:** within 72 hours.

## Considerations

- Detectors must not touch the filesystem
- `webaudit` is SSRF-hardened (public IPs, same-origin sitemaps)
- LLM adapters are opt-in; they may exfiltrate source if pointed at a remote API
- `AstWASM` paths must be local; URL plugins are rejected by ast
- Layer B / origin scores are not vendor-detector certificates
