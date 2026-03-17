// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

func TestHealthCommand_Configuration(t *testing.T) {
	if healthCmd == nil {
		t.Fatal("healthCmd is nil")
	}
	if healthCmd.Use != "health" {
		t.Errorf("healthCmd.Use = %v, want health", healthCmd.Use)
	}
	if healthCmd.RunE == nil {
		t.Error("healthCmd.RunE is nil")
	}
}

func TestHealthCommand_Description(t *testing.T) {
	if healthCmd.Short == "" {
		t.Error("health command should have a short description")
	}
	if healthCmd.Long == "" {
		t.Error("health command should have a long description")
	}
}

func TestHealthStatus_Values(t *testing.T) {
	tests := []struct {
		name   string
		status HealthStatus
		want   string
	}{
		{"healthy status", HealthStatusHealthy, "healthy"},
		{"degraded status", HealthStatusDegraded, "degraded"},
		{"unhealthy status", HealthStatusUnhealthy, "unhealthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("HealthStatus = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

func TestHealthReport_Structure(t *testing.T) {
	report := &HealthReport{
		Status:      HealthStatusHealthy,
		Components:  []ComponentHealth{},
		Environment: map[string]string{},
	}

	if report.Status != HealthStatusHealthy {
		t.Errorf("Status = %v, want %v", report.Status, HealthStatusHealthy)
	}
	if report.Components == nil {
		t.Error("Components should not be nil")
	}
	if report.Environment == nil {
		t.Error("Environment should not be nil")
	}
}

func TestComponentHealth_Creation(t *testing.T) {
	component := ComponentHealth{
		Name:    "test-component",
		Status:  HealthStatusHealthy,
		Message: "All systems operational",
		Details: map[string]string{
			"version": "1.0.0",
		},
	}

	if component.Name != "test-component" {
		t.Errorf("Name = %v, want test-component", component.Name)
	}
	if component.Status != HealthStatusHealthy {
		t.Errorf("Status = %v, want %v", component.Status, HealthStatusHealthy)
	}
	if component.Message == "" {
		t.Error("Message should not be empty")
	}
	if len(component.Details) != 1 {
		t.Errorf("Details length = %d, want 1", len(component.Details))
	}
}

func TestComponentHealth_WithoutDetails(t *testing.T) {
	component := ComponentHealth{
		Name:    "simple-component",
		Status:  HealthStatusHealthy,
		Message: "OK",
	}

	if component.Details != nil {
		t.Errorf("Details should be nil when not set, got %v", component.Details)
	}
}

func TestHealthReport_MultipleComponents(t *testing.T) {
	report := &HealthReport{
		Status: HealthStatusHealthy,
		Components: []ComponentHealth{
			{
				Name:    "git",
				Status:  HealthStatusHealthy,
				Message: "Git is available",
			},
			{
				Name:    "repository",
				Status:  HealthStatusHealthy,
				Message: "Repository is valid",
			},
		},
		Environment: map[string]string{
			"OS": "linux",
		},
	}

	if len(report.Components) != 2 {
		t.Errorf("Components length = %d, want 2", len(report.Components))
	}
	if len(report.Environment) != 1 {
		t.Errorf("Environment length = %d, want 1", len(report.Environment))
	}
}

func TestRunHealthJSONProducesReport(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	origOutput := outputJSON
	outputJSON = true
	defer func() { outputJSON = origOutput }()

	origExitHook := exitWithHealthStatusHook
	exitWithHealthStatusHook = func(status HealthStatus) error {
		return nil
	}
	defer func() { exitWithHealthStatusHook = origExitHook }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runHealth(cmd, nil); err != nil {
		t.Fatalf("runHealth failed: %v", err)
	}
}

func TestHealthStatus_StringRepresentation(t *testing.T) {
	// Test that HealthStatus can be used as a string
	status := HealthStatusHealthy
	if status != "healthy" {
		t.Errorf("HealthStatus string value = %v, want healthy", status)
	}

	// Test string comparison
	statusStr := string(status)
	if statusStr != "healthy" {
		t.Errorf("string(HealthStatus) = %v, want healthy", statusStr)
	}
}
