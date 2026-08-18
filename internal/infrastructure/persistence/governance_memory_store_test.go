package persistence

// governance_memory_store_test.go covers what persistence.backend now decides for the
// governance record.
//
// The assertions are about where the bytes went, not about a call returning nil, because the
// defect ADR-013 was written about is a call that returns nil while the bytes go somewhere the
// operator did not ask for.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// unreachableGovernancePostgres names a port nothing listens on, so the connection is refused
// at once instead of waiting out the ten second budget.
const unreachableGovernancePostgres = "postgres://relicta@127.0.0.1:1/relicta?sslmode=disable"

// The default is the whole compatibility promise: no persistence section must mean the store
// relicta has always written, in the file it has always written it to.
func TestTheDefaultBackendStillWritesGovernanceMemoryToJSON(t *testing.T) {
	repoRoot := t.TempDir()
	fileDir := filepath.Join(repoRoot, ".relicta", "governance")

	store := openGovernance(t, config.PersistenceConfig{}, repoRoot, fileDir)
	recordRelease(t, store.Store, "rel-1")

	if store.Backend != config.BackendFile {
		t.Errorf("an empty persistence section resolved to backend %q, want file: wiring the "+
			"setting changed the default store, which moves every existing user's governance "+
			"history on upgrade", store.Backend)
	}
	if _, err := os.Stat(filepath.Join(fileDir, "memory.json")); err != nil {
		t.Errorf("no memory.json after recording a release with no persistence section: %v", err)
	}
	if store.Closer != nil {
		t.Error("the file backend returned a Closer: it holds no connection, and a caller " +
			"registering one would be closing nothing on shutdown")
	}
}

