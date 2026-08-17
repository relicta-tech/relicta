package postgres

// release_run_schema.go answers one question before any release command runs: is the
// release_runs table there?
//
// It exists because of what happens without it. Under `migration_mode: manual` relicta must
// not create tables in a database an operator provisioned, so a configured-but-unmigrated
// database is an ordinary state — and the first thing the operator would otherwise see is
// `relicta plan` failing with `ERROR: relation "release_runs" does not exist (SQLSTATE
// 42P01)` out of the middle of a save. That names a table they have never heard of and no
// command they could run. ADR-013 is about a setting that tells the truth; a truthful
// failure is part of it.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// undefinedTable is PostgreSQL's SQLSTATE for a relation that does not exist.
const undefinedTable = "42P01"

// ErrSchemaMissing reports a database that has no release run schema.
//
// A sentinel rather than only a message, so a caller can distinguish "migrate this database"
// from "this database is unreachable" — the first is the operator's next step, the second is
// their infrastructure.
var ErrSchemaMissing = errors.New("release run schema is missing")

// VerifyReleaseRunSchema reports whether the release run tables exist, naming the command
// that creates them when they do not.
//
// A probe query rather than a look at schema_migrations: the migrations table would have to
// be created to be read, which is the one thing manual mode promises not to do, and what
// callers actually depend on is the table, not the bookkeeping about it.
func VerifyReleaseRunSchema(ctx context.Context, pool *pgxpool.Pool) error {
	// LIMIT 0 so the plan touches no rows: this asks whether the relation exists, and a
	// database with a long release history should not pay for the question.
	_, err := pool.Exec(ctx, `SELECT 1 FROM release_runs LIMIT 0`)
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == undefinedTable {
		return fmt.Errorf("%w: run 'relicta db migrate' to create it, or set "+
			"persistence.migration_mode to 'auto' to have relicta apply migrations itself",
			ErrSchemaMissing)
	}
	return fmt.Errorf("checking the release run schema: %w", err)
}
