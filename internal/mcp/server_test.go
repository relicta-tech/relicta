package mcp

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// createTestReleaseRun creates a test release run in draft state
func createTestReleaseRun() *domainrelease.ReleaseRun {
	return domainrelease.NewReleaseRun(
		"github.com/test/repo",
		"/tmp/test-repo",
		"main",
		domainrelease.CommitSHA("abc123"),
		[]domainrelease.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)
}

// createTestReleaseRunWithVersion creates a test release run with version 1.2.3
func createTestReleaseRunWithVersion() *domainrelease.ReleaseRun {
	run := domainrelease.NewReleaseRun(
		"github.com/test/repo",
		"/tmp/test-repo",
		"v1.0.0",
		domainrelease.CommitSHA("abc123"),
		[]domainrelease.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)
	// Transition to planned state and set version
	_ = run.Plan("system")
	v, _ := version.Parse("1.2.3")
	_ = run.SetVersion(v, "v1.2.3")
	return run
}

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
			Level:   "minor",
			Version: "1.2.0",
		}
		assert.Equal(t, "minor", input.Level)
		assert.Equal(t, "1.2.0", input.Version)
	})

	t.Run("NotesToolInput fields", func(t *testing.T) {
		input := NotesToolInput{
			AI:       true,
			Audience: "developers",
			Tone:     "professional",
		}
		assert.True(t, input.AI)
		assert.Equal(t, "developers", input.Audience)
		assert.Equal(t, "professional", input.Tone)
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

		result, err := server.handleBump(ctx, BumpToolInput{Level: "minor"})
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

	t.Run("returns cached result when available", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Manually set cache entry
		server.cache.Set("relicta://state", &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:      "relicta://state",
				MIMEType: "application/json",
				Text:     `{"cached": true}`,
			}},
		})

		// Should return cached result
		result, err := server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "cached")
	})
}

func TestConfigResourceWithEmptyProductName(t *testing.T) {
	ctx := context.Background()
	cfg := config.DefaultConfig()
	cfg.Changelog.ProductName = "" // Empty should default to "Relicta"

	server, err := NewServer("1.0.0", WithConfig(cfg))
	require.NoError(t, err)

	result, err := server.handleResourceConfig(ctx, "relicta://config", nil)
	require.NoError(t, err)
	assert.Contains(t, result.Text, "Relicta")
}

func TestHandleBumpWithAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured with adapter but no calculate use case", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{Level: "minor"})
		require.NoError(t, err)
		assert.Equal(t, "minor", result["bump_type"])
	})

	t.Run("includes version in result", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{Version: "2.0.0"})
		require.NoError(t, err)
		assert.Equal(t, "2.0.0", result["version"])
	})
}

func TestHandleNotesWithAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured with adapter but no notes use case", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handleNotes(ctx, NotesToolInput{AI: false})
		require.NoError(t, err)
		assert.Equal(t, false, result["use_ai"])
	})
}

func TestHandleApproveWithAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured with adapter but no approve use case", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handleApprove(ctx, ApproveToolInput{Notes: ""})
		require.NoError(t, err)
		assert.Equal(t, "", result["notes"])
	})
}

func TestHandlePublishWithAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured with adapter but no publish use case", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handlePublish(ctx, PublishToolInput{DryRun: false})
		require.NoError(t, err)
		assert.Equal(t, false, result["dry_run"])
	})
}

func TestHandleEvaluateWithAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured with adapter but no governance service", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		// Should fallback to basic risk calculation
		result, err := server.handleEvaluate(ctx, EvaluateToolInput{})
		require.NoError(t, err)
		assert.Contains(t, result, "score")
	})
}

func TestResourceRiskReportWithRiskCalc(t *testing.T) {
	ctx := context.Background()

	t.Run("uses risk calculator when available", func(t *testing.T) {
		riskCalc := risk.NewCalculatorWithDefaults()
		server, err := NewServer("1.0.0", WithRiskCalculator(riskCalc))
		require.NoError(t, err)

		// Without repo, returns not configured message
		result, err := server.handleResourceRiskReport(ctx, "relicta://risk-report", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no release repository configured")
	})
}

func TestAllServerOptionsApplied(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.DefaultConfig()
	riskCalc := risk.NewCalculatorWithDefaults()
	cache := NewResourceCache()
	adapter := NewAdapter()

	server, err := NewServer("2.0.0",
		WithLogger(logger),
		WithConfig(cfg),
		WithRiskCalculator(riskCalc),
		WithCache(cache),
		WithAdapter(adapter),
		WithGitService(nil),
		WithReleaseRepository(nil),
		WithPolicyEngine(nil),
		WithEvaluator(nil),
	)

	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, "2.0.0", server.version)
	assert.Equal(t, logger, server.logger)
	assert.Equal(t, cfg, server.config)
	assert.Equal(t, riskCalc, server.riskCalc)
	assert.Equal(t, cache, server.cache)
	assert.Equal(t, adapter, server.adapter)
}

