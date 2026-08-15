// Package conformance holds the behavioral contract every ReleaseRunRepository must satisfy,
// as one suite that runs against any implementation.
//
// ADR-013 puts three adapters behind persistence.backend — file, sqlite and postgres. Three
// implementations of one port drift: a query that filters in Go and a query that filters in SQL
// disagree about ordering, about what an absent row means, about whether a state comparison is
// case sensitive, and nobody notices until an operator switches backend and the behavior
// changes underneath them. A shared suite executed once per adapter is the only thing that
// keeps them honest.
//
// The suite is the specification. Where the port's documentation is silent — does Load return
// an error or a nil run for an unknown ID? — the file adapter's long-standing behavior is the
// answer, because it is what every caller in the tree was written against. A new adapter that
// disagrees is wrong even if its answer is more defensible in isolation.
package conformance

import (
	"context"
	"errors"
	"testing"

	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Factory builds a repository for one test, along with the repository root it serves.
//
// Called once per test rather than once per suite, so a failure cannot leak state into the next
// case and adapters that hold a connection get it closed by t.Cleanup.
type Factory func(t *testing.T) (repo ports.ReleaseRunRepository, repoRoot string)

// Run executes the whole contract against one adapter.
//
//	func TestSQLiteConformance(t *testing.T) {
//	    conformance.Run(t, func(t *testing.T) (ports.ReleaseRunRepository, string) { ... })
//	}
func Run(t *testing.T, newRepo Factory) {
	t.Helper()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, root := newRepo(t)
			c.run(t, repo, root)
		})
	}
}

type testCase struct {
	name string
	run  func(t *testing.T, repo ports.ReleaseRunRepository, root string)
}

var cases = []testCase{
	{"a saved run loads back", aSavedRunLoadsBack},
	{"an unknown run is not found", anUnknownRunIsNotFound},
	{"saving twice updates rather than duplicates", savingTwiceUpdates},
	{"the latest pointer survives a round trip", theLatestPointerRoundTrips},
	{"latest is absent before anything is saved", latestIsAbsentInitially},
	{"list returns every saved run", listReturnsEverySavedRun},
	{"list is empty for an untouched repository", listIsEmptyInitially},
	{"a deleted run is gone", aDeletedRunIsGone},
	{"deleting an unknown run reports it", deletingAnUnknownRunReportsIt},
	{"load batch skips what it cannot find", loadBatchSkipsMissing},
	{"find by state matches only that state", findByStateMatchesOnlyThatState},
	{"find by state is empty when nothing matches", findByStateEmptyWhenNoMatch},
	{"find by plan hash returns nil when absent", findByPlanHashNilWhenAbsent},
	{"the run's own fields survive the round trip", runFieldsSurvive},
}

