package postgres

// release_run_repository.go implements ports.ReleaseRunRepository against PostgreSQL:
// the shared-state backend of ADR-013, for teams whose governance record has to outlive
// any one developer's working copy.
//
// The file backend answers FindByState by reading every run off disk and comparing in
// Go. That is the cost ADR-013 was written to remove, so the queries here are queries —
// the columns exist because these methods filter on them, and nothing loads a run it is
// about to discard.
//
// What this file does not do is decide who uses it. `persistence.backend` still resolves
// to `file` for everyone; ADR-013 flips the default on evidence, and the evidence is the
// conformance suite passing, not the adapter existing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// ReleaseRunRepository stores release runs in PostgreSQL.
//
// Safe for concurrent use: it holds no mutable state, and pgxpool is itself concurrent.
// The file adapter needs a mutex because two goroutines writing one directory can
// interleave; here the database serializes writers, and Save is a single statement.
type ReleaseRunRepository struct {
	pool *pgxpool.Pool
}

// NewReleaseRunRepository creates a repository over an existing pool.
//
// The pool is borrowed, not owned — the caller closes it. A repository sharing the pool
// that already serves the event store is the point: ADR-013 wants a run and the record
// it produces writable in one transaction, and two pools cannot share a transaction.
func NewReleaseRunRepository(pool *pgxpool.Pool) *ReleaseRunRepository {
	return &ReleaseRunRepository{pool: pool}
}

// Ensure ReleaseRunRepository implements the port in full.
var _ ports.ReleaseRunRepository = (*ReleaseRunRepository)(nil)

// scopeKey normalizes a repository root into the key rows are stored under.
//
// Cleaned, but deliberately not resolved against the filesystem. The file adapter stats
// the directory because it is about to read a file out of it; this adapter is not, and a
// dashboard server rendering a team's release history has never had any of their
// checkouts on disk. Requiring the path to exist would make the shared backend answerable
// only from the machines it was trying to stop being tied to.
//
// There is no injection risk in trading the check away: repo_root reaches the database as
// a bound parameter, never as concatenated SQL and never as a path this process opens.
func scopeKey(repoRoot string) string {
	if repoRoot == "" {
		// Clean("") is ".", which would quietly file runs under the process's working
		// directory. An empty root is passed through as itself so it matches only what
		// was stored under it.
		return ""
	}
	return filepath.Clean(repoRoot)
}

// orderNewestFirst is the ordering List and every query built on it returns.
//
// The port's documentation says "creation time (newest first)", and the reference
// implementation sorts by file modification time — which is the last save, not the
// creation. Callers were written against the reference, and a backend switch silently
// reordering `relicta history` is exactly the drift the conformance suite exists to
// catch, so this follows the reference: newest write first. The run's own UpdatedAt is
// used rather than a stored-at column so the order describes the run rather than when a
// process happened to flush it.
//
// run_id breaks ties. The file adapter inherits whatever order sort.Slice leaves equal
// mtimes in, which is not stable; two runs saved in the same instant should still come
// back the same way twice.
const orderNewestFirst = ` ORDER BY updated_at DESC, run_id ASC`

