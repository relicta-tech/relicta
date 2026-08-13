package handlers

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Every handler in this package resolved runs with Repository.Load, and the whole
// dashboard API therefore returned nothing.
//
// Load finds a run by scanning repository roots the repository has seen during
// this process, and it learns them from Save. The HTTP server never saves a run —
// the CLI does — so its repository has seen no roots, and Load returned "release
// run not found" for every run sitting on disk. The visible result was a 200 with
// an empty body:
//
//	GET /api/v1/releases/         -> {"data":[],"total":0}
//	GET /api/v1/governance/decisions -> {"data":null,"total":0}
//	GET /api/v1/actors/          -> {"data":[],"total":0}
//
// on a repository with a planned release. List worked, because List takes the root
// as an argument; the Load inside each loop failed, and each loop's `continue`
// turned the failure into absence. An empty dashboard reads as "no releases yet",
// which is why this survived.
//
// The same defect was fixed for the CLI by routing through LoadBatch, which takes
// the root explicitly (issue #247). These helpers do the same for the handlers, so
// there is one way to resolve a run here and it is the one that works.

// loadRun resolves a single run, addressing the store by repository root.
func loadRun(ctx context.Context, repo ports.RunReader, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
	runs, err := repo.LoadBatch(ctx, repoRoot, []domain.RunID{runID})
	if err != nil {
		return nil, err
	}
	run := runs[runID]
	if run == nil {
		return nil, fmt.Errorf("release run %s not found", runID)
	}
	return run, nil
}

// loadRuns resolves many runs in the order requested, skipping any that cannot be
// read.
//
// One call rather than one per ID: the loops this replaces issued a Load per run,
// and LoadBatch is the operation the store actually offers for this.
func loadRuns(ctx context.Context, repo ports.RunReader, repoRoot string, runIDs []domain.RunID) []*domain.ReleaseRun {
	if len(runIDs) == 0 {
		return nil
	}

	runs, err := repo.LoadBatch(ctx, repoRoot, runIDs)
	if err != nil {
		return nil
	}

	ordered := make([]*domain.ReleaseRun, 0, len(runIDs))
	for _, id := range runIDs {
		if run := runs[id]; run != nil {
			ordered = append(ordered, run)
		}
	}
	return ordered
}
