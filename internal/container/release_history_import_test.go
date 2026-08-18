package container

// release_history_import_test.go is the round trip ADR-013 names as evidence.
//
// The ADR defers flipping persistence.backend's default until there is "a conformance suite
// passing on all three adapters, and an importer with a round trip test". The suite exists. This
// is the other half, and it is only evidence if it reads the history back out of the
// destination field by field: an importer that moves run IDs and drops what the runs contain
// would pass any test that counted rows.
//
// Which fields matter is not a guess. internal/domain/release/adapters/run_round_trip_test.go
// records a second file implementation that reconstructed runs lossily — BaseRef filled from
// the branch, HeadSHA empty, Commits dropped, the changeset looked for elsewhere — and
// `relicta evaluate` then refused every release in every repository with "invalid scope". The
// failure mode is a record that is wrong rather than absent, so those are the fields asserted
// here, on runs in several states, plus the version and the pointer to the current release.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/application/releasehistory"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
)

// TestAFileHistoryRoundTripsThroughTheImporterIntoSQLite is the deliverable.
//
// SQLite rather than PostgreSQL because it needs no Docker, so this runs everywhere the rest of
// the suite does — and the conformance suite is what says the two adapters agree.
func TestAFileHistoryRoundTripsThroughTheImporterIntoSQLite(t *testing.T) {
	repoRoot := repoDir(t)
	source := writeFileHistory(t, repoRoot)
	before := snapshotTree(t, filepath.Join(repoRoot, ".relicta"))

	ctx := context.Background()
	result, err := ImportHistory(ctx, sqliteConfig(), repoRoot, releasehistory.Options{})
	if err != nil {
		t.Fatalf("ImportHistory: %v", err)
	}
	report, into := result.Runs, result.Into

	if into.Backend != config.BackendSQLite {
		t.Errorf("imported into backend %q, want sqlite: the report is what tells an operator "+
			"the migration went where they configured", into.Backend)
	}
	if report.Runs != len(source.runs) || report.Created != len(source.runs) || report.Replaced != 0 {
		t.Errorf("report says %d runs, %d new, %d replaced; want %d runs all new: an operator "+
			"checks the migration happened against these counts",
			report.Runs, report.Created, report.Replaced, len(source.runs))
	}
	if !report.LatestTransferred || report.Latest != source.latest.ID() {
		t.Errorf("report says latest=%q transferred=%t, want %q transferred: a history whose "+
			"current release is missing is not a migrated history",
			report.Latest, report.LatestTransferred, source.latest.ID())
	}

	destination := openSQLite(t, repoRoot)

	// Every run, read back out of the database and compared to the run that was written to
	// JSON — not to a rebuilt expectation, so a field the importer never touched cannot pass
	// by coincidence.
	ids, err := destination.List(ctx, repoRoot)
	if err != nil {
		t.Fatalf("List in the destination: %v", err)
	}
	if len(ids) != len(source.runs) {
		t.Fatalf("the destination lists %d runs, want %d: %v", len(ids), len(source.runs), ids)
	}

	loaded, err := destination.LoadBatch(ctx, repoRoot, ids)
	if err != nil {
		t.Fatalf("LoadBatch in the destination: %v", err)
	}
	for _, want := range source.runs {
		got, ok := loaded[want.ID()]
		if !ok {
			t.Errorf("run %s (state %s) is not in the destination: a release missing from the "+
				"imported history is a missing audit record", want.ID(), want.State())
			continue
		}
		assertSameRun(t, got, want)
	}

	// The pointer, resolved through the destination rather than read as a string: what an
	// operator cares about is that `relicta status` finds the same release it found before.
	latest, err := destination.LoadLatest(ctx, repoRoot)
	if err != nil {
		t.Fatalf("LoadLatest in the destination: %v — the current release did not survive the "+
			"import, so `relicta status` reports no release in a migrated repository", err)
	}
	assertSameRun(t, latest, source.latest)

	// Non-destructive, in the strong form: byte for byte, mode and modification time
	// included. ADR-013 says the JSON stays as an export until the operator removes it, and
	// an importer that rewrote it — even harmlessly, even only the timestamps — would have
	// destroyed the one copy an operator can fall back to if the migration was wrong.
	assertTreeUnchanged(t, filepath.Join(repoRoot, ".relicta"), before)

	// Idempotent. Both database adapters upsert by run ID, which is a claim about SQL; this
	// is the claim about the command, and it is the property an operator relies on when an
	// import failed halfway and they run it again.
	secondResult, err := ImportHistory(ctx, sqliteConfig(), repoRoot, releasehistory.Options{})
	if err != nil {
		t.Fatalf("second ImportHistory: %v", err)
	}
	second := secondResult.Runs
	if second.Created != 0 || second.Replaced != len(source.runs) {
		t.Errorf("the second import reports %d new and %d replaced, want 0 new and %d "+
			"replaced: an import that creates rows again has duplicated the history",
			second.Created, second.Replaced, len(source.runs))
	}

	afterSecond := openSQLite(t, repoRoot)
	ids, err = afterSecond.List(ctx, repoRoot)
	if err != nil {
		t.Fatalf("List after the second import: %v", err)
	}
	if len(ids) != len(source.runs) {
		t.Errorf("the destination holds %d runs after two imports, want %d: %v",
			len(ids), len(source.runs), ids)
	}
	loaded, err = afterSecond.LoadBatch(ctx, repoRoot, ids)
	if err != nil {
		t.Fatalf("LoadBatch after the second import: %v", err)
	}
	for _, want := range source.runs {
		if got, ok := loaded[want.ID()]; ok {
			assertSameRun(t, got, want)
		}
	}
	assertTreeUnchanged(t, filepath.Join(repoRoot, ".relicta"), before)
}

