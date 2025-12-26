package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

// Context keys for progress tracking.
type ctxKey string

const (
	ctxKeyProgressToken   ctxKey = "progressToken"
	ctxKeyProgressWriter  ctxKey = "progressWriter"
	ctxKeyProgressCounter ctxKey = "progressCounter"
)

// ProgressWriter is used by tool handlers to send progress notifications.
type ProgressWriter interface {
	WriteProgress(notification *ProgressNotification) error
}

// ContextWithProgress adds progress tracking to a context.
func ContextWithProgress(ctx context.Context, token string, writer ProgressWriter) context.Context {
	ctx = context.WithValue(ctx, ctxKeyProgressToken, token)
	ctx = context.WithValue(ctx, ctxKeyProgressWriter, writer)
	ctx = context.WithValue(ctx, ctxKeyProgressCounter, &progressCounter{})
	return ctx
}

// progressCounter tracks the current progress value to ensure it increases.
type progressCounter struct {
	mu      sync.Mutex
	current float64
}

func (c *progressCounter) next(delta float64) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current += delta
	return c.current
}

// SendProgress sends a progress notification if a progress token is present in the context.
// Returns silently if no progress tracking is configured.
func SendProgress(ctx context.Context, message string, total float64) error {
	token, ok := ctx.Value(ctxKeyProgressToken).(string)
	if !ok || token == "" {
		return nil // No progress tracking, silently succeed
	}

	writer, ok := ctx.Value(ctxKeyProgressWriter).(ProgressWriter)
	if !ok || writer == nil {
		return nil
	}

	counter, ok := ctx.Value(ctxKeyProgressCounter).(*progressCounter)
	if !ok {
		return nil
	}

	notification := &ProgressNotification{
		ProgressToken: token,
		Progress:      counter.next(1),
		Total:         total,
		Message:       message,
	}

	return writer.WriteProgress(notification)
}

// SendProgressPercent sends a progress notification with a percentage value.
func SendProgressPercent(ctx context.Context, message string, percent float64) error {
	token, ok := ctx.Value(ctxKeyProgressToken).(string)
	if !ok || token == "" {
		return nil
	}

	writer, ok := ctx.Value(ctxKeyProgressWriter).(ProgressWriter)
	if !ok || writer == nil {
		return nil
	}

	counter, ok := ctx.Value(ctxKeyProgressCounter).(*progressCounter)
	if !ok {
		return nil
	}

	notification := &ProgressNotification{
		ProgressToken: token,
		Progress:      counter.next(percent),
		Total:         100,
		Message:       message,
	}

	return writer.WriteProgress(notification)
}

// Server implements the MCP server for Relicta.
type Server struct {
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

	// Tool and resource handlers
	tools     map[string]ToolHandler
	resources map[string]ResourceHandler
	prompts   map[string]PromptHandler

	// Progress notification writer (set during ServeStdio)
	progressWriter ProgressWriter

	// Resource cache for improved read performance
	cache *ResourceCache
}

// ToolHandler handles a tool call.
type ToolHandler func(ctx context.Context, args map[string]any) (*CallToolResult, error)

// ResourceHandler handles a resource read.
type ResourceHandler func(ctx context.Context, uri string) (*ReadResourceResult, error)

// PromptHandler handles a prompt request.
type PromptHandler func(ctx context.Context, args map[string]string) (*GetPromptResult, error)

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

