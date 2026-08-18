package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// Everything here is a claim the conformance suite does not make, because it is a claim
// about this adapter rather than about the port: how one file serves several
// repositories, what the pragmas buy, and whether the schema is really a migration.

func plannedRun(t *testing.T, root, id string) *domainrelease.ReleaseRun {
	t.Helper()

	run := domainrelease.NewReleaseRunForTest(domainrelease.RunID(id), "main", root)
	if err := run.Plan("store-test"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return run
}

func mustSave(t *testing.T, store *sqlite.Store, run *domainrelease.ReleaseRun) {
	t.Helper()
	if err := store.Save(context.Background(), run); err != nil {
		t.Fatalf("Save %s: %v", run.ID(), err)
	}
}

// The file adapter keys by directory, so two repositories cannot see each other by
// construction. One database file has to enforce that itself, and a repo_root column
// that any query forgot would leak another project's release history into this
// project's `history`, `report` and dashboard.
func TestOneDatabaseKeepsTwoRepositoriesApart(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	alpha, beta := t.TempDir(), t.TempDir()
	alphaRun := plannedRun(t, alpha, "run-alpha")
	betaRun := plannedRun(t, beta, "run-beta")
	mustSave(t, store, alphaRun)
	mustSave(t, store, betaRun)

	ids, err := store.List(ctx, alpha)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 || ids[0] != alphaRun.ID() {
		t.Errorf("List for one repository returned %v; the other repository's releases "+
			"are visible in this one's history", ids)
	}

	found, err := store.FindByState(ctx, alpha, alphaRun.State())
	if err != nil {
		t.Fatalf("FindByState: %v", err)
	}
	if len(found) != 1 || found[0].ID() != alphaRun.ID() {
		t.Errorf("FindByState crossed repositories: got %d runs, want only the one in "+
			"this repository", len(found))
	}

	// The latest pointer is per repository too: pointing one repo at its run must not
	// answer the other repo's question.
	if err := store.SetLatest(ctx, alpha, alphaRun.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}
	if _, err := store.LoadLatest(ctx, beta); !errors.Is(err, domain.ErrRunNotFound) {
		t.Errorf("LoadLatest for a repository with no pointer returned %v, want "+
			"ErrRunNotFound; a release in one repo was reported as in progress in another", err)
	}
}

// The port documents List as newest first, and callers page through it. The file
// adapter approximates that with file modification time, and a column could hold the real
// creation time — but the contract is the reference's behavior, because callers were written
// against it. Ordering by creation here gave one repository two histories depending on which
// backend read it, which the conformance suite caught.
func TestListReturnsRunsMostRecentlySavedFirst(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	root := t.TempDir()

	// Distinct creation times, saved oldest last so insertion order cannot be what
	// produces the answer.
	newest := plannedRun(t, root, "run-newest")
	middle := plannedRun(t, root, "run-middle")
	oldest := plannedRun(t, root, "run-oldest")
	// Saved oldest last, so insertion order cannot be what produces the answer.
	backdate(t, newest, 0)
	backdate(t, middle, 1)
	backdate(t, oldest, 2)
	mustSave(t, store, oldest)
	mustSave(t, store, middle)
	mustSave(t, store, newest)

	ids, err := store.List(context.Background(), root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []domain.RunID{newest.ID(), middle.ID(), oldest.ID()}
	if len(ids) != len(want) {
		t.Fatalf("List returned %d runs, want %d", len(ids), len(want))
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("List returned %v, want %v: history pages through this order, so a "+
				"wrong one repeats and skips entries", ids, want)
		}
	}
}

// backdate rewinds a run's creation time by whole hours, so ordering assertions do not
// depend on how fast the test machine constructs three runs.
func backdate(t *testing.T, run *domainrelease.ReleaseRun, hours int) {
	t.Helper()

	snapshot := domain.RunSnapshot{
		ID:         run.ID(),
		PlanHash:   run.PlanHash(),
		RepoID:     run.RepoID(),
		RepoRoot:   run.RepoRoot(),
		BaseRef:    run.BaseRef(),
		HeadSHA:    run.HeadSHA(),
		Commits:    run.Commits(),
		State:      run.State(),
		StepStatus: map[string]*domain.StepStatus{},
		CreatedAt:  run.CreatedAt().Add(-time.Duration(hours) * time.Hour),
		// Rewound with it: List orders by when a run last changed, and a fixture whose
		// creation moves while its update time stays put describes a run that was
		// modified before it existed.
		UpdatedAt: run.UpdatedAt().Add(-time.Duration(hours) * time.Hour),
	}
	run.ReconstructState(snapshot)
}

// `relicta` is run from wherever the developer's shell happens to be, and on macOS the
// repository is very often under a symlink (/tmp, /var, a homedir on another volume).
// The file adapter gets path equivalence from the filesystem; this store compares
// strings, so every spelling of one directory has to normalize to one repo_root or the
// history is simply not there — which looks like data loss, not like a path bug.
func TestOneRepositorySpelledSeveralWaysIsOneRepository(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()

	root := t.TempDir()
	run := plannedRun(t, root, "run-spellings")
	mustSave(t, store, run)

	// A path Clean has not been near: filepath.Join would have collapsed the "..".
	uncleaned := root + string(filepath.Separator) + "subdir" + string(filepath.Separator) + ".."

	// A symlink pointing at the same directory from somewhere else.
	linked := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, linked); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// And the relative path a shell sitting in the repository would produce.
	t.Chdir(root)

	for _, spelling := range []string{root, uncleaned, linked, ".", "./"} {
		ids, err := store.List(ctx, spelling)
		if err != nil {
			t.Fatalf("List(%q): %v", spelling, err)
		}
		if len(ids) != 1 || ids[0] != run.ID() {
			t.Errorf("List(%q) returned %v, want the run saved under %q: the same "+
				"repository spelled two ways must be one repository", spelling, ids, root)
		}
	}
}

