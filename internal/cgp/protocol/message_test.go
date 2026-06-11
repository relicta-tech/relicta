package protocol

import (
	"encoding/json"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

func TestNewMessage(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test User"),
		cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		},
		cgp.ProposalIntent{
			Summary:    "Test change",
			Confidence: 0.8,
		},
	)

	msg, err := NewMessage(cgp.MessageTypeProposal, proposal)
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	if msg.Header.Type != cgp.MessageTypeProposal {
		t.Errorf("Header.Type = %s, want %s", msg.Header.Type, cgp.MessageTypeProposal)
	}
	if msg.Header.CGPVersion != cgp.Version {
		t.Errorf("Header.CGPVersion = %s, want %s", msg.Header.CGPVersion, cgp.Version)
	}
	if msg.Header.MessageID == "" {
		t.Error("Header.MessageID should not be empty")
	}
	if msg.Header.Timestamp.IsZero() {
		t.Error("Header.Timestamp should not be zero")
	}
	if len(msg.Payload) == 0 {
		t.Error("Payload should not be empty")
	}
}

func TestNewProposalMessage(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test User"),
		cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		},
		cgp.ProposalIntent{
			Summary:    "Test change",
			Confidence: 0.8,
		},
	)

	msg, err := NewProposalMessage(proposal)
	if err != nil {
		t.Fatalf("NewProposalMessage failed: %v", err)
	}

	if msg.Header.CorrelationID != proposal.ID {
		t.Errorf("CorrelationID = %s, want %s", msg.Header.CorrelationID, proposal.ID)
	}
}

func TestNewDecisionMessage(t *testing.T) {
	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved)

	msg, err := NewDecisionMessage(decision)
	if err != nil {
		t.Fatalf("NewDecisionMessage failed: %v", err)
	}

	if msg.Header.CorrelationID != decision.ProposalID {
		t.Errorf("CorrelationID = %s, want %s", msg.Header.CorrelationID, decision.ProposalID)
	}
}

func TestMessage_EncodeAndDecode(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test User"),
		cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		},
		cgp.ProposalIntent{
			Summary:    "Test change",
			Confidence: 0.8,
		},
	)

	msg, err := NewProposalMessage(proposal)
	if err != nil {
		t.Fatalf("NewProposalMessage failed: %v", err)
	}

	// Encode
	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	if decoded.Header.MessageID != msg.Header.MessageID {
		t.Errorf("Decoded MessageID = %s, want %s", decoded.Header.MessageID, msg.Header.MessageID)
	}
	if decoded.Header.Type != msg.Header.Type {
		t.Errorf("Decoded Type = %s, want %s", decoded.Header.Type, msg.Header.Type)
	}
}

func TestMessage_AsProposal(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test User"),
		cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		},
		cgp.ProposalIntent{
			Summary:    "Test change",
			Confidence: 0.8,
		},
	)

	msg, _ := NewProposalMessage(proposal)

	decoded, err := msg.AsProposal()
	if err != nil {
		t.Fatalf("AsProposal failed: %v", err)
	}

	if decoded.ID != proposal.ID {
		t.Errorf("Decoded ID = %s, want %s", decoded.ID, proposal.ID)
	}
	if decoded.Intent.Summary != proposal.Intent.Summary {
		t.Errorf("Decoded Summary = %s, want %s", decoded.Intent.Summary, proposal.Intent.Summary)
	}
}

func TestMessage_AsProposal_WrongType(t *testing.T) {
	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved)
	msg, _ := NewDecisionMessage(decision)

	_, err := msg.AsProposal()
	if err == nil {
		t.Error("AsProposal should fail for non-proposal message")
	}
}