// NewServer creates a new MCP server for Relicta.
func NewServer(version string, opts ...ServerOption) (*Server, error) {
	s := &Server{
		version:   version,
		logger:    slog.Default(),
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		prompts:   make(map[string]PromptHandler),
		cache:     NewResourceCache(), // Enable caching by default
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create defaults if not provided
	if s.riskCalc == nil {
		s.riskCalc = risk.NewCalculatorWithDefaults()
	}

	// Register handlers
	s.registerTools()
	s.registerResources()
	s.registerPrompts()

	return s, nil
}

// ServeStdio starts the MCP server on stdio transport.
func (s *Server) ServeStdio() error {
	return s.Serve(os.Stdin, os.Stdout)
}

// Serve starts the MCP server with custom reader/writer.
func (s *Server) Serve(reader io.Reader, writer io.Writer) error {
	transport := NewStdioTransport(reader, writer)
	loop := NewMessageLoop(transport, s)

	// Set progress writer for streaming progress notifications
	s.progressWriter = transport

	s.logger.Info("MCP server started", "version", s.version)
	return loop.Run(context.Background())
}

// HandleRequest implements MessageHandler.
func (s *Server) HandleRequest(ctx context.Context, req *Request) *Response {
	s.logger.Debug("handling request", "method", req.Method, "id", req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		return nil
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(ctx, req)
	case "resources/list":
		return s.handleListResources(req)
	case "resources/read":
		return s.handleReadResource(ctx, req)
	case "prompts/list":
		return s.handleListPrompts(req)
	case "prompts/get":
		return s.handleGetPrompt(ctx, req)
	case "ping":
		return NewResponse(req.ID, map[string]any{})
	default:
		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, "Method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req *Request) *Response {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
	}

	result := InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
			Prompts:   &PromptsCapability{},
			Logging:   &LoggingCapability{},
		},
		ServerInfo: Implementation{
			Name:    "relicta",
			Version: s.version,
		},
		Instructions: `Relicta is an AI-powered release management tool.

Use these tools to manage software releases:
- relicta.status: Get current release state
- relicta.plan: Analyze commits and plan a release
- relicta.bump: Set the next version
- relicta.notes: Generate release notes
- relicta.evaluate: Evaluate risk using CGP
- relicta.approve: Approve the release
- relicta.publish: Execute the release

Resources provide read-only access to:
- relicta://state: Current release state
- relicta://config: Configuration settings
- relicta://commits: Recent commit history
- relicta://changelog: Generated changelog
- relicta://risk-report: CGP risk assessment`,
	}

	return NewResponse(req.ID, result)
}

func (s *Server) handleListTools(req *Request) *Response {
	tools := []Tool{
		{
			Name:        "relicta.status",
			Description: "Get the current release state and pending actions",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "relicta.plan",
			Description: "Analyze commits since the last release and suggest a version bump",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"from":    {Type: "string", Description: "Starting point for commit analysis (tag, commit SHA, or 'auto')", Default: "auto"},
					"analyze": {Type: "boolean", Description: "Include detailed commit analysis"},
				},
			},
		},
		{
			Name:        "relicta.bump",
			Description: "Calculate and set the next version based on commits",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"bump":    {Type: "string", Description: "Version bump type: major, minor, patch, or auto", Default: "auto"},
					"version": {Type: "string", Description: "Explicit version to set (overrides bump type)"},
				},
			},
		},
		{
			Name:        "relicta.notes",
			Description: "Generate changelog and release notes for the current release",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"ai": {Type: "boolean", Description: "Use AI to enhance release notes"},
				},
			},
		},
		{
			Name:        "relicta.evaluate",
			Description: "Evaluate release risk using the Change Governance Protocol (CGP)",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "relicta.approve",
			Description: "Approve the release for publishing",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"notes": {Type: "string", Description: "Updated release notes (optional)"},
				},
			},
		},
		{
			Name:        "relicta.publish",
			Description: "Execute the release by creating tags and running plugins",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"dry_run": {Type: "boolean", Description: "Simulate the release without making changes"},
				},
			},
		},
	}

	return NewResponse(req.ID, ListToolsResult{Tools: tools})
}

func (s *Server) handleCallTool(ctx context.Context, req *Request) *Response {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
	}

	handler, ok := s.tools[params.Name]
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, "Tool not found", params.Name)
	}

	// Inject progress tracking into context if a progress token was provided
	if params.Meta != nil && params.Meta.ProgressToken != "" && s.progressWriter != nil {
		ctx = ContextWithProgress(ctx, params.Meta.ProgressToken, s.progressWriter)
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternalError, "Tool execution failed", err.Error())
	}

	return NewResponse(req.ID, result)
}

