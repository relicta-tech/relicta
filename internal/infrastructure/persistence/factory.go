// Package persistence provides a factory for creating EventStore instances
// based on configuration.
package persistence

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/infrastructure/persistence/postgres"
)

// NewEventStore creates an EventStore based on the given persistence configuration.
// For postgres backends with MigrationModeAuto, migrations are applied automatically.
func NewEventStore(ctx context.Context, cfg config.PersistenceConfig) (ports.EventStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid persistence config: %w", err)
	}

	switch cfg.Backend {
	case config.BackendPostgres:
		connStr := config.ExpandEnvVars(cfg.ConnectionString)

		store, err := postgres.New(ctx, postgres.Config{
			ConnectionString: connStr,
			PoolSize:         cfg.PoolSize,
		})
		if err != nil {
			return nil, fmt.Errorf("creating postgres event store: %w", err)
		}

		if cfg.MigrationMode == config.MigrationModeAuto {
			pool, err := postgres.NewPool(ctx, connStr, cfg.PoolSize)
			if err != nil {
				_ = store.Close()
				return nil, fmt.Errorf("creating migration pool: %w", err)
			}
			defer pool.Close()

			migrator := postgres.NewMigrator(pool)
			applied, err := migrator.Up(ctx)
			if err != nil {
				_ = store.Close()
				return nil, fmt.Errorf("running auto-migrations: %w", err)
			}
			if applied > 0 {
				// Migrations were applied successfully.
				_ = applied
			}
		}

		return store, nil

	default:
		return nil, fmt.Errorf("unsupported persistence backend for factory: %q (use file-based adapter directly)", cfg.Backend)
	}
}
