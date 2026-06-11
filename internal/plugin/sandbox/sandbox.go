// Package sandbox provides security isolation for plugin execution.
package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// ViolationType represents the type of capability violation.
type ViolationType string

const (
	ViolationFilesystem ViolationType = "filesystem"
	ViolationNetwork    ViolationType = "network"
	ViolationEnvVar     ViolationType = "env_var"
	ViolationResource   ViolationType = "resource"
)

// Violation represents a security policy violation by a plugin.
type Violation struct {
	Type        ViolationType `json:"type"`
	Plugin      string        `json:"plugin"`
	Description string        `json:"description"`
	Path        string        `json:"path,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	Blocked     bool          `json:"blocked"`
}

// Sandbox provides security isolation for plugin processes.
type Sandbox struct {
	name         string
	capabilities *config.PluginCapabilities

	// Violation tracking
	mu         sync.Mutex
	violations []Violation
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

// authEnvVars are authentication tokens that plugins commonly need.
// These are passed through when AllowEnvRead is true.
var authEnvVars = []string{
	// GitHub
	"GITHUB_TOKEN", "GH_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
	// GitLab
	"GITLAB_TOKEN", "GL_TOKEN", "GITLAB_PRIVATE_TOKEN",
	// Slack
	"SLACK_TOKEN", "SLACK_WEBHOOK_URL", "SLACK_BOT_TOKEN",
	// Discord
	"DISCORD_TOKEN", "DISCORD_WEBHOOK_URL",
	// Jira
	"JIRA_TOKEN", "JIRA_API_TOKEN", "JIRA_USER", "JIRA_URL",
	// Package registries
	"NPM_TOKEN", "PYPI_TOKEN", "NUGET_API_KEY",
	// OpenAI (for AI-enhanced plugins)
	"OPENAI_API_KEY",
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

	// Filter to only allowed vars plus essential ones plus auth vars
	allowed := make(map[string]bool)
	for _, v := range s.capabilities.AllowedEnvVars {
		allowed[v] = true
	}

	// Essential vars always allowed
	for _, v := range []string{"PATH", "HOME", "USER", "SHELL", "LANG", "LC_ALL", "TZ", "TMPDIR"} {
		allowed[v] = true
	}

	// Auth vars are allowed when env reading is enabled
	for _, v := range authEnvVars {
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

// IsFilesystemAllowed returns whether filesystem access is permitted.
func (s *Sandbox) IsFilesystemAllowed() bool {
	if s.capabilities == nil {
		return false // Default: restrictive
	}
	return s.capabilities.AllowFilesystem
}

// IsNetworkAllowed returns whether network access is permitted.
func (s *Sandbox) IsNetworkAllowed() bool {
	if s.capabilities == nil {
		return true // Default: allow for API access
	}
	return s.capabilities.AllowNetwork
}

// IsPathAllowed checks if access to a specific path is permitted.
// Returns true if:
// - Filesystem access is allowed AND (no path restrictions OR path is in allowed list)
func (s *Sandbox) IsPathAllowed(path string) bool {
	if !s.IsFilesystemAllowed() {
		return false
	}

	if s.capabilities == nil || len(s.capabilities.AllowedPaths) == 0 {
		// No path restrictions configured
		return true
	}

	// Clean and resolve the path
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}

	// Check if path is within any allowed path
	for _, allowed := range s.capabilities.AllowedPaths {
		allowedAbs, err := filepath.Abs(filepath.Clean(allowed))
		if err != nil {
			continue
		}

		// Check if cleanPath is within allowedAbs
		rel, err := filepath.Rel(allowedAbs, cleanPath)
		if err != nil {
			continue
		}

		// If relative path doesn't start with "..", it's within the allowed path
		if !strings.HasPrefix(rel, "..") {
			return true
		}
	}

	return false
}

// CheckFilesystemAccess validates filesystem access and records violations.
// Returns nil if access is allowed, error if blocked.
func (s *Sandbox) CheckFilesystemAccess(path string) error {
	if s.IsPathAllowed(path) {
		return nil
	}

	violation := Violation{
		Type:        ViolationFilesystem,
		Plugin:      s.name,
		Description: "filesystem access denied",
		Path:        path,
		Timestamp:   time.Now(),
		Blocked:     true,
	}
	s.recordViolation(violation)

	slog.Warn("plugin filesystem access blocked",
		"plugin", s.name,
		"path", path,
		"allowed_paths", s.capabilities.AllowedPaths)

	return fmt.Errorf("filesystem access denied for plugin %s: path %s not in allowed paths", s.name, path)
}

// CheckNetworkAccess validates network access and records violations.
// Returns nil if access is allowed, error if blocked.
func (s *Sandbox) CheckNetworkAccess(host string, port int) error {
	if s.IsNetworkAllowed() {
		return nil
	}

	violation := Violation{
		Type:        ViolationNetwork,
		Plugin:      s.name,
		Description: fmt.Sprintf("network access denied to %s:%d", host, port),
		Timestamp:   time.Now(),
		Blocked:     true,
	}
	s.recordViolation(violation)

	slog.Warn("plugin network access blocked",
		"plugin", s.name,
		"host", host,
		"port", port)

	return fmt.Errorf("network access denied for plugin %s: allow_network is false", s.name)
}

// recordViolation adds a violation to the sandbox's violation log.
func (s *Sandbox) recordViolation(v Violation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.violations = append(s.violations, v)
}

// GetViolations returns all recorded violations.
func (s *Sandbox) GetViolations() []Violation {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Return a copy to prevent external modification
	result := make([]Violation, len(s.violations))
	copy(result, s.violations)
	return result
}

// HasViolations returns true if any violations have been recorded.
func (s *Sandbox) HasViolations() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.violations) > 0
}

// ClearViolations clears all recorded violations.
func (s *Sandbox) ClearViolations() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.violations = nil
}

// LogCapabilitySummary logs the sandbox capabilities for auditing.
func (s *Sandbox) LogCapabilitySummary() {
	if s.capabilities == nil {
		slog.Info("plugin sandbox using default capabilities", "plugin", s.name)
		return
	}

	slog.Info("plugin sandbox capabilities",
		"plugin", s.name,
		"allow_network", s.capabilities.AllowNetwork,
		"allow_filesystem", s.capabilities.AllowFilesystem,
		"allowed_paths", s.capabilities.AllowedPaths,
		"allow_env_read", s.capabilities.AllowEnvRead,
		"max_memory_mb", s.capabilities.MaxMemoryMB,
		"max_cpu_percent", s.capabilities.MaxCPUPercent,
		"max_file_descriptors", s.capabilities.MaxFileDescriptors,
		"max_cpu_seconds", s.capabilities.MaxCPUSeconds)
}