// A dry run reports and writes nothing.
//
// It still opens the destination, deliberately: a dry run whose only failure mode is
// "everything looks fine" would let an operator plan a migration into a database that is
// unreachable or unmigrated, which is precisely what they ran the dry run to find out.
func TestADryRunImportReportsTheHistoryAndWritesNoRuns(t *testing.T) {
	repoRoot := repoDir(t)
	source := writeFileHistory(t, repoRoot)
	before := snapshotTree(t, filepath.Join(repoRoot, ".relicta"))

	ctx := context.Background()
	dryRun, err := ImportHistory(ctx, sqliteConfig(), repoRoot,
		releasehistory.Options{DryRun: true})
	if err != nil {
		t.Fatalf("ImportHistory --dry-run: %v", err)
	}
	report := dryRun.Runs

	if report.Runs != len(source.runs) {
		t.Errorf("the dry run reports %d runs, want %d", report.Runs, len(source.runs))
	}
	if report.Written() != 0 || report.LatestTransferred {
		t.Errorf("the dry run reports %d runs written and latest transferred=%t, want 0 and "+
			"false", report.Written(), report.LatestTransferred)
	}

	ids, err := openSQLite(t, repoRoot).List(ctx, repoRoot)
	if err != nil {
		t.Fatalf("List in the destination after a dry run: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("the database holds %v after a dry run: --dry-run promises to write nothing", ids)
	}
	assertTreeUnchanged(t, filepath.Join(repoRoot, ".relicta"), before)
}

// The file backend is refused, and the refusal has to be actionable: the operator's mistake is
// almost always that they have not changed persistence.backend yet.
func TestImportRefusesTheFileBackendBecauseItWouldBeTheSourceAndTheDestination(t *testing.T) {
	repoRoot := repoDir(t)
	writeFileHistory(t, repoRoot)

	_, err := ImportHistory(context.Background(), config.DefaultConfig(),
		repoRoot, releasehistory.Options{})

	if err == nil {
		t.Fatal("importing into the file backend succeeded: relicta would report a migration " +
			"to an operator whose history never left the JSON tree")
	}
	for _, want := range []string{"persistence.backend", "sqlite", "postgres"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v — it has to name what to change, "+
				"not only that this is wrong", want, err)
		}
	}
}

// A repository that has never planned a release is not an error state.
func TestImportingARepositoryWithNoHistoryReportsNothingToDo(t *testing.T) {
	repoRoot := repoDir(t)

	empty, err := ImportHistory(context.Background(), sqliteConfig(), repoRoot,
		releasehistory.Options{})
	report := empty.Runs

	if err != nil {
		t.Fatalf("importing a repository with no history: %v — nothing to move is not a "+
			"failure, and a non-zero exit would break a migration script that runs this "+
			"across every repository it owns", err)
	}
	if report.Runs != 0 || report.Written() != 0 {
		t.Errorf("report says %d runs and %d written for a repository with no history",
			report.Runs, report.Written())
	}
}

