-- 002_create_release_runs.down.sql
-- Drops the release run tables and their indexes. The events table is untouched:
-- rolling back the system of record must not take the event log with it.

DROP TABLE IF EXISTS release_run_latest;
DROP TABLE IF EXISTS release_runs;