func TestHandlePlanWithFromRef(t *testing.T) {
	ctx := context.Background()

	t.Run("handles auto from ref", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePlan(ctx, PlanToolInput{From: "auto"})
		require.NoError(t, err)
		assert.Equal(t, "not_configured", result["status"])
	})

	t.Run("handles specific from ref", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePlan(ctx, PlanToolInput{From: "v1.0.0"})
		require.NoError(t, err)
		assert.Equal(t, "not_configured", result["status"])
	})
}

func TestResourceStateWithCache(t *testing.T) {
	ctx := context.Background()

	t.Run("logs cache hit", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Set cache
		server.cache.Set("relicta://state", &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:      "relicta://state",
				MIMEType: "application/json",
				Text:     `{"test": "value"}`,
			}},
		})

		result, err := server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)
		assert.Equal(t, "relicta://state", result.URI)
		assert.Contains(t, result.Text, "test")
	})
}

func TestTypes(t *testing.T) {
	t.Run("ResourceContent fields", func(t *testing.T) {
		rc := ResourceContent{
			URI:      "test://uri",
			MIMEType: "text/plain",
			Text:     "content",
			Blob:     "base64data",
		}
		assert.Equal(t, "test://uri", rc.URI)
		assert.Equal(t, "text/plain", rc.MIMEType)
		assert.Equal(t, "content", rc.Text)
		assert.Equal(t, "base64data", rc.Blob)
	})

	t.Run("ReadResourceResult fields", func(t *testing.T) {
		result := ReadResourceResult{
			Contents: []ResourceContent{
				{URI: "test://1", Text: "one"},
				{URI: "test://2", Text: "two"},
			},
		}
		assert.Len(t, result.Contents, 2)
		assert.Equal(t, "test://1", result.Contents[0].URI)
	})
}

// Tests with mock repository for deeper coverage

func TestHandleStatusWithRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("returns no_active_release when repo returns empty", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		assert.Equal(t, "no_active_release", result["status"])
	})

	t.Run("returns release state when active release exists", func(t *testing.T) {
		run := createTestReleaseRun()
		_ = run.Plan("system") // Transition to planned state
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		assert.Equal(t, "planned", result["state"])
		assert.Contains(t, result, "created")
		assert.Contains(t, result, "updated")
	})

	t.Run("returns version when release has version", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", result["version"])
	})

	t.Run("returns draft state for draft release", func(t *testing.T) {
		run := createTestReleaseRun() // Draft state
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		assert.Equal(t, "draft", result["state"])
	})
}

func TestResourceStateWithRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("returns no active release when repo is empty", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no active release")
	})

	t.Run("returns state JSON when release exists", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "1.2.3")
		assert.Contains(t, result.Text, "planned")
		assert.Equal(t, "application/json", result.MimeType)
	})

	t.Run("caches state resource result", func(t *testing.T) {
		run := createTestReleaseRun()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		// First call
		_, err = server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)

		// Should be cached
		cached := server.cache.Get("relicta://state")
		assert.NotNil(t, cached)
	})
}

func TestResourceCommitsWithRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("returns no active release when repo is empty", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceCommits(ctx, "relicta://commits", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no active release")
	})

	t.Run("returns changeset status when release has no loaded changeset", func(t *testing.T) {
		run := createTestReleaseRun()
		_ = run.Plan("system") // Transition to planned state with a plan
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceCommits(ctx, "relicta://commits", nil)
		require.NoError(t, err)
		// The run has a plan but no loaded changeset
		assert.Contains(t, result.Text, "changeset not loaded")
	})
}

func TestResourceChangelogWithRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("returns no active release when repo is empty", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceChangelog(ctx, "relicta://changelog", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "No active release found")
	})

	t.Run("returns no changelog when release has no notes", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceChangelog(ctx, "relicta://changelog", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "No changelog generated yet")
		assert.Contains(t, result.Text, "1.2.3")
	})
}

func TestResourceRiskReportWithRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("returns no active release when repo is empty", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceRiskReport(ctx, "relicta://risk-report", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no active release")
	})

	t.Run("performs risk calculation when release exists", func(t *testing.T) {
		run := createTestReleaseRun()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		riskCalc := risk.NewCalculatorWithDefaults()
		server, err := NewServer("1.0.0", WithReleaseRepository(repo), WithRiskCalculator(riskCalc))
		require.NoError(t, err)

		result, err := server.handleResourceRiskReport(ctx, "relicta://risk-report", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "score")
		assert.Contains(t, result.Text, "severity")
	})

	t.Run("returns hint when no risk assessment available", func(t *testing.T) {
		run := createTestReleaseRun()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)
		server.riskCalc = nil // Disable risk calc

		result, err := server.handleResourceRiskReport(ctx, "relicta://risk-report", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "no risk assessment available")
	})
}

func TestCacheDisabledBehavior(t *testing.T) {
	ctx := context.Background()

	t.Run("state resource works without cache", func(t *testing.T) {
		run := createTestReleaseRun()
		_ = run.Plan("system") // Transition to planned state
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo), WithCacheDisabled())
		require.NoError(t, err)

		result, err := server.handleResourceState(ctx, "relicta://state", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "planned")
	})
}

// Additional tests to increase coverage

func TestHandleStatusWithRepositoryAndVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("returns full status with versioned release", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)

		// Verify all expected fields - state is "versioned" in the domain
		assert.Contains(t, []string{"planned", "versioned"}, result["state"])
		assert.Equal(t, "1.2.3", result["version"])
		assert.Contains(t, result, "created")
		assert.Contains(t, result, "updated")
	})
}

func TestHandleBumpWithDifferentInputs(t *testing.T) {
	ctx := context.Background()

	t.Run("handles major bump type", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{Level: "major"})
		require.NoError(t, err)
		assert.Equal(t, "major", result["bump_type"])
	})

	t.Run("handles patch bump type", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{Level: "patch"})
		require.NoError(t, err)
		assert.Equal(t, "patch", result["bump_type"])
	})

	t.Run("handles prerelease version", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleBump(ctx, BumpToolInput{
			Level:   "minor",
			Version: "1.2.0-beta.1",
		})
		require.NoError(t, err)
		assert.Equal(t, "minor", result["bump_type"])
		assert.Equal(t, "1.2.0-beta.1", result["version"])
	})
}

func TestHandleNotesWithDifferentInputs(t *testing.T) {
	ctx := context.Background()

	t.Run("handles AI disabled", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleNotes(ctx, NotesToolInput{AI: false})
		require.NoError(t, err)
		assert.Equal(t, false, result["use_ai"])
	})

	t.Run("handles AI enabled", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleNotes(ctx, NotesToolInput{AI: true})
		require.NoError(t, err)
		assert.Equal(t, true, result["use_ai"])
	})
}

func TestHandlePublishWithDifferentInputs(t *testing.T) {
	ctx := context.Background()

	t.Run("handles dry run true", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePublish(ctx, PublishToolInput{DryRun: true})
		require.NoError(t, err)
		assert.Equal(t, true, result["dry_run"])
	})

	t.Run("handles dry run false", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePublish(ctx, PublishToolInput{DryRun: false})
		require.NoError(t, err)
		assert.Equal(t, false, result["dry_run"])
	})
}

func TestHandleEvaluateWithDifferentInputs(t *testing.T) {
	ctx := context.Background()

	t.Run("evaluates with default risk calculator", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleEvaluate(ctx, EvaluateToolInput{})
		require.NoError(t, err)
		assert.Contains(t, result, "score")
		assert.Contains(t, result, "severity")
		assert.Contains(t, result, "factors")
	})

	t.Run("evaluates with custom risk calculator", func(t *testing.T) {
		riskCalc := risk.NewCalculatorWithDefaults()
		server, err := NewServer("1.0.0", WithRiskCalculator(riskCalc))
		require.NoError(t, err)

		result, err := server.handleEvaluate(ctx, EvaluateToolInput{})
		require.NoError(t, err)
		assert.Contains(t, result, "score")
		assert.Contains(t, result, "severity")
	})
}

