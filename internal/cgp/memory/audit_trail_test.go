package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestInMemoryStore_RecordDecision(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	decision := cgp.NewDecision("prop_123", cgp.DecisionApproved)
	decision.WithRiskScore(0.3)
	decision.AddRationale("Low risk release")

	err := store.RecordDecision(ctx, decision)
	require.NoError(t, err)

	// Retrieve the decision
	retrieved, err := store.GetDecision(ctx, decision.ID)
	require.NoError(t, err)
	assert.Equal(t, decision.ID, retrieved.ID)
	assert.Equal(t, decision.ProposalID, retrieved.ProposalID)
	assert.Equal(t, cgp.DecisionApproved, retrieved.Decision)
	assert.Equal(t, 0.3, retrieved.RiskScore)
}

func TestInMemoryStore_RecordDecision_Validation(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Nil decision
	err := store.RecordDecision(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decision is required")

	// Empty ID
	decision := &cgp.GovernanceDecision{}
	err = store.RecordDecision(ctx, decision)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decision ID is required")
}

func TestInMemoryStore_GetDecisionsByProposal(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create multiple decisions for the same proposal
	prop1 := "prop_123"
	d1 := cgp.NewDecision(prop1, cgp.DecisionApprovalRequired)
	d2 := cgp.NewDecision(prop1, cgp.DecisionApproved)
	d3 := cgp.NewDecision("prop_456", cgp.DecisionApproved)

	require.NoError(t, store.RecordDecision(ctx, d1))
	require.NoError(t, store.RecordDecision(ctx, d2))
	require.NoError(t, store.RecordDecision(ctx, d3))

	// Get decisions for prop1
	decisions, err := store.GetDecisionsByProposal(ctx, prop1)
	require.NoError(t, err)
	assert.Len(t, decisions, 2)

	// Get decisions for prop_456
	decisions, err = store.GetDecisionsByProposal(ctx, "prop_456")
	require.NoError(t, err)
	assert.Len(t, decisions, 1)

	// Get decisions for non-existent proposal
	decisions, err = store.GetDecisionsByProposal(ctx, "prop_999")
	require.NoError(t, err)
	assert.Len(t, decisions, 0)
}

func TestInMemoryStore_RecordAuthorization(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	actor := cgp.Actor{
		ID:   "user_123",
		Kind: cgp.ActorKindHuman,
		Name: "Test User",
	}

	auth := cgp.NewAuthorization("dec_123", "prop_123", actor, "1.2.0")
	auth.WithReleaseNotes("Release notes")
	auth.RecordApproval(actor, cgp.ApprovalActionApprove, "Looks good")

	err := store.RecordAuthorization(ctx, auth)
	require.NoError(t, err)

	// Retrieve the authorization
	retrieved, err := store.GetAuthorization(ctx, auth.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.ID, retrieved.ID)
	assert.Equal(t, auth.DecisionID, retrieved.DecisionID)
	assert.Equal(t, "1.2.0", retrieved.Version)
	assert.Equal(t, "Release notes", retrieved.ReleaseNotes)
	assert.Len(t, retrieved.ApprovalChain, 1)
}

func TestInMemoryStore_RecordAuthorization_Validation(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Nil authorization
	err := store.RecordAuthorization(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization is required")

	// Empty ID
	auth := &cgp.ExecutionAuthorization{}
	err = store.RecordAuthorization(ctx, auth)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization ID is required")
}

func TestInMemoryStore_GetAuthorizationsByDecision(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	actor := cgp.Actor{ID: "user_123", Kind: cgp.ActorKindHuman}

	// Create authorizations for different decisions
	auth1 := cgp.NewAuthorization("dec_123", "prop_1", actor, "1.0.0")
	auth2 := cgp.NewAuthorization("dec_123", "prop_1", actor, "1.0.1") // Same decision
	auth3 := cgp.NewAuthorization("dec_456", "prop_2", actor, "2.0.0")

	require.NoError(t, store.RecordAuthorization(ctx, auth1))
	require.NoError(t, store.RecordAuthorization(ctx, auth2))
	require.NoError(t, store.RecordAuthorization(ctx, auth3))

	// Get authorizations for dec_123
	auths, err := store.GetAuthorizationsByDecision(ctx, "dec_123")
	require.NoError(t, err)
	assert.Len(t, auths, 2)

	// Get authorizations for dec_456
	auths, err = store.GetAuthorizationsByDecision(ctx, "dec_456")
	require.NoError(t, err)
	assert.Len(t, auths, 1)
}

func TestInMemoryStore_GetAuditTrail(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	proposalID := "prop_123"
	actor := cgp.Actor{ID: "user_123", Kind: cgp.ActorKindHuman, Name: "Test User"}

	// Create governance decision
	decision := cgp.NewDecision(proposalID, cgp.DecisionApprovalRequired)
	decision.WithRiskScore(0.5)
	decision.AddRationale("Needs review due to breaking changes")
	require.NoError(t, store.RecordDecision(ctx, decision))

	// Wait a bit for timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Create authorization
	auth := cgp.NewAuthorization(decision.ID, proposalID, actor, "2.0.0")
	auth.RecordApproval(actor, cgp.ApprovalActionApprove, "Approved after review")
	require.NoError(t, store.RecordAuthorization(ctx, auth))

	// Get audit trail
	trail, err := store.GetAuditTrail(ctx, proposalID)
	require.NoError(t, err)

	assert.Equal(t, proposalID, trail.ProposalID)
	assert.Len(t, trail.Decisions, 1)
	assert.Len(t, trail.Authorizations, 1)
	assert.Equal(t, decision.ID, trail.Decisions[0].ID)
	assert.Equal(t, auth.ID, trail.Authorizations[0].ID)
	assert.False(t, trail.CreatedAt.IsZero())
	assert.False(t, trail.UpdatedAt.IsZero())
	assert.True(t, trail.UpdatedAt.After(trail.CreatedAt) || trail.UpdatedAt.Equal(trail.CreatedAt))
}

func TestInMemoryStore_GetAuditTrail_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetAuditTrail(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no audit trail found")
}

func TestFileStore_AuditTrailPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit_trail_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	proposalID := "prop_123"
	actor := cgp.Actor{ID: "user_123", Kind: cgp.ActorKindHuman, Name: "Test User"}

	// Create store and add data
	store, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	decision := cgp.NewDecision(proposalID, cgp.DecisionApproved)
	decision.WithRiskScore(0.2)
	require.NoError(t, store.RecordDecision(ctx, decision))

	auth := cgp.NewAuthorization(decision.ID, proposalID, actor, "1.0.0")
	require.NoError(t, store.RecordAuthorization(ctx, auth))

	// Verify stats
	stats := store.Stats()
	assert.Equal(t, 1, stats.TotalDecisions)
	assert.Equal(t, 1, stats.TotalAuthorizations)

	// Create new store from same path - should load persisted data
	store2, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	// Verify data was persisted and loaded
	retrievedDecision, err := store2.GetDecision(ctx, decision.ID)
	require.NoError(t, err)
	assert.Equal(t, decision.ID, retrievedDecision.ID)
	assert.Equal(t, cgp.DecisionApproved, retrievedDecision.Decision)

	retrievedAuth, err := store2.GetAuthorization(ctx, auth.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.ID, retrievedAuth.ID)
	assert.Equal(t, "1.0.0", retrievedAuth.Version)

	// Verify audit trail
	trail, err := store2.GetAuditTrail(ctx, proposalID)
	require.NoError(t, err)
	assert.Len(t, trail.Decisions, 1)
	assert.Len(t, trail.Authorizations, 1)
}

func TestFileStore_AuditTrailJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "audit_trail_json_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	store, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	// Add a decision with complex data
	decision := cgp.NewDecision("prop_1", cgp.DecisionApprovalRequired)
	decision.WithRiskScore(0.65)
	decision.AddRiskFactor("api_change", "Breaking API change", 0.4, cgp.SeverityHigh)
	decision.AddRiskFactor("blast_radius", "High blast radius", 0.25, cgp.SeverityMedium)
	decision.AddRationale("Breaking changes detected")
	decision.AddRequiredAction("human_approval", "Senior developer review required")

	require.NoError(t, store.RecordDecision(ctx, decision))

	// Add authorization with approval chain
	actor := cgp.Actor{ID: "dev_1", Kind: cgp.ActorKindHuman, Name: "Developer"}
	reviewer := cgp.Actor{ID: "senior_1", Kind: cgp.ActorKindHuman, Name: "Senior Dev"}

	auth := cgp.NewAuthorization(decision.ID, "prop_1", reviewer, "2.0.0")
	auth.RecordApproval(actor, cgp.ApprovalActionRequestChanges, "Needs tests")
	auth.RecordApproval(actor, cgp.ApprovalActionComment, "Tests added")
	auth.RecordApproval(reviewer, cgp.ApprovalActionApprove, "LGTM")
	auth.WithReleaseNotes("# Version 2.0.0\n- Breaking change")

	require.NoError(t, store.RecordAuthorization(ctx, auth))

	// Verify JSON file was created
	jsonPath := filepath.Join(tmpDir, "memory.json")
	data, err := os.ReadFile(jsonPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "decisions")
	assert.Contains(t, string(data), "authorizations")
	assert.Contains(t, string(data), "approval_required")
	assert.Contains(t, string(data), "approvalChain")

	// Reload and verify full data
	store2, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	trail, err := store2.GetAuditTrail(ctx, "prop_1")
	require.NoError(t, err)

	assert.Len(t, trail.Decisions, 1)
	assert.Len(t, trail.Decisions[0].RiskFactors, 2)
	assert.Len(t, trail.Decisions[0].Rationale, 1)
	assert.Len(t, trail.Decisions[0].RequiredActions, 1)

	assert.Len(t, trail.Authorizations, 1)
	assert.Len(t, trail.Authorizations[0].ApprovalChain, 3)
	assert.Equal(t, "# Version 2.0.0\n- Breaking change", trail.Authorizations[0].ReleaseNotes)
}

func TestFileStore_DecisionNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "decision_not_found_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	_, err = store.GetDecision(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decision not found")
}

func TestFileStore_AuthorizationNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "auth_not_found_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	_, err = store.GetAuthorization(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization not found")
}
