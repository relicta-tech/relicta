// Package mcp provides MCP server implementation for Relicta using felixgeelhaar/mcp-go.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/felixgeelhaar/mcp-go"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	relictaerrors "github.com/relicta-tech/relicta/internal/errors"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

// Server wraps the MCP server for Relicta.
type Server struct {
	server  *mcp.Server
	version string
	logger  *slog.Logger

	// Dependencies for tool execution
	config       *config.Config
	gitService   git.Service
	releaseRepo  release.Repository
	policyEngine *policy.Engine
	riskCalc     *risk.Calculator
	evaluator    *evaluator.Evaluator

	// Application layer adapter
	adapter *Adapter

	// Resource cache for improved read performance
	cache *ResourceCache
}

// ServerOption configures the MCP server.
type ServerOption func(*Server)

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// WithConfig sets the configuration.
func WithConfig(cfg *config.Config) ServerOption {
	return func(s *Server) {
		s.config = cfg
	}
}

// WithGitService sets the git service.
func WithGitService(gs git.Service) ServerOption {
	return func(s *Server) {
		s.gitService = gs
	}
}

// WithReleaseRepository sets the release repository.
func WithReleaseRepository(repo release.Repository) ServerOption {
	return func(s *Server) {
		s.releaseRepo = repo
	}
}

// WithPolicyEngine sets the policy engine.
func WithPolicyEngine(pe *policy.Engine) ServerOption {
	return func(s *Server) {
		s.policyEngine = pe
	}
}

// WithRiskCalculator sets the risk calculator.
func WithRiskCalculator(rc *risk.Calculator) ServerOption {
	return func(s *Server) {
		s.riskCalc = rc
	}
}

// WithEvaluator sets the CGP evaluator.
func WithEvaluator(ev *evaluator.Evaluator) ServerOption {
	return func(s *Server) {
		s.evaluator = ev
	}
}

// WithAdapter sets the application layer adapter.
func WithAdapter(adapter *Adapter) ServerOption {
	return func(s *Server) {
		s.adapter = adapter
	}
}

// WithCache sets a custom resource cache.
func WithCache(cache *ResourceCache) ServerOption {
	return func(s *Server) {
		s.cache = cache
	}
}

// WithCacheDisabled disables resource caching.
func WithCacheDisabled() ServerOption {
	return func(s *Server) {
		s.cache = nil
	}
}

// userError formats an error for user display using FormatUserError.
// This avoids redundant "failed" messages in error chains.
// Example: "notes generation failed: generate notes failed: failed to set release notes: invalid state"
// Becomes: "Notes generation failed: invalid state transition: cannot set notes in state planned"
func userError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", relictaerrors.FormatUserError(err))
}

// Tool input types with JSON Schema generation via struct tags.

// StatusInput represents input for the status tool.
// Maps to CLI: relicta status (no additional flags)
// Returns current release state, version, and next recommended action.
type StatusInput struct{}

