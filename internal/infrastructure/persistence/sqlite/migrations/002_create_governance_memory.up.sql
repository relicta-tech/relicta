-- 002_create_governance_memory.up.sql
-- Creates the governance memory tables: releases, incidents, decisions, authorizations.
--
-- The second half of ADR-013's system of record. `.relicta/governance/memory.json` is
-- one JSON document holding every release, incident, decision and authorization the
-- tool has ever seen, rewritten in full on each write; these are the same records as
-- rows, in the same database file as the release runs, so that a run and the governance
-- record it produces can eventually be written in one transaction. Two files cannot
-- offer that, and "the two records disagree" is the failure ADR-013 is about.
--
-- There is no actor metrics table. The file store keeps a materialized ActorMetrics per
-- actor because a map is all it can query, and it has to rebuild that projection
-- whenever a record is replaced — Accumulate keeps a running average, which cannot be
-- un-added. Here the releases and incidents rows are the facts and the metrics are
-- derived from them on read (see MemoryStore.GetActorMetrics), so a re-recorded release
-- cannot be counted twice: there is no second copy to fall out of step. The arithmetic
-- is still memory.RebuildActorMetrics, not SQL — AVG() over risk_score would be a
-- fourth definition of an actor's reputation, and it would quietly include the canceled
-- runs that ReleaseOutcome.CountsAsRelease exists to exclude.
--
-- STRICT everywhere, for the reason 001 gives: a declared type that the database does
-- not enforce leaves the adapter as the only thing between a mistyped column and the
-- domain.

-- One row per release record.
--
-- recorded_seq is the order records arrived, and it is what GetReleaseHistory orders by.
-- Not released_at: the reference returns its slice in reverse insertion order, and a
-- backdated record — an import, a hub sync, a release whose ReleasedAt is zero because
-- it is unknown — sorts to a different place under the two rules. The release-run store
-- learned this the expensive way (see the ordering note in 001): the conformance suite
-- caught one backend answering [older, newer] and the other [newer, older] for the same
-- history. Changing what `relicta history` shows is a decision to take once, for all
-- three adapters, not a side effect of choosing a backend.
--
-- So released_at gets no column. It would order nothing and filter nothing — the Store
-- interface has no method that takes a time range — and an unused duplicate of a field
-- already in the document is just a second place for it to be wrong.
CREATE TABLE IF NOT EXISTS governance_releases (
    recorded_seq INTEGER PRIMARY KEY AUTOINCREMENT,

    -- "owner/repo", not a filesystem path. Deliberately not normalized the way
    -- release_runs.repo_root is: that column holds a directory, where two spellings of
    -- one path are one repository, and this one holds an identifier the caller chose.
    -- Canonicalizing it would invent a repository the caller never named.
    repository   TEXT NOT NULL,

    -- Unique per repository rather than globally, because that is the scope in which
    -- the reference deduplicates: UpsertReleaseRecord scans the slice held under one
    -- repository key. A run ID is derived from a plan hash that already includes the
    -- repository, so a collision across two repositories is not expected — but if one
    -- happened, a global key would silently merge two projects' releases into one.
    release_id   TEXT NOT NULL,

    -- Every actor metric is scoped by this, and deriving them means selecting on it.
    actor_id     TEXT NOT NULL,

    document     TEXT NOT NULL,

    UNIQUE (repository, release_id)
) STRICT;

-- GetReleaseHistory: one repository, newest first, with a limit.
CREATE INDEX IF NOT EXISTS idx_governance_releases_repository
    ON governance_releases (repository, recorded_seq DESC);

-- GetActorMetrics and UpdateActorMetrics, which read one actor's releases across every
-- repository — an actor's record follows the actor, not the project.
CREATE INDEX IF NOT EXISTS idx_governance_releases_actor
    ON governance_releases (actor_id, recorded_seq);

-- One row per incident record.
--
-- No unique constraint on (repository, incident_id), which looks like an omission and is
-- not: the reference appends every incident it is given, so recording one twice leaves
-- two in its history. Deduplicating here would make the same two calls produce two
-- incidents on the file backend and one on this one. Releases got an explicit
-- UpsertReleaseRecord, with a comment about the duplicates it was written to stop;
-- incidents never did, and inventing that rule in one adapter is how the backends come
-- to disagree about what happened.
CREATE TABLE IF NOT EXISTS governance_incidents (
    recorded_seq INTEGER PRIMARY KEY AUTOINCREMENT,
    repository   TEXT NOT NULL,
    incident_id  TEXT NOT NULL,

    -- Incidents count toward the actor's metrics, so deriving those selects on this.
    -- Empty for an incident recorded without one, which is a value and not a null:
    -- an actor ID of "" matches nothing rather than behaving like SQL's unknown.
    actor_id     TEXT NOT NULL,

    document     TEXT NOT NULL
) STRICT;

-- GetIncidentHistory.
CREATE INDEX IF NOT EXISTS idx_governance_incidents_repository
    ON governance_incidents (repository, recorded_seq DESC);

-- The incident half of an actor's derived metrics.
CREATE INDEX IF NOT EXISTS idx_governance_incidents_actor
    ON governance_incidents (actor_id, recorded_seq);

-- One row per governance decision.
--
-- decided_at earns its column here where released_at did not, because something does
-- order by it: the reference answers GetDecisionsByProposal and GetAuditTrail by
-- iterating a Go map, which is randomized on purpose, so no caller can depend on the
-- order it gets today. That makes a deterministic order free to choose, and an audit
-- trail is the one place where "in the order the decisions were made" is the answer a
-- reader wants. It is also where AuditTrail.CreatedAt comes from.
CREATE TABLE IF NOT EXISTS governance_decisions (
    decision_id TEXT    NOT NULL PRIMARY KEY,
    proposal_id TEXT    NOT NULL,

    -- Unix nanoseconds, matching release_runs: RFC 3339 text trims trailing zeros from
    -- the fraction, so "…:00.1Z" sorts after "…:00.12Z" as a string. The authoritative
    -- timestamp, with its offset, stays in the document.
    decided_at  INTEGER NOT NULL,

    document    TEXT    NOT NULL
) STRICT;

-- GetDecisionsByProposal and GetAuditTrail, both ordered. decision_id is in the index so
-- two decisions made in the same nanosecond still come back in one fixed order.
CREATE INDEX IF NOT EXISTS idx_governance_decisions_proposal
    ON governance_decisions (proposal_id, decided_at, decision_id);

-- One row per execution authorization. Same reasoning as decisions.
--
-- No foreign key to governance_decisions, for the reason 001 gives for the latest
-- pointer: the reference stores an authorization whatever decision it names, and
-- GetAuthorizationsByDecision simply finds nothing for a decision that was never
-- recorded. A foreign key would turn a recoverable gap in an audit trail into a write
-- that fails, and an operator would meet the difference only after switching backend.
CREATE TABLE IF NOT EXISTS governance_authorizations (
    authorization_id TEXT    NOT NULL PRIMARY KEY,
    decision_id      TEXT    NOT NULL,
    authorized_at    INTEGER NOT NULL,
    document         TEXT    NOT NULL
) STRICT;

-- GetAuthorizationsByDecision and the authorization half of an audit trail.
CREATE INDEX IF NOT EXISTS idx_governance_authorizations_decision
    ON governance_authorizations (decision_id, authorized_at, authorization_id);
