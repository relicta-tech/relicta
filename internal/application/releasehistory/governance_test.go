package releasehistory

// governance_test.go is the round trip for the half of the audit trail that would otherwise be
// orphaned by a backend switch.
//
// The failure it guards against is not an error: it is `relicta history` reporting nothing in
// a repository with three years of releases, because the operator changed one config key and
// the importer only knew about release runs.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

func TestAGovernanceHistoryRoundTripsThroughTheImporter(t *testing.T) {
	ctx := context.Background()
	source := sourceWithGovernanceHistory(t)
	destination := memory.NewInMemoryStore()

	report, err := ImportGovernance(ctx, source, destination, Options{})
	if err != nil {
		t.Fatalf("ImportGovernance: %v", err)
	}

	if report.Releases != 2 || report.Incidents != 1 || report.Decisions != 1 ||
		report.Authorizations != 1 {
		t.Errorf("report says %s; want 2 releases, 1 incident, 1 decision, 1 authorization: "+
			"an operator checks the migration happened against these counts",
			countsFor(report))
	}

	// Read the records back out of the destination rather than trusting the counts. An
	// importer that moved IDs and dropped what the records contain would pass any test that
	// counted rows, and a governance record without its risk score and decision is not a
	// governance record.
	history, err := destination.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory in the destination: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("the destination holds %d release records, want 2", len(history))
	}
	newest := history[0]
	if newest.ID != "rel-2" || newest.Version != "1.1.0" || newest.RiskScore != 0.7 ||
		newest.Decision != cgp.DecisionApproved || newest.Outcome != memory.OutcomeSuccess {
		t.Errorf("the newest record came back as %+v, with fields lost in transit: the risk "+
			"score and decision are the record", newest)
	}

	// Actor metrics are derived rather than copied, so this asserts they can be derived at
	// all in the destination — the number reputation and the autonomy budget are read from.
	metrics, err := destination.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics in the destination: %v — an imported history whose actors "+
			"are unknown scores every one of them as having no record", err)
	}
	if metrics.TotalReleases != 2 || metrics.IncidentCount != 1 {
		t.Errorf("the imported actor has %d releases and %d incidents, want 2 and 1",
			metrics.TotalReleases, metrics.IncidentCount)
	}

	incidents, err := destination.GetIncidentHistory(ctx, "owner/repo", 10)
	if err != nil || len(incidents) != 1 || incidents[0].ID != "inc-1" {
		t.Errorf("incident history in the destination is %v (%v), want the one incident that "+
			"was recorded", incidents, err)
	}

	decision, err := destination.GetDecision(ctx, "dec-1")
	if err != nil || decision == nil || decision.ProposalID != "prop-1" {
		t.Errorf("GetDecision in the destination returned %+v (%v): a decision that did not "+
			"survive the import is an audit record that no longer exists", decision, err)
	}

	auth, err := destination.GetAuthorization(ctx, "auth-1")
	if err != nil || auth == nil || auth.DecisionID != "dec-1" {
		t.Errorf("GetAuthorization in the destination returned %+v (%v)", auth, err)
	}
}

// Idempotent, which is what an operator relies on when an import failed halfway and they run
// it again.
func TestASecondGovernanceImportDoesNotDuplicateTheHistory(t *testing.T) {
	ctx := context.Background()
	source := sourceWithGovernanceHistory(t)
	destination := memory.NewInMemoryStore()

	for i := range 2 {
		if _, err := ImportGovernance(ctx, source, destination, Options{}); err != nil {
			t.Fatalf("ImportGovernance run %d: %v", i+1, err)
		}
	}

	history, err := destination.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("the destination holds %d release records after two imports, want 2: a "+
			"second import has duplicated the history", len(history))
	}

	metrics, err := destination.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.TotalReleases != 2 {
		t.Errorf("the actor has %d releases after two imports, want 2: their record is "+
			"inflated, and it is the number that decides whether their next change is "+
			"auto-approved", metrics.TotalReleases)
	}
}

// Non-destructive: the source is only ever read. ADR-013 says memory.json stays as an export
// until the operator removes it, and it is the one copy they can fall back to.
func TestAGovernanceImportLeavesTheSourceUntouched(t *testing.T) {
	ctx := context.Background()
	source := sourceWithGovernanceHistory(t)

	if _, err := ImportGovernance(ctx, source, memory.NewInMemoryStore(), Options{}); err != nil {
		t.Fatalf("ImportGovernance: %v", err)
	}

	history, err := source.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory in the source: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("the source holds %d release records after the import, want 2", len(history))
	}
}

// Deployments cannot move, and the whole point is that this is said out loud. An operator whose
// deployment frequency quietly changes after a backend switch has no way to find out why.
func TestAGovernanceImportCountsTheDeploymentsItCannotMove(t *testing.T) {
	ctx := context.Background()
	source := sourceWithGovernanceHistory(t)
	if err := source.RecordDeployment(ctx, &memory.DeploymentRecord{
		ID:          "deploy-1",
		Repository:  "owner/repo",
		Environment: "production",
		Version:     "1.1.0",
		Outcome:     memory.DeploymentSucceeded,
		Provenance:  memory.ProvenanceReported,
		DeployedAt:  time.Now(),
	}); err != nil {
		t.Fatalf("RecordDeployment: %v", err)
	}

	report, err := ImportGovernance(ctx, source, memory.NewInMemoryStore(), Options{})
	if err != nil {
		t.Fatalf("ImportGovernance: %v", err)
	}

	if report.Deployments != 1 {
		t.Errorf("the report says %d deployments were left behind, want 1: a record the "+
			"import silently drops is evidence that disappears with a success message over it",
			report.Deployments)
	}
}