// newRun builds a run the adapter has to store, with the fields a governance record needs.
func newRun(t *testing.T, root, id string) *domainrelease.ReleaseRun {
	t.Helper()

	run := domainrelease.NewReleaseRunForTest(domainrelease.RunID(id), "main", root)
	if err := run.Plan("conformance"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return run
}

func save(t *testing.T, repo ports.ReleaseRunRepository, run *domainrelease.ReleaseRun) {
	t.Helper()
	if err := repo.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func aSavedRunLoadsBack(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run := newRun(t, root, "run-load")
	save(t, repo, run)

	loaded, err := repo.Load(context.Background(), run.ID())
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned no run for an ID that was just saved")
	}
	if loaded.ID() != run.ID() {
		t.Errorf("Load returned %q, want %q", loaded.ID(), run.ID())
	}
}

// Callers distinguish "no such run" from "the store is broken" — `relicta status` prints a
// different thing for each. An adapter that returns a zero-valued run for an unknown ID would
// have status report a release that does not exist.
func anUnknownRunIsNotFound(t *testing.T, repo ports.ReleaseRunRepository, _ string) {
	loaded, err := repo.Load(context.Background(), domain.RunID("run-never-saved"))

	if err == nil && loaded != nil {
		t.Fatal("Load invented a run for an ID that was never saved")
	}
	if err != nil && loaded != nil {
		t.Error("Load returned both a run and an error; a caller cannot act on both")
	}
}

// A run is saved at every state transition, so this is the common case rather than an edge one.
func savingTwiceUpdates(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run := newRun(t, root, "run-twice")
	save(t, repo, run)
	save(t, repo, run)

	ids, err := repo.List(context.Background(), root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	seen := 0
	for _, id := range ids {
		if id == run.ID() {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the run appears %d times after two saves, want once: a release advances "+
			"through several states and is saved at each one", seen)
	}
}

func theLatestPointerRoundTrips(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run := newRun(t, root, "run-latest")
	save(t, repo, run)

	if err := repo.SetLatest(context.Background(), root, run.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}

	latest, err := repo.LoadLatest(context.Background(), root)
	if err != nil {
		t.Fatalf("LoadLatest: %v", err)
	}
	if latest == nil || latest.ID() != run.ID() {
		t.Errorf("LoadLatest returned %v, want the run just pointed at", latest)
	}
}

// Every command that starts with "what release am I in the middle of" asks this first, in
// repositories that have never run one.
func latestIsAbsentInitially(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	latest, err := repo.LoadLatest(context.Background(), root)

	if err == nil && latest != nil {
		t.Fatal("LoadLatest returned a run in a repository where nothing was saved")
	}
}

func listReturnsEverySavedRun(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	first := newRun(t, root, "run-a")
	second := newRun(t, root, "run-b")
	save(t, repo, first)
	save(t, repo, second)

	ids, err := repo.List(context.Background(), root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := map[domain.RunID]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found[first.ID()] || !found[second.ID()] {
		t.Errorf("List returned %v, want both saved runs: history and audit read this, so a "+
			"missing run is a missing record", ids)
	}
}

func listIsEmptyInitially(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	ids, err := repo.List(context.Background(), root)
	if err != nil {
		t.Fatalf("List on an untouched repository: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("List returned %v for a repository where nothing was saved", ids)
	}
}

func aDeletedRunIsGone(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run := newRun(t, root, "run-delete")
	save(t, repo, run)

	if err := repo.Delete(context.Background(), run.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	ids, err := repo.List(context.Background(), root)
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	for _, id := range ids {
		if id == run.ID() {
			t.Error("the run is still listed after Delete; `relicta clean` would not clean")
		}
	}
}

// Delete reports an unknown run rather than shrugging, and the contract says so because the
// reference implementation does it.
//
// This case was written the other way round first — idempotent delete, on the reasoning that
// removing what is already gone has achieved the caller's intent — and the file adapter failed
// it with ErrRunNotFound. That reasoning was an opinion about what a repository should do; the
// contract is what callers were written against, and `relicta clean` can only report which runs
// it could not remove if the repository tells it.
//
// Worth knowing while implementing an adapter: this asymmetry is deliberate in the reference.
// Delete searches the roots it knows and reports not-found; DeleteFromRepo, which is given the
// root, tolerates a missing file. The difference is whether the caller has already established
// that the run should be there.
func deletingAnUnknownRunReportsIt(t *testing.T, repo ports.ReleaseRunRepository, _ string) {
	err := repo.Delete(context.Background(), domain.RunID("run-never-existed"))

	if err == nil {
		t.Fatal("Delete of an absent run reported success, so a caller cleaning up cannot " +
			"tell which runs it actually removed")
	}
	if !errors.Is(err, domain.ErrRunNotFound) {
		t.Errorf("Delete of an absent run returned %v, want ErrRunNotFound: callers match on "+
			"that error to separate \"nothing to do\" from \"the store is broken\"", err)
	}
}

func loadBatchSkipsMissing(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run := newRun(t, root, "run-present")
	save(t, repo, run)

	loaded, err := repo.LoadBatch(context.Background(), root,
		[]domain.RunID{run.ID(), domain.RunID("run-absent")})
	if err != nil {
		t.Fatalf("LoadBatch: %v", err)
	}

	if _, ok := loaded[run.ID()]; !ok {
		t.Error("LoadBatch omitted a run that exists")
	}
	if _, ok := loaded["run-absent"]; ok {
		t.Error("LoadBatch invented an entry for a run that does not exist")
	}
}

func findByStateMatchesOnlyThatState(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	planned := newRun(t, root, "run-planned")
	save(t, repo, planned)

	found, err := repo.FindByState(context.Background(), root, planned.State())
	if err != nil {
		t.Fatalf("FindByState: %v", err)
	}

	matched := false
	for _, run := range found {
		if run == nil {
			t.Fatal("FindByState returned a nil run")
		}
		if run.State() != planned.State() {
			t.Errorf("FindByState(%q) returned a run in state %q", planned.State(), run.State())
		}
		if run.ID() == planned.ID() {
			matched = true
		}
	}
	if !matched {
		t.Errorf("FindByState(%q) did not return the run in that state", planned.State())
	}
}

func findByStateEmptyWhenNoMatch(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	save(t, repo, newRun(t, root, "run-only"))

	found, err := repo.FindByState(context.Background(), root, domain.RunState("no-such-state"))
	if err != nil {
		t.Fatalf("FindByState with an unmatched state: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("FindByState returned %d runs for a state nothing is in", len(found))
	}
}

// Documented as returning nil, nil — duplicate detection asks this before every plan, and an
// error would make "no duplicate" indistinguishable from "the store is unreadable".
func findByPlanHashNilWhenAbsent(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run, err := repo.FindByPlanHash(context.Background(), root, "hash-that-was-never-planned")

	if err != nil {
		t.Errorf("FindByPlanHash returned %v for an absent hash, want nil, nil: the caller "+
			"cannot tell a missing duplicate from a broken store", err)
	}
	if run != nil {
		t.Error("FindByPlanHash invented a run for a hash nothing was planned under")
	}
}

// The fields governance reads. A round trip that loses one produces a record that is wrong
// rather than absent, which is the failure mode that reads as data.
func runFieldsSurvive(t *testing.T, repo ports.ReleaseRunRepository, root string) {
	run := newRun(t, root, "run-fields")
	save(t, repo, run)

	loaded, err := repo.Load(context.Background(), run.ID())
	if err != nil || loaded == nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.RepoRoot() != run.RepoRoot() {
		t.Errorf("RepoRoot = %q, want %q", loaded.RepoRoot(), run.RepoRoot())
	}
	if loaded.BaseRef() != run.BaseRef() {
		t.Errorf("BaseRef = %q, want %q: it was filled from the branch by a lossy loader "+
			"once, which is wrong rather than empty", loaded.BaseRef(), run.BaseRef())
	}
	if loaded.State() != run.State() {
		t.Errorf("State = %q, want %q", loaded.State(), run.State())
	}
}
