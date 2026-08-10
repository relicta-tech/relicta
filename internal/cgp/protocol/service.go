package protocol

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	internalcgp "github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	cgpsdk "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

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

// Service bridges CGP wire format messages to the internal evaluation pipeline.
type Service struct {
	evaluator *evaluator.Evaluator
	store     ProposalStore
	logger    *slog.Logger
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
		s.logger.Warn("failed to persist proposal", "error", err)
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
		s.logger.Warn("failed to persist decision", "error", err)
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
	}

	auth, err := s.store.GetAuthorization(ctx, proposalID)
	if err == nil {
		status.Authorization = auth
		status.State = "authorized"
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
		return nil, fmt.Errorf("proposal %s not found", id)
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
		return nil, fmt.Errorf("no decision for proposal %s", proposalID)
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
		return nil, fmt.Errorf("no authorization for proposal %s", proposalID)
	}
	return a, nil
}
