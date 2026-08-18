package postgres_test

// governance_memory_store_test.go runs the ADR-013 governance memory conformance suite
// against a real PostgreSQL, plus the claims the suite does not make because they belong
// to a backend a whole team shares: that one database keeps repositories apart, that two
// processes recording one release leave one release, and that the actor metrics this
// adapter derives rather than stores follow the records they are derived from.
//
// Gating follows testcontainer_test.go exactly — short mode skips, an unreachable Docker
// daemon skips, anything else fails. CI without a database must not go red, and must not
// go green pretending the suite ran either.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory/conformance"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres"
)

// TestThePostgresGovernanceMemoryStoreSatisfiesTheContract is the bar this adapter had to
// clear.
//
// The file store is the reference implementation — `relicta history`, the DORA and SOC 2
// reports, the deployment gate and hub sync were all written against it — so a postgres
// store that disagrees with it is wrong however defensible its answer looks alone.
//
// One container serves every case. The conformance factory is documented as running once
// per test so a failure cannot leak into the next case, and that isolation is preserved
// without paying for fourteen containers: the suite records everything under one fixed
// repository, so the tables are emptied after each case instead.
func TestThePostgresGovernanceMemoryStoreSatisfiesTheContract(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	conformance.Run(t, func(t *testing.T) cgpmemory.Store {
		t.Cleanup(func() { truncateGovernanceMemory(t, pool) })
		return postgres.NewGovernanceMemoryStore(pool)
	})
}

// TestOneDatabaseKeepsGovernanceRecordsApart is the difference between this backend and
// the file one, which gets the separation free from having a memory.json per checkout.
//
// Here every row lives in one table, so a missing WHERE clause does not fail — it returns
// somebody else's release. Two teams sharing a database would read each other's history
// in `relicta history`, and the SOC 2 report would attest to releases the repository it
// names never had.
func TestOneDatabaseKeepsGovernanceRecordsApart(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	store := postgres.NewGovernanceMemoryStore(pool)
	now := time.Now()

	mustRecordRelease(t, ctx, store, governanceRelease("rel-in-a", "owner/a", "human:alice", now))
	mustRecordRelease(t, ctx, store, governanceRelease("rel-in-b", "owner/b", "human:bob", now))

	history, err := store.GetReleaseHistory(ctx, "owner/a", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 || history[0].ID != "rel-in-a" {
		t.Errorf("owner/a's history is %v, want only rel-in-a: a team sharing this "+
			"database would read another team's releases", history)
	}

	patterns, err := store.GetRiskPatterns(ctx, "owner/a")
	if err != nil {
		t.Fatalf("GetRiskPatterns: %v", err)
	}
	if patterns.TotalReleases != 1 {
		t.Errorf("risk patterns for owner/a analyzed %d releases, want 1: the risk score "+
			"of one repository would be set by another's history", patterns.TotalReleases)
	}
}

// TestConcurrentRecordingsOfOneReleaseLeaveOneRelease covers the case this backend exists
// for, and the case that decided against a stored ActorMetrics.
//
// A team sharing governance state has several processes finishing releases at once, and a
// single publish is already recorded twice — by the outcome tracker and by the CLI. Two
// failures are possible and both are silent. A second row would list the release twice and
// count the actor's release twice; a lost update to a materialized metrics row would leave
// the actor's totals below their history, and that number is what the autonomy budget
// reads before auto-approving their next change.
func TestConcurrentRecordingsOfOneReleaseLeaveOneRelease(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	store := postgres.NewGovernanceMemoryStore(pool)
	released := time.Now()

	const writers = 8
	var wg sync.WaitGroup
	errs := make([]error, writers)

	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Separate records carrying one identity, which is what two processes each
			// holding their own copy of the release actually looks like. Half report a
			// rollback, so the writers genuinely disagree about the row.
			record := governanceRelease("rel-contended", "owner/repo", "human:alice", released)
			if i%2 == 0 {
				record.Outcome = cgpmemory.OutcomeRollback
			}
			errs[i] = store.RecordRelease(ctx, record)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: RecordRelease under contention failed: %v", i, err)
		}
	}

	history, err := store.GetReleaseHistory(ctx, "owner/repo", 100)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("%d records are stored after %d concurrent recordings of one release, "+
			"want 1: the audit trail holds several records of a single release",
			len(history), writers)
	}

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.TotalReleases != 1 {
		t.Errorf("TotalReleases = %d after %d concurrent recordings of one release, want "+
			"1: the actor's record is inflated by writers that raced",
			metrics.TotalReleases, writers)
	}
	// Last writer wins, so either outcome is correct — metrics that agree with neither
	// are not. That is what a lost update to a stored ActorMetrics would look like.
	if metrics.SuccessfulReleases+metrics.FailedReleases != 1 {
		t.Errorf("the actor has %d successes and %d failures over 1 release: the metrics "+
			"describe a history that was never recorded",
			metrics.SuccessfulReleases, metrics.FailedReleases)
	}
}

