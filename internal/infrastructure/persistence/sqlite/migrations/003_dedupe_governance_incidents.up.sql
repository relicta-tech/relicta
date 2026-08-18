-- 003_dedupe_governance_incidents.up.sql
--
-- Makes an incident ID name one incident per repository, which is what the conformance suite
-- now pins for every backend.
--
-- 002 gave governance_incidents an autoincrement key and no uniqueness, matching the file
-- store, which appended incidents unconditionally while upserting releases — an asymmetry
-- inside one implementation rather than a decision. A retried incident, or two processes
-- reacting to one alert, left two rows and counted twice against the actor's incident rate,
-- which feeds ReliabilityScore and the autonomy budget.
--
-- 002 is not edited: it has shipped to anyone who ran it, and a migration that changes after
-- being applied is a schema nobody can reason about.

-- Existing duplicates have to go before the index can exist. The newest row wins, on the same
-- rule the upsert now applies: a later record of one incident is a correction of it.
DELETE FROM governance_incidents
WHERE recorded_seq NOT IN (
    SELECT MAX(recorded_seq)
    FROM governance_incidents
    GROUP BY repository, incident_id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_governance_incidents_identity
    ON governance_incidents (repository, incident_id);
