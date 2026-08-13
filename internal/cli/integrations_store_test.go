package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// `relicta integrations vanta push` and `drata push` generated their evidence from
// memory.NewInMemoryStore() — a store constructed on the spot and never populated.
// So the evidence was built from zero records and then pushed to Vanta or Drata,
// where an auditor reads it as this organization's compliance record.
//
// `relicta report` had the same defect and was fixed. This path was missed, and it is
// the worse of the two: a blank report is visibly blank, while an Article 12 artifact
// asserting no incidents and no failed releases — from a store that was never read —
// is an affirmative false statement inside a compliance platform.
//
// The tests drive the commands' RunE with --dry-run in a repository whose governance
// store has records, and assert evidence comes out. Against the old code they report
// "No evidence to push", which is the exact symptom.

// seedGovernanceStore writes one published release into the store that
// getMemoryStoreCtx resolves for repoDir, and returns the repository identity it was
// recorded under.
func seedGovernanceStore(t *testing.T, repoDir, repository string) {
	t.Helper()

	store, err := memory.NewFileStore(filepath.Dir(governance.MemoryStorePath("", repoDir)))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	released := time.Now().UTC().Add(-24 * time.Hour)
	if err := store.RecordRelease(context.Background(), &memory.ReleaseRecord{
		ID:            "run-seeded-1",
		Repository:    repository,
		Version:       "1.0.0",
		Actor:         cgp.Actor{Kind: cgp.ActorKindHuman, ID: "alice@example.com"},
		RiskScore:     0.2,
		Decision:      cgp.DecisionApproved,
		Outcome:       memory.OutcomeSuccess,
		ReleasedAt:    released,
		FirstCommitAt: released.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveReleaseRecord: %v", err)
	}
}

// currentPeriod returns a period string covering the seeded record, so the test does
// not depend on which quarter it runs in.
func currentPeriod() string {
	now := time.Now().UTC()
	return fmt.Sprintf("%d-Q%d", now.Year(), (int(now.Month())-1)/3+1)
}

// newTestCommand returns a command carrying a context, since the push handlers read
// cmd.Context() to resolve the store.
func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

// runInRepo chdirs into a repository seeded with governance history, runs fn, and
// returns what the command printed.
func runInRepo(t *testing.T, fn func() error) string {
	t.Helper()

	repoDir := newGitRepoWithRemote(t, "https://github.com/acme/widget.git")
	seedGovernanceStore(t, repoDir, "acme/widget")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	// The print helpers write to stdout, so capture it rather than asserting on
	// internals: what the operator is told is the thing that was wrong.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	if runErr != nil {
		t.Fatalf("command returned error: %v (output %s)", runErr, out.String())
	}
	return out.String()
}

func TestVantaPushReadsThePersistedGovernanceStore(t *testing.T) {
	vantaPeriod = currentPeriod()
	vantaRepo = "acme/widget"
	vantaEvType = "article12"
	vantaDryRun = true
	t.Cleanup(func() { vantaPeriod, vantaRepo, vantaEvType, vantaDryRun = "", "", "article12", false })

	out := runInRepo(t, func() error {
		return runVantaPush(newTestCommand(), nil)
	})

	if strings.Contains(out, "No evidence to push") {
		t.Errorf("Vanta push found no evidence in a repository with a published release: "+
			"the evidence is being generated from a fresh in-memory store rather than the "+
			"persisted governance history, so what reaches the auditor is an empty record "+
			"presented as a complete one.\noutput: %s", out)
	}
	if !strings.Contains(out, "evidence record") {
		t.Errorf("output does not report any prepared evidence: %s", out)
	}
}

func TestDrataPushReadsThePersistedGovernanceStore(t *testing.T) {
	drataPeriod = currentPeriod()
	drataRepo = "acme/widget"
	drataEvType = "article12"
	drataDryRun = true
	t.Cleanup(func() { drataPeriod, drataRepo, drataEvType, drataDryRun = "", "", "article12", false })

	out := runInRepo(t, func() error {
		return runDrataPush(newTestCommand(), nil)
	})

	if strings.Contains(out, "No evidence to push") {
		t.Errorf("Drata push found no evidence in a repository with a published release: "+
			"same defect as the Vanta path — evidence generated from an empty store is "+
			"pushed to a compliance platform as this organization's record.\noutput: %s", out)
	}
	if !strings.Contains(out, "evidence record") {
		t.Errorf("output does not report any prepared evidence: %s", out)
	}
}
