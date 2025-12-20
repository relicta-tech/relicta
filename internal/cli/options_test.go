package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/relicta-tech/relicta/internal/config"
)

func TestOptions_PrintHelpersAndSetters(t *testing.T) {
	var buf bytes.Buffer
	opts := NewOptions()
	opts.Stdout = &buf
	opts.SetVersion("1.2.3", "commit", "2025-01-02")

	if opts.Version.Version != "1.2.3" || opts.Version.Commit != "commit" || opts.Version.Date != "2025-01-02" {
		t.Fatalf("SetVersion did not set fields: %+v", opts.Version)
	}

	opts.PrintTitle("Title")
	opts.PrintSuccess("ok")
	opts.PrintError("fail")
	opts.PrintWarning("warn")
	opts.PrintInfo("info")
	opts.PrintSubtle("subtle")

	out := buf.String()
	for _, substr := range []string{"Title", "ok", "fail", "warn", "info", "subtle"} {
		if !bytes.Contains([]byte(out), []byte(substr)) {
			t.Fatalf("expected output to contain %q, got %q", substr, out)
		}
	}

	if opts.LogFile == nil {
		tmp, err := os.CreateTemp("", "relicta-log")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		opts.LogFile = tmp
		opts.Cleanup()
		if opts.LogFile != nil {
			t.Fatalf("Cleanup did not reset LogFile")
		}
		if _, err := os.Stat(tmp.Name()); !os.IsNotExist(err) && err != nil {
			_ = tmp.Close()
		}
	}
}

func TestOptions_DryRunAndVerboseFallbacks(t *testing.T) {
	opts := NewOptions()
	if !opts.IsJSON() && opts.IsCI() {
		t.Fatalf("default flags should be false")
	}

	opts.DryRun = true
	if !opts.IsDryRun() {
		t.Fatalf("IsDryRun should reflect DryRun flag")
	}

	opts.DryRun = false
	opts.Config = &config.Config{
		Workflow: config.WorkflowConfig{DryRunByDefault: true},
	}
	if !opts.IsDryRun() {
		t.Fatalf("IsDryRun should inherit from config workflow")
	}

	opts.Verbose = false
	opts.Config.Output.Verbose = true
	if !opts.IsVerbose() {
		t.Fatalf("IsVerbose should reflect config output verbose")
	}
}

func TestApplyGlobalFlagsAndModelFlag(t *testing.T) {
	origCfg := cfg
	origVerbose := verbose
	origDryRun := dryRun
	origNoColor := noColor
	origModelFlag := modelFlag
	defer func() {
		cfg = origCfg
		verbose = origVerbose
		dryRun = origDryRun
		noColor = origNoColor
		modelFlag = origModelFlag
		lipgloss.SetColorProfile(termenv.EnvColorProfile())
	}()

	cfg = &config.Config{
		Workflow: config.WorkflowConfig{},
		Output:   config.OutputConfig{Color: true},
	}

	verbose = true
	dryRun = true
	noColor = true
	applyGlobalFlags()

	if !cfg.Output.Verbose {
		t.Fatalf("applyGlobalFlags should set Output.Verbose")
	}
	if !cfg.Workflow.DryRunByDefault {
		t.Fatalf("applyGlobalFlags should set Workflow.DryRunByDefault")
	}

	modelFlag = "openai/gpt-4"
	cfg.AI = config.AIConfig{}
	applyModelFlag()
	if cfg.AI.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", cfg.AI.Provider)
	}
	if cfg.AI.Model != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %q", cfg.AI.Model)
	}
}
