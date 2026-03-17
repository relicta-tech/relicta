package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "days lowercase",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
		},
		{
			name:     "days uppercase",
			input:    "7D",
			expected: 7 * 24 * time.Hour,
		},
		{
			name:     "weeks lowercase",
			input:    "2w",
			expected: 2 * 7 * 24 * time.Hour,
		},
		{
			name:     "weeks uppercase",
			input:    "2W",
			expected: 2 * 7 * 24 * time.Hour,
		},
		{
			name:     "hours lowercase",
			input:    "24h",
			expected: 24 * time.Hour,
		},
		{
			name:     "hours uppercase",
			input:    "24H",
			expected: 24 * time.Hour,
		},
		{
			name:     "30 days",
			input:    "30d",
			expected: 30 * 24 * time.Hour,
		},
		{
			name:    "too short",
			input:   "d",
			wantErr: true,
		},
		{
			name:    "invalid value",
			input:   "xd",
			wantErr: true,
		},
		{
			name:    "unknown unit",
			input:   "7m",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestShortenID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "long ID",
			input:    "abcdefghijklmnopqrstuvwxyz",
			expected: "abcdefghijkl",
		},
		{
			name:     "exactly 12 chars",
			input:    "123456789012",
			expected: "123456789012",
		},
		{
			name:     "short ID",
			input:    "short",
			expected: "short",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "13 chars",
			input:    "1234567890123",
			expected: "123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputCleanJSON(t *testing.T) {
	result := &cleanResult{
		DryRun:        true,
		TotalRuns:     10,
		DeletedCount:  3,
		DeletedIDs:    []string{"run-1", "run-2", "run-3"},
		KeptCount:     7,
		KeptIDs:       []string{"run-4", "run-5"},
		SkippedActive: 0,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputCleanJSON(result)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	// Verify it's valid JSON
	var decoded cleanResult
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.DryRun)
	assert.Equal(t, 10, decoded.TotalRuns)
	assert.Equal(t, 3, decoded.DeletedCount)
	assert.Len(t, decoded.DeletedIDs, 3)
}

func TestCleanResultFields(t *testing.T) {
	result := cleanResult{
		DryRun:        false,
		TotalRuns:     5,
		DeletedCount:  2,
		DeletedIDs:    []string{"a", "b"},
		KeptCount:     3,
		KeptIDs:       []string{"c", "d", "e"},
		SkippedActive: 1,
	}

	assert.False(t, result.DryRun)
	assert.Equal(t, 5, result.TotalRuns)
	assert.Equal(t, 2, result.DeletedCount)
	assert.Equal(t, 3, result.KeptCount)
	assert.Len(t, result.DeletedIDs, 2)
	assert.Len(t, result.KeptIDs, 3)
	assert.Equal(t, 1, result.SkippedActive)
}
