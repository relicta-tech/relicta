-- 002_create_governance_memory.down.sql
-- Drops the governance memory schema.
--
-- This discards every release record, incident, decision and authorization, which is why
-- nothing calls it automatically. It exists so the up migration is testable as a
-- migration rather than as a schema that happens to have been applied once, and so an
-- operator who ran `relicta db migrate` against the wrong file can undo it.

DROP INDEX IF EXISTS idx_governance_authorizations_decision;
DROP INDEX IF EXISTS idx_governance_decisions_proposal;
DROP INDEX IF EXISTS idx_governance_incidents_actor;
DROP INDEX IF EXISTS idx_governance_incidents_repository;
DROP INDEX IF EXISTS idx_governance_releases_actor;
DROP INDEX IF EXISTS idx_governance_releases_repository;
DROP TABLE IF EXISTS governance_authorizations;
DROP TABLE IF EXISTS governance_decisions;
DROP TABLE IF EXISTS governance_incidents;
DROP TABLE IF EXISTS governance_releases;