func (s *Server) handleListResources(req *Request) *Response {
	resources := []Resource{
		{URI: "relicta://state", Name: "Release State", Description: "Current release state machine status", MIMEType: "application/json"},
		{URI: "relicta://config", Name: "Configuration", Description: "Current Relicta configuration", MIMEType: "application/json"},
		{URI: "relicta://commits", Name: "Commits", Description: "Recent commits since last release", MIMEType: "application/json"},
		{URI: "relicta://changelog", Name: "Changelog", Description: "Generated changelog for current release", MIMEType: "text/markdown"},
		{URI: "relicta://risk-report", Name: "Risk Report", Description: "CGP risk assessment for current release", MIMEType: "application/json"},
	}

	return NewResponse(req.ID, ListResourcesResult{Resources: resources})
}

func (s *Server) handleReadResource(ctx context.Context, req *Request) *Response {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
	}

	handler, ok := s.resources[params.URI]
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, "Resource not found", params.URI)
	}

	// Check cache first
	if s.cache != nil {
		if cached := s.cache.Get(params.URI); cached != nil {
			s.logger.Debug("cache hit", "uri", params.URI)
			return NewResponse(req.ID, cached)
		}
	}

	result, err := handler(ctx, params.URI)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternalError, "Resource read failed", err.Error())
	}

	// Cache the result
	if s.cache != nil {
		s.cache.Set(params.URI, result)
	}

	return NewResponse(req.ID, result)
}

func (s *Server) handleListPrompts(req *Request) *Response {
	prompts := []Prompt{
		{
			Name:        "release-summary",
			Description: "Generate a summary of the upcoming release",
			Arguments: []PromptArgument{
				{Name: "style", Description: "Summary style: brief, detailed, or technical"},
			},
		},
		{
			Name:        "risk-analysis",
			Description: "Analyze and explain the risk factors for the current release",
		},
		{
			Name:        "commit-review",
			Description: "Review commits for conventional commit compliance and quality",
			Arguments: []PromptArgument{
				{Name: "focus", Description: "Review focus: compliance, quality, or security"},
			},
		},
		{
			Name:        "breaking-changes",
			Description: "Document breaking changes and their impact on users",
		},
		{
			Name:        "migration-guide",
			Description: "Generate migration instructions for upgrading to this release",
			Arguments: []PromptArgument{
				{Name: "audience", Description: "Target audience: developer, operator, or end-user"},
			},
		},
		{
			Name:        "release-announcement",
			Description: "Generate a release announcement for publishing",
			Arguments: []PromptArgument{
				{Name: "channel", Description: "Target channel: github, blog, social, or email"},
			},
		},
		{
			Name:        "approval-decision",
			Description: "Help make an informed approval decision based on CGP analysis",
		},
	}

	return NewResponse(req.ID, ListPromptsResult{Prompts: prompts})
}

func (s *Server) handleGetPrompt(ctx context.Context, req *Request) *Response {
	var params GetPromptParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
	}

	handler, ok := s.prompts[params.Name]
	if !ok {
		return NewErrorResponse(req.ID, ErrCodeMethodNotFound, "Prompt not found", params.Name)
	}

	result, err := handler(ctx, params.Arguments)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternalError, "Prompt generation failed", err.Error())
	}

	return NewResponse(req.ID, result)
}

// registerTools registers all tool handlers.
func (s *Server) registerTools() {
	s.tools["relicta.status"] = s.toolStatus
	s.tools["relicta.plan"] = s.toolPlan
	s.tools["relicta.bump"] = s.toolBump
	s.tools["relicta.notes"] = s.toolNotes
	s.tools["relicta.evaluate"] = s.toolEvaluate
	s.tools["relicta.approve"] = s.toolApprove
	s.tools["relicta.publish"] = s.toolPublish
}

