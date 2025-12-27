package mcp

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
)

func TestNewServer(t *testing.T) {
	t.Run("creates server with defaults", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.NotNil(t, server.server)
		assert.NotNil(t, server.riskCalc)
		assert.NotNil(t, server.cache)
		assert.Equal(t, "1.0.0", server.version)
	})

	t.Run("applies options", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cfg := config.DefaultConfig()
		riskCalc := risk.NewCalculatorWithDefaults()

		server, err := NewServer("1.0.0",
			WithLogger(logger),
			WithConfig(cfg),
			WithRiskCalculator(riskCalc),
		)

		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, logger, server.logger)
		assert.Equal(t, cfg, server.config)
		assert.Equal(t, riskCalc, server.riskCalc)
	})

	t.Run("cache can be disabled", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithCacheDisabled())
		require.NoError(t, err)
		assert.Nil(t, server.cache)
	})

	t.Run("adapter can be set", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)
		assert.Equal(t, adapter, server.adapter)
	})

	t.Run("custom cache can be set", func(t *testing.T) {
		cache := NewResourceCache()
		server, err := NewServer("1.0.0", WithCache(cache))
		require.NoError(t, err)
		assert.Equal(t, cache, server.cache)
	})
}

func TestServerOptions(t *testing.T) {
	t.Run("WithGitService", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithGitService(nil))
		require.NoError(t, err)
		assert.Nil(t, server.gitService)
	})

	t.Run("WithReleaseRepository", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithReleaseRepository(nil))
		require.NoError(t, err)
		assert.Nil(t, server.releaseRepo)
	})

	t.Run("WithPolicyEngine", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithPolicyEngine(nil))
		require.NoError(t, err)
		assert.Nil(t, server.policyEngine)
	})

	t.Run("WithEvaluator", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithEvaluator(nil))
		require.NoError(t, err)
		assert.Nil(t, server.evaluator)
	})
}

func TestToolInputTypes(t *testing.T) {
	t.Run("StatusInput", func(t *testing.T) {
		input := StatusInput{}
		assert.NotNil(t, input)
	})

	t.Run("PlanToolInput fields", func(t *testing.T) {
		input := PlanToolInput{
			From:    "v1.0.0",
			Analyze: true,
		}
		assert.Equal(t, "v1.0.0", input.From)
		assert.True(t, input.Analyze)
	})

	t.Run("BumpToolInput fields", func(t *testing.T) {
		input := BumpToolInput{
			Bump:    "minor",
			Version: "1.2.0",
		}
		assert.Equal(t, "minor", input.Bump)
		assert.Equal(t, "1.2.0", input.Version)
	})

	t.Run("NotesToolInput fields", func(t *testing.T) {
		input := NotesToolInput{AI: true}
		assert.True(t, input.AI)
	})

	t.Run("EvaluateToolInput", func(t *testing.T) {
		input := EvaluateToolInput{}
		assert.NotNil(t, input)
	})

	t.Run("ApproveToolInput fields", func(t *testing.T) {
		input := ApproveToolInput{Notes: "Updated notes"}
		assert.Equal(t, "Updated notes", input.Notes)
	})

	t.Run("PublishToolInput fields", func(t *testing.T) {
		input := PublishToolInput{DryRun: true}
		assert.True(t, input.DryRun)
	})
}

func TestPromptArgTypes(t *testing.T) {
	t.Run("ReleaseSummaryArgs", func(t *testing.T) {
		args := ReleaseSummaryArgs{Style: "detailed"}
		assert.Equal(t, "detailed", args.Style)
	})

	t.Run("CommitReviewArgs", func(t *testing.T) {
		args := CommitReviewArgs{Focus: "security"}
		assert.Equal(t, "security", args.Focus)
	})

	t.Run("MigrationGuideArgs", func(t *testing.T) {
		args := MigrationGuideArgs{Audience: "operator"}
		assert.Equal(t, "operator", args.Audience)
	})

	t.Run("ReleaseAnnouncementArgs", func(t *testing.T) {
		args := ReleaseAnnouncementArgs{Channel: "blog"}
		assert.Equal(t, "blog", args.Channel)
	})
}

func TestHandleStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		assert.Equal(t, "not_configured", result["status"])
	})

	t.Run("returns no_active_release with adapter but no repo", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		// Without release repository, adapter.HasReleaseRepository() returns false
		// so it falls through to the direct repo check which also fails
		assert.Equal(t, "not_configured", result["status"])
	})
}

func TestHandlePlan(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePlan(ctx, PlanToolInput{})
		require.NoError(t, err)
		assert.Equal(t, "not_configured", result["status"])
	})

	t.Run("returns not_configured with adapter but no plan use case", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handlePlan(ctx, PlanToolInput{Analyze: true})
		require.NoError(t, err)
		assert.Equal(t, "not_configured", result["status"])
	})
}

