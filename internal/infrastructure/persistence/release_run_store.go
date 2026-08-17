package persistence

// release_run_store.go is the one place `persistence.backend` becomes a repository.
//
// ADR-013 gave one port three adapters and a conformance suite to keep them honest at the
// adapter level. This is the same argument one level up: two places that read the setting
// would be two places that can disagree about what `sqlite` means, and the way that defect
// shows up is an operator who configured a database, watched `relicta plan` report success,
// and found the release history in a JSON file — the failure the ADR was written about. So
// every caller that needs a ports.ReleaseRunRepository asks here, and a backend that cannot
// be opened is an error rather than a quiet return to the files.

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// connectTimeout bounds opening a database and checking its schema.
//
// Every relicta command builds a container, so a database that accepts a TCP connection and
// then says nothing — a firewalled host, a DSN naming an address that routes nowhere — would
// otherwise hang the command instead of failing it. Ten seconds is longer than any healthy
// connection takes and short enough that an operator reads it as an error rather than as
// relicta being slow. Applying migrations is deliberately outside this budget: DDL on a
// populated table is legitimately slower than a connect.
const connectTimeout = 10 * time.Second

// ReleaseRunStore is a resolved repository together with whatever it holds open.
type ReleaseRunStore struct {
	// Repository is the adapter the configuration selected, undecorated. Event publishing
	// is added by each caller that wants it, because the callers differ: the release
	// services and the container's bridge each wrap this in their own
	// EventPublishingRepository, and a store that published on its own would publish twice.
	Repository ports.ReleaseRunRepository

	// Closer releases the connection the backend holds, and is nil for the file backend,
	// which holds none. The caller registers it for shutdown — see the container's
	// registerCloseable — rather than closing it here, because the repository is useless
	// afterwards and the caller decides when that is.
	Closer io.Closer

	// Backend and Location say which store was opened and where, for the log line that
	// tells an operator their configuration took effect. Location never contains
	// credentials; see postgres.Target.
	Backend  config.PersistenceBackend
	Location string
}

// closerFunc adapts a plain shutdown function to io.Closer.
type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// OpenReleaseRunStore resolves persistence.backend into a repository for repoRoot.
//
// repoRoot scopes the store: it is where the file backend's .relicta/releases lives, where
// the SQLite file is created, and the key every PostgreSQL query filters on. An empty root
// is tolerated only by the file backend, whose methods take a path per call and fail with
// their own message when reached; a database has to be *placed* before it can be opened, and
// guessing the working directory is how relicta once littered subdirectories with stray
// .relicta trees.
func OpenReleaseRunStore(
	ctx context.Context, cfg config.PersistenceConfig, repoRoot string,
) (ReleaseRunStore, error) {
	// An unset backend is the default one. The loader fills the field in, so "" reaches
	// here only from a Config assembled in code — a test, an embedder — and means "not
	// configured" rather than a value to refuse.
	if cfg.Backend == "" {
		cfg.Backend = config.BackendFile
	}

	// The same rule the load path checks, applied again here rather than trusted. This
	// function is reachable from a container built in code, where nothing validated the
	// configuration, and "the setting stops lying" cannot depend on which caller you are.
	if err := cfg.Validate(); err != nil {
		return ReleaseRunStore{}, fmt.Errorf("persistence: %w", err)
	}

	switch cfg.Backend {
	case config.BackendFile:
		// Unchanged and deliberately identical to what the container constructed before
		// any of this existed: the same adapter, no path from the configuration, no
		// connection. ADR-013 flips the default on evidence, in its own change, so the
		// default backend must behave exactly as it did.
		return ReleaseRunStore{
			Repository: adapters.NewFileReleaseRunRepository(),
			Backend:    config.BackendFile,
			Location:   repoRoot,
		}, nil

	case config.BackendSQLite:
		return openSQLite(ctx, repoRoot)

	case config.BackendPostgres:
		return openPostgres(ctx, cfg)

	default:
		// Unreachable while cfg.Validate covers the same set, and kept because the two
		// would drift silently otherwise: a backend added to the config package and not
		// here must refuse to open, not fall through to files.
		return ReleaseRunStore{}, fmt.Errorf(
			"persistence: unsupported backend %q for the release run store", cfg.Backend)
	}
}

