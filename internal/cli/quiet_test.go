package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// output.quiet had no reader, so a repository that asked for quiet output got the full chrome
// on every command.
func TestQuietSuppressesTheChrome(t *testing.T) {
	origCfg, origJSON := cfg, outputJSON
	t.Cleanup(func() { cfg, outputJSON = origCfg, origJSON })
	outputJSON = false

	cfg = config.DefaultConfig()
	if humanOutputSuppressed() {
		t.Error("output is suppressed by default")
	}

	cfg.Output.Quiet = true
	if !humanOutputSuppressed() {
		t.Error("output.quiet did not suppress the chrome")
	}
}

// The gate runs before configuration is loaded in some paths, and a nil dereference there would
// take out every command that prints anything.
func TestTheQuietGateSurvivesNoConfiguration(t *testing.T) {
	origCfg, origJSON := cfg, outputJSON
	t.Cleanup(func() { cfg, outputJSON = origCfg, origJSON })

	cfg, outputJSON = nil, false
	if humanOutputSuppressed() {
		t.Error("output is suppressed when no configuration is loaded")
	}

	outputJSON = true
	if !humanOutputSuppressed() {
		t.Error("--json no longer suppresses human output")
	}
}