// Save persists a run, inserting it or replacing the stored copy.
//
// One upsert, no transaction, last writer wins. Three reasons, since this backend exists
// precisely so several processes write the same repository:
//
// A run is saved at every state transition, so "already there" is the common path rather
// than a conflict to report. The reference implementation resolves it by writing a temp
// file and renaming over the old one — last writer wins, atomically — and this is the
// same guarantee: a single statement, so a concurrent reader sees the run before or
// after, never halfway through.
//
// Optimistic concurrency was the alternative and there is nothing to be optimistic about.
// ReleaseRun carries no revision, so a version column would be state this adapter
// invented and only this adapter could check; Save would then fail in a way no caller in
// the tree handles, on a conflict that is usually two processes writing the same
// transition. Detecting genuinely divergent concurrent edits needs the aggregate to grow
// a revision, and that is a domain change, not a schema one.
func (r *ReleaseRunRepository) Save(ctx context.Context, run *domain.ReleaseRun) error {
	if run == nil {
		return fmt.Errorf("cannot save a nil run")
	}

	payload, err := json.Marshal(adapters.ToDTO(run))
	if err != nil {
		return fmt.Errorf("marshaling run %s: %w", run.ID(), err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO release_runs
			(repo_root, run_id, state, plan_hash, created_at, updated_at, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (repo_root, run_id) DO UPDATE SET
			state      = EXCLUDED.state,
			plan_hash  = EXCLUDED.plan_hash,
			updated_at = EXCLUDED.updated_at,
			payload    = EXCLUDED.payload`,
		scopeKey(run.RepoRoot()),
		string(run.ID()),
		string(run.State()),
		run.PlanHash(),
		run.CreatedAt(),
		run.UpdatedAt(),
		payload,
	)
	if err != nil {
		return fmt.Errorf("saving run %s: %w", run.ID(), err)
	}
	return nil
}

// Load retrieves a run by ID alone.
//
// One database serves many repositories, so an ID with no root does not name a row. The
// reference resolves the same problem by scanning the roots its own instance happens to
// have written and taking the first hit — which cannot work here, because the whole
// reason to share a database is that the run may have been saved by another process
// entirely. So this searches every repository and returns the most recently updated
// match.
//
// Collisions are possible rather than hypothetical: a run ID is derived from the plan
// hash, so the same release planned from two checkouts of one repository produces the
// same ID under two roots. Newest-first makes that resolution deterministic, and Delete
// resolves identically so that a bare ID names the same run to both.
//
// Callers that know their root should reach for LoadFromRepo, which is scoped and cannot
// cross repositories at all; the use cases in internal/domain/release/app already do.
func (r *ReleaseRunRepository) Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	var payload []byte
	err := r.pool.QueryRow(ctx,
		`SELECT payload FROM release_runs WHERE run_id = $1`+orderNewestFirst+` LIMIT 1`,
		string(runID),
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRunNotFound
		}
		return nil, fmt.Errorf("loading run %s: %w", runID, err)
	}

	return runFromPayload(payload)
}

// LoadFromRepo retrieves a run from one repository.
//
// Six use cases — bump, status, notes, publish, approve, retry — type-assert for this
// method and fall back to Load when the repository does not have it. Implementing it is
// what keeps them scoped: without it they would resolve a bare ID against every
// repository in a shared database.
func (r *ReleaseRunRepository) LoadFromRepo(
	ctx context.Context, repoRoot string, runID domain.RunID,
) (*domain.ReleaseRun, error) {
	var payload []byte
	err := r.pool.QueryRow(ctx,
		`SELECT payload FROM release_runs WHERE repo_root = $1 AND run_id = $2`,
		scopeKey(repoRoot), string(runID),
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRunNotFound
		}
		return nil, fmt.Errorf("loading run %s: %w", runID, err)
	}

	return runFromPayload(payload)
}

// LoadBatch retrieves several runs at once, omitting the ones that are not there.
//
// A missing run is an absence rather than a failure — history asks for the IDs it listed,
// and one deleted in between should shrink the answer, not fail it.
func (r *ReleaseRunRepository) LoadBatch(
	ctx context.Context, repoRoot string, runIDs []domain.RunID,
) (map[domain.RunID]*domain.ReleaseRun, error) {
	result := make(map[domain.RunID]*domain.ReleaseRun, len(runIDs))
	if len(runIDs) == 0 {
		return result, nil
	}

	ids := make([]string, len(runIDs))
	for i, id := range runIDs {
		ids[i] = string(id)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT run_id, payload FROM release_runs WHERE repo_root = $1 AND run_id = ANY($2)`,
		scopeKey(repoRoot), ids,
	)
	if err != nil {
		return nil, fmt.Errorf("loading run batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var payload []byte
		if err := rows.Scan(&id, &payload); err != nil {
			return nil, fmt.Errorf("scanning run row: %w", err)
		}

		run, err := runFromPayload(payload)
		if err != nil {
			// Skipped rather than returned, matching the reference: one unreadable run
			// should not deny the caller the rest of its history.
			continue
		}
		result[domain.RunID(id)] = run
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run rows: %w", err)
	}

	return result, nil
}

// LoadLatest retrieves the run the repository's latest pointer names.
//
// Two failures collapse into ErrRunNotFound, as they do on the file backend: no pointer
// at all, and a pointer naming a run that has since been deleted. Every command that
// starts with "what release am I in the middle of" asks this in repositories that have
// never run one, and neither case is an error worth distinguishing to them.
func (r *ReleaseRunRepository) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	root := scopeKey(repoRoot)

	var runID string
	err := r.pool.QueryRow(ctx,
		`SELECT run_id FROM release_run_latest WHERE repo_root = $1`, root,
	).Scan(&runID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRunNotFound
		}
		return nil, fmt.Errorf("reading latest pointer: %w", err)
	}

	return r.LoadFromRepo(ctx, root, domain.RunID(runID))
}

// SetLatest points a repository at its current run.
//
// The pointer is a name, not a reference: it is written whether or not the run exists,
// because that is what the file backend does and a caller that sets the pointer first
// must not start failing on a backend switch. LoadLatest reports the run missing.
func (r *ReleaseRunRepository) SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO release_run_latest (repo_root, run_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (repo_root) DO UPDATE SET
			run_id     = EXCLUDED.run_id,
			updated_at = EXCLUDED.updated_at`,
		scopeKey(repoRoot), string(runID),
	)
	if err != nil {
		return fmt.Errorf("setting latest run for %s: %w", repoRoot, err)
	}
	return nil
}

// List returns every run ID in a repository, newest write first.
func (r *ReleaseRunRepository) List(ctx context.Context, repoRoot string) ([]domain.RunID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT run_id FROM release_runs WHERE repo_root = $1`+orderNewestFirst,
		scopeKey(repoRoot),
	)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	defer rows.Close()

	var ids []domain.RunID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning run ID: %w", err)
		}
		ids = append(ids, domain.RunID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run IDs: %w", err)
	}

	return ids, nil
}

