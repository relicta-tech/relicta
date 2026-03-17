package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

func TestGetNextSteps(t *testing.T) {
	tests := []struct {
		name     string
		state    domain.RunState
		expected []string
	}{
		{
			name:     "draft state",
			state:    domain.StateDraft,
			expected: []string{"relicta plan"},
		},
		{
			name:     "planned state",
			state:    domain.StatePlanned,
			expected: []string{"relicta bump"},
		},
		{
			name:     "versioned state",
			state:    domain.StateVersioned,
			expected: []string{"relicta notes"},
		},
		{
			name:     "notes ready state",
			state:    domain.StateNotesReady,
			expected: []string{"relicta approve"},
		},
		{
			name:     "approved state",
			state:    domain.StateApproved,
			expected: []string{"relicta publish"},
		},
		{
			name:     "publishing state",
			state:    domain.StatePublishing,
			expected: []string{"Wait for publish to complete or run 'relicta publish' to retry"},
		},
		{
			name:     "published state",
			state:    domain.StatePublished,
			expected: []string{"Release complete! Run 'relicta plan' for next release"},
		},
		{
			name:     "failed state",
			state:    domain.StateFailed,
			expected: []string{"relicta reset", "Then run 'relicta plan' to start over"},
		},
		{
			name:     "canceled state",
			state:    domain.StateCanceled,
			expected: []string{"relicta reset", "Then run 'relicta plan' to start over"},
		},
		{
			name:     "unknown state",
			state:    domain.RunState("unknown"),
			expected: []string{"relicta status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNextSteps(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStateMessage(t *testing.T) {
	tests := []struct {
		name     string
		state    domain.RunState
		expected string
	}{
		{
			name:     "draft state",
			state:    domain.StateDraft,
			expected: "Release is in draft state",
		},
		{
			name:     "planned state",
			state:    domain.StatePlanned,
			expected: "Release is planned, ready for version bump",
		},
		{
			name:     "versioned state",
			state:    domain.StateVersioned,
			expected: "Version bumped, ready for release notes",
		},
		{
			name:     "notes ready state",
			state:    domain.StateNotesReady,
			expected: "Release notes generated, ready for approval",
		},
		{
			name:     "approved state",
			state:    domain.StateApproved,
			expected: "Release approved, ready to publish",
		},
		{
			name:     "publishing state",
			state:    domain.StatePublishing,
			expected: "Release is being published...",
		},
		{
			name:     "published state",
			state:    domain.StatePublished,
			expected: "Release published successfully",
		},
		{
			name:     "failed state",
			state:    domain.StateFailed,
			expected: "Release failed. Use 'relicta reset' to clear state",
		},
		{
			name:     "canceled state",
			state:    domain.StateCanceled,
			expected: "Release was canceled. Use 'relicta reset' to clear state",
		},
		{
			name:     "unknown state",
			state:    domain.RunState("custom"),
			expected: "Release is in 'custom' state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStateMessage(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatState(t *testing.T) {
	tests := []struct {
		name        string
		state       string
		shouldMatch bool // just verify it returns something
	}{
		{name: "planned", state: "planned", shouldMatch: true},
		{name: "versioned", state: "versioned", shouldMatch: true},
		{name: "notes_ready", state: "notes_ready", shouldMatch: true},
		{name: "approved", state: "approved", shouldMatch: true},
		{name: "publishing", state: "publishing", shouldMatch: true},
		{name: "published", state: "published", shouldMatch: true},
		{name: "failed", state: "failed", shouldMatch: true},
		{name: "canceled", state: "canceled", shouldMatch: true},
		{name: "unknown", state: "unknown", shouldMatch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatState(tt.state)
			assert.NotEmpty(t, result)
			if tt.state == "unknown" {
				// Unknown states return the state as-is
				assert.Equal(t, tt.state, result)
			}
		})
	}
}

func TestOutputStatusJSON(t *testing.T) {
	output := &StatusOutput{
		HasActiveRelease: true,
		ReleaseID:        "test-release-123",
		State:            "planned",
		CurrentVersion:   "1.0.0",
		NextVersion:      "1.1.0",
		BumpKind:         "minor",
		CommitCount:      5,
		Message:          "Release is planned",
		NextSteps:        []string{"relicta bump"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputStatusJSON(output)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	// Verify it's valid JSON
	var decoded StatusOutput
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "test-release-123", decoded.ReleaseID)
	assert.Equal(t, "planned", decoded.State)
	assert.Equal(t, "1.0.0", decoded.CurrentVersion)
}

func TestOutputStatusText(t *testing.T) {
	t.Run("no active release", func(t *testing.T) {
		output := &StatusOutput{
			HasActiveRelease: false,
			Message:          "No active release found",
			NextSteps:        []string{"relicta plan"},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := outputStatusText(output)
		require.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		result := buf.String()

		assert.Contains(t, result, "No active release found")
		assert.Contains(t, result, "relicta plan")
	})

	t.Run("with active release", func(t *testing.T) {
		output := &StatusOutput{
			HasActiveRelease: true,
			ReleaseID:        "test-release",
			State:            "planned",
			CurrentVersion:   "1.0.0",
			NextVersion:      "1.1.0",
			BumpKind:         "minor",
			CommitCount:      3,
			RiskScore:        0.5,
			NextSteps:        []string{"relicta bump"},
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := outputStatusText(output)
		require.NoError(t, err)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		result := buf.String()

		assert.Contains(t, result, "test-release")
		assert.Contains(t, result, "1.0.0")
		assert.Contains(t, result, "1.1.0")
	})
}

func TestStatusOutputFields(t *testing.T) {
	output := StatusOutput{
		HasActiveRelease: true,
		ReleaseID:        "release-123",
		State:            "approved",
		CurrentVersion:   "2.0.0",
		NextVersion:      "2.1.0",
		BumpKind:         "minor",
		RiskScore:        0.25,
		CommitCount:      10,
		Message:          "Ready to publish",
		NextSteps:        []string{"relicta publish"},
	}

	assert.True(t, output.HasActiveRelease)
	assert.Equal(t, "release-123", output.ReleaseID)
	assert.Equal(t, "approved", output.State)
	assert.Equal(t, "2.0.0", output.CurrentVersion)
	assert.Equal(t, "2.1.0", output.NextVersion)
	assert.Equal(t, "minor", output.BumpKind)
	assert.Equal(t, 0.25, output.RiskScore)
	assert.Equal(t, 10, output.CommitCount)
	assert.Equal(t, "Ready to publish", output.Message)
	assert.Equal(t, []string{"relicta publish"}, output.NextSteps)
}
