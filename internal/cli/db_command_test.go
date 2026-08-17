package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// DBCommands, the migrator, the migrations and the connection pool were all implemented and
// tested, and NewDBCommands was called from nowhere — so no `db` command existed in the binary
// at all. `relicta db migrate` is documented in CLAUDE.md and the shipped binary answered:
//
//	unknown command "db" for "relicta"
//	Did you mean this?
//	        hub
//
// The same shape as every other finding in this sweep: a finished component with no caller.

func TestTheDatabaseCommandIsRegistered(t *testing.T) {
	sub, _, err := rootCmd.Find([]string{"db"})
	if err != nil || sub == nil || sub.Name() != "db" {
		t.Fatalf("rootCmd has no \"db\" command (found %v, err %v).\nThe migrator and its "+
			"migrations exist and are documented; without registration they are unreachable "+
			"from the binary", sub, err)
	}

	for _, name := range []string{"migrate", "migrate-down", "status", "import"} {
		t.Run(name, func(t *testing.T) {
			found, _, findErr := rootCmd.Find([]string{"db", name})
			if findErr != nil || found == nil || found.Name() != name {
				t.Errorf("db has no %q subcommand", name)
			}
			if found != nil && found.RunE == nil {
				t.Errorf("db %s is registered but does nothing", name)
			}
		})
	}
}

// Cobra builds the command tree at init time, before any config file is read, so the commands
// must reach configuration through a function. A captured value would always be the zero one.
func TestTheDatabaseCommandReadsConfigWhenItRuns(t *testing.T) {
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })

	cfg = config.DefaultConfig()
	cfg.Persistence.Backend = config.BackendPostgres
	cfg.Persistence.ConnectionString = "postgres://example/db"

	got := newDBCommands().getConfig()
	if got.Backend != config.BackendPostgres {
		t.Errorf("Backend = %q, want postgres: the commands are reading a config snapshot "+
			"taken before the file was loaded", got.Backend)
	}
	if got.ConnectionString != "postgres://example/db" {
		t.Errorf("ConnectionString = %q, want the configured one", got.ConnectionString)
	}
}

// `db import` is not postgres-only, and its help has to say so.
//
// migrate, migrate-down and status refuse anything but postgres because they drive the postgres
// migrator. Import drives no migrator: it writes through whichever ReleaseRunRepository the
// setting selected, which includes sqlite. An operator who read the group's help and concluded
// otherwise would go on believing there is no way to migrate a local history.
//
// The promise about the JSON tree is asserted here too, because it is a promise: ADR-013 leaves
// the export in place until the operator removes it, and the only place they learn that before
// running the command is this text.
func TestTheImportCommandDocumentsWhatItAcceptsAndWhatItLeavesAlone(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"db", "import"})
	if err != nil || found == nil {
		t.Fatalf("db import is not registered: %v", err)
	}

	help := found.Long + " " + found.Short
	if !strings.Contains(help, "sqlite") && !strings.Contains(dbCmd.Long, "sqlite") {
		t.Error("neither `db import`'s help nor the db group's help mentions sqlite, so an " +
			"operator reading \"these commands require postgres\" concludes a local history " +
			"cannot be migrated at all")
	}
	if !strings.Contains(help, "left exactly as it was") {
		t.Error("db import's help does not say the JSON tree is left in place: ADR-013 keeps " +
			"it as an export until the operator removes it, and this is where they find out")
	}
}

// Before this the section had no defaults at all, so an operator who wrote only a backend and a
// connection string was refused with "pool_size must be greater than 0" — a value they never
// saw and could not have known to set.
func TestPersistenceDefaultsApplyToAPartialSection(t *testing.T) {
	defaults := config.DefaultConfig().Persistence

	if defaults.Backend != config.BackendFile {
		t.Errorf("Backend = %q, want %q: an empty backend reaches the user as \"db commands "+
			"require postgres backend (current: )\"", defaults.Backend, config.BackendFile)
	}
	if defaults.PoolSize <= 0 {
		t.Errorf("PoolSize = %d, want a usable pool: a postgres config naming only a "+
			"backend and a connection string fails validation without it", defaults.PoolSize)
	}
	if defaults.MigrationMode == "" {
		t.Error("MigrationMode is empty, which Validate rejects for the postgres backend")
	}

	partial := config.DefaultConfig().Persistence
	partial.Backend = config.BackendPostgres
	partial.ConnectionString = "postgres://example/db"
	if err := partial.Validate(); err != nil {
		t.Errorf("a config naming only backend and connection_string is invalid: %v", err)
	}
}

// The db commands operate on a database; refusing the file backend is the whole point, and the
// message has to name what is actually configured.
func TestTheDatabaseCommandRefusesTheFileBackendByName(t *testing.T) {
	cmds := NewDBCommands(func() config.PersistenceConfig {
		return config.DefaultConfig().Persistence
	})

	err := cmds.RunStatus(t.Context())
	if err == nil {
		t.Fatal("RunStatus accepted the file backend")
	}
	if !strings.Contains(err.Error(), "file") {
		t.Errorf("refusal %q does not name the configured backend", err)
	}
}
