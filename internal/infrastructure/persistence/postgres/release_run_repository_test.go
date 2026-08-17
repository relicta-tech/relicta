package postgres_test

// release_run_repository_test.go runs the ADR-013 conformance suite against a real
// PostgreSQL, plus the two claims the suite does not make because they are specific to a
// backend a whole team shares: that one database keeps repositories apart, and that two
// processes saving the same run leave one run behind.
//
// Gating follows testcontainer_test.go exactly — short mode skips, an unreachable Docker
// daemon skips, anything else fails. CI without a database must not go red, and must not
// go green pretending the suite ran either, which is why the skip is loud and narrow
// rather than a blanket recover.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports/conformance"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres"
)

// TestThePostgresRepositorySatisfiesTheContract is the bar this adapter had to clear.
//
// The file adapter is the reference implementation, and a postgres adapter that disagrees
// with it is wrong however defensible its answer looks alone — an operator who switches
// `persistence.backend` must not find that `relicta status` means something different
// afterwards.
//
// One container serves every case. The conformance factory is documented as running once
// per test so a failure cannot leak into the next case, and that isolation is preserved
// without paying for fourteen containers: each case gets a repository root of its own,
// which is the key every scoped query filters on, and the tables are emptied afterwards
// so the two methods that take a bare run ID cannot see a previous case either.
func TestThePostgresRepositorySatisfiesTheContract(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	conformance.Run(t, func(t *testing.T) (ports.ReleaseRunRepository, string) {
		t.Cleanup(func() { truncateRuns(t, pool) })
		return postgres.NewReleaseRunRepository(pool), t.TempDir()
	})
}

// TestOneDatabaseKeepsRepositoriesApart is the difference between this backend and the
// file one, which gets the separation free from having a directory per repository.
//
// Here every row lives in one table, so a missing WHERE clause does not fail — it returns
// somebody else's release. Two teams sharing a database would see each other's history in
// `relicta history`, and a duplicate check would match a plan from a repository the caller
// has never heard of.
func TestOneDatabaseKeepsRepositoriesApart(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	repo := postgres.NewReleaseRunRepository(pool)
	rootA, rootB := t.TempDir(), t.TempDir()

	runA := domainrelease.NewReleaseRunForTest("run-in-a", "main", rootA)
	runB := domainrelease.NewReleaseRunForTest("run-in-b", "main", rootB)
	if err := repo.Save(ctx, runA); err != nil {
		t.Fatalf("Save into repository A: %v", err)
	}
	if err := repo.Save(ctx, runB); err != nil {
		t.Fatalf("Save into repository B: %v", err)
	}

	ids, err := repo.List(ctx, rootA)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 || ids[0] != runA.ID() {
		t.Errorf("List(A) returned %v, want only %q: a team sharing this database would "+
			"read another team's release history", ids, runA.ID())
	}

	found, err := repo.FindByState(ctx, rootA, runB.State())
	if err != nil {
		t.Fatalf("FindByState: %v", err)
	}
	for _, run := range found {
		if run.ID() == runB.ID() {
			t.Errorf("FindByState(A) returned %q, which belongs to repository B",
				run.ID())
		}
	}

	// The latest pointer is per repository, and the run it names has to be looked up in
	// the same repository that owns the pointer.
	//
	// Both roots are given a run under one ID, which is not a contrivance: a run ID is
	// derived from the plan hash, so one release planned from two checkouts of a
	// repository lands under two roots with the same ID. That is what makes this
	// assertion bite. Pointing only A at that ID must leave B with no current release —
	// a pointer table keyed globally would hand B its own copy of the run instead, and
	// `relicta status` in B would report a release nobody there started.
	const shared = domain.RunID("run-shared-id")
	for _, root := range []string{rootA, rootB} {
		if err := repo.Save(ctx, domainrelease.NewReleaseRunForTest(shared, "main", root)); err != nil {
			t.Fatalf("Save %q into %s: %v", shared, root, err)
		}
	}

	if err := repo.SetLatest(ctx, rootA, shared); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}

	latest, err := repo.LoadLatest(ctx, rootB)
	if latest != nil {
		t.Errorf("LoadLatest(B) returned %q after only A was pointed at a run; every "+
			"repository would share one release as its current one", latest.ID())
	}
	if err == nil {
		t.Error("LoadLatest(B) reported success for a repository with no latest pointer")
	}

	// A's pointer still resolves, to A's copy — the scoping must not have cost the
	// pointer its meaning.
	latestA, err := repo.LoadLatest(ctx, rootA)
	if err != nil || latestA == nil {
		t.Fatalf("LoadLatest(A): %v", err)
	}
	if latestA.RepoRoot() != rootA {
		t.Errorf("LoadLatest(A) resolved to the copy under %q, want the one under %q",
			latestA.RepoRoot(), rootA)
	}
}

// TestConcurrentSavesOfOneRunLeaveOneRun covers the case this backend exists for.
//
// A team sharing governance state has several processes advancing releases at once, and a
// run is written at every state transition, so two writers meeting on one run is normal
// traffic rather than an edge case. The failure this guards against is a second row: the
// audit trail would then hold two records of one release, and `relicta history` would
// list it twice with no way to say which is current.
func TestConcurrentSavesOfOneRunLeaveOneRun(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	repo := postgres.NewReleaseRunRepository(pool)
	root := t.TempDir()

	const writers = 8
	var wg sync.WaitGroup
	errs := make([]error, writers)

	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Separate aggregates carrying one identity, which is what two processes
			// each holding their own copy of the run actually looks like. Half of them
			// have advanced a state, so the writers genuinely disagree about the row.
			run := domainrelease.NewReleaseRunForTest("run-contended", "main", root)
			if i%2 == 0 {
				if err := run.Plan("concurrent"); err != nil {
					errs[i] = err
					return
				}
			}
			errs[i] = repo.Save(ctx, run)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: Save under contention failed: %v", i, err)
		}
	}

	ids, err := repo.List(ctx, root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("%d runs are stored after %d concurrent saves of one run, want 1: the "+
			"audit trail would hold several records of a single release", len(ids), writers)
	}

	loaded, err := repo.Load(ctx, "run-contended")
	if err != nil || loaded == nil {
		t.Fatalf("Load after concurrent saves: %v", err)
	}
	// Last writer wins, so either state is correct — a torn one is not. The point of the
	// single upsert is that a reader never sees a run assembled from two writers.
	if loaded.State() != domain.StateDraft && loaded.State() != domain.StatePlanned {
		t.Errorf("state = %q after concurrent saves, want one of the states that were "+
			"written: the stored run is a mix of two writers", loaded.State())
	}
	if loaded.RepoRoot() != root {
		t.Errorf("RepoRoot = %q, want %q", loaded.RepoRoot(), root)
	}
}

// truncateRuns empties the run tables between cases sharing one container.
//
// Registered on the subtest, not the parent: a t.Cleanup on the parent would run after
// its deferred pool.Close and truncate through a closed pool. The tests that own their
// container do not need this at all — terminating it takes the rows with it.
func truncateRuns(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, `TRUNCATE release_runs, release_run_latest`); err != nil {
		t.Fatalf("truncating run tables: %v; a later case would read this one's rows", err)
	}
}
