package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Current returns the embedded semantic version.
func Current() string {
	return strings.TrimSpace(raw)
}
