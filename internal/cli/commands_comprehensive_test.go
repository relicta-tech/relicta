// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Test all root command global flags systematically
func TestRootCommand_AllGlobalFlags(t *testing.T) {
	flags := []struct {
		name         string
		shorthand    string
		defaultValue string
		required     bool
	}{
		{"config", "c", "", false},
		{"verbose", "v", "false", false},
		{"dry-run", "", "false", false},
		{"json", "", "false", false},
		{"no-color", "", "false", false},
		{"log-level", "", "info", false},
		{"model", "", "", false},
		{"ci", "", "false", false},
	}

	for _, f := range flags {
		t.Run("flag_"+f.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(f.name)
			if flag == nil {
				t.Fatalf("global flag %s not found", f.name)
			}
			if f.shorthand != "" && flag.Shorthand != f.shorthand {
				t.Errorf("flag %s shorthand = %v, want %v", f.name, flag.Shorthand, f.shorthand)
			}
			if flag.DefValue != f.defaultValue {
				t.Errorf("flag %s default = %v, want %v", f.name, flag.DefValue, f.defaultValue)
			}
		})
	}
}

// Test all commands are added to root
func TestRootCommand_AllSubcommands(t *testing.T) {
	expectedCommands := []string{
		"version",
		"init",
		"plan",
		"bump",
		"notes",
		"approve",
		"publish",
		"health",
		"metrics",
	}

	commands := rootCmd.Commands()
	commandMap := make(map[string]bool)
	for _, cmd := range commands {
		commandMap[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		t.Run("subcommand_"+expected, func(t *testing.T) {
			if !commandMap[expected] {
				t.Errorf("subcommand %s not found in root command", expected)
			}
		})
	}
}

// Test all commands have proper metadata
func TestAllCommands_HaveMetadata(t *testing.T) {
	commands := []*cobra.Command{
		rootCmd,
		versionCmd,
		initCmd,
		planCmd,
		bumpCmd,
		notesCmd,
		approveCmd,
		publishCmd,
		healthCmd,
		metricsCmd,
	}

	for _, cmd := range commands {
		if cmd == nil {
			continue
		}
		t.Run(cmd.Name()+"_has_use", func(t *testing.T) {
			if cmd.Use == "" {
				t.Errorf("command %s has empty Use field", cmd.Name())
			}
		})
		t.Run(cmd.Name()+"_has_short", func(t *testing.T) {
			if cmd.Short == "" {
				t.Errorf("command %s has empty Short description", cmd.Name())
			}
		})
	}
}

// Test version command structure
func TestVersionCommand_Structure(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %v, want version", versionCmd.Use)
	}
	if versionCmd.Run == nil {
		t.Error("versionCmd.Run should not be nil")
	}
	if versionCmd.Short == "" {
		t.Error("versionCmd.Short should not be empty")
	}
}

// Test styles are initialized
func TestStyles_Initialized(t *testing.T) {
	testCases := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Title", styles.Title},
		{"Success", styles.Success},
		{"Error", styles.Error},
		{"Warning", styles.Warning},
		{"Info", styles.Info},
		{"Subtle", styles.Subtle},
		{"Bold", styles.Bold},
	}

	for _, tc := range testCases {
		t.Run("style_"+tc.name, func(t *testing.T) {
			// Test that the style can render text
			rendered := tc.style.Render("test")
			if rendered == "" {
				t.Errorf("%s style rendered empty string", tc.name)
			}
		})
	}
}

// Test parseModelFlag edge cases
func TestParseModelFlag_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantProvider string
		wantModel    string
	}{
		{"whitespace only", "   ", "", ""},
		{"slash only", "/", "", ""},
		{"multiple slashes", "a/b/c/d", "a", "b/c/d"},
		{"trailing slash", "provider/", "provider", ""},
		{"leading slash", "/model", "", "model"},
		{"mixed case provider", "OpenAI/gpt-4", "openai", "gpt-4"},
		{"local lowercase", "local/model", "ollama", "model"},
		{"LOCAL uppercase", "LOCAL/model", "ollama", "model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotModel := parseModelFlag(tt.input)
			if gotProvider != tt.wantProvider {
				t.Errorf("parseModelFlag() provider = %v, want %v", gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Errorf("parseModelFlag() model = %v, want %v", gotModel, tt.wantModel)
			}
		})
	}
}

// Test global variable initialization
func TestGlobalVariables_Initialization(t *testing.T) {
	if logger == nil {
		t.Error("logger should be initialized")
	}
	// Just verify styles can render
	if styles.Title.Render("test") == "" {
		t.Error("styles should be initialized and able to render")
	}
}

// Test command RunE functions are set
func TestCommands_HaveRunEFunctions(t *testing.T) {
	commands := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"init", initCmd},
		{"plan", planCmd},
		{"bump", bumpCmd},
		{"notes", notesCmd},
		{"approve", approveCmd},
		{"publish", publishCmd},
		{"health", healthCmd},
		{"metrics", metricsCmd},
	}

	for _, c := range commands {
		t.Run(c.name+"_has_RunE", func(t *testing.T) {
			if c.cmd.RunE == nil {
				t.Errorf("command %s has nil RunE function", c.name)
			}
		})
	}
}

// Test HealthStatus type values
func TestHealthStatus_AllValues(t *testing.T) {
	statuses := []struct {
		status HealthStatus
		value  string
	}{
		{HealthStatusHealthy, "healthy"},
		{HealthStatusDegraded, "degraded"},
		{HealthStatusUnhealthy, "unhealthy"},
	}

	for _, s := range statuses {
		t.Run("status_"+s.value, func(t *testing.T) {
			if string(s.status) != s.value {
				t.Errorf("HealthStatus = %v, want %v", s.status, s.value)
			}
		})
	}
}

// Test all allowed editors
func TestAllowedEditors_Comprehensive(t *testing.T) {
	expectedEditors := []string{
		"vim", "nvim", "nano", "emacs", "vi",
		"code", "subl", "gedit", "kate", "micro",
	}

	for _, editor := range expectedEditors {
		t.Run("editor_"+editor, func(t *testing.T) {
			if !allowedEditors[editor] {
				t.Errorf("editor %s should be in allowed list", editor)
			}
		})
	}
}

// Test command descriptions are not empty
func TestCommands_HaveDescriptions(t *testing.T) {
	commands := []*cobra.Command{
		initCmd, planCmd, bumpCmd, notesCmd,
		approveCmd, publishCmd, healthCmd, metricsCmd,
	}

	for _, cmd := range commands {
		t.Run(cmd.Name()+"_long_description", func(t *testing.T) {
			if cmd.Long == "" {
				t.Errorf("command %s should have long description", cmd.Name())
			}
		})
	}
}