// TestCorrectingAReleaseCorrectsTheActorsMetrics is why the re-record case passes.
//
// It passes structurally rather than procedurally: metrics are derived from the release
// rows, so a corrected record cannot leave a stale contribution behind and there is no
// rebuild anyone can forget to trigger. Asserting only that the total stays at one — which
// is all the conformance case can portably ask — would also pass on a store that ignored
// the correction entirely, so this checks the correction actually landed.
func TestCorrectingAReleaseCorrectsTheActorsMetrics(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	store := postgres.NewGovernanceMemoryStore(pool)
	released := time.Now()

	mustRecordRelease(t, ctx, store, governanceRelease("rel-1", "owner/repo", "human:alice", released))

	before, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if before.SuccessfulReleases != 1 || before.FailedReleases != 0 {
		t.Fatalf("metrics after one successful release are %+v, want one success and no "+
			"failures", before)
	}

	// The same release, now known to have been rolled back.
	corrected := governanceRelease("rel-1", "owner/repo", "human:alice", released)
	corrected.Outcome = cgpmemory.OutcomeRollback
	mustRecordRelease(t, ctx, store, corrected)

	after, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics after the correction: %v", err)
	}
	if after.TotalReleases != 1 {
		t.Errorf("TotalReleases = %d after correcting one release, want 1", after.TotalReleases)
	}
	if after.RollbackCount != 1 || after.FailedReleases != 1 || after.SuccessfulReleases != 0 {
		t.Errorf("metrics after the correction are %+v, want the rollback counted and the "+
			"success withdrawn: the actor keeps a clean record for a release that was "+
			"rolled back, and earned trust reads that as grounds to widen their autonomy",
			after)
	}
	if after.SuccessRate != 0 {
		t.Errorf("SuccessRate = %v after the only release rolled back, want 0", after.SuccessRate)
	}

	// UpdateActorMetrics is the method that has nowhere to write once metrics are derived.
	// Its one observable promise is the same asymmetry GetActorMetrics keeps — an actor
	// nobody has a record of is an error, not a blank slate — and that survives. What it
	// must not do is invent a second, writable copy of numbers the records already fix.
	if err := store.UpdateActorMetrics(ctx, "human:nobody", cgpmemory.OutcomeRollback); err == nil {
		t.Error("UpdateActorMetrics succeeded for an actor with no history; a caller " +
			"cannot tell an unrecorded actor from one whose record is clean")
	}
	if err := store.UpdateActorMetrics(ctx, "human:alice", cgpmemory.OutcomeRollback); err != nil {
		t.Errorf("UpdateActorMetrics for a known actor: %v", err)
	}

	unchanged, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if unchanged.RollbackCount != after.RollbackCount || unchanged.TotalReleases != after.TotalReleases {
		t.Errorf("metrics moved to %+v after UpdateActorMetrics, from %+v: the derived "+
			"numbers now disagree with the release records they are derived from",
			unchanged, after)
	}
}

