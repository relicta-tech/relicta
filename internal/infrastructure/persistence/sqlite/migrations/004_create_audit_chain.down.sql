-- 004_create_audit_chain.down.sql
-- Drops the audit chain.
--
-- This discards the evidence for every governance event the repository has recorded, and
-- unlike the records in 002 it cannot be rebuilt from anything: the chain's value is that
-- it was written at the time, so a reconstruction would be a new chain making the same
-- claims with none of the weight. Nothing calls this automatically. It exists so the up
-- migration is testable as a migration, and so an operator who migrated the wrong file
-- can undo it.

DROP INDEX IF EXISTS idx_governance_audit_entries_proposal;
DROP INDEX IF EXISTS idx_governance_audit_entries_chain;
DROP TABLE IF EXISTS governance_audit_entries;
