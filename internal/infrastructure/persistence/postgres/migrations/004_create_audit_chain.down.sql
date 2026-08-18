-- 004_create_audit_chain.down.sql
-- Drops the audit chain.
--
-- This discards the evidence for every governance event recorded against this database,
-- and unlike the records in 003 it cannot be rebuilt from anything: the chain's value is
-- that it was written at the time, so a reconstruction would be a new chain making the
-- same claims with none of the weight. Nothing calls this automatically.

DROP INDEX IF EXISTS idx_governance_audit_entries_proposal;
DROP INDEX IF EXISTS idx_governance_audit_entries_chain;
DROP TABLE IF EXISTS governance_audit_entries;
