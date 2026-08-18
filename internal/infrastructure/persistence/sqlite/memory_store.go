package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// MemoryStore implements memory.Store — the governance record — on the same SQLite
// file as the release runs.
//
// ADR-013 names the governance record part of the system of record alongside the run,
// and gives the reason for putting them in one file: a crash between writing the run
// and updating governance memory leaves the two disagreeing, and for a tool whose
// product is audit evidence that is the worst available failure. Two files cannot be
// written in one transaction; these tables can.
//
// The other reason is the same as the run store's. `.relicta/governance/memory.json` is
// one document holding every release, incident, decision and authorization the tool has
// ever seen: reading an actor's metrics parses the whole history, and so does recording
// a single release, which then rewrites the whole file. Here each of those is a query
// against an index.
//
// Nothing selects this store yet — ADR-013 keeps `file` the default until parity is
// proven, and the conformance suite in memory_conformance_test.go is that proof.
type MemoryStore struct {
	db   *sql.DB
	path string
}

// Ensure MemoryStore implements the whole port rather than a convenient subset of it.
var _ memory.Store = (*MemoryStore)(nil)

// OpenMemoryStore connects to the database at path, creating it and its parent
// directory if needed, and applies any pending migrations.
//
// Same file, same pragmas and same migrator as Open: see openDatabase. The pragmas are
// not "also fine" here, they are required for the same reason — WAL so `relicta status`
// reading governance memory does not exclude `relicta publish` writing it, a busy
// timeout so two processes in one repository wait rather than fail, and immediate
// transactions so a read-then-write cannot meet the unrecoverable upgrade error that
// the busy handler is not allowed to absorb.
func OpenMemoryStore(ctx context.Context, path string) (*MemoryStore, error) {
	db, err := openDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	return &MemoryStore{db: db, path: path}, nil
}

// Path returns the database file this store was opened on.
func (s *MemoryStore) Path() string { return s.path }

// Close releases the connection pool.
func (s *MemoryStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing sqlite database %s: %w", s.path, err)
	}
	return nil
}

// RecordRelease stores a release record, replacing the one that already carries its ID.
//
// The ON CONFLICT clause is memory.UpsertReleaseRecord expressed as a constraint. That
// function replaces the matching element of a slice in place rather than appending a
// second one, because two records for one run inflate deployment frequency and count
// the actor's release twice; the row keeping its recorded_seq is what "in place" means
// here, so a corrected record also keeps its position in the history.
//
// No actor metrics are touched, which is the point of deriving them. The reference has
// to rebuild an actor's metrics whenever a record is replaced — Accumulate keeps a
// running average and a running average cannot be un-added — and a store that forgot
// would silently inflate the reputation its autonomy decisions are made from. Here the
// row is the only copy, so the correction is complete once the row is written.
func (s *MemoryStore) RecordRelease(ctx context.Context, record *memory.ReleaseRecord) error {
	if record == nil {
		return errors.New("record is required")
	}
	if record.ID == "" {
		return errors.New("record ID is required")
	}
	if record.Repository == "" {
		return errors.New("repository is required")
	}

	document, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding release record %s: %w", record.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO governance_releases (repository, release_id, actor_id, document)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(repository, release_id) DO UPDATE SET
			actor_id = excluded.actor_id,
			document = excluded.document`,
		record.Repository, record.ID, record.Actor.ID, string(document),
	)
	if err != nil {
		return fmt.Errorf("recording release %s: %w", record.ID, err)
	}
	return nil
}

// RecordIncident stores an incident record.
//
// An insert and not an upsert, unlike RecordRelease: the reference appends every
// incident it is given. See the schema for why this adapter does not invent a
// deduplication rule the other backends do not have.
func (s *MemoryStore) RecordIncident(ctx context.Context, incident *memory.IncidentRecord) error {
	if incident == nil {
		return errors.New("incident is required")
	}
	if incident.ID == "" {
		return errors.New("incident ID is required")
	}
	if incident.Repository == "" {
		return errors.New("repository is required")
	}

	document, err := json.Marshal(incident)
	if err != nil {
		return fmt.Errorf("encoding incident record %s: %w", incident.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO governance_incidents (repository, incident_id, actor_id, document)
		VALUES (?, ?, ?, ?)`,
		incident.Repository, incident.ID, incident.ActorID, string(document),
	)
	if err != nil {
		return fmt.Errorf("recording incident %s: %w", incident.ID, err)
	}
	return nil
}