// openSQLite opens the repository's own database file.
//
// The path is sqlite.DefaultPath(repoRoot) — .relicta/relicta.db — and not
// persistence.file_path. That setting defaults to `.relicta/events` and names a *directory*,
// from the event-store design where every event was a file; pointing it at a database would
// be a category error that surfaces the first time something tries to list it. There is also
// nothing to gain: SQLite writes -wal and -shm files beside the database, so it has to live
// in a directory relicta owns and the repository ignores, which .relicta already is.
func openSQLite(ctx context.Context, repoRoot string) (ReleaseRunStore, error) {
	if repoRoot == "" {
		return ReleaseRunStore{}, fmt.Errorf(
			"persistence: the sqlite backend needs a repository root to place its database; " +
				"relicta could not resolve one, so run it inside a git repository")
	}

	path := sqlite.DefaultPath(repoRoot)

	openCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	// Open migrates. ADR-013's "migration is explicit" protects an operator's *data* —
	// `relicta db import` never moves an audit trail behind their back — not the schema of
	// a file relicta created itself, and persistence.migration_mode therefore does not
	// apply here. Honoring it would also be unimplementable: `relicta db migrate` speaks
	// only postgres, so `manual` would leave a local database with no way to be migrated.
	store, err := sqlite.Open(openCtx, path)
	if err != nil {
		return ReleaseRunStore{}, fmt.Errorf("persistence: opening the sqlite release store: %w", err)
	}

	return ReleaseRunStore{
		Repository: store,
		Closer:     store,
		Backend:    config.BackendSQLite,
		Location:   path,
	}, nil
}

// openPostgres connects the shared backend and makes sure it can serve a release run.
//
// Both failures here used to be invisible. An unreachable database and an unmigrated one both
// ended with relicta writing JSON files and reporting success, which is the defect ADR-013
// exists to remove; each is now the command's error, named, before anything is written.
func openPostgres(ctx context.Context, cfg config.PersistenceConfig) (ReleaseRunStore, error) {
	connStr := config.ExpandEnvVars(cfg.ConnectionString)
	target := postgres.Target(connStr)

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	// NewPool pings, so this fails here rather than at the first save.
	pool, err := postgres.NewPool(connectCtx, connStr, cfg.PoolSize)
	if err != nil {
		return ReleaseRunStore{}, fmt.Errorf(
			"persistence: connecting to the postgres release store at %s: %w", target, err)
	}
	closer := closerFunc(func() error {
		pool.Close()
		return nil
	})

	switch cfg.MigrationMode {
	case config.MigrationModeAuto:
		// On the caller's context, not connectCtx: DDL against a populated table can
		// legitimately outlast a connect budget, and a migration killed halfway is a worse
		// outcome than a slow command. Each migration is its own transaction, so an
		// interrupted run leaves the schema consistent either way.
		if _, err := postgres.NewMigrator(pool).Up(ctx); err != nil {
			pool.Close()
			return ReleaseRunStore{}, fmt.Errorf(
				"persistence: migrating the postgres release store at %s: %w", target, err)
		}
	default:
		// manual, which is the default: relicta does not change the schema of a database
		// an operator provisioned. It does have to refuse to run against one that cannot
		// hold a release run, and say which command fixes that.
		if err := postgres.VerifyReleaseRunSchema(connectCtx, pool); err != nil {
			pool.Close()
			return ReleaseRunStore{}, fmt.Errorf(
				"persistence: the postgres release store at %s is not ready: %w", target, err)
		}
	}

	return ReleaseRunStore{
		Repository: postgres.NewReleaseRunRepository(pool),
		Closer:     closer,
		Backend:    config.BackendPostgres,
		Location:   target,
	}, nil
}
