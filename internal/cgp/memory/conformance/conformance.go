// Package conformance holds the behavioral contract every governance memory Store must satisfy,
// as one suite that runs against any implementation.
//
// ADR-013 names the governance record — not only the release run — as part of the system of
// record, so the same reasoning that produced the release-run contract applies here: the store
// has a file implementation today and will have database ones, and three implementations of a
// fourteen-method interface drift silently. They drift hardest at the edges nobody writes down.
// This store has three of those already:
//
//   - an unknown repository's history is an empty slice and no error, while an unknown actor's
//     metrics are an *error*. Two adapters will not guess that pairing the same way.
//   - a limit of zero returns nothing rather than everything, which is what the reference does
//     and the opposite of how "no limit" is usually spelled.
//   - re-recording a release ID replaces it and rebuilds the actor's metrics rather than
//     accumulating twice, because the running average would otherwise count one release twice.
//
// The suite is the specification. Where the interface's documentation is silent, the file
// store's behavior is the answer, because it is what every caller in the tree was written
// against — `relicta history`, the DORA and SOC 2 reports, the deployment gate, hub sync.
package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// Factory builds a store for one test.
//
// Called once per test rather than once per suite, so a failure cannot leak state into the next
// case and an implementation holding a connection gets it closed by t.Cleanup.
type Factory func(t *testing.T) memory.Store

// Run executes the whole contract against one implementation.
func Run(t *testing.T, newStore Factory) {
	t.Helper()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t, newStore(t))
		})
	}
}

type testCase struct {
	name string
	run  func(t *testing.T, store memory.Store)
}

var cases = []testCase{
	{"a recorded release is in the repository's history", aRecordedReleaseIsInHistory},
	{"history is most recent first", historyIsMostRecentFirst},
	{"history honors the limit", historyHonorsTheLimit},
	{"a limit of zero returns nothing", aLimitOfZeroReturnsNothing},
	{"an unknown repository has an empty history", anUnknownRepositoryHasEmptyHistory},
	{"an unknown actor has no metrics", anUnknownActorHasNoMetrics},
	{"recording a release gives its actor metrics", recordingGivesTheActorMetrics},
	{"re-recording a release does not count it twice", reRecordingDoesNotDoubleCount},
	{"an incident round trips", anIncidentRoundTrips},
	{"re-recording an incident does not count it twice", reRecordingAnIncidentDoesNotDoubleCount},
	{"an incident counts whichever order it arrives in", anIncidentCountsWhicheverOrderItArrives},
	{"a decision round trips by id", aDecisionRoundTripsByID},
	{"an unknown decision is not found", anUnknownDecisionIsNotFound},
	{"decisions are findable by proposal", decisionsAreFindableByProposal},
	{"an authorization round trips by id", anAuthorizationRoundTripsByID},
	{"risk patterns need a repository with releases", riskPatternsNeedReleases},
}

const testRepo = "owner/repo"

func releaseRecord(id, version string, at time.Time) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         id,
		Repository: testRepo,
		Version:    version,
		Actor:      cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"},
		RiskScore:  0.2,
		Decision:   cgp.DecisionApproved,
		Outcome:    memory.OutcomeSuccess,
		ReleasedAt: at,
	}
}

func record(t *testing.T, store memory.Store, r *memory.ReleaseRecord) {
	t.Helper()
	if err := store.RecordRelease(context.Background(), r); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
}

func aRecordedReleaseIsInHistory(t *testing.T, store memory.Store) {
	record(t, store, releaseRecord("rel-1", "1.0.0", time.Now()))

	history, err := store.GetReleaseHistory(context.Background(), testRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 || history[0].ID != "rel-1" {
		t.Fatalf("history = %v, want the release just recorded", history)
	}
	if history[0].Version != "1.0.0" || history[0].Outcome != memory.OutcomeSuccess {
		t.Errorf("the record came back as %+v, with its version or outcome lost", history[0])
	}
}

// `relicta history` and the reports page through this, so the order is part of the answer.
func historyIsMostRecentFirst(t *testing.T, store memory.Store) {
	now := time.Now()
	record(t, store, releaseRecord("rel-old", "1.0.0", now.Add(-2*time.Hour)))
	record(t, store, releaseRecord("rel-new", "1.1.0", now))

	history, err := store.GetReleaseHistory(context.Background(), testRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history has %d entries, want 2", len(history))
	}
	if history[0].ID != "rel-new" {
		t.Errorf("history = [%s %s], want the newest first: an implementation that reverses "+
			"this shows an operator their oldest release as the current one",
			history[0].ID, history[1].ID)
	}
}

func historyHonorsTheLimit(t *testing.T, store memory.Store) {
	now := time.Now()
	for i, id := range []string{"rel-1", "rel-2", "rel-3"} {
		record(t, store, releaseRecord(id, "1.0.0", now.Add(time.Duration(i)*time.Minute)))
	}

	history, err := store.GetReleaseHistory(context.Background(), testRepo, 2)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history has %d entries for limit 2", len(history))
	}
	if history[0].ID != "rel-3" {
		t.Errorf("the limit dropped the newest rather than the oldest: got %s first", history[0].ID)
	}
}