// RecordDecision stores a governance decision, replacing one already recorded under its
// ID — which is what assigning into the reference's map does.
func (s *MemoryStore) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	if decision == nil {
		return errors.New("decision is required")
	}
	if decision.ID == "" {
		return errors.New("decision ID is required")
	}

	document, err := json.Marshal(decision)
	if err != nil {
		return fmt.Errorf("encoding decision %s: %w", decision.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO governance_decisions (decision_id, proposal_id, decided_at, document)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(decision_id) DO UPDATE SET
			proposal_id = excluded.proposal_id,
			decided_at  = excluded.decided_at,
			document    = excluded.document`,
		decision.ID, decision.ProposalID, decision.Timestamp.UnixNano(), string(document),
	)
	if err != nil {
		return fmt.Errorf("recording decision %s: %w", decision.ID, err)
	}
	return nil
}

// RecordAuthorization stores an execution authorization, replacing one already recorded
// under its ID.
func (s *MemoryStore) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	if auth == nil {
		return errors.New("authorization is required")
	}
	if auth.ID == "" {
		return errors.New("authorization ID is required")
	}

	document, err := json.Marshal(auth)
	if err != nil {
		return fmt.Errorf("encoding authorization %s: %w", auth.ID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO governance_authorizations
			(authorization_id, decision_id, authorized_at, document)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(authorization_id) DO UPDATE SET
			decision_id   = excluded.decision_id,
			authorized_at = excluded.authorized_at,
			document      = excluded.document`,
		auth.ID, auth.DecisionID, auth.Timestamp.UnixNano(), string(document),
	)
	if err != nil {
		return fmt.Errorf("recording authorization %s: %w", auth.ID, err)
	}
	return nil
}

// GetReleaseHistory returns a repository's release records, most recent first.
//
// A repository nothing was ever recorded against is an empty history and not an error:
// it is the ordinary starting point, and every report runs against one at least once.
func (s *MemoryStore) GetReleaseHistory(
	ctx context.Context, repository string, limit int,
) ([]*memory.ReleaseRecord, error) {
	if limit <= 0 {
		// Not a fall-through to SQL. `LIMIT 0` happens to agree with the reference, but
		// `LIMIT -1` means *unlimited* in SQLite, so a caller asking for -1 records
		// would be handed the entire history — the whole of `relicta history` rendered
		// because an off-by-one produced a negative page size. The reference computes
		// `start := len(releases) - limit`, which for any limit at or below zero puts
		// the start past the end and returns nothing.
		return []*memory.ReleaseRecord{}, nil
	}

	records, err := queryDocuments[memory.ReleaseRecord](ctx, s.db, "release record", `
		SELECT document FROM governance_releases
		WHERE repository = ?
		ORDER BY recorded_seq DESC
		LIMIT ?`, repository, limit)
	if err != nil {
		return nil, err
	}
	if records == nil {
		// An empty slice rather than nil, matching the reference. Callers range over
		// this and JSON-encode it, and `"releases": null` in an API response is not the
		// same document as `"releases": []`.
		return []*memory.ReleaseRecord{}, nil
	}
	return records, nil
}

// GetIncidentHistory returns a repository's incident records, most recent first.
func (s *MemoryStore) GetIncidentHistory(
	ctx context.Context, repository string, limit int,
) ([]*memory.IncidentRecord, error) {
	if limit <= 0 {
		// See GetReleaseHistory: zero and below return nothing, and LIMIT -1 would not.
		return []*memory.IncidentRecord{}, nil
	}

	records, err := queryDocuments[memory.IncidentRecord](ctx, s.db, "incident record", `
		SELECT document FROM governance_incidents
		WHERE repository = ?
		ORDER BY recorded_seq DESC
		LIMIT ?`, repository, limit)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return []*memory.IncidentRecord{}, nil
	}
	return records, nil
}

