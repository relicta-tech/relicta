package persistence_test

// release_run_store_test.go covers the one place persistence.backend becomes a repository.
//
// The case that matters most is the boring one: with no persistence section, the resolver
// must return exactly the adapter the container built before any of this existed. ADR-013
// defers flipping the default deliberately, so a change that improved the default would be a
// change nobody asked for, applied to every existing user's audit trail.
//
// The rest are the ways the setting used to lie. An unreachable database and an unknown
// backend both ended with relicta writing JSON files and reporting success; here each is an
// error, and no repository comes back that a caller could accidentally use.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports/conformance"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
)

// unreachablePostgres names a port nothing listens on, so connecting fails immediately with
// the operating system's refusal rather than waiting for a timeout.
const unreachablePostgres = "postgres://relicta@127.0.0.1:1/relicta?sslmode=disable"

func TestTheDefaultConfigurationResolvesToTheFileStore(t *testing.T) {
	root := t.TempDir()

	store, err := persistence.OpenReleaseRunStore(
		context.Background(), config.DefaultPersistenceConfig(), root)
	if err != nil {
		t.Fatalf("OpenReleaseRunStore with the default configuration: %v", err)
	}

	if _, ok := store.Repository.(*adapters.FileReleaseRunRepository); !ok {
		t.Errorf("the default backend resolved to %T, want the file adapter: ADR-013 keeps "+
			"`file` the default until parity is proven, so wiring the setting must not "+
			"migrate anybody's release history", store.Repository)
	}
	if store.Closer != nil {
		t.Error("the file backend reported something to close; it holds no connection, and a " +
			"closer here would be closed on every command for no reason")
	}
	if entries, err := os.ReadDir(root); err == nil && len(entries) != 0 {
		t.Errorf("opening the default store created %d entries in the repository; it must "+
			"touch nothing until something is saved", len(entries))
	}
}

// An empty backend is not a choice anybody made — it is a Config built in code, and the
// loader's default for the field is `file`. Refusing it would break every embedder that
// assembles a Config without a persistence section.
func TestAnUnsetBackendResolvesToTheFileStore(t *testing.T) {
	store, err := persistence.OpenReleaseRunStore(
		context.Background(), config.PersistenceConfig{}, t.TempDir())
	if err != nil {
		t.Fatalf("OpenReleaseRunStore with an unset backend: %v", err)
	}
	if _, ok := store.Repository.(*adapters.FileReleaseRunRepository); !ok {
		t.Errorf("an unset backend resolved to %T, want the file adapter", store.Repository)
	}
}

// The typo case, and the reason ADR-013 says a value the build cannot honor is refused rather
// than ignored: `backend: postgress` used to read as "not postgres", so relicta wrote the
// team's audit trail to local files while they believed it was in their database.
func TestAnUnknownBackendIsRefusedRatherThanTreatedAsFiles(t *testing.T) {
	store, err := persistence.OpenReleaseRunStore(context.Background(),
		config.PersistenceConfig{Backend: config.PersistenceBackend("postgress")}, t.TempDir())

	if err == nil {
		t.Fatal("a misspelled backend was accepted; the operator would get local files and " +
			"a command that reports success")
	}
	if store.Repository != nil {
		t.Error("a refused backend still returned a repository, which a caller could use " +
			"without ever seeing the error")
	}
}

