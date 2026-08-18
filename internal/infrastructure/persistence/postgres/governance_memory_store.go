package postgres

// governance_memory_store.go implements cgpmemory.Store against PostgreSQL: the
// governance record of ADR-013's system of record, for teams whose audit trail has to
// outlive any one developer's working copy.
//
// The file store keeps the whole of governance memory in one memory.json, read into maps
// at startup and rewritten in full on every write. That is fine for one process and one
// checkout, and it is exactly what a shared backend cannot do — two developers approving
// two releases would each write the file they loaded, and the second would erase the
// first. Here every write is one statement against one row, so concurrent writers are the
// ordinary case rather than the losing one.
//
// What this file does not do is decide who uses it. `persistence.backend` still resolves
// to `file` for governance memory; ADR-013 flips the default on evidence, and the
// evidence is the conformance suite passing, not the adapter existing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// GovernanceMemoryStore stores CGP governance memory in PostgreSQL.
//
// Safe for concurrent use: it holds no mutable state, and pgxpool is itself concurrent.
// The file store needs a mutex around an in-process cache; this one has no cache to
// protect, because the answer to every read is a query.
type GovernanceMemoryStore struct {
	pool *pgxpool.Pool
}

// NewGovernanceMemoryStore creates a store over an existing pool.
//
// The pool is borrowed, not owned — the caller closes it. Sharing the pool that already
// serves the release runs is the point: ADR-013 wants a run and the governance record it
// produces writable in one transaction, and two pools cannot share a transaction.
func NewGovernanceMemoryStore(pool *pgxpool.Pool) *GovernanceMemoryStore {
	return &GovernanceMemoryStore{pool: pool}
}

// Ensure GovernanceMemoryStore implements the port in full.
var _ cgpmemory.Store = (*GovernanceMemoryStore)(nil)

// RecordRelease stores a release record, replacing the one already under its ID.
//
// One upsert, no transaction, last writer wins — the same shape as the run repository's
// Save, and for the same reason: this backend exists so several processes share state, so
// two of them recording one release is normal traffic. A retried publish, or the outcome
// tracker and the CLI both reporting a single publish, must leave one record and not two.
//
// memory.UpsertReleaseRecord is the shared definition of that rule, and it is deliberately
// not called here. It replaces within a Go slice and reports the record it displaced so
// its caller can rebuild the actor metrics the replacement invalidated. Using it would
// mean loading a repository's whole history into memory, mutating it and writing it all
// back, which is the file backend's algorithm and the cost ADR-013 exists to remove. The
// constraint on (repository, release_id) enforces the same rule in the database, and the
// displaced record has no consumer here because the metrics are derived rather than
// accumulated — see GetActorMetrics.
func (s *GovernanceMemoryStore) RecordRelease(ctx context.Context, record *cgpmemory.ReleaseRecord) error {
	if record == nil {
		return fmt.Errorf("record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("record ID is required")
	}
	if record.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling release record %s: %w", record.ID, err)
	}

	// recorded_seq is left out of the update so a corrected record keeps the position
	// its original had, which is what replacing in place means on the file store.
	_, err = s.pool.Exec(ctx, `
		INSERT INTO governance_releases
			(repository, release_id, actor_id, released_at, payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (repository, release_id) DO UPDATE SET
			actor_id    = EXCLUDED.actor_id,
			released_at = EXCLUDED.released_at,
			payload     = EXCLUDED.payload`,
		record.Repository, record.ID, record.Actor.ID, record.ReleasedAt, payload,
	)
	if err != nil {
		return fmt.Errorf("recording release %s: %w", record.ID, err)
	}
	return nil
}