func TestMessage_AsDecision(t *testing.T) {
	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved).
		WithRiskScore(0.3).
		AddRationale("Low risk change")

	msg, _ := NewDecisionMessage(decision)

	decoded, err := msg.AsDecision()
	if err != nil {
		t.Fatalf("AsDecision failed: %v", err)
	}

	if decoded.ID != decision.ID {
		t.Errorf("Decoded ID = %s, want %s", decoded.ID, decision.ID)
	}
	if decoded.Decision != cgp.DecisionApproved {
		t.Errorf("Decoded Decision = %s, want %s", decoded.Decision, cgp.DecisionApproved)
	}
}

func TestMessage_AsEvaluation(t *testing.T) {
	eval := cgp.NewEvaluation("proposal-123").
		WithRiskAssessment(&cgp.RiskAssessment{
			OverallScore: 0.4,
			Severity:     cgp.SeverityMedium,
		})

	msg, _ := NewEvaluationMessage(eval)

	decoded, err := msg.AsEvaluation()
	if err != nil {
		t.Fatalf("AsEvaluation failed: %v", err)
	}

	if decoded.ID != eval.ID {
		t.Errorf("Decoded ID = %s, want %s", decoded.ID, eval.ID)
	}
	if decoded.RiskAssessment.OverallScore != 0.4 {
		t.Errorf("Decoded OverallScore = %f, want 0.4", decoded.RiskAssessment.OverallScore)
	}
}

func TestMessage_WithSource(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)

	msg, _ := NewProposalMessage(proposal)
	msg.WithSource("cli").WithDestination("governance")

	if msg.Header.Source != "cli" {
		t.Errorf("Source = %s, want cli", msg.Header.Source)
	}
	if msg.Header.Destination != "governance" {
		t.Errorf("Destination = %s, want governance", msg.Header.Destination)
	}
}

func TestMessageChain(t *testing.T) {
	chain := NewMessageChain("correlation-123")

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)
	proposalMsg, _ := NewProposalMessage(proposal)
	chain.Add(proposalMsg)

	decision := cgp.NewDecision(proposal.ID, cgp.DecisionApproved)
	decisionMsg, _ := NewDecisionMessage(decision)
	chain.Add(decisionMsg)

	if chain.Len() != 2 {
		t.Errorf("Chain Len() = %d, want 2", chain.Len())
	}

	proposals := chain.GetByType(cgp.MessageTypeProposal)
	if len(proposals) != 1 {
		t.Errorf("GetByType(proposal) = %d, want 1", len(proposals))
	}

	decisions := chain.GetByType(cgp.MessageTypeDecision)
	if len(decisions) != 1 {
		t.Errorf("GetByType(decision) = %d, want 1", len(decisions))
	}

	// All messages should have the chain's correlation ID
	for _, msg := range chain.Messages {
		if msg.Header.CorrelationID != chain.CorrelationID {
			t.Errorf("Message CorrelationID = %s, want %s", msg.Header.CorrelationID, chain.CorrelationID)
		}
	}
}

func TestMessageChain_GetProposal(t *testing.T) {
	chain := NewMessageChain("correlation-123")

	// Empty chain
	_, err := chain.GetProposal()
	if err == nil {
		t.Error("GetProposal should fail on empty chain")
	}

	// Add proposal
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)
	proposalMsg, _ := NewProposalMessage(proposal)
	chain.Add(proposalMsg)

	decoded, err := chain.GetProposal()
	if err != nil {
		t.Fatalf("GetProposal failed: %v", err)
	}
	if decoded.ID != proposal.ID {
		t.Errorf("GetProposal ID = %s, want %s", decoded.ID, proposal.ID)
	}
}

func TestMessageChain_GetDecision(t *testing.T) {
	chain := NewMessageChain("correlation-123")

	// Empty chain
	_, err := chain.GetDecision()
	if err == nil {
		t.Error("GetDecision should fail on empty chain")
	}

	// Add two decisions (should return the last one)
	decision1 := cgp.NewDecision("proposal-123", cgp.DecisionDeferred)
	decision2 := cgp.NewDecision("proposal-123", cgp.DecisionApproved)

	msg1, _ := NewDecisionMessage(decision1)
	msg2, _ := NewDecisionMessage(decision2)
	chain.Add(msg1)
	chain.Add(msg2)

	decoded, err := chain.GetDecision()
	if err != nil {
		t.Fatalf("GetDecision failed: %v", err)
	}
	// Should return the last decision
	if decoded.Decision != cgp.DecisionApproved {
		t.Errorf("GetDecision Decision = %s, want %s", decoded.Decision, cgp.DecisionApproved)
	}
}