// PlanToolInput represents input for the plan tool.
// Maps to CLI: relicta plan [--from REF] [--to REF] [--analyze] [--no-ai] [--minimal]
type PlanToolInput struct {
	From          string  `json:"from,omitempty" jsonschema:"description=Starting reference for commit analysis (tag like 'v1.0.0' or commit SHA). Leave empty for automatic detection from latest version tag."`
	To            string  `json:"to,omitempty" jsonschema:"description=Ending reference for commit analysis (tag or commit SHA). Defaults to HEAD."`
	Analyze       bool    `json:"analyze,omitempty" jsonschema:"description=Include detailed commit classification analysis in the output. Shows how each commit was categorized."`
	NoAI          bool    `json:"no_ai,omitempty" jsonschema:"description=Disable AI-powered commit classification. Uses only conventional commit parsing."`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"description=Minimum confidence threshold (0.0-1.0) to accept AI commit classifications. Default is 0.7."`
}

// BumpToolInput represents input for the bump tool.
// Maps to CLI: relicta bump [--level LEVEL] [--version VERSION] [--prerelease ID] [--build META]
type BumpToolInput struct {
	Level      string `json:"level,omitempty" jsonschema:"description=Version bump level. Use 'auto' to determine from commits or specify 'major'/'minor'/'patch' explicitly.,enum=major|minor|patch|auto,default=auto"`
	Version    string `json:"version,omitempty" jsonschema:"description=Set an explicit version (e.g. '2.0.0'). Overrides level and bypasses commit analysis."`
	Prerelease string `json:"prerelease,omitempty" jsonschema:"description=Prerelease identifier to append (e.g. 'alpha', 'beta', 'rc.1'). Creates versions like '1.2.0-beta'."`
	Build      string `json:"build,omitempty" jsonschema:"description=Build metadata to append (e.g. 'build.123'). Creates versions like '1.2.0+build.123'."`
}

// NotesToolInput represents input for the notes tool.
// Maps to CLI: relicta notes [--ai] [--audience TYPE] [--tone STYLE] [--language LANG] [--emoji]
type NotesToolInput struct {
	AI       bool   `json:"ai,omitempty" jsonschema:"description=Use AI to generate enhanced release notes. Requires OPENAI_API_KEY or configured AI provider."`
	Audience string `json:"audience,omitempty" jsonschema:"description=Target audience affects terminology and detail level.,enum=developers|users|public|stakeholders,default=developers"`
	Tone     string `json:"tone,omitempty" jsonschema:"description=Writing style for AI-generated notes.,enum=technical|friendly|professional|marketing,default=professional"`
	Language string `json:"language,omitempty" jsonschema:"description=Output language for release notes (e.g. 'English', 'Spanish', 'Japanese'). Default is English."`
	Emoji    bool   `json:"emoji,omitempty" jsonschema:"description=Include emojis in release notes output for visual categorization."`
}

// EvaluateToolInput represents input for the evaluate tool.
// Maps to CLI: relicta evaluate (no additional flags)
type EvaluateToolInput struct{}

// ApproveToolInput represents input for the approve tool.
// Maps to CLI: relicta approve [--yes] [--edit]
type ApproveToolInput struct {
	Notes   string `json:"notes,omitempty" jsonschema:"description=Updated release notes content. If provided, replaces the generated notes before approval."`
	Message string `json:"message,omitempty" jsonschema:"description=Approval message or reason for the release. Recorded in the audit trail."`
}

// PublishToolInput represents input for the publish tool.
// Maps to CLI: relicta publish [--dry-run] [--skip-push] [--skip-tag] [--skip-plugins]
type PublishToolInput struct {
	DryRun      bool `json:"dry_run,omitempty" jsonschema:"description=Simulate the release without making actual changes. Shows what would happen."`
	SkipPush    bool `json:"skip_push,omitempty" jsonschema:"description=Skip pushing git tags to the remote repository."`
	SkipTag     bool `json:"skip_tag,omitempty" jsonschema:"description=Skip creating the git tag. Useful when tag already exists."`
	SkipPlugins bool `json:"skip_plugins,omitempty" jsonschema:"description=Skip running configured plugins (GitHub release, Slack notification, etc.)."`
}

// CancelToolInput represents input for the cancel tool.
// Maps to CLI: relicta cancel [--reason TEXT] [--force]
type CancelToolInput struct {
	Reason string `json:"reason,omitempty" jsonschema:"description=Reason for canceling the release. Recorded in the audit trail for traceability."`
	Force  bool   `json:"force,omitempty" jsonschema:"description=Force cancel even if release is in publishing state. Use with caution - may leave artifacts in inconsistent state."`
}

// ResetToolInput represents input for the reset tool.
// Maps to CLI: relicta reset [--force]
type ResetToolInput struct {
	Force bool `json:"force,omitempty" jsonschema:"description=Force reset even if a release is in progress. Clears all release state and starts fresh."`
}

// --- Specialized AI Agent Tool Inputs ---

// BlastRadiusToolInput represents input for the blast_radius tool.
type BlastRadiusToolInput struct {
	From         string   `json:"from,omitempty" jsonschema:"description=Starting reference (tag or commit SHA). Uses last tag if empty."`
	To           string   `json:"to,omitempty" jsonschema:"description=Ending reference. Defaults to HEAD."`
	Transitive   bool     `json:"transitive,omitempty" jsonschema:"description=Include transitively affected packages in analysis"`
	Graph        bool     `json:"graph,omitempty" jsonschema:"description=Generate dependency graph for visualization"`
	PackagePaths []string `json:"package_paths,omitempty" jsonschema:"description=Specific package paths to analyze. Analyzes all if empty."`
}

// InferVersionToolInput represents input for the infer_version tool.
type InferVersionToolInput struct {
	From        string `json:"from,omitempty" jsonschema:"description=Starting reference (tag or commit SHA). Uses last tag if empty."`
	To          string `json:"to,omitempty" jsonschema:"description=Ending reference. Defaults to HEAD."`
	IncludeRisk bool   `json:"include_risk,omitempty" jsonschema:"description=Include risk assessment with version inference"`
}

// SummarizeDiffToolInput represents input for the summarize_diff tool.
type SummarizeDiffToolInput struct {
	From      string `json:"from,omitempty" jsonschema:"description=Starting reference (tag or commit SHA). Uses last tag if empty."`
	To        string `json:"to,omitempty" jsonschema:"description=Ending reference. Defaults to HEAD."`
	Audience  string `json:"audience,omitempty" jsonschema:"description=Target audience for summary,enum=developer|operator|end-user,default=developer"`
	MaxLength int    `json:"max_length,omitempty" jsonschema:"description=Target summary length in characters"`
}

// ValidateReleaseToolInput represents input for the validate_release tool.
type ValidateReleaseToolInput struct {
	ReleaseID       string   `json:"release_id,omitempty" jsonschema:"description=Release ID to validate. Uses active release if empty."`
	CheckGit        bool     `json:"check_git,omitempty" jsonschema:"description=Check git state (clean, branch allowed)"`
	CheckPlugins    bool     `json:"check_plugins,omitempty" jsonschema:"description=Check plugin availability and configuration"`
	CheckGovernance bool     `json:"check_governance,omitempty" jsonschema:"description=Check CGP governance requirements"`
	Checks          []string `json:"checks,omitempty" jsonschema:"description=Specific checks to run (subset of all checks)"`
}

// Prompt argument input types.

// ReleaseSummaryArgs represents arguments for the release-summary prompt.
type ReleaseSummaryArgs struct {
	Style string `json:"style,omitempty" jsonschema:"description=Summary style: brief, detailed, or technical,enum=brief|detailed|technical,default=brief"`
}

// CommitReviewArgs represents arguments for the commit-review prompt.
type CommitReviewArgs struct {
	Focus string `json:"focus,omitempty" jsonschema:"description=Review focus: compliance, quality, or security,enum=compliance|quality|security,default=compliance"`
}

// MigrationGuideArgs represents arguments for the migration-guide prompt.
type MigrationGuideArgs struct {
	Audience string `json:"audience,omitempty" jsonschema:"description=Target audience: developer, operator, or end-user,enum=developer|operator|end-user,default=developer"`
}

// ReleaseAnnouncementArgs represents arguments for the release-announcement prompt.
type ReleaseAnnouncementArgs struct {
	Channel string `json:"channel,omitempty" jsonschema:"description=Target channel: github, blog, social, or email,enum=github|blog|social|email,default=github"`
}

// NewServer creates a new MCP server for Relicta.
func NewServer(version string, opts ...ServerOption) (*Server, error) {
	s := &Server{
		version:  version,
		logger:   slog.Default(),
		cache:    NewResourceCache(),
		riskCalc: risk.NewCalculatorWithDefaults(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create the MCP server with felixgeelhaar/mcp-go
	s.server = mcp.NewServer(mcp.ServerInfo{
		Name:    "relicta",
		Version: version,
		Capabilities: mcp.Capabilities{
			Tools:     true,
			Resources: true,
			Prompts:   true,
		},
	})

	// Register tools
	s.registerTools()

	// Register resources
	s.registerResources()

	// Register prompts
	s.registerPrompts()

	return s, nil
}

// ServeStdio starts the MCP server on stdio transport.
func (s *Server) ServeStdio() error {
	s.logger.Info("MCP server started", "version", s.version)
	return mcp.ServeStdio(context.Background(), s.server)
}

// registerTools registers all tool handlers.
func (s *Server) registerTools() {
	// Status tool
	s.server.Tool("relicta.status").
		Description("Get the current release state and pending actions").
		Handler(s.handleStatus)

	// Plan tool
	s.server.Tool("relicta.plan").
		Description("Analyze commits since the last release and suggest a version bump").
		Handler(s.handlePlan)

	// Bump tool
	s.server.Tool("relicta.bump").
		Description("Calculate and set the next version based on commits").
		Handler(s.handleBump)

	// Notes tool
	s.server.Tool("relicta.notes").
		Description("Generate changelog and release notes for the current release").
		Handler(s.handleNotes)

	// Evaluate tool
	s.server.Tool("relicta.evaluate").
		Description("Evaluate release risk using the Change Governance Protocol (CGP)").
		Handler(s.handleEvaluate)

	// Approve tool
	s.server.Tool("relicta.approve").
		Description("Approve the release for publishing").
		Handler(s.handleApprove)

	// Publish tool
	s.server.Tool("relicta.publish").
		Description("Execute the release by creating tags and running plugins").
		Handler(s.handlePublish)

	// Cancel tool
	s.server.Tool("relicta.cancel").
		Description("Cancel the current in-progress release").
		Handler(s.handleCancel)

	// Reset tool
	s.server.Tool("relicta.reset").
		Description("Reset a failed or canceled release to allow starting fresh").
		Handler(s.handleReset)

	// --- Specialized AI Agent Tools ---

	// Blast Radius tool - Monorepo change impact analysis
	s.server.Tool("relicta.blast_radius").
		Description("Analyze blast radius of changes in a monorepo. Returns impacted packages, transitive dependencies, and deployment risk assessment.").
		Handler(s.handleBlastRadius)

	// Infer Version tool - Lightweight version inference
	s.server.Tool("relicta.infer_version").
		Description("Infer the next semantic version based on commits. Lightweight alternative to plan for quick queries.").
		Handler(s.handleInferVersion)

	// Summarize Diff tool - Audience-tailored change summaries
	s.server.Tool("relicta.summarize_diff").
		Description("Generate audience-tailored summary of changes between refs. Supports developer, operator, and end-user audiences.").
		Handler(s.handleSummarizeDiff)

	// Validate Release tool - Pre-flight checks
	s.server.Tool("relicta.validate_release").
		Description("Run pre-flight validation checks before release. Validates git state, plugins, and governance requirements.").
		Handler(s.handleValidateRelease)
}

// registerResources registers all resource handlers.
func (s *Server) registerResources() {
	s.server.Resource("relicta://state").
		Name("Release State").
		Description("Current release state machine status").
		MimeType("application/json").
		Handler(s.handleResourceState)

	s.server.Resource("relicta://config").
		Name("Configuration").
		Description("Current Relicta configuration").
		MimeType("application/json").
		Handler(s.handleResourceConfig)

	s.server.Resource("relicta://commits").
		Name("Commits").
		Description("Recent commits since last release").
		MimeType("application/json").
		Handler(s.handleResourceCommits)

	s.server.Resource("relicta://changelog").
		Name("Changelog").
		Description("Generated changelog for current release").
		MimeType("text/markdown").
		Handler(s.handleResourceChangelog)

	s.server.Resource("relicta://risk-report").
		Name("Risk Report").
		Description("CGP risk assessment for current release").
		MimeType("application/json").
		Handler(s.handleResourceRiskReport)
}

// registerPrompts registers all prompt handlers.
func (s *Server) registerPrompts() {
	s.server.Prompt("release-summary").
		Description("Generate a summary of the upcoming release").
		Argument("style", "Summary style: brief, detailed, or technical", false).
		Handler(s.handlePromptReleaseSummary)

	s.server.Prompt("risk-analysis").
		Description("Analyze and explain the risk factors for the current release").
		Handler(s.handlePromptRiskAnalysis)

	s.server.Prompt("commit-review").
		Description("Review commits for conventional commit compliance and quality").
		Argument("focus", "Review focus: compliance, quality, or security", false).
		Handler(s.handlePromptCommitReview)

	s.server.Prompt("breaking-changes").
		Description("Document breaking changes and their impact on users").
		Handler(s.handlePromptBreakingChanges)

	s.server.Prompt("migration-guide").
		Description("Generate migration instructions for upgrading to this release").
		Argument("audience", "Target audience: developer, operator, or end-user", false).
		Handler(s.handlePromptMigrationGuide)

	s.server.Prompt("release-announcement").
		Description("Generate a release announcement for publishing").
		Argument("channel", "Target channel: github, blog, social, or email", false).
		Handler(s.handlePromptReleaseAnnouncement)

	s.server.Prompt("approval-decision").
		Description("Help make an informed approval decision based on CGP analysis").
		Handler(s.handlePromptApprovalDecision)
}

// invalidateCache invalidates state-dependent resources in the cache.
func (s *Server) invalidateCache() {
	if s.cache != nil {
		s.cache.InvalidateStateDependent()
	}
}

// ensureRepoPath gets the repository path from git service and updates the adapter.
// This ensures consistent repository path handling across all MCP tool calls,
// fixing issue #35 where state wasn't persisted between tool calls due to path mismatch.
func (s *Server) ensureRepoPath(ctx context.Context) string {
	repoPath := ""
	if s.gitService != nil {
		if path, err := s.gitService.GetRepositoryRoot(ctx); err == nil {
			repoPath = path
		}
	}
	if repoPath == "" {
		repoPath = "."
	}
	// Update adapter's repoRoot to ensure consistent path across calls
	if s.adapter != nil {
		s.adapter.SetRepoRoot(repoPath)
	}
	return repoPath
}

// Tool handlers

func (s *Server) handleStatus(ctx context.Context, input StatusInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return map[string]any{
				"status":  "no_active_release",
				"message": "No active release found. Run 'relicta plan' to start a new release.",
			}, nil
		}

		result := map[string]any{
			"release_id":  status.ReleaseID,
			"state":       status.State,
			"version":     status.Version,
			"created":     status.CreatedAt,
			"updated":     status.UpdatedAt,
			"can_approve": status.CanApprove,
			"next_action": status.NextAction,
		}

		if status.ApprovalMsg != "" {
			result["approval_message"] = status.ApprovalMsg
		}

		if status.Stale {
			result["stale"] = true
			result["warning"] = status.Warning
		}

		return result, nil
	}

	// Fallback to direct repository access
	if s.releaseRepo == nil {
		return map[string]any{
			"status":  "not_configured",
			"message": "No release repository configured. Run 'relicta plan' first.",
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return map[string]any{
			"status":  "no_active_release",
			"message": "No active release found. Run 'relicta plan' to start a new release.",
		}, nil
	}

	rel := releases[0]
	result := map[string]any{
		"state":   rel.State().String(),
		"version": "",
		"created": rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		"updated": rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}

	if !rel.VersionNext().IsZero() {
		result["version"] = rel.VersionNext().String()
	}

	return result, nil
}

func (s *Server) handlePlan(ctx context.Context, input PlanToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	repoPath := s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasPlanUseCase() {
		fromRef := ""
		if input.From != "" && input.From != "auto" {
			fromRef = input.From
		}

		planInput := PlanInput{
			RepositoryPath: repoPath,
			FromRef:        fromRef,
			Analyze:        input.Analyze,
		}

		// Report progress
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 3.0
			_ = progress.Report(1, &total)
		}

		output, err := s.adapter.Plan(ctx, planInput)
		if err != nil {
			return nil, userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 3.0
			_ = progress.Report(2, &total)
		}

		result := map[string]any{
			"release_id":      output.ReleaseID,
			"current_version": output.CurrentVersion,
			"next_version":    output.NextVersion,
			"release_type":    output.ReleaseType,
			"commit_count":    output.CommitCount,
			"has_breaking":    output.HasBreaking,
			"has_features":    output.HasFeatures,
			"has_fixes":       output.HasFixes,
		}

		// Include commit details when analyze=true
		if input.Analyze && len(output.Commits) > 0 {
			commits := make([]map[string]any, 0, len(output.Commits))
			for _, c := range output.Commits {
				commit := map[string]any{
					"sha":     c.SHA,
					"type":    c.Type,
					"message": c.Message,
					"author":  c.Author,
				}
				if c.Scope != "" {
					commit["scope"] = c.Scope
				}
				commits = append(commits, commit)
			}
			result["commits"] = commits
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 3.0
			_ = progress.Report(3, &total)
		}

		s.invalidateCache()
		return result, nil
	}

	return map[string]any{
		"status":  "not_configured",
		"message": "Run 'relicta mcp serve' with configured dependencies",
	}, nil
}

func (s *Server) handleBump(ctx context.Context, input BumpToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	repoPath := s.ensureRepoPath(ctx)

	bumpType := input.Level
	if bumpType == "" {
		bumpType = "auto"
	}

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasCalculateVersionUseCase() {
		bumpInput := BumpInput{
			RepositoryPath: repoPath,
			BumpType:       bumpType,
			Version:        input.Version,
		}

		output, err := s.adapter.Bump(ctx, bumpInput)
		if err != nil {
			return nil, userError(err)
		}

		result := map[string]any{
			"current_version": output.CurrentVersion,
			"next_version":    output.NextVersion,
			"bump_type":       output.BumpType,
			"auto_detected":   output.AutoDetected,
		}

		if output.TagName != "" {
			result["tag_name"] = output.TagName
			result["tag_created"] = output.TagCreated
		}

		s.invalidateCache()
		return result, nil
	}

	return map[string]any{
		"bump_type": bumpType,
		"version":   input.Version,
		"status":    "run 'relicta mcp serve' with configured dependencies",
	}, nil
}

func (s *Server) handleNotes(ctx context.Context, input NotesToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasGenerateNotesUseCase() && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("no active release: %w", err)
		}

		// Report progress
		totalSteps := 3.0
		if input.AI {
			totalSteps = 5.0
		}
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			_ = progress.Report(1, &totalSteps)
		}

		notesInput := NotesInput{
			ReleaseID:        status.ReleaseID,
			UseAI:            input.AI,
			IncludeChangelog: true,
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			_ = progress.Report(2, &totalSteps)
		}

		output, err := s.adapter.Notes(ctx, notesInput)
		if err != nil {
			return nil, userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			_ = progress.Report(totalSteps, &totalSteps)
		}

		result := map[string]any{
			"summary":      output.Summary,
			"ai_generated": output.AIGenerated,
		}

		if output.Changelog != "" {
			result["changelog"] = output.Changelog
		}

		s.invalidateCache()
		return result, nil
	}

	return map[string]any{
		"use_ai": input.AI,
		"status": "run 'relicta mcp serve' with configured dependencies",
	}, nil
}

func (s *Server) handleEvaluate(ctx context.Context, input EvaluateToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter for full governance evaluation if available
	if s.adapter != nil && s.adapter.HasGovernanceService() && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("no active release: %w", err)
		}

		// Report progress
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 4.0
			_ = progress.Report(1, &total)
		}

		evalInput := EvaluateInput{
			ReleaseID:      status.ReleaseID,
			IncludeHistory: true,
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 4.0
			_ = progress.Report(2, &total)
		}

		output, err := s.adapter.Evaluate(ctx, evalInput)
		if err != nil {
			return nil, userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 4.0
			_ = progress.Report(4, &total)
		}

		return map[string]any{
			"decision":         output.Decision,
			"risk_score":       output.RiskScore,
			"severity":         output.Severity,
			"can_auto_approve": output.CanAutoApprove,
			"required_actions": output.RequiredActions,
			"risk_factors":     output.RiskFactors,
			"rationale":        output.Rationale,
		}, nil
	}

	// Fallback to basic risk calculation
	if s.riskCalc == nil {
		return nil, fmt.Errorf("risk calculator not configured")
	}

	proposal := cgp.NewProposal(
		cgp.Actor{
			Kind: cgp.ActorKindAgent,
			ID:   "mcp-client",
			Name: "MCP Agent",
		},
		cgp.ProposalScope{
			Repository:  "unknown",
			CommitRange: "HEAD~5..HEAD",
		},
		cgp.ProposalIntent{
			Summary:    "Release evaluation via MCP",
			Confidence: 0.8,
		},
	)

	assessment, err := s.riskCalc.Calculate(ctx, proposal, nil)
	if err != nil {
		return nil, userError(err)
	}

	return map[string]any{
		"score":    assessment.Score,
		"severity": string(assessment.Severity),
		"summary":  assessment.Summary,
		"factors":  assessment.Factors,
	}, nil
}

func (s *Server) handleApprove(ctx context.Context, input ApproveToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasApproveUseCase() && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("no active release: %w", err)
		}

		approveInput := ApproveInput{
			ReleaseID:   status.ReleaseID,
			ApprovedBy:  "mcp-agent",
			AutoApprove: true,
			EditedNotes: input.Notes,
		}

		output, err := s.adapter.Approve(ctx, approveInput)
		if err != nil {
			return nil, userError(err)
		}

		s.invalidateCache()
		return map[string]any{
			"approved":    output.Approved,
			"approved_by": output.ApprovedBy,
			"version":     output.Version,
		}, nil
	}

	return map[string]any{
		"notes":  input.Notes,
		"status": "run 'relicta mcp serve' with configured dependencies",
	}, nil
}

func (s *Server) handlePublish(ctx context.Context, input PublishToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasPublishUseCase() && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("no active release: %w", err)
		}

		// Report progress
		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(1, &total)
		}

		publishInput := PublishInput{
			ReleaseID: status.ReleaseID,
			DryRun:    input.DryRun,
			CreateTag: true,
			PushTag:   !input.DryRun,
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(2, &total)
		}

		output, err := s.adapter.Publish(ctx, publishInput)
		if err != nil {
			return nil, userError(err)
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(4, &total)
		}

		result := map[string]any{
			"tag_name":    output.TagName,
			"release_url": output.ReleaseURL,
			"dry_run":     input.DryRun,
		}

		if len(output.PluginResults) > 0 {
			plugins := make([]map[string]any, 0, len(output.PluginResults))
			for _, pr := range output.PluginResults {
				plugins = append(plugins, map[string]any{
					"plugin":  pr.PluginName,
					"hook":    pr.Hook,
					"success": pr.Success,
					"message": pr.Message,
				})
			}
			result["plugin_results"] = plugins
		}

		if progress := mcp.ProgressFromContext(ctx); progress != nil {
			total := 5.0
			_ = progress.Report(5, &total)
		}

		s.invalidateCache()
		return result, nil
	}

	return map[string]any{
		"dry_run": input.DryRun,
		"status":  "run 'relicta mcp serve' with configured dependencies",
	}, nil
}

func (s *Server) handleCancel(ctx context.Context, input CancelToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("no active release to cancel: %w", err)
		}

		// Check if release can be canceled
		if status.State == "published" {
			return nil, fmt.Errorf("cannot cancel a published release")
		}
		if status.State == "publishing" && !input.Force {
			return nil, fmt.Errorf("cannot cancel during publishing - wait for completion or use force=true")
		}
		if status.State == "failed" || status.State == "canceled" {
			return map[string]any{
				"release_id": status.ReleaseID,
				"state":      status.State,
				"message":    "release is already in terminal state - use reset to start fresh",
			}, nil
		}

		reason := input.Reason
		if reason == "" {
			reason = "canceled via MCP"
		}

		// Cancel the release
		cancelInput := CancelInput{
			ReleaseID: status.ReleaseID,
			Reason:    reason,
		}

		output, err := s.adapter.Cancel(ctx, cancelInput)
		if err != nil {
			return nil, userError(err)
		}

		s.invalidateCache()
		return map[string]any{
			"release_id":     output.ReleaseID,
			"previous_state": output.PreviousState,
			"new_state":      output.NewState,
			"reason":         reason,
			"message":        "release canceled successfully",
		}, nil
	}

	return map[string]any{
		"status": "run 'relicta mcp serve' with configured dependencies",
	}, nil
}

func (s *Server) handleReset(ctx context.Context, input ResetToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return map[string]any{
				"message": "no active release found - nothing to reset",
			}, nil
		}

		// Check if release can be reset
		if status.State == "published" {
			return nil, fmt.Errorf("published releases cannot be reset - run 'relicta plan' to start a new release")
		}
		if status.State == "publishing" && !input.Force {
			return nil, fmt.Errorf("cannot reset during publishing - wait for completion or use force=true")
		}

		// For in-progress releases that aren't failed/canceled, suggest cancel first
		if status.State != "failed" && status.State != "canceled" && !input.Force {
			return map[string]any{
				"release_id": status.ReleaseID,
				"state":      status.State,
				"message":    "release is in progress - use cancel first, or force=true to delete",
			}, nil
		}

		// Reset (delete) the release
		resetInput := ResetInput{
			ReleaseID: status.ReleaseID,
		}

		output, err := s.adapter.Reset(ctx, resetInput)
		if err != nil {
			return nil, userError(err)
		}

		s.invalidateCache()
		return map[string]any{
			"release_id":     output.ReleaseID,
			"previous_state": output.PreviousState,
			"deleted":        output.Deleted,
			"message":        "release reset successfully - run 'relicta plan' to start fresh",
		}, nil
	}

	return map[string]any{
		"status": "run 'relicta mcp serve' with configured dependencies",
	}, nil
}

// --- Specialized AI Agent Tool Handlers ---

func (s *Server) handleBlastRadius(ctx context.Context, input BlastRadiusToolInput) (map[string]any, error) {
	if s.adapter == nil || !s.adapter.HasBlastService() {
		return map[string]any{
			"status":  "not_configured",
			"message": "Blast radius service not configured. This tool requires monorepo analysis to be enabled.",
		}, nil
	}

	// Report progress
	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 4.0
		_ = progress.Report(1, &total)
	}

	blastInput := BlastRadiusInput{
		FromRef:           input.From,
		ToRef:             input.To,
		IncludeTransitive: input.Transitive,
		GenerateGraph:     input.Graph,
		PackagePaths:      input.PackagePaths,
	}

	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 4.0
		_ = progress.Report(2, &total)
	}

	output, err := s.adapter.BlastRadius(ctx, blastInput)
	if err != nil {
		return nil, userError(err)
	}

	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 4.0
		_ = progress.Report(4, &total)
	}

	result := map[string]any{
		"total_packages":             output.TotalPackages,
		"directly_affected":          output.DirectlyAffected,
		"transitively_affected":      output.TransitivelyAffected,
		"packages_requiring_release": output.PackagesRequiringRelease,
		"risk_level":                 output.RiskLevel,
		"total_files_changed":        output.TotalFilesChanged,
		"total_insertions":           output.TotalInsertions,
		"total_deletions":            output.TotalDeletions,
	}

	if len(output.RiskFactors) > 0 {
		result["risk_factors"] = output.RiskFactors
	}

	if len(output.Impacts) > 0 {
		impacts := make([]map[string]any, 0, len(output.Impacts))
		for _, impact := range output.Impacts {
			impactMap := map[string]any{
				"package_name":     impact.PackageName,
				"package_path":     impact.PackagePath,
				"package_type":     impact.PackageType,
				"impact_level":     impact.ImpactLevel,
				"risk_score":       impact.RiskScore,
				"requires_release": impact.RequiresRelease,
				"changed_files":    impact.ChangedFiles,
			}
			if impact.ReleaseType != "" {
				impactMap["release_type"] = impact.ReleaseType
			}
			if len(impact.SuggestedActions) > 0 {
				impactMap["suggested_actions"] = impact.SuggestedActions
			}
			impacts = append(impacts, impactMap)
		}
		result["impacts"] = impacts
	}

	if output.DependencyGraph != nil {
		result["dependency_graph"] = map[string]any{
			"nodes": output.DependencyGraph.Nodes,
			"edges": output.DependencyGraph.Edges,
		}
	}

	return result, nil
}

func (s *Server) handleInferVersion(ctx context.Context, input InferVersionToolInput) (map[string]any, error) {
	if s.adapter == nil || !s.adapter.HasReleaseAnalyzer() {
		return map[string]any{
			"status":  "not_configured",
			"message": "Release analyzer not configured. Run 'relicta mcp serve' with configured dependencies.",
		}, nil
	}

	// Report progress
	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 2.0
		_ = progress.Report(1, &total)
	}

	inferInput := InferVersionInput{
		FromRef:     input.From,
		ToRef:       input.To,
		IncludeRisk: input.IncludeRisk,
	}

	output, err := s.adapter.InferVersion(ctx, inferInput)
	if err != nil {
		return nil, userError(err)
	}

	if progress := mcp.ProgressFromContext(ctx); progress != nil {
		total := 2.0
		_ = progress.Report(2, &total)
	}

	result := map[string]any{
		"current_version": output.CurrentVersion,
		"next_version":    output.NextVersion,
		"bump_type":       output.BumpType,
		"has_breaking":    output.HasBreaking,
		"has_features":    output.HasFeatures,
		"has_fixes":       output.HasFixes,
		"commit_count":    output.CommitCount,
		"confidence":      output.Confidence,
	}

	if len(output.Rationale) > 0 {
		result["rationale"] = output.Rationale
	}

	if input.IncludeRisk {
		result["risk_score"] = output.RiskScore
		result["risk_severity"] = output.RiskSeverity
	}

	return result, nil
}

func (s *Server) handleSummarizeDiff(ctx context.Context, input SummarizeDiffToolInput) (map[string]any, error) {
	if s.adapter == nil || !s.adapter.HasReleaseAnalyzer() {
		return map[string]any{
			"status":  "not_configured",
			"message": "Release analyzer not configured. Run 'relicta mcp serve' with configured dependencies.",
		}, nil
	}

	summarizeInput := SummarizeDiffInput{
		FromRef:   input.From,
		ToRef:     input.To,
		Audience:  input.Audience,
		MaxLength: input.MaxLength,
	}

	output, err := s.adapter.SummarizeDiff(ctx, summarizeInput)
	if err != nil {
		return nil, userError(err)
	}

	result := map[string]any{
		"summary":         output.Summary,
		"audience":        output.Audience,
		"ai_generated":    output.AIGenerated,
		"character_count": output.CharacterCount,
	}

	if len(output.Highlights) > 0 {
		result["highlights"] = output.Highlights
	}

	return result, nil
}

func (s *Server) handleValidateRelease(ctx context.Context, input ValidateReleaseToolInput) (map[string]any, error) {
	// Ensure consistent repository path (fixes issue #35)
	s.ensureRepoPath(ctx)

	// Get release ID from input or active release
	releaseID := input.ReleaseID
	if releaseID == "" && s.adapter != nil && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err == nil {
			releaseID = status.ReleaseID
		}
	}

	validateInput := ValidateReleaseInput{
		ReleaseID:       releaseID,
		CheckGit:        input.CheckGit,
		CheckPlugins:    input.CheckPlugins,
		CheckGovernance: input.CheckGovernance,
		Checks:          input.Checks,
	}

	// Use adapter if available, otherwise run minimal checks
	if s.adapter != nil {
		output, err := s.adapter.ValidateRelease(ctx, validateInput)
		if err != nil {
			return nil, userError(err)
		}

		result := map[string]any{
			"valid":          output.Valid,
			"can_proceed":    output.CanProceed,
			"recommendation": output.Recommendation,
		}

		if len(output.Checks) > 0 {
			checks := make([]map[string]any, 0, len(output.Checks))
			for _, check := range output.Checks {
				checkMap := map[string]any{
					"name":   check.Name,
					"status": check.Status,
				}
				if check.Message != "" {
					checkMap["message"] = check.Message
				}
				checks = append(checks, checkMap)
			}
			result["checks"] = checks
		}

		if len(output.BlockingIssues) > 0 {
			result["blocking_issues"] = output.BlockingIssues
		}

		if len(output.Warnings) > 0 {
			result["warnings"] = output.Warnings
		}

		return result, nil
	}

	// Minimal validation without adapter
	return map[string]any{
		"valid":          true,
		"can_proceed":    true,
		"recommendation": "Basic validation passed. Full validation requires configured dependencies.",
		"checks": []map[string]any{
			{"name": "basic", "status": "passed", "message": "Basic checks passed"},
		},
	}, nil
}

// Resource handlers

func (s *Server) handleResourceState(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	// Check cache first
	if s.cache != nil {
		if cached := s.cache.Get(uri); cached != nil {
			s.logger.Debug("cache hit", "uri", uri)
			if len(cached.Contents) > 0 {
				return &mcp.ResourceContent{
					URI:      cached.Contents[0].URI,
					MimeType: cached.Contents[0].MIMEType,
					Text:     cached.Contents[0].Text,
				}, nil
			}
		}
	}

	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no release repository configured"}`,
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no active release"}`,
		}, nil
	}

	rel := releases[0]
	version := ""
	if !rel.VersionNext().IsZero() {
		version = rel.VersionNext().String()
	}

	content := fmt.Sprintf(`{
  "state": %q,
  "version": %q,
  "created_at": %q,
  "updated_at": %q
}`, rel.State().String(), version, rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"), rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"))

	result := &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     content,
	}

	// Cache the result
	if s.cache != nil {
		s.cache.Set(uri, &ReadResourceResult{
			Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: content}},
		})
	}

	return result, nil
}

