// Package releasehistory copies a repository's release history from one store into another.
//
// ADR-013 puts three adapters behind persistence.backend and keeps `file` the default until
// the payoff is earned on evidence: a conformance suite passing on all three adapters, and an
// importer with a round trip test. This is that importer's core — the part that only knows two
// implementations of ports.ReleaseRunRepository and moves runs between them.
//
// It is deliberately ignorant of which backends those are. Selecting a backend happens in one
// place (persistence.OpenReleaseRunStore) and refusing a nonsensical pair happens where the
// configuration is known (the composition root); what is left here is the transfer itself,
// which is the part a round trip test can hold still and read every field back out of.
//
// Three properties are the whole design:
//
//   - Non-destructive. The source is only ever read. ADR-013 says the JSON tree stays as an
//     export until the operator removes it, so nothing here deletes, moves or rewrites it.
//   - Idempotent. Both database adapters upsert by run ID, so a second import converges on the
//     same history rather than duplicating it. The report separates runs it created from runs
//     it replaced so an operator can see which of the two happened.
//   - Complete or reported. The source is read in full and verified before the first write, and
//     a write that fails stops the import with a count of what was written. A silent partial
//     migration is the worst outcome for an audit trail: it looks like a migration.
package releasehistory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Source is the store being read. Reads only — see the package comment.
type Source = ports.RunReader

// Destination is the store being written.
//
// LoadBatch rather than Load for the existence check, because it is the root-scoped read every
// adapter answers the same way: the file adapter's Load searches whichever roots the process
// has happened to touch, so on a freshly constructed repository it finds nothing.
type Destination interface {
	LoadBatch(ctx context.Context, repoRoot string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error)
	Save(ctx context.Context, run *domain.ReleaseRun) error
	SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error
}

// Options says which repository to import and whether to write anything.
type Options struct {
	// RepoRoot scopes both stores: it is the tree whose history is read and the key the
	// destination files the runs under.
	RepoRoot string

	// DryRun reports what would be imported and writes nothing.
	DryRun bool
}

// Report is what the import did, in the terms an operator can check it against.
type Report struct {
	// RepoRoot is the repository whose history was read.
	RepoRoot string

	// Runs is how many runs the source holds.
	Runs int

	// Created and Replaced split the writes: a run the destination did not have, versus one
	// it did. On a second import every run is Replaced, which is what makes idempotence
	// visible rather than merely claimed.
	Created  int
	Replaced int

	// Latest is the run the source calls the current release, empty when it has none.
	Latest domain.RunID

	// LatestTransferred says the destination's pointer now names Latest. False after a dry
	// run, and false when a write failed before the pointer was reached — a history whose
	// current release is missing is not a migrated history, so the operator has to be able
	// to tell.
	LatestTransferred bool

	// ForeignRoots names runs whose own stored repository root is not RepoRoot.
	//
	// They are imported unchanged — rewriting a stored root would edit an audit record to
	// make a migration look tidier — but they are reported, because both database adapters
	// scope every query by the root the run carries. Without this line the import would say
	// it moved twelve runs and `relicta history` would show nine.
	ForeignRoots []domain.RunID

	// DryRun records that nothing was written.
	DryRun bool
}

// Written is how many runs reached the destination.
func (r Report) Written() int { return r.Created + r.Replaced }