// The governance record moves too, and this is the test that says so.
//
// `relicta db import` covered release runs only. Once persistence.backend selects the
// governance store as well, that is not an incomplete feature but a trap: an operator switches
// to sqlite, runs the importer, is told their history moved, and then finds `relicta history`
// empty, the DORA and SOC 2 reports computed from nothing, and the deployment gate authorizing
// against a record with no releases in it. Nothing fails.
func TestTheImporterMovesTheGovernanceRecordAndNotOnlyTheRuns(t *testing.T) {
	repoRoot := repoDir(t)
	writeFileHistory(t, repoRoot)
	writeGovernanceHistory(t, repoRoot)

	ctx := context.Background()
	result, err := ImportHistory(ctx, sqliteConfig(), repoRoot, releasehistory.Options{})
	if err != nil {
		t.Fatalf("ImportHistory: %v", err)
	}

	if result.Governance.Releases != 2 || result.Governance.Incidents != 1 {
		t.Errorf("the report says %d governance releases and %d incidents, want 2 and 1: an "+
			"import that moves runs alone leaves the audit trail behind",
			result.Governance.Releases, result.Governance.Incidents)
	}
	if result.Into.GovernanceLocation == "" {
		t.Error("the report does not say where the governance record went, so an operator " +
			"cannot check that it went where they configured")
	}

	// Read it back through the resolver, the way `relicta history` will.
	destination := openGovernanceStore(t, repoRoot)
	history, err := destination.GetReleaseHistory(ctx, governanceTestRepo, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory in the destination: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("the destination holds %d governance records, want 2: `relicta history` "+
			"reports what is in here", len(history))
	}
	if history[0].RiskScore != 0.7 || history[0].Decision != cgp.DecisionApproved {
		t.Errorf("the newest record came back as %+v, with the risk score or decision lost: "+
			"a governance record without them is not evidence of anything", history[0])
	}

	// Non-destructive, byte for byte. memory.json is the operator's fallback if the
	// migration was wrong, and an importer that rewrote it would have destroyed it.
	if _, err := os.Stat(filepath.Join(repoRoot, ".relicta", "governance", "memory.json")); err != nil {
		t.Errorf("memory.json is gone after the import: %v — ADR-013 leaves the JSON as an "+
			"export until the operator removes it", err)
	}
}

const governanceTestRepo = "owner/repo"

