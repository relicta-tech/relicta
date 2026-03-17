//go:build linux

package sandbox

import (
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// applyProcessLimits applies Linux-specific resource limits to the command.
// Sets up process group for proper signal handling.
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

	return nil
}

// ApplyResourceLimits applies resource limits to a running process via prlimit.
// This should be called immediately after the process starts.
// Note: There's an inherent race condition - the process may allocate memory
// before this is called. For security-critical limits, use cgroups instead.
func (s *Sandbox) ApplyResourceLimits(pid int) error {
	if s.capabilities == nil || pid <= 0 {
		return nil
	}

	var errs []error

	// Apply memory limit if configured
	if s.capabilities.MaxMemoryMB > 0 {
		// Convert MB to bytes
		memBytes := uint64(s.capabilities.MaxMemoryMB) * 1024 * 1024

		// Set virtual memory limit (RLIMIT_AS)
		// This limits the total virtual address space, including all allocations
		rlimit := unix.Rlimit{
			Cur: memBytes,
			Max: memBytes,
		}

		if err := unix.Prlimit(pid, unix.RLIMIT_AS, &rlimit, nil); err != nil {
			errs = append(errs, fmt.Errorf("failed to set memory limit: %w", err))
		}

		// Also set data segment limit (RLIMIT_DATA) for heap allocations
		if err := unix.Prlimit(pid, unix.RLIMIT_DATA, &rlimit, nil); err != nil {
			// This is less critical, just log it
			errs = append(errs, fmt.Errorf("failed to set data limit: %w", err))
		}
	}

	// Apply file descriptor limit if configured
	if s.capabilities.MaxFileDescriptors > 0 {
		rlimit := unix.Rlimit{
			Cur: uint64(s.capabilities.MaxFileDescriptors),
			Max: uint64(s.capabilities.MaxFileDescriptors),
		}

		if err := unix.Prlimit(pid, unix.RLIMIT_NOFILE, &rlimit, nil); err != nil {
			errs = append(errs, fmt.Errorf("failed to set file descriptor limit: %w", err))
		}
	}

	// Apply CPU time limit if configured (in seconds)
	// Note: This is CPU time, not wall-clock time
	if s.capabilities.MaxCPUSeconds > 0 {
		rlimit := unix.Rlimit{
			Cur: uint64(s.capabilities.MaxCPUSeconds),
			Max: uint64(s.capabilities.MaxCPUSeconds),
		}

		if err := unix.Prlimit(pid, unix.RLIMIT_CPU, &rlimit, nil); err != nil {
			errs = append(errs, fmt.Errorf("failed to set CPU time limit: %w", err))
		}
	}

	// Combine errors if any
	if len(errs) > 0 {
		return fmt.Errorf("resource limit errors: %v", errs)
	}

	return nil
}
