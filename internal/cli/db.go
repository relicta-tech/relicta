// Package cli implements the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/infrastructure/persistence/postgres"
)

// DBCommands holds the CLI commands for database operations.
// This is designed to be integrated with the Cobra command tree.
type DBCommands struct {
	getConfig func() config.PersistenceConfig
}

// NewDBCommands creates a new DBCommands with the given config provider.
func NewDBCommands(getConfig func() config.PersistenceConfig) *DBCommands {
	return &DBCommands{getConfig: getConfig}
}

// RunMigrate applies all pending database migrations.
func (d *DBCommands) RunMigrate(ctx context.Context) error {
	cfg := d.getConfig()

	if cfg.Backend != config.BackendPostgres {
		return fmt.Errorf("db commands require postgres backend (current: %s)", cfg.Backend)
	}

	connStr := config.ExpandEnvVars(cfg.ConnectionString)
	pool, err := postgres.NewPool(ctx, connStr, cfg.PoolSize)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	migrator := postgres.NewMigrator(pool)
	applied, err := migrator.Up(ctx)
	if err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	if applied == 0 {
		fmt.Println("Database is up to date. No migrations applied.")
	} else {
		fmt.Printf("Applied %d migration(s) successfully.\n", applied)
	}
	return nil
}

// RunMigrateDown rolls back the last applied migration.
func (d *DBCommands) RunMigrateDown(ctx context.Context) error {
	cfg := d.getConfig()

	if cfg.Backend != config.BackendPostgres {
		return fmt.Errorf("db commands require postgres backend (current: %s)", cfg.Backend)
	}

	connStr := config.ExpandEnvVars(cfg.ConnectionString)
	pool, err := postgres.NewPool(ctx, connStr, cfg.PoolSize)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	migrator := postgres.NewMigrator(pool)
	if err := migrator.Down(ctx); err != nil {
		return fmt.Errorf("rolling back migration: %w", err)
	}

	fmt.Println("Rolled back one migration successfully.")
	return nil
}

// RunStatus displays the current migration status.
func (d *DBCommands) RunStatus(ctx context.Context) error {
	cfg := d.getConfig()

	if cfg.Backend != config.BackendPostgres {
		return fmt.Errorf("db commands require postgres backend (current: %s)", cfg.Backend)
	}

	connStr := config.ExpandEnvVars(cfg.ConnectionString)
	pool, err := postgres.NewPool(ctx, connStr, cfg.PoolSize)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	migrator := postgres.NewMigrator(pool)
	statuses, err := migrator.Status(ctx)
	if err != nil {
		return fmt.Errorf("checking migration status: %w", err)
	}

	if len(statuses) == 0 {
		fmt.Println("No migrations found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tNAME\tSTATUS\tAPPLIED AT")
	fmt.Fprintln(w, "-------\t----\t------\t----------")

	for _, s := range statuses {
		status := "pending"
		appliedAt := "-"
		if s.Applied {
			status = "applied"
			appliedAt = s.AppliedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Version, s.Name, status, appliedAt)
	}

	return w.Flush()
}
