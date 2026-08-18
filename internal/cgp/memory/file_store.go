package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/fileutil"
)

// MaxMemoryFileSize is the maximum allowed size for memory files (5MB).
const MaxMemoryFileSize = 5 << 20 // 5MB

// FileStore provides a file-based implementation of the Store interface.
// Data is stored in JSON files in the specified directory.
type FileStore struct {
	basePath string
	mu       sync.RWMutex

	// In-memory cache for fast reads
	releases       map[string][]*ReleaseRecord            // keyed by repository
	deployments    map[string][]*DeploymentRecord         // keyed by repository
	incidents      map[string][]*IncidentRecord           // keyed by repository
	actors         map[string]*ActorMetrics               // keyed by actor ID
	decisions      map[string]*cgp.GovernanceDecision     // keyed by decision ID
	authorizations map[string]*cgp.ExecutionAuthorization // keyed by authorization ID
	chains         map[string][]*audit.Entry              // keyed by repository, in append order

	// Track if data has been loaded
	loaded bool
}

// NewFileStore creates a new file-based memory store.
func NewFileStore(basePath string) (*FileStore, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create memory directory: %w", err)
	}

	store := &FileStore{
		basePath:       basePath,
		releases:       make(map[string][]*ReleaseRecord),
		deployments:    make(map[string][]*DeploymentRecord),
		incidents:      make(map[string][]*IncidentRecord),
		actors:         make(map[string]*ActorMetrics),
		decisions:      make(map[string]*cgp.GovernanceDecision),
		authorizations: make(map[string]*cgp.ExecutionAuthorization),
		chains:         make(map[string][]*audit.Entry),
	}

	// Load existing data
	if err := store.load(); err != nil {
		return nil, fmt.Errorf("failed to load memory data: %w", err)
	}

	return store, nil
}

