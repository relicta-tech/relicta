package postgres

// governance_memory_schema.go asks the same question release_run_schema.go asks, about the
// other half of ADR-013's system of record: are the governance memory tables there?
//
// It is a separate probe rather than a second table in the same query because the two are
// separately reachable. `relicta history` and `relicta report` open governance memory and no
// run store at all, so a database migrated to 002 and not 003 must fail those commands by
// name — not pass a release-run check and then die inside a SELECT against
// governance_releases with a relation an operator has never heard of.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrGovernanceSchemaMissing reports a database that has no governance memory schema.
//
// Distinct from ErrSchemaMissing rather than shared with it, because the two name different
// migrations and an operator reading "release run schema is missing" out of `relicta history`
// would go and check the wrong table.
var ErrGovernanceSchemaMissing = errors.New("governance memory schema is missing")

// VerifyGovernanceMemorySchema reports whether the governance memory tables exist, naming the
// command that creates them when they do not.
//
// Two relations are probed, one per migration, and not all six tables. Within a migration the
// tables arrive together or not at all — each runs in one transaction — so a database holding
// some of 003's tables has been edited by hand, a state a probe cannot repair and should not
// pretend to diagnose. Across migrations is different: a database stopped at 003 is an
// ordinary upgrade that has not been run yet, and the audit chain it lacks is where every
// governance event is about to be recorded. Probing only governance_releases would let such a
// database open cleanly and then fail at the first append, mid-release, with a relation an
// operator has never heard of.
func VerifyGovernanceMemorySchema(ctx context.Context, pool *pgxpool.Pool) error {
	// LIMIT 0 so the plan touches no rows: this asks whether the relation exists, and a
	// database holding years of governance history should not pay for the question.
	// Whole statements rather than a table name spliced into one. Nothing here is
	// caller-supplied, so it is not an injection today; a literal per probe means it
	// cannot become one when somebody later makes the list configurable.
	probes := []struct {
		relation string
		query    string
	}{
		{"governance_releases", `SELECT 1 FROM governance_releases LIMIT 0`},
		{"governance_audit_entries", `SELECT 1 FROM governance_audit_entries LIMIT 0`},
	}

	for _, probe := range probes {
		if _, err := pool.Exec(ctx, probe.query); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == undefinedTable {
				return fmt.Errorf("%w: %s is not there; run 'relicta db migrate' to "+
					"create it, or set persistence.migration_mode to 'auto' to have "+
					"relicta apply migrations itself",
					ErrGovernanceSchemaMissing, probe.relation)
			}
			return fmt.Errorf("checking the governance memory schema: %w", err)
		}
	}
	return nil
}