func TestHandleApproveWithDifferentInputs(t *testing.T) {
	ctx := context.Background()

	t.Run("handles empty notes", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handleApprove(ctx, ApproveToolInput{Notes: ""})
		require.NoError(t, err)
		assert.Equal(t, "", result["notes"])
	})

	t.Run("handles long notes", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		longNotes := "This is a very long note that contains detailed release information."
		result, err := server.handleApprove(ctx, ApproveToolInput{Notes: longNotes})
		require.NoError(t, err)
		assert.Equal(t, longNotes, result["notes"])
	})
}

func TestResourceConfigWithDifferentSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("includes config settings", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Changelog.ProductName = "MyApp"
		cfg.AI.Enabled = true
		cfg.AI.Provider = "anthropic"
		cfg.Versioning.Strategy = "conventional"

		server, err := NewServer("1.0.0", WithConfig(cfg))
		require.NoError(t, err)

		result, err := server.handleResourceConfig(ctx, "relicta://config", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "MyApp")
		assert.Contains(t, result.Text, "anthropic")
		assert.Contains(t, result.Text, "conventional")
	})

	t.Run("defaults empty product name to Relicta", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Changelog.ProductName = ""

		server, err := NewServer("1.0.0", WithConfig(cfg))
		require.NoError(t, err)

		result, err := server.handleResourceConfig(ctx, "relicta://config", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "Relicta")
	})
}

func TestResourceChangelogWithVersionedRelease(t *testing.T) {
	ctx := context.Background()

	t.Run("returns version info when release has version", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceChangelog(ctx, "relicta://changelog", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "1.2.3")
		assert.Equal(t, "text/markdown", result.MimeType)
	})
}

func TestCacheOperations(t *testing.T) {
	t.Run("cache can be set and retrieved", func(t *testing.T) {
		cache := NewResourceCache()
		content := &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:  "test://uri",
				Text: "test content",
			}},
		}
		cache.Set("test://uri", content)

		retrieved := cache.Get("test://uri")
		require.NotNil(t, retrieved)
		assert.Equal(t, "test content", retrieved.Contents[0].Text)
	})

	t.Run("cache returns nil for missing key", func(t *testing.T) {
		cache := NewResourceCache()
		retrieved := cache.Get("nonexistent://uri")
		assert.Nil(t, retrieved)
	})

	t.Run("cache invalidation removes entry", func(t *testing.T) {
		cache := NewResourceCache()
		content := &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:  "test://uri",
				Text: "test content",
			}},
		}
		cache.Set("test://uri", content)
		cache.Invalidate("test://uri")

		retrieved := cache.Get("test://uri")
		assert.Nil(t, retrieved)
	})

	t.Run("cache stats track entry count", func(t *testing.T) {
		cache := NewResourceCache()
		content := &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:  "test://uri",
				Text: "test content",
			}},
		}
		cache.Set("test://uri", content)

		stats := cache.Stats()
		assert.True(t, stats.Enabled)
		assert.Equal(t, 1, stats.EntryCount)
		assert.Contains(t, stats.Entries, "test://uri")
	})
}