// Delete removes the run a bare ID names, and reports one that is not there.
//
// ErrRunNotFound rather than success, because `relicta clean` can only tell an operator
// which runs it failed to remove if the repository tells it. The reference is deliberately
// asymmetric about this and so is this adapter: Delete searches and reports,
// DeleteFromRepo is given the root and tolerates the absence, the difference being
// whether the caller has already established the run should be there.
//
// The inner select resolves the ID exactly as Load does, so deleting what Load returned
// deletes the run the caller was looking at. When one ID exists under two roots, the
// other copy survives — which is again the reference's behavior, whose Delete removes
// the run from a single repository.
func (r *ReleaseRunRepository) Delete(ctx context.Context, runID domain.RunID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM release_runs
		WHERE run_id = $1
		  AND repo_root = (
		      SELECT repo_root FROM release_runs WHERE run_id = $1`+orderNewestFirst+` LIMIT 1
		  )`,
		string(runID),
	)
	if err != nil {
		return fmt.Errorf("deleting run %s: %w", runID, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrRunNotFound
	}
	return nil
}

// DeleteFromRepo removes a run from one repository, tolerating its absence.
//
// The scoped counterpart to Delete: the caller named the repository, so it has already
// established what it is removing, and "already gone" is the outcome it asked for.
func (r *ReleaseRunRepository) DeleteFromRepo(ctx context.Context, repoRoot string, runID domain.RunID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM release_runs WHERE repo_root = $1 AND run_id = $2`,
		scopeKey(repoRoot), string(runID),
	)
	if err != nil {
		return fmt.Errorf("deleting run %s: %w", runID, err)
	}
	return nil
}

