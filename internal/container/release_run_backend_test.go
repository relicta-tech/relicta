package container

// release_run_backend_test.go covers what persistence.backend now decides in a real container.
//
// The setting had never been read. A team that configured PostgreSQL for shared governance
// state got JSON files in each developer's working copy, and `relicta plan` reported "Release
// plan saved" — which is why these tests assert where the bytes went rather than that a call
// returned nil.
//
// Two properties are load bearing here. The default must be untouched, because ADR-013 flips
// it on evidence and in its own change; and both seams the CLI reaches the aggregate through —
// app.ReleaseRepository() and ReleaseServices().Repository — must land in the same store,
// because a plan written to a database and a cancel read from a file is the same defect one
// layer down.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// unreachablePostgres names a port nothing listens on, so the connection is refused at once
// instead of waiting out a timeout.
const unreachablePostgres = "postgres://relicta@127.0.0.1:1/relicta?sslmode=disable"

// The default is the whole compatibility promise: a container built with no persistence
// section must write what it wrote before the setting was wired.
func TestAContainerWithNoPersistenceSectionStillWritesJSONRuns(t *testing.T) {
	repo := gitRepoAt(t, "default-backend")
	app := initializedApp(t, config.DefaultConfig(), repo)

	run := runIn(t, repo)
	if err := app.ReleaseRepository().Save(context.Background(), run); err != nil {
		t.Fatalf("Save through the default backend: %v", err)
	}

	runs, err := filepath.Glob(filepath.Join(repo, ".relicta", "releases", "run-*.json"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(runs) == 0 {
		t.Error("no .relicta/releases/run-*.json after saving with no persistence section: " +
			"wiring the setting changed the default store, which migrates every existing " +
			"user's release history on upgrade")
	}
}

func TestAContainerConfiguredForSQLiteWritesToTheDatabaseAndNotToJSON(t *testing.T) {
	repo := gitRepoAt(t, "sqlite-backend")
	app := initializedApp(t, configWithBackend(config.BackendSQLite, ""), repo)

	run := runIn(t, repo)
	if err := app.ReleaseRepository().Save(context.Background(), run); err != nil {
		t.Fatalf("Save through the sqlite backend: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, ".relicta", "relicta.db")); err != nil {
		t.Errorf("no .relicta/relicta.db after saving with `backend: sqlite`: %v", err)
	}

	runs, err := filepath.Glob(filepath.Join(repo, ".relicta", "releases", "run-*.json"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("the run was written to %v as well as the database: two copies of the "+
			"governance record that can disagree is the failure ADR-013 names as the worst "+
			"available one", runs)
	}

	loaded, err := app.ReleaseRepository().FindByID(context.Background(), run.ID())
	if err != nil || loaded == nil {
		t.Fatalf("FindByID from the sqlite backend: %v — a store that cannot be read back "+
			"is a lost release", err)
	}
}

// The one-store property, at the two seams the CLI actually uses. `relicta plan` saves through
// the release services and `relicta cancel` reads through app.ReleaseRepository(); if those
// resolve the backend separately, one of them reads an empty store.
func TestTheBridgeAndTheReleaseServicesReadOneStore(t *testing.T) {
	repo := gitRepoAt(t, "one-store")
	app := initializedApp(t, configWithBackend(config.BackendSQLite, ""), repo)

	ctx := context.Background()
	if err := app.InitReleaseServices(ctx, repo); err != nil {
		t.Fatalf("InitReleaseServices: %v", err)
	}

	run := runIn(t, repo)
	if err := app.ReleaseServices().Repository.Save(ctx, run); err != nil {
		t.Fatalf("Save through the release services: %v", err)
	}

	loaded, err := app.ReleaseRepository().FindByID(ctx, run.ID())
	if err != nil || loaded == nil {
		t.Fatalf("the bridge could not find a run the release services saved: %v — plan "+
			"would write to the database and cancel would report no release found", err)
	}
}

// The defect ADR-013 was written about. A user who asked for postgres and got local files is
// the specific outcome that must be impossible.
func TestAContainerRefusesToStartWhenPostgresIsUnreachable(t *testing.T) {
	repo := gitRepoAt(t, "unreachable-postgres")

	app, err := NewForRepo(configWithBackend(config.BackendPostgres, unreachablePostgres), repo)
	if err != nil {
		t.Fatalf("NewForRepo: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	initErr := app.Initialize(context.Background())

	if initErr == nil {
		t.Fatal("a container whose database is unreachable initialized anyway: the release " +
			"would be written to local files and reported as saved")
	}
	if !strings.Contains(initErr.Error(), "127.0.0.1:1") {
		t.Errorf("the error is %q and does not name the database it could not reach", initErr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".relicta", "releases")); err == nil {
		t.Error("a release directory was created for a container that failed to reach its " +
			"configured database")
	}
}

// The connection is registered for shutdown rather than left to the process exit, and this is
// how that is observable: a closed pool refuses work. Without the registration a long-lived
// process — `relicta serve`, the MCP server — would accumulate one pool per container.
func TestClosingTheContainerReleasesTheSQLiteConnection(t *testing.T) {
	repo := gitRepoAt(t, "sqlite-close")
	app := initializedApp(t, configWithBackend(config.BackendSQLite, ""), repo)

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := app.ReleaseRepository().Save(context.Background(), runIn(t, repo))
	if err == nil {
		t.Error("saving succeeded after the container was closed, so the database connection " +
			"was never registered for shutdown and outlives the container that opened it")
	}
}

// configWithBackend returns the default configuration with one persistence section replaced.
func configWithBackend(backend config.PersistenceBackend, connStr string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Persistence.Backend = backend
	cfg.Persistence.ConnectionString = connStr
	return cfg
}

// initializedApp builds and initializes a container for one repository, closed by the test.
func initializedApp(t *testing.T, cfg *config.Config, repo string) *App {
	t.Helper()

	app, err := NewForRepo(cfg, repo)
	if err != nil {
		t.Fatalf("NewForRepo: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	if err := app.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return app
}
