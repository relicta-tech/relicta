package releasehistory

// import_test.go covers the failure paths a real store makes hard to provoke.
//
// The round trip against SQLite lives in internal/container, where both real adapters are
// reachable; what needs fakes is the question ADR-013 cares most about — what an operator has,
// and is told, when a write fails partway through moving an audit trail.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

const repoRoot = "/repo/widget"

// A partial migration is the worst outcome ADR-013 names, so it must be impossible for it to
// look like a finished one: the count is reported, and the current release pointer is not set.
func TestAWriteFailurePartwayReportsWhatReachedTheDestinationAndSetsNoPointer(t *testing.T) {
	src := sourceWith(t, 4)
	dst := &fakeDestination{failOnSave: 3, saveErr: errors.New("disk is full")}

	report, err := Import(context.Background(), src, dst, Options{RepoRoot: repoRoot})

	if err == nil {
		t.Fatal("a failed write reported success: an operator would delete the JSON tree " +
			"believing the history had moved")
	}
	if report.Written() != 2 {
		t.Errorf("report says %d runs written, want 2: the count is the only way an operator "+
			"can tell a partial migration from one that did nothing", report.Written())
	}
	if report.LatestTransferred || dst.latestSet != 0 {
		t.Errorf("the current release pointer was set after a failed import (transferred=%t, "+
			"SetLatest calls=%d): a destination that answers \"what am I releasing\" out of a "+
			"history with holes in it is worse than one that answers nothing",
			report.LatestTransferred, dst.latestSet)
	}
	for _, want := range []string{"partial history", "was not modified", "again is safe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not say %q: %v — it has to tell the operator what state "+
				"they are in and what to do about it", want, err)
		}
	}
}

// LoadBatch skips what it cannot load, by contract. That is right for rendering a history and
// wrong for copying one, and the difference has to be caught before the first write.
func TestARunTheSourceCannotReadStopsTheImportBeforeAnythingIsWritten(t *testing.T) {
	src := sourceWith(t, 3)
	unreadable := src.order[1]
	delete(src.runs, unreadable)

	dst := &fakeDestination{}
	_, err := Import(context.Background(), src, dst, Options{RepoRoot: repoRoot})

	if err == nil {
		t.Fatal("a history with an unreadable run imported anyway: the run would be missing " +
			"from the destination and the command would report success")
	}
	if !strings.Contains(err.Error(), string(unreadable)) {
		t.Errorf("the error does not name the run that could not be read: %v", err)
	}
	if len(dst.saved) != 0 {
		t.Errorf("%d runs were written before the source was found to be incomplete: refusing "+
			"leaves the operator with an untouched destination rather than a partial one",
			len(dst.saved))
	}
}

// Oldest first, so an interrupted import holds the beginning of a history rather than its end —
// the end being the part that looks complete.
func TestRunsAreWrittenOldestFirst(t *testing.T) {
	src := sourceWith(t, 3)
	// List hands back most-recently-changed first, which is the order that must not decide
	// the write sequence.
	src.order = reversed(src.order)

	dst := &fakeDestination{}
	if _, err := Import(context.Background(), src, dst, Options{RepoRoot: repoRoot}); err != nil {
		t.Fatalf("Import: %v", err)
	}

	for i := 1; i < len(dst.saved); i++ {
		previous := src.runs[dst.saved[i-1]]
		current := src.runs[dst.saved[i]]
		if current.CreatedAt().Before(previous.CreatedAt()) {
			t.Errorf("run %s was written before %s, which is older: an interrupted import "+
				"would leave the destination holding the most recent releases and none of the "+
				"history behind them", dst.saved[i], dst.saved[i-1])
		}
	}
}

