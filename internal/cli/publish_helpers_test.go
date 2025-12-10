// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"testing"
)

func TestPrintPublishSummary(t *testing.T) {
	// Just verify it doesn't panic
	printPublishSummary("1.0.0", "v1.0.0")
}