// A dry run reports and writes nothing.
func TestADryRunGovernanceImportReportsTheHistoryAndWritesNothing(t *testing.T) {
	ctx := context.Background()
	source := sourceWithGovernanceHistory(t)
	destination := memory.NewInMemoryStore()

	report, err := ImportGovernance(ctx, source, destination, Options{DryRun: true})
	if err != nil {
		t.Fatalf("ImportGovernance --dry-run: %v", err)
	}

	if report.Releases != 2 || report.Incidents != 1 {
		t.Errorf("the dry run reports %s, and has to report what a real run would move",
			countsFor(report))
	}

	history, err := destination.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("the destination holds %d records after a dry run: --dry-run promises to "+
			"write nothing", len(history))
	}
}

// A repository that has never released is not an error state — a migration script running this
// across every repository an organization owns will meet one.
func TestImportingAnEmptyGovernanceStoreReportsNothingToDo(t *testing.T) {
	report, err := ImportGovernance(context.Background(),
		newFileGovernanceSource(t), memory.NewInMemoryStore(), Options{})

	if err != nil {
		t.Fatalf("importing an empty governance store: %v — nothing to move is not a failure", err)
	}
	if report.Records() != 0 {
		t.Errorf("report says %d records for an empty store", report.Records())
	}
}

// A write that fails stops the import and says how far it got. A silent partial migration is
// the worst outcome for an audit trail: it looks like a migration.
func TestAFailedGovernanceWriteStopsTheImportAndSaysHowFarItGot(t *testing.T) {
	source := sourceWithGovernanceHistory(t)
	destination := &refusingGovernanceStore{
		Store:     memory.NewInMemoryStore(),
		failAfter: 1,
	}

	report, err := ImportGovernance(context.Background(), source, destination, Options{})

	if err == nil {
		t.Fatal("the import succeeded against a store that refused a write: a partial " +
			"migration reported as complete is how an audit trail is lost")
	}
	if report.Written() != 1 {
		t.Errorf("the report says %d records were written before the failure, want 1: an "+
			"operator told only that it failed has to go and find out what moved",
			report.Written())
	}
	if !errorMentions(err, "release", "rel-2") {
		t.Errorf("the error does not name the record that failed: %v", err)
	}
}

// refusingGovernanceStore accepts failAfter release writes and then refuses.
type refusingGovernanceStore struct {
	memory.Store
	written   int
	failAfter int
}

func (s *refusingGovernanceStore) RecordRelease(ctx context.Context, record *memory.ReleaseRecord) error {
	if s.written >= s.failAfter {
		return errors.New("disk is full")
	}
	s.written++
	return s.Store.RecordRelease(ctx, record)
}

func errorMentions(err error, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(err.Error(), part) {
			return false
		}
	}
	return true
}

func countsFor(report GovernanceReport) string {
	return fmt.Sprintf("%d releases, %d incidents, %d decisions, %d authorizations",
		report.Releases, report.Incidents, report.Decisions, report.Authorizations)
}

// newFileGovernanceSource builds an empty file-backed governance store in a temp directory.
//
// The real file store rather than a fake, because it is the only implementation that can be a
// source and the importer's whole job is to read it correctly.
func newFileGovernanceSource(t *testing.T) *memory.FileStore {
	t.Helper()

	store, err := memory.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}

// sourceWithGovernanceHistory writes one of each record type, in a repository with two releases
// and an incident against one of them.
func sourceWithGovernanceHistory(t *testing.T) *memory.FileStore {
	t.Helper()

	ctx := context.Background()
	store := newFileGovernanceSource(t)
	now := time.Now()

	for i, spec := range []struct {
		id      string
		version string
		risk    float64
		at      time.Time
	}{
		{"rel-1", "1.0.0", 0.2, now.Add(-2 * time.Hour)},
		{"rel-2", "1.1.0", 0.7, now},
	} {
		if err := store.RecordRelease(ctx, &memory.ReleaseRecord{
			ID:              spec.id,
			Repository:      "owner/repo",
			Version:         spec.version,
			Actor:           cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"},
			RiskScore:       spec.risk,
			Decision:        cgp.DecisionApproved,
			BreakingChanges: i,
			Outcome:         memory.OutcomeSuccess,
			ReleasedAt:      spec.at,
		}); err != nil {
			t.Fatalf("RecordRelease %s: %v", spec.id, err)
		}
	}

	if err := store.RecordIncident(ctx, &memory.IncidentRecord{
		ID:         "inc-1",
		Repository: "owner/repo",
		ReleaseID:  "rel-1",
		ActorID:    "human:alice",
		Version:    "1.0.0",
		DetectedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	if err := store.RecordDecision(ctx, &cgp.GovernanceDecision{
		ID:         "dec-1",
		ProposalID: "prop-1",
		Decision:   cgp.DecisionApproved,
		Timestamp:  now,
	}); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	if err := store.RecordAuthorization(ctx, &cgp.ExecutionAuthorization{
		ID:         "auth-1",
		DecisionID: "dec-1",
		Timestamp:  now,
	}); err != nil {
		t.Fatalf("RecordAuthorization: %v", err)
	}

	return store
}