// FindByState finds the runs of a repository that are in one state.
//
// The state comparison is exact, and unmatched states return nothing rather than
// erroring: a state no run is in is an ordinary answer, and callers ask about states the
// repository may never have reached.
func (r *ReleaseRunRepository) FindByState(
	ctx context.Context, repoRoot string, state domain.RunState,
) ([]*domain.ReleaseRun, error) {
	return r.queryRuns(ctx,
		`SELECT payload FROM release_runs WHERE repo_root = $1 AND state = $2`+orderNewestFirst,
		scopeKey(repoRoot), string(state),
	)
}

// FindActive finds the runs a repository has not finished.
//
// Which states count as active is asked of the domain rather than written into the SQL.
// A state added to the machine, or IsFinal changing its mind about one, would otherwise
// leave a WHERE clause here quietly disagreeing with RunState.IsActive — and this adapter
// would report a release as finished that the domain considers in flight.
func (r *ReleaseRunRepository) FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error) {
	var active []string
	for _, state := range domain.AllStates() {
		if state.IsActive() {
			active = append(active, string(state))
		}
	}

	return r.queryRuns(ctx,
		`SELECT payload FROM release_runs WHERE repo_root = $1 AND state = ANY($2)`+orderNewestFirst,
		scopeKey(repoRoot), active,
	)
}

// FindByPlanHash finds a repository's run with a given plan hash, if it has one.
//
// nil, nil when there is none. Duplicate detection asks this before every plan, and an
// error would leave the caller unable to tell "nothing was planned under this hash" from
// "the store is unreadable" — it would refuse to plan either way.
//
// Plan hashes are not unique, so this takes the newest of any matches: runs planned
// before a version was set share the empty hash, and the most recent one is the one a
// duplicate check is asking about.
func (r *ReleaseRunRepository) FindByPlanHash(
	ctx context.Context, repoRoot string, planHash string,
) (*domain.ReleaseRun, error) {
	var payload []byte
	err := r.pool.QueryRow(ctx,
		`SELECT payload FROM release_runs WHERE repo_root = $1 AND plan_hash = $2`+
			orderNewestFirst+` LIMIT 1`,
		scopeKey(repoRoot), planHash,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // documented: no duplicate is not an error
		}
		return nil, fmt.Errorf("finding run by plan hash: %w", err)
	}

	return runFromPayload(payload)
}

// queryRuns runs a payload-returning query and reconstructs each row.
func (r *ReleaseRunRepository) queryRuns(
	ctx context.Context, sql string, args ...any,
) ([]*domain.ReleaseRun, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("querying runs: %w", err)
	}
	defer rows.Close()

	var runs []*domain.ReleaseRun
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("scanning run payload: %w", err)
		}

		run, err := runFromPayload(payload)
		if err != nil {
			// Consistent with the reference, which skips runs it cannot parse. A nil in
			// the returned slice would be worse than an omission: callers range over
			// these and read State() straight off.
			continue
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run rows: %w", err)
	}

	return runs, nil
}

// runFromPayload reconstructs a run from its stored JSON.
//
// Through the same DTO the file backend writes, deliberately. Two serializations of one
// aggregate is how BaseRef came back filled from the branch and `relicta evaluate`
// refused every release; see adapters.ToDTO.
func runFromPayload(payload []byte) (*domain.ReleaseRun, error) {
	var dto adapters.ReleaseRunDTO
	if err := json.Unmarshal(payload, &dto); err != nil {
		return nil, fmt.Errorf("unmarshaling run payload: %w", err)
	}

	run, err := adapters.FromDTO(&dto)
	if err != nil {
		return nil, fmt.Errorf("reconstructing run %s: %w", dto.ID, err)
	}
	return run, nil
}