// registerResources registers all resource handlers.
func (s *Server) registerResources() {
	s.resources["relicta://state"] = s.resourceState
	s.resources["relicta://config"] = s.resourceConfig
	s.resources["relicta://commits"] = s.resourceCommits
	s.resources["relicta://changelog"] = s.resourceChangelog
	s.resources["relicta://risk-report"] = s.resourceRiskReport
}

// registerPrompts registers all prompt handlers.
func (s *Server) registerPrompts() {
	s.prompts["release-summary"] = s.promptReleaseSummary
	s.prompts["risk-analysis"] = s.promptRiskAnalysis
	s.prompts["commit-review"] = s.promptCommitReview
	s.prompts["breaking-changes"] = s.promptBreakingChanges
	s.prompts["migration-guide"] = s.promptMigrationGuide
	s.prompts["release-announcement"] = s.promptReleaseAnnouncement
	s.prompts["approval-decision"] = s.promptApprovalDecision
}

// invalidateCache invalidates state-dependent resources in the cache.
// Called after tools that modify release state.
func (s *Server) invalidateCache() {
	if s.cache != nil {
		s.cache.InvalidateStateDependent()
	}
}

// Tool implementations

func (s *Server) toolStatus(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	// Use adapter if available
	if s.adapter != nil && s.adapter.HasReleaseRepository() {
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return NewToolResult("No active release found. Run 'relicta plan' to start a new release."), nil
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

		return NewToolResultJSON(result)
	}

	// Fallback to direct repository access
	if s.releaseRepo == nil {
		return NewToolResult("No release repository configured. Run 'relicta plan' first."), nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return NewToolResult("No active release found. Run 'relicta plan' to start a new release."), nil
	}

	rel := releases[0]
	result := map[string]any{
		"state":   rel.State().String(),
		"version": "",
		"created": rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		"updated": rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}

	if rel.Version() != nil {
		result["version"] = rel.Version().String()
	}

	return NewToolResultJSON(result)
}