// FindActive answers "what release am I in the middle of". The SQL cannot restate
// RunState.IsActive without the two drifting, so this checks the store agrees with the
// domain across every state the domain has, including any added later.
func TestFindActiveAgreesWithTheDomainAboutEveryState(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()
	root := t.TempDir()

	wantActive := map[domain.RunID]bool{}
	for _, state := range domain.AllStates() {
		run := plannedRun(t, root, "run-"+string(state))
		forceState(t, run, state)
		mustSave(t, store, run)
		if state.IsActive() {
			wantActive[run.ID()] = true
		}
	}

	found, err := store.FindActive(ctx, root)
	if err != nil {
		t.Fatalf("FindActive: %v", err)
	}

	gotActive := map[domain.RunID]bool{}
	for _, run := range found {
		gotActive[run.ID()] = true
		if !run.State().IsActive() {
			t.Errorf("FindActive returned a run in state %q, which the domain calls "+
				"inactive: a finished release would be reported as in progress", run.State())
		}
	}
	for id := range wantActive {
		if !gotActive[id] {
			t.Errorf("FindActive omitted %s; an in-flight release the domain calls active "+
				"is invisible to every command that asks what is in progress", id)
		}
	}
}

// forceState puts a run in a state directly, bypassing the transition rules, so every
// state in the machine can be stored without walking a path to it.
func forceState(t *testing.T, run *domainrelease.ReleaseRun, state domain.RunState) {
	t.Helper()

	run.ReconstructState(domain.RunSnapshot{
		ID:         run.ID(),
		PlanHash:   run.PlanHash(),
		RepoID:     run.RepoID(),
		RepoRoot:   run.RepoRoot(),
		BaseRef:    run.BaseRef(),
		HeadSHA:    run.HeadSHA(),
		Commits:    run.Commits(),
		State:      state,
		StepStatus: map[string]*domain.StepStatus{},
		CreatedAt:  run.CreatedAt(),
		UpdatedAt:  run.UpdatedAt(),
	})
}

