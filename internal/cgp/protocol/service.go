package protocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	internalcgp "github.com/relicta-tech/relicta/v5/internal/cgp"
	"github.com/relicta-tech/relicta/v5/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v5/internal/cgp/evaluator"
	cgpsdk "github.com/relicta-tech/relicta/v5/pkg/cgp"
)

// ErrRecordNotFound marks an ordinary absent protocol record. Callers can
// distinguish an incomplete handshake from storage failures or corrupt data.
var ErrRecordNotFound = errors.New("protocol record not found")

// ProposalStore persists proposals and their associated decisions.
type ProposalStore interface {
	// SaveProposal stores a proposal for later retrieval.
	SaveProposal(ctx context.Context, proposal *cgpsdk.ChangeProposal) error

	// GetProposal retrieves a proposal by ID.
	GetProposal(ctx context.Context, proposalID string) (*cgpsdk.ChangeProposal, error)

	// SaveDecision stores a decision linked to a proposal.
	SaveDecision(ctx context.Context, decision *cgpsdk.GovernanceDecision) error

	// GetDecision retrieves the decision for a proposal.
	GetDecision(ctx context.Context, proposalID string) (*cgpsdk.GovernanceDecision, error)

	// SaveAuthorization stores an execution authorization.
	SaveAuthorization(ctx context.Context, auth *cgpsdk.ExecutionAuthorization) error

	// GetAuthorization retrieves the authorization for a proposal.
	GetAuthorization(ctx context.Context, proposalID string) (*cgpsdk.ExecutionAuthorization, error)
}

// RecordStore adds proposal enumeration for human-readable audit surfaces.
type RecordStore interface {
	ProposalStore
	ListProposals(ctx context.Context) ([]string, error)
}

// Service bridges CGP wire format messages to the internal evaluation pipeline.
type Service struct {
	evaluator  *evaluator.Evaluator
	store      ProposalStore
	auditStore audit.Store
	logger     *slog.Logger
}

// ServiceOption configures the CGP protocol service.
type ServiceOption func(*Service)

// WithServiceLogger sets the logger.
func WithServiceLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithStore sets the proposal store.
func WithStore(store ProposalStore) ServiceOption {
	return func(s *Service) {
		s.store = store
	}
}

// WithAuditStore appends protocol actions to the repository's shared governance chain.
func WithAuditStore(store audit.Store) ServiceOption {
	return func(s *Service) { s.auditStore = store }
}

