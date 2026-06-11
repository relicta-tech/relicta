package sandbox

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name         string
		pluginName   string
		capabilities *config.PluginCapabilities
		wantDefaults bool
	}{
		{
			name:         "with nil capabilities uses defaults",
			pluginName:   "test-plugin",
			capabilities: nil,
			wantDefaults: true,
		},
		{
			name:       "with custom capabilities",
			pluginName: "custom-plugin",
			capabilities: &config.PluginCapabilities{
				AllowNetwork:    false,
				AllowFilesystem: true,
				MaxMemoryMB:     1024,
				MaxCPUPercent:   75,
			},
			wantDefaults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New(tt.pluginName, tt.capabilities)

			if sb.Name() != tt.pluginName {
				t.Errorf("Name() = %v, want %v", sb.Name(), tt.pluginName)
			}

			caps := sb.Capabilities()
			if caps == nil {
				t.Fatal("Capabilities() returned nil")
			}

			if tt.wantDefaults {
				// Check default values
				if !caps.AllowNetwork {
					t.Error("default AllowNetwork should be true")
				}
				if caps.AllowFilesystem {
					t.Error("default AllowFilesystem should be false")
				}
				if caps.MaxMemoryMB != 512 {
					t.Errorf("default MaxMemoryMB = %d, want 512", caps.MaxMemoryMB)
				}
				if caps.MaxCPUPercent != 50 {
					t.Errorf("default MaxCPUPercent = %d, want 50", caps.MaxCPUPercent)
				}
			} else {
				// Check custom values
				if caps.AllowNetwork != tt.capabilities.AllowNetwork {
					t.Errorf("AllowNetwork = %v, want %v", caps.AllowNetwork, tt.capabilities.AllowNetwork)
				}
				if caps.AllowFilesystem != tt.capabilities.AllowFilesystem {
					t.Errorf("AllowFilesystem = %v, want %v", caps.AllowFilesystem, tt.capabilities.AllowFilesystem)
				}
			}
		})
	}
}

func TestSandbox_filterEnv(t *testing.T) {
	tests := []struct {
		name     string
		caps     *config.PluginCapabilities
		environ  []string
		wantKeys []string
		denyKeys []string
	}{
		{
			name:     "nil capabilities passes all",
			caps:     nil,
			environ:  []string{"FOO=bar", "SECRET=value", "PATH=/usr/bin"},
			wantKeys: []string{"FOO", "SECRET", "PATH"},
		},
		{
			name: "allow all env without restrictions",
			caps: &config.PluginCapabilities{
				AllowEnvRead:   true,
				AllowedEnvVars: nil, // Empty = allow all
			},
			environ:  []string{"FOO=bar", "SECRET=value", "PATH=/usr/bin"},
			wantKeys: []string{"FOO", "SECRET", "PATH"},
		},
		{
			name: "deny env read - only essential vars",
			caps: &config.PluginCapabilities{
				AllowEnvRead: false,
			},
			environ:  []string{"FOO=bar", "SECRET=value", "PATH=/usr/bin", "HOME=/home/user"},
			wantKeys: []string{"PATH", "HOME"},
			denyKeys: []string{"FOO", "SECRET"},
		},
		{
			name: "restricted to specific vars",
			caps: &config.PluginCapabilities{
				AllowEnvRead:   true,
				AllowedEnvVars: []string{"GITHUB_TOKEN", "CI"},
			},
			environ:  []string{"GITHUB_TOKEN=xxx", "CI=true", "SECRET=bad", "PATH=/bin"},
			wantKeys: []string{"GITHUB_TOKEN", "CI", "PATH"}, // PATH is essential
			denyKeys: []string{"SECRET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &Sandbox{capabilities: tt.caps}
			result := sb.filterEnv(tt.environ)

			// Build map for easy lookup
			resultMap := make(map[string]bool)
			for _, env := range result {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) > 0 {
					resultMap[parts[0]] = true
				}
			}

			// Check wanted keys are present
			for _, key := range tt.wantKeys {
				if !resultMap[key] {
					t.Errorf("expected %s to be in filtered env, but it wasn't", key)
				}
			}

			// Check denied keys are absent
			for _, key := range tt.denyKeys {
				if resultMap[key] {
					t.Errorf("expected %s to be filtered out, but it was present", key)
				}
			}
		})
	}
}

