-- 002_create_release_runs.up.sql
-- Creates the release run tables: the system of record under ADR-013.
--
-- This sits beside `events` rather than replacing it. ADR-013 keeps the event log as a
-- publication mechanism for webhooks and subscribers, and makes the run the truth. Three
-- copies that can disagree are worse than one that cannot, so 001_create_events is left
-- exactly as it was.
--
-- Where the column line is drawn: a field is a column when a query filters, orders or
-- keys on it, and payload otherwise. FindByState, FindActive, FindByPlanHash and every
-- repoRoot-scoped call are the queries that exist, which makes run_id, repo_root, state
-- and plan_hash columns; List's ordering makes updated_at one. Everything else the
-- aggregate carries — commits, changeset, steps, history, approval, notes — is read only
-- after a run has been selected, so promoting it to a column would buy an index nothing
-- uses and a migration every time the aggregate grows a field.

CREATE TABLE IF NOT EXISTS release_runs (
    -- repo_root leads the key because one database serves many repositories, unlike the
    -- file backend's one directory per repo. Every scoped query starts here.
    repo_root  TEXT        NOT NULL,
    run_id     TEXT        NOT NULL,

    state      TEXT        NOT NULL,

    -- Deliberately not unique, even per repository. A run ID is derived from the plan
    -- hash, but the hash is recomputed when a version is set, and runs planned before
    -- anything was versioned share the empty hash. A unique constraint here would refuse
    -- the second run in a fresh repository.
    plan_hash  TEXT        NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    -- The full run, in the same shape the file backend writes to
    -- .relicta/releases/<id>.json. JSONB rather than TEXT so an operator can inspect a
    -- run without a Go program; the adapter itself never queries inside it.
    payload    JSONB       NOT NULL,

    PRIMARY KEY (repo_root, run_id)
);

-- Load and Delete are given a run ID and no repository, so they cannot use the primary
-- key's leading column. Without this index they degrade into the sequential scan ADR-013
-- exists to remove.
CREATE INDEX IF NOT EXISTS idx_release_runs_run_id ON release_runs (run_id);

-- FindByState, and FindActive, which is FindByState over the non-terminal states.
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_state ON release_runs (repo_root, state);

-- FindByPlanHash: duplicate detection asks this before every plan.
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_plan_hash ON release_runs (repo_root, plan_hash);

-- List, and every query that inherits its newest-first ordering.
CREATE INDEX IF NOT EXISTS idx_release_runs_repo_updated_at
    ON release_runs (repo_root, updated_at DESC);

-- The latest pointer, one row per repository.
--
-- A separate table rather than a flag on release_runs, and with no foreign key, because
-- the file backend's pointer is a name and not a reference: SetLatest writes the file
-- whether or not the run exists, and LoadLatest reports the run missing afterwards. A
-- caller that sets the pointer before saving the run works on the file backend today,
-- and a foreign key would make that same sequence fail on this one — a backend switch
-- changing behavior is the thing the conformance suite exists to prevent.
CREATE TABLE IF NOT EXISTS release_run_latest (
    repo_root  TEXT        NOT NULL PRIMARY KEY,
    run_id     TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