// Surprising, and pinned because it is: the reference returns nothing for a limit of zero, not
// everything. An adapter reading zero as "unlimited" would hand a report the entire history.
func aLimitOfZeroReturnsNothing(t *testing.T, store memory.Store) {
	record(t, store, releaseRecord("rel-1", "1.0.0", time.Now()))

	history, err := store.GetReleaseHistory(context.Background(), testRepo, 0)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("a limit of zero returned %d entries, want none", len(history))
	}
}

// A repository with no history is the ordinary starting point, not a failure — every report
// runs against one at least once.
func anUnknownRepositoryHasEmptyHistory(t *testing.T, store memory.Store) {
	history, err := store.GetReleaseHistory(context.Background(), "owner/never-released", 10)

	if err != nil {
		t.Errorf("GetReleaseHistory errored for a repository with no releases: %v.\nThe "+
			"reference returns an empty slice, and callers do not distinguish the two", err)
	}
	if len(history) != 0 {
		t.Errorf("history = %v for a repository nothing was recorded against", history)
	}
}

// The other half of the asymmetry: unknown *actor* is an error, unlike unknown repository. The
// autonomy budget relies on telling "no record of this actor" apart from "this actor is clean".
func anUnknownActorHasNoMetrics(t *testing.T, store memory.Store) {
	metrics, err := store.GetActorMetrics(context.Background(), "human:nobody")

	if err == nil {
		t.Fatalf("GetActorMetrics returned %+v for an actor with no history, want an error: "+
			"an implementation answering with zeroed metrics reports an unknown actor as one "+
			"with a spotless record", metrics)
	}
}

func recordingGivesTheActorMetrics(t *testing.T, store memory.Store) {
	record(t, store, releaseRecord("rel-1", "1.0.0", time.Now()))

	metrics, err := store.GetActorMetrics(context.Background(), "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics after recording a release: %v", err)
	}
	if metrics.TotalReleases != 1 {
		t.Errorf("TotalReleases = %d after one release, want 1", metrics.TotalReleases)
	}
}

// Re-recording is how a corrected record lands. Accumulating twice would inflate the actor's
// history and the running averages built on it, which is what reputation reads.
func reRecordingDoesNotDoubleCount(t *testing.T, store memory.Store) {
	rec := releaseRecord("rel-1", "1.0.0", time.Now())
	record(t, store, rec)
	record(t, store, rec)

	history, err := store.GetReleaseHistory(context.Background(), testRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("the same release ID appears %d times, want once", len(history))
	}

	metrics, err := store.GetActorMetrics(context.Background(), "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.TotalReleases != 1 {
		t.Errorf("TotalReleases = %d after recording one release twice, want 1: the actor's "+
			"record is inflated by a correction", metrics.TotalReleases)
	}
}

