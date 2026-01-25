package audit

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestEntry_ComputeHash(t *testing.T) {
	entry := &Entry{
		ID:           "test-1",
		Timestamp:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventType:    EventDecisionMade,
		ProposalID:   "proposal-123",
		ActorID:      "user@example.com",
		ActorKind:    cgp.ActorKindHuman,
		PreviousHash: "",
	}

	hash1 := entry.ComputeHash()
	if hash1 == "" {
		t.Error("ComputeHash returned empty string")
	}

	// Same entry should produce same hash
	hash2 := entry.ComputeHash()
	if hash1 != hash2 {
		t.Error("ComputeHash is not deterministic")
	}

	// Different data should produce different hash
	entry.ActorID = "other@example.com"
	hash3 := entry.ComputeHash()
	if hash1 == hash3 {
		t.Error("Different data produced same hash")
	}
}

func TestEntry_Verify(t *testing.T) {
	entry := &Entry{
		ID:           "test-1",
		Timestamp:    time.Now().UTC(),
		EventType:    EventDecisionMade,
		ProposalID:   "proposal-123",
		ActorID:      "user@example.com",
		ActorKind:    cgp.ActorKindHuman,
		PreviousHash: "",
	}

	// Set correct hash
	entry.Hash = entry.ComputeHash()

	if !entry.Verify() {
		t.Error("Verify returned false for valid entry")
	}

	// Tamper with entry
	entry.ActorID = "hacker@example.com"
	if entry.Verify() {
		t.Error("Verify returned true for tampered entry")
	}
}

func TestChain_Append(t *testing.T) {
	chain := NewChain()

	// Append first entry (genesis)
	entry1 := NewEntry("entry-1", EventProposalReceived).
		WithProposal("proposal-1").
		WithActor("user@example.com", cgp.ActorKindHuman).
		Build()

	err := chain.Append(entry1)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if entry1.PreviousHash != "" {
		t.Error("Genesis entry should have empty previous hash")
	}
	if entry1.Hash == "" {
		t.Error("Entry hash not set after append")
	}

	// Append second entry
	entry2 := NewEntry("entry-2", EventDecisionMade).
		WithProposal("proposal-1").
		WithActor("user@example.com", cgp.ActorKindHuman).
		Build()

	err = chain.Append(entry2)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	if entry2.PreviousHash != entry1.Hash {
		t.Errorf("Entry2 previous hash = %s, want %s", entry2.PreviousHash, entry1.Hash)
	}

	if chain.Len() != 2 {
		t.Errorf("Chain length = %d, want 2", chain.Len())
	}
}

func TestChain_AppendDuplicate(t *testing.T) {
	chain := NewChain()

	entry := NewEntry("entry-1", EventProposalReceived).
		WithProposal("proposal-1").
		Build()

	err := chain.Append(entry)
	if err != nil {
		t.Fatalf("First append failed: %v", err)
	}

	// Try to append duplicate
	duplicate := NewEntry("entry-1", EventDecisionMade).
		WithProposal("proposal-2").
		Build()

	err = chain.Append(duplicate)
	if err != ErrDuplicateEntry {
		t.Errorf("Expected ErrDuplicateEntry, got %v", err)
	}
}

