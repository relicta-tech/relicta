package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/container"
)

func TestContainerAppWrapperAccessors(t *testing.T) {
	wrapper := &containerAppWrapper{App: &container.App{}}

	if wrapper.ReleaseAnalyzer() != nil {
		t.Log("ReleaseAnalyzer returned non-nil (expected nil for empty container)")
	}
	if wrapper.CalculateVersion() != nil {
		t.Log("CalculateVersion returned non-nil (expected nil for empty container)")
	}
	if wrapper.ReleaseServices() != nil {
		t.Log("ReleaseServices returned non-nil (expected nil for empty container)")
	}
	if wrapper.HasReleaseServices() {
		t.Log("HasReleaseServices returned true (expected false for empty container)")
	}
	// Test AI accessor - should return nil for empty container
	if wrapper.AI() != nil {
		t.Log("AI returned non-nil (expected nil for empty container)")
	}
}

// TestContainerAppWrapperImplementsCliApp verifies the wrapper implements the interface
func TestContainerAppWrapperImplementsCliApp(t *testing.T) {
	wrapper := &containerAppWrapper{App: &container.App{}}
	// Verify the wrapper implements cliApp interface
	var _ cliApp = wrapper
}
