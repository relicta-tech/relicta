package postgres_test

// release_run_schema_test.go covers the check that runs before any release command touches a
// PostgreSQL store under `migration_mode: manual`.
//
// Gating follows testcontainer_test.go: short mode skips, an unreachable Docker daemon skips,
// anything else fails. The claim is about what an operator reads, so it needs a real database —
// the message only helps if it is the one PostgreSQL's own error would otherwise replace.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence/postgres"
)

// An unmigrated database is an ordinary state under manual migrations, and relicta must name
// the command that fixes it. Without this check the operator's first sign of trouble is
// `ERROR: relation "release_runs" does not exist (SQLSTATE 42P01)` from inside `relicta plan`,
// which names a table they have never heard of and nothing they could run.
func TestAnUnmigratedDatabaseIsReportedWithTheCommandThatFixesIt(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, dsn, 2)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	err = postgres.VerifyReleaseRunSchema(ctx, pool)

	if err == nil {
		t.Fatal("a database with no release_runs table passed the schema check; the first " +
			"failure an operator sees would come out of the middle of a release")
	}
	if !errors.Is(err, postgres.ErrSchemaMissing) {
		t.Errorf("the error is %v, not ErrSchemaMissing: a caller cannot separate "+
			"\"migrate this database\" from \"this database is unreachable\"", err)
	}
	if !strings.Contains(err.Error(), "relicta db migrate") {
		t.Errorf("the error is %q and does not name the command that creates the schema", err)
	}
}

func TestAMigratedDatabasePassesTheSchemaCheck(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	if err := postgres.VerifyReleaseRunSchema(ctx, pool); err != nil {
		t.Errorf("a migrated database failed the schema check: %v — every command would "+
			"refuse to run against a database that is in fact ready", err)
	}
}
