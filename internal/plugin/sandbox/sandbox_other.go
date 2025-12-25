//go:build !linux && !darwin

package sandbox

import (
	"os/exec"
)

// applyProcessLimits is a no-op on unsupported platforms.
// Resource limiting relies on the plugin manager's timeout mechanism.
func (s *Sandbox) applyProcessLimits(cmd *exec.Cmd) error {
	// On Windows and other platforms, we rely on:
	// 1. Environment variable filtering (done in sandbox.go)
	// 2. Timeout mechanism in the plugin manager
	// 3. The OS's built-in process isolation

	// True sandboxing on Windows would require:
	// - Job objects for resource limits
	// - AppContainer for isolation
	// These require elevated privileges and complex setup

	return nil
}

// ApplyResourceLimits is a no-op on unsupported platforms.
// Windows and other platforms don't have a prlimit equivalent.
// Resource limiting relies on the plugin manager's timeout mechanism.
func (s *Sandbox) ApplyResourceLimits(pid int) error {
	// On Windows and other platforms, there's no prlimit equivalent.
	// True resource limiting would require Job objects on Windows.
	return nil
}