func TestDecodeMessage_InvalidJSON(t *testing.T) {
	_, err := DecodeMessage([]byte("invalid json"))
	if err == nil {
		t.Error("DecodeMessage should fail on invalid JSON")
	}
}

func TestEncodePretty(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)

	msg, _ := NewProposalMessage(proposal)
	data, err := msg.EncodePretty()
	if err != nil {
		t.Fatalf("EncodePretty failed: %v", err)
	}

	// Pretty printed JSON should contain newlines
	if len(data) == 0 {
		t.Error("EncodePretty returned empty data")
	}

	// Should be valid JSON
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("EncodePretty output is not valid JSON: %v", err)
	}
}

func TestNewAuthorizationMessage(t *testing.T) {
	actor := cgp.NewHumanActor("test@example.com", "Test User")
	auth := cgp.NewAuthorization("decision-456", "proposal-123", actor, "1.0.0")

	msg, err := NewAuthorizationMessage(auth)
	if err != nil {
		t.Fatalf("NewAuthorizationMessage failed: %v", err)
	}

	if msg.Header.Type != cgp.MessageTypeAuthorization {
		t.Errorf("Header.Type = %s, want %s", msg.Header.Type, cgp.MessageTypeAuthorization)
	}
	if msg.Header.CorrelationID != auth.ProposalID {
		t.Errorf("CorrelationID = %s, want %s", msg.Header.CorrelationID, auth.ProposalID)
	}
}

func TestMessage_WithCorrelationID(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)

	msg, _ := NewMessage(cgp.MessageTypeProposal, proposal)

	// Set correlation ID
	result := msg.WithCorrelationID("custom-correlation-123")

	// Should return the same message for chaining
	if result != msg {
		t.Error("WithCorrelationID should return the same message")
	}

	if msg.Header.CorrelationID != "custom-correlation-123" {
		t.Errorf("CorrelationID = %s, want custom-correlation-123", msg.Header.CorrelationID)
	}
}

func TestMessage_AsAuthorization(t *testing.T) {
	actor := cgp.NewHumanActor("test@example.com", "Test User")
	auth := cgp.NewAuthorization("decision-456", "proposal-123", actor, "1.0.0")

	msg, _ := NewAuthorizationMessage(auth)

	decoded, err := msg.AsAuthorization()
	if err != nil {
		t.Fatalf("AsAuthorization failed: %v", err)
	}

	if decoded.ProposalID != auth.ProposalID {
		t.Errorf("ProposalID = %s, want %s", decoded.ProposalID, auth.ProposalID)
	}
	if decoded.DecisionID != auth.DecisionID {
		t.Errorf("DecisionID = %s, want %s", decoded.DecisionID, auth.DecisionID)
	}
	if decoded.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", decoded.Version)
	}
}

func TestMessage_AsAuthorization_WrongType(t *testing.T) {
	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved)
	msg, _ := NewDecisionMessage(decision)

	_, err := msg.AsAuthorization()
	if err == nil {
		t.Error("AsAuthorization should fail for non-authorization message")
	}
}

func TestMessage_AsEvaluation_WrongType(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)
	msg, _ := NewProposalMessage(proposal)

	_, err := msg.AsEvaluation()
	if err == nil {
		t.Error("AsEvaluation should fail for non-evaluation message")
	}
}

func TestMessage_AsDecision_WrongType(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)
	msg, _ := NewProposalMessage(proposal)

	_, err := msg.AsDecision()
	if err == nil {
		t.Error("AsDecision should fail for non-decision message")
	}
}