// fileData represents the JSON structure for persistence.
type fileData struct {
	Releases       map[string][]*ReleaseRecord            `json:"releases"`
	Deployments    map[string][]*DeploymentRecord         `json:"deployments,omitempty"`
	Incidents      map[string][]*IncidentRecord           `json:"incidents"`
	Actors         map[string]*ActorMetrics               `json:"actors"`
	Decisions      map[string]*cgp.GovernanceDecision     `json:"decisions,omitempty"`
	Authorizations map[string]*cgp.ExecutionAuthorization `json:"authorizations,omitempty"`

	// AuditChains is one hash-linked chain per repository, in append order.
	//
	// A JSON array and not a map, because order is the evidence: each entry names its
	// predecessor's hash, so a chain read back in a different order does not verify.
	// The other collections here are maps because their order carries nothing.
	AuditChains map[string][]*audit.Entry `json:"auditChains,omitempty"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// dataFilePath returns the path to the main data file.
func (s *FileStore) dataFilePath() string {
	return filepath.Join(s.basePath, "memory.json")
}

// load reads existing data from disk.
func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.dataFilePath()
	data, err := fileutil.ReadFileLimited(path, MaxMemoryFileSize)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing data, start fresh
			s.loaded = true
			return nil
		}
		return fmt.Errorf("failed to read memory file: %w", err)
	}

	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		return fmt.Errorf("failed to unmarshal memory data: %w", err)
	}

	// Load into cache
	if fd.Releases != nil {
		s.releases = fd.Releases
	}
	if fd.Deployments != nil {
		s.deployments = fd.Deployments
	}
	if fd.Incidents != nil {
		s.incidents = fd.Incidents
	}
	if fd.Actors != nil {
		s.actors = fd.Actors
	}
	if fd.Decisions != nil {
		s.decisions = fd.Decisions
	}
	if fd.Authorizations != nil {
		s.authorizations = fd.Authorizations
	}
	if fd.AuditChains != nil {
		s.chains = fd.AuditChains
	}

	s.loaded = true
	return nil
}

// save persists data to disk.
func (s *FileStore) save() error {
	fd := fileData{
		Releases:       s.releases,
		Deployments:    s.deployments,
		Incidents:      s.incidents,
		Actors:         s.actors,
		Decisions:      s.decisions,
		Authorizations: s.authorizations,
		AuditChains:    s.chains,
		UpdatedAt:      time.Now(),
	}

	data, err := json.MarshalIndent(fd, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memory data: %w", err)
	}

	if err := fileutil.AtomicWriteFile(s.dataFilePath(), data, 0600); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// RecordRelease stores a release record.
func (s *FileStore) RecordRelease(ctx context.Context, record *ReleaseRecord) error {
	if record == nil {
		return fmt.Errorf("record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("record ID is required")
	}
	if record.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records, replaced := UpsertReleaseRecord(s.releases[record.Repository], record)
	s.releases[record.Repository] = records

	if replaced != nil {
		// A replacement cannot be folded in: Accumulate keeps a running average, so
		// adding the new record on top of the old one's contribution would count the
		// release twice. Both actors are rebuilt because a corrected record may name a
		// different actor than the one it replaces.
		s.rebuildActorMetricsLocked(record.Actor.ID, record.Actor.Kind)
		if replaced.Actor.ID != record.Actor.ID {
			s.rebuildActorMetricsLocked(replaced.Actor.ID, replaced.Actor.Kind)
		}
	} else {
		s.updateActorMetricsLocked(record)
	}

	return s.save()
}

// knownActorKindLocked returns the kind already recorded for an actor, or the zero kind.
//
// An IncidentRecord names an actor but not their kind, so an incident cannot introduce one.
// Reading the kind back rather than passing a blank preserves what a release already established;
// an actor known only by an incident has no kind yet, and inventing one would be a claim about
// them that nothing recorded. Must be called with the lock held.
func (s *FileStore) knownActorKindLocked(actorID string) cgp.ActorKind {
	if metrics, exists := s.actors[actorID]; exists {
		return metrics.ActorKind
	}
	return ""
}

// rebuildActorMetricsLocked recomputes one actor's metrics from the stored records.
// Must be called with the lock held.
func (s *FileStore) rebuildActorMetricsLocked(actorID string, kind cgp.ActorKind) {
	if actorID == "" {
		return
	}
	s.actors[actorID] = RebuildActorMetrics(actorID, kind, s.releases, s.incidents, time.Now())
}

// updateActorMetricsLocked updates actor metrics based on a release record.
// Must be called with the lock held.
func (s *FileStore) updateActorMetricsLocked(record *ReleaseRecord) {
	actorID := record.Actor.ID
	metrics, exists := s.actors[actorID]
	if !exists {
		metrics = &ActorMetrics{ActorID: actorID, ActorKind: record.Actor.Kind}
		s.actors[actorID] = metrics
	}
	// One definition, shared with InMemoryStore and the Mnemos adapter.
	metrics.Accumulate(record, time.Now())
}

// RecordIncident stores an incident record.
func (s *FileStore) RecordIncident(ctx context.Context, incident *IncidentRecord) error {
	if incident == nil {
		return fmt.Errorf("incident is required")
	}
	if incident.ID == "" {
		return fmt.Errorf("incident ID is required")
	}
	if incident.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records, replaced := UpsertIncidentRecord(s.incidents[incident.Repository], incident)
	s.incidents[incident.Repository] = records

	// Rebuilt rather than incremented, for the reason RecordRelease rebuilds on a
	// replacement: a counter cannot be un-added, so a corrected incident would leave the
	// actor scored against one that happened once. Rebuilding also covers the actor an
	// incident names before they have any release — the old code only incremented
	// `if metrics, exists`, so that incident was dropped and reputation read them as clean.
	if incident.ActorID != "" {
		s.rebuildActorMetricsLocked(incident.ActorID, s.knownActorKindLocked(incident.ActorID))
	}
	if replaced != nil && replaced.ActorID != "" && replaced.ActorID != incident.ActorID {
		s.rebuildActorMetricsLocked(replaced.ActorID, s.knownActorKindLocked(replaced.ActorID))
	}

	return s.save()
}

// GetReleaseHistory returns release records for a repository.
func (s *FileStore) GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*ReleaseRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	releases := s.releases[repository]
	if len(releases) == 0 {
		return []*ReleaseRecord{}, nil
	}

	// Return most recent first
	result := make([]*ReleaseRecord, 0, min(limit, len(releases)))
	start := len(releases) - limit
	if start < 0 {
		start = 0
	}
	for i := len(releases) - 1; i >= start; i-- {
		result = append(result, releases[i])
	}

	return result, nil
}

// GetIncidentHistory returns incident records for a repository.
func (s *FileStore) GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*IncidentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	incidents := s.incidents[repository]
	if len(incidents) == 0 {
		return []*IncidentRecord{}, nil
	}

	// Return most recent first
	result := make([]*IncidentRecord, 0, min(limit, len(incidents)))
	start := len(incidents) - limit
	if start < 0 {
		start = 0
	}
	for i := len(incidents) - 1; i >= start; i-- {
		result = append(result, incidents[i])
	}

	return result, nil
}

// GetActorMetrics returns behavior metrics for an actor.
func (s *FileStore) GetActorMetrics(ctx context.Context, actorID string) (*ActorMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics, exists := s.actors[actorID]
	if !exists {
		return nil, fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// GetRiskPatterns returns historical risk patterns for a repository.
func (s *FileStore) GetRiskPatterns(ctx context.Context, repository string) (*RiskPatterns, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return RiskPatternsFrom(repository, s.releases[repository], time.Now())
}

// UpdateActorMetrics updates metrics for an actor based on a release outcome.
func (s *FileStore) UpdateActorMetrics(ctx context.Context, actorID string, outcome ReleaseOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics, exists := s.actors[actorID]
	if !exists {
		return fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Update based on outcome (used for updating after initial record)
	switch outcome {
	case OutcomeRollback:
		metrics.RollbackCount++
		metrics.FailedReleases++
		metrics.SuccessfulReleases-- // Undo the initial success count
	}

	metrics.SuccessRate = float64(metrics.SuccessfulReleases) / float64(metrics.TotalReleases)
	metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
	metrics.UpdatedAt = time.Now()

	return s.save()
}

// Flush ensures all data is written to disk.
func (s *FileStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// Stats returns store statistics.
func (s *FileStore) Stats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalReleases, totalIncidents int
	for _, releases := range s.releases {
		totalReleases += len(releases)
	}
	for _, incidents := range s.incidents {
		totalIncidents += len(incidents)
	}

	return StoreStats{
		Repositories:        len(s.releases),
		TotalReleases:       totalReleases,
		TotalIncidents:      totalIncidents,
		TrackedActors:       len(s.actors),
		TotalDecisions:      len(s.decisions),
		TotalAuthorizations: len(s.authorizations),
	}
}

// Snapshot is everything a store holds, as records rather than as counts.
//
// The Store interface cannot answer "give me everything": GetReleaseHistory needs a
// repository, GetDecision needs an ID, and nothing enumerates either. That is right for the
// readers — every one of them is asking about a repository, an actor or a proposal — and
// wrong for the one caller that has to move an audit trail from one backend to another.
// Hence this, next to the file store rather than on the port: only the store being migrated
// away from has to be readable this way.
type Snapshot struct {
	// Releases and Incidents are keyed by repository, as they are on disk.
	Releases  map[string][]*ReleaseRecord
	Incidents map[string][]*IncidentRecord

	// Decisions and Authorizations are keyed by their own ID, because neither carries a
	// repository — a decision hangs off a proposal.
	Decisions      map[string]*cgp.GovernanceDecision
	Authorizations map[string]*cgp.ExecutionAuthorization

	// Deployments are keyed by repository. They are in the snapshot even though no
	// database adapter can hold one, because a caller that cannot move them has to be able
	// to say how many it is leaving behind. Silence there would be the thing this whole
	// exercise is about.
	Deployments map[string][]*DeploymentRecord
}

// ActorMetrics are deliberately absent from Snapshot.
//
// The file store accumulates them; the database adapters derive them from the releases and
// incidents they hold, through the same memory.RebuildActorMetrics. Copying a materialized
// figure into a store that recomputes it would either be ignored or, worse, disagree with
// what the rows say — and the number in question is the one that decides whether an actor's
// next change is auto-approved.

// Snapshot returns a copy of everything this store holds.
//
// The maps and slices are fresh, so a caller iterating a snapshot cannot be tripped by a
// concurrent write, but the records inside are shared pointers. That is deliberate and safe
// for the only caller: the importer reads them and writes them elsewhere, and a deep copy of
// an entire audit trail would double the memory for no property anyone needs.
func (s *FileStore) Snapshot(_ context.Context) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := &Snapshot{
		Releases:       make(map[string][]*ReleaseRecord, len(s.releases)),
		Incidents:      make(map[string][]*IncidentRecord, len(s.incidents)),
		Decisions:      make(map[string]*cgp.GovernanceDecision, len(s.decisions)),
		Authorizations: make(map[string]*cgp.ExecutionAuthorization, len(s.authorizations)),
		Deployments:    make(map[string][]*DeploymentRecord, len(s.deployments)),
	}

	for repository, records := range s.releases {
		snapshot.Releases[repository] = append([]*ReleaseRecord(nil), records...)
	}
	for repository, records := range s.incidents {
		snapshot.Incidents[repository] = append([]*IncidentRecord(nil), records...)
	}
	for repository, records := range s.deployments {
		snapshot.Deployments[repository] = append([]*DeploymentRecord(nil), records...)
	}
	for id, decision := range s.decisions {
		snapshot.Decisions[id] = decision
	}
	for id, auth := range s.authorizations {
		snapshot.Authorizations[id] = auth
	}

	return snapshot, nil
}

// StoreStats contains store statistics.
type StoreStats struct {
	Repositories        int `json:"repositories"`
	TotalReleases       int `json:"totalReleases"`
	TotalIncidents      int `json:"totalIncidents"`
	TrackedActors       int `json:"trackedActors"`
	TotalDecisions      int `json:"totalDecisions"`
	TotalAuthorizations int `json:"totalAuthorizations"`
}

// RecordDecision stores a governance decision.
func (s *FileStore) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	if decision == nil {
		return fmt.Errorf("decision is required")
	}
	if decision.ID == "" {
		return fmt.Errorf("decision ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.decisions[decision.ID] = decision
	return s.save()
}

// RecordAuthorization stores an execution authorization.
func (s *FileStore) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	if auth == nil {
		return fmt.Errorf("authorization is required")
	}
	if auth.ID == "" {
		return fmt.Errorf("authorization ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.authorizations[auth.ID] = auth
	return s.save()
}

// GetDecision returns a governance decision by ID.
func (s *FileStore) GetDecision(ctx context.Context, decisionID string) (*cgp.GovernanceDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	decision, exists := s.decisions[decisionID]
	if !exists {
		return nil, fmt.Errorf("decision not found: %s", decisionID)
	}
	return decision, nil
}

// GetDecisionsByProposal returns all decisions for a proposal.
func (s *FileStore) GetDecisionsByProposal(ctx context.Context, proposalID string) ([]*cgp.GovernanceDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var decisions []*cgp.GovernanceDecision
	for _, d := range s.decisions {
		if d.ProposalID == proposalID {
			decisions = append(decisions, d)
		}
	}
	return decisions, nil
}

// GetAuthorization returns an execution authorization by ID.
func (s *FileStore) GetAuthorization(ctx context.Context, authID string) (*cgp.ExecutionAuthorization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	auth, exists := s.authorizations[authID]
	if !exists {
		return nil, fmt.Errorf("authorization not found: %s", authID)
	}
	return auth, nil
}

// GetAuthorizationsByDecision returns all authorizations for a decision.
func (s *FileStore) GetAuthorizationsByDecision(ctx context.Context, decisionID string) ([]*cgp.ExecutionAuthorization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var auths []*cgp.ExecutionAuthorization
	for _, a := range s.authorizations {
		if a.DecisionID == decisionID {
			auths = append(auths, a)
		}
	}
	return auths, nil
}

// GetAuditTrail returns the complete audit trail for a proposal.
func (s *FileStore) GetAuditTrail(ctx context.Context, proposalID string) (*AuditTrail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Gather all decisions for this proposal
	var decisions []*cgp.GovernanceDecision
	for _, d := range s.decisions {
		if d.ProposalID == proposalID {
			decisions = append(decisions, d)
		}
	}

	if len(decisions) == 0 {
		return nil, fmt.Errorf("no audit trail found for proposal: %s", proposalID)
	}

	// Gather all authorizations for these decisions
	var auths []*cgp.ExecutionAuthorization
	decisionIDs := make(map[string]bool)
	for _, d := range decisions {
		decisionIDs[d.ID] = true
	}
	for _, a := range s.authorizations {
		if decisionIDs[a.DecisionID] {
			auths = append(auths, a)
		}
	}

	// Find earliest and latest timestamps
	var earliest, latest time.Time
	for i, d := range decisions {
		if i == 0 || d.Timestamp.Before(earliest) {
			earliest = d.Timestamp
		}
		if i == 0 || d.Timestamp.After(latest) {
			latest = d.Timestamp
		}
	}
	for _, a := range auths {
		if a.Timestamp.After(latest) {
			latest = a.Timestamp
		}
	}

	return &AuditTrail{
		ProposalID:     proposalID,
		Decisions:      decisions,
		Authorizations: auths,
		CreatedAt:      earliest,
		UpdatedAt:      latest,
	}, nil
}

// AppendAuditEntry appends one linked entry to a repository's chain and writes the file.
//
// The write is not batched. Every other record here can be reconstructed from the run
// directory or re-derived, so losing the last one to a crash costs an entry in a history;
// an audit chain entry that was never written is a governance event with no evidence,
// and the release it belongs to has already happened by the time the process exits.
//
// This is also the backend where the chain has a ceiling. memory.json is one document read
// under MaxMemoryFileSize, and a chain entry costs roughly 550 bytes of it — about eight
// entries per release, so the 5 MB limit is reached somewhere near a thousand releases, and
// reaching it fails the whole store rather than only the chain. Not raised here, because a
// larger number would move the wall rather than remove it and would silently change what
// every existing memory.json is allowed to grow to. ADR-013 already names sqlite as the
// destination for a repository with real history; this is the compatibility path, and a
// thousand releases is where it stops being one.
func (s *FileStore) AppendAuditEntry(
	_ context.Context, repository string, entry *audit.Entry,
) error {
	if repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := appendAuditEntry(s.chains[repository], entry)
	if err != nil {
		return err
	}
	s.chains[repository] = entries

	return s.save()
}

// LastAuditEntry returns the repository's tail entry, or nil when it has no chain yet.
func (s *FileStore) LastAuditEntry(
	_ context.Context, repository string,
) (*audit.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return lastAuditEntry(s.chains[repository]), nil
}

// AuditChain returns the repository's entries in append order.
func (s *FileStore) AuditChain(
	_ context.Context, repository string,
) ([]*audit.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return copyAuditEntries(s.chains[repository]), nil
}