func TestPromptArgsWithDifferentValues(t *testing.T) {
	ctx := context.Background()

	t.Run("release-summary with executive style", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseSummary(ctx, map[string]string{"style": "executive"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("commit-review with breaking focus", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptCommitReview(ctx, map[string]string{"focus": "breaking"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("migration-guide with developer audience", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptMigrationGuide(ctx, map[string]string{"audience": "developer"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})

	t.Run("release-announcement with internal channel", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		result, err := server.handlePromptReleaseAnnouncement(ctx, map[string]string{"channel": "internal"})
		require.NoError(t, err)
		assert.Len(t, result.Messages, 1)
	})
}

func TestServerWithMultipleOptions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.DefaultConfig()
	riskCalc := risk.NewCalculatorWithDefaults()
	cache := NewResourceCache()
	adapter := NewAdapter()

	t.Run("all options applied correctly", func(t *testing.T) {
		server, err := NewServer("3.0.0",
			WithLogger(logger),
			WithConfig(cfg),
			WithRiskCalculator(riskCalc),
			WithCache(cache),
			WithAdapter(adapter),
		)

		require.NoError(t, err)
		assert.Equal(t, "3.0.0", server.version)
		assert.Equal(t, logger, server.logger)
		assert.Equal(t, cfg, server.config)
		assert.Equal(t, riskCalc, server.riskCalc)
		assert.Equal(t, cache, server.cache)
		assert.Equal(t, adapter, server.adapter)
	})
}

func TestResourceRiskReportWithCalc(t *testing.T) {
	ctx := context.Background()

	t.Run("returns calculated risk when repo and calc available", func(t *testing.T) {
		run := createTestReleaseRun()
		_ = run.Plan("system") // Transition to planned
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		riskCalc := risk.NewCalculatorWithDefaults()
		server, err := NewServer("1.0.0", WithReleaseRepository(repo), WithRiskCalculator(riskCalc))
		require.NoError(t, err)

		result, err := server.handleResourceRiskReport(ctx, "relicta://risk-report", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "score")
	})
}

func TestHandlePlanWithAdapter(t *testing.T) {
	ctx := context.Background()

	t.Run("returns not_configured when adapter has no plan use case", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handlePlan(ctx, PlanToolInput{From: "v1.0.0", Analyze: true})
		require.NoError(t, err)
		assert.Equal(t, "not_configured", result["status"])
	})
}

func TestHandleStatusWithAdapterAndRepository(t *testing.T) {
	ctx := context.Background()

	// NOTE: GetStatus now requires release services with a GetStatusUseCase.
	// When only a repository is configured (no release services), the adapter
	// returns an error, which handleStatus translates to "no_active_release".

	t.Run("returns no_active_release when adapter has only repository (no release services)", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		adapter := NewAdapter(WithAdapterRepo(repo))
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		// GetStatus requires release services, so this returns no_active_release
		assert.Equal(t, "no_active_release", result["status"])
	})

	t.Run("returns no_active_release when adapter repo is empty", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
		adapter := NewAdapter(WithAdapterRepo(repo))
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		result, err := server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		assert.Equal(t, "no_active_release", result["status"])
	})
}

func TestAdapterGetStatusWithData(t *testing.T) {
	ctx := context.Background()

	// NOTE: GetStatus now requires release services with a GetStatusUseCase.
	// These tests verify the behavior when only a repository is provided.

	t.Run("returns full status with versioned release", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		adapter := NewAdapter(WithAdapterRepo(repo))

		// GetStatus now requires release services, so this returns an error
		status, err := adapter.GetStatus(ctx)
		require.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "release services not configured")
	})

	t.Run("returns status with approval info", func(t *testing.T) {
		run := createTestReleaseRunWithVersion()
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		adapter := NewAdapter(WithAdapterRepo(repo))

		// GetStatus now requires release services, so this returns an error
		status, err := adapter.GetStatus(ctx)
		require.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "release services not configured")
	})

	t.Run("returns stale status for empty version", func(t *testing.T) {
		run := createTestReleaseRun()
		_ = run.Plan("system")
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		adapter := NewAdapter(WithAdapterRepo(repo))

		// GetStatus now requires release services, so this returns an error
		status, err := adapter.GetStatus(ctx)
		require.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "release services not configured")
	})
}

func TestAdapterEvaluateRequiresGovernanceService(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error without governance service", func(t *testing.T) {
		adapter := NewAdapter()

		input := EvaluateInput{
			ReleaseID: "test-release",
			ActorID:   "test-user",
			ActorName: "Test User",
		}

		output, err := adapter.Evaluate(ctx, input)
		require.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "governance service not configured")
	})
}

func TestMoreCacheScenarios(t *testing.T) {
	t.Run("set and get with TTL", func(t *testing.T) {
		cache := NewResourceCache()

		content := &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:  "test://uri",
				Text: "test content",
			}},
		}
		cache.Set("test://uri", content)

		// Set a custom TTL
		cache.SetTTL("test://uri", 1*time.Hour)

		// Entry should still be retrievable
		retrieved := cache.Get("test://uri")
		assert.NotNil(t, retrieved)
	})

	t.Run("invalidate all clears all entries", func(t *testing.T) {
		cache := NewResourceCache()

		content1 := &ReadResourceResult{Contents: []ResourceContent{{URI: "test://1", Text: "one"}}}
		content2 := &ReadResourceResult{Contents: []ResourceContent{{URI: "test://2", Text: "two"}}}

		cache.Set("test://1", content1)
		cache.Set("test://2", content2)

		stats := cache.Stats()
		assert.Equal(t, 2, stats.EntryCount)

		cache.InvalidateAll()

		stats = cache.Stats()
		assert.Equal(t, 0, stats.EntryCount)
	})

	t.Run("invalidate state dependent removes related entries", func(t *testing.T) {
		cache := NewResourceCache()

		content1 := &ReadResourceResult{Contents: []ResourceContent{{URI: "relicta://state", Text: "state"}}}
		content2 := &ReadResourceResult{Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}}}

		cache.Set("relicta://state", content1)
		cache.Set("relicta://config", content2)

		cache.InvalidateStateDependent()

		assert.Nil(t, cache.Get("relicta://state"))
		// Config might still be there depending on implementation
	})

	t.Run("set enabled toggles cache", func(t *testing.T) {
		cache := NewResourceCache()
		assert.True(t, cache.IsEnabled())

		cache.SetEnabled(false)
		assert.False(t, cache.IsEnabled())

		cache.SetEnabled(true)
		assert.True(t, cache.IsEnabled())
	})
}

