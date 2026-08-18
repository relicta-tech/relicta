// Package sqlite implements ports.ReleaseRunRepository on a single SQLite file.
//
// ADR-013 puts three adapters behind persistence.backend and makes the release run the
// system of record, so this store holds runs rather than events. It exists because the
// strongest case for a database in relicta is a problem one developer has on one
// laptop: every query in ports.RunQuery is os.ReadDir plus a parse of every file in the
// file adapter, and `history`, `audit`, `report` and the analytics service all walk
// that tree. Here they are queries against indexed columns.
//
// The driver is modernc.org/sqlite, the pure-Go one. Not a preference: .goreleaser.yaml
// sets CGO_ENABLED=0 so one binary cross-compiles to every target, which rules out
// mattn/go-sqlite3 and anything else needing a C toolchain. A cgo driver would trade
// relicta's distribution story for its storage one.
//
// Nothing selects this store yet — ADR-013 keeps `file` the default until parity is
// proven, and the conformance suite in conformance_test.go is that proof.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	// Registers the "sqlite" driver name used by Open. Pure Go, so CGO_ENABLED=0 holds.
	_ "modernc.org/sqlite"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

const (
	// maxOpenConns caps the connection pool.
	//
	// A pure-Go SQLite connection is not free — each carries its own libc thread-local
	// state — and one relicta invocation is a CLI process, not a server. Under WAL the
	// readers among these do not block each other or the writer; writers serialize on
	// SQLite's own write lock, which busyTimeoutMS absorbs.
	maxOpenConns = 4

	// busyTimeoutMS is how long a connection waits for a lock before reporting
	// SQLITE_BUSY.
	//
	// Two relicta processes in one repository is a normal CI situation — a workflow
	// running `relicta plan` while a webhook handler reads status — and the default
	// busy timeout is zero, which turns a lock held for a millisecond into a failed
	// command. Five seconds is long enough to cover any write this store makes (a
	// single upsert of one document) and short enough that a genuinely stuck writer
	// still surfaces as an error rather than a hang.
	busyTimeoutMS = 5000

	// loadBatchChunk bounds how many run IDs go into one IN clause.
	//
	// SQLite has a per-statement host parameter limit, and LoadBatch is handed whatever
	// the caller has: `relicta report` over a year of history is thousands of IDs. Two
	// queries are cheaper than one error.
	loadBatchChunk = 500

	// dbFileName is the store's file inside .relicta/.
	dbFileName = "relicta.db"

	// relictaDir is the directory relicta owns in a repository.
	relictaDir = ".relicta"
)

// ErrInvalidDatabasePath is returned for a path this store cannot turn into a DSN.
var ErrInvalidDatabasePath = errors.New("invalid sqlite database path")

// Store implements ports.ReleaseRunRepository against one SQLite file.
//
// Safe for concurrent use: database/sql serializes access to pooled connections, and
// the pragmas set in Open make concurrent processes wait for each other rather than
// fail.
type Store struct {
	db   *sql.DB
	path string
}

// Ensure Store implements the full port, not a convenient subset of it.
var _ ports.ReleaseRunRepository = (*Store)(nil)

// DefaultPath returns where a repository's run database lives.
//
// `.relicta/relicta.db`, not the existing persistence.file_path default of
// `.relicta/events`. That setting names a *directory*, because it came from the event
// store design where each event was a file; a database is one file, and pointing a
// directory setting at it would be a category error that only shows up when something
// tries to list it. `.relicta/` is also the right parent for a second reason: WAL mode
// puts `-wal` and `-shm` files next to the database, so it has to live somewhere
// relicta owns and the repository ignores.
//
// Which path the CLI actually passes is a config question this package deliberately
// does not answer — Open takes a path, and wiring persistence.backend is a separate
// change.
func DefaultPath(repoRoot string) string {
	return filepath.Join(repoRoot, relictaDir, dbFileName)
}

// Open connects to the database at path, creating it and its parent directory if
// needed, and applies any pending migrations.
//
// Migrations run here rather than only under `relicta db migrate`. ADR-013's "migration
// is explicit" is about an operator's *data* — `relicta db import` never moves an audit
// trail behind their back — not about the schema of a file relicta created itself. A
// local database the user never provisioned should not need a second command before it
// works.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := openDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

