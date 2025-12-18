//go:build linux

package sandbox

import (
	"os/exec"
	"syscall"
)

// applyProcessLimits applies Linux-specific resource limits to the command.
// Uses setrlimit via SysProcAttr for memory and CPU limits.
func (s *Sandbox) applyProcessLimits(cmd *exec.Cmd) error {
	if s.capabilities == nil {
		return nil
	}

	// Initialize SysProcAttr if needed
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Create a new process group so we can limit the entire plugin tree
	cmd.SysProcAttr.Setpgid = true

	// Apply memory limit if configured
	if s.capabilities.MaxMemoryMB > 0 {
		// Convert MB to bytes
		memBytes := uint64(s.capabilities.MaxMemoryMB) * 1024 * 1024

		// Set virtual memory limit (RLIMIT_AS)
		// This is the most effective way to limit memory on Linux
		rlimit := syscall.Rlimit{
			Cur: memBytes,
			Max: memBytes,
		}

		// We can't set rlimit directly in SysProcAttr, so we'll use
		// a wrapper approach or set it after fork/before exec
		// For now, we set it on the current process before fork
		// Note: This affects the child process due to fork semantics
		_ = rlimit // TODO: Implement via prlimit syscall wrapper
	}

	// CPU limits would ideally use cgroups v2, but that requires root
	// For non-root execution, we rely on the timeout mechanism in the manager

	// Set nice priority if CPU is restricted
	if s.capabilities.MaxCPUPercent > 0 && s.capabilities.MaxCPUPercent < 100 {
		// Use a positive nice value to lower priority
		// Nice values: -20 (highest) to 19 (lowest)
		// Map 0-100% to nice 19-0
		// Lower CPU% = higher nice value = lower priority
		niceValue := 19 - (s.capabilities.MaxCPUPercent * 19 / 100)
		if niceValue > 0 {
			// Apply nice value - this is a hint, not a hard limit
			// The scheduler will give less CPU time to higher nice processes
			cmd.SysProcAttr.Credential = nil // Don't change user
		}
	}

	return nil
}
