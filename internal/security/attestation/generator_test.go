package attestation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp/audit"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// newTestRun creates a ReleaseRun in the specified state for testing.
func newTestRun(t *testing.T, targetState domain.RunState) *domain.ReleaseRun {
	t.Helper()

	run := domain.NewReleaseRun(
		"github.com/example/repo",
		"/tmp/repo",
		"main",
		domain.CommitSHA("abc123def456789012345678901234567890abcd"),
		[]domain.CommitSHA{"abc123def456789012345678901234567890abcd"},
		"config-hash",
		"plugin-hash",
	)

	v := version.MustParse("1.2.3")
	require.NoError(t, run.SetVersionProposal(version.MustParse("1.2.2"), v, domain.BumpPatch, 1.0))

	run.SetPolicyEvaluation(0.25, []string{"Low risk"}, domain.PolicyThresholds{
		AutoApproveRiskThreshold: 0.3,
		RequireApprovalAbove:     0.5,
		BlockReleaseAbove:        0.9,
	})
	run.SetActor(domain.ActorHuman, "alice")

	// Advance state machine
	require.NoError(t, run.Plan("alice"))
	require.NoError(t, run.SetVersion(v, "v1.2.3"))
	require.NoError(t, run.Bump("alice"))
	require.NoError(t, run.GenerateNotes(&domain.ReleaseNotes{
		Text:        "Test release notes",
		GeneratedAt: time.Now(),
	}, "inputs-hash", "alice"))
	require.NoError(t, run.Approve("alice", true))

	if targetState == domain.StatePublishing || targetState == domain.StatePublished {
		run.SetExecutionPlan([]domain.StepPlan{
			{Name: "create-tag", Type: domain.StepTypeTag},
		})
		require.NoError(t, run.StartPublishing("alice"))
	}

	if targetState == domain.StatePublished {
		require.NoError(t, run.MarkStepStarted("create-tag"))
		require.NoError(t, run.MarkStepDone("create-tag", "Tag created"))
		require.NoError(t, run.MarkPublished("alice"))
	}

	return run
}

func TestGenerator_Generate_PublishingState(t *testing.T) {
	run := newTestRun(t, domain.StatePublishing)
	chain := audit.NewChain()

	gen := NewGenerator("github.com/example/repo", chain)
	stmt, err := gen.Generate(context.Background(), run)
	require.NoError(t, err)

	assert.Equal(t, StatementType, stmt.Type)
	assert.Equal(t, PredicateTypeGovernance, stmt.PredicateType)
	assert.Len(t, stmt.Subject, 1)
	assert.Equal(t, "v1.2.3", stmt.Subject[0].Name)
	assert.Contains(t, stmt.Subject[0].Digest, "sha256")

	pred, ok := stmt.Predicate.(GovernancePredicate)
	require.True(t, ok)
	assert.Equal(t, "1.2.3", pred.Version)
	assert.Equal(t, "v1.2.3", pred.Tag)
	assert.Equal(t, "github.com/example/repo", pred.Repository)
	assert.Equal(t, 0.25, pred.RiskScore)
	assert.Equal(t, "approved", pred.Decision)
	assert.True(t, pred.AutoApproved)
	assert.Len(t, pred.Approvals, 1)
	assert.Equal(t, "alice", pred.Approvals[0].ApproverID)
}

func TestGenerator_Generate_PublishedState(t *testing.T) {
	run := newTestRun(t, domain.StatePublished)
	chain := audit.NewChain()

	gen := NewGenerator("github.com/example/repo", chain)
	stmt, err := gen.Generate(context.Background(), run)
	require.NoError(t, err)

	pred, ok := stmt.Predicate.(GovernancePredicate)
	require.True(t, ok)
	assert.NotZero(t, pred.ReleasedAt)
}

func TestGenerator_Generate_InvalidState(t *testing.T) {
	run := domain.NewReleaseRun(
		"github.com/example/repo",
		"/tmp/repo",
		"main",
		domain.CommitSHA("abc123"),
		nil,
		"hash", "hash",
	)

	gen := NewGenerator("github.com/example/repo", nil)
	_, err := gen.Generate(context.Background(), run)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot generate attestation in state draft")
}

func TestGenerator_Generate_WithAuditChain(t *testing.T) {
	run := newTestRun(t, domain.StatePublishing)

	chain := audit.NewChain()
	entry := audit.NewEntry("entry-1", audit.EventApprovalGranted).
		WithProposal("proposal-1").
		WithActor("alice", "human").
		Build()
	require.NoError(t, chain.Append(entry))

	gen := NewGenerator("github.com/example/repo", chain)
	stmt, err := gen.Generate(context.Background(), run)
	require.NoError(t, err)

	pred, ok := stmt.Predicate.(GovernancePredicate)
	require.True(t, ok)
	assert.NotEmpty(t, pred.AuditChainHash)
	assert.Equal(t, 1, pred.AuditEntryCount)
}

func TestGenerator_Generate_NilAuditChain(t *testing.T) {
	run := newTestRun(t, domain.StatePublishing)

	gen := NewGenerator("github.com/example/repo", nil)
	stmt, err := gen.Generate(context.Background(), run)
	require.NoError(t, err)

	pred, ok := stmt.Predicate.(GovernancePredicate)
	require.True(t, ok)
	assert.Empty(t, pred.AuditChainHash)
	assert.Equal(t, 0, pred.AuditEntryCount)
}

func TestGenerator_Generate_ActorIdentity(t *testing.T) {
	run := newTestRun(t, domain.StatePublishing)

	gen := NewGenerator("github.com/example/repo", nil)
	stmt, err := gen.Generate(context.Background(), run)
	require.NoError(t, err)

	pred, ok := stmt.Predicate.(GovernancePredicate)
	require.True(t, ok)
	assert.Equal(t, "alice", pred.Initiator.ID)
	assert.Equal(t, "human", pred.Initiator.Kind)
}

func Test_sha256Hex(t *testing.T) {
	h := sha256Hex("test")
	assert.Len(t, h, 64) // SHA-256 produces 32 bytes = 64 hex chars
	// Deterministic
	assert.Equal(t, h, sha256Hex("test"))
	// Different input, different hash
	assert.NotEqual(t, h, sha256Hex("other"))
}