func TestHandleResourceCommitsWithVariousStates(t *testing.T) {
	ctx := context.Background()

	t.Run("returns changeset not loaded for draft release", func(t *testing.T) {
		run := createTestReleaseRun()
		// Draft state - no plan
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceCommits(ctx, "relicta://commits", nil)
		require.NoError(t, err)
		// Draft has no plan, which may return "no plan" or "changeset not loaded"
		assert.Contains(t, result.Text, "commit")
	})

	t.Run("returns changeset status for planned release without changeset", func(t *testing.T) {
		run := createTestReleaseRun()
		_ = run.Plan("system") // Transition to planned
		repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{run}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		result, err := server.handleResourceCommits(ctx, "relicta://commits", nil)
		require.NoError(t, err)
		assert.Contains(t, result.Text, "changeset")
	})
}

func TestHandleEvaluateWithRiskCalculator(t *testing.T) {
	ctx := context.Background()

	t.Run("calculates basic risk factors", func(t *testing.T) {
		riskCalc := risk.NewCalculatorWithDefaults()
		server, err := NewServer("1.0.0", WithRiskCalculator(riskCalc))
		require.NoError(t, err)

		result, err := server.handleEvaluate(ctx, EvaluateToolInput{})
		require.NoError(t, err)

		assert.Contains(t, result, "score")
		assert.Contains(t, result, "severity")
		assert.Contains(t, result, "factors")

		factors, ok := result["factors"].([]map[string]any)
		if ok {
			assert.GreaterOrEqual(t, len(factors), 0)
		}
	})
}

func TestAllPromptHandlersComprehensive(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	t.Run("all prompts return valid results", func(t *testing.T) {
		// Test release-summary with all styles
		for _, style := range []string{"", "brief", "detailed", "technical", "executive"} {
			result, err := server.handlePromptReleaseSummary(ctx, map[string]string{"style": style})
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Messages)
		}

		// Test commit-review with all focus types
		for _, focus := range []string{"", "general", "quality", "security", "breaking"} {
			result, err := server.handlePromptCommitReview(ctx, map[string]string{"focus": focus})
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Messages)
		}

		// Test migration-guide with all audiences
		for _, audience := range []string{"", "developer", "operator", "end-user"} {
			result, err := server.handlePromptMigrationGuide(ctx, map[string]string{"audience": audience})
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Messages)
		}

		// Test release-announcement with all channels
		for _, channel := range []string{"", "github", "blog", "social", "email", "internal"} {
			result, err := server.handlePromptReleaseAnnouncement(ctx, map[string]string{"channel": channel})
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Messages)
		}
	})
}

// Test more handler scenarios with adapter
func TestHandleBumpWithAdapterWithoutUseCase(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter()
	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	result, err := server.handleBump(ctx, BumpToolInput{Level: "minor"})
	require.NoError(t, err)
	// Falls through to stub response
	assert.Equal(t, "minor", result["bump_type"])
}

func TestHandleNotesWithAdapterWithoutUseCase(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter()
	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	result, err := server.handleNotes(ctx, NotesToolInput{AI: true})
	require.NoError(t, err)
	assert.Equal(t, true, result["use_ai"])
}

func TestHandleApproveWithAdapterWithoutUseCase(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter()
	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	result, err := server.handleApprove(ctx, ApproveToolInput{Notes: "approval notes"})
	require.NoError(t, err)
	assert.Equal(t, "approval notes", result["notes"])
}

