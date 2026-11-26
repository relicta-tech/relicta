package persistence

import (
	"context"
	"fmt"
	"io"

	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/postgres"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/sqlite"
	"github.com/relicta-tech/relicta/v5/internal/security/token"
)

// TokenRevocationBackend holds the durable session-denial store and its connection.
type TokenRevocationBackend struct {
	Store  token.RevocationStore
	Closer io.Closer
}

// OpenTokenRevocationStore uses the configured persistence backend. The file backend keeps
// release data as files but places its small transactional session store in .relicta/relicta.db.
func OpenTokenRevocationStore(ctx context.Context, cfg config.PersistenceConfig, repoRoot string) (TokenRevocationBackend, error) {
	if cfg.Backend == "" {
		cfg.Backend = config.BackendFile
	}
	if err := cfg.Validate(); err != nil {
		return TokenRevocationBackend{}, fmt.Errorf("persistence: %w", err)
	}
	switch cfg.Backend {
	case config.BackendFile, config.BackendSQLite:
		if repoRoot == "" {
			return TokenRevocationBackend{}, fmt.Errorf("persistence: dashboard session storage needs a working directory")
		}
		openCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		store, err := sqlite.OpenTokenRevocationStore(openCtx, sqlite.DefaultPath(repoRoot))
		if err != nil {
			return TokenRevocationBackend{}, fmt.Errorf("persistence: opening durable dashboard session store: %w", err)
		}
		return TokenRevocationBackend{Store: store, Closer: store}, nil
	case config.BackendPostgres:
		connStr := config.ExpandEnvVars(cfg.ConnectionString)
		target := postgres.Target(connStr)
		connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
		defer cancel()
		pool, err := postgres.NewPool(connectCtx, connStr, cfg.PoolSize)
		if err != nil {
			return TokenRevocationBackend{}, fmt.Errorf("persistence: connecting to dashboard session store at %s: %w", target, err)
		}
		closePool := closerFunc(func() error { pool.Close(); return nil })
		if cfg.MigrationMode == config.MigrationModeAuto {
			if _, err := postgres.NewMigrator(pool).Up(ctx); err != nil {
				_ = closePool.Close()
				return TokenRevocationBackend{}, fmt.Errorf("persistence: migrating dashboard session store at %s: %w", target, err)
			}
		} else if err := postgres.VerifyTokenRevocationSchema(connectCtx, pool); err != nil {
			_ = closePool.Close()
			return TokenRevocationBackend{}, fmt.Errorf("persistence: dashboard session store at %s is not ready: %w", target, err)
		}
		return TokenRevocationBackend{Store: postgres.NewTokenRevocationStore(pool), Closer: closePool}, nil
	default:
		return TokenRevocationBackend{}, fmt.Errorf("persistence: unsupported backend %q for dashboard sessions", cfg.Backend)
	}
}
