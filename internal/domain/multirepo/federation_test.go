package multirepo

import (
	"context"
	"fmt"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// mockWebhookSender records sent messages for testing.
type mockWebhookSender struct {
	sent    []*FederationMessage
	failFor map[string]bool
}

func (m *mockWebhookSender) Send(_ context.Context, targetURL string, msg *FederationMessage) error {
	if m.failFor[targetURL] {
		return fmt.Errorf("webhook delivery failed for %s", targetURL)
	}
	m.sent = append(m.sent, msg)
	return nil
}

func newTestFederatedGovernor(t *testing.T) (*FederatedGovernor, *RepositoryGroup, *DependencyGraph) {
	t.Helper()

	group := &RepositoryGroup{
		Name:     "platform",
		Strategy: StrategyCoordinated,
		Repositories: []RepoConfig{
			{Name: "core-lib"},
			{Name: "auth-service", Dependencies: []string{"core-lib"}},
			{Name: "api-gateway", Dependencies: []string{"auth-service"}},
		},
	}

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("failed to build graph: %v", err)
	}

	gov := NewFederatedGovernor(group, graph)
	return gov, group, graph
}

func TestFederatedGovernor_CreateProposal(t *testing.T) {
	gov, _, _ := newTestFederatedGovernor(t)

	proposal := &FederatedProposal{
		ChangeDescription: "Updated authentication middleware",
		ProposedVersion:   "2.0.0",
		BreakingChanges:   true,
		RiskScore:         0.7,
		AffectedAPIs:      []string{"/auth/login", "/auth/token"},
	}

	messages, err := gov.CreateProposal("core-lib", proposal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// core-lib affects auth-service and api-gateway.
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// All messages should have the same correlation ID.
	correlationID := messages[0].CorrelationID
	for _, msg := range messages {
		if msg.CorrelationID != correlationID {
			t.Errorf("expected same correlation ID, got %q and %q", correlationID, msg.CorrelationID)
		}
		if msg.Type != FederationProposal {
			t.Errorf("expected type %q, got %q", FederationProposal, msg.Type)
		}
		if msg.SourceRepo != "core-lib" {
			t.Errorf("expected source repo 'core-lib', got %q", msg.SourceRepo)
		}
		if msg.Proposal == nil {
			t.Error("expected proposal payload")
		}
		if msg.Proposal.BreakingChanges != true {
			t.Error("expected breaking changes flag")
		}
	}

	// Verify target repos.
	targets := make(map[string]bool)
	for _, msg := range messages {
		targets[msg.TargetRepo] = true
	}
	if !targets["auth-service"] || !targets["api-gateway"] {
		t.Errorf("expected targets [auth-service, api-gateway], got %v", targets)
	}
}

func TestFederatedGovernor_CreateProposal_LeafRepo(t *testing.T) {
	gov, _, _ := newTestFederatedGovernor(t)

	proposal := &FederatedProposal{
		ChangeDescription: "Minor UI change",
		ProposedVersion:   "1.0.1",
	}

	messages, err := gov.CreateProposal("api-gateway", proposal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// api-gateway is a leaf node, so no downstream repos.
	if len(messages) != 0 {
		t.Errorf("expected 0 messages for leaf repo, got %d", len(messages))
	}
}

func TestFederatedGovernor_SendProposals(t *testing.T) {
	sender := &mockWebhookSender{failFor: make(map[string]bool)}

	group := &RepositoryGroup{
		Name:     "platform",
		Strategy: StrategyCoordinated,
		Repositories: []RepoConfig{
			{Name: "core-lib"},
			{Name: "auth-service", Dependencies: []string{"core-lib"}},
		},
	}

	graph, _ := NewDependencyGraph(group)
	gov := NewFederatedGovernor(group, graph,
		WithWebhookSender(sender),
		WithWebhookURL("auth-service", "https://auth.example.com/webhook"),
	)

	messages, _ := gov.CreateProposal("core-lib", &FederatedProposal{
		ChangeDescription: "test change",
		ProposedVersion:   "1.0.0",
	})

	err := gov.SendProposals(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Errorf("expected 1 sent message, got %d", len(sender.sent))
	}
}

func TestFederatedGovernor_SendProposals_PartialFailure(t *testing.T) {
	sender := &mockWebhookSender{
		failFor: map[string]bool{
			"https://auth.example.com/webhook": true,
		},
	}

	group := &RepositoryGroup{
		Name:     "platform",
		Strategy: StrategyCoordinated,
		Repositories: []RepoConfig{
			{Name: "core-lib"},
			{Name: "auth-service", Dependencies: []string{"core-lib"}},
		},
	}

	graph, _ := NewDependencyGraph(group)
	gov := NewFederatedGovernor(group, graph,
		WithWebhookSender(sender),
		WithWebhookURL("auth-service", "https://auth.example.com/webhook"),
	)

	messages, _ := gov.CreateProposal("core-lib", &FederatedProposal{
		ChangeDescription: "test",
		ProposedVersion:   "1.0.0",
	})

	err := gov.SendProposals(context.Background(), messages)
	if err == nil {
		t.Fatal("expected error for failed delivery")
	}
}

func TestFederatedGovernor_RecordDecision_Aggregation(t *testing.T) {
	gov, _, _ := newTestFederatedGovernor(t)

	correlationID := "test-corr-001"

	// Record first decision - not complete yet.
	_, complete := gov.RecordDecision(correlationID, RepoDecisionSummary{
		RepoName:  "core-lib",
		Decision:  cgp.DecisionApproved,
		RiskScore: 0.2,
		Version:   "1.0.0",
	})
	if complete {
		t.Error("should not be complete with 1/3 decisions")
	}

	// Record second decision.
	_, complete = gov.RecordDecision(correlationID, RepoDecisionSummary{
		RepoName:  "auth-service",
		Decision:  cgp.DecisionApproved,
		RiskScore: 0.3,
		Version:   "2.0.0",
	})
	if complete {
		t.Error("should not be complete with 2/3 decisions")
	}

	// Record third decision - now complete.
	aggregated, complete := gov.RecordDecision(correlationID, RepoDecisionSummary{
		RepoName:  "api-gateway",
		Decision:  cgp.DecisionApproved,
		RiskScore: 0.1,
		Version:   "3.0.0",
	})
	if !complete {
		t.Fatal("should be complete with 3/3 decisions")
	}

	if aggregated.Decision != cgp.DecisionApproved {
		t.Errorf("expected approved, got %s", aggregated.Decision)
	}

	// Average risk: (0.2 + 0.3 + 0.1) / 3 = 0.2
	expectedRisk := 0.2
	if aggregated.AggregateRiskScore < expectedRisk-0.01 || aggregated.AggregateRiskScore > expectedRisk+0.01 {
		t.Errorf("expected risk ~%f, got %f", expectedRisk, aggregated.AggregateRiskScore)
	}
}

func TestFederatedGovernor_AggregateDecisions_MostRestrictive(t *testing.T) {
	tests := []struct {
		name         string
		decisions    map[string]RepoDecisionSummary
		wantDecision cgp.DecisionType
	}{
		{
			name: "all approved",
			decisions: map[string]RepoDecisionSummary{
				"a": {RepoName: "a", Decision: cgp.DecisionApproved, RiskScore: 0.1},
				"b": {RepoName: "b", Decision: cgp.DecisionApproved, RiskScore: 0.2},
			},
			wantDecision: cgp.DecisionApproved,
		},
		{
			name: "one requires approval escalates all",
			decisions: map[string]RepoDecisionSummary{
				"a": {RepoName: "a", Decision: cgp.DecisionApproved, RiskScore: 0.1},
				"b": {RepoName: "b", Decision: cgp.DecisionApprovalRequired, RiskScore: 0.5},
			},
			wantDecision: cgp.DecisionApprovalRequired,
		},
		{
			name: "rejection overrides approval required",
			decisions: map[string]RepoDecisionSummary{
				"a": {RepoName: "a", Decision: cgp.DecisionApprovalRequired, RiskScore: 0.5},
				"b": {RepoName: "b", Decision: cgp.DecisionRejected, RiskScore: 0.9},
			},
			wantDecision: cgp.DecisionRejected,
		},
		{
			name: "deferred escalates from approved",
			decisions: map[string]RepoDecisionSummary{
				"a": {RepoName: "a", Decision: cgp.DecisionApproved, RiskScore: 0.1},
				"b": {RepoName: "b", Decision: cgp.DecisionDeferred, RiskScore: 0.3},
			},
			wantDecision: cgp.DecisionDeferred,
		},
	}

	gov, _, _ := newTestFederatedGovernor(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gov.AggregateDecisions(tt.decisions)
			if result.Decision != tt.wantDecision {
				t.Errorf("got decision %s, want %s", result.Decision, tt.wantDecision)
			}
		})
	}
}

func TestFederatedGovernor_CreateNotification(t *testing.T) {
	gov, _, _ := newTestFederatedGovernor(t)

	msg := gov.CreateNotification("auth-service", "release.published", "2.0.0", map[string]any{
		"commit": "abc123",
	})

	if msg.Type != FederationNotification {
		t.Errorf("expected type %s, got %s", FederationNotification, msg.Type)
	}
	if msg.Notification.Event != "release.published" {
		t.Errorf("expected event 'release.published', got %q", msg.Notification.Event)
	}
	if msg.Notification.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got %q", msg.Notification.Version)
	}
	if msg.GroupName != "platform" {
		t.Errorf("expected group 'platform', got %q", msg.GroupName)
	}
}