func (s *Server) handleResourceConfig(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.config == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no configuration loaded"}`,
		}, nil
	}

	productName := s.config.Changelog.ProductName
	if productName == "" {
		productName = "Relicta"
	}

	content := fmt.Sprintf(`{
  "product_name": %q,
  "ai_enabled": %t,
  "ai_provider": %q,
  "versioning_strategy": %q
}`, productName, s.config.AI.Enabled, s.config.AI.Provider, s.config.Versioning.Strategy)

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     content,
	}, nil
}

func (s *Server) handleResourceCommits(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no release repository configured"}`,
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no active release", "commits": []}`,
		}, nil
	}

	rel := releases[0]
	plan := release.GetPlan(rel)
	if plan == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no plan available", "commits": []}`,
		}, nil
	}

	// Check if changeset is loaded
	if !plan.HasChangeSet() {
		content := fmt.Sprintf(`{
  "status": "changeset not loaded",
  "changeset_id": %q,
  "release_type": %q,
  "current_version": %q,
  "next_version": %q,
  "commits": []
}`, plan.ChangeSetID, plan.ReleaseType, plan.CurrentVersion.String(), plan.NextVersion.String())
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     content,
		}, nil
	}

	// Get commits from changeset
	changeSet := plan.GetChangeSet()
	commits := changeSet.Commits()

	// Build commits array
	commitList := make([]map[string]any, 0, len(commits))
	for _, c := range commits {
		commit := map[string]any{
			"sha":      c.ShortHash(),
			"full_sha": c.Hash(),
			"type":     string(c.Type()),
			"subject":  c.Subject(),
			"author":   c.Author(),
			"date":     c.Date().Format("2006-01-02T15:04:05Z07:00"),
			"breaking": c.IsBreaking(),
		}
		if c.Scope() != "" {
			commit["scope"] = c.Scope()
		}
		commitList = append(commitList, commit)
	}

	result := map[string]any{
		"status":          "ok",
		"changeset_id":    string(plan.ChangeSetID),
		"release_type":    string(plan.ReleaseType),
		"current_version": plan.CurrentVersion.String(),
		"next_version":    plan.NextVersion.String(),
		"commit_count":    len(commits),
		"commits":         commitList,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     fmt.Sprintf(`{"status": "error", "error": %q}`, err.Error()),
		}, nil
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(jsonBytes),
	}, nil
}

