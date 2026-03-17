package version

import (
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	version := Get()

	// Should have v prefix
	if !strings.HasPrefix(version, "v") {
		t.Errorf("Get() = %s, want prefix 'v'", version)
	}

	// Should not be empty (after removing v prefix)
	trimmed := strings.TrimPrefix(version, "v")
	if len(trimmed) == 0 {
		t.Error("Get() returned only 'v' with no version number")
	}
}
