package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/security"
)

func resetCLIState(t *testing.T) func() {
	t.Helper()
	prevCfg := cfg
	prevVerbose := verbose
	prevDryRun := dryRun
	prevNoColor := noColor
	prevOutputJSON := outputJSON
	prevCI := ciMode
	prevModel := modelFlag
	prevRedact := redactSecrets
	prevLogger := logger
	prevLogFile := logFile

	return func() {
		verbose = prevVerbose
		dryRun = prevDryRun
		noColor = prevNoColor
		outputJSON = prevOutputJSON
		ciMode = prevCI
		modelFlag = prevModel
		redactSecrets = prevRedact
		cfg = prevCfg
		logger = prevLogger
		logFile = prevLogFile
	}
}

func TestApplyGlobalFlagsAdditional(t *testing.T) {
	t.Cleanup(resetCLIState(t))

	cfg = config.DefaultConfig()
	verbose = true
	dryRun = true
	noColor = true

	applyGlobalFlags()

	if !cfg.Output.Verbose {
		t.Errorf("expected verbose output to be enabled")
	}
	if !cfg.Workflow.DryRunByDefault {
		t.Errorf("expected dry run workflow flag to be set")
	}
	if cfg.Output.Color {
		t.Errorf("expected colors to be disabled")
	}
}

func TestApplyModelFlagAdditional(t *testing.T) {
	t.Cleanup(resetCLIState(t))

	t.Cleanup(func() {
		modelFlag = ""
	})

	cfg = config.DefaultConfig()

	cases := []struct {
		name        string
		flag        string
		wantProv    string
		wantModel   string
		wantEnabled bool
	}{
		{"provider and model", "openai/gpt-4", "openai", "gpt-4", true},
		{"local alias", "local/llama3.2", "ollama", "llama3.2", true},
		{"model only", "custom-model", "openai", "custom-model", false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			modelFlag = tt.flag
			cfg.AI.Enabled = false
			cfg.AI.Provider = "openai"
			cfg.AI.Model = "gpt-4"

			applyModelFlag()

			if cfg.AI.Provider != tt.wantProv {
				t.Errorf("provider = %v, want %v", cfg.AI.Provider, tt.wantProv)
			}
			if cfg.AI.Model != tt.wantModel {
				t.Errorf("model = %v, want %v", cfg.AI.Model, tt.wantModel)
			}
			if cfg.AI.Enabled != tt.wantEnabled {
				t.Errorf("AI enabled = %v, want %v", cfg.AI.Enabled, tt.wantEnabled)
			}
		})
	}
}

func TestApplyCIModeFlagEnsuresSettings(t *testing.T) {
	t.Cleanup(resetCLIState(t))
	security.Disable()
	t.Cleanup(security.Disable)

	ciMode = true

	cfg = config.DefaultConfig()
	cfg.Workflow.RequireApproval = true
	outputJSON = false
	noColor = false

	applyCIModeFlag()

	if !outputJSON {
		t.Error("expected JSON output to be forced in CI mode")
	}
	if cfg.Workflow.RequireApproval {
		t.Error("expected approval to be disabled in CI mode")
	}
	if !noColor {
		t.Error("expected colors to be disabled")
	}
	if !security.IsEnabled() {
		t.Error("expected secrets to be masked in CI mode")
	}
	if !IsCIMode() {
		t.Error("expected IsCIMode to return true")
	}
}

func TestApplyRedactSecretsFlag(t *testing.T) {
	t.Cleanup(resetCLIState(t))
	security.Disable()
	t.Cleanup(security.Disable)
	redactSecrets = true

	applyRedactSecretsFlag()
	if !security.IsEnabled() {
		t.Error("expected security masking to be enabled when --redact is set")
	}

	security.Disable()
	redactSecrets = false

	os.Setenv("CI", "true")
	t.Cleanup(func() { _ = os.Unsetenv("CI") })

	applyRedactSecretsFlag()
	if !security.IsEnabled() {
		t.Error("expected CI environment to enable masking even without --redact")
	}
}

func TestParseModelFlagVariants(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"openai/gpt-4", "openai", "gpt-4"},
		{"local/llama3.2", "ollama", "llama3.2"},
		{"model-only", "", "model-only"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			provider, model := parseModelFlag(tt.input)
			if provider != tt.wantProvider {
				t.Errorf("provider = %q, want %q", provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("model = %q, want %q", model, tt.wantModel)
			}
		})
	}
}

func TestConfigureLogLevelAndFile(t *testing.T) {
	t.Cleanup(resetCLIState(t))

	tmpDir := t.TempDir()
	cfg = config.DefaultConfig()
	cfg.Output.LogLevel = "warn"
	cfg.Output.Verbose = true
	cfg.Output.LogFile = filepath.Join(tmpDir, "relicta.log")

	logger = log.NewWithOptions(io.Discard, log.Options{
		ReportTimestamp: true,
	})

	configureLogLevel()
	if logger.GetLevel() != log.DebugLevel {
		t.Errorf("expected debug level when verbose, got %v", logger.GetLevel())
	}

	if err := configureLogFile(); err != nil {
		t.Fatalf("configureLogFile returned error: %v", err)
	}
	if logFile == nil {
		t.Fatal("expected logFile to be set")
	}

	os.Remove(logFile.Name())
	Cleanup()
}

func TestConfigureLogFileError(t *testing.T) {
	t.Cleanup(resetCLIState(t))

	cfg = config.DefaultConfig()
	cfg.Output.LogFile = os.TempDir() // directory path triggers error
	logger = log.NewWithOptions(io.Discard, log.Options{})

	if err := configureLogFile(); err == nil {
		t.Fatal("expected configureLogFile to fail when pointing at a directory")
	}
}

func TestSpinnerStopsWithMessages(t *testing.T) {
	t.Cleanup(resetCLIState(t))

	spinner := NewSpinner("working")
	spinner.Start()
	spinner.StopWithSuccess("done")

	spinner = NewSpinner("failing")
	spinner.Start()
	spinner.StopWithError("boom")
}