func TestTheSQLiteBackendWritesGovernanceMemoryToTheDatabaseAndNotToJSON(t *testing.T) {
	repoRoot := t.TempDir()
	fileDir := filepath.Join(repoRoot, ".relicta", "governance")

	store := openGovernance(t, sqliteBackend(), repoRoot, fileDir)
	recordRelease(t, store.Store, "rel-1")

	if _, err := os.Stat(filepath.Join(repoRoot, ".relicta", "relicta.db")); err != nil {
		t.Errorf("no .relicta/relicta.db after recording with `backend: sqlite`: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fileDir, "memory.json")); err == nil {
		t.Error("memory.json was written as well as the database: two copies of the governance " +
			"record that can disagree is the failure ADR-013 names as the worst available one")
	}

	history, err := store.Store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil || len(history) != 1 {
		t.Fatalf("GetReleaseHistory from the sqlite backend returned %d records, %v — a store "+
			"that cannot be read back is a lost audit trail", len(history), err)
	}
}

// The governance store and the release run store must land in one SQLite file. ADR-013 puts
// one backend behind the setting so a run and the record it produces can eventually be written
// in one transaction, and two files cannot be.
func TestTheSQLiteGovernanceStoreSharesTheReleaseRunDatabaseFile(t *testing.T) {
	repoRoot := t.TempDir()

	governance := openGovernance(t, sqliteBackend(), repoRoot, filepath.Join(repoRoot, "gov"))
	runs, err := OpenReleaseRunStore(context.Background(), sqliteBackend(), repoRoot)
	if err != nil {
		t.Fatalf("OpenReleaseRunStore: %v", err)
	}
	t.Cleanup(func() {
		if runs.Closer != nil {
			_ = runs.Closer.Close()
		}
	})

	if governance.Location != runs.Location {
		t.Errorf("governance memory is at %s and the release runs are at %s: separate files "+
			"cannot be written in one transaction, so a crash between the two leaves the "+
			"record and the run disagreeing", governance.Location, runs.Location)
	}
}

// The defect ADR-013 was written about, in the governance half. An operator who asked for
// postgres and silently got local JSON is the outcome that must be impossible.
func TestAnUnreachablePostgresGovernanceStoreFailsInsteadOfFallingBackToJSON(t *testing.T) {
	repoRoot := t.TempDir()
	fileDir := filepath.Join(repoRoot, ".relicta", "governance")

	cfg := config.DefaultPersistenceConfig()
	cfg.Backend = config.BackendPostgres
	cfg.ConnectionString = unreachableGovernancePostgres

	_, err := OpenGovernanceMemoryStore(context.Background(), cfg, repoRoot, fileDir)

	if err == nil {
		t.Fatal("opening an unreachable postgres governance store succeeded: the release " +
			"would be recorded in local JSON and reported as governed")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("the error is %q and does not name the database it could not reach", err)
	}
	if _, statErr := os.Stat(filepath.Join(fileDir, "memory.json")); statErr == nil {
		t.Error("memory.json was created for a backend that failed to open: that is the " +
			"silent fallback to files ADR-013 exists to remove")
	}
}

// The connection string is never echoed, because it routinely comes from ${DATABASE_URL} and
// carries a password.
func TestThePostgresGovernanceErrorNamesTheTargetWithoutItsPassword(t *testing.T) {
	cfg := config.DefaultPersistenceConfig()
	cfg.Backend = config.BackendPostgres
	cfg.ConnectionString = "postgres://relicta:hunter2@127.0.0.1:1/relicta?sslmode=disable"

	_, err := OpenGovernanceMemoryStore(context.Background(), cfg, t.TempDir(), t.TempDir())
	if err == nil {
		t.Fatal("opening an unreachable postgres governance store succeeded")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("the error contains the connection string's password: %v — this message "+
			"reaches logs and CI output", err)
	}
}

// A value the build cannot honor is refused at the resolver, not ignored. "The setting stops
// lying" cannot depend on which caller you are, so this is checked here as well as at load.
func TestAnUnsupportedGovernanceBackendIsRefusedRatherThanTreatedAsFile(t *testing.T) {
	repoRoot := t.TempDir()
	fileDir := filepath.Join(repoRoot, ".relicta", "governance")

	_, err := OpenGovernanceMemoryStore(context.Background(),
		config.PersistenceConfig{Backend: "mysql"}, repoRoot, fileDir)

	if err == nil {
		t.Fatal("backend \"mysql\" resolved to a store: an unknown backend that falls through " +
			"to files is the setting lying again")
	}
	if !strings.Contains(err.Error(), "mysql") {
		t.Errorf("the refusal does not name the offending value: %v", err)
	}
}

// SQLite has to be placed before it can be opened, and guessing the working directory is how
// relicta once littered subdirectories with stray .relicta trees.
func TestTheSQLiteGovernanceBackendRefusesToGuessARepositoryRoot(t *testing.T) {
	_, err := OpenGovernanceMemoryStore(context.Background(), sqliteBackend(), "", t.TempDir())

	if err == nil {
		t.Fatal("the sqlite governance store opened with no repository root: it would have " +
			"placed a database relative to whichever directory the command ran from")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("the error does not tell the operator what to do about it: %v", err)
	}
}

func sqliteBackend() config.PersistenceConfig {
	cfg := config.DefaultPersistenceConfig()
	cfg.Backend = config.BackendSQLite
	return cfg
}

func openGovernance(
	t *testing.T, cfg config.PersistenceConfig, repoRoot, fileDir string,
) GovernanceMemoryStore {
	t.Helper()

	store, err := OpenGovernanceMemoryStore(context.Background(), cfg, repoRoot, fileDir)
	if err != nil {
		t.Fatalf("OpenGovernanceMemoryStore(%q): %v", cfg.Backend, err)
	}
	t.Cleanup(func() {
		if store.Closer != nil {
			_ = store.Closer.Close()
		}
	})
	return store
}

func recordRelease(t *testing.T, store cgpmemory.Store, id string) {
	t.Helper()

	err := store.RecordRelease(context.Background(), &cgpmemory.ReleaseRecord{
		ID:         id,
		Repository: "owner/repo",
		Version:    "1.0.0",
		Actor:      cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"},
		Decision:   cgp.DecisionApproved,
		Outcome:    cgpmemory.OutcomeSuccess,
		ReleasedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
}