// openDatabase connects to path, creating it and its parent directory if needed, and
// applies any pending migrations.
//
// Shared with OpenMemoryStore, because ADR-013 puts one backend behind
// persistence.backend rather than one per store: the release runs and the governance
// record belong in the same file, which is what makes writing both in one transaction
// possible at all. Two entry points opening two files with two pool configurations
// would give that up before it was ever tried.
func openDatabase(ctx context.Context, path string) (*sql.DB, error) {
	dsn, err := dataSourceName(path)
	if err != nil {
		return nil, err
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		// 0700 matches the file adapter's ensureDir: a release history names branches,
		// authors and unreleased changes, and it is not other users' business.
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating database directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database %s: %w", path, err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to sqlite database %s: %w", path, err)
	}

	if _, err := NewMigrator(db).Up(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating sqlite database %s: %w", path, err)
	}

	return db, nil
}

// Path returns the database file this store was opened on.
func (s *Store) Path() string { return s.path }

// Close releases the connection pool.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing sqlite database %s: %w", s.path, err)
	}
	return nil
}

// dataSourceName builds the driver DSN, which is where every concurrency decision is
// made because these pragmas are per connection and the pool opens connections lazily.
//
//   - WAL, so a reader and the writer do not exclude each other. Under the default
//     rollback journal, `relicta status` in one shell fails while `relicta publish`
//     commits in another.
//   - busy_timeout, so contention waits instead of erroring. See busyTimeoutMS.
//   - immediate transactions, so a write transaction takes the write lock at BEGIN.
//     This is the one that is not obvious: a deferred transaction that starts by
//     reading and later writes has to *upgrade*, and SQLite refuses an upgrade against
//     another writer with SQLITE_BUSY immediately — the busy handler cannot be used
//     there, because waiting could deadlock. Taking the lock up front turns that
//     unrecoverable error back into an ordinary wait.
//   - synchronous=NORMAL, which under WAL survives a process crash and can lose only
//     the most recent commits to a power cut. That is already stronger than the file
//     adapter, whose write-then-rename never fsyncs at all, and FULL would cost an
//     fsync on every state transition of every release.
func dataSourceName(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%w: empty path", ErrInvalidDatabasePath)
	}
	// The driver splits the DSN at the first '?' to find its parameters, so a path
	// containing one would be truncated into a different database. Refuse it rather
	// than silently opening the wrong file.
	if strings.ContainsRune(path, '?') {
		return "", fmt.Errorf("%w: path contains '?': %s", ErrInvalidDatabasePath, path)
	}

	params := url.Values{}
	params.Set("_journal_mode", "WAL")
	params.Set("_busy_timeout", fmt.Sprint(busyTimeoutMS))
	params.Set("_txlock", "immediate")
	params.Set("_synchronous", "NORMAL")
	return path + "?" + params.Encode(), nil
}

