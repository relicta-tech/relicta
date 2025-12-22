package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/container"
)

func TestContainerAppWrapperAccessors(t *testing.T) {
	wrapper := &containerAppWrapper{App: &container.App{}}

	if wrapper.PlanRelease() != nil {
		t.Log("PlanRelease returned non-nil (expected nil for empty container)")
	}
	if wrapper.GenerateNotes() != nil {
		t.Log("GenerateNotes returned non-nil (expected nil for empty container)")
	}
	if wrapper.ApproveRelease() != nil {
		t.Log("ApproveRelease returned non-nil (expected nil for empty container)")
	}
	if wrapper.PublishRelease() != nil {
		t.Log("PublishRelease returned non-nil (expected nil for empty container)")
	}
	if wrapper.CalculateVersion() != nil {
		t.Log("CalculateVersion returned non-nil (expected nil for empty container)")
	}
	if wrapper.SetVersion() != nil {
		t.Log("SetVersion returned non-nil (expected nil for empty container)")
	}
}
