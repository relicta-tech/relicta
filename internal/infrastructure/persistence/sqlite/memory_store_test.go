package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// Everything here is a claim the conformance suite does not make, because it is a claim
// about this adapter rather than about the port: what SQL would do with the limit if it
// were handed one, how one file keeps two repositories apart, and that metrics derived
// from rows are still the shared arithmetic rather than a SQL reimplementation of it.

const memoryTestRepo = "owner/repo"

func releaseFor(actorID, id, version string, at time.Time) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         id,
		Repository: memoryTestRepo,
		Version:    version,
		Actor:      cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman, Name: "alice"},
		RiskScore:  0.2,
		Decision:   cgp.DecisionApproved,
		Outcome:    memory.OutcomeSuccess,
		ReleasedAt: at,
	}
}

func mustRecord(t *testing.T, store memory.Store, record *memory.ReleaseRecord) {
	t.Helper()
	if err := store.RecordRelease(context.Background(), record); err != nil {
		t.Fatalf("RecordRelease %s: %v", record.ID, err)
	}
}

// The contract pins a limit of zero, and SQL agrees with it by accident. It does not
// agree about a negative one: `LIMIT -1` means *unlimited* in SQLite, so an off-by-one
// that produced a negative page size would render the entire history instead of nothing.
// The reference's arithmetic returns nothing for every limit at or below zero.
func TestALimitBelowZeroReturnsNothingRatherThanEverything(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	mustRecord(t, store, releaseFor("human:alice", "rel-1", "1.0.0", time.Now()))
	if err := store.RecordIncident(ctx, &memory.IncidentRecord{
		ID: "inc-1", Repository: memoryTestRepo, ReleaseID: "rel-1", DetectedAt: time.Now(),
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	history, err := store.GetReleaseHistory(ctx, memoryTestRepo, -1)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("a limit of -1 returned %d releases, want none: SQL read it as "+
			"unlimited and handed the caller the whole history", len(history))
	}

	incidents, err := store.GetIncidentHistory(ctx, memoryTestRepo, -1)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	if len(incidents) != 0 {
		t.Errorf("a limit of -1 returned %d incidents, want none", len(incidents))
	}
}

// The file store keys by repository, so two projects cannot see each other by
// construction. One table has to enforce that in every query, and a WHERE that was
// forgotten would put another project's releases into this project's history, reports
// and risk patterns.
func TestOneDatabaseKeepsTwoRepositoriesHistoriesApart(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	mine := releaseFor("human:alice", "rel-mine", "1.0.0", time.Now())
	theirs := releaseFor("human:alice", "rel-theirs", "2.0.0", time.Now())
	theirs.Repository = "owner/other"
	theirs.RiskScore = 0.9
	mustRecord(t, store, mine)
	mustRecord(t, store, theirs)

	history, err := store.GetReleaseHistory(ctx, memoryTestRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 || history[0].ID != "rel-mine" {
		t.Errorf("history for one repository is %v; the other repository's releases are "+
			"visible in this one's history", history)
	}

	patterns, err := store.GetRiskPatterns(ctx, memoryTestRepo)
	if err != nil {
		t.Fatalf("GetRiskPatterns: %v", err)
	}
	if patterns.TotalReleases != 1 {
		t.Errorf("risk patterns counted %d releases, want 1: another project's risk "+
			"score is being scored against this one's changes", patterns.TotalReleases)
	}

	// An actor's record, in contrast, follows the actor across repositories — the same
	// person releasing two projects has one reputation.
	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.TotalReleases != 2 {
		t.Errorf("TotalReleases = %d, want 2: an actor's history stops at a repository "+
			"boundary that reputation does not have", metrics.TotalReleases)
	}
}

// The metrics have no table of their own, so this is the claim that they are still
// there after the process that computed them is gone: a new invocation derives the same
// numbers from the rows.
func TestActorMetricsSurviveAReopenBecauseTheyAreDerived(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	ctx := context.Background()

	first, err := sqlite.OpenMemoryStore(ctx, path)
	if err != nil {
		t.Fatalf("OpenMemoryStore: %v", err)
	}
	mustRecord(t, first, releaseFor("human:alice", "rel-1", "1.0.0", time.Now()))
	rolled := releaseFor("human:alice", "rel-2", "1.1.0", time.Now())
	rolled.Outcome = memory.OutcomeRollback
	mustRecord(t, first, rolled)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	metrics, err := newMemoryStore(t, path).GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics after reopening: %v; the next relicta invocation "+
			"cannot see the record the last one wrote", err)
	}
	if metrics.TotalReleases != 2 || metrics.SuccessfulReleases != 1 || metrics.RollbackCount != 1 {
		t.Errorf("metrics after a reopen are %d releases, %d successful, %d rolled back; "+
			"want 2, 1, 1", metrics.TotalReleases, metrics.SuccessfulReleases,
			metrics.RollbackCount)
	}
	if metrics.SuccessRate != 0.5 {
		t.Errorf("SuccessRate = %v, want 0.5: the rate autonomy is granted on does not "+
			"survive the process that recorded the releases", metrics.SuccessRate)
	}
}

