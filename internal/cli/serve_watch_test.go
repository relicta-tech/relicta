package cli

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// The dashboard hears domain events from its own process, and its own process never publishes:
// `relicta publish` is a separate command, usually on a developer's machine or in CI. Wiring
// the health watch to that subscription alone would have pointed it at a signal that never
// arrives — the defect this whole pass exists to remove, one level up.
//
// So the server reads the store the CLI writes to, and these cover which releases it picks up.

type startedWatches struct{ ids []string }

func (s *startedWatches) StartWatch(_ context.Context, releaseID string) error {
	s.ids = append(s.ids, releaseID)
	return nil
}

// publishedRun stores a run in the published state, published at the given time.
//
// Reconstructed rather than transitioned, because the point of the test is a release published
// at a particular moment — including one published days ago, which no sequence of transitions
// can produce today.
func publishedRun(t *testing.T, repo *adapters.FileReleaseRunRepository, root, id string, at time.Time) {
	t.Helper()

	run := release.NewReleaseRunForTestWithCommits(release.RunID(id), "main", root)
	publishedAt := at
	run.ReconstructState(release.RunSnapshot{
		ID:          release.RunID(id),
		RepoID:      root,
		RepoRoot:    root,
		BaseRef:     "v0.0.0",
		VersionNext: version.MustParse("1.0.0"),
		State:       release.StatePublished,
		CreatedAt:   at,
		PublishedAt: &publishedAt,
	})

	if err := repo.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestOnlyReleasesInsideTheWindowArePickedUp(t *testing.T) {
	root := t.TempDir()
	repo := adapters.NewFileReleaseRunRepository()

	publishedRun(t, repo, root, "run-recent", time.Now().Add(-time.Minute))
	publishedRun(t, repo, root, "run-old", time.Now().Add(-72*time.Hour))

	watches := &startedWatches{}
	services := &release.Services{Repository: repo}

	pickUpPublishedReleases(context.Background(), watches, services, root,
		30*time.Minute, map[string]struct{}{})

	if len(watches.ids) != 1 || watches.ids[0] != "run-recent" {
		t.Errorf("started watches for %v, want only run-recent.\nWatching a release from last "+
			"week attributes today's metrics to it, which is the wrong-data failure one step "+
			"along from the one this is all about", watches.ids)
	}
}

// A release already being watched is not watched twice: StartWatch refuses a duplicate, and
// the refusal would be logged on every poll for the length of the window.
func TestAReleaseIsNotPickedUpTwice(t *testing.T) {
	root := t.TempDir()
	repo := adapters.NewFileReleaseRunRepository()
	publishedRun(t, repo, root, "run-recent", time.Now())

	watches := &startedWatches{}
	services := &release.Services{Repository: repo}
	seen := map[string]struct{}{}

	pickUpPublishedReleases(context.Background(), watches, services, root, time.Hour, seen)
	pickUpPublishedReleases(context.Background(), watches, services, root, time.Hour, seen)

	if len(watches.ids) != 1 {
		t.Errorf("started %d watches across two polls, want 1: %v", len(watches.ids), watches.ids)
	}
}
