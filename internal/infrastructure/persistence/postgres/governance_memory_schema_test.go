package postgres_test

// governance_memory_schema_test.go covers the check that runs before `relicta history`,
// `relicta report` or a release touches a PostgreSQL governance store under
// `migration_mode: manual`.
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

// A database migrated for release runs and not for governance memory is a state an operator
// can reach — the two migrations landed separately — and it has to be named as itself. Without
// this check the first sign of trouble is `ERROR: relation "governance_releases" does not
// exist` out of the middle of `relicta history`.
func TestADatabaseWithNoGovernanceSchemaIsReportedWithTheCommandThatFixesIt(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, dsn, 2)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	err = postgres.VerifyGovernanceMemorySchema(ctx, pool)

	if err == nil {
		t.Fatal("a database with no governance_releases table passed the schema check; the " +
			"first failure an operator sees would come out of the middle of a release")
	}
	if !errors.Is(err, postgres.ErrGovernanceSchemaMissing) {
		t.Errorf("the error is %v, not ErrGovernanceSchemaMissing: a caller cannot separate "+
			"\"migrate this database\" from \"this database is unreachable\"", err)
	}
	if errors.Is(err, postgres.ErrSchemaMissing) {
		t.Error("the governance check reports ErrSchemaMissing, which reads as \"release run " +
			"schema is missing\" and sends the operator to check the wrong table")
	}
	if !strings.Contains(err.Error(), "relicta db migrate") {
		t.Errorf("the error is %q and does not name the command that creates the schema", err)
	}
}

func TestAMigratedDatabasePassesTheGovernanceSchemaCheck(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool := mustMigrate(t, ctx, dsn)
	defer pool.Close()

	if err := postgres.VerifyGovernanceMemorySchema(ctx, pool); err != nil {
		t.Errorf("a migrated database failed the governance schema check: %v — every command "+
			"that reads the audit trail would refuse a database that is in fact ready", err)
	}
}
