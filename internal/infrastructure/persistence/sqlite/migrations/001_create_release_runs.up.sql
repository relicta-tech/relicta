-- 001_create_release_runs.up.sql
-- Creates the release run table, its query indexes, and the latest pointer.
--
-- Not a copy of the postgres 001_create_events schema. ADR-013 decides that the system
-- of record is the release run, not an event log; wiring the event table here would add
-- a third copy of the truth beside the runs and the governance record, and three copies
-- that can disagree are worse than one that cannot.

-- The run aggregate, stored as one row.
--
-- `document` holds the whole aggregate as the JSON the file adapter writes (see
-- adapters.MarshalRun). The columns beside it are a projection of fields the port
-- queries on, duplicated out of the document so the query planner can reach them:
-- ADR-013's whole point is that FindByState stops meaning "load every run and filter in
-- Go". A field earns a column when a port method filters, scopes or orders by it, and
-- nothing else does — versions, notes, approvals, step status and history are read only
-- after a run is already in hand, so promoting them would buy nothing and add a second
-- place for each to be wrong.
--
-- STRICT because the type declarations should mean something. Without it SQLite would
-- accept a run whose state was stored as an integer and hand it back as one, and the
-- adapter would be the only thing standing between that and the domain.
CREATE TABLE IF NOT EXISTS release_runs (
    -- The aggregate identity. PRIMARY KEY rather than (repo_root, run_id) because the
    -- port's Load takes a run ID and no repository root, so the ID has to identify one
    -- run for that call to have an answer at all. It does: domain.NewReleaseRun derives
    -- the ID from the plan hash, which hashes the repo ID with the base ref and commits.
    run_id     TEXT    NOT NULL PRIMARY KEY,

    -- The repository this run belongs to, stored absolute and cleaned. Every other
    -- method on the port is scoped by it, and one database file can therefore serve
    -- several working copies — a `relicta` on a laptop that releases three repositories
    -- keeps one store, not three.
    repo_root  TEXT    NOT NULL,

    -- Compared with BINARY collation, i.e. case sensitively, which is what Go's == does
    -- in the file adapter. A NOCASE column here would make one backend match "Planned"
    -- against "planned" and the other not.
    state      TEXT    NOT NULL,

    -- Duplicate detection asks for this before every plan. Empty for runs that were
    -- never planned, which is a value and not a null: FindByPlanHash("") should behave
    -- the same everywhere rather than depending on SQL's three-valued logic.
    plan_hash  TEXT    NOT NULL,

    -- Unix nanoseconds, not RFC 3339 text. The port orders List by creation time, and
    -- RFC3339Nano trims trailing zeros from the fraction, so "…:00.1Z" sorts after
    -- "…:00.12Z" as text. An integer cannot get that wrong. The authoritative
    -- timestamps, with their offsets, stay in the document.
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,

    document   TEXT    NOT NULL
) STRICT;

-- List and every Find, which order most-recently-saved first within one repository.
--
-- updated_at, not created_at. A column can hold the real creation time and the file adapter
-- only approximates it with the run file's modification time, so ordering by creation looks
-- like the better answer and is what the port's comment used to claim. It was written that
-- way here first, and the conformance suite caught it: the file backend returned
-- [older, newer] and this one [newer, older] for the same two runs, which is one repository
-- with two histories depending on a config key. Matching the reference is what makes the
-- backends interchangeable; changing what `relicta history` shows is a separate decision,
-- taken once, with all three adapters moving together.
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_root_updated_at
    ON release_runs (repo_root, updated_at DESC, run_id DESC);

-- FindByState and FindActive.
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_root_state
    ON release_runs (repo_root, state);

-- FindByPlanHash.
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_root_plan_hash
    ON release_runs (repo_root, plan_hash);

-- The latest pointer: one row per repository, replacing the `latest` file.
--
-- A table rather than a column on release_runs, because "which run is latest" is a
-- property of the repository and not of any run. As a boolean column it would be an
-- invariant the schema cannot state — nothing stops two rows in one repo_root from
-- claiming it — and every SetLatest would be two writes that have to agree. As a row
-- keyed by repo_root it is unrepresentable for a repository to have two.
--
-- Deliberately no foreign key to release_runs. The file adapter writes the pointer file
-- whatever it names, and leaves it dangling when the run is deleted; LoadLatest then
-- reports not-found. A foreign key would turn the first into a write error and the
-- second into a cascade, and both are behavior changes an operator would meet only
-- after switching backend. The join in LoadLatest reproduces the reference exactly.
CREATE TABLE IF NOT EXISTS release_run_latest (
    repo_root TEXT    NOT NULL PRIMARY KEY,
    run_id    TEXT    NOT NULL,
    set_at    INTEGER NOT NULL
) STRICT;
