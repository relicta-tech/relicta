package cli

import (
	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// dbCmd exposes the PostgreSQL migration runner.
//
// DBCommands, the migrator, the migrations and the connection pool were all implemented and
// tested, and `NewDBCommands` was called from nowhere — so no `db` command existed in the
// binary at all. `relicta db migrate` is documented in CLAUDE.md and answered "unknown command
// for relicta. Did you mean this? hub", which is the shape this defect always takes here: a
// finished component with no caller.
//
// Reaching config through a function rather than capturing it is what DBCommands already asks
// for, and it matters: cobra builds the command tree at init time, before any config file has
// been read, so a value captured here would always be the zero one.
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage the database backing relicta's release history",
	Long: `Manage the database relicta keeps its release history in.

migrate, migrate-down and status drive the PostgreSQL migrator, so they require
persistence.backend to be "postgres". The connection string supports environment
expansion, so it can be kept out of the file entirely:

  persistence:
    backend: postgres
    connection_string: "${DATABASE_URL}"
    pool_size: 10

import is not tied to postgres: it reads the existing .relicta/releases tree into
whichever backend persistence.backend selects, so it works for sqlite too. The
one backend it refuses is "file", where the source and the destination would be
the same store.`,
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply all pending database migrations",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newDBCommands().RunMigrate(cmd.Context())
	},
}

var dbMigrateDownCmd = &cobra.Command{
	Use:   "migrate-down",
	Short: "Roll back the most recently applied migration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newDBCommands().RunMigrateDown(cmd.Context())
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which migrations have been applied",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return newDBCommands().RunStatus(cmd.Context())
	},
}

// newDBCommands binds the db commands to the loaded configuration.
func newDBCommands() *DBCommands {
	return NewDBCommands(func() config.PersistenceConfig {
		if cfg == nil {
			return config.DefaultConfig().Persistence
		}
		return cfg.Persistence
	})
}

func init() {
	dbCmd.AddCommand(dbMigrateCmd, dbMigrateDownCmd, dbStatusCmd, dbImportCmd)
}
