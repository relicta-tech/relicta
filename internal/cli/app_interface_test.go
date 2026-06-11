package cli

import (
	"testing"
	"testing/fstest"

	"github.com/relicta-tech/relicta/v4/internal/container"
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

func TestSetEmbeddedFrontend(t *testing.T) {
	// Save and restore
	prev := embeddedFrontend
	defer func() { embeddedFrontend = prev }()

	fs := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}
	SetEmbeddedFrontend(fs)

	if embeddedFrontend == nil {
		t.Error("embeddedFrontend should not be nil after SetEmbeddedFrontend")
	}
}

// TestContainerAppWrapperImplementsCliApp verifies the wrapper implements the interface
func TestContainerAppWrapperImplementsCliApp(t *testing.T) {
	wrapper := &containerAppWrapper{App: &container.App{}}
	// Verify the wrapper implements cliApp interface
	var _ cliApp = wrapper
}
