package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/config"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
)

func TestOutputEvaluateJSON(t *testing.T) {
	result := &governance.EvaluateReleaseOutput{
		Decision:       cgp.DecisionApproved,
		RiskScore:      0.25,
		Severity:       cgp.SeverityLow,
		CanAutoApprove: true,
		Rationale:      []string{"Low risk"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputEvaluateJSON("rel-123", result)
	if err != nil {
		t.Fatalf("outputEvaluateJSON failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var out evaluateOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if out.ReleaseID != "rel-123" {
		t.Fatalf("ReleaseID = %q, want rel-123", out.ReleaseID)
	}
	if out.Decision != cgp.DecisionApproved {
		t.Fatalf("Decision = %q, want %q", out.Decision, cgp.DecisionApproved)
	}
}

func TestRunEvaluateJSON(t *testing.T) {
	origCfg := cfg
	origOutputJSON := outputJSON
	defer func() {
		cfg = origCfg
		outputJSON = origOutputJSON
	}()

	cfg = config.DefaultConfig()
	cfg.Governance.Enabled = true
	outputJSON = true

	// evaluate loads through the release services' repository now, not
	// app.ReleaseRepository() — the latter is a second, lossy implementation that
	// drops the changeset and made evaluate fail on every real release. The fake
	// has to supply the same path the command takes, or the test passes against a
	// repository the command no longer reads.
	rel := newTestReleaseWithCommits(t, "eval-1")
	portsRepo := &portsReleaseRepoStub{run: rel}
	app := commandTestApp{
		gitRepo:         stubGitRepo{},
		releaseRepo:     testReleaseRepo{latest: rel},
		releaseServices: &domainrelease.Services{Repository: portsRepo},
		govSvc:          governance.NewService(evaluator.New()),
		hasGov:          true,
	}
	withStubContainerApp(t, app)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runEvaluate(cmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runEvaluate failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var out evaluateOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if out.ReleaseID == "" {
		t.Fatal("expected release_id in output")
	}
}
