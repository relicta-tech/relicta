package adapters

import (
	"bytes"
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// The recommendation artifact is served back over HTTP verbatim, because it
// carries a digest over its own content. These tests are mostly about bytes
// surviving unchanged.

func TestRecommendationRoundTripsByteForByte(t *testing.T) {
	root := t.TempDir()
	repo := NewFileReleaseRunRepository()

	// Deliberately not canonical JSON: whatever was produced is what must come
	// back, since a consumer verifies InputsDigest against these exact bytes.
	original := []byte("{\n  \"schema_version\": \"1.0.0\",\n  \"facts\": {}\n}\n")

	if err := repo.SaveRecommendation(root, "run-1", original); err != nil {
		t.Fatalf("SaveRecommendation: %v", err)
	}

	got, found, err := repo.LoadRecommendation(root, "run-1")
	if err != nil {
		t.Fatalf("LoadRecommendation: %v", err)
	}
	if !found {
		t.Fatal("the artifact was saved and must be found")
	}
	if !bytes.Equal(got, original) {
		t.Errorf("bytes changed in storage:\n stored: %q\n loaded: %q", original, got)
	}
}

// A run with no artifact is an ordinary case, not an error: runs planned before
// artifacts were persisted have none. The HTTP endpoint depends on this
// distinction to answer "nothing recorded" differently from "read failed".
func TestLoadRecommendationReportsAbsenceWithoutError(t *testing.T) {
	repo := NewFileReleaseRunRepository()

	got, found, err := repo.LoadRecommendation(t.TempDir(), "never-written")
	if err != nil {
		t.Errorf("a missing artifact is not an error: %v", err)
	}
	if found {
		t.Error("found must be false")
	}
	if got != nil {
		t.Errorf("expected no bytes, got %q", got)
	}
}

func TestSaveRecommendationOverwrites(t *testing.T) {
	root := t.TempDir()
	repo := NewFileReleaseRunRepository()

	if err := repo.SaveRecommendation(root, "run-1", []byte(`{"v":1}`)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := repo.SaveRecommendation(root, "run-1", []byte(`{"v":2}`)); err != nil {
		t.Fatalf("second save: %v", err)
	}

	got, _, err := repo.LoadRecommendation(root, "run-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if string(got) != `{"v":2}` {
		t.Errorf("expected the second artifact, got %q", got)
	}
}

// Artifacts must not collide across runs, and must not collide with the run file
// itself — they live in the same directory, distinguished only by suffix.
func TestRecommendationIsPerRunAndDoesNotClobberTheRun(t *testing.T) {
	root := t.TempDir()
	repo := NewFileReleaseRunRepository()

	if err := repo.SaveRecommendation(root, "run-a", []byte(`{"id":"a"}`)); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := repo.SaveRecommendation(root, "run-b", []byte(`{"id":"b"}`)); err != nil {
		t.Fatalf("save b: %v", err)
	}

	a, _, _ := repo.LoadRecommendation(root, "run-a")
	b, _, _ := repo.LoadRecommendation(root, "run-b")
	if string(a) != `{"id":"a"}` || string(b) != `{"id":"b"}` {
		t.Errorf("artifacts collided: a=%q b=%q", a, b)
	}

	// And the run's own path must be untouched by an artifact write.
	if runPath(root, "run-a") == recommendationPath(root, "run-a") {
		t.Error("the run file and its artifact resolve to the same path")
	}
}

// The HTTP handler reaches these methods by asserting this interface. Without the
// assertion the endpoint answers 501 and the feature is silently absent — the
// failure mode this codebase keeps producing.
func TestFileRepositorySatisfiesRecommendationStore(t *testing.T) {
	var _ ports.RecommendationStore = NewFileReleaseRunRepository()
}

// Guards against a runID being used to escape the runs directory, since it
// arrives from a URL parameter in the HTTP path.
func TestRecommendationPathIsConfinedToTheRunsDirectory(t *testing.T) {
	root := t.TempDir()
	repo := NewFileReleaseRunRepository()

	if err := repo.SaveRecommendation(root, domain.RunID("../escaped"), []byte(`{}`)); err != nil {
		t.Fatalf("save: %v", err)
	}

	// filepath.Base is applied when building the path, so the traversal collapses
	// to a plain name inside the runs directory rather than writing above it.
	if _, found, _ := repo.LoadRecommendation(root, domain.RunID("../escaped")); !found {
		t.Error("expected the write to land somewhere readable by the same key")
	}
	if got := recommendationPath(root, domain.RunID("../escaped")); !bytes.Contains([]byte(got), []byte("escaped")) {
		t.Errorf("unexpected path %q", got)
	}
	if bytes.Contains([]byte(recommendationPath(root, domain.RunID("../escaped"))), []byte("..")) {
		t.Error("the path still contains a traversal segment")
	}
}

// List returns run IDs, and the runs directory also holds sibling artifacts that
// end in .json. Adding the recommendation file made List return
// "run-x.recommendation" as a run ID; runPath then resolved that to the artifact's
// own path, LoadBatch read it, and JSON's tolerance of unknown fields turned it
// into a ReleaseRun with an empty ID and a zero risk score. The dashboard showed a
// phantom release and nothing errored.
func TestListIgnoresSiblingArtifacts(t *testing.T) {
	root := t.TempDir()
	repo := NewFileReleaseRunRepository()
	ctx := context.Background()

	run := domain.NewReleaseRun("repo", root, "v1.0.0", "abc123",
		[]domain.CommitSHA{"abc123"}, "cfg", "plugins")
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Every sibling the store writes next to a run.
	if err := repo.SaveRecommendation(root, run.ID(), []byte(`{"schema_version":"1.0.0"}`)); err != nil {
		t.Fatalf("SaveRecommendation: %v", err)
	}
	if err := repo.SaveMachineJSON(root, run.ID(), []byte(`{"states":{}}`)); err != nil {
		t.Fatalf("SaveMachineJSON: %v", err)
	}

	ids, err := repo.List(ctx, root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("List returned %d runs (%v), want exactly the one saved run", len(ids), ids)
	}
	if ids[0] != run.ID() {
		t.Errorf("List returned %q, want %q", ids[0], run.ID())
	}

	// The consequence the count above prevents: a phantom run that loads from an
	// artifact file and reports no ID.
	runs := make([]domain.RunID, 0, len(ids))
	loaded, err := repo.LoadBatch(ctx, root, append(runs, ids...))
	if err != nil {
		t.Fatalf("LoadBatch: %v", err)
	}
	for id, r := range loaded {
		if r.ID() == "" {
			t.Errorf("id %q loaded a run with an empty ID — an artifact was read as a run", id)
		}
	}
}
