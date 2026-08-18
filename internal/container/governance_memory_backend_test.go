package container

// governance_memory_backend_test.go covers what persistence.backend now decides about the
// governance record in a real container.
//
// The setting selected the release run store and nothing else, which is half a wiring and
// worse than none: `relicta plan` would have saved its run to SQLite while the outcome
// tracker, the governance service, `relicta history` and the reports all kept writing and
// reading .relicta/governance/memory.json. So these tests assert where the record went, and
// that the two things inside the container that hold a governance store hold the same one.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The default is the whole compatibility promise: a container built with no persistence
// section must record governance exactly where it recorded it before the setting was wired.
func TestAContainerWithNoPersistenceSectionStillWritesGovernanceMemoryToJSON(t *testing.T) {
	repo := gitRepoAt(t, "governance-default")
	app := initializedApp(t, config.DefaultConfig(), repo)

	recordGovernanceRelease(t, app.MemoryStore(), "rel-1")

	memoryJSON := filepath.Join(repo, ".relicta", "governance", "memory.json")
	if _, err := os.Stat(memoryJSON); err != nil {
		t.Errorf("no %s after recording with no persistence section: %v — wiring the setting "+
			"moved the default store, which orphans every existing user's audit trail on "+
			"upgrade", memoryJSON, err)
	}
}

func TestAContainerConfiguredForSQLiteRecordsGovernanceInTheDatabaseAndNotInJSON(t *testing.T) {
	repo := gitRepoAt(t, "governance-sqlite")
	app := initializedApp(t, configWithBackend(config.BackendSQLite, ""), repo)

	recordGovernanceRelease(t, app.MemoryStore(), "rel-1")

	if _, err := os.Stat(filepath.Join(repo, ".relicta", "relicta.db")); err != nil {
		t.Errorf("no .relicta/relicta.db after recording with `backend: sqlite`: %v", err)
	}
	memoryJSON := filepath.Join(repo, ".relicta", "governance", "memory.json")
	if _, err := os.Stat(memoryJSON); err == nil {
		t.Errorf("%s was written as well as the database: two copies of the governance record "+
			"that can disagree is the failure ADR-013 names as the worst available one",
			memoryJSON)
	}

	history, err := app.MemoryStore().GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil || len(history) != 1 {
		t.Fatalf("GetReleaseHistory returned %d records, %v: a record that cannot be read "+
			"back is a lost audit trail", len(history), err)
	}
}

// The one-store property, at the two seams inside the container that hold a governance store.
//
// The outcome tracker records what a release did; the governance service records the decision
// behind it and reads the history that scores the next one. The service used to open its own
// FileStore, so under `backend: sqlite` it would have kept writing JSON beside a database, and
// under the file backend the two instances each cached the whole memory.json and rewrote all
// of it — whichever wrote last erasing the other.
func TestTheOutcomeTrackerAndTheGovernanceServiceShareOneStore(t *testing.T) {
	repo := gitRepoAt(t, "governance-one-store")

	// governance.enabled is false in the defaults — `relicta init` writes true — and without
	// it the container builds no governance service for this test to say anything about.
	cfg := configWithBackend(config.BackendSQLite, "")
	cfg.Governance.Enabled = true
	app := initializedApp(t, cfg, repo)

	if app.GovernanceService() == nil {
		t.Fatal("no governance service: this test cannot say anything without one")
	}

	ctx := context.Background()
	err := app.GovernanceService().RecordReleaseOutcome(ctx, governance.RecordOutcomeInput{
		ReleaseID:  "run-through-the-service",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Actor:      cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"},
		Decision:   cgp.DecisionApproved,
		Outcome:    cgpmemory.OutcomeSuccess,
	})
	if err != nil {
		t.Fatalf("RecordReleaseOutcome: %v", err)
	}

	// Read back through the container's store, which is what the outcome tracker writes to
	// and what `relicta history` and the reports read. If the service opened one of its own,
	// this finds nothing.
	history, err := app.MemoryStore().GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(history) != 1 || history[0].ID != "run-through-the-service" {
		t.Errorf("the container's store holds %d records after the governance service "+
			"recorded one: the service is writing to a store of its own, so `relicta "+
			"publish` records a release the reports cannot find", len(history))
	}

	// And the service's write went into the database rather than beside it.
	memoryJSON := filepath.Join(repo, ".relicta", "governance", "memory.json")
	if _, err := os.Stat(memoryJSON); err == nil {
		t.Errorf("%s exists after a governance service write under `backend: sqlite`: the "+
			"service resolved a backend of its own", memoryJSON)
	}
}

// The defect ADR-013 was written about. A user who asked for postgres and got local JSON is
// the specific outcome that must be impossible — including when only the governance half is
// unreachable.
func TestAContainerRefusesToStartWhenTheGovernanceDatabaseIsUnreachable(t *testing.T) {
	repo := gitRepoAt(t, "governance-unreachable")

	app, err := NewForRepo(configWithBackend(config.BackendPostgres, unreachablePostgres), repo)
	if err != nil {
		t.Fatalf("NewForRepo: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	initErr := app.Initialize(context.Background())

	if initErr == nil {
		t.Fatal("a container whose governance database is unreachable initialized anyway: the " +
			"release would be recorded in local JSON and reported as governed")
	}
	if !strings.Contains(initErr.Error(), "127.0.0.1:1") {
		t.Errorf("the error is %q and does not name the database it could not reach", initErr)
	}
	// Named as the governance store, not merely as "a database". The release run store fails
	// on the same DSN, so an error that did not say which one would pass this test while the
	// governance half quietly warned and carried on into the file store.
	if !strings.Contains(initErr.Error(), "governance memory store") {
		t.Errorf("the error is %q and does not say that it is the governance store that could "+
			"not be opened: the release run store fails on the same connection string, so a "+
			"governance failure that only warns is invisible here", initErr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".relicta", "governance", "memory.json")); err == nil {
		t.Error("a governance memory.json was created for a container that failed to reach " +
			"its configured database: that is the silent fallback to files")
	}
}

// The connection is registered for shutdown rather than left to process exit, and this is how
// that is observable: a closed database refuses work. Without the registration a long-lived
// process — `relicta serve`, the MCP server — accumulates one handle per container.
func TestClosingTheContainerReleasesTheGovernanceStoreConnection(t *testing.T) {
	repo := gitRepoAt(t, "governance-close")
	app := initializedApp(t, configWithBackend(config.BackendSQLite, ""), repo)
	store := app.MemoryStore()

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := store.RecordRelease(context.Background(), governanceRelease("rel-after-close"))
	if err == nil {
		t.Error("recording succeeded after the container was closed, so the governance " +
			"database connection was never registered for shutdown and outlives the " +
			"container that opened it")
	}
}

func recordGovernanceRelease(t *testing.T, store cgpmemory.Store, id string) {
	t.Helper()

	if store == nil {
		t.Fatal("the container exposes no governance memory store: memory_enabled defaults " +
			"to true, so this is the wiring being absent rather than off")
	}
	if err := store.RecordRelease(context.Background(), governanceRelease(id)); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
}

func governanceRelease(id string) *cgpmemory.ReleaseRecord {
	return &cgpmemory.ReleaseRecord{
		ID:         id,
		Repository: "owner/repo",
		Version:    "1.0.0",
		Actor:      cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"},
		Decision:   cgp.DecisionApproved,
		Outcome:    cgpmemory.OutcomeSuccess,
		ReleasedAt: time.Now(),
	}
}
