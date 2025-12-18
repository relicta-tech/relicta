// Package sandbox provides security isolation for plugin execution.
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/relicta-tech/relicta/internal/config"
)

// Sandbox provides security isolation for plugin processes.
type Sandbox struct {
	name         string
	capabilities *config.PluginCapabilities
}

// New creates a new sandbox with the given capabilities.
// If capabilities is nil, default secure settings are used.
func New(name string, caps *config.PluginCapabilities) *Sandbox {
	if caps == nil {
		// Default: restrictive capabilities
		caps = &config.PluginCapabilities{
			AllowNetwork:    true,  // Plugins typically need network for APIs
			AllowFilesystem: false, // Restricted by default
			AllowEnvRead:    true,  // Typically need to read config from env
			MaxMemoryMB:     512,   // 512MB default
			MaxCPUPercent:   50,    // 50% CPU cap
		}
	}
	return &Sandbox{
		name:         name,
		capabilities: caps,
	}
}

// PrepareCommand configures the exec.Cmd with sandbox restrictions.
// This is the main entry point - it calls OS-specific implementations.
func (s *Sandbox) PrepareCommand(ctx context.Context, cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	// Filter environment variables based on capabilities
	cmd.Env = s.filterEnv(os.Environ())

	// Apply OS-specific process limits (ulimit, cgroups, etc.)
	if err := s.applyProcessLimits(cmd); err != nil {
		// Log warning but don't fail - sandboxing is best-effort on some platforms
		// The error is returned for the caller to decide how to handle
		return fmt.Errorf("failed to apply process limits: %w", err)
	}

	return nil
}

// filterEnv filters environment variables based on capabilities.
func (s *Sandbox) filterEnv(environ []string) []string {
	if s.capabilities == nil {
		return environ
	}

	// If env reading is fully allowed without restrictions, pass all
	if s.capabilities.AllowEnvRead && len(s.capabilities.AllowedEnvVars) == 0 {
		return environ
	}

	// If env reading is disabled, only pass essential vars
	if !s.capabilities.AllowEnvRead {
		return s.essentialEnvVars(environ)
	}

	// Filter to only allowed vars plus essential ones
	allowed := make(map[string]bool)
	for _, v := range s.capabilities.AllowedEnvVars {
		allowed[v] = true
	}

	// Essential vars always allowed
	for _, v := range []string{"PATH", "HOME", "USER", "SHELL", "LANG", "LC_ALL", "TZ", "TMPDIR"} {
		allowed[v] = true
	}

	filtered := make([]string, 0, len(environ))
	for _, env := range environ {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) < 1 {
			continue
		}
		name := parts[0]
		if allowed[name] {
			filtered = append(filtered, env)
		}
	}

	return filtered
}

// essentialEnvVars returns only essential environment variables.
func (s *Sandbox) essentialEnvVars(environ []string) []string {
	essential := map[string]bool{
		"PATH":   true,
		"HOME":   true,
		"USER":   true,
		"SHELL":  true,
		"LANG":   true,
		"LC_ALL": true,
		"TZ":     true,
		"TMPDIR": true,
	}

	filtered := make([]string, 0, len(essential))
	for _, env := range environ {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) < 1 {
			continue
		}
		if essential[parts[0]] {
			filtered = append(filtered, env)
		}
	}

	return filtered
}

// Name returns the sandbox name (plugin name).
func (s *Sandbox) Name() string {
	return s.name
}

// Capabilities returns the sandbox capabilities.
func (s *Sandbox) Capabilities() *config.PluginCapabilities {
	return s.capabilities
}
