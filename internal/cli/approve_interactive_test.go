package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/ui"
)

func TestRunInteractiveApprovalDryRun(t *testing.T) {
	origRun := runApprovalTUI
	origDryRun := dryRun
	origCfg := cfg
	defer func() {
		runApprovalTUI = origRun
		dryRun = origDryRun
		cfg = origCfg
	}()

	cfg = config.DefaultConfig()
	dryRun = true
	runApprovalTUI = func(summary ui.ReleaseSummary) (ui.ApprovalResult, error) {
		return ui.ApprovalAccepted, nil
	}

	rel := newTestRelease(t, "interactive-1")
	app := testCLIApp{}

	if err := runInteractiveApproval(context.Background(), app, rel); err != nil {
		t.Fatalf("runInteractiveApproval error: %v", err)
	}
}

func TestRunInteractiveApprovalRejected(t *testing.T) {
	origRun := runApprovalTUI
	origDryRun := dryRun
	origCfg := cfg
	defer func() {
		runApprovalTUI = origRun
		dryRun = origDryRun
		cfg = origCfg
	}()

	cfg = config.DefaultConfig()
	dryRun = false
	runApprovalTUI = func(summary ui.ReleaseSummary) (ui.ApprovalResult, error) {
		return ui.ApprovalRejected, nil
	}

	rel := newTestRelease(t, "interactive-2")
	app := testCLIApp{}

	if err := runInteractiveApproval(context.Background(), app, rel); err != nil {
		t.Fatalf("runInteractiveApproval error: %v", err)
	}
}

func TestEditReleaseNotes_InvalidEditor(t *testing.T) {
	origEditor := approveEditor
	defer func() { approveEditor = origEditor }()

	approveEditor = "not-allowed"
	if _, err := editReleaseNotes("notes"); err == nil {
		t.Fatal("expected error for invalid editor")
	}
}
