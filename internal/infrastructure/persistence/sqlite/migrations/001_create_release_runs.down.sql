-- 001_create_release_runs.down.sql
-- Drops the release run schema.
--
-- This discards every stored run, which is why nothing calls it automatically. It
-- exists so the up migration is testable as a migration rather than as a schema that
-- happens to be applied once, and so an operator who ran `relicta db migrate` against
-- the wrong file can undo it.

DROP INDEX IF EXISTS idx_release_runs_repo_root_plan_hash;
DROP INDEX IF EXISTS idx_release_runs_repo_root_state;
DROP INDEX IF EXISTS idx_release_runs_repo_root_updated_at;
DROP TABLE IF EXISTS release_run_latest;
DROP TABLE IF EXISTS release_runs;