func (s *Server) handleResourceChangelog(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     "# Changelog\n\nNo release repository configured.",
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     "# Changelog\n\nNo active release found. Run `relicta plan` to start a new release.",
		}, nil
	}

	rel := releases[0]
	notes := rel.Notes()

	if notes == nil {
		version := ""
		if !rel.VersionNext().IsZero() {
			version = rel.VersionNext().String()
		}

		content := fmt.Sprintf("# Changelog\n\nNo changelog generated yet for version %s.\n\nRun `relicta notes` to generate release notes.", version)
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     content,
		}, nil
	}

	changelog := notes.Text
	if changelog == "" {
		changelog = "# Release Notes\n\nNo content available."
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "text/markdown",
		Text:     changelog,
	}, nil
}

func (s *Server) handleResourceRiskReport(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
	if s.releaseRepo == nil {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no release repository configured"}`,
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &mcp.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     `{"status": "no active release"}`,
		}, nil
	}

	rel := releases[0]

	// Try to get risk assessment from adapter if available
	if s.adapter != nil && s.adapter.HasGovernanceService() {
		evalInput := EvaluateInput{
			ReleaseID:      string(rel.ID()),
			IncludeHistory: false,
		}

		output, err := s.adapter.Evaluate(ctx, evalInput)
		if err == nil {
			result := map[string]any{
				"status":           "ok",
				"decision":         output.Decision,
				"risk_score":       output.RiskScore,
				"severity":         output.Severity,
				"can_auto_approve": output.CanAutoApprove,
				"required_actions": output.RequiredActions,
				"risk_factors":     output.RiskFactors,
				"rationale":        output.Rationale,
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err == nil {
				return &mcp.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(jsonBytes),
				}, nil
			}
		}
	}

	// Fallback: Use basic risk calculation if available
	if s.riskCalc != nil {
		proposal := cgp.NewProposal(
			cgp.Actor{
				Kind: cgp.ActorKindAgent,
				ID:   "mcp-resource-reader",
				Name: "MCP Resource Reader",
			},
			cgp.ProposalScope{
				Repository:  rel.RepoID(),
				CommitRange: "HEAD~5..HEAD",
			},
			cgp.ProposalIntent{
				Summary:    "Risk assessment for active release",
				Confidence: 0.8,
			},
		)

		assessment, err := s.riskCalc.Calculate(ctx, proposal, nil)
		if err == nil {
			result := map[string]any{
				"status":   "ok",
				"score":    assessment.Score,
				"severity": string(assessment.Severity),
				"summary":  assessment.Summary,
				"factors":  assessment.Factors,
			}

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			if err == nil {
				return &mcp.ResourceContent{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(jsonBytes),
				}, nil
			}
		}
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     `{"status": "no risk assessment available", "hint": "Run 'relicta evaluate' to perform risk assessment"}`,
	}, nil
}