// RecordIncident stores an incident record, replacing the one already under its ID.
//
// The file store appends here rather than upserting, which is an asymmetry with its own
// handling of releases and not a behavior any caller reads: an incident ID names one
// incident. Keeping the append would let a retry, or two processes reacting to one alert,
// leave two rows — and every incident row counts against its actor's incident rate, which
// reliability is scored on. So a shared backend has to collapse them.
func (s *GovernanceMemoryStore) RecordIncident(ctx context.Context, incident *cgpmemory.IncidentRecord) error {
	if incident == nil {
		return fmt.Errorf("incident is required")
	}
	if incident.ID == "" {
		return fmt.Errorf("incident ID is required")
	}
	if incident.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	payload, err := json.Marshal(incident)
	if err != nil {
		return fmt.Errorf("marshaling incident %s: %w", incident.ID, err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO governance_incidents
			(repository, incident_id, actor_id, detected_at, payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (repository, incident_id) DO UPDATE SET
			actor_id    = EXCLUDED.actor_id,
			detected_at = EXCLUDED.detected_at,
			payload     = EXCLUDED.payload`,
		incident.Repository, incident.ID, incident.ActorID, incident.DetectedAt, payload,
	)
	if err != nil {
		return fmt.Errorf("recording incident %s: %w", incident.ID, err)
	}

	// No actor bookkeeping to do. The file store increments a materialized IncidentCount
	// here, and only when the actor already has metrics — so an incident recorded before
	// the actor's first release never reaches their record at all. Deriving the count
	// from these rows has no such ordering hazard.
	return nil
}

// RecordDecision stores a governance decision.
func (s *GovernanceMemoryStore) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	if decision == nil {
		return fmt.Errorf("decision is required")
	}
	if decision.ID == "" {
		return fmt.Errorf("decision ID is required")
	}

	payload, err := json.Marshal(decision)
	if err != nil {
		return fmt.Errorf("marshaling decision %s: %w", decision.ID, err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO governance_decisions (decision_id, proposal_id, decided_at, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (decision_id) DO UPDATE SET
			proposal_id = EXCLUDED.proposal_id,
			decided_at  = EXCLUDED.decided_at,
			payload     = EXCLUDED.payload`,
		decision.ID, decision.ProposalID, decision.Timestamp, payload,
	)
	if err != nil {
		return fmt.Errorf("recording decision %s: %w", decision.ID, err)
	}
	return nil
}

// RecordAuthorization stores an execution authorization.
func (s *GovernanceMemoryStore) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	if auth == nil {
		return fmt.Errorf("authorization is required")
	}
	if auth.ID == "" {
		return fmt.Errorf("authorization ID is required")
	}

	payload, err := json.Marshal(auth)
	if err != nil {
		return fmt.Errorf("marshaling authorization %s: %w", auth.ID, err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO governance_authorizations
			(authorization_id, decision_id, authorized_at, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (authorization_id) DO UPDATE SET
			decision_id   = EXCLUDED.decision_id,
			authorized_at = EXCLUDED.authorized_at,
			payload       = EXCLUDED.payload`,
		auth.ID, auth.DecisionID, auth.Timestamp, payload,
	)
	if err != nil {
		return fmt.Errorf("recording authorization %s: %w", auth.ID, err)
	}
	return nil
}

// GetReleaseHistory returns a repository's release records, most recent first.
//
// A repository nothing was recorded against is an empty slice and no error. That is the
// ordinary starting point rather than a failure — every report runs against one at least
// once — and it is the half of the contract's asymmetry that does not error, the other
// being GetActorMetrics.
func (s *GovernanceMemoryStore) GetReleaseHistory(
	ctx context.Context, repository string, limit int,
) ([]*cgpmemory.ReleaseRecord, error) {
	if limit <= 0 {
		// A limit of zero returns nothing rather than everything, which is what the
		// reference does and the opposite of how "no limit" is usually spelled. SQL
		// agrees for zero; it does not agree for a negative, where PostgreSQL raises
		// "LIMIT must not be negative" and the file store panics building a slice with a
		// negative capacity. Neither is an answer, so both collapse into the pinned one.
		return []*cgpmemory.ReleaseRecord{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_releases
		WHERE repository = $1
		ORDER BY released_at DESC, recorded_seq DESC
		LIMIT $2`,
		repository, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("reading release history for %s: %w", repository, err)
	}

	return scanPayloads[cgpmemory.ReleaseRecord](rows, "release record")
}

// GetIncidentHistory returns a repository's incident records, most recent first.
func (s *GovernanceMemoryStore) GetIncidentHistory(
	ctx context.Context, repository string, limit int,
) ([]*cgpmemory.IncidentRecord, error) {
	if limit <= 0 {
		return []*cgpmemory.IncidentRecord{}, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_incidents
		WHERE repository = $1
		ORDER BY detected_at DESC, recorded_seq DESC
		LIMIT $2`,
		repository, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("reading incident history for %s: %w", repository, err)
	}

	return scanPayloads[cgpmemory.IncidentRecord](rows, "incident record")
}

// GetDecision returns a governance decision by ID.
func (s *GovernanceMemoryStore) GetDecision(
	ctx context.Context, decisionID string,
) (*cgp.GovernanceDecision, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx,
		`SELECT payload FROM governance_decisions WHERE decision_id = $1`, decisionID,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Not found, distinctly from a broken store: a governance tool answering
			// "approved" for a decision it never recorded is the worst available failure.
			return nil, fmt.Errorf("decision not found: %s", decisionID)
		}
		return nil, fmt.Errorf("reading decision %s: %w", decisionID, err)
	}

	return unmarshalPayload[cgp.GovernanceDecision](payload, "decision")
}

// GetDecisionsByProposal returns all decisions recorded for a proposal, oldest first.
func (s *GovernanceMemoryStore) GetDecisionsByProposal(
	ctx context.Context, proposalID string,
) ([]*cgp.GovernanceDecision, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_decisions
		WHERE proposal_id = $1
		ORDER BY decided_at ASC, decision_id ASC`,
		proposalID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading decisions for proposal %s: %w", proposalID, err)
	}

	return scanPayloads[cgp.GovernanceDecision](rows, "decision")
}

// GetAuthorization returns an execution authorization by ID.
func (s *GovernanceMemoryStore) GetAuthorization(
	ctx context.Context, authID string,
) (*cgp.ExecutionAuthorization, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx,
		`SELECT payload FROM governance_authorizations WHERE authorization_id = $1`, authID,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("authorization not found: %s", authID)
		}
		return nil, fmt.Errorf("reading authorization %s: %w", authID, err)
	}

	return unmarshalPayload[cgp.ExecutionAuthorization](payload, "authorization")
}

// GetAuthorizationsByDecision returns all authorizations granted under a decision.
func (s *GovernanceMemoryStore) GetAuthorizationsByDecision(
	ctx context.Context, decisionID string,
) ([]*cgp.ExecutionAuthorization, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_authorizations
		WHERE decision_id = $1
		ORDER BY authorized_at ASC, authorization_id ASC`,
		decisionID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading authorizations for decision %s: %w", decisionID, err)
	}

	return scanPayloads[cgp.ExecutionAuthorization](rows, "authorization")
}

// GetActorMetrics returns an actor's behavior metrics, derived from their records.
//
// Derived rather than stored, which is the decision this adapter turns on.
//
// The file store materializes an ActorMetrics and folds each release into it with
// Accumulate, then has to rebuild the whole thing whenever a record is replaced, because a
// running average cannot be un-added. That bookkeeping is a read-modify-write, and this
// backend exists precisely so several processes write at once: two of them recording
// releases for one actor would each read the row, fold in their own release and write
// back, and one release would vanish. Not visibly — the actor's failure count would simply
// be lower than their history, and that number decides whether their next change is
// auto-approved. Deriving removes the write entirely, so there is nothing to lose.
//
// It also makes re-recording safe for a structural reason rather than a procedural one:
// the primary key means a release ID is one row, so there is no second contribution to
// subtract and no rebuild to remember to trigger.
//
// The computation itself is memory.RebuildActorMetrics — the same function the file store
// and InMemoryStore rebuild with, which folds each release through ActorMetrics.Accumulate.
// A fourth definition of how a release affects an actor's record is how two backends come
// to disagree about reputation while both look right.
//
// An actor with no releases is an error, unlike a repository with no history. The autonomy
// budget relies on telling "no record of this actor" apart from "this actor is clean".
func (s *GovernanceMemoryStore) GetActorMetrics(
	ctx context.Context, actorID string,
) (*cgpmemory.ActorMetrics, error) {
	releases, err := s.releasesByActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	incidents, err := s.incidentsByActor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	// Releases alone decide whether the actor is known: an actor nobody has seen release
	// anything is unknown rather than clean. Their incidents still count once a release makes
	// them known — see the contract's arrival-order case.
	if len(releases) == 0 {
		return nil, fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Grouped by repository because that is the shape RebuildActorMetrics reads, and the
	// kind comes off the newest record: an actor that changed kind is described by what
	// it is now, and the query is already ordered newest first.
	byRepo := make(map[string][]*cgpmemory.ReleaseRecord, 1)
	for _, r := range releases {
		byRepo[r.Repository] = append(byRepo[r.Repository], r)
	}
	incidentsByRepo := make(map[string][]*cgpmemory.IncidentRecord, 1)
	for _, i := range incidents {
		incidentsByRepo[i.Repository] = append(incidentsByRepo[i.Repository], i)
	}

	// An IncidentRecord names an actor without their kind, so an actor known only by an
	// incident has none to read and it stays zero rather than being guessed.
	var kind cgp.ActorKind
	if len(releases) > 0 {
		kind = releases[0].Actor.Kind
	}

	return cgpmemory.RebuildActorMetrics(
		actorID, kind, byRepo, incidentsByRepo, time.Now(),
	), nil
}

// UpdateActorMetrics reports whether the actor is known and otherwise does nothing.
//
// The file store patches its materialized metrics here — a rollback bumps the rollback and
// failure counts and decrements the successes — leaving the metrics saying something its
// own release records do not, until the next replacement rebuilds them and the patch
// silently disappears. There is nothing to patch in a derived store, and reintroducing a
// writable copy of numbers the records already determine would be the one thing this
// adapter set out not to have.
//
// So the way to record that a release rolled back is to record the release again with
// OutcomeRollback: the upsert replaces the row and every metric derived from it follows.
// No production code calls this method on any implementation — the only observable part of
// its contract is that an unknown actor is an error, and that is what is kept.
func (s *GovernanceMemoryStore) UpdateActorMetrics(
	ctx context.Context, actorID string, _ cgpmemory.ReleaseOutcome,
) error {
	var known bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM governance_releases WHERE actor_id = $1)`, actorID,
	).Scan(&known)
	if err != nil {
		return fmt.Errorf("checking actor %s: %w", actorID, err)
	}
	if !known {
		return fmt.Errorf("no metrics found for actor: %s", actorID)
	}
	return nil
}