// Canceled runs are the case that proves the numbers come from Accumulate rather than
// from SQL aggregates. A cancel is in the store for audit and out of every rate computed
// over releases, so COUNT(*) and AVG(risk_score) would both be wrong here — and wrong in
// the direction that makes a team which cancels carefully look worse than one that ships
// everything.
func TestACanceledRunIsInTheHistoryAndOutOfTheRates(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	mustRecord(t, store, releaseFor("human:alice", "rel-1", "1.0.0", time.Now()))
	canceled := releaseFor("human:alice", "rel-2", "1.1.0", time.Now())
	canceled.Outcome = memory.OutcomeCanceled
	canceled.RiskScore = 0.9
	mustRecord(t, store, canceled)

	history, err := store.GetReleaseHistory(ctx, memoryTestRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("history has %d entries, want 2: a canceled run belongs in the audit "+
			"record even though it never shipped", len(history))
	}

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.TotalReleases != 1 {
		t.Errorf("TotalReleases = %d, want 1: the canceled run entered the denominator, "+
			"so deciding not to ship damaged the actor's record", metrics.TotalReleases)
	}
	if metrics.AverageRiskScore != 0.2 {
		t.Errorf("AverageRiskScore = %v, want 0.2: the canceled run's risk was averaged "+
			"into a history it was never part of", metrics.AverageRiskScore)
	}
}

// A correction replaces the record in place in the reference, which keeps its position
// in the history. Here that is the row keeping its recorded_seq, and an upsert that
// deleted and reinserted instead would silently move a corrected release to the top of
// `relicta history`.
func TestACorrectedReleaseKeepsItsPlaceInTheHistory(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	now := time.Now()
	mustRecord(t, store, releaseFor("human:alice", "rel-1", "1.0.0", now))
	mustRecord(t, store, releaseFor("human:alice", "rel-2", "1.1.0", now.Add(time.Minute)))

	corrected := releaseFor("human:alice", "rel-1", "1.0.1", now)
	corrected.Outcome = memory.OutcomeRollback
	mustRecord(t, store, corrected)

	history, err := store.GetReleaseHistory(ctx, memoryTestRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history has %d entries after a correction, want 2", len(history))
	}
	if history[0].ID != "rel-2" {
		t.Errorf("history = [%s %s], want [rel-2 rel-1]: correcting the older release "+
			"moved it to the top and made it look like the current one",
			history[0].ID, history[1].ID)
	}
	if history[1].Version != "1.0.1" || history[1].Outcome != memory.OutcomeRollback {
		t.Errorf("the corrected record came back as %+v, still carrying what it "+
			"replaced", history[1])
	}
}

