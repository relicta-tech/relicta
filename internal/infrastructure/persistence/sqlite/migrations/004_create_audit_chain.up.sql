-- 004_create_audit_chain.up.sql
-- Creates the governance audit chain: the append-only, hash-linked half of the record.
--
-- 002 stores what the governance layer concluded — a decision, an authorization — and
-- stores it as an upsert, so a later run can correct a record it already wrote. That is
-- right for a record and wrong for evidence: a hash chain computed over rows that can be
-- rewritten re-links around every edit and reports itself intact. These rows are written
-- once, at the moment each event happens, and nothing in the adapter ever updates one.
--
-- The chain is here rather than in a file of its own for the reason ADR-013 gives for
-- everything else being here: one backend behind one setting. An operator who moves to
-- sqlite must not move their decisions and leave their evidence in memory.json.

CREATE TABLE IF NOT EXISTS governance_audit_entries (
    -- Append order, and the only order the chain can be read in. Each entry hashes its
    -- predecessor's hash, so a chain returned in any other order does not verify —
    -- which makes this column part of the integrity guarantee and not a convenience.
    -- Not recorded_at: two entries in one millisecond, or a clock that stepped
    -- backwards between two `relicta` invocations, would reorder the chain into one
    -- that reports itself corrupt.
    seq           INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Governance identity ("owner/repo"), the same key governance_releases uses. One
    -- database serves several checkouts and their chains must not interleave: entries
    -- from two repositories in one chain would each link through the other's, so
    -- reading either repository alone would find a broken chain.
    repository    TEXT NOT NULL,

    -- Unique per repository, which is the scope the chain is. An ID already present is
    -- a replay of a transition that is already evidence, and the adapter refuses it
    -- rather than recording the same event twice.
    entry_id      TEXT NOT NULL,

    -- The release run the event belongs to, so an audit trail for one release is a
    -- query rather than a scan. Named for the CGP field it carries (Entry.ProposalID).
    proposal_id   TEXT NOT NULL,

    event_type    TEXT NOT NULL,

    -- This entry's hash and its predecessor's, lifted out of the document because the
    -- append precondition reads them without decoding anything: an append is refused
    -- unless previous_hash equals the current tail's entry_hash. Empty for the genesis
    -- entry, which is a value and not a null — SQL's unknown compares equal to nothing,
    -- including to the empty predecessor a first entry legitimately has.
    entry_hash    TEXT NOT NULL,
    previous_hash TEXT NOT NULL,

    -- The whole entry as it was hashed. The columns above are extracted from it for
    -- querying; this is what Verify recomputes against, so it is the authority and they
    -- are the index.
    document      TEXT NOT NULL,

    UNIQUE (repository, entry_id)
) STRICT;

-- Reading a repository's chain, and finding its tail: both are this index, forward for
-- the whole chain and backward with a limit for the tail.
CREATE INDEX IF NOT EXISTS idx_governance_audit_entries_chain
    ON governance_audit_entries (repository, seq);

-- The per-release audit trail.
CREATE INDEX IF NOT EXISTS idx_governance_audit_entries_proposal
    ON governance_audit_entries (repository, proposal_id, seq);