// GetRiskPatterns returns a repository's historical risk patterns.
//
// A repository with no releases is an error rather than a zeroed pattern, which risk
// scoring would read as "historically safe" instead of "unknown".
//
// The analysis is run by memory.InMemoryStore over rows loaded from here, rather than
// reimplemented in SQL or in Go. Unlike Accumulate, the pattern computation was never
// extracted — FileStore and InMemoryStore each carry a verbatim copy — so writing a third
// would put the average, the trend threshold and the tag frequencies in three places, on
// numbers that feed risk scoring. Loading the history to analyze it is what the file store
// does too; the query is indexed and the volume is a repository's releases, and if this
// ever needs to be an aggregate the honest move is to extract the function first.
//
// Records are replayed oldest first so the trend's first-half/second-half comparison is
// chronological. The file store gets that from its append order, which is chronological
// only by habit.
func (s *GovernanceMemoryStore) GetRiskPatterns(
	ctx context.Context, repository string,
) (*cgpmemory.RiskPatterns, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_releases
		WHERE repository = $1
		ORDER BY released_at ASC, recorded_seq ASC`,
		repository,
	)
	if err != nil {
		return nil, fmt.Errorf("reading releases for %s: %w", repository, err)
	}

	releases, err := scanPayloads[cgpmemory.ReleaseRecord](rows, "release record")
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found for repository: %s", repository)
	}

	analysis := cgpmemory.NewInMemoryStore()
	for _, record := range releases {
		if err := analysis.RecordRelease(ctx, record); err != nil {
			return nil, fmt.Errorf("replaying release %s for analysis: %w", record.ID, err)
		}
	}

	return analysis.GetRiskPatterns(ctx, repository)
}

// GetAuditTrail returns the complete governance history for a proposal.
//
// A proposal with no decisions is an error: there is no trail, and an empty one would read
// as a release that passed through governance without anybody deciding anything.
func (s *GovernanceMemoryStore) GetAuditTrail(
	ctx context.Context, proposalID string,
) (*cgpmemory.AuditTrail, error) {
	decisions, err := s.GetDecisionsByProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if len(decisions) == 0 {
		return nil, fmt.Errorf("no audit trail found for proposal: %s", proposalID)
	}

	// One query for every authorization under any of the proposal's decisions, rather
	// than one per decision: a trail with a dozen decisions should still be two queries.
	rows, err := s.pool.Query(ctx, `
		SELECT a.payload FROM governance_authorizations a
		JOIN governance_decisions d ON d.decision_id = a.decision_id
		WHERE d.proposal_id = $1
		ORDER BY a.authorized_at ASC, a.authorization_id ASC`,
		proposalID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading authorizations for proposal %s: %w", proposalID, err)
	}

	auths, err := scanPayloads[cgp.ExecutionAuthorization](rows, "authorization")
	if err != nil {
		return nil, err
	}

	// Decisions arrive oldest first, so the ends of the trail are the ends of the slice;
	// an authorization can be later than every decision, and the trail's UpdatedAt has to
	// reflect that or it would date the record before its last entry.
	created := decisions[0].Timestamp
	updated := decisions[len(decisions)-1].Timestamp
	for _, a := range auths {
		if a.Timestamp.After(updated) {
			updated = a.Timestamp
		}
	}

	return &cgpmemory.AuditTrail{
		ProposalID:     proposalID,
		Decisions:      decisions,
		Authorizations: auths,
		CreatedAt:      created,
		UpdatedAt:      updated,
	}, nil
}

// releasesByActor loads every release an actor made, newest first.
//
// Not repository-scoped: an actor's reputation follows them across the repositories they
// release from, and RebuildActorMetrics is written to read exactly that population.
func (s *GovernanceMemoryStore) releasesByActor(
	ctx context.Context, actorID string,
) ([]*cgpmemory.ReleaseRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_releases
		WHERE actor_id = $1
		ORDER BY released_at DESC, recorded_seq DESC`,
		actorID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading releases for actor %s: %w", actorID, err)
	}

	return scanPayloads[cgpmemory.ReleaseRecord](rows, "release record")
}