// The file store increments an actor's incident count only if that actor already has a
// metrics entry, so an incident recorded before their first release is lost until
// something forces a rebuild. RebuildActorMetrics — the definition both stores share —
// counts every incident the actor owns, and deriving on read means this store always
// answers with that definition rather than with whichever path last touched a counter.
func TestAnIncidentCountsAgainstTheActorWhoseReleaseItFollowed(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	mustRecord(t, store, releaseFor("human:alice", "rel-1", "1.0.0", time.Now()))
	if err := store.RecordIncident(ctx, &memory.IncidentRecord{
		ID:         "inc-1",
		Repository: memoryTestRepo,
		ReleaseID:  "rel-1",
		ActorID:    "human:alice",
		DetectedAt: time.Now(),
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.IncidentCount != 1 {
		t.Errorf("IncidentCount = %d after one incident, want 1: the actor's "+
			"reliability score is computed as though the incident never happened",
			metrics.IncidentCount)
	}
}

// UpdateActorMetrics has no counter to nudge in a derived store, so it corrects the
// release the counters come from. The resulting numbers are the reference's, and the
// history no longer contradicts them.
func TestARollbackReportedAfterTheFactLandsOnTheRelease(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	mustRecord(t, store, releaseFor("human:alice", "rel-1", "1.0.0", time.Now()))
	if err := store.UpdateActorMetrics(ctx, "human:alice", memory.OutcomeRollback); err != nil {
		t.Fatalf("UpdateActorMetrics: %v", err)
	}

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.RollbackCount != 1 || metrics.FailedReleases != 1 || metrics.SuccessfulReleases != 0 {
		t.Errorf("after a reported rollback the actor has %d rollbacks, %d failures and "+
			"%d successes; want 1, 1, 0", metrics.RollbackCount, metrics.FailedReleases,
			metrics.SuccessfulReleases)
	}

	history, err := store.GetReleaseHistory(ctx, memoryTestRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 || history[0].Outcome != memory.OutcomeRollback {
		t.Errorf("the release still reads %v in the history while the actor's metrics "+
			"count a rollback; one of the two is telling an auditor the wrong story",
			history[0].Outcome)
	}

	if err := store.UpdateActorMetrics(ctx, "human:nobody", memory.OutcomeRollback); err == nil {
		t.Error("UpdateActorMetrics accepted an actor with no releases, so a typo in an " +
			"actor ID is recorded as a rollback nobody can find")
	}
}

// An audit trail is read by a human and by the SOC 2 report, and the reference builds it
// by ranging over a Go map — an order that is randomized per iteration, so nothing can
// depend on it. Ordering by decision time is what makes the trail readable, and the
// bounds are its ends.
func TestAnAuditTrailReadsInTheOrderTheDecisionsWereMade(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	base := time.Now().Truncate(time.Second)
	for i, id := range []string{"dec-3", "dec-1", "dec-2"} {
		offsets := map[string]time.Duration{"dec-1": 0, "dec-2": time.Minute, "dec-3": 2 * time.Minute}
		if err := store.RecordDecision(ctx, &cgp.GovernanceDecision{
			ID:         id,
			ProposalID: "prop-1",
			Decision:   cgp.DecisionApproved,
			Timestamp:  base.Add(offsets[id]),
		}); err != nil {
			t.Fatalf("RecordDecision %d: %v", i, err)
		}
	}
	authorizedAt := base.Add(5 * time.Minute)
	if err := store.RecordAuthorization(ctx, &cgp.ExecutionAuthorization{
		ID: "auth-1", DecisionID: "dec-3", ProposalID: "prop-1", Timestamp: authorizedAt,
	}); err != nil {
		t.Fatalf("RecordAuthorization: %v", err)
	}

	trail, err := store.GetAuditTrail(ctx, "prop-1")
	if err != nil {
		t.Fatalf("GetAuditTrail: %v", err)
	}
	got := []string{trail.Decisions[0].ID, trail.Decisions[1].ID, trail.Decisions[2].ID}
	want := []string{"dec-1", "dec-2", "dec-3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the trail reads %v, want %v: an audit trail out of order describes "+
				"a different sequence of events than the one that happened", got, want)
		}
	}
	if len(trail.Authorizations) != 1 {
		t.Errorf("the trail carries %d authorizations, want 1: an approval that granted "+
			"execution is missing from the record of it", len(trail.Authorizations))
	}
	if !trail.CreatedAt.Equal(base) {
		t.Errorf("CreatedAt = %v, want the first decision at %v", trail.CreatedAt, base)
	}
	if !trail.UpdatedAt.Equal(authorizedAt) {
		t.Errorf("UpdatedAt = %v, want the authorization at %v: the trail claims to end "+
			"before its last entry", trail.UpdatedAt, authorizedAt)
	}
}

// A proposal that was never decided is an error and not an empty trail, because "here is
// the trail, it is empty" reads as "nobody decided this" when the truth may be that this
// store never saw it.
func TestAnUnknownProposalHasNoAuditTrail(t *testing.T) {
	store := newMemoryStore(t, filepath.Join(t.TempDir(), "relicta.db"))

	trail, err := store.GetAuditTrail(context.Background(), "prop-never-proposed")
	if err == nil {
		t.Fatalf("GetAuditTrail invented %+v for a proposal nothing was recorded against", trail)
	}
}

// One backend means one file. ADR-013's reason for a database at all is that a run and
// the governance record it produces can be written together, and they cannot be if the
// two stores opened different databases.
func TestTheRunsAndTheGovernanceRecordShareOneDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")

	runs := newStore(t, path)
	governance := newMemoryStore(t, path)
	mustSave(t, runs, plannedRun(t, t.TempDir(), "run-1"))
	mustRecord(t, governance, releaseFor("human:alice", "rel-1", "1.0.0", time.Now()))

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening the database directly: %v", err)
	}
	defer func() { _ = db.Close() }()

	var runCount, releaseCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM release_runs`).Scan(&runCount); err != nil {
		t.Fatalf("counting release_runs: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM governance_releases`).Scan(&releaseCount); err != nil {
		t.Fatalf("counting governance_releases: %v", err)
	}
	if runCount != 1 || releaseCount != 1 {
		t.Errorf("one file holds %d runs and %d release records, want 1 and 1: the two "+
			"halves of the system of record are in different databases and cannot be "+
			"written in one transaction", runCount, releaseCount)
	}
}