// Import copies every run in src into dst, then transfers the latest pointer.
//
// The order matters and is the answer to "what does the operator have when run 7 of 12 fails":
// runs first, pointer last. A partial import leaves a destination holding some history and no
// current release, which reads as unfinished — the state it is in. The reverse order would
// leave a destination that answers "what am I releasing" confidently out of a history with
// holes in it.
//
// On failure the report is returned alongside the error, populated with what was written, so
// the caller can say how far it got instead of only that it stopped.
func Import(ctx context.Context, src Source, dst Destination, opts Options) (Report, error) {
	report := Report{RepoRoot: opts.RepoRoot, DryRun: opts.DryRun}

	if opts.RepoRoot == "" {
		return report, errors.New("importing release history: no repository root given")
	}

	ids, err := src.List(ctx, opts.RepoRoot)
	if err != nil {
		return report, fmt.Errorf("reading the release history in %s: %w", opts.RepoRoot, err)
	}

	runs, err := readAll(ctx, src, opts.RepoRoot, ids)
	if err != nil {
		return report, err
	}

	// The latest pointer is resolved before anything is written, so a source that cannot
	// answer it fails the import rather than half of it.
	//
	// LoadLatest is the only read of the pointer the port offers, which means an absent
	// pointer and a pointer naming a run that is gone are the same answer here. They are also
	// the same answer to `relicta status` — "no current release" — so collapsing them loses
	// nothing an operator could act on.
	latest, err := src.LoadLatest(ctx, opts.RepoRoot)
	switch {
	case err == nil && latest != nil:
		report.Latest = latest.ID()
		// A pointer at a run that List did not return still has to arrive, or the
		// destination would name a current release it does not hold. List skips sibling
		// artifacts structurally, so this is cheap insurance rather than a known case.
		if _, ok := runs[latest.ID()]; !ok {
			ids = append(ids, latest.ID())
			runs[latest.ID()] = latest
		}
	case errors.Is(err, domain.ErrRunNotFound):
		// No current release. An ordinary state for a repository that has planned runs
		// without publishing one.
	case err != nil:
		return report, fmt.Errorf("reading the current release pointer in %s: %w", opts.RepoRoot, err)
	}

	report.Runs = len(runs)
	report.ForeignRoots = foreignRoots(runs, opts.RepoRoot)

	if len(runs) == 0 {
		// Not an error. A repository with no history has nothing to import, and saying so
		// is more useful than a non-zero exit an operator has to interpret.
		return report, nil
	}

	if opts.DryRun {
		return report, nil
	}

	existing, err := dst.LoadBatch(ctx, opts.RepoRoot, ids)
	if err != nil {
		return report, fmt.Errorf("checking which runs the destination already holds: %w", err)
	}

	// Oldest first. List hands back most-recently-changed first, and importing in that order
	// would make an interrupted import hold the *end* of a history — the shape that looks
	// most like a complete one.
	for _, id := range writeOrder(runs) {
		if err := dst.Save(ctx, runs[id]); err != nil {
			// The message says which run, how far the import got, and what state that
			// leaves the operator in. "failed to import release history" would tell them
			// none of the three, and the first thing they would do is go and find out.
			return report, fmt.Errorf(
				"importing run %s after %d of %d runs: %s and its current release pointer "+
					"was not set; the release history under %s was not modified, so running "+
					"the import again is safe: %w",
				id, report.Written(), len(runs), destinationStateAfterFailure(report), opts.RepoRoot, err)
		}
		if _, had := existing[id]; had {
			report.Replaced++
		} else {
			report.Created++
		}
	}

	if report.Latest != "" {
		if err := dst.SetLatest(ctx, opts.RepoRoot, report.Latest); err != nil {
			return report, fmt.Errorf(
				"pointing the destination at the current release %s: all %d runs were "+
					"imported but the destination does not know which one is current, so "+
					"running the import again is both safe and the fix: %w",
				report.Latest, report.Written(), err)
		}
		report.LatestTransferred = true
	}

	return report, nil
}

// destinationStateAfterFailure describes what the destination holds, accurately for the case
// where the very first write failed and it holds nothing at all.
func destinationStateAfterFailure(report Report) string {
	if report.Written() == 0 {
		return "no runs reached the destination"
	}
	return "the destination now holds a partial history"
}

// readAll loads every listed run and refuses the import if one cannot be read.
//
// LoadBatch skips what it cannot load, by contract — which is right for a caller rendering a
// history and wrong for one copying it, because a run silently dropped here is a record that
// disappears from an audit trail with a success message over it. Comparing the returned map
// against the IDs that were asked for is how that becomes a failure, and doing it before the
// first write means the operator gets a destination that is untouched rather than incomplete.
func readAll(
	ctx context.Context, src Source, repoRoot string, ids []domain.RunID,
) (map[domain.RunID]*domain.ReleaseRun, error) {
	runs := map[domain.RunID]*domain.ReleaseRun{}
	if len(ids) == 0 {
		return runs, nil
	}

	loaded, err := src.LoadBatch(ctx, repoRoot, ids)
	if err != nil {
		return nil, fmt.Errorf("reading the release runs in %s: %w", repoRoot, err)
	}

	var unreadable []domain.RunID
	for _, id := range ids {
		run, ok := loaded[id]
		if !ok || run == nil {
			unreadable = append(unreadable, id)
			continue
		}
		runs[id] = run
	}

	if len(unreadable) > 0 {
		sortIDs(unreadable)
		return nil, fmt.Errorf(
			"the release history in %s lists %d run(s) that cannot be read: %v; nothing was "+
				"imported, because a history missing a run is worse than one not yet moved",
			repoRoot, len(unreadable), unreadable)
	}

	return runs, nil
}

// foreignRoots reports runs stored under a repository root other than the one being imported.
func foreignRoots(runs map[domain.RunID]*domain.ReleaseRun, repoRoot string) []domain.RunID {
	want := normalizeRoot(repoRoot)

	var foreign []domain.RunID
	for id, run := range runs {
		if normalizeRoot(run.RepoRoot()) != want {
			foreign = append(foreign, id)
		}
	}
	sortIDs(foreign)
	return foreign
}

// normalizeRoot resolves a path the way the database adapters do, so a comparison here agrees
// with the key they file a run under. Symlinks matter: macOS /tmp is /private/tmp, and a run
// read through one and stored under the other would be reported as foreign to its own
// repository.
func normalizeRoot(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// writeOrder returns the run IDs oldest first, by the run's own creation time.
//
// The aggregate's timestamp rather than the source's ordering, so the sequence a destination
// receives does not depend on which adapter was read.
func writeOrder(runs map[domain.RunID]*domain.ReleaseRun) []domain.RunID {
	ids := make([]domain.RunID, 0, len(runs))
	for id := range runs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := runs[ids[i]], runs[ids[j]]
		if left.CreatedAt().Equal(right.CreatedAt()) {
			// A total order, so an import writes the same sequence every time it runs.
			return ids[i] < ids[j]
		}
		return left.CreatedAt().Before(right.CreatedAt())
	})
	return ids
}

func sortIDs(ids []domain.RunID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
}