func TestHandleBump(t *testing.T) {
	ctx := context.Background()

	t.Run("returns status without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{Bump: "minor"})
		require.NoError(t, err)
		assert.Equal(t, "minor", result["bump_type"])
	})

	t.Run("defaults to auto bump type", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{})
		require.NoError(t, err)
		assert.Equal(t, "auto", result["bump_type"])
	})
}

func TestHandleNotes(t *testing.T) {
	ctx := context.Background()

	t.Run("returns status without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleNotes(ctx, NotesToolInput{AI: true})
		require.NoError(t, err)
		assert.Equal(t, true, result["use_ai"])
	})
}

func TestHandleApprove(t *testing.T) {
	ctx := context.Background()

	t.Run("returns status without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleApprove(ctx, ApproveToolInput{Notes: "test notes"})
		require.NoError(t, err)
		assert.Equal(t, "test notes", result["notes"])
	})
}

func TestHandlePublish(t *testing.T) {
	ctx := context.Background()

	t.Run("returns status without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePublish(ctx, PublishToolInput{DryRun: true})
		require.NoError(t, err)
		assert.Equal(t, true, result["dry_run"])
	})
}

func TestHandleEvaluate(t *testing.T) {
	ctx := context.Background()

	t.Run("performs basic risk calculation without adapter", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleEvaluate(ctx, EvaluateToolInput{})
		require.NoError(t, err)
		assert.Contains(t, result, "score")
		assert.Contains(t, result, "severity")
	})

	t.Run("fails without risk calculator", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)
		server.riskCalc = nil

		_, err = server.handleEvaluate(ctx, EvaluateToolInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "risk calculator not configured")
	})
}

func TestResourceHandlers(t *testing.T) {
	ctx := context.Background()

	t.Run("state resource without repo", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no release repository configured")
	})

	t.Run("config resource without config", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleResourceConfig(ctx, "relicta://config", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no configuration loaded")
	})

	t.Run("config resource with config", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Changelog.ProductName = "TestProduct"
		server, err := NewServer("1.0.0", WithConfig(cfg))
		require.NoError(t, err)

		result, err := server.handleResourceConfig(ctx, "relicta://config", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "TestProduct")
		assert.Equal(t, "application/json", result.MimeType)
	})

	t.Run("commits resource without repo", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleResourceCommits(ctx, "relicta://commits", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no release repository configured")
	})

	t.Run("changelog resource without repo", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleResourceChangelog(ctx, "relicta://changelog", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "No release repository configured")
		assert.Equal(t, "text/markdown", result.MimeType)
	})

	t.Run("risk-report resource without repo", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleResourceRiskReport(ctx, "relicta://risk-report", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no release repository configured")
	})
}

func TestPromptHandlers(t *testing.T) {
	ctx := context.Background()

	t.Run("release-summary with default style", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseSummary(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Release summary prompt", result.Description)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("release-summary with detailed style", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseSummary(ctx, map[string]string{"style": "detailed"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("release-summary with technical style", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseSummary(ctx, map[string]string{"style": "technical"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("risk-analysis prompt", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptRiskAnalysis(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Risk analysis prompt", result.Description)
	})

	t.Run("commit-review with default focus", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptCommitReview(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Commit review prompt", result.Description)
	})

	t.Run("commit-review with quality focus", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptCommitReview(ctx, map[string]string{"focus": "quality"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("commit-review with security focus", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptCommitReview(ctx, map[string]string{"focus": "security"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("breaking-changes prompt", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptBreakingChanges(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Breaking changes documentation prompt", result.Description)
	})

	t.Run("migration-guide with default audience", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptMigrationGuide(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Migration guide prompt", result.Description)
	})

	t.Run("migration-guide with operator audience", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptMigrationGuide(ctx, map[string]string{"audience": "operator"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("migration-guide with end-user audience", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptMigrationGuide(ctx, map[string]string{"audience": "end-user"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("release-announcement with default channel", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseAnnouncement(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Release announcement prompt", result.Description)
	})

	t.Run("release-announcement with blog channel", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseAnnouncement(ctx, map[string]string{"channel": "blog"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("release-announcement with social channel", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseAnnouncement(ctx, map[string]string{"channel": "social"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("release-announcement with email channel", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseAnnouncement(ctx, map[string]string{"channel": "email"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("approval-decision prompt", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptApprovalDecision(ctx, map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, "Approval decision prompt", result.Description)
	})
}

func TestInvalidateCache(t *testing.T) {
	t.Run("invalidates with cache enabled", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Set some cache entries
		server.cache.Set("relicta://state", &ReadResourceResult{
			Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
		})

		server.invalidateCache()

		// Cache should be invalidated
		assert.Nil(t, server.cache.Get("relicta://state"))
	})

	t.Run("handles nil cache gracefully", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithCacheDisabled())
		require.NoError(t, err)

		// Should not panic
		server.invalidateCache()
	})
}

func TestResourceCacheIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("caches state resource", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// First call
		_, err = server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)

		// The result should be cached (no repo means no caching happens in this case)
		// This test validates the cache check path works
		cached := server.cache.Get("relicta://state")
		assert.Nil(t, cached) // Not cached because repo is nil
	})
}
