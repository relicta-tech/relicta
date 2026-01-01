// Package integration provides integration tests for Relicta.
package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/release/app"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
	"github.com/relicta-tech/relicta/internal/mcp"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

// mockNotesGenerator is a simple mock for testing.
type mockNotesGenerator struct{}

func (m *mockNotesGenerator) Generate(_ context.Context, run *domain.ReleaseRun, _ ports.NotesOptions) (*domain.ReleaseNotes, error) {
	return &domain.ReleaseNotes{
		Text:        "Test release notes for " + run.VersionNext().String(),
		GeneratedAt: time.Now(),
	}, nil
}

func (m *mockNotesGenerator) ComputeInputsHash(run *domain.ReleaseRun, _ ports.NotesOptions) string {
	h := sha256.Sum256([]byte(run.ID()))
	return hex.EncodeToString(h[:8])
}

// setupMCPAdapter creates a fully configured MCP adapter for testing.
// This mirrors the production setup used by the CLI.
func setupMCPAdapter(t *testing.T, repoDir string) *mcp.Adapter {
	t.Helper()

	// Create git service and adapter (same as CLI)
	gitService, err := git.NewService(git.WithRepoPath(repoDir))
	require.NoError(t, err)

	gitAdapter := git.NewAdapter(gitService)

	// Create version calculator
	versionCalc := version.NewDefaultVersionCalculator()

	// Create analysis factory (no AI for tests)
	analysisFactory := analysisfactory.NewFactory(nil)

	// Create release analyzer (used by Plan)
	releaseAnalyzer := servicerelease.NewAnalyzer(gitAdapter, versionCalc, analysisFactory)

	// Create DDD release services (ADR-007: all operations go through services)
	services, err := release.NewServices(release.Config{
		RepoRoot:       repoDir,
		GitAdapter:     gitAdapter,
		NotesGenerator: &mockNotesGenerator{},
	})
	require.NoError(t, err)

	// Create MCP adapter with full infrastructure (ADR-007 compliant)
	return mcp.NewAdapter(
		mcp.WithReleaseAnalyzer(releaseAnalyzer),
		mcp.WithReleaseServices(services),
		mcp.WithRepoRoot(repoDir),
	)
}

// TestMCPWorkflowE2E tests the full MCP release workflow matches CLI behavior.
// This test ensures MCP adapter properly uses DDD use cases (ADR-007).
func TestMCPWorkflowE2E(t *testing.T) {
	RequireGitVersion(t, "2.0.0")
	ctx := context.Background()

	// Setup test repository with conventional commits
	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test Project\n")
	repo.Commit("feat: initial project setup")
	repo.Tag("v1.0.0")

	// Add commits for release
	repo.WriteFile("main.go", "package main\n\nfunc main() {}\n")
	repo.Commit("feat: add main entry point")

	repo.WriteFile("utils.go", "package main\n\nfunc helper() {}\n")
	repo.Commit("fix: add helper function")

	adapter := setupMCPAdapter(t, repo.Dir)

	t.Run("full workflow via MCP persists state correctly", func(t *testing.T) {
		// Step 1: Plan via MCP
		planOutput, err := adapter.Plan(ctx, mcp.PlanInput{
			FromRef: "v1.0.0",
			Analyze: true,
		})
		require.NoError(t, err)
		require.NotNil(t, planOutput)
		assert.NotEmpty(t, planOutput.ReleaseID)
		assert.Equal(t, "1.0.0", planOutput.CurrentVersion)
		assert.Equal(t, "1.1.0", planOutput.NextVersion) // Minor bump for feat commits
		assert.Equal(t, 2, planOutput.CommitCount)

		// Verify state via GetStatus
		status, err := adapter.GetStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "planned", status.State)
		assert.Equal(t, planOutput.ReleaseID, status.ReleaseID)

		// Step 2: Bump via MCP
		bumpOutput, err := adapter.Bump(ctx, mcp.BumpInput{
			BumpType: "auto",
		})
		require.NoError(t, err)
		require.NotNil(t, bumpOutput)
		assert.Equal(t, "1.1.0", bumpOutput.NextVersion)

		// Verify state transition
		status, err = adapter.GetStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "versioned", status.State)
		assert.Equal(t, "1.1.0", status.Version)

		// Step 3: Notes via MCP
		notesOutput, err := adapter.Notes(ctx, mcp.NotesInput{
			UseAI:            false,
			IncludeChangelog: true,
		})
		require.NoError(t, err)
		require.NotNil(t, notesOutput)
		assert.NotEmpty(t, notesOutput.Summary)

		// Verify state transition
		status, err = adapter.GetStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "notes_ready", status.State)

		// Step 4: Approve via MCP
		approveOutput, err := adapter.Approve(ctx, mcp.ApproveInput{
			ApprovedBy: "integration-test",
		})
		require.NoError(t, err)
		require.NotNil(t, approveOutput)
		assert.True(t, approveOutput.Approved)

		// Verify state transition
		status, err = adapter.GetStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "approved", status.State)
		assert.False(t, status.CanApprove) // Already approved

		// Step 5: Publish via MCP (dry run to avoid side effects)
		publishOutput, err := adapter.Publish(ctx, mcp.PublishInput{
			DryRun: true,
		})
		require.NoError(t, err)
		require.NotNil(t, publishOutput)
	})
}