func TestTheSQLiteBackendPutsItsDatabaseInTheRepository(t *testing.T) {
	root := t.TempDir()

	store := openSQLite(t, root)

	wantPath := filepath.Join(root, ".relicta", "relicta.db")
	if store.Location != wantPath {
		t.Errorf("the sqlite store opened %q, want %q: persistence.file_path names a "+
			"directory from the event-store design and cannot address a database file",
			store.Location, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("no database at %s after opening the sqlite backend: %v", wantPath, err)
	}
	if store.Closer == nil {
		t.Error("the sqlite store reported nothing to close; its connection pool would " +
			"outlive every command that opened it")
	}
}

// The schema has to be there without a second command. `relicta db migrate` speaks only
// postgres, so a sqlite store that waited to be migrated could never be.
func TestTheSQLiteBackendMigratesItselfOnOpen(t *testing.T) {
	root := t.TempDir()
	store := openSQLite(t, root)

	run := domainrelease.NewReleaseRunForTest("run-abc123", "main", root)
	if err := store.Repository.Save(context.Background(), run); err != nil {
		t.Fatalf("saving into a freshly opened sqlite store: %v — a store whose schema is "+
			"absent is unusable, and no command creates one for sqlite", err)
	}

	loaded, err := store.Repository.Load(context.Background(), run.ID())
	if err != nil || loaded == nil {
		t.Fatalf("Load after Save through the resolved store: %v", err)
	}
}

// A database has to be placed somewhere before it can be opened, and guessing the working
// directory is how relicta once created stray .relicta trees in whatever subdirectory a
// command ran from.
func TestTheSQLiteBackendRefusesToGuessWhereTheDatabaseGoes(t *testing.T) {
	store, err := persistence.OpenReleaseRunStore(context.Background(),
		config.PersistenceConfig{Backend: config.BackendSQLite}, "")

	if err == nil {
		t.Fatal("the sqlite backend opened a database with no repository root; it would " +
			"land in whatever directory the command happened to run from")
	}
	if store.Repository != nil {
		t.Error("a store that could not be placed still returned a repository")
	}
}

// The defect ADR-013 was written about, as a test: a configured database that cannot be
// reached must stop the command, not silently become the file backend.
func TestAnUnreachablePostgresIsReportedRatherThanReplacedByFiles(t *testing.T) {
	store, err := persistence.OpenReleaseRunStore(context.Background(), config.PersistenceConfig{
		Backend:          config.BackendPostgres,
		ConnectionString: unreachablePostgres,
		PoolSize:         2,
		MigrationMode:    config.MigrationModeManual,
	}, t.TempDir())

	if err == nil {
		t.Fatal("an unreachable postgres was accepted: this is the defect ADR-013 exists " +
			"for — the team believes its release history is in the database and it is in " +
			"each developer's working copy")
	}
	if store.Repository != nil {
		t.Fatal("a postgres store that could not be reached still returned a repository")
	}
	if _, ok := store.Repository.(*adapters.FileReleaseRunRepository); ok {
		t.Fatal("connecting to postgres failed and the file adapter came back instead")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("the error is %q, and does not name the database it could not reach: an "+
			"operator cannot tell a wrong host from a stopped server", err)
	}
}

// A postgres config missing its connection string is refused before anything connects, so the
// error names the setting rather than a driver's opinion about an empty DSN.
func TestPostgresWithoutAConnectionStringIsRefused(t *testing.T) {
	_, err := persistence.OpenReleaseRunStore(context.Background(), config.PersistenceConfig{
		Backend:       config.BackendPostgres,
		PoolSize:      2,
		MigrationMode: config.MigrationModeManual,
	}, t.TempDir())

	if err == nil {
		t.Fatal("postgres was selected with no connection string and the store opened anyway")
	}
	if !strings.Contains(err.Error(), "connection_string") {
		t.Errorf("the error is %q, and does not name connection_string", err)
	}
}

// The contract, through the selection path rather than the adapter's own constructor.
//
// The sqlite package already runs this suite against sqlite.Open. Running it again here is
// not duplication: what it pins is that the resolver hands back the store itself, undecorated
// and fully functional, rather than a partially wired one — the suite is the specification,
// and the composition root is where a store stops being an object and starts being what
// `relicta status` reads.
func TestTheResolvedSQLiteStoreSatisfiesTheContract(t *testing.T) {
	conformance.Run(t, func(t *testing.T) (ports.ReleaseRunRepository, string) {
		root := t.TempDir()
		return openSQLite(t, root).Repository, root
	})
}

// openSQLite resolves a sqlite store for root and closes it when the test ends.
func openSQLite(t *testing.T, root string) persistence.ReleaseRunStore {
	t.Helper()

	store, err := persistence.OpenReleaseRunStore(context.Background(),
		config.PersistenceConfig{Backend: config.BackendSQLite}, root)
	if err != nil {
		t.Fatalf("OpenReleaseRunStore for sqlite in %s: %v", root, err)
	}
	if store.Closer != nil {
		t.Cleanup(func() {
			if err := store.Closer.Close(); err != nil {
				t.Errorf("closing the sqlite store: %v", err)
			}
		})
	}
	return store
}
