package ciapproval

import (
	"testing"
)

func TestExtractActorName(t *testing.T) {
	tests := []struct {
		name     string
		actorID  string
		expected string
	}{
		{"simple ID", "john", "john"},
		{"prefixed ID", "ci:github-actions", "github-actions"},
		{"multiple colons", "agent:ci:github", "github"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractActorName(tt.actorID)
			if got != tt.expected {
				t.Errorf("extractActorName(%q) = %q, want %q", tt.actorID, got, tt.expected)
			}
		})
	}
}
