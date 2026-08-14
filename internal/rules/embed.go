package rules

import "embed"

//go:embed v1/*.yaml
var packFS embed.FS