func (s *Server) toolPlan(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	// Use adapter if available
	if s.adapter != nil && s.adapter.HasPlanUseCase() {
		fromRef := ""
		if v, ok := args["from"].(string); ok && v != "auto" {
			fromRef = v
		}
		analyze := false
		if v, ok := args["analyze"].(bool); ok {
			analyze = v
		}

		// Get repository path from git service (required for release tracking)
		repoPath := ""
		if s.gitService != nil {
			if path, err := s.gitService.GetRepositoryRoot(ctx); err == nil {
				repoPath = path
			}
		}

		input := PlanInput{
			RepositoryPath: repoPath,
			FromRef:        fromRef,
			Analyze:        analyze,
		}

		// Send progress: starting plan
		_ = SendProgress(ctx, "Analyzing commit history...", 3)

		output, err := s.adapter.Plan(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Plan failed: %v", err)), nil
		}

		// Send progress: plan complete
		_ = SendProgress(ctx, "Computing version bump...", 3)

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
		if analyze && len(output.Commits) > 0 {
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

		// Send progress: complete
		_ = SendProgress(ctx, "Plan complete", 3)

		// Invalidate cache since plan modifies state
		s.invalidateCache()

		return NewToolResultJSON(result)
	}

	return NewToolResult("Plan tool called - run 'relicta mcp serve' with configured dependencies"), nil
}

func (s *Server) toolBump(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	bumpType := "auto"
	if v, ok := args["bump"].(string); ok {
		bumpType = v
	}
	version := ""
	if v, ok := args["version"].(string); ok {
		version = v
	}

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasCalculateVersionUseCase() {
		// Get repository path from git service (required for release state update)
		repoPath := ""
		if s.gitService != nil {
			if path, err := s.gitService.GetRepositoryRoot(ctx); err == nil {
				repoPath = path
			}
		}

		input := BumpInput{
			RepositoryPath: repoPath,
			BumpType:       bumpType,
			Version:        version,
		}

		output, err := s.adapter.Bump(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Bump failed: %v", err)), nil
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

		// Invalidate cache since bump modifies state
		s.invalidateCache()

		return NewToolResultJSON(result)
	}

	result := map[string]any{
		"bump_type": bumpType,
		"version":   version,
		"status":    "run 'relicta mcp serve' with configured dependencies",
	}

	return NewToolResultJSON(result)
}

func (s *Server) toolNotes(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	useAI := false
	if v, ok := args["ai"].(bool); ok {
		useAI = v
	}

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasGenerateNotesUseCase() && s.adapter.HasReleaseRepository() {
		// Get active release ID
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("No active release: %v", err)), nil
		}

		// Send progress: starting notes generation
		totalSteps := float64(3)
		if useAI {
			totalSteps = 5 // AI adds extra steps
		}
		_ = SendProgress(ctx, "Loading release context...", totalSteps)

		input := NotesInput{
			ReleaseID:        status.ReleaseID,
			UseAI:            useAI,
			IncludeChangelog: true,
		}

		if useAI {
			_ = SendProgress(ctx, "Generating AI-enhanced release notes...", totalSteps)
		} else {
			_ = SendProgress(ctx, "Generating changelog from commits...", totalSteps)
		}

		output, err := s.adapter.Notes(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Notes generation failed: %v", err)), nil
		}

		_ = SendProgress(ctx, "Formatting release notes...", totalSteps)

		result := map[string]any{
			"summary":      output.Summary,
			"ai_generated": output.AIGenerated,
		}

		if output.Changelog != "" {
			result["changelog"] = output.Changelog
		}

		_ = SendProgress(ctx, "Notes generation complete", totalSteps)

		// Invalidate cache since notes modifies changelog
		s.invalidateCache()

		return NewToolResultJSON(result)
	}

	result := map[string]any{
		"use_ai": useAI,
		"status": "run 'relicta mcp serve' with configured dependencies",
	}

	return NewToolResultJSON(result)
}

func (s *Server) toolEvaluate(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	// Use adapter for full governance evaluation if available
	if s.adapter != nil && s.adapter.HasGovernanceService() && s.adapter.HasReleaseRepository() {
		// Get active release ID
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("No active release: %v", err)), nil
		}

		// Send progress: starting evaluation
		_ = SendProgress(ctx, "Loading release data...", 4)

		input := EvaluateInput{
			ReleaseID:      status.ReleaseID,
			IncludeHistory: true,
		}

		_ = SendProgress(ctx, "Calculating risk factors...", 4)

		output, err := s.adapter.Evaluate(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Evaluation failed: %v", err)), nil
		}

		_ = SendProgress(ctx, "Applying governance policies...", 4)

		result := map[string]any{
			"decision":         output.Decision,
			"risk_score":       output.RiskScore,
			"severity":         output.Severity,
			"can_auto_approve": output.CanAutoApprove,
			"required_actions": output.RequiredActions,
			"risk_factors":     output.RiskFactors,
			"rationale":        output.Rationale,
		}

		_ = SendProgress(ctx, "Evaluation complete", 4)

		return NewToolResultJSON(result)
	}

	// Fallback to basic risk calculation
	if s.riskCalc == nil {
		return NewToolResultError("Risk calculator not configured"), nil
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
		return NewToolResultError(fmt.Sprintf("Failed to calculate risk: %v", err)), nil
	}

	result := map[string]any{
		"score":    assessment.Score,
		"severity": string(assessment.Severity),
		"summary":  assessment.Summary,
		"factors":  assessment.Factors,
	}

	return NewToolResultJSON(result)
}

