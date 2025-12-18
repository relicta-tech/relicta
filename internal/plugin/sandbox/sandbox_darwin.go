//go:build darwin

package sandbox

import (
	"os/exec"
	"syscall"
)

// applyProcessLimits applies macOS-specific resource limits to the command.
// Uses setrlimit via SysProcAttr for memory limits.
// Note: macOS has more limited sandboxing options without using sandbox-exec.
func (s *Sandbox) applyProcessLimits(cmd *exec.Cmd) error {
	if s.capabilities == nil {
		return nil
	}

	// Initialize SysProcAttr if needed
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Create a new process group for signal handling
	cmd.SysProcAttr.Setpgid = true

	// Apply memory limit if configured
	// Note: macOS resource limits are less effective than Linux cgroups
	if s.capabilities.MaxMemoryMB > 0 {
		// Convert MB to bytes
		memBytes := uint64(s.capabilities.MaxMemoryMB) * 1024 * 1024

		// Prepare rlimit structure
		rlimit := syscall.Rlimit{
			Cur: memBytes,
			Max: memBytes,
		}

		// Set RLIMIT_DATA for heap limit (more reliable on macOS)
		// Note: RLIMIT_AS is often ignored on modern macOS
		// RLIMIT_RSS is not available on darwin
		_ = syscall.Setrlimit(syscall.RLIMIT_DATA, &rlimit)
	}

	// CPU limits on macOS are primarily handled by:
	// 1. Process priority (nice value) - requires privileges for child processes
	// 2. Timeout mechanism in the plugin manager (primary protection)
	// True CPU throttling requires launchd job control or sandbox-exec

	return nil
}
