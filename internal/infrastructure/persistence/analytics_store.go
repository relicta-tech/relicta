package persistence

import (
	"context"
	"fmt"
	"io"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/postgres"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/sqlite"
)

// AnalyticsStore is an analytics adapter resolved from persistence.backend.
type AnalyticsStore struct {
	Store    analytics.Store
	Closer   io.Closer
	Backend  config.PersistenceBackend
	Location string
}

// OpenAnalyticsStore resolves the analytics adapter without silently changing backends.
func OpenAnalyticsStore(ctx context.Context, cfg config.PersistenceConfig, repoRoot, filePath string) (AnalyticsStore, error) {
	if cfg.Backend == "" {
		cfg.Backend = config.BackendFile
	}
	if err := cfg.Validate(); err != nil {
		return AnalyticsStore{}, fmt.Errorf("persistence: %w", err)
	}
	switch cfg.Backend {
	case config.BackendFile:
		store, err := analytics.NewFileStore(filePath)
		if err != nil {
			return AnalyticsStore{}, fmt.Errorf("persistence: opening file analytics store: %w", err)
		}
		return AnalyticsStore{Store: store, Backend: cfg.Backend, Location: filePath}, nil
	case config.BackendSQLite:
		if repoRoot == "" {
			return AnalyticsStore{}, fmt.Errorf("persistence: sqlite analytics store needs a repository root")
		}
		path := sqlite.DefaultPath(repoRoot)
		openCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		store, err := sqlite.OpenAnalyticsStore(openCtx, path)
		if err != nil {
			return AnalyticsStore{}, fmt.Errorf("persistence: opening sqlite analytics store: %w", err)
		}
		return AnalyticsStore{Store: store, Closer: store, Backend: cfg.Backend, Location: path}, nil
	case config.BackendPostgres:
		connStr := config.ExpandEnvVars(cfg.ConnectionString)
		target := postgres.Target(connStr)
		connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		pool, err := postgres.NewPool(connectCtx, connStr, cfg.PoolSize)
		if err != nil {
			return AnalyticsStore{}, fmt.Errorf("persistence: connecting to postgres analytics store at %s: %w", target, err)
		}
		if cfg.MigrationMode == config.MigrationModeAuto {
			if _, err := postgres.NewMigrator(pool).Up(ctx); err != nil {
				pool.Close()
				return AnalyticsStore{}, fmt.Errorf("persistence: migrating postgres analytics store at %s: %w", target, err)
			}
		} else if err := postgres.VerifyAnalyticsSchema(connectCtx, pool); err != nil {
			pool.Close()
			return AnalyticsStore{}, fmt.Errorf("persistence: postgres analytics store at %s is not ready: %w", target, err)
		}
		return AnalyticsStore{Store: postgres.NewAnalyticsStore(pool), Closer: closerFunc(func() error { pool.Close(); return nil }), Backend: cfg.Backend, Location: target}, nil
	default:
		return AnalyticsStore{}, fmt.Errorf("persistence: unsupported backend %q for analytics", cfg.Backend)
	}
}
