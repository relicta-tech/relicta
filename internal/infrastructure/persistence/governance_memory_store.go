package persistence

// governance_memory_store.go is the one place `persistence.backend` becomes a governance
// memory store, exactly as release_run_store.go is the one place it becomes a repository.
//
// ADR-013 names the governance record part of the system of record alongside the release run,
// so the setting has to select both or it still lies — half of it, which is worse than none of
// it. A build where `plan` saved its run to SQLite while the outcome tracker, `relicta
// history`, the DORA and SOC 2 reports and the deployment gate all kept writing and reading
// `.relicta/governance/memory.json` would look configured and would be split down the middle.
//
// There is one resolver for the same reason there is one for runs: two places reading the
// setting are two answers waiting to disagree, and the way that shows up is `relicta publish`
// recording an outcome in a database while `relicta history` reports no release history at
// all. Every caller that needs a cgpmemory.Store asks here, and a backend that cannot be
// opened is an error rather than a quiet return to the files.

import (
	"context"
	"fmt"
	"io"

	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite"
)

// GovernanceMemoryStore is a resolved governance store together with whatever it holds open.
type GovernanceMemoryStore struct {
	// Store is the adapter the configuration selected, undecorated. The outcome tracker,
	// the governance service and the reporting commands each wrap or read it as they
	// already do; a store that decorated itself here would be decorated twice by the
	// container, which builds the tracker around the same value it hands the service.
	Store cgpmemory.Store

	// Closer releases the connection the backend holds, and is nil for the file backend,
	// which holds none. The caller registers it for shutdown — see the container's
	// registerCloseable — rather than closing it here, because the store is useless
	// afterwards and the caller decides when that is.
	Closer io.Closer

	// Backend and Location say which store was opened and where, for the log line that
	// tells an operator their configuration took effect. Location never contains
	// credentials; see postgres.Target.
	Backend  config.PersistenceBackend
	Location string
}

// OpenGovernanceMemoryStore resolves persistence.backend into a governance memory store.
//
// repoRoot scopes the database backends: it is where the SQLite file is placed, and for
// PostgreSQL nothing at all — the governance tables are keyed by the repository's governance
// identity rather than its path, because a shared database serves checkouts that do not agree
// on where they live. fileDir is where the file backend's memory.json lives, and is ignored by
// the other two.
//
// The two paths are separate parameters rather than one derived from the other because
// different rules resolve them. The file store's location is `governance.memory_path`, which
// an operator may set and which application/governance already resolves for every caller;
// infrastructure has no business reaching up a layer to ask. Passing it in keeps the
// *selection* here and the *path* where it already was — one answer each, rather than one
// answer with two spellings.
func OpenGovernanceMemoryStore(
	ctx context.Context, cfg config.PersistenceConfig, repoRoot, fileDir string,
) (GovernanceMemoryStore, error) {
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
		return GovernanceMemoryStore{}, fmt.Errorf("persistence: %w", err)
	}

	switch cfg.Backend {
	case config.BackendFile:
		return openFileGovernanceMemory(fileDir)

	case config.BackendSQLite:
		return openSQLiteGovernanceMemory(ctx, repoRoot)

	case config.BackendPostgres:
		return openPostgresGovernanceMemory(ctx, cfg)

	default:
		// Unreachable while cfg.Validate covers the same set, and kept because the two
		// would drift silently otherwise: a backend added to the config package and not
		// here must refuse to open, not fall through to files.
		return GovernanceMemoryStore{}, fmt.Errorf(
			"persistence: unsupported backend %q for the governance memory store", cfg.Backend)
	}
}

// openFileGovernanceMemory constructs the store the default backend has always constructed.
//
// Deliberately identical to the call every site made before this resolver existed —
// cgpmemory.NewFileStore on the directory holding memory.json, nothing read from the
// persistence section, no connection. ADR-013 flips the default on evidence, in its own
// change, so the default backend must behave exactly as it did.
func openFileGovernanceMemory(fileDir string) (GovernanceMemoryStore, error) {
	if fileDir == "" {
		return GovernanceMemoryStore{}, fmt.Errorf(
			"persistence: the file backend needs a directory for its governance memory store")
	}

	store, err := cgpmemory.NewFileStore(fileDir)
	if err != nil {
		return GovernanceMemoryStore{}, fmt.Errorf(
			"persistence: opening the governance memory store in %s: %w", fileDir, err)
	}

	return GovernanceMemoryStore{
		Store:    store,
		Backend:  config.BackendFile,
		Location: fileDir,
	}, nil
}

