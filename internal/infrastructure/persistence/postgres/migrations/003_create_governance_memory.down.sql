-- 003_create_governance_memory.down.sql
-- Drops the governance memory tables. The release runs of 002 and the event log of 001
-- are untouched: rolling back the governance record must not take the runs with it.

DROP TABLE IF EXISTS governance_authorizations;
DROP TABLE IF EXISTS governance_decisions;
DROP TABLE IF EXISTS governance_incidents;
DROP TABLE IF EXISTS governance_releases;
