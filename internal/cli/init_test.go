// Package cli provides the command-line interface for Relicta.
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/config"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		want      string
	}{
		{
			name:      "SSH format",
			remoteURL: "git@github.com:owner/repo.git",
			want:      "https://github.com/owner/repo",
		},
		{
			name:      "SSH format without .git",
			remoteURL: "git@github.com:owner/repo",
			want:      "https://github.com/owner/repo",
		},
		{
			name:      "HTTPS format",
			remoteURL: "https://github.com/owner/repo.git",
			want:      "https://github.com/owner/repo",
		},
		{
			name:      "HTTPS format without .git",
			remoteURL: "https://github.com/owner/repo",
			want:      "https://github.com/owner/repo",
		},
		{
			name:      "HTTP format converted to HTTPS",
			remoteURL: "http://github.com/owner/repo.git",
			want:      "https://github.com/owner/repo",
		},
		{
			name:      "non-GitHub URL",
			remoteURL: "git@gitlab.com:owner/repo.git",
			want:      "",
		},
		{
			name:      "empty string",
			remoteURL: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGitHubURL(tt.remoteURL)
			if got != tt.want {
				t.Errorf("parseGitHubURL(%q) = %q, want %q", tt.remoteURL, got, tt.want)
			}
		})
	}
}