// The governance record is written from more places than the run is — the outcome
// tracker, the CLI's own recordPublishOutcome, hub sync — and in CI two of them run in
// different processes. This store therefore needs the same pragmas as the run store for
// the same reason, and this is the test that says so: without WAL and a busy timeout the
// loser of any overlap gets SQLITE_BUSY and its command fails.
func TestConcurrentWritersOnTheGovernanceRecordAllSucceed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	first := newMemoryStore(t, path)
	second := newMemoryStore(t, path)

	const writersPerStore, recordsPerWriter = 4, 25

	type writer struct {
		store *sqlite.MemoryStore
		id    string
	}
	writers := make([]writer, 0, 2*writersPerStore)
	for storeIndex, store := range []*sqlite.MemoryStore{first, second} {
		for i := range writersPerStore {
			writers = append(writers, writer{store: store, id: fmt.Sprintf("rel-p%d-w%d", storeIndex, i)})
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(writers)*recordsPerWriter)

	for _, w := range writers {
		wg.Add(1)
		go func(w writer) {
			defer wg.Done()
			// The same release ID re-recorded, which is the contended path: every write
			// is an upsert onto one row rather than an insert of a fresh one.
			record := releaseFor("human:alice", w.id, "1.0.0", time.Now())
			for range recordsPerWriter {
				if err := w.store.RecordRelease(context.Background(), record); err != nil {
					errCh <- fmt.Errorf("%s: %w", w.id, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("a concurrent RecordRelease failed: %v; a second relicta process in the "+
			"same repository makes the first one's command fail", err)
	}

	history, err := second.GetReleaseHistory(context.Background(), memoryTestRepo, 100)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != len(writers) {
		t.Errorf("the database holds %d releases, want %d: either a write from one "+
			"process is invisible to the other, or re-recording one release stored it "+
			"more than once", len(history), len(writers))
	}
}