// normalizeRepoRoot canonicalizes a repository root for the repo_root column.
//
// The file adapter gets path equivalence for free: it joins repoRoot into a filename
// and the operating system resolves "..", a relative prefix and every symlink on the
// way, so all the spellings of one directory reach one file. A string column resolves
// nothing, and two spellings of one repository would be two repositories to this store
// — List returning empty for a history that is right there, which reads as data loss.
//
// So all three are undone here. Symlinks matter more than they look: /tmp is a symlink
// to /private/tmp on macOS, and a repository under one is reachable by two absolute
// paths that Clean cannot reconcile. EvalSymlinks is best effort because it needs the
// directory to exist, and unlike the file adapter's validateRepoRoot this deliberately
// does not require that — a database can hold the history of a working copy that has
// since been deleted, and reading it back should not depend on the filesystem.
func normalizeRepoRoot(repoRoot string) string {
	if repoRoot == "" {
		return ""
	}
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		abs = filepath.Clean(repoRoot)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// Save persists a run, inserting it or replacing the row it already has.
//
// One upsert rather than a delete and an insert: a run is saved at every state
// transition, so this is the common path, and a caller that saw the store between the
// two statements would see the release vanish.
func (s *Store) Save(ctx context.Context, run *domain.ReleaseRun) error {
	if run == nil {
		return errors.New("saving release run: no run given")
	}

	document, err := adapters.MarshalRun(run)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO release_runs
			(run_id, repo_root, state, plan_hash, created_at, updated_at, document)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			repo_root  = excluded.repo_root,
			state      = excluded.state,
			plan_hash  = excluded.plan_hash,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			document   = excluded.document`,
		string(run.ID()),
		normalizeRepoRoot(run.RepoRoot()),
		string(run.State()),
		run.PlanHash(),
		run.CreatedAt().UnixNano(),
		run.UpdatedAt().UnixNano(),
		string(document),
	)
	if err != nil {
		return fmt.Errorf("saving release run %s: %w", run.ID(), err)
	}
	return nil
}

// Load retrieves a run by ID, across every repository in the database.
//
// No repoRoot parameter, matching the port. The file adapter answers this by scanning
// the roots it happens to have touched in this process, which means the answer depends
// on what the process did earlier; here it is a primary key lookup, so a run saved by a
// previous invocation is found too.
func (s *Store) Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	var document string
	err := s.db.QueryRowContext(ctx,
		`SELECT document FROM release_runs WHERE run_id = ?`, string(runID),
	).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading release run %s: %w", runID, err)
	}
	return adapters.UnmarshalRun([]byte(document))
}

// LoadBatch retrieves several runs from one repository, skipping any it cannot find.
//
// Skipping rather than reporting is the contract: callers pass IDs from a listing that
// may have raced with a delete, and an absent run there is ordinary.
func (s *Store) LoadBatch(
	ctx context.Context, repoRoot string, runIDs []domain.RunID,
) (map[domain.RunID]*domain.ReleaseRun, error) {
	result := make(map[domain.RunID]*domain.ReleaseRun, len(runIDs))
	if len(runIDs) == 0 {
		return result, nil
	}

	root := normalizeRepoRoot(repoRoot)
	for start := 0; start < len(runIDs); start += loadBatchChunk {
		end := min(start+loadBatchChunk, len(runIDs))
		chunk := runIDs[start:end]

		args := make([]any, 0, len(chunk)+1)
		args = append(args, root)
		for _, id := range chunk {
			args = append(args, string(id))
		}

		query := `SELECT run_id, document FROM release_runs
			WHERE repo_root = ? AND run_id IN (` + placeholders(len(chunk)) + `)`

		if err := s.collectInto(ctx, result, query, args...); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// collectInto runs a run_id/document query and decodes each row into result.
func (s *Store) collectInto(
	ctx context.Context, result map[domain.RunID]*domain.ReleaseRun, query string, args ...any,
) error {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("loading release runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, document string
		if err := rows.Scan(&id, &document); err != nil {
			return fmt.Errorf("scanning release run: %w", err)
		}
		run, err := adapters.UnmarshalRun([]byte(document))
		if err != nil {
			// A row that will not decode is skipped, matching the file adapter's
			// treatment of a file that will not parse. LoadBatch's contract is that a
			// run it cannot produce is simply absent from the map.
			continue
		}
		result[domain.RunID(id)] = run
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating release runs: %w", err)
	}
	return nil
}

// LoadLatest returns the run the latest pointer names, or ErrRunNotFound.
//
// The join is what makes a dangling pointer behave like the file adapter's: deleting a
// run leaves the pointer naming it, and the next LoadLatest reports not-found rather
// than an error about a missing row. See the schema for why there is no foreign key.
func (s *Store) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT r.document
		FROM release_run_latest AS l
		JOIN release_runs AS r
			ON r.run_id = l.run_id AND r.repo_root = l.repo_root
		WHERE l.repo_root = ?`, normalizeRepoRoot(repoRoot),
	).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading latest release run for %s: %w", repoRoot, err)
	}
	return adapters.UnmarshalRun([]byte(document))
}

// SetLatest points a repository at a run.
//
// Accepts a run ID the store has never seen, because the file adapter does — it writes
// whatever string it is given into the `latest` file. LoadLatest is the place that
// decides such a pointer resolves to nothing.
func (s *Store) SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO release_run_latest (repo_root, run_id, set_at)
		VALUES (?, ?, unixepoch())
		ON CONFLICT(repo_root) DO UPDATE SET
			run_id = excluded.run_id,
			set_at = excluded.set_at`,
		normalizeRepoRoot(repoRoot), string(runID),
	)
	if err != nil {
		return fmt.Errorf("setting latest release run for %s: %w", repoRoot, err)
	}
	return nil
}

// List returns a repository's run IDs, newest first.
//
// Ordered by created_at, which is what the port documents. The file adapter sorts by
// file modification time, so its order is really "most recently saved" and a run
// reshuffles every time it advances a state — an approximation of creation order that
// its storage forced on it, not a decision. run_id breaks ties so the order is total:
// two runs created in the same nanosecond must not come back in a different order on
// each call, or a paginated history would repeat and skip entries.
func (s *Store) List(ctx context.Context, repoRoot string) ([]domain.RunID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id FROM release_runs
		WHERE repo_root = ?
		ORDER BY updated_at DESC, run_id DESC`, normalizeRepoRoot(repoRoot))
	if err != nil {
		return nil, fmt.Errorf("listing release runs for %s: %w", repoRoot, err)
	}
	defer func() { _ = rows.Close() }()

	var ids []domain.RunID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning run id: %w", err)
		}
		ids = append(ids, domain.RunID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run ids: %w", err)
	}
	return ids, nil
}