func TestSandbox_essentialEnvVars(t *testing.T) {
	sb := &Sandbox{}
	environ := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"USER=testuser",
		"SECRET=bad",
		"API_KEY=xxx",
		"LANG=en_US.UTF-8",
	}

	result := sb.essentialEnvVars(environ)

	// Build map
	resultMap := make(map[string]bool)
	for _, env := range result {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) > 0 {
			resultMap[parts[0]] = true
		}
	}

	// Essential vars should be present
	essentials := []string{"PATH", "HOME", "USER", "LANG"}
	for _, key := range essentials {
		if !resultMap[key] {
			t.Errorf("expected essential var %s to be present", key)
		}
	}

	// Non-essential vars should be absent
	nonEssentials := []string{"SECRET", "API_KEY"}
	for _, key := range nonEssentials {
		if resultMap[key] {
			t.Errorf("expected non-essential var %s to be filtered out", key)
		}
	}
}

func TestSandbox_PrepareCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *exec.Cmd
		wantErr bool
	}{
		{
			name:    "nil command returns error",
			cmd:     nil,
			wantErr: true,
		},
		{
			name:    "valid command succeeds",
			cmd:     exec.Command("echo", "test"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New("test", nil)
			err := sb.PrepareCommand(context.Background(), tt.cmd)

			if (err != nil) != tt.wantErr {
				t.Errorf("PrepareCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If command was prepared, check env was set
			if !tt.wantErr && tt.cmd != nil {
				if tt.cmd.Env == nil {
					t.Error("expected Env to be set after PrepareCommand")
				}
			}
		})
	}
}

func TestSandbox_PrepareCommand_EnvFiltering(t *testing.T) {
	// Set a test env var temporarily
	t.Setenv("TEST_SECRET", "should-be-filtered")
	t.Setenv("PATH", "/usr/bin") // Ensure PATH is set

	sb := New("test", &config.PluginCapabilities{
		AllowEnvRead:   true,
		AllowedEnvVars: []string{"ALLOWED_VAR"},
	})

	cmd := exec.Command("echo", "test")
	err := sb.PrepareCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("PrepareCommand() error = %v", err)
	}

	// Check that env was filtered
	envMap := make(map[string]bool)
	for _, env := range cmd.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) > 0 {
			envMap[parts[0]] = true
		}
	}

	// PATH should be present (essential)
	if !envMap["PATH"] {
		t.Error("PATH should be present in filtered env")
	}

	// TEST_SECRET should be filtered out (not in allowed list)
	if envMap["TEST_SECRET"] {
		t.Error("TEST_SECRET should be filtered out")
	}
}

func TestSandbox_Name(t *testing.T) {
	sb := New("my-plugin", nil)
	if sb.Name() != "my-plugin" {
		t.Errorf("Name() = %v, want my-plugin", sb.Name())
	}
}

func TestSandbox_Capabilities(t *testing.T) {
	caps := &config.PluginCapabilities{
		AllowNetwork: true,
		MaxMemoryMB:  256,
	}
	sb := New("test", caps)

	got := sb.Capabilities()
	if got.AllowNetwork != caps.AllowNetwork {
		t.Errorf("Capabilities().AllowNetwork = %v, want %v", got.AllowNetwork, caps.AllowNetwork)
	}
	if got.MaxMemoryMB != caps.MaxMemoryMB {
		t.Errorf("Capabilities().MaxMemoryMB = %v, want %v", got.MaxMemoryMB, caps.MaxMemoryMB)
	}
}

