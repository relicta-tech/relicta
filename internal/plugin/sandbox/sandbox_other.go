//go:build !linux && !darwin

package sandbox

import (
	"fmt"
	"os/exec"
	"runtime"
)

// applyProcessLimits rejects requested isolation on unsupported platforms.
// Resource limiting otherwise relies on the plugin manager's timeout mechanism.
func (s *Sandbox) applyProcessLimits(cmd *exec.Cmd) error {
	if s.capabilities != nil {
		if !s.capabilities.AllowNetwork {
			return fmt.Errorf("network isolation is not implemented on %s; enable network access or use an external sandbox runtime", runtime.GOOS)
		}
		if !s.capabilities.AllowFilesystem || len(s.capabilities.AllowedPaths) > 0 {
			return fmt.Errorf("filesystem isolation is not implemented on %s; enable filesystem access or use an external sandbox runtime", runtime.GOOS)
		}
	}
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
