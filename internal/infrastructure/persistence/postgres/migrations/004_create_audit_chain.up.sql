-- 004_create_audit_chain.up.sql
-- Creates the governance audit chain: the append-only, hash-linked half of the record.
--
-- 003 stores what the governance layer concluded, as upserts, so a later run can correct
-- a record it already wrote. That is right for a record and wrong for evidence: a hash
-- chain computed over rows that can be rewritten re-links around every edit and reports
-- itself intact. These rows are written once, at the moment each event happens, and
-- nothing in the adapter ever updates one.
--
-- This is the backend where the chain matters most and is hardest to get right. A shared
-- database has several writers, and the whole guarantee is that entry N names entry
-- N-1's hash — so two writers who both read the same tail and both insert would produce
-- a fork: two chains that each verify and disagree about what happened. The append is a
-- serializable transaction in the adapter, and the constraints below are what make the
-- losing writer fail rather than fork.

CREATE TABLE IF NOT EXISTS governance_audit_entries (
    -- Append order, and the only order the chain can be read in, because each entry
    -- hashes its predecessor's hash. Not recorded_at: two entries in one microsecond, or
    -- two application servers whose clocks disagree, would reorder the chain into one
    -- that reports itself corrupt.
    seq           BIGSERIAL   PRIMARY KEY,

    -- Governance identity ("owner/repo"). One database serves many checkouts and their
    -- chains must not interleave: entries from two repositories in one chain would each
    -- link through the other's, so reading either alone would find a broken chain.
    repository    TEXT        NOT NULL,

    entry_id      TEXT        NOT NULL,

    -- The release run the event belongs to, so one release's trail is a query.
    proposal_id   TEXT        NOT NULL DEFAULT '',

    event_type    TEXT        NOT NULL,

    -- Lifted out of the payload because the append precondition compares them without
    -- decoding anything. Empty for the genesis entry, which is a value and not a null:
    -- SQL's unknown compares equal to nothing, including to the empty predecessor a
    -- first entry legitimately has, so a nullable column would make every genesis append
    -- fail its own check.
    entry_hash    TEXT        NOT NULL,
    previous_hash TEXT        NOT NULL,

    recorded_at   TIMESTAMPTZ NOT NULL,

    -- The whole entry as it was hashed. The columns above are extracted from it for
    -- querying; this is what Verify recomputes against.
    payload       JSONB       NOT NULL,

    -- A transition already recorded is refused rather than recorded twice.
    UNIQUE (repository, entry_id),

    -- The fork guard, as a constraint rather than only as adapter logic. Two concurrent
    -- appends that both read the same tail produce the same previous_hash, and the
    -- second one violates this — so a fork is impossible even if a future caller
    -- forgets the transaction. NULLS NOT DISTINCT is unnecessary because previous_hash
    -- is never null; the genesis entry's empty string is a real value, and a second
    -- genesis entry for one repository collides with the first exactly as it should.
    UNIQUE (repository, previous_hash)
);

-- Reading a repository's chain, and finding its tail.
CREATE INDEX IF NOT EXISTS idx_governance_audit_entries_chain
    ON governance_audit_entries (repository, seq);

-- The per-release audit trail.
CREATE INDEX IF NOT EXISTS idx_governance_audit_entries_proposal
    ON governance_audit_entries (repository, proposal_id, seq);
