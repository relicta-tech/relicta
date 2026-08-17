package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/sqlite/migrations"
)

// MigrationDirection indicates whether a migration is being applied or rolled back.
type MigrationDirection string

const (
	// DirectionUp applies a migration.
	DirectionUp MigrationDirection = "up"
	// DirectionDown rolls one back.
	DirectionDown MigrationDirection = "down"
)

// MigrationStatus reports whether one known migration has been applied.
type MigrationStatus struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt time.Time
}

// Migrator applies the embedded schema to a SQLite database.
//
// Same shape as the PostgreSQL migrator — a schema_migrations table, up/down file
// pairs named NNN_description, one transaction per migration — because an operator who
// has read `relicta db migrate` for one backend should not have to learn a second
// model for the other. The differences are the ones the database forces: database/sql
// instead of pgx, and INTEGER unix seconds instead of TIMESTAMPTZ.
type Migrator struct {
	db *sql.DB
}

// NewMigrator creates a Migrator for an open database.
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// migration is one parsed up/down file pair.
type migration struct {
	version string
	name    string
	upSQL   string
	downSQL string
}

// EnsureMigrationsTable creates the tracking table if it is absent.
func (m *Migrator) EnsureMigrationsTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT    NOT NULL PRIMARY KEY,
			name       TEXT    NOT NULL,
			applied_at INTEGER NOT NULL
		) STRICT
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}
	return nil
}

// Up applies every pending migration in version order and reports how many ran.
func (m *Migrator) Up(ctx context.Context) (int, error) {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return 0, err
	}

	migs, err := parseMigrations()
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
		if err := m.apply(ctx, mig, DirectionUp); err != nil {
			return count, fmt.Errorf("applying migration %s (%s): %w", mig.version, mig.name, err)
		}
		count++
	}
	return count, nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return err
	}

	migs, err := parseMigrations()
	if err != nil {
		return err
	}

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return err
	}

	for i := len(migs) - 1; i >= 0; i-- {
		mig := migs[i]
		if !applied[mig.version] {
			continue
		}
		if err := m.apply(ctx, mig, DirectionDown); err != nil {
			return fmt.Errorf("rolling back migration %s (%s): %w", mig.version, mig.name, err)
		}
		return nil
	}

	return fmt.Errorf("no migrations to roll back")
}

// Status reports every known migration and whether it has been applied.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.EnsureMigrationsTable(ctx); err != nil {
		return nil, err
	}

	migs, err := parseMigrations()
	if err != nil {
		return nil, err
	}

	rows, err := m.db.QueryContext(ctx,
		`SELECT version, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("querying schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	appliedAt := make(map[string]time.Time)
	for rows.Next() {
		var version string
		var unixSeconds int64
		if err := rows.Scan(&version, &unixSeconds); err != nil {
			return nil, fmt.Errorf("scanning schema_migrations row: %w", err)
		}
		appliedAt[version] = time.Unix(unixSeconds, 0)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schema_migrations rows: %w", err)
	}

	statuses := make([]MigrationStatus, 0, len(migs))
	for _, mig := range migs {
		status := MigrationStatus{Version: mig.version, Name: mig.name}
		if at, ok := appliedAt[mig.version]; ok {
			status.Applied = true
			status.AppliedAt = at
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// apply runs one migration and records it, in a single transaction.
//
// The schema change and the schema_migrations row commit together on purpose: a
// migration that ran without being recorded would be applied again on the next Up, and
// CREATE INDEX is only forgiving of that because every statement carries IF NOT EXISTS.
// The transaction means that forgiveness is never load bearing.
func (m *Migrator) apply(ctx context.Context, mig migration, dir MigrationDirection) error {
	var script string
	switch dir {
	case DirectionUp:
		script = mig.upSQL
	case DirectionDown:
		script = mig.downSQL
	}
	if script == "" {
		return fmt.Errorf("no %s SQL for migration %s", dir, mig.version)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("executing %s SQL: %w", dir, err)
	}

	switch dir {
	case DirectionUp:
		_, err = tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			mig.version, mig.name, time.Now().Unix())
	case DirectionDown:
		_, err = tx.ExecContext(ctx,
			`DELETE FROM schema_migrations WHERE version = ?`, mig.version)
	}
	if err != nil {
		return fmt.Errorf("updating schema_migrations: %w", err)
	}

	return tx.Commit()
}

// appliedVersions returns the set of versions already recorded.
func (m *Migrator) appliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("querying applied versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

// parseMigrations reads the embedded files and groups them into version-ordered pairs.
func parseMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("reading embedded migrations: %w", err)
	}

	byVersion := make(map[string]*migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		var dir MigrationDirection
		var base string
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			dir, base = DirectionUp, strings.TrimSuffix(name, ".up.sql")
		case strings.HasSuffix(name, ".down.sql"):
			dir, base = DirectionDown, strings.TrimSuffix(name, ".down.sql")
		default:
			continue
		}

		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, fmt.Errorf("reading migration file %s: %w", name, err)
		}

		parts := strings.SplitN(base, "_", 2)
		version := parts[0]
		migName := ""
		if len(parts) > 1 {
			migName = parts[1]
		}

		mig, ok := byVersion[version]
		if !ok {
			mig = &migration{version: version, name: migName}
			byVersion[version] = mig
		}
		switch dir {
		case DirectionUp:
			mig.upSQL = string(data)
		case DirectionDown:
			mig.downSQL = string(data)
		}
	}

	result := make([]migration, 0, len(byVersion))
	for _, mig := range byVersion {
		result = append(result, *mig)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}