func (s *Server) toolApprove(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	notes := ""
	if v, ok := args["notes"].(string); ok {
		notes = v
	}

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasApproveUseCase() && s.adapter.HasReleaseRepository() {
		// Get active release ID
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("No active release: %v", err)), nil
		}

		input := ApproveInput{
			ReleaseID:   status.ReleaseID,
			ApprovedBy:  "mcp-agent",
			AutoApprove: true,
			EditedNotes: notes,
		}

		output, err := s.adapter.Approve(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Approval failed: %v", err)), nil
		}

		result := map[string]any{
			"approved":    output.Approved,
			"approved_by": output.ApprovedBy,
			"version":     output.Version,
		}

		// Invalidate cache since approve modifies state
		s.invalidateCache()

		return NewToolResultJSON(result)
	}

	result := map[string]any{
		"notes":  notes,
		"status": "run 'relicta mcp serve' with configured dependencies",
	}

	return NewToolResultJSON(result)
}

func (s *Server) toolPublish(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	dryRun := true
	if v, ok := args["dry_run"].(bool); ok {
		dryRun = v
	}

	// Use adapter if available
	if s.adapter != nil && s.adapter.HasPublishUseCase() && s.adapter.HasReleaseRepository() {
		// Get active release ID
		status, err := s.adapter.GetStatus(ctx)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("No active release: %v", err)), nil
		}

		// Send progress: starting publish
		totalSteps := float64(5)
		_ = SendProgress(ctx, "Preparing release...", totalSteps)

		input := PublishInput{
			ReleaseID: status.ReleaseID,
			DryRun:    dryRun,
			CreateTag: true,
			PushTag:   !dryRun,
		}

		if dryRun {
			_ = SendProgress(ctx, "Validating release (dry run)...", totalSteps)
		} else {
			_ = SendProgress(ctx, "Creating git tag...", totalSteps)
		}

		output, err := s.adapter.Publish(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Publish failed: %v", err)), nil
		}

		_ = SendProgress(ctx, "Running plugins...", totalSteps)

		result := map[string]any{
			"tag_name":    output.TagName,
			"release_url": output.ReleaseURL,
			"dry_run":     dryRun,
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

		_ = SendProgress(ctx, "Publish complete", totalSteps)

		// Invalidate cache since publish modifies state
		s.invalidateCache()

		return NewToolResultJSON(result)
	}

	result := map[string]any{
		"dry_run": dryRun,
		"status":  "run 'relicta mcp serve' with configured dependencies",
	}

	return NewToolResultJSON(result)
}

// Resource implementations

func (s *Server) resourceState(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if s.releaseRepo == nil {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no release repository configured"}`),
			},
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no active release"}`),
			},
		}, nil
	}

	rel := releases[0]
	version := ""
	if rel.Version() != nil {
		version = rel.Version().String()
	}

	content := fmt.Sprintf(`{
  "state": %q,
  "version": %q,
  "created_at": %q,
  "updated_at": %q
}`, rel.State().String(), version, rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"), rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"))

	return &ReadResourceResult{
		Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: content}},
	}, nil
}

func (s *Server) resourceConfig(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if s.config == nil {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no configuration loaded"}`),
			},
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

	return &ReadResourceResult{
		Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: content}},
	}, nil
}

func (s *Server) resourceCommits(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if s.releaseRepo == nil {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no release repository configured"}`),
			},
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no active release", "commits": []}`),
			},
		}, nil
	}

	rel := releases[0]
	plan := rel.Plan()
	if plan == nil {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no plan available", "commits": []}`),
			},
		}, nil
	}

	// Check if changeset is loaded
	if !plan.HasChangeSet() {
		// Return plan metadata without commits
		content := fmt.Sprintf(`{
  "status": "changeset not loaded",
  "changeset_id": %q,
  "release_type": %q,
  "current_version": %q,
  "next_version": %q,
  "commits": []
}`, plan.ChangeSetID, plan.ReleaseType, plan.CurrentVersion.String(), plan.NextVersion.String())
		return &ReadResourceResult{
			Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: content}},
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
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, fmt.Sprintf(`{"status": "error", "error": %q}`, err.Error())),
			},
		}, nil
	}

	return &ReadResourceResult{
		Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)}},
	}, nil
}

