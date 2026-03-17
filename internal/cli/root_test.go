// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"testing"
)

func TestRootCommand_SilenceUsage(t *testing.T) {
	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage should be true")
	}
}

func TestRootCommand_SilenceErrors(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors should be true")
	}
}

func TestParseModelFlag_EmptyFlag(t *testing.T) {
	provider, model := parseModelFlag("")
	if provider != "" || model != "" {
		t.Errorf("parseModelFlag('') = (%q, %q), want ('', '')", provider, model)
	}
}

func TestParseModelFlag_ProviderAndModel(t *testing.T) {
	tests := []struct {
		name         string
		flag         string
		wantProvider string
		wantModel    string
	}{
		{
			name:         "ollama with model",
			flag:         "ollama/llama3.2",
			wantProvider: "ollama",
			wantModel:    "llama3.2",
		},
		{
			name:         "openai with model",
			flag:         "openai/gpt-4",
			wantProvider: "openai",
			wantModel:    "gpt-4",
		},
		{
			name:         "anthropic with model",
			flag:         "anthropic/claude-3",
			wantProvider: "anthropic",
			wantModel:    "claude-3",
		},
		{
			name:         "local alias maps to ollama",
			flag:         "local/mistral",
			wantProvider: "ollama",
			wantModel:    "mistral",
		},
		{
			name:         "LOCAL uppercase alias",
			flag:         "LOCAL/codellama",
			wantProvider: "ollama",
			wantModel:    "codellama",
		},
		{
			name:         "model only without provider",
			flag:         "gpt-4o",
			wantProvider: "",
			wantModel:    "gpt-4o",
		},
		{
			name:         "whitespace trimming",
			flag:         "  ollama/llama3.2  ",
			wantProvider: "ollama",
			wantModel:    "llama3.2",
		},
		{
			name:         "model with version tag",
			flag:         "ollama/codellama:13b",
			wantProvider: "ollama",
			wantModel:    "codellama:13b",
		},
		{
			name:         "complex model path",
			flag:         "provider/namespace/model",
			wantProvider: "provider",
			wantModel:    "namespace/model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProvider, gotModel := parseModelFlag(tt.flag)
			if gotProvider != tt.wantProvider {
				t.Errorf("parseModelFlag() provider = %v, want %v", gotProvider, tt.wantProvider)
			}
			if gotModel != tt.wantModel {
				t.Errorf("parseModelFlag() model = %v, want %v", gotModel, tt.wantModel)
			}
		})
	}
}

func TestInitCommand_Use(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %v, want init", initCmd.Use)
	}
}

func TestInitCommand_RunE(t *testing.T) {
	if initCmd.RunE == nil {
		t.Error("initCmd.RunE should not be nil")
	}
}

func TestPlanCommand_Use(t *testing.T) {
	if planCmd.Use != "plan" {
		t.Errorf("planCmd.Use = %v, want plan", planCmd.Use)
	}
}

func TestBumpCommand_Use(t *testing.T) {
	if bumpCmd.Use != "bump" {
		t.Errorf("bumpCmd.Use = %v, want bump", bumpCmd.Use)
	}
}

func TestBumpCommand_Aliases(t *testing.T) {
	found := false
	for _, alias := range bumpCmd.Aliases {
		if alias == "version-bump" {
			found = true
			break
		}
	}
	if !found {
		t.Error("bumpCmd should have 'version-bump' alias")
	}
}

func TestNotesCommand_Use(t *testing.T) {
	if notesCmd.Use != "notes" {
		t.Errorf("notesCmd.Use = %v, want notes", notesCmd.Use)
	}
}

func TestApproveCommand_Use(t *testing.T) {
	if approveCmd.Use != "approve" {
		t.Errorf("approveCmd.Use = %v, want approve", approveCmd.Use)
	}
}

func TestPublishCommand_Use(t *testing.T) {
	if publishCmd.Use != "publish" {
		t.Errorf("publishCmd.Use = %v, want publish", publishCmd.Use)
	}
}

func TestVersionCommand_Run(t *testing.T) {
	if versionCmd.Run == nil {
		t.Error("versionCmd.Run should not be nil")
	}
}

func TestVersionCommand_Use(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %v, want version", versionCmd.Use)
	}
}

func TestRootCommand_PersistentPreRunE(t *testing.T) {
	if rootCmd.PersistentPreRunE == nil {
		t.Error("rootCmd.PersistentPreRunE should not be nil")
	}
}

func TestExecute_FunctionExists(t *testing.T) {
	// Just verify the function exists - we can't actually test execution
	// without running the whole CLI
	t.Log("Execute function exists and is exported")
}

func TestExecuteContext_FunctionExists(t *testing.T) {
	// Just verify the function exists
	t.Log("ExecuteContext function exists and is exported")
}

func TestExecute_HelpCommandSucceeds(t *testing.T) {
	origArgs := rootCmd.Args
	rootCmd.SetArgs([]string{"help"})
	defer rootCmd.SetArgs(nil)
	if err := Execute(); err != nil {
		t.Fatalf("root Execute failed: %v", err)
	}
	rootCmd.SetArgs(nil)
	rootCmd.Args = origArgs
}

func TestExecuteContext_HelpCommandSucceeds(t *testing.T) {
	origArgs := rootCmd.Args
	rootCmd.SetArgs([]string{"help"})
	defer rootCmd.SetArgs(nil)
	if err := ExecuteContext(context.Background()); err != nil {
		t.Fatalf("root ExecuteContext failed: %v", err)
	}
	rootCmd.SetArgs(nil)
	rootCmd.Args = origArgs
}

func TestSetVersionInfo_Function(t *testing.T) {
	// Save original
	origVersion := versionInfo.Version
	origCommit := versionInfo.Commit
	origDate := versionInfo.Date
	defer func() {
		versionInfo.Version = origVersion
		versionInfo.Commit = origCommit
		versionInfo.Date = origDate
	}()

	SetVersionInfo("test-version", "test-commit", "test-date")

	if versionInfo.Version != "test-version" {
		t.Errorf("Version = %v, want test-version", versionInfo.Version)
	}
	if versionInfo.Commit != "test-commit" {
		t.Errorf("Commit = %v, want test-commit", versionInfo.Commit)
	}
	if versionInfo.Date != "test-date" {
		t.Errorf("Date = %v, want test-date", versionInfo.Date)
	}
}

func TestCleanup_Function(t *testing.T) {
	// Just verify function doesn't panic when called
	Cleanup()
}

func TestIsCIMode_Function(t *testing.T) {
	// Save original
	origCIMode := ciMode
	defer func() { ciMode = origCIMode }()

	ciMode = false
	if IsCIMode() {
		t.Error("IsCIMode() should return false when ciMode is false")
	}

	ciMode = true
	if !IsCIMode() {
		t.Error("IsCIMode() should return true when ciMode is true")
	}
}

func TestIsJSONOutput_Function(t *testing.T) {
	// Save original
	origOutputJSON := outputJSON
	defer func() { outputJSON = origOutputJSON }()

	outputJSON = false
	if IsJSONOutput() {
		t.Error("IsJSONOutput() should return false when outputJSON is false")
	}

	outputJSON = true
	if !IsJSONOutput() {
		t.Error("IsJSONOutput() should return true when outputJSON is true")
	}
}