func TestChain_Verify(t *testing.T) {
	chain := NewChain()

	// Build a valid chain
	for i := 0; i < 5; i++ {
		entry := NewEntry(
			"entry-"+string(rune('a'+i)),
			EventDecisionMade,
		).
			WithProposal("proposal-1").
			WithActor("user@example.com", cgp.ActorKindHuman).
			Build()

		if err := chain.Append(entry); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Verify should pass
	if err := chain.Verify(); err != nil {
		t.Errorf("Verify failed on valid chain: %v", err)
	}
}

func TestChain_VerifyTamperedHash(t *testing.T) {
	chain := NewChain()

	entry1 := NewEntry("entry-1", EventProposalReceived).
		WithProposal("proposal-1").
		Build()
	chain.Append(entry1)

	entry2 := NewEntry("entry-2", EventDecisionMade).
		WithProposal("proposal-1").
		Build()
	chain.Append(entry2)

	// Tamper with first entry's hash
	chain.entries[0].Hash = "tampered-hash"

	err := chain.Verify()
	if err == nil {
		t.Error("Verify should fail on tampered chain")
	}
}

func TestChain_VerifyBrokenLinkage(t *testing.T) {
	chain := NewChain()

	entry1 := NewEntry("entry-1", EventProposalReceived).
		WithProposal("proposal-1").
		Build()
	chain.Append(entry1)

	entry2 := NewEntry("entry-2", EventDecisionMade).
		WithProposal("proposal-1").
		Build()
	chain.Append(entry2)

	// Break the chain linkage
	chain.entries[1].PreviousHash = "wrong-hash"
	// Recompute hash to make it valid on its own
	chain.entries[1].Hash = chain.entries[1].ComputeHash()

	err := chain.Verify()
	if err == nil {
		t.Error("Verify should fail on broken linkage")
	}
}

func TestChain_GetByProposal(t *testing.T) {
	chain := NewChain()

	// Add entries for different proposals
	chain.Append(NewEntry("e1", EventProposalReceived).WithProposal("p1").Build())
	chain.Append(NewEntry("e2", EventDecisionMade).WithProposal("p1").Build())
	chain.Append(NewEntry("e3", EventProposalReceived).WithProposal("p2").Build())
	chain.Append(NewEntry("e4", EventExecutionAuthorized).WithProposal("p1").Build())

	// Query proposal 1
	p1Entries := chain.GetByProposal("p1")
	if len(p1Entries) != 3 {
		t.Errorf("GetByProposal(p1) returned %d entries, want 3", len(p1Entries))
	}

	// Query proposal 2
	p2Entries := chain.GetByProposal("p2")
	if len(p2Entries) != 1 {
		t.Errorf("GetByProposal(p2) returned %d entries, want 1", len(p2Entries))
	}

	// Query non-existent proposal
	p3Entries := chain.GetByProposal("p3")
	if len(p3Entries) != 0 {
		t.Errorf("GetByProposal(p3) returned %d entries, want 0", len(p3Entries))
	}
}

func TestChain_Search(t *testing.T) {
	chain := NewChain()

	now := time.Now().UTC()

	// Add entries with different attributes
	chain.Append(NewEntry("e1", EventProposalReceived).
		WithProposal("p1").
		WithActor("user1", cgp.ActorKindHuman).
		WithTimestamp(now.Add(-2 * time.Hour)).
		Build())

	chain.Append(NewEntry("e2", EventDecisionMade).
		WithProposal("p1").
		WithActor("user2", cgp.ActorKindCI).
		WithTimestamp(now.Add(-1 * time.Hour)).
		Build())

	chain.Append(NewEntry("e3", EventExecutionCompleted).
		WithProposal("p2").
		WithActor("user1", cgp.ActorKindHuman).
		WithTimestamp(now).
		Build())

	// Search by event type
	results := chain.Search(Query{EventType: EventDecisionMade})
	if len(results) != 1 {
		t.Errorf("Search by event type returned %d, want 1", len(results))
	}

	// Search by actor
	results = chain.Search(Query{ActorID: "user1"})
	if len(results) != 2 {
		t.Errorf("Search by actor returned %d, want 2", len(results))
	}

	// Search by time range
	results = chain.Search(Query{
		From: now.Add(-90 * time.Minute),
		To:   now.Add(-30 * time.Minute),
	})
	if len(results) != 1 {
		t.Errorf("Search by time range returned %d, want 1", len(results))
	}

	// Search with limit
	results = chain.Search(Query{Limit: 2})
	if len(results) != 2 {
		t.Errorf("Search with limit returned %d, want 2", len(results))
	}

	// Search with offset
	results = chain.Search(Query{Offset: 1, Limit: 2})
	if len(results) != 2 {
		t.Errorf("Search with offset returned %d, want 2", len(results))
	}
}

func TestFromDecision(t *testing.T) {
	decision := &cgp.GovernanceDecision{
		ID:         "decision-123",
		ProposalID: "proposal-456",
		Timestamp:  time.Now().UTC(),
		Decision:   cgp.DecisionApproved,
		RiskScore:  0.35,
		RiskFactors: []cgp.RiskFactor{
			{Category: "api_change", Score: 0.5},
		},
		Rationale: []string{"Low risk change"},
	}

	entry := FromDecision(decision)

	if entry.EventType != EventDecisionMade {
		t.Errorf("EventType = %s, want %s", entry.EventType, EventDecisionMade)
	}
	if entry.ProposalID != decision.ProposalID {
		t.Errorf("ProposalID = %s, want %s", entry.ProposalID, decision.ProposalID)
	}
	if entry.Details["decisionType"] != cgp.DecisionApproved {
		t.Errorf("decisionType = %v, want %v", entry.Details["decisionType"], cgp.DecisionApproved)
	}
	if entry.Details["riskScore"] != 0.35 {
		t.Errorf("riskScore = %v, want 0.35", entry.Details["riskScore"])
	}
}

func TestFromAuthorization(t *testing.T) {
	auth := &cgp.ExecutionAuthorization{
		ID:         "auth-123",
		ProposalID: "proposal-456",
		ApprovedBy: cgp.NewHumanActor("approver@example.com", "Approver"),
		ApprovedAt: time.Now().UTC(),
		Version:    "1.2.3",
		AllowedSteps: []cgp.ExecutionStep{
			cgp.ExecutionStepTag,
			cgp.ExecutionStepChangelog,
		},
	}

	entry := FromAuthorization(auth)

	if entry.EventType != EventExecutionAuthorized {
		t.Errorf("EventType = %s, want %s", entry.EventType, EventExecutionAuthorized)
	}
	if entry.ActorID != "human:approver@example.com" {
		t.Errorf("ActorID = %s, want human:approver@example.com", entry.ActorID)
	}
	if entry.Details["version"] != "1.2.3" {
		t.Errorf("version = %v, want 1.2.3", entry.Details["version"])
	}
}

func TestFromApproval(t *testing.T) {
	auth := &cgp.ExecutionAuthorization{
		ID:         "auth-123",
		ProposalID: "proposal-456",
	}

	// Test approve action
	approval := cgp.ApprovalRecord{
		Actor:     cgp.NewHumanActor("reviewer@example.com", "Reviewer"),
		Action:    "approve",
		Timestamp: time.Now().UTC(),
		Comment:   "LGTM",
	}

	entry := FromApproval(auth, approval)

	if entry.EventType != EventApprovalGranted {
		t.Errorf("EventType = %s, want %s", entry.EventType, EventApprovalGranted)
	}
	if entry.Details["comment"] != "LGTM" {
		t.Errorf("comment = %v, want LGTM", entry.Details["comment"])
	}

	// Test reject action
	rejection := cgp.ApprovalRecord{
		Actor:     cgp.NewHumanActor("reviewer@example.com", "Reviewer"),
		Action:    "reject",
		Timestamp: time.Now().UTC(),
	}

	entry = FromApproval(auth, rejection)
	if entry.EventType != EventApprovalDenied {
		t.Errorf("EventType = %s, want %s", entry.EventType, EventApprovalDenied)
	}
}

func TestChain_LastHash(t *testing.T) {
	chain := NewChain()

	// Empty chain
	if chain.LastHash() != "" {
		t.Error("LastHash should be empty for empty chain")
	}

	// Add entry
	entry := NewEntry("e1", EventProposalReceived).WithProposal("p1").Build()
	chain.Append(entry)

	if chain.LastHash() != entry.Hash {
		t.Errorf("LastHash = %s, want %s", chain.LastHash(), entry.Hash)
	}
}

func TestEntryBuilder(t *testing.T) {
	entry := NewEntry("test-id", EventDecisionMade).
		WithProposal("proposal-123").
		WithActor("actor-456", cgp.ActorKindCI).
		WithDetail("key1", "value1").
		WithDetail("key2", 42).
		Build()

	if entry.ID != "test-id" {
		t.Errorf("ID = %s, want test-id", entry.ID)
	}
	if entry.EventType != EventDecisionMade {
		t.Errorf("EventType = %s, want %s", entry.EventType, EventDecisionMade)
	}
	if entry.ProposalID != "proposal-123" {
		t.Errorf("ProposalID = %s, want proposal-123", entry.ProposalID)
	}
	if entry.ActorID != "actor-456" {
		t.Errorf("ActorID = %s, want actor-456", entry.ActorID)
	}
	if entry.Details["key1"] != "value1" {
		t.Errorf("Details[key1] = %v, want value1", entry.Details["key1"])
	}
	if entry.Details["key2"] != 42 {
		t.Errorf("Details[key2] = %v, want 42", entry.Details["key2"])
	}
}
