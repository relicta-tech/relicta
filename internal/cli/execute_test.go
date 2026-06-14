// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bytes"
	"testing"
)

// runRootWithArgs points the global rootCmd at the given args and discards its
// output for the rest of the test, restoring both on cleanup. "--help" exits
// cleanly (cobra returns nil and skips RunE/PersistentPreRunE), so it exercises
// the Execute wrappers without side effects or os.Exit.
func runRootWithArgs(t *testing.T, args []string) {
	t.Helper()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
}

func TestExecute(t *testing.T) {
	runRootWithArgs(t, []string{"--help"})
	if err := Execute(); err != nil {
		t.Fatalf("Execute() with --help error = %v", err)
	}
}

func TestExecuteContext(t *testing.T) {
	runRootWithArgs(t, []string{"--help"})
	if err := ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() with --help error = %v", err)
	}
}

func TestHealthStatusExitCode(t *testing.T) {
	tests := []struct {
		name   string
		status HealthStatus
		want   int
	}{
		{"healthy is zero", HealthStatusHealthy, 0},
		{"degraded is one", HealthStatusDegraded, 1},
		{"unhealthy is two", HealthStatusUnhealthy, 2},
		{"unknown status is zero", HealthStatus("bogus"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthStatusExitCode(tt.status); got != tt.want {
				t.Fatalf("healthStatusExitCode(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}

// withCapturedHealthExit replaces the exit hook with one that records the status
// instead of calling os.Exit, returning a pointer to the captured value and
// restoring the hook on cleanup.
func withCapturedHealthExit(t *testing.T) *HealthStatus {
	t.Helper()
	var captured HealthStatus = "<none>"
	orig := exitWithHealthStatusHook
	exitWithHealthStatusHook = func(status HealthStatus) error {
		captured = status
		return nil
	}
	t.Cleanup(func() { exitWithHealthStatusHook = orig })
	return &captured
}

func TestExitWithHealthStatus_DelegatesToHook(t *testing.T) {
	captured := withCapturedHealthExit(t)

	if err := exitWithHealthStatus(HealthStatusUnhealthy); err != nil {
		t.Fatalf("exitWithHealthStatus() error = %v", err)
	}
	if *captured != HealthStatusUnhealthy {
		t.Fatalf("hook received %q, want %q", *captured, HealthStatusUnhealthy)
	}
}