func TestHasPlugin(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *config.Config
		pluginName string
		want       bool
	}{
		{
			name: "plugin exists and enabled",
			cfg: &config.Config{
				Plugins: []config.PluginConfig{
					{
						Name:    "github",
						Enabled: boolPtr(true),
					},
				},
			},
			pluginName: "github",
			want:       true,
		},
		{
			name: "plugin exists but disabled",
			cfg: &config.Config{
				Plugins: []config.PluginConfig{
					{
						Name:    "github",
						Enabled: boolPtr(false),
					},
				},
			},
			pluginName: "github",
			want:       false,
		},
		{
			name: "plugin does not exist",
			cfg: &config.Config{
				Plugins: []config.PluginConfig{
					{
						Name:    "github",
						Enabled: boolPtr(true),
					},
				},
			},
			pluginName: "slack",
			want:       false,
		},
		{
			name: "empty plugin list",
			cfg: &config.Config{
				Plugins: []config.PluginConfig{},
			},
			pluginName: "github",
			want:       false,
		},
		{
			name: "nil plugins",
			cfg: &config.Config{
				Plugins: nil,
			},
			pluginName: "github",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasPlugin(tt.cfg, tt.pluginName)
			if got != tt.want {
				t.Errorf("hasPlugin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsurePlugin(t *testing.T) {
	tests := []struct {
		name           string
		initialPlugins []config.PluginConfig
		pluginName     string
		wantEnabled    bool
		wantCount      int
	}{
		{
			name:           "add new plugin to empty list",
			initialPlugins: []config.PluginConfig{},
			pluginName:     "github",
			wantEnabled:    true,
			wantCount:      1,
		},
		{
			name: "enable existing disabled plugin",
			initialPlugins: []config.PluginConfig{
				{
					Name:    "github",
					Enabled: boolPtr(false),
				},
			},
			pluginName:  "github",
			wantEnabled: true,
			wantCount:   1,
		},
		{
			name: "plugin already enabled",
			initialPlugins: []config.PluginConfig{
				{
					Name:    "github",
					Enabled: boolPtr(true),
				},
			},
			pluginName:  "github",
			wantEnabled: true,
			wantCount:   1,
		},
		{
			name: "add plugin to existing list",
			initialPlugins: []config.PluginConfig{
				{
					Name:    "github",
					Enabled: boolPtr(true),
				},
			},
			pluginName:  "slack",
			wantEnabled: true,
			wantCount:   2,
		},
		{
			name:           "add npm plugin with default config",
			initialPlugins: []config.PluginConfig{},
			pluginName:     "npm",
			wantEnabled:    true,
			wantCount:      1,
		},
		{
			name:           "add slack plugin with default config",
			initialPlugins: []config.PluginConfig{},
			pluginName:     "slack",
			wantEnabled:    true,
			wantCount:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Plugins: tt.initialPlugins,
			}

			ensurePlugin(cfg, tt.pluginName)

			if len(cfg.Plugins) != tt.wantCount {
				t.Errorf("ensurePlugin() plugin count = %d, want %d", len(cfg.Plugins), tt.wantCount)
			}

			// Find the plugin and check if it's enabled
			found := false
			for _, p := range cfg.Plugins {
				if p.Name == tt.pluginName {
					found = true
					if p.IsEnabled() != tt.wantEnabled {
						t.Errorf("ensurePlugin() plugin enabled = %v, want %v", p.IsEnabled(), tt.wantEnabled)
					}

					// Check default configs for known plugins
					if tt.pluginName == "github" && p.Config != nil {
						if draft, ok := p.Config["draft"]; !ok || draft != false {
							t.Errorf("ensurePlugin() github plugin should have draft: false in config")
						}
					}
					if tt.pluginName == "npm" && p.Config != nil {
						if access, ok := p.Config["access"]; !ok || access != "public" {
							t.Errorf("ensurePlugin() npm plugin should have access: public in config")
						}
					}
					if tt.pluginName == "slack" && p.Config != nil {
						if _, ok := p.Config["webhook"]; !ok {
							t.Errorf("ensurePlugin() slack plugin should have webhook in config")
						}
					}
					break
				}
			}

			if !found {
				t.Errorf("ensurePlugin() plugin %s not found in config", tt.pluginName)
			}
		})
	}
}

func TestRemovePlugin(t *testing.T) {
	tests := []struct {
		name           string
		initialPlugins []config.PluginConfig
		pluginName     string
		wantCount      int
		wantRemains    []string
	}{
		{
			name: "remove existing plugin",
			initialPlugins: []config.PluginConfig{
				{Name: "github", Enabled: boolPtr(true)},
				{Name: "slack", Enabled: boolPtr(true)},
			},
			pluginName:  "github",
			wantCount:   1,
			wantRemains: []string{"slack"},
		},
		{
			name: "remove non-existent plugin",
			initialPlugins: []config.PluginConfig{
				{Name: "github", Enabled: boolPtr(true)},
			},
			pluginName:  "slack",
			wantCount:   1,
			wantRemains: []string{"github"},
		},
		{
			name:           "remove from empty list",
			initialPlugins: []config.PluginConfig{},
			pluginName:     "github",
			wantCount:      0,
			wantRemains:    []string{},
		},
		{
			name: "remove all plugins",
			initialPlugins: []config.PluginConfig{
				{Name: "github", Enabled: boolPtr(true)},
			},
			pluginName:  "github",
			wantCount:   0,
			wantRemains: []string{},
		},
		{
			name: "remove one of many",
			initialPlugins: []config.PluginConfig{
				{Name: "github", Enabled: boolPtr(true)},
				{Name: "npm", Enabled: boolPtr(true)},
				{Name: "slack", Enabled: boolPtr(true)},
			},
			pluginName:  "npm",
			wantCount:   2,
			wantRemains: []string{"github", "slack"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Plugins: tt.initialPlugins,
			}

			removePlugin(cfg, tt.pluginName)

			if len(cfg.Plugins) != tt.wantCount {
				t.Errorf("removePlugin() plugin count = %d, want %d", len(cfg.Plugins), tt.wantCount)
			}

			// Check remaining plugins
			remaining := make(map[string]bool)
			for _, p := range cfg.Plugins {
				remaining[p.Name] = true
			}

			for _, expected := range tt.wantRemains {
				if !remaining[expected] {
					t.Errorf("removePlugin() expected plugin %s to remain", expected)
				}
			}

			// Ensure removed plugin is not present
			if remaining[tt.pluginName] {
				t.Errorf("removePlugin() plugin %s should have been removed", tt.pluginName)
			}
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

func TestInitCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"force flag", "force"},
		{"interactive flag", "interactive"},
		{"format flag", "format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := initCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("init command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestInitCommand_FlagDefaults(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		wantDefault string
	}{
		{"force default", "force", "false"},
		{"interactive default", "interactive", "true"},
		{"format default", "format", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := initCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("%s flag not found", tt.flagName)
			}
			if flag.DefValue != tt.wantDefault {
				t.Errorf("%s flag default = %v, want %v", tt.flagName, flag.DefValue, tt.wantDefault)
			}
		})
	}
}

func TestInitCommand_FlagShorthands(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		wantShorthand string
	}{
		{"force shorthand", "force", "f"},
		{"interactive shorthand", "interactive", "i"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := initCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("%s flag not found", tt.flagName)
			}
			if flag.Shorthand != tt.wantShorthand {
				t.Errorf("%s flag shorthand = %v, want %v", tt.flagName, flag.Shorthand, tt.wantShorthand)
			}
		})
	}
}

