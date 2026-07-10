// Package mcp provides MCP server implementation for Relicta using go.klarlabs.de/mcp.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.klarlabs.de/mcp"
	mcpmw "go.klarlabs.de/mcp/middleware"
	"go.klarlabs.de/mcp/transport"

	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	cgpprotocol "github.com/relicta-tech/relicta/v4/internal/cgp/protocol"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	relictaerrors "github.com/relicta-tech/relicta/v4/internal/errors"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// ConfigReloader is called after relicta_init creates a config file mid-session.
// It reloads the config and reinitializes the container/adapter so that
// subsequent tool calls work without restarting the MCP server.
// Returns the new config and adapter (adapter may be nil if initialization fails).
type ConfigReloader func(ctx context.Context) (*config.Config, *Adapter, error)

// Server wraps the MCP server for Relicta.
type Server struct {
	server  *mcp.Server
	version string
	logger  *slog.Logger

	// mu protects config and adapter which may be updated by handleInit's
	// config reloader on the HTTP transport where concurrent requests are possible.
	mu sync.RWMutex

	// Dependencies for tool execution
	config       *config.Config
	gitService   git.Service
	releaseRepo  release.Repository
	policyEngine *policy.Engine
	riskCalc     *risk.Calculator
	evaluator    *evaluator.Evaluator

	// CGP protocol service for wire format tools
	cgpService *cgpprotocol.Service

	// Application layer adapter
	adapter *Adapter

	// Resource cache for improved read performance
	cache *ResourceCache

	// configReloader reinitializes config and adapter after init creates a config file.
	configReloader ConfigReloader

	// actorBudgets enforces per-actor autonomy budgets on privileged MCP tools.
	// When nil, checkBudget falls back to DefaultRestrictiveAgentBudget — agent
	// callers fail closed by design.
	actorBudgets *policy.ActorBudgetSet
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

// WithActorBudgets sets the per-actor autonomy budget set used to gate
// privileged MCP tools (relicta_approve, relicta_publish, relicta_reset,
// relicta_cancel). When unset, the server falls back to a restrictive default
// budget for any agent caller.
func WithActorBudgets(set *policy.ActorBudgetSet) ServerOption {
	return func(s *Server) {
		s.actorBudgets = set
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

// WithCGPService sets the CGP protocol service for wire format tools.
func WithCGPService(svc *cgpprotocol.Service) ServerOption {
	return func(s *Server) {
		s.cgpService = svc
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

// WithConfigReloader sets a callback to reload config after init creates a config file.
func WithConfigReloader(reloader ConfigReloader) ServerOption {
	return func(s *Server) {
		s.configReloader = reloader
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

// toJSONString converts a map to a JSON string for MCP text content.
// MCP spec requires content[].text to be a string, not a JSON object.
func toJSONString(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// errNotConfigured reports a tool invocation that cannot run because the
// server is missing its release-service wiring. This must be an error, not
// a success-shaped payload: returning {"status": "run 'relicta mcp serve'
// ..."} made agents treat a no-op as a completed operation (issue #128 —
// reset reported a confusing hint while resetting nothing).
func errNotConfigured(tool string) error {
	return fmt.Errorf("%s is unavailable: release services are not configured on this MCP server — start it with 'relicta mcp serve' from an initialized repository ('relicta init')", tool)
}

// Tool input types with JSON Schema generation via struct tags.

// StatusInput represents input for the status tool.
// Maps to CLI: relicta status (no additional flags)
// Returns current release state, version, and next recommended action.
type StatusInput struct{}

// InitToolInput represents input for the init tool.
// Maps to CLI: relicta init [--force] [--format FORMAT]
// Creates a new .relicta.yaml configuration file with sensible defaults.
type InitToolInput struct {
	Force  bool   `json:"force,omitempty" jsonschema:"description=Overwrite existing configuration file if one exists."`
	Format string `json:"format,omitempty" jsonschema:"description=Configuration file format.,enum=yaml|json,default=yaml"`
}

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

// --- CGP Protocol Wire Format Tool Inputs ---

// CGPProposeToolInput represents input for the cgp_propose tool.
type CGPProposeToolInput struct {
	ActorKind   string   `json:"actor_kind" jsonschema:"description=Actor type proposing the change.,enum=agent|ci|human|system"`
	ActorID     string   `json:"actor_id" jsonschema:"description=Unique identifier for the actor (e.g. 'agent:cursor' or 'ci:github-actions')."`
	ActorName   string   `json:"actor_name,omitempty" jsonschema:"description=Human-readable name for the actor."`
	Repository  string   `json:"repository" jsonschema:"description=Target repository in owner/repo format."`
	CommitRange string   `json:"commit_range" jsonschema:"description=Commit range to evaluate in from..to format (e.g. 'v1.0.0..HEAD')."`
	Summary     string   `json:"summary" jsonschema:"description=Human-readable description of the proposed changes."`
	Confidence  float64  `json:"confidence" jsonschema:"description=Proposer's confidence in their assessment (0.0-1.0)."`
	Categories  []string `json:"categories,omitempty" jsonschema:"description=Change categories: feature, bugfix, security, performance, documentation."`
}

// CGPAuthorizeToolInput represents input for the cgp_authorize tool.
type CGPAuthorizeToolInput struct {
	ProposalID string `json:"proposal_id" jsonschema:"description=ID of the proposal to authorize."`
	DecisionID string `json:"decision_id" jsonschema:"description=ID of the governance decision."`
	ApproverID string `json:"approver_id" jsonschema:"description=ID of the approver (e.g. 'human:alice@example.com')."`
	Version    string `json:"version" jsonschema:"description=Version to release (e.g. '1.2.0')."`
}

// CGPStatusToolInput represents input for the cgp_status tool.
type CGPStatusToolInput struct {
	ProposalID string `json:"proposal_id" jsonschema:"description=ID of the proposal to query status for."`
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

	// Create the MCP server with go.klarlabs.de/mcp
	s.server = mcp.NewServer(mcp.ServerInfo{
		Name:        "relicta",
		Version:     version,
		Title:       "Relicta Release Governance",
		Description: "Governs software change with risk scoring, policy enforcement, and audit trails",
		WebsiteURL:  "https://relicta.tech",
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

	// Register MCP app resources
	s.registerApps()

	return s, nil
}

// mcpMiddlewareStack returns the MCP middleware chain for all transports.
func (s *Server) mcpMiddlewareStack() []mcp.Middleware {
	return []mcp.Middleware{
		mcpmw.RequestID(),
		mcpmw.Recover(),
		mcpmw.Tracing(),
		mcpmw.Timeout(30 * time.Second),
		s.auditMiddleware(),
	}
}

// auditMiddleware creates an MCP audit middleware that logs to slog.
func (s *Server) auditMiddleware() mcp.Middleware {
	return mcpmw.NewAuditMiddleware(&slogAuditLogger{logger: s.logger}).Middleware()
}

// slogAuditLogger adapts mcp-go's AuditLogger to slog.
type slogAuditLogger struct {
	logger *slog.Logger
}

func (l *slogAuditLogger) LogEvent(_ context.Context, event mcpmw.AuditEvent) {
	l.logger.Info("mcp audit",
		"method", event.Method,
		"action", event.Action,
		"status", event.Status,
		"duration", event.Duration,
		"actor", event.Actor,
		"correlation_id", event.CorrelationID,
	)
}

// ServeStdio starts the MCP server on stdio transport with middleware.
func (s *Server) ServeStdio() error {
	s.logger.Info("MCP server started", "version", s.version)
	return mcp.ServeStdio(context.Background(), s.server,
		mcp.WithMiddleware(s.mcpMiddlewareStack()...),
	)
}

// ServeHTTP starts the MCP server on HTTP transport with middleware,
// discovery endpoint, and security features.
func (s *Server) ServeHTTP(ctx context.Context, address string) error {
	s.logger.Info("MCP server started", "version", s.version, "transport", "http", "address", address)

	httpOpts := []mcp.HTTPOption{
		mcp.WithDiscovery(&transport.ServerDiscovery{
			MCPPVersion: "2025-11-25",
			Server: transport.ServerInfo{
				Name:        "relicta",
				Version:     s.version,
				Title:       "Relicta Release Governance",
				Description: "Governs software change with risk scoring, policy enforcement, and audit trails",
				WebsiteURL:  "https://relicta.tech",
			},
			Capabilities: transport.ServerCapabilities{
				Tools:     true,
				Resources: true,
				Prompts:   true,
			},
		}),
	}

	serveOpts := []mcp.ServeOption{
		mcp.WithMiddleware(s.mcpMiddlewareStack()...),
	}

	return mcp.ServeHTTPWithMiddleware(ctx, s.server, address, httpOpts, serveOpts...)
}

// registerTools registers all tool handlers.
func (s *Server) registerTools() {
	// Status tool
	s.server.Tool("relicta_status").
		Description("Get the current release state and pending actions").
		UIResource("ui://relicta/status").
		OutputSchema(StatusToolOutput{}).
		Handler(s.handleStatus)

	// Init tool
	s.server.Tool("relicta_init").
		Description("Initialize a new Relicta configuration file with sensible defaults").
		Handler(s.handleInit)

	// Plan tool
	s.server.Tool("relicta_plan").
		Description("Analyze commits since the last release and suggest a version bump").
		UIResource("ui://relicta/commits").
		OutputSchema(PlanToolOutput{}).
		Handler(s.handlePlan)

	// Bump tool
	s.server.Tool("relicta_bump").
		Description("Calculate and set the next version based on commits").
		Handler(s.handleBump)

	// Notes tool
	s.server.Tool("relicta_notes").
		Description("Generate changelog and release notes for the current release").
		Handler(s.handleNotes)

	// Evaluate tool
	s.server.Tool("relicta_evaluate").
		Description("Evaluate release risk using the Change Governance Protocol (CGP)").
		UIResource("ui://relicta/risk").
		Handler(s.handleEvaluate)

	// Approve tool
	s.server.Tool("relicta_approve").
		Description("Approve the release for publishing").
		UIResource("ui://relicta/approval").
		Handler(s.handleApprove)

	// Publish tool
	s.server.Tool("relicta_publish").
		Description("Execute the release by creating tags and running plugins").
		UIResource("ui://relicta/pipeline").
		Handler(s.handlePublish)

	// Cancel tool
	s.server.Tool("relicta_cancel").
		Description("Cancel the current in-progress release").
		Handler(s.handleCancel)

	// Reset tool
	s.server.Tool("relicta_reset").
		Description("Reset a failed or canceled release to allow starting fresh").
		Handler(s.handleReset)

	// --- Specialized AI Agent Tools ---

	// Blast Radius tool - Monorepo change impact analysis
	s.server.Tool("relicta_blast_radius").
		Description("Analyze blast radius of changes in a monorepo. Returns impacted packages, transitive dependencies, and deployment risk assessment.").
		UIResource("ui://relicta/blast").
		OutputSchema(BlastRadiusOutput{}).
		Handler(s.handleBlastRadius)

	// Infer Version tool - Lightweight version inference
	s.server.Tool("relicta_infer_version").
		Description("Infer the next semantic version based on commits. Lightweight alternative to plan for quick queries.").
		OutputSchema(InferVersionToolOutput{}).
		Handler(s.handleInferVersion)

	// Summarize Diff tool - Audience-tailored change summaries
	s.server.Tool("relicta_summarize_diff").
		Description("Generate audience-tailored summary of changes between refs. Supports developer, operator, and end-user audiences.").
		Handler(s.handleSummarizeDiff)

	// Validate Release tool - Pre-flight checks
	s.server.Tool("relicta_validate_release").
		Description("Run pre-flight validation checks before release. Validates git state, plugins, and governance requirements.").
		UIResource("ui://relicta/approval").
		OutputSchema(ValidateReleaseToolOutput{}).
		Handler(s.handleValidateRelease)

	// --- CGP Protocol Wire Format Tools ---

	// CGP Propose tool - Submit a CGP ChangeProposal for governance evaluation
	s.server.Tool("cgp_propose").
		Description("Submit a CGP ChangeProposal for governance evaluation. Returns a GovernanceDecision with risk score, rationale, and required actions.").
		Handler(s.handleCGPPropose)

	// CGP Authorize tool - Record an ExecutionAuthorization for an approved proposal
	s.server.Tool("cgp_authorize").
		Description("Record an ExecutionAuthorization for an approved proposal. Requires a prior governance decision.").
		Handler(s.handleCGPAuthorize)

	// CGP Status tool - Query the current governance state of a proposal
	s.server.Tool("cgp_status").
		Description("Query the current governance state (proposed, decided, authorized) for a CGP proposal by ID.").
		OutputSchema(CGPStatusToolOutput{}).
		Handler(s.handleCGPStatus)
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

	// Reverse-MCP governance-as-context resources — coding agents read these
	// BEFORE proposing changes so they plan with org policy in scope.
	s.registerGovernanceContextResources()
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
func (s *Server) ensureCGPService() error {
	if s.cgpService != nil {
		return nil
	}

	// Lazily create a CGP service from the evaluator if available.
	if s.evaluator != nil {
		s.cgpService = cgpprotocol.NewService(s.evaluator)
		return nil
	}

	return fmt.Errorf("CGP protocol service not configured")
}

// timeNowUTC returns the current time in UTC. Extracted for testability.
func timeNowUTC() time.Time {
	return time.Now().UTC()
}