// The pointer has to arrive even if the run it names was not listed, or the destination would
// claim a current release it does not hold.
func TestAPointerAtAnUnlistedRunImportsThatRunToo(t *testing.T) {
	src := sourceWith(t, 2)
	orphan := newRun(t, "fff666")
	src.runs[orphan.ID()] = orphan
	src.latest = orphan

	dst := &fakeDestination{}
	report, err := Import(context.Background(), src, dst, Options{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if report.Runs != 3 || len(dst.saved) != 3 {
		t.Errorf("imported %d runs and wrote %d, want 3: the run the pointer names was left "+
			"out, so the destination's current release resolves to nothing", report.Runs, len(dst.saved))
	}
	if !report.LatestTransferred || dst.latest != orphan.ID() {
		t.Errorf("the destination's pointer is %q (transferred=%t), want %q",
			dst.latest, report.LatestTransferred, orphan.ID())
	}
}

// A history with no current release is ordinary — a repository that has planned runs without
// publishing one — and the import must not invent a pointer for it.
func TestAHistoryWithNoCurrentReleaseImportsWithoutAPointer(t *testing.T) {
	src := sourceWith(t, 2)
	src.latest = nil

	dst := &fakeDestination{}
	report, err := Import(context.Background(), src, dst, Options{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if report.Latest != "" || report.LatestTransferred || dst.latestSet != 0 {
		t.Errorf("a pointer was reported or set for a history that has none: latest=%q "+
			"transferred=%t SetLatest calls=%d", report.Latest, report.LatestTransferred, dst.latestSet)
	}
	if report.Written() != 2 {
		t.Errorf("wrote %d runs, want 2: an absent pointer is not a reason to skip the history",
			report.Written())
	}
}

// Both database adapters file a run under the root the run itself carries, so a run copied in
// from another checkout is imported and then invisible to this repository's history. Reporting
// it is the difference between a confusing outcome and an explained one.
func TestARunStoredUnderAnotherRepositoryRootIsReported(t *testing.T) {
	src := sourceWith(t, 2)
	elsewhere := domain.NewReleaseRun(
		"acme/widget", "/somewhere/else", "refs/tags/v1.0.0",
		domain.CommitSHA("999zzz"), []domain.CommitSHA{"999zzz"}, "config-hash", "plugin-hash",
	)
	src.runs[elsewhere.ID()] = elsewhere
	src.order = append(src.order, elsewhere.ID())

	report, err := Import(context.Background(), src, &fakeDestination{}, Options{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(report.ForeignRoots) != 1 || report.ForeignRoots[0] != elsewhere.ID() {
		t.Errorf("ForeignRoots = %v, want [%s]: without the warning the import reports three "+
			"runs and `relicta history` shows two", report.ForeignRoots, elsewhere.ID())
	}
	if report.Written() != 3 {
		t.Errorf("wrote %d runs, want 3: a run is reported, not dropped — rewriting the root "+
			"it records would edit an audit record to make the migration look tidier",
			report.Written())
	}
}

// A dry run touches the destination only to read it.
func TestADryRunWritesNothing(t *testing.T) {
	dst := &fakeDestination{}

	report, err := Import(context.Background(), sourceWith(t, 3), dst,
		Options{RepoRoot: repoRoot, DryRun: true})
	if err != nil {
		t.Fatalf("Import --dry-run: %v", err)
	}

	if len(dst.saved) != 0 || dst.latestSet != 0 {
		t.Errorf("a dry run wrote %d runs and set the pointer %d times", len(dst.saved), dst.latestSet)
	}
	if report.Runs != 3 || !report.DryRun {
		t.Errorf("report says %d runs, dryRun=%t, want 3 and true", report.Runs, report.DryRun)
	}
}

// Fakes.

type fakeSource struct {
	runs    map[domain.RunID]*domain.ReleaseRun
	order   []domain.RunID
	latest  *domain.ReleaseRun
	listErr error
}

func (f *fakeSource) List(_ context.Context, _ string) ([]domain.RunID, error) {
	return f.order, f.listErr
}

// LoadBatch skips what it does not hold, matching the port's documented behavior — which is the
// behavior the importer has to compensate for.
func (f *fakeSource) LoadBatch(
	_ context.Context, _ string, ids []domain.RunID,
) (map[domain.RunID]*domain.ReleaseRun, error) {
	loaded := map[domain.RunID]*domain.ReleaseRun{}
	for _, id := range ids {
		if run, ok := f.runs[id]; ok {
			loaded[id] = run
		}
	}
	return loaded, nil
}

func (f *fakeSource) LoadLatest(_ context.Context, _ string) (*domain.ReleaseRun, error) {
	if f.latest == nil {
		return nil, domain.ErrRunNotFound
	}
	return f.latest, nil
}

func (f *fakeSource) Load(_ context.Context, id domain.RunID) (*domain.ReleaseRun, error) {
	if run, ok := f.runs[id]; ok {
		return run, nil
	}
	return nil, domain.ErrRunNotFound
}

type fakeDestination struct {
	saved      []domain.RunID
	latest     domain.RunID
	latestSet  int
	failOnSave int // the 1-based call number that fails, zero for none
	saveErr    error
}

func (f *fakeDestination) LoadBatch(
	_ context.Context, _ string, _ []domain.RunID,
) (map[domain.RunID]*domain.ReleaseRun, error) {
	// Empty: a destination that holds nothing yet, so every run counts as created.
	return map[domain.RunID]*domain.ReleaseRun{}, nil
}

func (f *fakeDestination) Save(_ context.Context, run *domain.ReleaseRun) error {
	if f.failOnSave != 0 && len(f.saved)+1 == f.failOnSave {
		return f.saveErr
	}
	f.saved = append(f.saved, run.ID())
	return nil
}

func (f *fakeDestination) SetLatest(_ context.Context, _ string, id domain.RunID) error {
	f.latestSet++
	f.latest = id
	return nil
}

// sourceWith builds a source holding n runs, newest first, with the last one current.
func sourceWith(t *testing.T, n int) *fakeSource {
	t.Helper()

	src := &fakeSource{runs: map[domain.RunID]*domain.ReleaseRun{}}
	for i := 0; i < n; i++ {
		run := newRun(t, string(rune('a'+i))+"aa111")
		src.runs[run.ID()] = run
		src.order = append(src.order, run.ID())
		src.latest = run
	}
	return src
}

func newRun(t *testing.T, head string) *domain.ReleaseRun {
	t.Helper()

	// The aggregate stamps its own creation time, and the write order is derived from it, so
	// the runs in one history have to be distinguishable. A short sleep is enough and keeps
	// the runs ordinary aggregates rather than reconstructed ones.
	time.Sleep(time.Millisecond)

	return domain.NewReleaseRun(
		"acme/widget", repoRoot, "refs/tags/v1.2.0",
		domain.CommitSHA(head), []domain.CommitSHA{domain.CommitSHA(head)},
		"config-hash", "plugin-hash",
	)
}

func reversed(ids []domain.RunID) []domain.RunID {
	out := make([]domain.RunID, len(ids))
	for i, id := range ids {
		out[len(ids)-1-i] = id
	}
	return out
}