func TestInitCommand_Configuration(t *testing.T) {
	if initCmd == nil {
		t.Fatal("initCmd is nil")
	}
	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %v, want init", initCmd.Use)
	}
	if initCmd.RunE == nil {
		t.Error("initCmd.RunE is nil")
	}
	if initCmd.Short == "" {
		t.Error("initCmd.Short should not be empty")
	}
	if initCmd.Long == "" {
		t.Error("initCmd.Long should not be empty")
	}
}

func TestRunInitNonInteractiveCreatesConfig(t *testing.T) {
	origInteractive := initInteractive
	origForce := initForce
	origFormat := initFormat
	origVerbose := verbose
	defer func() {
		initInteractive = origInteractive
		initForce = origForce
		initFormat = origFormat
		verbose = origVerbose
	}()

	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	initInteractive = false
	initForce = true
	initFormat = "yaml"
	verbose = false

	cmd := &cobra.Command{}
	if err := runInit(cmd, nil); err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, ".relicta.yaml")); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestDetectRepoSettings_NoRepo(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	cfg := config.DefaultConfig()
	if err := detectRepoSettings(cfg); err == nil {
		t.Fatal("expected detectRepoSettings to fail outside a git repo")
	}
}

func TestIsGitHubRemote(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		want      bool
	}{
		{"SSH GitHub URL", "git@github.com:owner/repo.git", true},
		{"HTTPS GitHub URL", "https://github.com/owner/repo.git", true},
		{"HTTP GitHub URL", "http://github.com/owner/repo", true},
		{"GitLab URL", "git@gitlab.com:owner/repo.git", false},
		{"Bitbucket URL", "git@bitbucket.org:owner/repo.git", false},
		{"custom URL", "git@example.com:owner/repo.git", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitHubRemote(tt.remoteURL)
			if got != tt.want {
				t.Errorf("isGitHubRemote(%q) = %v, want %v", tt.remoteURL, got, tt.want)
			}
		})
	}
}

func TestIsGitLabRemote(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		want      bool
	}{
		{"SSH GitLab.com URL", "git@gitlab.com:owner/repo.git", true},
		{"HTTPS GitLab.com URL", "https://gitlab.com/owner/repo.git", true},
		{"self-hosted GitLab", "git@gitlab.example.com:owner/repo.git", true},
		{"GitHub URL", "git@github.com:owner/repo.git", false},
		{"Bitbucket URL", "git@bitbucket.org:owner/repo.git", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitLabRemote(tt.remoteURL)
			if got != tt.want {
				t.Errorf("isGitLabRemote(%q) = %v, want %v", tt.remoteURL, got, tt.want)
			}
		})
	}
}

func TestRunInit_ExistingConfigNoForce(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	// Create an existing config file
	configPath := filepath.Join(tmpDir, ".relicta.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	origInteractive := initInteractive
	origForce := initForce
	t.Cleanup(func() {
		initInteractive = origInteractive
		initForce = origForce
	})
	initInteractive = false
	initForce = false

	cmd := &cobra.Command{}
	if err := runInit(cmd, nil); err != nil {
		t.Errorf("runInit should not return error for existing config, got: %v", err)
	}
}

func TestRunInit_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	origInteractive := initInteractive
	origForce := initForce
	origFormat := initFormat
	t.Cleanup(func() {
		initInteractive = origInteractive
		initForce = origForce
		initFormat = origFormat
	})
	initInteractive = false
	initForce = true
	initFormat = "json"

	cmd := &cobra.Command{}
	if err := runInit(cmd, nil); err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, ".relicta.json")); err != nil {
		t.Fatalf("expected JSON config file to be created: %v", err)
	}
}
