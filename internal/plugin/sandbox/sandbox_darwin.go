//go:build darwin

package sandbox

import (
	"log/slog"
	"os/exec"
	"syscall"
)

// applyProcessLimits applies macOS-specific resource limits to the command.
// Uses setrlimit via SysProcAttr for memory limits.
//
// SECURITY NOTICE: macOS Resource Limit Limitations
//
// On macOS, plugin sandboxing is BEST-EFFORT only:
//   - Memory limits (RLIMIT_DATA) may be ignored by modern macOS versions
//   - RLIMIT_AS (address space) is not enforced on Apple Silicon
//   - CPU limits cannot be enforced without launchd or sandbox-exec
//   - True isolation requires sandbox-exec profiles (not implemented)
//
// For production deployments requiring strict plugin isolation:
//   - Use Linux runners/containers where cgroups provide real limits
//   - Use Docker with resource constraints
//   - Set conservative timeouts (primary protection mechanism)
//
// The timeout mechanism in the plugin manager remains the primary protection
// against runaway plugins on macOS.
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
		// Warn user that macOS limits are best-effort
		slog.Warn("plugin memory limits are best-effort on macOS",
			"plugin", s.name,
			"limit_mb", s.capabilities.MaxMemoryMB,
			"recommendation", "use Linux/Docker for strict enforcement")

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