// The columns are a projection; the document is the record. This is the assertion that
// the projection did not become the record — the fields nothing filters on have to come
// back too, and these four are the exact set a lossy loader dropped once, leaving
// `relicta evaluate` refusing every release with "invalid scope".
func TestTheWholeAggregateSurvivesTheDatabase(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()
	root := t.TempDir()

	commits := []domain.CommitSHA{"aaa111", "bbb222", "ccc333"}
	run := domain.NewReleaseRun(
		"acme/widget",
		root,
		"refs/tags/v1.2.0", // deliberately a tag, not the branch a lossy loader substitutes
		domain.CommitSHA("ccc333"),
		commits,
		"config-hash",
		"plugin-hash",
	)
	changeSet := changes.NewChangeSet("cs-sqlite", "refs/tags/v1.2.0", "ccc333")
	changeSet.AddCommit(changes.NewConventionalCommit("aaa111", changes.CommitTypeFeat, "add the thing"))
	changeSet.AddCommit(changes.NewConventionalCommit("bbb222", changes.CommitTypeFix, "correct the thing"))
	run.SetChangeSet(changeSet)

	if err := store.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, run.ID())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.BaseRef() != run.BaseRef() {
		t.Errorf("BaseRef = %q, want %q: a run whose base ref does not survive describes a "+
			"different range than the one that was planned", loaded.BaseRef(), run.BaseRef())
	}
	if loaded.HeadSHA() != run.HeadSHA() {
		t.Errorf("HeadSHA = %q, want %q: an empty HEAD leaves governance unable to say what "+
			"it evaluated", loaded.HeadSHA(), run.HeadSHA())
	}
	if got := loaded.Commits(); len(got) != len(commits) {
		t.Errorf("loaded %d commits, want %d: without them the proposal has no scope and "+
			"evaluation refuses it", len(got), len(commits))
	}
	if !loaded.HasChangeSet() {
		t.Fatal("the changeset did not survive the database: this is the exact failure that " +
			"made `relicta evaluate` refuse every release with \"invalid scope\"")
	}
	if got, want := len(loaded.ChangeSet().Commits()), len(changeSet.Commits()); got != want {
		t.Errorf("changeset carries %d commits, want %d", got, want)
	}
}