func (s *Server) resourceChangelog(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if s.releaseRepo == nil {
		return &ReadResourceResult{
			Contents: []ResourceContent{{URI: uri, MIMEType: "text/markdown", Text: "# Changelog\n\nNo release repository configured."}},
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &ReadResourceResult{
			Contents: []ResourceContent{{URI: uri, MIMEType: "text/markdown", Text: "# Changelog\n\nNo active release found. Run `relicta plan` to start a new release."}},
		}, nil
	}

	rel := releases[0]
	notes := rel.Notes()

	if notes == nil {
		// No notes generated yet - provide helpful message
		version := ""
		if rel.Version() != nil {
			version = rel.Version().String()
		} else if rel.Plan() != nil {
			version = rel.Plan().NextVersion.String()
		}

		content := fmt.Sprintf("# Changelog\n\nNo changelog generated yet for version %s.\n\nRun `relicta notes` to generate release notes.", version)
		return &ReadResourceResult{
			Contents: []ResourceContent{{URI: uri, MIMEType: "text/markdown", Text: content}},
		}, nil
	}

	// Return the actual changelog
	changelog := notes.Changelog
	if changelog == "" {
		// Fall back to summary if changelog is empty
		changelog = fmt.Sprintf("# Release Notes\n\n%s", notes.Summary)
	}

	return &ReadResourceResult{
		Contents: []ResourceContent{{URI: uri, MIMEType: "text/markdown", Text: changelog}},
	}, nil
}

func (s *Server) resourceRiskReport(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if s.releaseRepo == nil {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no release repository configured"}`),
			},
		}, nil
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return &ReadResourceResult{
			Contents: []ResourceContent{
				NewTextResourceContent(uri, `{"status": "no active release"}`),
			},
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
				return &ReadResourceResult{
					Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)}},
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
				Repository:  rel.RepositoryName(),
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
				return &ReadResourceResult{
					Contents: []ResourceContent{{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)}},
				}, nil
			}
		}
	}

	return &ReadResourceResult{
		Contents: []ResourceContent{
			NewTextResourceContent(uri, `{"status": "no risk assessment available", "hint": "Run 'relicta evaluate' to perform risk assessment"}`),
		},
	}, nil
}

// Prompt implementations

func (s *Server) promptReleaseSummary(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
	style := "brief"
	if s, ok := args["style"]; ok {
		style = s
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

	return &GetPromptResult{
		Description: "Release summary prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}

func (s *Server) promptRiskAnalysis(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
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

	return &GetPromptResult{
		Description: "Risk analysis prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}

func (s *Server) promptCommitReview(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
	focus := "compliance"
	if f, ok := args["focus"]; ok {
		focus = f
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

	return &GetPromptResult{
		Description: "Commit review prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}

func (s *Server) promptBreakingChanges(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
	content := `You are a technical writer documenting breaking changes for users.

For each breaking change in this release, provide:

1. **Change Summary**: One-line description of what changed
2. **Reason**: Why this breaking change was necessary
3. **Impact**: Who is affected and how
4. **Migration Path**: Step-by-step instructions to adapt
5. **Code Examples**: Before/after code snippets where applicable

Format the output as a structured breaking changes document suitable for inclusion in release notes.

If there are no breaking changes, confirm this and explain what safeguards prevented them.`

	return &GetPromptResult{
		Description: "Breaking changes documentation prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}

func (s *Server) promptMigrationGuide(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
	audience := "developer"
	if a, ok := args["audience"]; ok {
		audience = a
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

	return &GetPromptResult{
		Description: "Migration guide prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}

func (s *Server) promptReleaseAnnouncement(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
	channel := "github"
	if c, ok := args["channel"]; ok {
		channel = c
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

	return &GetPromptResult{
		Description: "Release announcement prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}

func (s *Server) promptApprovalDecision(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
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

	return &GetPromptResult{
		Description: "Approval decision prompt",
		Messages:    []PromptMessage{NewPromptMessage(content)},
	}, nil
}
