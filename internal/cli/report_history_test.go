package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// `relicta report` constructed memory.NewInMemoryStore() and handed it straight to
// the generator, with a comment saying it "can be populated by the governance
// pipeline". Nothing populated it, so every report was produced from zero records:
// `relicta report --type dora` said "Total Deployments: 0" for a repository with
// twelve published releases that `relicta history` listed.
//
// Empty is not the worst of it. SOC 2 and EU AI Act Article 12 reports are handed
// to auditors, and a clean one — no deployments, no failures — asserted from data
// that was never read is an affirmative false statement rather than a missing
// feature.
//
// The generator and the store were both fine. Nothing connected them, and no test
// covered the connection, which is why this survived.

// seedGovernanceHistory writes release records where the governance store lives
// for a repository, so a report has something true to find.
func seedGovernanceHistory(t *testing.T, repoRoot, repository string, count int) {
	t.Helper()

	storeDir := filepath.Join(repoRoot, ".relicta", "governance")
	if err := os.MkdirAll(storeDir, 0o750); err != nil {
		t.Fatalf("create store dir: %v", err)
	}
	store, err := memory.NewFileStore(storeDir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	for i := 0; i < count; i++ {
		record := &memory.ReleaseRecord{
			ID:         "run-seed-" + string(rune('a'+i)),
			Repository: repository,
			Version:    "1.0." + string(rune('0'+i)),
			Actor:      cgp.NewHumanActor("dev", "dev"),
			RiskScore:  0.1,
			Outcome:    memory.OutcomeSuccess,
			ReleasedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
		if err := store.RecordRelease(context.Background(), record); err != nil {
			t.Fatalf("RecordRelease: %v", err)
		}
	}
}

// reportOn runs the report command in a repository and returns what it printed.
func reportOn(t *testing.T, repoRoot, reportKind string) string {
	t.Helper()

	restore := struct{ typ, period, format, repo string }{
		reportType, reportPeriod, reportFormat, reportRepo,
	}
	origCfg := cfg
	t.Cleanup(func() {
		reportType, reportPeriod, reportFormat, reportRepo = restore.typ, restore.period, restore.format, restore.repo
		cfg = origCfg
	})

	cfg = config.DefaultConfig()
	reportType = reportKind
	reportFormat = "json"
	// A window wide enough that the seeded records fall inside it whenever the
	// test runs, so this does not fail on a quarter boundary.
	reportPeriod = "2000-01-01:2100-01-01"
	reportRepo = "acme/widget"

	t.Chdir(repoRoot)

	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := runReport(reportCmd, nil)
	_ = w.Close()
	os.Stdout = stdout

	outBytes, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read captured output: %v", readErr)
	}
	out := string(outBytes)
	if runErr != nil {
		t.Fatalf("runReport: %v (output: %s)", runErr, out)
	}
	return out
}

// TestReportReadsRecordedHistory is the connection that was missing: a release
// recorded by the governance pipeline has to appear in the report.
func TestReportReadsRecordedHistory(t *testing.T) {
	repoRoot := t.TempDir()
	seedGovernanceHistory(t, repoRoot, "acme/widget", 3)

	out := reportOn(t, repoRoot, "dora")

	// Parsed and read at the exact field, not searched for a substring. The first
	// version of this test asserted that "3" appeared somewhere in the JSON, and
	// passed against the broken wiring — a report full of zeros still contains a 3
	// in a timestamp. A test that cannot fail on the defect it describes is worse
	// than no test, because it certifies the defect as covered.
	got := deploymentCount(t, out)
	if got != 3 {
		t.Errorf("totalDeployments = %d, want 3: the report is not reading the "+
			"governance history", got)
	}
}

// deploymentCount reads dora.deploymentFrequency.totalDeployments.
func deploymentCount(t *testing.T, reportJSON string) int {
	t.Helper()

	var doc struct {
		DORA struct {
			DeploymentFrequency struct {
				TotalDeployments int `json:"totalDeployments"`
			} `json:"deploymentFrequency"`
		} `json:"dora"`
	}
	if err := json.Unmarshal([]byte(reportJSON), &doc); err != nil {
		t.Fatalf("the report is not valid JSON: %v (%s)", err, truncate(reportJSON, 200))
	}
	return doc.DORA.DeploymentFrequency.TotalDeployments
}

// A repository with no history must report zero — and that zero is now true,
// where before it was asserted from a store that had never been consulted. The
// distinction is the whole point: this test and the one above fail for opposite
// reasons if the store is wired wrongly.
func TestReportOnAnEmptyRepositoryReportsNothing(t *testing.T) {
	repoRoot := t.TempDir()

	out := reportOn(t, repoRoot, "dora")

	if got := deploymentCount(t, out); got != 0 {
		t.Errorf("totalDeployments = %d for a repository with no history, want 0", got)
	}
}

// SOC 2 evidence is the artifact an auditor reads, so it must carry the actual
// change log rather than an empty table under a compliant-looking heading.
func TestSOC2ReportCarriesTheChangeLog(t *testing.T) {
	repoRoot := t.TempDir()
	seedGovernanceHistory(t, repoRoot, "acme/widget", 2)

	out := reportOn(t, repoRoot, "soc2")

	if !strings.Contains(out, "run-seed-") {
		t.Errorf("the SOC 2 report contains no change requests for a repository with 2 "+
			"recorded releases — an auditor would read a clean record that was never "+
			"checked: %s", truncate(out, 400))
	}
}