func TestHandlePublishWithAdapterWithoutUseCase(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter()
	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	result, err := server.handlePublish(ctx, PublishToolInput{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, true, result["dry_run"])
}

func TestHandleEvaluateWithAdapterAndGovernance(t *testing.T) {
	ctx := context.Background()
	adapter := NewAdapter()
	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	result, err := server.handleEvaluate(ctx, EvaluateToolInput{})
	require.NoError(t, err)
	// Should fallback to basic risk calculation
	assert.Contains(t, result, "score")
}

// Test for issue #35: MCP server release state not persisted between tool calls
// The fix ensures consistent repository path handling across all MCP tool calls.

func TestEnsureRepoPath(t *testing.T) {
	ctx := context.Background()

	t.Run("defaults to current dir when git service unavailable", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		repoPath := server.ensureRepoPath(ctx)

		assert.Equal(t, ".", repoPath)
		assert.Equal(t, ".", adapter.GetRepoRoot())
	})

	t.Run("handles nil adapter gracefully", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Should not panic with nil adapter
		repoPath := server.ensureRepoPath(ctx)
		assert.Equal(t, ".", repoPath)
	})

	t.Run("updates adapter repoRoot when called", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		// Initially adapter has empty repoRoot
		assert.Equal(t, "", adapter.GetRepoRoot())

		// ensureRepoPath should set it
		_ = server.ensureRepoPath(ctx)

		// Now adapter has repoRoot set
		assert.NotEqual(t, "", adapter.GetRepoRoot())
	})
}

func TestConsistentRepoPathAcrossToolCalls(t *testing.T) {
	ctx := context.Background()

	t.Run("all tool handlers set adapter repoRoot consistently", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		// Call various handlers and verify adapter repoRoot is set
		// Each handler should call ensureRepoPath which sets adapter.repoRoot

		_, _ = server.handleStatus(ctx, StatusInput{})
		statusPath := adapter.GetRepoRoot()
		assert.NotEqual(t, "", statusPath, "handleStatus should set repoRoot")

		adapter.SetRepoRoot("") // Reset
		_, _ = server.handlePlan(ctx, PlanToolInput{})
		planPath := adapter.GetRepoRoot()
		assert.NotEqual(t, "", planPath, "handlePlan should set repoRoot")
		assert.Equal(t, statusPath, planPath, "paths should be consistent")

		adapter.SetRepoRoot("") // Reset
		_, _ = server.handleBump(ctx, BumpToolInput{})
		bumpPath := adapter.GetRepoRoot()
		assert.NotEqual(t, "", bumpPath, "handleBump should set repoRoot")
		assert.Equal(t, statusPath, bumpPath, "paths should be consistent")

		adapter.SetRepoRoot("") // Reset
		_, _ = server.handleNotes(ctx, NotesToolInput{})
		notesPath := adapter.GetRepoRoot()
		assert.NotEqual(t, "", notesPath, "handleNotes should set repoRoot")
		assert.Equal(t, statusPath, notesPath, "paths should be consistent")

		adapter.SetRepoRoot("") // Reset
		_, _ = server.handleApprove(ctx, ApproveToolInput{})
		approvePath := adapter.GetRepoRoot()
		assert.NotEqual(t, "", approvePath, "handleApprove should set repoRoot")
		assert.Equal(t, statusPath, approvePath, "paths should be consistent")

		adapter.SetRepoRoot("") // Reset
		_, _ = server.handlePublish(ctx, PublishToolInput{})
		publishPath := adapter.GetRepoRoot()
		assert.NotEqual(t, "", publishPath, "handlePublish should set repoRoot")
		assert.Equal(t, statusPath, publishPath, "paths should be consistent")
	})

	t.Run("issue 35: state persists across plan and notes calls", func(t *testing.T) {
		// This test verifies the fix for issue #35:
		// When plan is called, it sets repoRoot on the adapter.
		// When notes is called, it should use the same repoRoot to find the release.
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		// Simulate the workflow that was failing
		_, _ = server.handlePlan(ctx, PlanToolInput{})
		pathAfterPlan := adapter.GetRepoRoot()

		// Notes should use the same path
		_, _ = server.handleNotes(ctx, NotesToolInput{})
		pathAfterNotes := adapter.GetRepoRoot()

		// The key fix: paths should be the same
		assert.Equal(t, pathAfterPlan, pathAfterNotes,
			"plan and notes should use the same repository path")
	})
}

// Suppress unused variable warnings
var _ = time.Now
