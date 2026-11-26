package persistence

import (
	"context"
	"fmt"
	"io"

	"github.com/relicta-tech/relicta/v5/internal/cgp/identity"
	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/postgres"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/sqlite"
)

type IdentityRegistryStore struct {
	Store    identity.RegistryStore
	Closer   io.Closer
	Backend  config.PersistenceBackend
	Location string
}

// OpenIdentityRegistryStore resolves organization identity persistence using the same backend
// selected for repository history. filePath is the configured JSON directory used only by file.
func OpenIdentityRegistryStore(ctx context.Context, cfg config.PersistenceConfig, repoRoot, filePath string) (IdentityRegistryStore, error) {
	if cfg.Backend == "" {
		cfg.Backend = config.BackendFile
	}
	if err := cfg.Validate(); err != nil {
		return IdentityRegistryStore{}, fmt.Errorf("persistence: %w", err)
	}
	switch cfg.Backend {
	case config.BackendFile:
		store, err := identity.NewFileStore(filePath)
		if err != nil {
			return IdentityRegistryStore{}, fmt.Errorf("persistence: opening file identity registry: %w", err)
		}
		return IdentityRegistryStore{Store: store, Backend: config.BackendFile, Location: filePath}, nil
	case config.BackendSQLite:
		if repoRoot == "" {
			return IdentityRegistryStore{}, fmt.Errorf("persistence: sqlite identity registry needs a repository root")
		}
		path := sqlite.DefaultPath(repoRoot)
		openCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		store, err := sqlite.OpenIdentityStore(openCtx, path)
		if err != nil {
			return IdentityRegistryStore{}, fmt.Errorf("persistence: opening sqlite identity registry: %w", err)
		}
		return IdentityRegistryStore{Store: store, Closer: store, Backend: config.BackendSQLite, Location: path}, nil
	case config.BackendPostgres:
		connStr := config.ExpandEnvVars(cfg.ConnectionString)
		target := postgres.Target(connStr)
		connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		pool, err := postgres.NewPool(connectCtx, connStr, cfg.PoolSize)
		if err != nil {
			return IdentityRegistryStore{}, fmt.Errorf("persistence: connecting to postgres identity registry at %s: %w", target, err)
		}
		if cfg.MigrationMode == config.MigrationModeAuto {
			if _, err := postgres.NewMigrator(pool).Up(ctx); err != nil {
				pool.Close()
				return IdentityRegistryStore{}, fmt.Errorf("persistence: migrating postgres identity registry at %s: %w", target, err)
			}
		} else if err := postgres.VerifyIdentitySchema(connectCtx, pool); err != nil {
			pool.Close()
			return IdentityRegistryStore{}, fmt.Errorf("persistence: postgres identity registry at %s is not ready: %w", target, err)
		}
		return IdentityRegistryStore{Store: postgres.NewIdentityStore(pool), Closer: closerFunc(func() error { pool.Close(); return nil }), Backend: config.BackendPostgres, Location: target}, nil
	default:
		return IdentityRegistryStore{}, fmt.Errorf("persistence: unsupported backend %q for identity registry", cfg.Backend)
	}
}