// GetDecision returns a governance decision by ID.
//
// Not found is an error, and deliberately not a nil record with a nil error: the audit
// trail has to tell "no such decision" apart from a store that answered, and a
// governance tool reporting "approved" for a decision it never recorded is the worst
// available failure.
func (s *MemoryStore) GetDecision(
	ctx context.Context, decisionID string,
) (*cgp.GovernanceDecision, error) {
	decision, err := queryDocument[cgp.GovernanceDecision](ctx, s.db, "decision",
		`SELECT document FROM governance_decisions WHERE decision_id = ?`, decisionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("decision not found: %s", decisionID)
	}
	if err != nil {
		return nil, err
	}
	return decision, nil
}

// GetDecisionsByProposal returns every decision recorded for a proposal, oldest first.
func (s *MemoryStore) GetDecisionsByProposal(
	ctx context.Context, proposalID string,
) ([]*cgp.GovernanceDecision, error) {
	return queryDocuments[cgp.GovernanceDecision](ctx, s.db, "decision", `
		SELECT document FROM governance_decisions
		WHERE proposal_id = ?
		ORDER BY decided_at, decision_id`, proposalID)
}

// GetAuthorization returns an execution authorization by ID.
func (s *MemoryStore) GetAuthorization(
	ctx context.Context, authID string,
) (*cgp.ExecutionAuthorization, error) {
	auth, err := queryDocument[cgp.ExecutionAuthorization](ctx, s.db, "authorization",
		`SELECT document FROM governance_authorizations WHERE authorization_id = ?`, authID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("authorization not found: %s", authID)
	}
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// GetAuthorizationsByDecision returns every authorization granted under a decision,
// oldest first.
func (s *MemoryStore) GetAuthorizationsByDecision(
	ctx context.Context, decisionID string,
) ([]*cgp.ExecutionAuthorization, error) {
	return queryDocuments[cgp.ExecutionAuthorization](ctx, s.db, "authorization", `
		SELECT document FROM governance_authorizations
		WHERE decision_id = ?
		ORDER BY authorized_at, authorization_id`, decisionID)
}

// GetAuditTrail assembles the governance history of one proposal.
//
// A proposal with no decisions is an error rather than an empty trail, matching the
// reference: an audit answer of "here is the trail, it is empty" claims the proposal was
// never decided, when the truth may be that this store never saw it.
func (s *MemoryStore) GetAuditTrail(
	ctx context.Context, proposalID string,
) (*memory.AuditTrail, error) {
	decisions, err := s.GetDecisionsByProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if len(decisions) == 0 {
		return nil, fmt.Errorf("no audit trail found for proposal: %s", proposalID)
	}

	// Joined through the decisions rather than fetched per decision: an authorization
	// belongs to the trail when its decision does, and one query cannot go out of step
	// with the list above the way N queries in a loop can.
	auths, err := queryDocuments[cgp.ExecutionAuthorization](ctx, s.db, "authorization", `
		SELECT a.document
		FROM governance_authorizations AS a
		JOIN governance_decisions AS d ON d.decision_id = a.decision_id
		WHERE d.proposal_id = ?
		ORDER BY a.authorized_at, a.authorization_id`, proposalID)
	if err != nil {
		return nil, err
	}

	// Oldest decision first out of the query, so the trail's bounds are its ends —
	// extended by any authorization granted after the last decision, which is the
	// ordinary case for an approved proposal.
	createdAt := decisions[0].Timestamp
	updatedAt := decisions[len(decisions)-1].Timestamp
	if len(auths) > 0 {
		if last := auths[len(auths)-1].Timestamp; last.After(updatedAt) {
			updatedAt = last
		}
	}

	return &memory.AuditTrail{
		ProposalID:     proposalID,
		Decisions:      decisions,
		Authorizations: auths,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

// GetActorMetrics derives an actor's behavior metrics from the records that define them.
//
// Derived rather than stored. The reference materializes an ActorMetrics per actor
// because a map is all it can query, and pays for it: every replacement of a release
// record invalidates the running average folded into that projection, so RecordRelease
// has to notice and rebuild — and rebuild the *replaced* record's actor too, in case the
// correction changed who it names. A backend that forgets any of that reports an
// inflated history, and the numbers here are what decide whether an actor's next change
// is auto-approved. With the rows as the only copy the question does not arise.
//
// The arithmetic is still memory.RebuildActorMetrics, feeding each record through
// Accumulate. Expressing it as SQL aggregates would be a fourth definition of an actor's
// reputation — and one that would count the canceled runs ReleaseOutcome.CountsAsRelease
// exists to keep out of every rate.
//
// An actor with no releases is an error, not zeroed metrics. That is the asymmetry with
// GetReleaseHistory, and it is deliberate on both sides: the autonomy budget has to tell
// "no record of this actor" apart from "this actor is clean", and zeroed metrics say the
// second.
func (s *MemoryStore) GetActorMetrics(
	ctx context.Context, actorID string,
) (*memory.ActorMetrics, error) {
	releases, err := queryDocuments[memory.ReleaseRecord](ctx, s.db, "release record", `
		SELECT document FROM governance_releases
		WHERE actor_id = ?
		ORDER BY recorded_seq`, actorID)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	incidents, err := queryDocuments[memory.IncidentRecord](ctx, s.db, "incident record", `
		SELECT document FROM governance_incidents
		WHERE actor_id = ?
		ORDER BY recorded_seq`, actorID)
	if err != nil {
		return nil, err
	}

	// The kind comes from the actor's most recent release rather than from the first or
	// from a default. An agent or CI pipeline recorded as human is exactly the
	// attribution a governance audit exists to make legible, and the latest record is
	// the store's best evidence of what this actor is now.
	kind := releases[len(releases)-1].Actor.Kind

	return memory.RebuildActorMetrics(
		actorID, kind, releasesByRepository(releases), incidentsByRepository(incidents),
		time.Now(),
	), nil
}

// UpdateActorMetrics records that an actor's most recent release was rolled back.
//
// The reference nudges counters on its stored projection: RollbackCount and
// FailedReleases up, SuccessfulReleases down, rates recomputed. A derived store has no
// counter to nudge, so the correction lands where those counters come from — the release
// record itself. The resulting metrics are identical for the case this method exists to
// serve, a release recorded as successful and later withdrawn, and the record and the
// metrics now agree: the reference leaves its history saying "success" while the actor's
// numbers say otherwise.
//
// Applying it twice is a no-op here, where the reference would decrement
// SuccessfulReleases a second time and take it negative. That difference is not
// load bearing — nothing in the tree calls this method on any implementation — but a
// backend cannot be the place where an incoherent count becomes reachable.
//
// An outcome other than a rollback changes nothing, matching the reference, which only
// switches on OutcomeRollback and otherwise just recomputes what it already had.
func (s *MemoryStore) UpdateActorMetrics(
	ctx context.Context, actorID string, outcome memory.ReleaseOutcome,
) error {
	var seq int64
	var document string
	err := s.db.QueryRowContext(ctx, `
		SELECT recorded_seq, document FROM governance_releases
		WHERE actor_id = ?
		ORDER BY recorded_seq DESC
		LIMIT 1`, actorID,
	).Scan(&seq, &document)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("no metrics found for actor: %s", actorID)
	}
	if err != nil {
		return fmt.Errorf("reading releases for actor %s: %w", actorID, err)
	}

	if outcome != memory.OutcomeRollback {
		return nil
	}

	var record memory.ReleaseRecord
	if err := json.Unmarshal([]byte(document), &record); err != nil {
		return fmt.Errorf("decoding stored release record for actor %s: %w", actorID, err)
	}
	if record.Outcome == memory.OutcomeRollback {
		return nil
	}

	record.Outcome = memory.OutcomeRollback
	updated, err := json.Marshal(&record)
	if err != nil {
		return fmt.Errorf("encoding release record %s: %w", record.ID, err)
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE governance_releases SET document = ? WHERE recorded_seq = ?`,
		string(updated), seq,
	); err != nil {
		return fmt.Errorf("recording rollback for actor %s: %w", actorID, err)
	}
	return nil
}

// GetRiskPatterns derives a repository's historical risk patterns from its releases.
//
// Ordered by recorded_seq, i.e. the order the reference holds its slice in, because the
// trend memory.RiskPatternsFrom computes compares the first half of that order against
// the second. A repository with no releases is an error: zeroed patterns feed risk
// evaluation as "this repository has never shipped anything risky", which is an
// assertion about its history rather than an admission that there is none.
func (s *MemoryStore) GetRiskPatterns(
	ctx context.Context, repository string,
) (*memory.RiskPatterns, error) {
	releases, err := queryDocuments[memory.ReleaseRecord](ctx, s.db, "release record", `
		SELECT document FROM governance_releases
		WHERE repository = ?
		ORDER BY recorded_seq`, repository)
	if err != nil {
		return nil, err
	}
	return memory.RiskPatternsFrom(repository, releases, time.Now())
}

// releasesByRepository groups release records the way RebuildActorMetrics reads them.
//
// It filters by actor itself, so a flat list under one key would give the same numbers —
// but grouping honestly costs nothing and keeps the argument meaning what its parameter
// name says, which is what the next reader will assume.
func releasesByRepository(records []*memory.ReleaseRecord) map[string][]*memory.ReleaseRecord {
	grouped := make(map[string][]*memory.ReleaseRecord)
	for _, record := range records {
		grouped[record.Repository] = append(grouped[record.Repository], record)
	}
	return grouped
}

// incidentsByRepository is releasesByRepository for the incident half of the same call.
func incidentsByRepository(records []*memory.IncidentRecord) map[string][]*memory.IncidentRecord {
	grouped := make(map[string][]*memory.IncidentRecord)
	for _, record := range records {
		grouped[record.Repository] = append(grouped[record.Repository], record)
	}
	return grouped
}

// queryDocument decodes the single stored document a query selects.
//
// Reports sql.ErrNoRows unwrapped so callers can turn it into the not-found message
// their method documents.
func queryDocument[T any](
	ctx context.Context, db *sql.DB, what, query string, args ...any,
) (*T, error) {
	var document string
	if err := db.QueryRowContext(ctx, query, args...).Scan(&document); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("reading stored %s: %w", what, err)
	}
	return decodeDocument[T](document, what)
}

// queryDocuments decodes every stored document a query selects, in the query's order.
func queryDocuments[T any](
	ctx context.Context, db *sql.DB, what, query string, args ...any,
) ([]*T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading stored %ss: %w", what, err)
	}
	defer func() { _ = rows.Close() }()

	var records []*T
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, fmt.Errorf("scanning stored %s: %w", what, err)
		}
		record, err := decodeDocument[T](document, what)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stored %ss: %w", what, err)
	}
	return records, nil
}

// decodeDocument turns one stored JSON document back into its record.
//
// A document that will not decode is reported, not skipped. The run store skips such a
// row because its reference reads one file per run and an unreadable one hides only
// itself; the governance reference reads a single JSON document, so a record it cannot
// parse fails the whole load there too. Silently dropping one would hand a report an
// audit trail with a hole in it and no indication that anything was missing.
func decodeDocument[T any](document, what string) (*T, error) {
	record := new(T)
	if err := json.Unmarshal([]byte(document), record); err != nil {
		return nil, fmt.Errorf("decoding stored %s: %w", what, err)
	}
	return record, nil
}
