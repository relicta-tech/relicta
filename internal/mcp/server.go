package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

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

// NewServer creates a new MCP server for Relicta.
func NewServer(version string, opts ...ServerOption) (*Server, error) {
	s := &Server{
		version:   version,
		logger:    slog.Default(),
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		prompts:   make(map[string]PromptHandler),
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

	result, err := handler(ctx, params.URI)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternalError, "Resource read failed", err.Error())
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

		input := PlanInput{
			FromRef: fromRef,
			Analyze: analyze,
		}

		output, err := s.adapter.Plan(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Plan failed: %v", err)), nil
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
		input := BumpInput{
			BumpType: bumpType,
			Version:  version,
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

		input := NotesInput{
			ReleaseID:        status.ReleaseID,
			UseAI:            useAI,
			IncludeChangelog: true,
		}

		output, err := s.adapter.Notes(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Notes generation failed: %v", err)), nil
		}

		result := map[string]any{
			"summary":      output.Summary,
			"ai_generated": output.AIGenerated,
		}

		if output.Changelog != "" {
			result["changelog"] = output.Changelog
		}

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

		input := EvaluateInput{
			ReleaseID:      status.ReleaseID,
			IncludeHistory: true,
		}

		output, err := s.adapter.Evaluate(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Evaluation failed: %v", err)), nil
		}

		result := map[string]any{
			"decision":         output.Decision,
			"risk_score":       output.RiskScore,
			"severity":         output.Severity,
			"can_auto_approve": output.CanAutoApprove,
			"required_actions": output.RequiredActions,
			"risk_factors":     output.RiskFactors,
			"rationale":        output.Rationale,
		}

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

		input := PublishInput{
			ReleaseID: status.ReleaseID,
			DryRun:    dryRun,
			CreateTag: true,
			PushTag:   !dryRun,
		}

		output, err := s.adapter.Publish(ctx, input)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Publish failed: %v", err)), nil
		}

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
	return &ReadResourceResult{
		Contents: []ResourceContent{
			NewTextResourceContent(uri, `{"status": "commit history pending implementation"}`),
		},
	}, nil
}

func (s *Server) resourceChangelog(ctx context.Context, uri string) (*ReadResourceResult, error) {
	return &ReadResourceResult{
		Contents: []ResourceContent{{URI: uri, MIMEType: "text/markdown", Text: "# Changelog\n\nNo changelog generated yet."}},
	}, nil
}

func (s *Server) resourceRiskReport(ctx context.Context, uri string) (*ReadResourceResult, error) {
	return &ReadResourceResult{
		Contents: []ResourceContent{
			NewTextResourceContent(uri, `{"status": "no risk assessment performed"}`),
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