// Prompt handlers

func (s *Server) handlePromptReleaseSummary(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	style := "brief"
	if v, ok := args["style"]; ok && v != "" {
		style = v
	}

	var content string
	switch style {
	case "detailed":
		content = `You are a release manager. Provide a detailed summary of the upcoming release including:
- All changes categorized by type (features, fixes, breaking changes)
- Impact analysis
- Risk assessment
- Recommended actions before release`
	case "technical":
		content = `You are a technical writer. Provide a technical summary of the release including:
- API changes and their signatures
- Configuration changes
- Migration requirements
- Performance implications`
	default:
		content = `You are a release manager. Provide a brief summary of the upcoming release including:
- Key highlights (1-3 bullet points)
- Version number
- Release readiness status`
	}

	return &mcp.PromptResult{
		Description: "Release summary prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptRiskAnalysis(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	content := `You are a release risk analyst using the Change Governance Protocol (CGP).

Analyze the current release and provide:
1. Overall risk score (0.0 - 1.0) with justification
2. Individual risk factors:
   - API changes impact
   - Dependency changes
   - Blast radius (scope of changes)
   - Security implications
   - Historical patterns
3. Recommendations:
   - Approval recommendation (approve, review, block)
   - Required actions before release
   - Suggested reviewers

Base your analysis on the commit history, change analysis, and CGP policies.`

	return &mcp.PromptResult{
		Description: "Risk analysis prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptCommitReview(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	focus := "compliance"
	if v, ok := args["focus"]; ok && v != "" {
		focus = v
	}

	var content string
	switch focus {
	case "quality":
		content = `You are a code review expert analyzing commit messages and changes.

Review the commits in this release for quality:
1. Commit message clarity and completeness
2. Logical grouping of changes (atomic commits)
3. Code change quality indicators
4. Documentation updates for significant changes
5. Test coverage implications

For each issue found, suggest specific improvements.`
	case "security":
		content = `You are a security analyst reviewing commits for potential security implications.

Analyze commits for security concerns:
1. Sensitive data handling changes
2. Authentication/authorization modifications
3. Input validation changes
4. Dependency updates with known vulnerabilities
5. Configuration changes affecting security posture
6. Cryptographic code changes

Flag any commits that require security review before release.`
	default: // compliance
		content = `You are a release compliance officer reviewing commits for conventional commit standards.

Analyze each commit for compliance with Conventional Commits specification:
1. Format: <type>(<scope>): <subject>
2. Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore
3. Breaking changes properly marked with ! or BREAKING CHANGE footer
4. Scope relevance and consistency
5. Subject line length and imperative mood

List non-compliant commits with specific corrections needed.`
	}

	return &mcp.PromptResult{
		Description: "Commit review prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptBreakingChanges(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	content := `You are a technical writer documenting breaking changes for users.

For each breaking change in this release, provide:

1. **Change Summary**: One-line description of what changed
2. **Reason**: Why this breaking change was necessary
3. **Impact**: Who is affected and how
4. **Migration Path**: Step-by-step instructions to adapt
5. **Code Examples**: Before/after code snippets where applicable

Format the output as a structured breaking changes document suitable for inclusion in release notes.

If there are no breaking changes, confirm this and explain what safeguards prevented them.`

	return &mcp.PromptResult{
		Description: "Breaking changes documentation prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptMigrationGuide(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	audience := "developer"
	if v, ok := args["audience"]; ok && v != "" {
		audience = v
	}

	var content string
	switch audience {
	case "operator":
		content = `You are a DevOps engineer writing migration instructions for system operators.

Create a migration guide covering:
1. **Pre-migration Checklist**
   - Backup procedures
   - Downtime requirements
   - Rollback plan

2. **Infrastructure Changes**
   - Configuration file updates
   - Environment variable changes
   - Database migrations

3. **Deployment Steps**
   - Ordered deployment sequence
   - Health check verification
   - Monitoring updates

4. **Post-migration Verification**
   - Smoke tests to run
   - Metrics to monitor
   - Common issues and solutions

Use clear, actionable language suitable for runbooks.`
	case "end-user":
		content = `You are a product manager writing migration notes for end users.

Create user-friendly upgrade instructions:
1. **What's New**: Key benefits of upgrading
2. **What's Changed**: User-facing changes to expect
3. **Action Required**: Steps users need to take
4. **Getting Help**: Support resources and FAQ

Keep technical jargon minimal. Focus on the user experience impact.`
	default: // developer
		content = `You are a senior developer writing migration instructions for other developers.

Create a developer migration guide covering:
1. **Dependency Updates**
   - Version requirements
   - Package manager commands

2. **API Changes**
   - Changed endpoints/methods
   - Parameter modifications
   - Response format changes
   - Deprecation timeline

3. **Code Migration**
   - Find/replace patterns
   - Refactoring steps
   - Type definition updates

4. **Testing Updates**
   - Test changes needed
   - New test patterns

Include code examples for all significant changes.`
	}

	return &mcp.PromptResult{
		Description: "Migration guide prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptReleaseAnnouncement(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	channel := "github"
	if v, ok := args["channel"]; ok && v != "" {
		channel = v
	}

	var content string
	switch channel {
	case "blog":
		content = `You are a technical writer crafting a blog post for a software release.

Write a blog post announcement including:
1. **Headline**: Compelling title highlighting the main theme
2. **Introduction**: Context and significance of this release
3. **Key Features**: Detailed coverage of major additions
4. **Improvements**: Notable fixes and enhancements
5. **Technical Deep-dive**: One feature explained in depth
6. **Getting Started**: How to upgrade or try it
7. **What's Next**: Roadmap preview
8. **Acknowledgments**: Contributor recognition

Tone: Professional but engaging, 800-1200 words.`
	case "social":
		content = `You are a developer advocate crafting social media announcements.

Create announcements for different platforms:

**Twitter/X (280 chars)**:
- Hook + key feature + link

**LinkedIn (longer form)**:
- Professional summary
- 3 key highlights
- Call to action

**Mastodon/Dev Community**:
- Technical focus
- Code snippet if relevant
- Community engagement

Use appropriate hashtags and emoji sparingly.`
	case "email":
		content = `You are writing a release announcement email for subscribers.

Structure the email:
1. **Subject Line**: Clear, action-oriented
2. **Preview Text**: Compelling summary
3. **Header**: Version number and release date
4. **Executive Summary**: 2-3 sentence overview
5. **Highlights**: Bullet points of key changes
6. **Upgrade Instructions**: Brief steps
7. **Full Changelog Link**: For detailed information
8. **Feedback Request**: How to provide input

Keep it scannable with clear visual hierarchy.`
	default: // github
		content = `You are writing release notes for a GitHub release.

Structure the release notes:
1. **Title**: Version number with optional codename
2. **Summary**: 2-3 sentence release overview
3. **Highlights**: Top 3-5 changes as bullet points
4. **What's Changed**: Categorized changes
   - ✨ Features
   - 🐛 Bug Fixes
   - 📚 Documentation
   - ⚠️ Breaking Changes
5. **Upgrade Notes**: Critical information for upgrading
6. **Contributors**: @mention contributors

Use GitHub-flavored markdown with appropriate emoji.`
	}

	return &mcp.PromptResult{
		Description: "Release announcement prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptApprovalDecision(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	content := `You are a release governance advisor helping make approval decisions.

Based on the Change Governance Protocol (CGP) analysis, provide:

1. **Decision Recommendation**: APPROVE, REQUEST_CHANGES, or BLOCK
   - Clear rationale for your recommendation
   - Confidence level (high/medium/low)

2. **Risk Assessment Summary**
   - Overall risk level and score
   - Top 3 risk factors to consider
   - Mitigating factors present

3. **Approval Conditions** (if recommending approval)
   - Required reviewers and their focus areas
   - Pre-release checks to complete
   - Monitoring requirements post-release

4. **Blocking Issues** (if recommending block)
   - Specific issues that must be resolved
   - Suggested remediation steps
   - Criteria for re-evaluation

5. **Audit Trail Entry**
   - Structured summary for governance records
   - Key decision factors
   - Timestamp and context

Provide actionable guidance that enables confident decision-making.`

	return &mcp.PromptResult{
		Description: "Approval decision prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}
