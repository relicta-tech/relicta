package recommendation

import (
	"encoding/json"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// The artifact is stored rather than recomputed on read. A reconstruction from a
// stored run would be lossy in a way the shape hides — risk factors and required
// actions are not persisted on a run, and Assessment.Factors serializes as `[]`
// rather than being omitted, so "no factors were computed" would be served as "no
// factors exist".

type fakeStore struct {
	saved    map[domain.RunID][]byte
	saveErr  error
	rootSeen string
}

func newFakeStore() *fakeStore {
	return &fakeStore{saved: map[domain.RunID][]byte{}}
}

func (f *fakeStore) SaveRecommendation(repoRoot string, runID domain.RunID, artifact []byte) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.rootSeen = repoRoot
	f.saved[runID] = artifact
	return nil
}

func (f *fakeStore) LoadRecommendation(_ string, runID domain.RunID) ([]byte, bool, error) {
	data, ok := f.saved[runID]
	return data, ok, nil
}

// A repository with no artifact storage — the shape Persist must tolerate.
type storelessRepo struct{}

func TestPersistStoresTheArtifact(t *testing.T) {
	store := newFakeStore()
	artifact := &Artifact{SchemaVersion: SchemaVersion, Subject: Subject{Repository: "owner/repo"}}

	stored, err := Persist(store, "/repo", "run-1", artifact)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if !stored {
		t.Fatal("expected the artifact to be stored")
	}
	if store.rootSeen != "/repo" {
		t.Errorf("repoRoot = %q, want /repo", store.rootSeen)
	}

	// It has to come back as a readable artifact, not merely as bytes.
	var round Artifact
	if err := json.Unmarshal(store.saved["run-1"], &round); err != nil {
		t.Fatalf("stored bytes are not valid JSON: %v", err)
	}
	if round.Subject.Repository != "owner/repo" {
		t.Errorf("round-tripped subject = %q", round.Subject.Repository)
	}
}

// A repository without artifact storage must not be an error. Storage is an
// addition, and a caller holding an older repository should keep working.
func TestPersistToleratesAStorelessRepository(t *testing.T) {
	stored, err := Persist(storelessRepo{}, "/repo", "run-1", &Artifact{})
	if err != nil {
		t.Errorf("a repository without artifact storage is not an error: %v", err)
	}
	if stored {
		t.Error("nothing can have been stored")
	}
}

func TestPersistIgnoresIncompleteInput(t *testing.T) {
	store := newFakeStore()

	cases := map[string]struct {
		artifact *Artifact
		root     string
		runID    domain.RunID
	}{
		"no artifact": {nil, "/repo", "run-1"},
		"no run id":   {&Artifact{}, "/repo", ""},
		"no root":     {&Artifact{}, "", "run-1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stored, err := Persist(store, tc.root, tc.runID, tc.artifact)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if stored {
				t.Error("expected nothing to be stored")
			}
		})
	}

	if len(store.saved) != 0 {
		t.Errorf("store received %d writes, want 0", len(store.saved))
	}
}

// A storage failure has to surface. Reported and non-fatal is the caller's
// choice; swallowing it here would make the HTTP endpoint 404 with no
// explanation of why the artifact is missing.
func TestPersistReportsStorageFailure(t *testing.T) {
	store := newFakeStore()
	store.saveErr = failedWriteError{}

	stored, err := Persist(store, "/repo", "run-1", &Artifact{})
	if err == nil {
		t.Fatal("a failed write must be reported")
	}
	if stored {
		t.Error("stored must be false when the write failed")
	}
}

type failedWriteError struct{}

func (failedWriteError) Error() string { return "disk full" }
