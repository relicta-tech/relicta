package sandbox

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
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
