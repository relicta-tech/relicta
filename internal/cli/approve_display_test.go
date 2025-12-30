package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/config"
)

func captureOutputForApproveDisplay(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	os.Stdout = old
	return buf.String()
}

func boolPtrForDisplay(b bool) *bool {
	return &b
}

func TestDisplayReleaseSummaryEmitsSections(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()
	cfg.Plugins = []config.PluginConfig{
		{Name: "github", Enabled: boolPtrForDisplay(true)},
	}

	rel := newNotesReadyRelease(t, "display-summary")
	out := captureOutputForApproveDisplay(t, func() {
		displayReleaseSummary(rel)
	})

	if !strings.Contains(out, "Release Summary") {
		t.Fatalf("expected release summary title, got: %s", out)
	}
	if !strings.Contains(out, "Release Notes Preview") {
		t.Fatalf("expected release notes preview, got: %s", out)
	}
	if !strings.Contains(out, "Plugins to Execute") {
		t.Fatalf("expected plugin section, got: %s", out)
	}
}

func TestDisplayGovernanceResultShowsDetails(t *testing.T) {
	out := captureOutputForApproveDisplay(t, func() {
		displayGovernanceResult(&governance.EvaluateReleaseOutput{
			RiskScore: 0.65,
			Severity:  cgp.SeverityMedium,
			Decision:  cgp.DecisionApproved,
			RiskFactors: []cgp.RiskFactor{
				{Category: "security", Description: "High privileges", Score: 0.75},
			},
			RequiredActions: []cgp.RequiredAction{
				{Type: "human_review", Description: "Add reviewer"},
			},
			Rationale: []string{"Risk acceptable"},
			HistoricalContext: &governance.HistoricalContext{
				RecentReleases: 5,
				SuccessRate:    0.8,
				RollbackRate:   0.1,
			},
		})
	})

	if !strings.Contains(out, "Governance Evaluation") {
		t.Fatalf("expected governance title, got: %s", out)
	}
	if !strings.Contains(out, "Risk Factors") {
		t.Fatalf("expected risk factors section, got: %s", out)
	}
	if !strings.Contains(out, "Required Actions") {
		t.Fatalf("expected required actions, got: %s", out)
	}
	if !strings.Contains(out, "Historical Context") {
		t.Fatalf("expected historical context, got: %s", out)
	}
}

func TestCreateCGPActorHonorsEnv(t *testing.T) {
	origCI := os.Getenv("CI")
	defer os.Setenv("CI", origCI)
	os.Setenv("CI", "true")
	origCfg := cfg
	cfg = config.DefaultConfig()
	defer func() { cfg = origCfg }()

	actor := createCGPActor()
	if actor.Kind != cgp.ActorKindCI {
		t.Fatalf("expected CI actor kind, got %s", actor.Kind)
	}
	if !strings.HasPrefix(actor.ID, "ci:") {
		t.Fatalf("expected CI prefix, got %q", actor.ID)
	}
}

func TestValidateEditorRejectsUnknown(t *testing.T) {
	if _, err := validateEditor("evil"); err == nil {
		t.Fatal("expected error for disallowed editor")
	}
}
