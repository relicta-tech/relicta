// Package sandbox: platform-specific security-notice surface.
//
// The sandbox subsystem reports its enforcement guarantees so callers
// (plugin manager, CLI startup, audit logs) can disclose them honestly to
// operators. Without this, marketing/docs claim a security boundary that
// the runtime cannot uphold — particularly on Apple Silicon where
// RLIMIT_AS is unenforced and we have no sandbox-exec profile.
//
// Two surfaces:
//   - `SecurityPosture`: structured assessment of platform capabilities.
//   - `SecurityNotice`: human-readable string for startup banners.
package sandbox

import (
	"runtime"
	"strings"
)

// EnforcementLevel ranks the strength of sandbox enforcement on this platform.
type EnforcementLevel string

const (
	// EnforcementStrict means the kernel can reliably enforce all configured
	// limits (memory, CPU, syscalls). Linux + cgroups falls here.
	EnforcementStrict EnforcementLevel = "strict"

	// EnforcementBestEffort means limits are advisory — they may be silently
	// ignored by the kernel. macOS Apple Silicon falls here.
	EnforcementBestEffort EnforcementLevel = "best_effort"

	// EnforcementNone means no kernel-level enforcement is configured at all.
	// Other (BSD/Windows fallback) builds fall here.
	EnforcementNone EnforcementLevel = "none"
)

// SecurityPosture reports the runtime sandbox guarantees on this platform.
type SecurityPosture struct {
	// Platform is the GOOS this build runs on.
	Platform string `json:"platform"`

	// Architecture is the GOARCH.
	Architecture string `json:"architecture"`

	// Level is the enforcement guarantee Relicta can offer.
	Level EnforcementLevel `json:"level"`

	// MemoryEnforced reports whether memory limits are reliably enforced.
	MemoryEnforced bool `json:"memoryEnforced"`

	// CPUEnforced reports whether CPU limits are reliably enforced.
	CPUEnforced bool `json:"cpuEnforced"`

	// FilesystemIsolated reports whether plugins are confined to a chroot
	// or sandbox-exec profile.
	FilesystemIsolated bool `json:"filesystemIsolated"`

	// SignatureVerification reports whether plugin signatures are verified
	// before load. Currently false everywhere (signing infrastructure not yet
	// shipped); flipped to true once signing lands.
	SignatureVerification bool `json:"signatureVerification"`

	// Caveats lists platform-specific gotchas the operator should know about.
	Caveats []string `json:"caveats,omitempty"`
}

// CurrentPosture returns the SecurityPosture for the running binary.
//
// Linux: cgroups make rlimits real → strict. macOS: rlimits are advisory on
// Apple Silicon, no sandbox-exec profile → best_effort. Other: none.
func CurrentPosture() SecurityPosture {
	p := SecurityPosture{
		Platform:              runtime.GOOS,
		Architecture:          runtime.GOARCH,
		SignatureVerification: false, // honest: not yet implemented
	}

	switch runtime.GOOS {
	case "linux":
		p.Level = EnforcementStrict
		p.MemoryEnforced = true
		p.CPUEnforced = true
		p.FilesystemIsolated = false // we don't currently chroot
		p.Caveats = []string{
			"Plugin signature verification is not yet implemented; only run trusted plugins.",
			"Filesystem isolation requires explicit chroot or namespace setup; not auto-applied.",
		}

	case "darwin":
		p.Level = EnforcementBestEffort
		p.MemoryEnforced = false
		p.CPUEnforced = false
		p.FilesystemIsolated = false
		p.Caveats = []string{
			"On Apple Silicon (arm64), RLIMIT_AS is silently ignored by the kernel.",
			"No sandbox-exec profile is generated; plugins run with the user's full filesystem access.",
			"CPU caps are not enforced on macOS without launchd job control.",
			"Primary protection is the per-plugin timeout in the plugin manager.",
			"Plugin signature verification is not yet implemented; only run trusted plugins.",
		}

	default:
		p.Level = EnforcementNone
		p.Caveats = []string{
			"This platform has no kernel-level resource enforcement implemented.",
			"Plugin signature verification is not yet implemented.",
		}
	}

	return p
}

// SecurityNotice returns a human-readable banner appropriate for startup
// or for `relicta plugin sandbox-status` output. It always lists the
// enforcement level and at least one caveat.
func SecurityNotice() string {
	p := CurrentPosture()
	var b strings.Builder
	b.WriteString("Plugin Sandbox Security Posture\n")
	b.WriteString("================================\n")
	b.WriteString("Platform: " + p.Platform + "/" + p.Architecture + "\n")
	b.WriteString("Enforcement: " + string(p.Level) + "\n")

	if p.MemoryEnforced {
		b.WriteString("  Memory limits: enforced\n")
	} else {
		b.WriteString("  Memory limits: best-effort (kernel may ignore)\n")
	}
	if p.CPUEnforced {
		b.WriteString("  CPU limits: enforced\n")
	} else {
		b.WriteString("  CPU limits: NOT enforced (use timeouts as primary protection)\n")
	}
	if p.SignatureVerification {
		b.WriteString("  Plugin signatures: verified before load\n")
	} else {
		b.WriteString("  Plugin signatures: NOT verified (only run trusted plugins)\n")
	}

	if len(p.Caveats) > 0 {
		b.WriteString("\nCaveats:\n")
		for _, c := range p.Caveats {
			b.WriteString("  - " + c + "\n")
		}
	}
	return b.String()
}

// RequireExplicitTrust reports whether the runtime should require operators
// to acknowledge the sandbox limitations before loading plugins. True when:
//   - plugin signature verification is not yet wired AND
//   - the platform offers less than strict enforcement
//
// Callers (plugin manager) should refuse to load plugins unless an explicit
// `--allow-untrusted-plugins` opt-in flag (or equivalent config) is set.
func RequireExplicitTrust() bool {
	p := CurrentPosture()
	if p.SignatureVerification {
		return false
	}
	return p.Level != EnforcementStrict
}
