// Package version provides version information for the relicta binary.
package version

import (
	_ "embed"
	"strings"
)

// VERSION contains the version from the VERSION file.
// This is used as a fallback when ldflags are not set (e.g., go install).
//
//go:embed VERSION
var VERSION string

// Get returns the version with "v" prefix.
func Get() string {
	return "v" + strings.TrimSpace(VERSION)
}