func TestSandbox_IsFilesystemAllowed(t *testing.T) {
	tests := []struct {
		name string
		caps *config.PluginCapabilities
		want bool
	}{
		{
			name: "nil capabilities - restrictive default",
			caps: nil,
			want: false,
		},
		{
			name: "filesystem explicitly allowed",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
			},
			want: true,
		},
		{
			name: "filesystem explicitly denied",
			caps: &config.PluginCapabilities{
				AllowFilesystem: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New("test-plugin", tt.caps)
			// Override capabilities for nil test
			if tt.caps == nil {
				sb.capabilities = nil
			}
			got := sb.IsFilesystemAllowed()
			if got != tt.want {
				t.Errorf("IsFilesystemAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSandbox_IsNetworkAllowed(t *testing.T) {
	tests := []struct {
		name string
		caps *config.PluginCapabilities
		want bool
	}{
		{
			name: "nil capabilities - permissive default for network",
			caps: nil,
			want: true,
		},
		{
			name: "network explicitly allowed",
			caps: &config.PluginCapabilities{
				AllowNetwork: true,
			},
			want: true,
		},
		{
			name: "network explicitly denied",
			caps: &config.PluginCapabilities{
				AllowNetwork: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New("test-plugin", tt.caps)
			// Override capabilities for nil test
			if tt.caps == nil {
				sb.capabilities = nil
			}
			got := sb.IsNetworkAllowed()
			if got != tt.want {
				t.Errorf("IsNetworkAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSandbox_IsPathAllowed(t *testing.T) {
	tests := []struct {
		name string
		caps *config.PluginCapabilities
		path string
		want bool
	}{
		{
			name: "filesystem disabled - all paths denied",
			caps: &config.PluginCapabilities{
				AllowFilesystem: false,
			},
			path: "/any/path",
			want: false,
		},
		{
			name: "filesystem enabled, no path restrictions - all allowed",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    nil,
			},
			path: "/any/path",
			want: true,
		},
		{
			name: "filesystem enabled, path within allowed",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    []string{"/home/user/project"},
			},
			path: "/home/user/project/file.txt",
			want: true,
		},
		{
			name: "filesystem enabled, path outside allowed",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    []string{"/home/user/project"},
			},
			path: "/etc/passwd",
			want: false,
		},
		{
			name: "filesystem enabled, exact path match",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    []string{"/home/user/project"},
			},
			path: "/home/user/project",
			want: true,
		},
		{
			name: "filesystem enabled, path traversal attempt",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    []string{"/home/user/project"},
			},
			path: "/home/user/project/../../../etc/passwd",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New("test-plugin", tt.caps)
			got := sb.IsPathAllowed(tt.path)
			if got != tt.want {
				t.Errorf("IsPathAllowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSandbox_CheckFilesystemAccess(t *testing.T) {
	tests := []struct {
		name          string
		caps          *config.PluginCapabilities
		path          string
		wantErr       bool
		wantViolation bool
	}{
		{
			name: "allowed path - no error, no violation",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    []string{"/home/user/project"},
			},
			path:          "/home/user/project/file.txt",
			wantErr:       false,
			wantViolation: false,
		},
		{
			name: "denied path - error and violation recorded",
			caps: &config.PluginCapabilities{
				AllowFilesystem: true,
				AllowedPaths:    []string{"/home/user/project"},
			},
			path:          "/etc/passwd",
			wantErr:       true,
			wantViolation: true,
		},
		{
			name: "filesystem disabled - error and violation",
			caps: &config.PluginCapabilities{
				AllowFilesystem: false,
			},
			path:          "/any/path",
			wantErr:       true,
			wantViolation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New("test-plugin", tt.caps)
			err := sb.CheckFilesystemAccess(tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckFilesystemAccess() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantViolation {
				if !sb.HasViolations() {
					t.Error("expected violation to be recorded")
				}
				violations := sb.GetViolations()
				if len(violations) == 0 {
					t.Error("expected at least one violation")
				} else {
					v := violations[0]
					if v.Type != ViolationFilesystem {
						t.Errorf("violation type = %v, want %v", v.Type, ViolationFilesystem)
					}
					if v.Plugin != "test-plugin" {
						t.Errorf("violation plugin = %v, want test-plugin", v.Plugin)
					}
					if v.Path != tt.path {
						t.Errorf("violation path = %v, want %v", v.Path, tt.path)
					}
					if !v.Blocked {
						t.Error("violation should be marked as blocked")
					}
				}
			} else {
				if sb.HasViolations() {
					t.Error("unexpected violation recorded")
				}
			}
		})
	}
}

func TestSandbox_CheckNetworkAccess(t *testing.T) {
	tests := []struct {
		name          string
		caps          *config.PluginCapabilities
		host          string
		port          int
		wantErr       bool
		wantViolation bool
	}{
		{
			name: "network allowed - no error, no violation",
			caps: &config.PluginCapabilities{
				AllowNetwork: true,
			},
			host:          "api.example.com",
			port:          443,
			wantErr:       false,
			wantViolation: false,
		},
		{
			name: "network denied - error and violation recorded",
			caps: &config.PluginCapabilities{
				AllowNetwork: false,
			},
			host:          "api.example.com",
			port:          443,
			wantErr:       true,
			wantViolation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := New("test-plugin", tt.caps)
			err := sb.CheckNetworkAccess(tt.host, tt.port)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckNetworkAccess() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantViolation {
				if !sb.HasViolations() {
					t.Error("expected violation to be recorded")
				}
				violations := sb.GetViolations()
				if len(violations) == 0 {
					t.Error("expected at least one violation")
				} else {
					v := violations[0]
					if v.Type != ViolationNetwork {
						t.Errorf("violation type = %v, want %v", v.Type, ViolationNetwork)
					}
					if v.Plugin != "test-plugin" {
						t.Errorf("violation plugin = %v, want test-plugin", v.Plugin)
					}
					if !v.Blocked {
						t.Error("violation should be marked as blocked")
					}
				}
			} else {
				if sb.HasViolations() {
					t.Error("unexpected violation recorded")
				}
			}
		})
	}
}

func TestSandbox_ViolationTracking(t *testing.T) {
	sb := New("test-plugin", &config.PluginCapabilities{
		AllowFilesystem: false,
		AllowNetwork:    false,
	})

	// Initially no violations
	if sb.HasViolations() {
		t.Error("expected no violations initially")
	}
	if len(sb.GetViolations()) != 0 {
		t.Error("expected empty violations slice initially")
	}

	// Trigger filesystem violation
	_ = sb.CheckFilesystemAccess("/some/path")
	if !sb.HasViolations() {
		t.Error("expected violations after filesystem check")
	}
	if len(sb.GetViolations()) != 1 {
		t.Errorf("expected 1 violation, got %d", len(sb.GetViolations()))
	}

	// Trigger network violation
	_ = sb.CheckNetworkAccess("example.com", 80)
	if len(sb.GetViolations()) != 2 {
		t.Errorf("expected 2 violations, got %d", len(sb.GetViolations()))
	}

	// Verify violations are copies (not affected by external modification)
	violations := sb.GetViolations()
	violations[0].Plugin = "modified"
	freshViolations := sb.GetViolations()
	if freshViolations[0].Plugin == "modified" {
		t.Error("GetViolations should return a copy, not original slice")
	}

	// Clear violations
	sb.ClearViolations()
	if sb.HasViolations() {
		t.Error("expected no violations after clear")
	}
	if len(sb.GetViolations()) != 0 {
		t.Error("expected empty violations after clear")
	}
}

func TestSandbox_LogCapabilitySummary(t *testing.T) {
	// Test with nil capabilities
	t.Run("nil capabilities", func(t *testing.T) {
		sb := New("test-plugin", nil)
		sb.capabilities = nil
		// Should not panic
		sb.LogCapabilitySummary()
	})

	// Test with full capabilities
	t.Run("full capabilities", func(t *testing.T) {
		sb := New("test-plugin", &config.PluginCapabilities{
			AllowNetwork:       true,
			AllowFilesystem:    true,
			AllowedPaths:       []string{"/home/user"},
			AllowEnvRead:       true,
			MaxMemoryMB:        512,
			MaxCPUPercent:      50,
			MaxFileDescriptors: 100,
			MaxCPUSeconds:      60,
		})
		// Should not panic
		sb.LogCapabilitySummary()
	})
}