// writeGovernanceHistory puts a governance record in the file store the importer reads.
func writeGovernanceHistory(t *testing.T, repoRoot string) {
	t.Helper()

	ctx := context.Background()
	store, err := cgpmemory.NewFileStore(GovernanceMemoryFileDir(sqliteConfig(), repoRoot))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	now := time.Now()
	for _, spec := range []struct {
		id      string
		version string
		risk    float64
		at      time.Time
	}{
		{"gov-rel-1", "1.0.0", 0.2, now.Add(-2 * time.Hour)},
		{"gov-rel-2", "1.1.0", 0.7, now},
	} {
		if err := store.RecordRelease(ctx, &cgpmemory.ReleaseRecord{
			ID:         spec.id,
			Repository: governanceTestRepo,
			Version:    spec.version,
			Actor:      cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"},
			RiskScore:  spec.risk,
			Decision:   cgp.DecisionApproved,
			Outcome:    cgpmemory.OutcomeSuccess,
			ReleasedAt: spec.at,
		}); err != nil {
			t.Fatalf("RecordRelease %s: %v", spec.id, err)
		}
	}

	if err := store.RecordIncident(ctx, &cgpmemory.IncidentRecord{
		ID:         "gov-inc-1",
		Repository: governanceTestRepo,
		ReleaseID:  "gov-rel-1",
		ActorID:    "human:alice",
		DetectedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}
}

// openGovernanceStore opens the destination the way a relicta command would, through the one
// place that reads persistence.backend.
func openGovernanceStore(t *testing.T, repoRoot string) cgpmemory.Store {
	t.Helper()

	store, err := OpenGovernanceMemory(context.Background(), sqliteConfig(), repoRoot)
	if err != nil {
		t.Fatalf("OpenGovernanceMemory: %v", err)
	}
	t.Cleanup(func() {
		if store.Closer != nil {
			_ = store.Closer.Close()
		}
	})
	return store.Store
}

// fileHistory is the source of truth a round trip is compared against.
type fileHistory struct {
	runs   []*domain.ReleaseRun
	latest *domain.ReleaseRun
}

// writeFileHistory builds a file-backed history with runs in several states.
//
// Several states because state is stored differently by every adapter — a column in SQL, a
// field in the JSON document — and a history that is all one state cannot catch an adapter that
// writes a constant.
func writeFileHistory(t *testing.T, repoRoot string) fileHistory {
	t.Helper()

	repo := adapters.NewFileReleaseRunRepository()
	ctx := context.Background()

	planned := newHistoryRun(t, repoRoot, "aaa111")
	if err := planned.Plan("alice"); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	versioned := newHistoryRun(t, repoRoot, "bbb222")
	if err := versioned.Plan("alice"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := versioned.SetVersion(version.MustParse("1.3.0"), "v1.3.0"); err != nil {
		t.Fatalf("SetVersion: %v", err)
	}
	if err := versioned.Bump("alice"); err != nil {
		t.Fatalf("Bump: %v", err)
	}

	published := newHistoryRun(t, repoRoot, "ccc333")
	if err := published.Plan("bob"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := published.SetVersion(version.MustParse("1.4.0"), "v1.4.0"); err != nil {
		t.Fatalf("SetVersion: %v", err)
	}
	if err := published.Bump("bob"); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if err := published.GenerateNotes(&domain.ReleaseNotes{Text: "## 1.4.0\n\n- add the thing"},
		"notes-hash", "bob"); err != nil {
		t.Fatalf("GenerateNotes: %v", err)
	}
	if err := published.Approve("bob", false); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := published.StartPublishing("bob"); err != nil {
		t.Fatalf("StartPublishing: %v", err)
	}
	if err := published.MarkPublished("bob"); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	canceled := newHistoryRun(t, repoRoot, "ddd444")
	if err := canceled.Plan("carol"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := canceled.Cancel("superseded", "carol"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	history := fileHistory{
		runs:   []*domain.ReleaseRun{planned, versioned, published, canceled},
		latest: published,
	}

	for _, run := range history.runs {
		if err := repo.Save(ctx, run); err != nil {
			t.Fatalf("Save %s: %v", run.ID(), err)
		}
	}
	if err := repo.SetLatest(ctx, repoRoot, history.latest.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}

	return history
}

// newHistoryRun builds one run with the fields governance reads.
//
// The base ref is a tag rather than a branch name on purpose: that is the field the lossy
// loader filled in from the branch, so a run whose base ref is "main" cannot tell a correct
// round trip from that bug.
func newHistoryRun(t *testing.T, repoRoot, head string) *domain.ReleaseRun {
	t.Helper()

	commits := []domain.CommitSHA{domain.CommitSHA(head), "eee555"}
	run := domain.NewReleaseRun(
		"acme/widget", repoRoot,
		"refs/tags/v1.2.0",
		domain.CommitSHA(head),
		commits,
		"config-hash", "plugin-hash",
	)

	changeSet := changes.NewChangeSet(changes.ChangeSetID("cs-"+head), "refs/tags/v1.2.0", head)
	changeSet.AddCommit(changes.NewConventionalCommit(head, changes.CommitTypeFeat, "add the thing"))
	changeSet.AddCommit(changes.NewConventionalCommit("eee555", changes.CommitTypeFix, "correct the thing"))
	run.SetChangeSet(changeSet)

	return run
}

// assertSameRun compares a run read out of the destination with the one written to the source.
func assertSameRun(t *testing.T, got, want *domain.ReleaseRun) {
	t.Helper()

	if got == nil {
		t.Fatalf("run %s came back nil from the destination", want.ID())
	}
	if got.ID() != want.ID() {
		t.Errorf("ID = %q, want %q", got.ID(), want.ID())
	}
	if got.State() != want.State() {
		t.Errorf("run %s: State = %q, want %q: state is what `relicta status` and every "+
			"FindByState query act on", want.ID(), got.State(), want.State())
	}
	if got.BaseRef() != want.BaseRef() {
		t.Errorf("run %s: BaseRef = %q, want %q: a run whose base ref does not survive "+
			"describes a different range than the one that was planned",
			want.ID(), got.BaseRef(), want.BaseRef())
	}
	if got.HeadSHA() != want.HeadSHA() {
		t.Errorf("run %s: HeadSHA = %q, want %q: an empty HEAD leaves governance unable to "+
			"say what it evaluated", want.ID(), got.HeadSHA(), want.HeadSHA())
	}
	if !reflect.DeepEqual(got.Commits(), want.Commits()) {
		t.Errorf("run %s: Commits = %v, want %v: without them the proposal has no scope and "+
			"`relicta evaluate` refuses it", want.ID(), got.Commits(), want.Commits())
	}
	if got.VersionNext().String() != want.VersionNext().String() {
		t.Errorf("run %s: VersionNext = %q, want %q: the version is the release",
			want.ID(), got.VersionNext(), want.VersionNext())
	}
	if got.TagName() != want.TagName() {
		t.Errorf("run %s: TagName = %q, want %q", want.ID(), got.TagName(), want.TagName())
	}
	if got.PlanHash() != want.PlanHash() {
		t.Errorf("run %s: PlanHash = %q, want %q: duplicate detection and approval binding "+
			"both compare it", want.ID(), got.PlanHash(), want.PlanHash())
	}
	if got.RepoRoot() != want.RepoRoot() {
		t.Errorf("run %s: RepoRoot = %q, want %q", want.ID(), got.RepoRoot(), want.RepoRoot())
	}

	if !got.HasChangeSet() {
		t.Errorf("run %s: the changeset did not survive the import: this is the exact failure "+
			"that made `relicta evaluate` refuse every release with \"invalid scope\"", want.ID())
		return
	}
	gotCommits, wantCommits := got.ChangeSet().Commits(), want.ChangeSet().Commits()
	if len(gotCommits) != len(wantCommits) {
		t.Errorf("run %s: the changeset carries %d commits, want %d",
			want.ID(), len(gotCommits), len(wantCommits))
		return
	}
	for i := range wantCommits {
		if gotCommits[i].Hash() != wantCommits[i].Hash() ||
			gotCommits[i].Subject() != wantCommits[i].Subject() {
			t.Errorf("run %s: changeset commit %d = %s %q, want %s %q", want.ID(), i,
				gotCommits[i].Hash(), gotCommits[i].Subject(),
				wantCommits[i].Hash(), wantCommits[i].Subject())
		}
	}
}

// repoDir returns a directory the file store can be placed in, with symlinks resolved so the
// path matches what the database adapters normalize to. On macOS /tmp is /private/tmp, and
// without this the imported runs would look like they belong to another repository.
func repoDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return dir
}

func sqliteConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Persistence = config.DefaultPersistenceConfig()
	cfg.Persistence.Backend = config.BackendSQLite
	return cfg
}

// openSQLite opens the destination the way a relicta command would, through the one place that
// reads persistence.backend. A test that opened the database itself could pass while the
// command wrote somewhere else.
func openSQLite(t *testing.T, repoRoot string) ports.ReleaseRunRepository {
	t.Helper()

	store, err := persistence.OpenReleaseRunStore(context.Background(), sqliteConfig().Persistence, repoRoot)
	if err != nil {
		t.Fatalf("OpenReleaseRunStore: %v", err)
	}
	if store.Closer != nil {
		t.Cleanup(func() { _ = store.Closer.Close() })
	}
	return store.Repository
}

// snapshotTree records every file under root with its content, mode and modification time.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path) //nolint:gosec // a path this test just walked
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = fmt.Sprintf("%s mode=%s mtime=%d",
			hex.EncodeToString(sum[:]), info.Mode(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}

// assertTreeUnchanged is the non-destructive guarantee, spelled out.
func assertTreeUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()

	after := snapshotTree(t, root)

	for name, want := range before {
		got, ok := after[name]
		if !ok {
			t.Errorf("%s is gone after the import: ADR-013 keeps the JSON tree as an export "+
				"until the operator removes it, and it is the only copy they can fall back "+
				"to if the migration was wrong", name)
			continue
		}
		if got != want {
			t.Errorf("%s changed during the import (%s -> %s): the source of a migration must "+
				"be read only", name, want, got)
		}
	}
}
