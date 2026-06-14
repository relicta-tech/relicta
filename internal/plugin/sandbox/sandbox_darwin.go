//go:build darwin

package sandbox

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// applyProcessLimits applies macOS-specific resource limits to the command and,
// when capabilities restrict network or filesystem access, wraps the command in
// a sandbox-exec (seatbelt) profile for real kernel-enforced confinement.
//
// SECURITY NOTICE: macOS Enforcement
//
// Resource limits (memory/CPU) remain BEST-EFFORT on macOS:
//   - Memory limits (RLIMIT_DATA) may be ignored by modern macOS versions
//   - RLIMIT_AS (address space) is not enforced on Apple Silicon
//   - CPU limits cannot be enforced without launchd job control
//
// Network and filesystem confinement ARE enforced when the `sandbox-exec`
// binary is available: a seatbelt profile denies outbound network (keeping
// loopback + unix sockets so the gRPC plugin handshake still works) and confines
// writes to temp and explicitly-allowed paths. When `sandbox-exec` is missing,
// the command runs unconfined and a warning is logged.
//
// The timeout mechanism in the plugin manager remains the primary protection
// against runaway (CPU/memory) plugins on macOS.
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

	// Confine network/filesystem via a seatbelt profile when capabilities ask
	// for it. Best-effort: a missing sandbox-exec degrades to unconfined.
	s.wrapWithSeatbelt(cmd)

	return nil
}

// needsSeatbelt reports whether the capabilities call for a sandbox-exec
// profile: any network restriction, filesystem restriction, or path allowlist.
// When everything is permitted there is nothing to enforce and we skip wrapping.
func needsSeatbelt(caps *config.PluginCapabilities) bool {
	if caps == nil {
		return false
	}
	return !caps.AllowNetwork || !caps.AllowFilesystem || len(caps.AllowedPaths) > 0
}

// seatbeltWritablePaths returns the directories a confined plugin may still
// write to: the system temp locations plus any explicitly-allowed paths. Reads
// stay unrestricted (the profile is allow-by-default) so dynamic libraries and
// the plugin binary load normally.
func seatbeltWritablePaths(caps *config.PluginCapabilities) []string {
	paths := []string{"/private/tmp", "/private/var/folders"}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		paths = append(paths, filepath.Clean(tmp))
	}
	for _, p := range caps.AllowedPaths {
		if abs, err := filepath.Abs(p); err == nil {
			paths = append(paths, abs)
		}
	}
	return paths
}

// buildSeatbeltProfile renders an SBPL (sandbox profile language) document for
// the given capabilities. It is allow-by-default and then selectively denies, so
// the plugin keeps the broad read/exec access it needs to run while losing the
// specific privileges the operator withheld.
func buildSeatbeltProfile(caps *config.PluginCapabilities) string {
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(allow default)\n")

	if !caps.AllowNetwork {
		// Block external egress but keep loopback and unix sockets so the
		// go-plugin gRPC handshake (127.0.0.1 / unix socket) still connects.
		b.WriteString("(deny network-outbound)\n")
		b.WriteString("(allow network-outbound (remote ip \"localhost:*\"))\n")
		b.WriteString("(allow network-outbound (remote unix-socket))\n")
		b.WriteString("(allow network-bind (local ip \"localhost:*\"))\n")
	}

	if !caps.AllowFilesystem || len(caps.AllowedPaths) > 0 {
		b.WriteString("(deny file-write*)\n")
		for _, p := range seatbeltWritablePaths(caps) {
			fmt.Fprintf(&b, "(allow file-write* (subpath %q))\n", p)
		}
		b.WriteString("(allow file-write-data (literal \"/dev/null\") (literal \"/dev/stdout\") (literal \"/dev/stderr\"))\n")
	}

	return b.String()
}

// wrapWithSeatbelt rewrites cmd to run under `sandbox-exec -p <profile>` when the
// capabilities require confinement. If sandbox-exec is unavailable the command is
// left unchanged and a warning is logged (best-effort, consistent with the
// resource-limit posture on macOS).
func (s *Sandbox) wrapWithSeatbelt(cmd *exec.Cmd) {
	if !needsSeatbelt(s.capabilities) {
		return
	}

	sbexec, err := exec.LookPath("sandbox-exec")
	if err != nil {
		slog.Warn("sandbox-exec not found; plugin runs without seatbelt confinement",
			"plugin", s.name,
			"recommendation", "ensure sandbox-exec is on PATH, or run on Linux for cgroup enforcement")
		return
	}

	profile := buildSeatbeltProfile(s.capabilities)

	// sandbox-exec -p <profile> <origBinary> <origArgs...>
	origPath := cmd.Path
	newArgs := []string{sbexec, "-p", profile, origPath}
	if len(cmd.Args) > 1 {
		newArgs = append(newArgs, cmd.Args[1:]...)
	}
	cmd.Path = sbexec
	cmd.Args = newArgs

	slog.Info("plugin confined with sandbox-exec seatbelt profile",
		"plugin", s.name,
		"network_egress_denied", !s.capabilities.AllowNetwork,
		"filesystem_write_confined", !s.capabilities.AllowFilesystem || len(s.capabilities.AllowedPaths) > 0)
}

// ApplyResourceLimits is a no-op on macOS.
// macOS does not support prlimit(2) - resource limits must be set before process starts
// via setrlimit in applyProcessLimits above, which has limited effectiveness.
// The timeout mechanism in the plugin manager is the primary protection on macOS.
func (s *Sandbox) ApplyResourceLimits(pid int) error {
	// On macOS, resource limits are applied at process start time via setrlimit.
	// There's no prlimit equivalent to modify limits of a running process.
	// This is intentionally a no-op - limits were already applied in applyProcessLimits.
	return nil
}