// openSQLiteGovernanceMemory opens the repository's own database file.
//
// The same file the release runs live in — sqlite.DefaultPath(repoRoot) — because ADR-013 puts
// one backend behind the setting rather than one per store: a run and the governance record it
// produces belong somewhere they can eventually be written in one transaction, and two files
// cannot be.
//
// It is a second *sql.DB over that file, not a shared handle, and that is a cost this accepts
// rather than a detail it missed. The two stores are separately reachable — `relicta history`
// and `relicta report` open governance memory and no run store at all — so a shared handle
// would have to be owned by whichever resolver ran first and kept alive for a caller that may
// never appear. Two handles are safe here for the reasons openDatabase sets its pragmas: WAL,
// so readers and the writer do not exclude each other, and a busy timeout, so the two wait
// rather than fail. What they cannot do is share a transaction, which is why ADR-013's
// atomicity is still described as available rather than taken.
func openSQLiteGovernanceMemory(ctx context.Context, repoRoot string) (GovernanceMemoryStore, error) {
	if repoRoot == "" {
		return GovernanceMemoryStore{}, fmt.Errorf(
			"persistence: the sqlite backend needs a repository root to place its database; " +
				"relicta could not resolve one, so run it inside a git repository")
	}

	path := sqlite.DefaultPath(repoRoot)

	openCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	// Open migrates, for the reason openSQLite gives: "migration is explicit" protects an
	// operator's data, not the schema of a file relicta created itself.
	store, err := sqlite.OpenMemoryStore(openCtx, path)
	if err != nil {
		return GovernanceMemoryStore{}, fmt.Errorf(
			"persistence: opening the sqlite governance memory store: %w", err)
	}

	return GovernanceMemoryStore{
		Store:    store,
		Closer:   store,
		Backend:  config.BackendSQLite,
		Location: path,
	}, nil
}

// openPostgresGovernanceMemory connects the shared backend and makes sure it can serve a
// governance record.
//
// Both failures here used to be invisible, in the way ADR-013 was written about: an
// unreachable database and an unmigrated one each ended with relicta writing memory.json and
// reporting success. Both are now the command's error, named, before anything is recorded.
func openPostgresGovernanceMemory(
	ctx context.Context, cfg config.PersistenceConfig,
) (GovernanceMemoryStore, error) {
	connStr := config.ExpandEnvVars(cfg.ConnectionString)
	target := postgres.Target(connStr)

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	// NewPool pings, so this fails here rather than at the first record.
	pool, err := postgres.NewPool(connectCtx, connStr, cfg.PoolSize)
	if err != nil {
		return GovernanceMemoryStore{}, fmt.Errorf(
			"persistence: connecting to the postgres governance memory store at %s: %w", target, err)
	}
	closer := closerFunc(func() error {
		pool.Close()
		return nil
	})

	switch cfg.MigrationMode {
	case config.MigrationModeAuto:
		// On the caller's context, not connectCtx: DDL against a populated table can
		// legitimately outlast a connect budget, and a migration killed halfway is a
		// worse outcome than a slow command. Each migration is its own transaction, so an
		// interrupted run leaves the schema consistent either way.
		if _, err := postgres.NewMigrator(pool).Up(ctx); err != nil {
			pool.Close()
			return GovernanceMemoryStore{}, fmt.Errorf(
				"persistence: migrating the postgres governance memory store at %s: %w", target, err)
		}
	default:
		// manual, which is the default: relicta does not change the schema of a database
		// an operator provisioned. It does have to refuse to run against one that cannot
		// hold a governance record, and say which command fixes that.
		if err := postgres.VerifyGovernanceMemorySchema(connectCtx, pool); err != nil {
			pool.Close()
			return GovernanceMemoryStore{}, fmt.Errorf(
				"persistence: the postgres governance memory store at %s is not ready: %w",
				target, err)
		}
	}

	return GovernanceMemoryStore{
		Store:    postgres.NewGovernanceMemoryStore(pool),
		Closer:   closer,
		Backend:  config.BackendPostgres,
		Location: target,
	}, nil
}
