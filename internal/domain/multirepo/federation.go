package multirepo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// FederationMessageType represents CGP federation message types for multi-repo coordination.
type FederationMessageType string

const (
	// FederationProposal is sent when a downstream dependency changes.
	FederationProposal FederationMessageType = "federation.change_proposal"
	// FederationDecision is the aggregated governance decision across repos.
	FederationDecision FederationMessageType = "federation.governance_decision"
	// FederationNotification is a webhook-based notification between instances.
	FederationNotification FederationMessageType = "federation.notification"
	// FederationRiskExchange is sent when a repo's risk state changes.
	FederationRiskExchange FederationMessageType = "federation.risk_update"
)

// FederationMessage is a CGP message exchanged between Relicta instances
// for coordinated multi-repo governance.
type FederationMessage struct {
	// ID is a unique identifier for this federation message.
	ID string `json:"id"`
	// Type is the federation message type.
	Type FederationMessageType `json:"type"`
	// SourceRepo identifies the originating repository.
	SourceRepo string `json:"source_repo"`
	// TargetRepo identifies the destination repository.
	TargetRepo string `json:"target_repo"`
	// GroupName is the repository group context.
	GroupName string `json:"group_name"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
	// CorrelationID links related federation messages.
	CorrelationID string `json:"correlation_id"`
	// Proposal is set when Type is FederationProposal.
	Proposal *FederatedProposal `json:"proposal,omitempty"`
	// Decision is set when Type is FederationDecision.
	Decision *FederatedDecision `json:"decision,omitempty"`
	// Notification is set when Type is FederationNotification.
	Notification *FederatedNotification `json:"notification,omitempty"`
	// RiskUpdate is set when Type is FederationRiskExchange.
	RiskUpdate *FederationRiskUpdate `json:"risk_update,omitempty"`
}

// FederationRiskUpdate carries a repo's current risk state to other repos in the group.
type FederationRiskUpdate struct {
	// Repository is the name of the repository reporting its risk.
	Repository string `json:"repository"`
	// RiskScore is the current risk score (0.0-1.0).
	RiskScore float64 `json:"risk_score"`
	// State is the release state (e.g., "planned", "releasing", "released").
	State string `json:"state"`
	// Version is the version being released.
	Version string `json:"version"`
	// Timestamp is when this risk state was recorded.
	Timestamp time.Time `json:"timestamp"`
}

// FederatedProposal represents a change proposal sent between repos.
type FederatedProposal struct {
	// ChangeDescription describes what changed in the source repo.
	ChangeDescription string `json:"change_description"`
	// ProposedVersion is the version the source repo will release.
	ProposedVersion string `json:"proposed_version"`
	// BreakingChanges indicates if the change contains breaking changes.
	BreakingChanges bool `json:"breaking_changes"`
	// RiskScore is the source repo's assessed risk (0.0-1.0).
	RiskScore float64 `json:"risk_score"`
	// AffectedAPIs lists the APIs that changed.
	AffectedAPIs []string `json:"affected_apis,omitempty"`
}

// FederatedDecision represents an aggregated governance decision.
type FederatedDecision struct {
	// Decision is the overall governance outcome.
	Decision cgp.DecisionType `json:"decision"`
	// RepoDecisions maps repo names to their individual decisions.
	RepoDecisions map[string]RepoDecisionSummary `json:"repo_decisions"`
	// AggregateRiskScore is the combined risk across all repos.
	AggregateRiskScore float64 `json:"aggregate_risk_score"`
	// Rationale explains the aggregate decision.
	Rationale []string `json:"rationale"`
}

// RepoDecisionSummary is a summary of one repo's governance decision.
type RepoDecisionSummary struct {
	// RepoName is the repository name.
	RepoName string `json:"repo_name"`
	// Decision is this repo's governance outcome.
	Decision cgp.DecisionType `json:"decision"`
	// RiskScore is this repo's risk assessment.
	RiskScore float64 `json:"risk_score"`
	// Version is the recommended version for this repo.
	Version string `json:"version,omitempty"`
}

// FederatedNotification is a webhook notification between instances.
type FederatedNotification struct {
	// Event describes what happened (e.g., "release.published", "release.failed").
	Event string `json:"event"`
	// RepoName is the repository that triggered the event.
	RepoName string `json:"repo_name"`
	// Version is the version involved (if applicable).
	Version string `json:"version,omitempty"`
	// Details contains event-specific information.
	Details map[string]any `json:"details,omitempty"`
}

// WebhookSender sends federation messages to remote Relicta instances.
type WebhookSender interface {
	// Send delivers a federation message to the target URL.
	Send(ctx context.Context, targetURL string, msg *FederationMessage) error
}

// FederatedGovernor coordinates governance across repositories using CGP messages.
// It translates local governance events into federation messages and aggregates
// decisions from multiple repositories into a unified view.
type FederatedGovernor struct {
	// group is the repository group being governed.
	group *RepositoryGroup
	// graph is the dependency graph for the group.
	graph *DependencyGraph
	// webhookSender sends notifications to remote instances.
	webhookSender WebhookSender
	// webhookURLs maps repo names to their Relicta instance webhook URLs.
	webhookURLs map[string]string
	// pendingDecisions collects decisions from individual repos.
	pendingDecisions map[string]map[string]RepoDecisionSummary
}

// NewFederatedGovernor creates a governor for coordinating governance across repos.
func NewFederatedGovernor(
	group *RepositoryGroup,
	graph *DependencyGraph,
	opts ...FederatedGovernorOption,
) *FederatedGovernor {
	gov := &FederatedGovernor{
		group:            group,
		graph:            graph,
		webhookURLs:      make(map[string]string),
		pendingDecisions: make(map[string]map[string]RepoDecisionSummary),
	}
	for _, opt := range opts {
		opt(gov)
	}
	return gov
}

// FederatedGovernorOption configures the federated governor.
type FederatedGovernorOption func(*FederatedGovernor)

// WithWebhookSender sets the webhook sender for remote notifications.
func WithWebhookSender(sender WebhookSender) FederatedGovernorOption {
	return func(g *FederatedGovernor) {
		g.webhookSender = sender
	}
}

// WithWebhookURL registers a webhook URL for a specific repository.
func WithWebhookURL(repoName, url string) FederatedGovernorOption {
	return func(g *FederatedGovernor) {
		g.webhookURLs[repoName] = url
	}
}

// CreateProposal generates a federation proposal message when a repo
// releases changes that may affect downstream dependents.
func (g *FederatedGovernor) CreateProposal(
	sourceRepo string,
	proposal *FederatedProposal,
) ([]*FederationMessage, error) {
	affected, err := g.graph.AffectedRepos(sourceRepo)
	if err != nil {
		return nil, fmt.Errorf("finding affected repos: %w", err)
	}

	correlationID := generateFederationID()
	messages := make([]*FederationMessage, 0, len(affected))

	for _, targetRepo := range affected {
		msg := &FederationMessage{
			ID:            generateFederationID(),
			Type:          FederationProposal,
			SourceRepo:    sourceRepo,
			TargetRepo:    targetRepo,
			GroupName:     g.group.Name,
			Timestamp:     time.Now().UTC(),
			CorrelationID: correlationID,
			Proposal:      proposal,
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// SendProposals sends federation proposal messages to downstream repos.
// It uses the configured webhook sender to deliver messages.
func (g *FederatedGovernor) SendProposals(ctx context.Context, messages []*FederationMessage) error {
	if g.webhookSender == nil {
		return nil // No sender configured, skip.
	}

	var errs []string
	for _, msg := range messages {
		url, ok := g.webhookURLs[msg.TargetRepo]
		if !ok {
			continue // No webhook URL for this repo.
		}
		if err := g.webhookSender.Send(ctx, url, msg); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", msg.TargetRepo, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send proposals to: %s", joinErrors(errs))
	}
	return nil
}

// RecordDecision records a governance decision from a single repository.
// Returns the aggregated decision if all repos have reported.
func (g *FederatedGovernor) RecordDecision(
	correlationID string,
	summary RepoDecisionSummary,
) (*FederatedDecision, bool) {
	if _, ok := g.pendingDecisions[correlationID]; !ok {
		g.pendingDecisions[correlationID] = make(map[string]RepoDecisionSummary)
	}

	g.pendingDecisions[correlationID][summary.RepoName] = summary

	// Check if all repos in the group have reported.
	if len(g.pendingDecisions[correlationID]) < len(g.group.Repositories) {
		return nil, false
	}

	// All repos reported - aggregate the decision.
	aggregated := g.aggregateDecisions(correlationID)
	delete(g.pendingDecisions, correlationID)
	return aggregated, true
}

// AggregateDecisions creates an aggregated view from individually collected decisions.
func (g *FederatedGovernor) AggregateDecisions(
	decisions map[string]RepoDecisionSummary,
) *FederatedDecision {
	return g.aggregateFromMap(decisions)
}

// aggregateDecisions creates a unified decision from pending repo decisions.
func (g *FederatedGovernor) aggregateDecisions(correlationID string) *FederatedDecision {
	decisions := g.pendingDecisions[correlationID]
	return g.aggregateFromMap(decisions)
}

// aggregateFromMap aggregates decisions from a map of repo summaries.
func (g *FederatedGovernor) aggregateFromMap(decisions map[string]RepoDecisionSummary) *FederatedDecision {
	aggregated := &FederatedDecision{
		Decision:      cgp.DecisionApproved,
		RepoDecisions: decisions,
		Rationale:     make([]string, 0),
	}

	var totalRisk float64
	var repoCount int

	for _, summary := range decisions {
		totalRisk += summary.RiskScore
		repoCount++

		// The aggregate decision is the most restrictive.
		switch {
		case summary.Decision == cgp.DecisionRejected:
			aggregated.Decision = cgp.DecisionRejected
			aggregated.Rationale = append(aggregated.Rationale,
				fmt.Sprintf("repository %q rejected the release", summary.RepoName))
		case summary.Decision == cgp.DecisionApprovalRequired && aggregated.Decision != cgp.DecisionRejected:
			aggregated.Decision = cgp.DecisionApprovalRequired
			aggregated.Rationale = append(aggregated.Rationale,
				fmt.Sprintf("repository %q requires manual approval", summary.RepoName))
		case summary.Decision == cgp.DecisionDeferred && aggregated.Decision == cgp.DecisionApproved:
			aggregated.Decision = cgp.DecisionDeferred
			aggregated.Rationale = append(aggregated.Rationale,
				fmt.Sprintf("repository %q deferred decision", summary.RepoName))
		}
	}

	if repoCount > 0 {
		aggregated.AggregateRiskScore = totalRisk / float64(repoCount)
	}

	if aggregated.Decision == cgp.DecisionApproved {
		aggregated.Rationale = append(aggregated.Rationale,
			"all repositories approved the coordinated release")
	}

	return aggregated
}

// CreateNotification creates a federation notification message.
func (g *FederatedGovernor) CreateNotification(
	repoName, event, version string,
	details map[string]any,
) *FederationMessage {
	return &FederationMessage{
		ID:         generateFederationID(),
		Type:       FederationNotification,
		SourceRepo: repoName,
		GroupName:  g.group.Name,
		Timestamp:  time.Now().UTC(),
		Notification: &FederatedNotification{
			Event:    event,
			RepoName: repoName,
			Version:  version,
			Details:  details,
		},
	}
}

// CreateRiskUpdate creates a federation message to share a repo's risk state with the group.
func (g *FederatedGovernor) CreateRiskUpdate(update *FederationRiskUpdate) *FederationMessage {
	return &FederationMessage{
		ID:         generateFederationID(),
		Type:       FederationRiskExchange,
		SourceRepo: update.Repository,
		GroupName:  g.group.Name,
		Timestamp:  time.Now().UTC(),
		RiskUpdate: update,
	}
}

// BroadcastRiskUpdate sends a risk update to all other repos in the group.
func (g *FederatedGovernor) BroadcastRiskUpdate(ctx context.Context, update *FederationRiskUpdate) error {
	if g.webhookSender == nil {
		return nil
	}

	msg := g.CreateRiskUpdate(update)
	var errs []string
	for _, repo := range g.group.Repositories {
		if repo.Name == update.Repository {
			continue
		}
		url, ok := g.webhookURLs[repo.Name]
		if !ok {
			continue
		}
		targetMsg := *msg
		targetMsg.TargetRepo = repo.Name
		if err := g.webhookSender.Send(ctx, url, &targetMsg); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", repo.Name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to broadcast risk update: %s", joinErrors(errs))
	}
	return nil
}

// generateFederationID creates a unique federation message ID.
func generateFederationID() string {
	return fmt.Sprintf("fed_%s", uuid.New().String()[:12])
}

// joinErrors joins error strings with commas.
func joinErrors(errs []string) string {
	result := ""
	for i, e := range errs {
		if i > 0 {
			result += ", "
		}
		result += e
	}
	return result
}
