package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	releasedomain "github.com/relicta-tech/relicta/internal/domain/release/domain"
)

func TestReleaseTypeToBumpKind(t *testing.T) {
	tests := []struct {
		name     string
		input    changes.ReleaseType
		expected releasedomain.BumpKind
	}{
		{
			name:     "major release",
			input:    changes.ReleaseTypeMajor,
			expected: releasedomain.BumpMajor,
		},
		{
			name:     "minor release",
			input:    changes.ReleaseTypeMinor,
			expected: releasedomain.BumpMinor,
		},
		{
			name:     "patch release",
			input:    changes.ReleaseTypePatch,
			expected: releasedomain.BumpPatch,
		},
		{
			name:     "unknown defaults to patch",
			input:    changes.ReleaseType("unknown"),
			expected: releasedomain.BumpPatch,
		},
		{
			name:     "empty defaults to patch",
			input:    changes.ReleaseType(""),
			expected: releasedomain.BumpPatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := releaseTypeToBumpKind(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
