package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// Governance outcomes were recorded under the repository's absolute checkout path
// and read under other identities, so records accumulated and were never found:
// `relicta history` was empty in every repository, and earned trust could never
// find the history it escalates on. Records written before the identity was made
// canonical are still path-keyed, and dropping them silently would leave a store
// that looks healthy and knows nothing about this repository's past — the same
// failure at the migration boundary.

func recordAt(repository, version string) *ReleaseRecord {
	return &ReleaseRecord{
		ID:         "run-" + version,
		Repository: repository,
		Version:    version,
		ReleasedAt: time.Unix(1_700_000_000, 0).UTC(),
		Outcome:    OutcomeSuccess,
	}
}

func TestAdoptLegacyRepositoryKey(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	ctx := context.Background()

	checkout := "/Users/dev/code/widget"
	legacy := checkout
	canonical := "acme/widget"

	if err := store.RecordRelease(ctx, recordAt(legacy, "1.0.0")); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}

	// Precondition: the canonical identity finds nothing, which is the state a user
	// upgrading into the fix is in.
	before, err := store.GetReleaseHistory(ctx, canonical, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("precondition: expected no history under %q, got %d", canonical, len(before))
	}

	adopted, err := store.AdoptLegacyRepositoryKey(ctx, canonical, checkout)
	if err != nil {
		t.Fatalf("AdoptLegacyRepositoryKey: %v", err)
	}
	if adopted != 1 {
		t.Errorf("adopted %d records, want 1", adopted)
	}

	after, err := store.GetReleaseHistory(ctx, canonical, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(after) != 1 || after[0].Version != "1.0.0" {
		t.Errorf("the record did not move to the canonical identity: %+v", after)
	}

	// Moved, not copied: a legacy key left in place would shadow nothing but would
	// be migrated again by every future process.
	stale, err := store.GetReleaseHistory(ctx, legacy, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory(legacy): %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("the legacy key still holds %d records", len(stale))
	}
}

// The migration must survive the process, or every run would redo it.
func TestAdoptionIsPersisted(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if err := store.RecordRelease(ctx, recordAt("/Users/dev/code/widget", "1.0.0")); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
	if _, err := store.AdoptLegacyRepositoryKey(ctx, "acme/widget", "/Users/dev/code/widget"); err != nil {
		t.Fatalf("AdoptLegacyRepositoryKey: %v", err)
	}

	reopened, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	history, err := reopened.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("a reopened store found %d records under the canonical identity, want 1", len(history))
	}
}

// A wrong history is worse than a missing one, because nothing downstream can tell
// it is wrong: it would feed another repository's risk calibration and actor
// reputation into this one. So adoption matches on the directory name rather than
// taking any path-keyed records it finds.
func TestAdoptionDoesNotStealAnotherRepositorysHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	ctx := context.Background()

	if err := store.RecordRelease(ctx, recordAt("/Users/dev/code/other-project", "9.9.9")); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}

	adopted, err := store.AdoptLegacyRepositoryKey(ctx, "acme/widget", "/Users/dev/code/widget")
	if err != nil {
		t.Fatalf("AdoptLegacyRepositoryKey: %v", err)
	}
	if adopted != 0 {
		t.Errorf("adopted %d records from an unrelated repository", adopted)
	}

	history, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("widget inherited %d records from other-project", len(history))
	}
}

// Existing canonical history is never overwritten. Adoption is a one-time repair,
// and running it against a repository that already has records must not merge a
// stale path-keyed set into a current one.
func TestAdoptionSkipsWhenCanonicalHistoryExists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	ctx := context.Background()

	if err := store.RecordRelease(ctx, recordAt("acme/widget", "2.0.0")); err != nil {
		t.Fatalf("RecordRelease(canonical): %v", err)
	}
	if err := store.RecordRelease(ctx, recordAt("/Users/dev/code/widget", "1.0.0")); err != nil {
		t.Fatalf("RecordRelease(legacy): %v", err)
	}

	adopted, err := store.AdoptLegacyRepositoryKey(ctx, "acme/widget", "/Users/dev/code/widget")
	if err != nil {
		t.Fatalf("AdoptLegacyRepositoryKey: %v", err)
	}
	if adopted != 0 {
		t.Errorf("adopted %d records into a repository that already had history", adopted)
	}
}

// Only paths are adopted. A canonical identity contains a slash too, so the test
// exists to keep "acme/widget" from ever being treated as a legacy key and moved
// onto something else.
func TestLegacyPathKeyRecognition(t *testing.T) {
	cases := map[string]bool{
		"/Users/dev/code/widget": true,
		`C:\Users\dev\widget`:    true,
		"C:/Users/dev/widget":    true,
		`\\server\share\widget`:  true,
		"acme/widget":            false,
		"local:widget":           false,
		"widget":                 false,
		"":                       false,
		"gitlab/group/subgroup":  false,
	}

	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			if got := legacyPathKey(key); got != want {
				t.Errorf("legacyPathKey(%q) = %v, want %v", key, got, want)
			}
		})
	}

	// Sanity: the migration keys on the final path segment, and a canonical
	// identity's final segment can match a directory name. The guard above is what
	// stops that becoming a move.
	if filepath.Base("acme/widget") != "widget" {
		t.Fatal("precondition changed")
	}
}