// Deleting the run the pointer names leaves the pointer behind, exactly as the file
// adapter leaves its `latest` file behind. What matters is that the next reader is told
// there is no latest run rather than shown a broken row or an error about one.
func TestTheLatestPointerToADeletedRunReportsNotFound(t *testing.T) {
	store := newStore(t, filepath.Join(t.TempDir(), "relicta.db"))
	ctx := context.Background()
	root := t.TempDir()

	run := plannedRun(t, root, "run-vanishing")
	mustSave(t, store, run)
	if err := store.SetLatest(ctx, root, run.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}
	if err := store.Delete(ctx, run.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	latest, err := store.LoadLatest(ctx, root)
	if latest != nil {
		t.Error("LoadLatest returned a run that was deleted")
	}
	if !errors.Is(err, domain.ErrRunNotFound) {
		t.Errorf("LoadLatest over a dangling pointer returned %v, want ErrRunNotFound: "+
			"`relicta status` distinguishes \"no release in progress\" from a broken store", err)
	}
}

// Two relicta processes in one repository is a normal CI situation — a workflow saving
// a run while a status check reads one. Two Stores on one file is what that looks like
// from inside a test, and without WAL and a busy timeout the loser of any overlap gets
// SQLITE_BUSY and the command fails.
func TestConcurrentWritersOnOneDatabaseFileAllSucceed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	first := newStore(t, path)
	second := newStore(t, path)
	root := t.TempDir()

	const writersPerStore, savesPerWriter = 4, 25

	// Runs are built up front: t.Fatalf from a goroutine is not allowed, and a helper
	// that can fail has no business inside the section under test anyway.
	type writer struct {
		store *sqlite.Store
		run   *domainrelease.ReleaseRun
	}
	writers := make([]writer, 0, 2*writersPerStore)
	for storeIndex, store := range []*sqlite.Store{first, second} {
		for i := range writersPerStore {
			name := fmt.Sprintf("run-p%d-w%d", storeIndex, i)
			writers = append(writers, writer{store: store, run: plannedRun(t, root, name)})
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(writers)*savesPerWriter)

	for _, w := range writers {
		wg.Add(1)
		go func(w writer) {
			defer wg.Done()
			for range savesPerWriter {
				if err := w.store.Save(context.Background(), w.run); err != nil {
					errCh <- fmt.Errorf("%s: %w", w.run.ID(), err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("a concurrent Save failed: %v; a second relicta process in the same "+
			"repository makes the first one's command fail", err)
	}

	// And the second process sees what the first committed.
	ids, err := second.List(context.Background(), root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != len(writers) {
		t.Errorf("the database holds %d runs, want %d: a write from one process is not "+
			"visible to the other", len(ids), len(writers))
	}
}

// journal_mode is a property of the file, not of a connection, so it is checkable from
// outside the store — and worth checking there, because it is the setting that decides
// whether a reader and a writer exclude each other.
func TestTheDatabaseFileIsInWALMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	newStore(t, path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening the database directly: %v", err)
	}
	defer func() { _ = db.Close() }()

	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want \"wal\": under a rollback journal a reader and "+
			"the writer lock each other out, so `relicta status` fails while `relicta "+
			"publish` commits", mode)
	}
}

// Reopening is the ordinary case — every CLI invocation is a new process against the
// same file — so a migration that ran twice, or a schema that did not survive the
// close, would break the second command rather than the first.
func TestReopeningAnExistingDatabaseKeepsItsRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	root := t.TempDir()
	ctx := context.Background()

	first, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	run := plannedRun(t, root, "run-persisted")
	mustSave(t, first, run)
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second := newStore(t, path)
	loaded, err := second.Load(ctx, run.ID())
	if err != nil {
		t.Fatalf("Load after reopening: %v; the next relicta invocation cannot see what "+
			"the last one saved", err)
	}
	if loaded.ID() != run.ID() {
		t.Errorf("Load returned %q, want %q", loaded.ID(), run.ID())
	}
}

// Open creates the directory because .relicta/ does not exist in a repository that has
// never released, and the first command must not fail on that.
func TestOpenCreatesTheDatabaseDirectory(t *testing.T) {
	path := sqlite.DefaultPath(t.TempDir())
	newStore(t, path)

	if _, err := os.Stat(path); err != nil {
		t.Errorf("no database at %s after Open: %v; the first release in a fresh "+
			"repository would fail because .relicta/ does not exist yet", path, err)
	}
}

// The schema is a migration, not a schema that happens to be applied once. That is only
// true if the down half works, and nothing else exercises it.
func TestTheMigrationRollsBackAndReapplies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	ctx := context.Background()

	store := newStore(t, path)
	root := t.TempDir()
	mustSave(t, store, plannedRun(t, root, "run-doomed"))

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening the database directly: %v", err)
	}
	defer func() { _ = db.Close() }()
	migrator := sqlite.NewMigrator(db)

	// Down rolls back one migration, most recently applied first, so undoing the whole
	// schema takes as many calls as there are migrations. Rolling back only the newest is
	// the behavior an operator who ran `relicta db migrate` against the wrong file needs;
	// rolling back everything on one call would be a data-loss surprise.
	//
	// Counted from the migrator rather than written down. A hardcoded 2 became wrong the
	// moment a third migration landed, and it failed as "Down did not drop the schema",
	// which describes the migrator rather than the test.
	status, err := migrator.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	migrationCount := len(status)
	if migrationCount == 0 {
		t.Fatal("the migrator reports no migrations, so this test would assert nothing")
	}
	for range migrationCount {
		if err := migrator.Down(ctx); err != nil {
			t.Fatalf("Down: %v", err)
		}
	}
	if _, err := store.List(ctx, root); err == nil {
		t.Error("List still worked after the schema was dropped, so Down did not drop it")
	}

	applied, err := migrator.Up(ctx)
	if err != nil {
		t.Fatalf("Up after Down: %v", err)
	}
	if applied != migrationCount {
		t.Errorf("Up reapplied %d migrations, want %d: a rolled-back migration that Up "+
			"skips leaves the database without its schema and every command failing",
			applied, migrationCount)
	}

	ids, err := store.List(ctx, root)
	if err != nil {
		t.Fatalf("List after Up: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("List returned %v after the schema was dropped and rebuilt; Down is "+
			"documented as discarding the runs", ids)
	}
}

// Status is what `relicta db migrate --status` would print, and an unapplied migration
// reported as applied is how an operator concludes their store is fine when it is empty.
func TestMigrationStatusReportsWhatHasBeenApplied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relicta.db")
	newStore(t, path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening the database directly: %v", err)
	}
	defer func() { _ = db.Close() }()

	statuses, err := sqlite.NewMigrator(db).Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(statuses) == 0 {
		t.Fatal("Status reported no migrations at all; the embedded schema was not found")
	}
	for _, status := range statuses {
		if !status.Applied {
			t.Errorf("migration %s (%s) is reported unapplied after Open, which runs Up",
				status.Version, status.Name)
		}
	}
}