// TestAnIncidentCountsAgainstItsActorWhicheverOrderItArrivesIn pins a deliberate
// difference from the file store.
//
// The file store increments a materialized IncidentCount when the incident is recorded,
// and only when the actor already has metrics — so an incident that arrives before the
// actor's first release never reaches their record at all, and no later release brings it
// back. That is an artifact of materializing, not a decision: RebuildActorMetrics counts
// incidents precisely because "a rebuild that ignored them would silently reset an actor's
// incident history to zero". Deriving on read has no order to get wrong.
func TestAnIncidentCountsAgainstItsActorWhicheverOrderItArrivesIn(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	store := postgres.NewGovernanceMemoryStore(pool)
	now := time.Now()

	// The incident first, before the actor has any release at all.
	if err := store.RecordIncident(ctx, &cgpmemory.IncidentRecord{
		ID:         "inc-1",
		Repository: "owner/repo",
		ReleaseID:  "rel-1",
		Version:    "1.0.0",
		ActorID:    "human:alice",
		DetectedAt: now,
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	// An actor with only incidents is still unknown: metrics are derived from releases,
	// and the contract's asymmetry says an unknown actor is an error rather than zeroes.
	if metrics, err := store.GetActorMetrics(ctx, "human:alice"); err == nil {
		t.Errorf("GetActorMetrics returned %+v for an actor with an incident and no "+
			"releases, want an error: an actor nobody has seen release anything would "+
			"be reported as one with a record", metrics)
	}

	mustRecordRelease(t, ctx, store, governanceRelease("rel-1", "owner/repo", "human:alice", now))

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.IncidentCount != 1 {
		t.Errorf("IncidentCount = %d for an actor with one incident, want 1: the incident "+
			"is missing from the reliability score because it was recorded first",
			metrics.IncidentCount)
	}
}

// TestANegativeLimitReturnsNothingRatherThanFailing covers the other side of the pinned
// limit-of-zero edge.
//
// The contract pins zero as "nothing", and SQL agrees. It does not agree below zero:
// PostgreSQL rejects a negative LIMIT outright, and the reference does not survive one
// either — it sizes a slice with a negative capacity and panics. Neither is an answer a
// report can render, so the adapter treats any non-positive limit as the pinned one.
func TestANegativeLimitReturnsNothingRatherThanFailing(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	// The database's own answer, which is what makes the guard load-bearing rather than
	// defensive: without it this error reaches the caller.
	if _, err := pool.Exec(ctx, `SELECT 1 LIMIT -1`); err == nil {
		t.Error("PostgreSQL accepted a negative LIMIT; the guard in GetReleaseHistory is " +
			"documented as standing in for an error the database raises")
	}

	store := postgres.NewGovernanceMemoryStore(pool)
	now := time.Now()

	mustRecordRelease(t, ctx, store, governanceRelease("rel-1", "owner/repo", "human:alice", now))
	if err := store.RecordIncident(ctx, &cgpmemory.IncidentRecord{
		ID: "inc-1", Repository: "owner/repo", DetectedAt: now,
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	releases, err := store.GetReleaseHistory(ctx, "owner/repo", -1)
	if err != nil {
		t.Fatalf("GetReleaseHistory with a negative limit: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("a negative limit returned %d releases, want none: a caller that computed "+
			"its limit and got it wrong would be handed the whole history", len(releases))
	}

	incidents, err := store.GetIncidentHistory(ctx, "owner/repo", -1)
	if err != nil {
		t.Fatalf("GetIncidentHistory with a negative limit: %v", err)
	}
	if len(incidents) != 0 {
		t.Errorf("a negative limit returned %d incidents, want none", len(incidents))
	}
}

// TestAnAuditTrailGathersTheProposalsDecisionsAndAuthorizations covers the one method the
// conformance suite does not reach.
//
// GetAuditTrail is what `relicta audit` and the SOC 2 report render, and it is the only
// read that spans two tables. A join that dropped the authorizations would produce a trail
// showing a decision that was never acted on.
func TestAnAuditTrailGathersTheProposalsDecisionsAndAuthorizations(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	store := postgres.NewGovernanceMemoryStore(pool)
	decidedAt := time.Now()

	if err := store.RecordDecision(ctx, &cgp.GovernanceDecision{
		ID: "dec-1", ProposalID: "prop-1", Decision: cgp.DecisionApproved, Timestamp: decidedAt,
	}); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}
	// A decision on a different proposal, which must not appear in this trail.
	if err := store.RecordDecision(ctx, &cgp.GovernanceDecision{
		ID: "dec-other", ProposalID: "prop-2", Decision: cgp.DecisionApproved, Timestamp: decidedAt,
	}); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}
	authorizedAt := decidedAt.Add(time.Minute)
	if err := store.RecordAuthorization(ctx, &cgp.ExecutionAuthorization{
		ID: "auth-1", DecisionID: "dec-1", ProposalID: "prop-1", Timestamp: authorizedAt,
	}); err != nil {
		t.Fatalf("RecordAuthorization: %v", err)
	}
	// The other proposal's authorization exists so the join has something to get wrong.
	// Without it, a join matching every decision to every authorization still returns one
	// row here and the case passes on a store that had lost the association entirely.
	if err := store.RecordAuthorization(ctx, &cgp.ExecutionAuthorization{
		ID: "auth-other", DecisionID: "dec-other", ProposalID: "prop-2", Timestamp: authorizedAt,
	}); err != nil {
		t.Fatalf("RecordAuthorization: %v", err)
	}

	trail, err := store.GetAuditTrail(ctx, "prop-1")
	if err != nil {
		t.Fatalf("GetAuditTrail: %v", err)
	}
	if len(trail.Decisions) != 1 || trail.Decisions[0].ID != "dec-1" {
		t.Errorf("the trail holds %v, want only the proposal's own decision", trail.Decisions)
	}
	if len(trail.Authorizations) != 1 || trail.Authorizations[0].ID != "auth-1" {
		t.Errorf("the trail holds %v authorizations, want the one granted under dec-1: a "+
			"trail showing a decision nobody acted on is a record of a different history",
			trail.Authorizations)
	}
	if !trail.UpdatedAt.Equal(authorizedAt) {
		t.Errorf("UpdatedAt = %v, want the authorization at %v: the trail is dated before "+
			"its own last entry", trail.UpdatedAt, authorizedAt)
	}

	if _, err := store.GetAuditTrail(ctx, "prop-never-proposed"); err == nil {
		t.Error("GetAuditTrail succeeded for a proposal nothing was decided on; an empty " +
			"trail reads as a release that passed governance with nobody deciding anything")
	}
}

// governanceRelease builds a successful release record for one actor.
func governanceRelease(id, repository, actorID string, at time.Time) *cgpmemory.ReleaseRecord {
	return &cgpmemory.ReleaseRecord{
		ID:         id,
		Repository: repository,
		Version:    "1.0.0",
		Actor:      cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman, Name: "alice"},
		RiskScore:  0.2,
		Decision:   cgp.DecisionApproved,
		Outcome:    cgpmemory.OutcomeSuccess,
		ReleasedAt: at,
	}
}

func mustRecordRelease(
	t *testing.T, ctx context.Context, store cgpmemory.Store, record *cgpmemory.ReleaseRecord,
) {
	t.Helper()
	if err := store.RecordRelease(ctx, record); err != nil {
		t.Fatalf("RecordRelease(%s): %v", record.ID, err)
	}
}

// truncateGovernanceMemory empties the governance tables between cases sharing one
// container.
//
// Registered on the subtest, not the parent: a t.Cleanup on the parent would run after its
// deferred pool.Close and truncate through a closed pool. The tests that own their
// container do not need this — terminating it takes the rows with it.
func truncateGovernanceMemory(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, `TRUNCATE governance_releases, governance_incidents,
		governance_decisions, governance_authorizations, governance_audit_entries`); err != nil {
		t.Fatalf("truncating governance tables: %v; a later case would read this one's rows", err)
	}
}
