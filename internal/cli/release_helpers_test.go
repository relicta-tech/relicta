package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestPrintStep(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStep(1, 5, "Testing step")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	result := buf.String()

	assert.Contains(t, result, "[1/5]")
	assert.Contains(t, result, "Testing step")
}

func TestPrintSubtitle(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSubtitle("Test Subtitle")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	result := buf.String()

	// The subtitle should be rendered (may have ANSI codes)
	assert.NotEmpty(t, result)
}

func TestIsTerminal(t *testing.T) {
	// When running in tests, stdout is typically not a terminal
	result := isTerminal()
	// We can't assert a specific value since it depends on the environment
	// Just verify the function runs without error
	_ = result
}

func TestEffectiveBumpType(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		next     string
		expected string
	}{
		{
			name:     "major bump",
			current:  "1.0.0",
			next:     "2.0.0",
			expected: "major",
		},
		{
			name:     "minor bump",
			current:  "1.0.0",
			next:     "1.1.0",
			expected: "minor",
		},
		{
			name:     "patch bump",
			current:  "1.0.0",
			next:     "1.0.1",
			expected: "patch",
		},
		{
			name:     "no bump",
			current:  "1.0.0",
			next:     "1.0.0",
			expected: "none",
		},
		{
			name:     "major with minor and patch reset",
			current:  "1.5.3",
			next:     "2.0.0",
			expected: "major",
		},
		{
			name:     "minor with patch reset",
			current:  "1.2.5",
			next:     "1.3.0",
			expected: "minor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, err := version.Parse(tt.current)
			assert.NoError(t, err)
			next, err := version.Parse(tt.next)
			assert.NoError(t, err)

			result := effectiveBumpType(current, next)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReleaseModeConstants(t *testing.T) {
	// Verify the release mode constants exist and are distinct
	assert.NotEqual(t, releaseModeNew, releaseModeTagPush)
	// Verify they are valid releaseMode values
	m := releaseModeNew
	assert.Equal(t, releaseModeNew, m)
	m = releaseModeTagPush
	assert.Equal(t, releaseModeTagPush, m)
}

func TestReleaseWorkflowContextFields(t *testing.T) {
	// Test the struct fields are accessible
	v, _ := version.Parse("1.0.0")
	ctx := releaseWorkflowContext{
		mode:            releaseModeNew,
		existingVersion: &v,
		prevTagName:     "v0.9.0",
	}

	assert.Equal(t, releaseModeNew, ctx.mode)
	assert.Equal(t, "1.0.0", ctx.existingVersion.String())
	assert.Equal(t, "v0.9.0", ctx.prevTagName)
}
