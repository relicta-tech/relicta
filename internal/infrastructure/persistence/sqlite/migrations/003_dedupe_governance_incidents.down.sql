-- 003_dedupe_governance_incidents.down.sql
--
-- Drops the uniqueness constraint. The rows removed on the way up are not restored: they were
-- duplicate records of one incident, and the information they held is in the row that survived.
DROP INDEX IF EXISTS idx_governance_incidents_identity;
