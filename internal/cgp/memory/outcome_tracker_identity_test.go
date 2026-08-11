package memory

import (
	"context"
	"testing"
	"time"

	release "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// The outcome tracker is wired as an event-publisher decorator (container.go), so it
// writes a release record for every run that publishes, fails or is canceled. Two
// defects made those records unusable, and they are independent:
//
//  1. The repository was the raw value from RunCreatedEvent.RepoID — a git remote URL,
//     since the plan use case sets it from GetRemoteURL. Every reader queries by
//     governance identity ("acme/widget"), so the records landed in a bucket nothing
//     reads and accumulated there unseen.
//
//  2. The cached version was only ever set by a handler for RunPlannedEvent, which the
//     aggregate never raises. The success path got away with it because
//     handlePublished overwrites the version from RunPublishedEvent, which carries
//     one — but RunFailedEvent and RunCanceledEvent do not, so failed and canceled
//     releases were recorded with no version at all and could not be tied to the
//     version that failed. That is the half of the history change failure rate is
//     computed from.

func trackerFor(store Store) *OutcomeTracker { return NewOutcomeTracker(store, nil) }

func mustVersion(t *testing.T, v string) version.SemanticVersion {
	t.Helper()
	parsed, err := version.Parse(v)
	if err != nil {
		t.Fatalf("version.Parse(%q): %v", v, err)
	}
	return parsed
}

// A record readers can actually find.
func TestTrackerRecordsAreKeyedForReaders(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := trackerFor(store).Publish(ctx,
		&release.RunCreatedEvent{RunID: "run-1", RepoID: "https://github.com/acme/widget.git", At: now.Add(-time.Hour)},
		&release.RunVersionedEvent{RunID: "run-1", VersionNext: mustVersion(t, "1.4.0"), At: now.Add(-30 * time.Minute)},
		&release.RunPublishedEvent{RunID: "run-1", Version: mustVersion(t, "1.4.0"), At: now},
	); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	records, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("found %d records under acme/widget, want 1: a record keyed under the raw "+
			"remote URL is invisible to history, the reports, reconcile and the deployment gate",
			len(records))
	}
	if records[0].Version != "1.4.0" {
		t.Errorf("Version = %q, want 1.4.0", records[0].Version)
	}
}

// The defect that survived on the success path. A failed release must still name the
// version that failed, or change failure rate cannot attribute the failure to anything.
func TestAFailedReleaseKeepsItsVersion(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	// RunFailedEvent carries no version of its own, so the only source is what the
	// versioned event cached.
	if err := trackerFor(store).Publish(ctx,
		&release.RunCreatedEvent{RunID: "run-2", RepoID: "https://github.com/acme/widget.git", At: now.Add(-time.Hour)},
		&release.RunVersionedEvent{RunID: "run-2", VersionNext: mustVersion(t, "2.1.0"), At: now.Add(-time.Minute)},
		&release.RunFailedEvent{RunID: "run-2", Reason: "publish step failed", At: now},
	); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	records, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("found %d records, want 1", len(records))
	}
	if records[0].Version != "2.1.0" {
		t.Errorf("Version = %q, want 2.1.0; an empty version means it is still only set by "+
			"a handler for an event the aggregate never raises, so no failed release can be "+
			"tied to the version that failed", records[0].Version)
	}
	if records[0].Outcome != OutcomeFailed {
		t.Errorf("Outcome = %q, want failed", records[0].Outcome)
	}
}

// Both remote spellings must reduce to one key, or the same repository is recorded
// twice depending on how it happened to be cloned.
func TestTrackerNormalizesEveryRemoteForm(t *testing.T) {
	for _, remote := range []string{
		"https://github.com/acme/widget.git",
		"git@github.com:acme/widget.git",
		"ssh://git@github.com:2222/acme/widget",
	} {
		store := NewInMemoryStore()
		ctx := context.Background()
		now := time.Now().UTC()

		if err := trackerFor(store).Publish(ctx,
			&release.RunCreatedEvent{RunID: "run-3", RepoID: remote, At: now.Add(-time.Hour)},
			&release.RunPublishedEvent{RunID: "run-3", Version: mustVersion(t, "3.0.0"), At: now},
		); err != nil {
			t.Fatalf("Publish: %v", err)
		}

		records, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
		if err != nil {
			t.Fatalf("GetReleaseHistory: %v", err)
		}
		if len(records) != 1 {
			t.Errorf("remote %q produced %d records under acme/widget, want 1", remote, len(records))
		}
	}
}

// A filesystem path must not be run through the remote parser. Its last two segments
// form a plausible-looking pair — "/Users/dev/checkout" becomes "dev/checkout" — that
// cannot be told apart from a real owner/repo, so it silently keys records to a
// repository that does not exist. The "local:" prefix is what GovernanceID produces for
// the same case, so both writers agree.
func TestAPathIsNotMistakenForARemote(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := trackerFor(store).Publish(ctx,
		&release.RunCreatedEvent{RunID: "run-4", RepoID: "/Users/dev/checkout", At: now.Add(-time.Hour)},
		&release.RunPublishedEvent{RunID: "run-4", Version: mustVersion(t, "1.0.0"), At: now},
	); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if bogus, err := store.GetReleaseHistory(ctx, "dev/checkout", 10); err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	} else if len(bogus) != 0 {
		t.Errorf("%d records were keyed to the invented repository \"dev/checkout\"", len(bogus))
	}

	records, err := store.GetReleaseHistory(ctx, "local:checkout", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("found %d records under local:checkout, want 1: a local repository's outcome "+
			"must still be recorded somewhere a reader can name", len(records))
	}
}

// A caller that supplied an identity deliberately must get that identity back. This
// case was caught by an existing test (TestOutcomeTracker_EventChaining) after a first
// version of the normalizer rewrote "owner/repo" to "local:repo" — moving records away
// from where the caller reads them, which is the same class of bug the normalizer is
// meant to fix.
func TestAnIdentityIsPassedThroughUnchanged(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := trackerFor(store).Publish(ctx,
		&release.RunCreatedEvent{RunID: "run-5", RepoID: "acme/widget", At: now.Add(-time.Hour)},
		&release.RunPublishedEvent{RunID: "run-5", Version: mustVersion(t, "1.0.0"), At: now},
	); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	records, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("found %d records under acme/widget, want 1: an identity the caller supplied "+
			"must not be rewritten", len(records))
	}
}