// TestMCPAndCLIStateConsistency verifies that operations via MCP and CLI
// produce consistent state when reading from the same repository.
func TestMCPAndCLIStateConsistency(t *testing.T) {
	RequireGitVersion(t, "2.0.0")
	ctx := context.Background()

	// Setup test repository
	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test Project\n")
	repo.Commit("feat: initial setup")
	repo.Tag("v1.0.0")

	repo.WriteFile("feature.go", "package main\n")
	repo.Commit("feat: add new feature")

	// Create shared infrastructure
	gitService, err := git.NewService(git.WithRepoPath(repo.Dir))
	require.NoError(t, err)

	gitAdapter := git.NewAdapter(gitService)

	versionCalc := version.NewDefaultVersionCalculator()
	analysisFactory := analysisfactory.NewFactory(nil)
	releaseAnalyzer := servicerelease.NewAnalyzer(gitAdapter, versionCalc, analysisFactory)

	services, err := release.NewServices(release.Config{
		RepoRoot:       repo.Dir,
		GitAdapter:     gitAdapter,
		NotesGenerator: &mockNotesGenerator{},
	})
	require.NoError(t, err)

	// Create MCP adapter
	mcpAdapter := mcp.NewAdapter(
		mcp.WithReleaseAnalyzer(releaseAnalyzer),
		mcp.WithReleaseServices(services),
		mcp.WithRepoRoot(repo.Dir),
	)

	t.Run("plan via MCP is visible to CLI status use case", func(t *testing.T) {
		// Plan via MCP
		_, err := mcpAdapter.Plan(ctx, mcp.PlanInput{FromRef: "v1.0.0"})
		require.NoError(t, err)

		// Read status via CLI use case directly
		cliStatusInput := app.GetStatusInput{
			RepoRoot: repo.Dir,
		}
		cliStatus, err := services.GetStatus.Execute(ctx, cliStatusInput)
		require.NoError(t, err)
		assert.Equal(t, "planned", string(cliStatus.State))
	})

	t.Run("bump via CLI use case is visible to MCP status", func(t *testing.T) {
		// Bump via CLI use case
		bumpInput := app.BumpVersionInput{
			RepoRoot: repo.Dir,
			Actor:    ports.ActorInfo{Type: "user", ID: "test"},
		}
		_, err := services.BumpVersion.Execute(ctx, bumpInput)
		require.NoError(t, err)

		// Read status via MCP
		mcpStatus, err := mcpAdapter.GetStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "versioned", mcpStatus.State)
		assert.Equal(t, "1.1.0", mcpStatus.Version)
	})
}

// TestMCPAdapterRequiresServices verifies that MCP adapter fails gracefully
// when services are not configured (defensive check for ADR-007 compliance).
func TestMCPAdapterRequiresServices(t *testing.T) {
	ctx := context.Background()

	t.Run("plan without services returns configuration error", func(t *testing.T) {
		adapter := mcp.NewAdapter() // No services configured
		_, err := adapter.Plan(ctx, mcp.PlanInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("notes without services returns configuration error", func(t *testing.T) {
		adapter := mcp.NewAdapter()
		_, err := adapter.Notes(ctx, mcp.NotesInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("approve without services returns configuration error", func(t *testing.T) {
		adapter := mcp.NewAdapter()
		_, err := adapter.Approve(ctx, mcp.ApproveInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("publish without services returns configuration error", func(t *testing.T) {
		adapter := mcp.NewAdapter()
		_, err := adapter.Publish(ctx, mcp.PublishInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("status without services returns configuration error", func(t *testing.T) {
		adapter := mcp.NewAdapter()
		_, err := adapter.GetStatus(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})
}

// TestMCPStateTransitionsAreEnforced verifies that MCP respects state machine rules.
func TestMCPStateTransitionsAreEnforced(t *testing.T) {
	RequireGitVersion(t, "2.0.0")
	ctx := context.Background()

	// Setup test repository
	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test\n")
	repo.Commit("feat: init")
	repo.Tag("v1.0.0")
	repo.WriteFile("code.go", "package main\n")
	repo.Commit("feat: add code")

	adapter := setupMCPAdapter(t, repo.Dir)

	t.Run("cannot approve before notes are generated", func(t *testing.T) {
		// Plan first
		_, err := adapter.Plan(ctx, mcp.PlanInput{FromRef: "v1.0.0"})
		require.NoError(t, err)

		// Try to approve without bump and notes - should fail
		_, err = adapter.Approve(ctx, mcp.ApproveInput{})
		require.Error(t, err)
		// Error message indicates state machine enforcement
		assert.Contains(t, err.Error(), "planned")
	})
}