func anIncidentRoundTrips(t *testing.T, store memory.Store) {
	incident := &memory.IncidentRecord{
		ID:         "inc-1",
		Repository: testRepo,
		ReleaseID:  "rel-1",
		Version:    "1.0.0",
		DetectedAt: time.Now(),
	}
	if err := store.RecordIncident(context.Background(), incident); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	history, err := store.GetIncidentHistory(context.Background(), testRepo, 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	if len(history) != 1 || history[0].ID != "inc-1" {
		t.Fatalf("incident history = %v, want the incident just recorded", history)
	}
	if history[0].ReleaseID != "rel-1" {
		t.Errorf("the incident lost its release association: %+v", history[0])
	}
}

// The same rule releases already follow, applied to incidents — and it was not followed.
//
// The file store appends here unconditionally while RecordRelease upserts by ID, an asymmetry
// inside one implementation. A retried incident, or two processes reacting to one alert, left two
// rows and incremented IncidentCount twice. The PostgreSQL store does not reproduce it, because a
// primary key makes an incident ID one row — so the backends disagreed about an actor's incident
// rate, which feeds ReliabilityScore and the autonomy budget.
//
// Pinned here rather than fixed in whichever adapter noticed, because a rule added in one place
// is how the backends came to disagree in the first place.
func reRecordingAnIncidentDoesNotDoubleCount(t *testing.T, store memory.Store) {
	record(t, store, releaseRecord("rel-1", "1.0.0", time.Now()))

	incident := &memory.IncidentRecord{
		ID:         "inc-1",
		Repository: testRepo,
		ReleaseID:  "rel-1",
		ActorID:    "human:alice",
		DetectedAt: time.Now(),
	}
	for range 2 {
		if err := store.RecordIncident(context.Background(), incident); err != nil {
			t.Fatalf("RecordIncident: %v", err)
		}
	}

	history, err := store.GetIncidentHistory(context.Background(), testRepo, 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("the same incident ID appears %d times, want once: a retry should correct the "+
			"record, not add to it", len(history))
	}

	metrics, err := store.GetActorMetrics(context.Background(), "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.IncidentCount != 1 {
		t.Errorf("IncidentCount = %d after recording one incident twice, want 1: the actor's "+
			"reliability is being scored against an incident that happened once", metrics.IncidentCount)
	}
}

// An incident and its actor's first release can arrive in either order — an incident imported
// ahead of the history it belongs to, or a first deploy that goes wrong and is recorded before
// the release it broke.
//
// The file store counted an incident only `if metrics, exists := s.actors[actorID]; exists`, so
// one arriving first was dropped and never counted, even after that actor's releases made them
// known. The count then depended on arrival order rather than on what happened.
//
// What this case does *not* assert is that an incident alone conjures an actor. An actor nobody
// has seen release anything stays unknown, which is the reference's behavior and what the
// PostgreSQL store's own test already pinned — GetActorMetrics erroring is how callers tell
// "no record of this actor" from "this actor is clean". An earlier version of this case asserted
// the opposite, on my reasoning rather than the code's, and that test caught it.
func anIncidentCountsWhicheverOrderItArrives(t *testing.T, store memory.Store) {
	ctx := context.Background()

	// Incident first, before this actor has any release at all.
	if err := store.RecordIncident(ctx, &memory.IncidentRecord{
		ID:         "inc-1",
		Repository: testRepo,
		ActorID:    "human:alice",
		DetectedAt: time.Now(),
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}
	record(t, store, releaseRecord("rel-1", "1.0.0", time.Now()))

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.IncidentCount != 1 {
		t.Errorf("IncidentCount = %d, want 1: the incident was recorded before the actor's "+
			"first release and never counted, so their reliability depends on the order the "+
			"two arrived in rather than on what happened", metrics.IncidentCount)
	}
}

func aDecisionRoundTripsByID(t *testing.T, store memory.Store) {
	decision := &cgp.GovernanceDecision{
		ID:         "dec-1",
		ProposalID: "prop-1",
		Decision:   cgp.DecisionApproved,
		Timestamp:  time.Now(),
	}
	if err := store.RecordDecision(context.Background(), decision); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	got, err := store.GetDecision(context.Background(), "dec-1")
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if got == nil || got.ID != "dec-1" || got.ProposalID != "prop-1" {
		t.Errorf("GetDecision returned %+v, want the decision just recorded", got)
	}
}

// The audit trail distinguishes "no such decision" from a broken store, and a governance tool
// answering "approved" for a decision it never recorded is the worst available failure.
func anUnknownDecisionIsNotFound(t *testing.T, store memory.Store) {
	got, err := store.GetDecision(context.Background(), "dec-never-made")

	if err == nil && got != nil {
		t.Fatalf("GetDecision invented %+v for an ID that was never recorded", got)
	}
}

func decisionsAreFindableByProposal(t *testing.T, store memory.Store) {
	for _, id := range []string{"dec-1", "dec-2"} {
		if err := store.RecordDecision(context.Background(), &cgp.GovernanceDecision{
			ID: id, ProposalID: "prop-1", Decision: cgp.DecisionApproved, Timestamp: time.Now(),
		}); err != nil {
			t.Fatalf("RecordDecision: %v", err)
		}
	}

	got, err := store.GetDecisionsByProposal(context.Background(), "prop-1")
	if err != nil {
		t.Fatalf("GetDecisionsByProposal: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d decisions for the proposal, want 2: an audit trail missing a "+
			"decision is a record of a different history", len(got))
	}
}

func anAuthorizationRoundTripsByID(t *testing.T, store memory.Store) {
	auth := &cgp.ExecutionAuthorization{
		ID:         "auth-1",
		DecisionID: "dec-1",
		Timestamp:  time.Now(),
	}
	if err := store.RecordAuthorization(context.Background(), auth); err != nil {
		t.Fatalf("RecordAuthorization: %v", err)
	}

	got, err := store.GetAuthorization(context.Background(), "auth-1")
	if err != nil {
		t.Fatalf("GetAuthorization: %v", err)
	}
	if got == nil || got.ID != "auth-1" {
		t.Errorf("GetAuthorization returned %+v, want the authorization just recorded", got)
	}

	byDecision, err := store.GetAuthorizationsByDecision(context.Background(), "dec-1")
	if err != nil {
		t.Fatalf("GetAuthorizationsByDecision: %v", err)
	}
	if len(byDecision) != 1 {
		t.Errorf("got %d authorizations for the decision, want 1", len(byDecision))
	}
}

// Patterns are derived from releases, so a repository with none has nothing to derive from —
// an error rather than a zeroed pattern, which risk scoring would read as "historically safe".
func riskPatternsNeedReleases(t *testing.T, store memory.Store) {
	patterns, err := store.GetRiskPatterns(context.Background(), "owner/never-released")

	if err == nil {
		t.Fatalf("GetRiskPatterns returned %+v for a repository with no releases, want an "+
			"error: zeroed patterns read as a clean history rather than an absent one", patterns)
	}
}
