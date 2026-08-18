-- 003_create_governance_memory.up.sql
-- Creates the governance memory tables: the other half of ADR-013's system of record.
--
-- 002 stores the release run — what a release did. These store what the governance layer
-- concluded about it: the release record, the incidents that followed, the decisions and
-- the authorizations they granted. ADR-013 names both as the system of record, and this
-- is the backend for teams whose audit trail has to outlive one developer's checkout.
--
-- Where the column line is drawn, on the same rule 002 used: a field is a column when a
-- query filters, orders or keys on it, and payload otherwise. Checked against what the
-- fourteen Store methods actually ask. GetReleaseHistory and GetIncidentHistory filter on
-- repository and order by time; GetActorMetrics is derived from an actor's own rows, so
-- actor_id is a filter; decisions and authorizations are fetched by their own ID and by
-- the proposal or decision they hang off. Everything else a record carries — risk score,
-- outcome, breaking-change counts, tags, rationale, the approval chain — is read only
-- after a row has been selected, so promoting it would buy an index nothing uses and a
-- migration every time a record type grows a field.
--
-- Actor metrics have no table on purpose. They are derived from these rows on read,
-- through the same memory.RebuildActorMetrics the file store rebuilds with. A materialized
-- metrics row would be a read-modify-write on the one path this backend exists to let
-- several processes take at once, and a lost update there silently understates an actor's
-- failures — which is the number that decides whether their next change is auto-approved.

-- One release record per run, per repository.
CREATE TABLE IF NOT EXISTS governance_releases (
    -- repository leads the key because one database serves many, unlike the file
    -- backend's single memory.json per checkout. Every history query starts here.
    repository   TEXT        NOT NULL,
    release_id   TEXT        NOT NULL,

    -- Denormalized out of the payload because GetActorMetrics filters on it. It is the
    -- only field a query reads without having chosen a row first.
    actor_id     TEXT        NOT NULL DEFAULT '',

    released_at  TIMESTAMPTZ NOT NULL,

    -- Insertion order, to break ties in released_at.
    --
    -- The file store's history is its append order reversed, so records sharing a
    -- timestamp still come back in a fixed order. Ordering by released_at alone would
    -- leave that to the planner, and `relicta history` would list two releases published
    -- in the same instant differently on consecutive runs. Left alone by the upsert
    -- below, so a corrected record keeps the position the original had — again matching
    -- the file store, whose UpsertReleaseRecord replaces in place.
    recorded_seq BIGSERIAL   NOT NULL,

    payload      JSONB       NOT NULL,

    -- Keyed on the pair, so re-recording one run replaces it rather than appending a
    -- second copy. Two rows for one run would count the actor's release twice and inflate
    -- deployment frequency, which is the defect memory.UpsertReleaseRecord exists to stop;
    -- here the constraint makes it unrepresentable rather than merely handled.
    PRIMARY KEY (repository, release_id)
);

-- GetReleaseHistory: the whole query, ordering included.
CREATE INDEX IF NOT EXISTS idx_governance_releases_history
    ON governance_releases (repository, released_at DESC, recorded_seq DESC);

-- GetActorMetrics: an actor's releases are the population its metrics are derived from,
-- and they span repositories, so this index is deliberately not repository-scoped.
CREATE INDEX IF NOT EXISTS idx_governance_releases_actor
    ON governance_releases (actor_id);

-- Incidents, keyed the same way and for the same reason.
--
-- The file store appends incidents unconditionally, where it upserts releases. That
-- asymmetry does not survive a shared backend: an incident recorded twice — a retry, or
-- two processes reacting to one alert — counts twice against its actor's incident rate,
-- and reliability is scored on that rate. An incident ID names one incident.
CREATE TABLE IF NOT EXISTS governance_incidents (
    repository   TEXT        NOT NULL,
    incident_id  TEXT        NOT NULL,
    actor_id     TEXT        NOT NULL DEFAULT '',
    detected_at  TIMESTAMPTZ NOT NULL,
    recorded_seq BIGSERIAL   NOT NULL,
    payload      JSONB       NOT NULL,

    PRIMARY KEY (repository, incident_id)
);

-- GetIncidentHistory.
CREATE INDEX IF NOT EXISTS idx_governance_incidents_history
    ON governance_incidents (repository, detected_at DESC, recorded_seq DESC);

-- GetActorMetrics again: incidents contribute to the same metrics as releases.
CREATE INDEX IF NOT EXISTS idx_governance_incidents_actor
    ON governance_incidents (actor_id);

-- Governance decisions, the audit trail proper.
--
-- Not repository-scoped, because a GovernanceDecision does not carry a repository — it
-- hangs off a proposal. The file store keys them by ID alone and so does this.
CREATE TABLE IF NOT EXISTS governance_decisions (
    decision_id TEXT        NOT NULL PRIMARY KEY,

    -- GetDecisionsByProposal and GetAuditTrail both filter on it.
    proposal_id TEXT        NOT NULL DEFAULT '',

    -- GetAuditTrail derives the trail's CreatedAt and UpdatedAt from these, and orders
    -- the decisions it returns. The file store iterates a map, so its trail arrives in a
    -- different order every call; an audit record that will not read the same way twice
    -- is worth the column.
    decided_at  TIMESTAMPTZ NOT NULL,

    payload     JSONB       NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_governance_decisions_proposal
    ON governance_decisions (proposal_id, decided_at);

-- Execution authorizations, which hang off a decision the same way.
CREATE TABLE IF NOT EXISTS governance_authorizations (
    authorization_id TEXT        NOT NULL PRIMARY KEY,
    decision_id      TEXT        NOT NULL DEFAULT '',
    authorized_at    TIMESTAMPTZ NOT NULL,
    payload          JSONB       NOT NULL
);

-- No foreign key to governance_decisions, matching the file store, which stores an
-- authorization whether or not its decision was recorded. A constraint here would make a
-- caller that writes them in the other order start failing on a backend switch, and a
-- backend switch changing behavior is what the conformance suite exists to prevent.
CREATE INDEX IF NOT EXISTS idx_governance_authorizations_decision
    ON governance_authorizations (decision_id, authorized_at);
