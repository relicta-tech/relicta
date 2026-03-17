package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/internal/infrastructure/persistence/postgres/migrations"
)

// MigrationDirection indicates whether a migration is being applied or rolled back.
type MigrationDirection string

const (
	DirectionUp   MigrationDirection = "up"
	DirectionDown MigrationDirection = "down"
)

// MigrationStatus represents the current state of a migration.
type MigrationStatus struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt time.Time
}

// Migrator manages database schema migrations using embedded SQL files.
type Migrator struct {
	pool *pgxpool.Pool
}

// NewMigrator creates a new Migrator backed by the given connection pool.
func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// migration represents a single parsed migration file pair.
type migration struct {
	version string
	name    string
	upSQL   string
	downSQL string
}

// EnsureMigrationsTable creates the schema_migrations tracking table if it does not exist.
func (m *Migrator) EnsureMigrationsTable(ctx context.Context) error {
	_, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT        PRIMARY KEY,
			name        TEXT        NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}
	return nil
}

// Up applies all pending migrations in order.
func (m *Migrator) Up(ctx context.Context) (int, error) {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return 0, err
	}

	migs, err := m.parseMigrations()
	if err != nil {
		return 0, err
	}

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, mig := range migs {
		if applied[mig.version] {
			continue
		}

		if err := m.applyMigration(ctx, mig, DirectionUp); err != nil {
			return count, fmt.Errorf("applying migration %s (%s): %w", mig.version, mig.name, err)
		}
		count++
	}
	return count, nil
}

// Down rolls back the last applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return err
	}

	migs, err := m.parseMigrations()
	if err != nil {
		return err
	}

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return err
	}

	// Find the last applied migration to roll back.
	for i := len(migs) - 1; i >= 0; i-- {
		mig := migs[i]
		if !applied[mig.version] {
			continue
		}

		if err := m.applyMigration(ctx, mig, DirectionDown); err != nil {
			return fmt.Errorf("rolling back migration %s (%s): %w", mig.version, mig.name, err)
		}
		return nil
	}

	return fmt.Errorf("no migrations to roll back")
}

// Status returns the status of all known migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return nil, err
	}

	migs, err := m.parseMigrations()
	if err != nil {
		return nil, err
	}

	// Load applied migration timestamps.
	rows, err := m.pool.Query(ctx,
		`SELECT version, applied_at FROM schema_migrations ORDER BY version`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying schema_migrations: %w", err)
	}
	defer rows.Close()

	appliedMap := make(map[string]time.Time)
	for rows.Next() {
		var version string
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, fmt.Errorf("scanning schema_migrations row: %w", err)
		}
		appliedMap[version] = appliedAt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schema_migrations rows: %w", err)
	}

	var statuses []MigrationStatus
	for _, mig := range migs {
		status := MigrationStatus{
			Version: mig.version,
			Name:    mig.name,
		}
		if t, ok := appliedMap[mig.version]; ok {
			status.Applied = true
			status.AppliedAt = t
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// applyMigration executes a migration within a transaction.
func (m *Migrator) applyMigration(ctx context.Context, mig migration, dir MigrationDirection) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var sql string
	switch dir {
	case DirectionUp:
		sql = mig.upSQL
	case DirectionDown:
		sql = mig.downSQL
	}

	if sql == "" {
		return fmt.Errorf("no %s SQL for migration %s", dir, mig.version)
	}

	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("executing %s SQL: %w", dir, err)
	}

	switch dir {
	case DirectionUp:
		_, err = tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
			mig.version, mig.name,
		)
	case DirectionDown:
		_, err = tx.Exec(ctx,
			`DELETE FROM schema_migrations WHERE version = $1`,
			mig.version,
		)
	}
	if err != nil {
		return fmt.Errorf("updating schema_migrations: %w", err)
	}

	return tx.Commit(ctx)
}

// appliedVersions returns a set of already-applied migration versions.
func (m *Migrator) appliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := m.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("querying applied versions: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scanning version: %w", err)
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

// parseMigrations reads embedded SQL files and groups them into migration pairs.
func (m *Migrator) parseMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}

	migMap := make(map[string]*migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, fmt.Errorf("reading migration file %s: %w", name, err)
		}

		// Parse filename: NNN_description.up.sql or NNN_description.down.sql
		var version, migName string
		var dir MigrationDirection

		if strings.HasSuffix(name, ".up.sql") {
			dir = DirectionUp
			base := strings.TrimSuffix(name, ".up.sql")
			parts := strings.SplitN(base, "_", 2)
			version = parts[0]
			if len(parts) > 1 {
				migName = parts[1]
			}
		} else if strings.HasSuffix(name, ".down.sql") {
			dir = DirectionDown
			base := strings.TrimSuffix(name, ".down.sql")
			parts := strings.SplitN(base, "_", 2)
			version = parts[0]
			if len(parts) > 1 {
				migName = parts[1]
			}
		} else {
			continue
		}

		mig, ok := migMap[version]
		if !ok {
			mig = &migration{version: version, name: migName}
			migMap[version] = mig
		}

		switch dir {
		case DirectionUp:
			mig.upSQL = string(data)
		case DirectionDown:
			mig.downSQL = string(data)
		}
	}

	// Sort by version.
	var result []migration
	for _, mig := range migMap {
		result = append(result, *mig)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].version < result[j].version
	})
	return result, nil
}