// Delete removes a run and reports ErrRunNotFound when there was none.
//
// Reporting rather than shrugging, because `relicta clean` can only say which runs it
// failed to remove if the repository tells it. The latest pointer is left alone even
// when it named this run: the file adapter leaves its pointer file behind too, and
// LoadLatest's join already treats that as not-found.
func (s *Store) Delete(ctx context.Context, runID domain.RunID) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM release_runs WHERE run_id = ?`, string(runID))
	if err != nil {
		return fmt.Errorf("deleting release run %s: %w", runID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("deleting release run %s: %w", runID, err)
	}
	if affected == 0 {
		return fmt.Errorf("deleting release run %s: %w", runID, domain.ErrRunNotFound)
	}
	return nil
}

// FindByState returns a repository's runs in one state.
//
// The comparison is SQLite's default BINARY collation, i.e. byte-exact, which is what
// `run.State() == state` does in the file adapter. A state nothing is in yields no
// rows rather than an error, so an unrecognized state reads as "no matches" on both
// backends.
func (s *Store) FindByState(
	ctx context.Context, repoRoot string, state domain.RunState,
) ([]*domain.ReleaseRun, error) {
	return s.queryRuns(ctx, `
		SELECT document FROM release_runs
		WHERE repo_root = ? AND state = ?
		ORDER BY updated_at DESC, run_id DESC`,
		normalizeRepoRoot(repoRoot), string(state))
}

// FindActive returns a repository's runs that are neither draft nor terminal.
//
// The set of active states is taken from the domain at call time rather than written
// into the SQL. domain.RunState.IsActive is the definition; a literal list here would
// be a copy of it, and the copy is what stays behind when a state is added — the query
// would keep working and quietly stop returning one kind of in-flight release.
func (s *Store) FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error) {
	states := activeStates()
	if len(states) == 0 {
		return nil, nil
	}

	args := make([]any, 0, len(states)+1)
	args = append(args, normalizeRepoRoot(repoRoot))
	for _, state := range states {
		args = append(args, string(state))
	}

	return s.queryRuns(ctx, `
		SELECT document FROM release_runs
		WHERE repo_root = ? AND state IN (`+placeholders(len(states))+`)
		ORDER BY updated_at DESC, run_id DESC`, args...)
}

// FindByPlanHash returns the run planned under a hash, or nil, nil when there is none.
//
// nil, nil and not an error: duplicate detection asks this before every plan, and an
// error would make "nothing was planned under this hash" indistinguishable from "the
// store is unreadable". Newest first with LIMIT 1 because the plan hash is not unique —
// re-planning the same commits produces the same hash — and the reference returns the
// first match in its newest-first listing.
func (s *Store) FindByPlanHash(
	ctx context.Context, repoRoot string, planHash string,
) (*domain.ReleaseRun, error) {
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT document FROM release_runs
		WHERE repo_root = ? AND plan_hash = ?
		ORDER BY updated_at DESC, run_id DESC
		LIMIT 1`, normalizeRepoRoot(repoRoot), planHash,
	).Scan(&document)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding release run by plan hash in %s: %w", repoRoot, err)
	}
	return adapters.UnmarshalRun([]byte(document))
}

// queryRuns runs a document-returning query and decodes every row.
func (s *Store) queryRuns(ctx context.Context, query string, args ...any) ([]*domain.ReleaseRun, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying release runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*domain.ReleaseRun
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, fmt.Errorf("scanning release run: %w", err)
		}
		run, err := adapters.UnmarshalRun([]byte(document))
		if err != nil {
			// Skipped rather than failing the whole query, matching the file adapter:
			// one unreadable run must not make the other nine invisible to `history`.
			continue
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating release runs: %w", err)
	}
	return runs, nil
}

// activeStates is domain.RunState.IsActive, enumerated so SQL can use it.
func activeStates() []domain.RunState {
	all := domain.AllStates()
	states := make([]domain.RunState, 0, len(all))
	for _, state := range all {
		if state.IsActive() {
			states = append(states, state)
		}
	}
	return states
}

// placeholders returns "?, ?, ..." for n bound parameters.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
