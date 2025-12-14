// Package cli provides the command-line interface for Relicta.
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
)

func TestLoadAndValidateConfig(t *testing.T) {
	// Create a temp directory with a valid config
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create a valid config file
	configPath := filepath.Join(tmpDir, "release.config.yaml")
	err := os.WriteFile(configPath, []byte(`
versioning:
  strategy: conventional
  tag_prefix: "v"
ai:
  enabled: false
`), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test loading config
	err = loadAndValidateConfig()
	if err != nil {
		t.Errorf("loadAndValidateConfig() error = %v, want nil", err)
	}

	if cfg == nil {
		t.Error("loadAndValidateConfig() should set cfg")
	}
}

func TestLoadAndValidateConfig_WithCustomPath(t *testing.T) {
	// Create a temp directory with a valid config
	tmpDir := t.TempDir()

	// Create a valid config file
	configPath := filepath.Join(tmpDir, "custom.yaml")
	err := os.WriteFile(configPath, []byte(`
versioning:
  strategy: conventional
  tag_prefix: "v"
ai:
  enabled: false
`), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Set custom config path
	origCfgFile := cfgFile
	defer func() { cfgFile = origCfgFile }()
	cfgFile = configPath

	// Test loading config
	err = loadAndValidateConfig()
	if err != nil {
		t.Errorf("loadAndValidateConfig() error = %v, want nil", err)
	}

	if cfg == nil {
		t.Error("loadAndValidateConfig() should set cfg")
	}
}

func TestLoadAndValidateConfig_InvalidConfig(t *testing.T) {
	// Create a temp directory with an invalid config
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create an invalid config file (missing required fields)
	configPath := filepath.Join(tmpDir, "relicta.config.yaml")
	err := os.WriteFile(configPath, []byte(`
invalid: yaml: structure
versioning:
`), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test loading config should fail
	err = loadAndValidateConfig()
	if err == nil {
		t.Error("loadAndValidateConfig() should return error for invalid config")
	}
}

func TestApplyGlobalFlags(t *testing.T) {
	// Setup
	origVerbose := verbose
	origDryRun := dryRun
	origNoColor := noColor
	defer func() {
		verbose = origVerbose
		dryRun = origDryRun
		noColor = origNoColor
	}()

	// Create a test config
	cfg = config.DefaultConfig()

	// Test verbose flag
	verbose = true
	dryRun = false
	noColor = false
	applyGlobalFlags()

	if !cfg.Output.Verbose {
		t.Error("applyGlobalFlags() should set cfg.Output.Verbose when verbose is true")
	}

	// Test dry-run flag
	cfg = config.DefaultConfig()
	verbose = false
	dryRun = true
	applyGlobalFlags()

	if !cfg.Workflow.DryRunByDefault {
		t.Error("applyGlobalFlags() should set cfg.Workflow.DryRunByDefault when dryRun is true")
	}

	// Test no-color flag
	cfg = config.DefaultConfig()
	noColor = true
	applyGlobalFlags()

	if cfg.Output.Color {
		t.Error("applyGlobalFlags() should set cfg.Output.Color to false when noColor is true")
	}
}

func TestApplyModelFlag(t *testing.T) {
	tests := []struct {
		name          string
		modelFlag     string
		wantProvider  string
		wantModel     string
		wantAIEnabled bool
	}{
		{
			name:          "empty flag",
			modelFlag:     "",
			wantProvider:  "",
			wantModel:     "",
			wantAIEnabled: false,
		},
		{
			name:          "ollama provider",
			modelFlag:     "ollama/llama3.2",
			wantProvider:  "ollama",
			wantModel:     "llama3.2",
			wantAIEnabled: true,
		},
		{
			name:          "openai provider",
			modelFlag:     "openai/gpt-4",
			wantProvider:  "openai",
			wantModel:     "gpt-4",
			wantAIEnabled: true,
		},
		{
			name:          "local alias",
			modelFlag:     "local/mistral",
			wantProvider:  "ollama",
			wantModel:     "mistral",
			wantAIEnabled: true,
		},
		{
			name:          "model only",
			modelFlag:     "gpt-4",
			wantProvider:  "",
			wantModel:     "gpt-4",
			wantAIEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			origModelFlag := modelFlag
			defer func() { modelFlag = origModelFlag }()

			cfg = config.DefaultConfig()
			cfg.AI.Enabled = false
			cfg.AI.Provider = ""
			cfg.AI.Model = ""

			modelFlag = tt.modelFlag
			applyModelFlag()

			if tt.wantProvider != "" && cfg.AI.Provider != tt.wantProvider {
				t.Errorf("applyModelFlag() provider = %v, want %v", cfg.AI.Provider, tt.wantProvider)
			}

			if tt.wantModel != "" && cfg.AI.Model != tt.wantModel {
				t.Errorf("applyModelFlag() model = %v, want %v", cfg.AI.Model, tt.wantModel)
			}

			if cfg.AI.Enabled != tt.wantAIEnabled {
				t.Errorf("applyModelFlag() AI enabled = %v, want %v", cfg.AI.Enabled, tt.wantAIEnabled)
			}
		})
	}
}

func TestApplyCIModeFlag(t *testing.T) {
	// Setup
	origCIMode := ciMode
	origOutputJSON := outputJSON
	origNoColor := noColor
	defer func() {
		ciMode = origCIMode
		outputJSON = origOutputJSON
		noColor = origNoColor
	}()

	cfg = config.DefaultConfig()
	cfg.Workflow.RequireApproval = true

	// Test CI mode disabled
	ciMode = false
	outputJSON = false
	noColor = false
	applyCIModeFlag()

	if outputJSON {
		t.Error("applyCIModeFlag() should not change outputJSON when ciMode is false")
	}
	if cfg.Workflow.RequireApproval != true {
		t.Error("applyCIModeFlag() should not change RequireApproval when ciMode is false")
	}

	// Test CI mode enabled
	cfg = config.DefaultConfig()
	cfg.Workflow.RequireApproval = true
	ciMode = true
	outputJSON = false
	noColor = false
	applyCIModeFlag()

	if !outputJSON {
		t.Error("applyCIModeFlag() should set outputJSON to true when ciMode is true")
	}
	if cfg.Workflow.RequireApproval {
		t.Error("applyCIModeFlag() should set RequireApproval to false when ciMode is true")
	}
	if !noColor {
		t.Error("applyCIModeFlag() should set noColor to true when ciMode is true")
	}
}

func TestConfigureLoggerFormat(t *testing.T) {
	// Setup
	origOutputJSON := outputJSON
	origNoColor := noColor
	defer func() {
		outputJSON = origOutputJSON
		noColor = origNoColor
	}()

	cfg = config.DefaultConfig()

	// Test JSON format
	outputJSON = true
	cfg.Output.Format = ""
	configureLoggerFormat()
	// Just verify it doesn't panic

	// Test text format with no color
	outputJSON = false
	noColor = true
	cfg.Output.Color = false
	configureLoggerFormat()
	// Just verify it doesn't panic

	// Test default format
	outputJSON = false
	noColor = false
	cfg.Output.Color = true
	configureLoggerFormat()
	// Just verify it doesn't panic
}

func TestConfigureLogLevel(t *testing.T) {
	cfg = config.DefaultConfig()

	tests := []struct {
		logLevel string
		verbose  bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"invalid", false},
		{"info", true}, // verbose overrides
	}

	for _, tt := range tests {
		t.Run(tt.logLevel, func(t *testing.T) {
			cfg.Output.LogLevel = tt.logLevel
			cfg.Output.Verbose = tt.verbose
			configureLogLevel()
			// Just verify it doesn't panic
		})
	}
}

func TestConfigureLogFile(t *testing.T) {
	// Test with no log file
	cfg = config.DefaultConfig()
	cfg.Output.LogFile = ""

	err := configureLogFile()
	if err != nil {
		t.Errorf("configureLogFile() with empty path should not error, got %v", err)
	}

	// Test with valid log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	cfg.Output.LogFile = logPath
	err = configureLogFile()
	if err != nil {
		t.Errorf("configureLogFile() with valid path should not error, got %v", err)
	}

	// Clean up
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("configureLogFile() should create log file")
	}

	// Test with invalid path
	cfg.Output.LogFile = "/invalid/path/that/does/not/exist/test.log"
	err = configureLogFile()
	if err == nil {
		t.Error("configureLogFile() with invalid path should return error")
	}
}

func TestPrintHelpers(t *testing.T) {
	// Test that print helpers don't panic
	tests := []struct {
		name string
		fn   func(string)
	}{
		{"printSuccess", printSuccess},
		{"printError", printError},
		{"printWarning", printWarning},
		{"printInfo", printInfo},
		{"printTitle", printTitle},
		{"printSubtle", printSubtle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify they don't panic
			tt.fn("test message")
		})
	}
}