// NewService creates a new CGP protocol service.
func NewService(eval *evaluator.Evaluator, opts ...ServiceOption) *Service {
	s := &Service{
		evaluator: eval,
		store:     &inMemoryStore{},
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// EvaluateProposal accepts a CGP wire format proposal, runs the internal
// evaluation pipeline, and returns a GovernanceDecision.
func (s *Service) EvaluateProposal(ctx context.Context, proposal *cgpsdk.ChangeProposal) (*cgpsdk.GovernanceDecision, error) {
	if err := cgpsdk.ValidateProposal(proposal); err != nil {
		return nil, fmt.Errorf("invalid proposal: %w", err)
	}

	s.logger.Info("evaluating CGP proposal",
		"proposal_id", proposal.ID,
		"actor", proposal.Actor.ID,
	)

	// Persist the incoming proposal.
	if err := s.store.SaveProposal(ctx, proposal); err != nil {
		return nil, fmt.Errorf("persist proposal %s: %w", proposal.ID, err)
	}
	if s.auditStore != nil {
		proposalDetails := map[string]any{
			"repository":  proposal.Scope.Repository,
			"branch":      proposal.Scope.Branch,
			"commitRange": proposal.Scope.CommitRange,
			"commits":     proposal.Scope.Commits,
			"files":       proposal.Scope.Files,
			"summary":     proposal.Intent.Summary,
			"confidence":  proposal.Intent.Confidence,
			"categories":  proposal.Intent.Categories,
		}
		detailsJSON, err := json.Marshal(proposalDetails)
		if err != nil {
			return nil, fmt.Errorf("encode proposal audit event %s: %w", proposal.ID, err)
		}
		detailsHash := sha256.Sum256(detailsJSON)
		entry := audit.NewEntry(proposal.ID+"_proposal_"+hex.EncodeToString(detailsHash[:8]), audit.EventProposalReceived).
			WithProposal(proposal.ID).
			WithActor(proposal.Actor.ID, internalcgp.ActorKind(proposal.Actor.Kind)).
			WithTimestamp(proposal.Timestamp).
			WithDetails(proposalDetails).Build()
		if err := audit.NewRecorder(s.auditStore, proposal.Scope.Repository).RecordIgnoringDuplicate(ctx, entry); err != nil {
			return nil, fmt.Errorf("record proposal audit event %s: %w", proposal.ID, err)
		}
	}

	// Convert SDK types to internal types for the evaluator.
	internalProposal := toInternalProposal(proposal)

	result, err := s.evaluator.Evaluate(ctx, internalProposal, nil)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	// Convert the internal decision back to the SDK wire format.
	decision := fromInternalDecision(result.Decision, proposal.ID)

	// Persist the decision.
	if err := s.store.SaveDecision(ctx, decision); err != nil {
		return nil, fmt.Errorf("persist decision for proposal %s: %w", proposal.ID, err)
	}
	if s.auditStore != nil {
		entry := audit.NewEntry(decision.ID+"_decision", audit.EventDecisionMade).
			WithProposal(decision.ProposalID).
			WithTimestamp(decision.Timestamp).
			WithDetail("decision", decision.Decision).
			WithDetail("riskScore", decision.RiskScore).
			WithDetail("riskFactors", result.Decision.RiskFactors).
			WithDetail("recommendedVersion", decision.RecommendedVersion).
			WithDetail("rationale", decision.Rationale).
			WithDetail("requiredActions", decision.RequiredActions).
			WithDetail("conditions", decision.Conditions).Build()
		if err := audit.NewRecorder(s.auditStore, proposal.Scope.Repository).RecordIgnoringDuplicate(ctx, entry); err != nil {
			return nil, fmt.Errorf("record decision audit event %s: %w", decision.ID, err)
		}
	}

	s.logger.Info("CGP evaluation complete",
		"proposal_id", proposal.ID,
		"decision", decision.Decision,
		"risk_score", decision.RiskScore,
	)

	return decision, nil
}

// RecordAuthorization stores an execution authorization for a proposal.
func (s *Service) RecordAuthorization(ctx context.Context, auth *cgpsdk.ExecutionAuthorization) error {
	if err := cgpsdk.ValidateAuthorization(auth); err != nil {
		// Returned as the validation error itself. FormatUserError reads the
		// outermost wrapping segment as the operation being attempted and appends
		// "failed", so wrapping this produced "Invalid authorization failed:
		// approvedBy.id is required" — the same garble GetStatus had. The convention
		// that formatter expects is `operation: cause`, and "invalid authorization"
		// is a cause.
		return err
	}

	// Verify the proposal exists.
	_, err := s.store.GetProposal(ctx, auth.ProposalID)
	if err != nil {
		// The store's error already names the proposal and says it was not found.
		return err
	}

	// Verify a decision exists for this proposal.
	decision, err := s.store.GetDecision(ctx, auth.ProposalID)
	if err != nil {
		return fmt.Errorf("no decision for proposal %s: %w", auth.ProposalID, err)
	}

	// Ensure the decision allows authorization.
	if decision.Decision == "denied" {
		return fmt.Errorf("cannot authorize: proposal %s was denied", auth.ProposalID)
	}

	if err := s.store.SaveAuthorization(ctx, auth); err != nil {
		return fmt.Errorf("failed to save authorization: %w", err)
	}
	if s.auditStore != nil {
		proposal, err := s.store.GetProposal(ctx, auth.ProposalID)
		if err != nil {
			return fmt.Errorf("read proposal for authorization audit: %w", err)
		}
		entry := audit.NewEntry(auth.ID+"_auth", audit.EventExecutionAuthorized).
			WithProposal(auth.ProposalID).
			WithActor(auth.ApprovedBy.ID, internalcgp.ActorKind(auth.ApprovedBy.Kind)).
			WithTimestamp(auth.Timestamp).
			WithDetail("version", auth.Version).
			WithDetail("scope", auth.Scope).
			WithDetail("validUntil", auth.ValidUntil).Build()
		if err := audit.NewRecorder(s.auditStore, proposal.Scope.Repository).RecordIgnoringDuplicate(ctx, entry); err != nil {
			return fmt.Errorf("record authorization audit event %s: %w", auth.ID, err)
		}
	}

	s.logger.Info("CGP authorization recorded",
		"proposal_id", auth.ProposalID,
		"decision_id", auth.DecisionID,
		"approved_by", auth.ApprovedBy.ID,
	)

	return nil
}

// GetStatus returns the current governance state for a proposal.
func (s *Service) GetStatus(ctx context.Context, proposalID string) (*ProposalStatus, error) {
	proposal, err := s.store.GetProposal(ctx, proposalID)
	if err != nil {
		// The store's error already says "proposal <id> not found", and it is
		// returned unwrapped on purpose. FormatUserError reads the outermost
		// wrapping segment as the operation being attempted and appends "failed",
		// so wrapping this in "proposal not found" produced the message an agent
		// actually received: "Proposal not found failed: proposal prop_x not
		// found". The convention that formatter expects is `operation: cause`, and
		// "proposal not found" is a cause.
		return nil, err
	}

	status := &ProposalStatus{
		ProposalID: proposalID,
		Proposal:   proposal,
		State:      "proposed",
	}

	decision, err := s.store.GetDecision(ctx, proposalID)
	if err == nil {
		status.Decision = decision
		status.State = "decided"
	} else if !errors.Is(err, ErrRecordNotFound) {
		return nil, fmt.Errorf("read decision for proposal %s: %w", proposalID, err)
	}

	auth, err := s.store.GetAuthorization(ctx, proposalID)
	if err == nil {
		status.Authorization = auth
		status.State = "authorized"
	} else if !errors.Is(err, ErrRecordNotFound) {
		return nil, fmt.Errorf("read authorization for proposal %s: %w", proposalID, err)
	}
	if status.Authorization != nil && status.Decision == nil {
		return nil, fmt.Errorf("proposal %s has an authorization without a decision", proposalID)
	}

	return status, nil
}

// ProposalStatus represents the current governance state for a proposal.
type ProposalStatus struct {
	ProposalID    string                         `json:"proposalId"`
	State         string                         `json:"state"` // "proposed", "decided", "authorized"
	Proposal      *cgpsdk.ChangeProposal         `json:"proposal"`
	Decision      *cgpsdk.GovernanceDecision     `json:"decision,omitempty"`
	Authorization *cgpsdk.ExecutionAuthorization `json:"authorization,omitempty"`
}

// toInternalProposal converts an SDK ChangeProposal to the internal CGP type.
func toInternalProposal(p *cgpsdk.ChangeProposal) *internalcgp.ChangeProposal {
	return internalcgp.NewProposal(
		internalcgp.Actor{
			Kind: internalcgp.ActorKind(p.Actor.Kind),
			ID:   p.Actor.ID,
			Name: p.Actor.Name,
		},
		internalcgp.ProposalScope{
			Repository:  p.Scope.Repository,
			Branch:      p.Scope.Branch,
			CommitRange: p.Scope.CommitRange,
			Commits:     p.Scope.Commits,
			Files:       p.Scope.Files,
		},
		internalcgp.ProposalIntent{
			Summary:    p.Intent.Summary,
			Confidence: p.Intent.Confidence,
			Categories: p.Intent.Categories,
		},
	)
}

// fromInternalDecision converts an internal GovernanceDecision to the SDK wire format.
func fromInternalDecision(d *internalcgp.GovernanceDecision, proposalID string) *cgpsdk.GovernanceDecision {
	decision := &cgpsdk.GovernanceDecision{
		CGPVersion:         cgpsdk.ProtocolVersion,
		Type:               cgpsdk.TypeGovernanceDecision,
		ID:                 d.ID,
		ProposalID:         proposalID,
		Timestamp:          time.Now().UTC(),
		Decision:           string(d.Decision),
		RiskScore:          d.RiskScore,
		RecommendedVersion: d.RecommendedVersion,
		Rationale:          d.Rationale,
	}

	for _, ra := range d.RequiredActions {
		decision.RequiredActions = append(decision.RequiredActions, cgpsdk.RequiredAction{
			Type:        ra.Type,
			Description: ra.Description,
			Assignee:    ra.Assignee,
		})
	}

	for _, c := range d.Conditions {
		decision.Conditions = append(decision.Conditions, cgpsdk.Condition{
			Type:  c.Type,
			Value: c.Value,
		})
	}

	return decision
}

// inMemoryStore is a default in-memory implementation of ProposalStore.
type inMemoryStore struct {
	proposals      map[string]*cgpsdk.ChangeProposal
	decisions      map[string]*cgpsdk.GovernanceDecision
	authorizations map[string]*cgpsdk.ExecutionAuthorization
}

func (s *inMemoryStore) init() {
	if s.proposals == nil {
		s.proposals = make(map[string]*cgpsdk.ChangeProposal)
	}
	if s.decisions == nil {
		s.decisions = make(map[string]*cgpsdk.GovernanceDecision)
	}
	if s.authorizations == nil {
		s.authorizations = make(map[string]*cgpsdk.ExecutionAuthorization)
	}
}

func (s *inMemoryStore) SaveProposal(_ context.Context, p *cgpsdk.ChangeProposal) error {
	s.init()
	s.proposals[p.ID] = p
	return nil
}

func (s *inMemoryStore) GetProposal(_ context.Context, id string) (*cgpsdk.ChangeProposal, error) {
	s.init()
	p, ok := s.proposals[id]
	if !ok {
		return nil, fmt.Errorf("%w: proposal %s not found", ErrRecordNotFound, id)
	}
	return p, nil
}

func (s *inMemoryStore) SaveDecision(_ context.Context, d *cgpsdk.GovernanceDecision) error {
	s.init()
	s.decisions[d.ProposalID] = d
	return nil
}

func (s *inMemoryStore) GetDecision(_ context.Context, proposalID string) (*cgpsdk.GovernanceDecision, error) {
	s.init()
	d, ok := s.decisions[proposalID]
	if !ok {
		return nil, fmt.Errorf("%w: no decision for proposal %s", ErrRecordNotFound, proposalID)
	}
	return d, nil
}

func (s *inMemoryStore) SaveAuthorization(_ context.Context, a *cgpsdk.ExecutionAuthorization) error {
	s.init()
	s.authorizations[a.ProposalID] = a
	return nil
}

func (s *inMemoryStore) GetAuthorization(_ context.Context, proposalID string) (*cgpsdk.ExecutionAuthorization, error) {
	s.init()
	a, ok := s.authorizations[proposalID]
	if !ok {
		return nil, fmt.Errorf("%w: no authorization for proposal %s", ErrRecordNotFound, proposalID)
	}
	return a, nil
}