// incidentsByActor loads every incident attributed to an actor.
//
// The whole record rather than a COUNT, because RebuildActorMetrics is given incidents and
// decides what they contribute. A count computed here would be this adapter deciding
// instead, and it would go stale the moment incidents contribute anything but their number.
func (s *GovernanceMemoryStore) incidentsByActor(
	ctx context.Context, actorID string,
) ([]*cgpmemory.IncidentRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_incidents
		WHERE actor_id = $1
		ORDER BY detected_at ASC, recorded_seq ASC`,
		actorID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading incidents for actor %s: %w", actorID, err)
	}

	return scanPayloads[cgpmemory.IncidentRecord](rows, "incident record")
}

// scanPayloads reads a payload-returning result set into records, closing it either way.
//
// A row that will not decode fails the call rather than being skipped, which is the
// opposite of what the run repository does with a malformed run. The difference is what
// the caller does with the answer: a run history the operator scrolls survives an
// omission, whereas these feed reputation, the DORA and SOC 2 reports and an audit trail,
// and a silently short answer there is a record of a different history.
func scanPayloads[T any](rows pgx.Rows, what string) ([]*T, error) {
	defer rows.Close()

	var records []*T
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", what, err)
		}

		record, err := unmarshalPayload[T](payload, what)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s rows: %w", what, err)
	}

	return records, nil
}

// unmarshalPayload decodes one stored record.
//
// Through encoding/json against the record type itself, which is the same encoding the
// file store writes into memory.json. One serialization, so a field added to a record
// reaches both backends at once and neither can quietly drop it on the way through.
func unmarshalPayload[T any](payload []byte, what string) (*T, error) {
	var record T
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, fmt.Errorf("decoding stored %s: %w", what, err)
	}
	return &record, nil
}
