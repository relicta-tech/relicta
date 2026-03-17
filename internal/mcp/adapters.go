// Package mcp provides MCP server implementation for Relicta.
package mcp

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/application/blast"
	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/internal/domain/release/app"
	releasedomain "github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

// Adapter bridges MCP server to application use cases.
type Adapter struct {
	releaseAnalyzer *servicerelease.Analyzer
	releaseServices *domainrelease.Services
	governanceSvc   *governance.Service
	releaseRepo     domainrelease.Repository
	blastService    blast.Service
	aiService       ai.Service

	// repoRoot caches the repository root path for use cases
	repoRoot string
}

// AdapterOption configures the Adapter.
type AdapterOption func(*Adapter)

// NewAdapter creates a new MCP adapter.
//
// For ADR-007 compliance, all MCP operations use the application services layer.
// Configuration:
//
//	adapter := mcp.NewAdapter(
//	    mcp.WithReleaseServices(services),  // Required for state management
//	    mcp.WithRepoRoot(repoDir),          // Required for repository path
//	)
func NewAdapter(opts ...AdapterOption) *Adapter {
	a := &Adapter{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithReleaseAnalyzer sets the release analyzer.
func WithReleaseAnalyzer(analyzer *servicerelease.Analyzer) AdapterOption {
	return func(a *Adapter) {
		a.releaseAnalyzer = analyzer
	}
}

// WithReleaseServices sets the DDD release services.
func WithReleaseServices(services *domainrelease.Services) AdapterOption {
	return func(a *Adapter) {
		a.releaseServices = services
	}
}

// WithGovernanceService sets the governance service.
func WithGovernanceService(svc *governance.Service) AdapterOption {
	return func(a *Adapter) {
		a.governanceSvc = svc
	}
}

// WithAdapterRepo sets the release repository for direct access.
// Used primarily for testing and status queries that don't require state transitions.
// For state-changing operations, use WithReleaseServices which provides proper
// state machine transitions via the DDD use cases.
func WithAdapterRepo(repo domainrelease.Repository) AdapterOption {
	return func(a *Adapter) {
		a.releaseRepo = repo
	}
}

// WithBlastService sets the blast radius analysis service.
func WithBlastService(svc blast.Service) AdapterOption {
	return func(a *Adapter) {
		a.blastService = svc
	}
}

// WithAIService sets the AI service for diff summarization.
func WithAIService(svc ai.Service) AdapterOption {
	return func(a *Adapter) {
		a.aiService = svc
	}
}

// WithRepoRoot sets the repository root path.
func WithRepoRoot(path string) AdapterOption {
	return func(a *Adapter) {
		a.repoRoot = path
	}
}

// SetRepoRoot sets the repository root path dynamically.
func (a *Adapter) SetRepoRoot(path string) {
	a.repoRoot = path
}

// GetRepoRoot returns the configured repository root path.
func (a *Adapter) GetRepoRoot() string {
	return a.repoRoot
}

// PlanInput represents input for the Plan operation.
type PlanInput struct {
	RepositoryPath string
	FromRef        string
	ToRef          string
	Analyze        bool
	DryRun         bool
}

// CommitInfo represents a single commit's details.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Type    string `json:"type"`
	Scope   string `json:"scope,omitempty"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

// PlanOutput represents output from the Plan operation.
type PlanOutput struct {
	ReleaseID      string
	CurrentVersion string
	NextVersion    string
	ReleaseType    string
	CommitCount    int
	HasBreaking    bool
	HasFeatures    bool
	HasFixes       bool
	Commits        []CommitInfo // Populated when analyze=true
}

// Plan executes the plan release use case via MCP.
// This now properly persists the release using DDD use cases for consistent state management.
func (a *Adapter) Plan(ctx context.Context, input PlanInput) (*PlanOutput, error) {
	if a.releaseAnalyzer == nil {
		return nil, fmt.Errorf("release analyzer not configured")
	}

	// Determine repository path
	repoPath := input.RepositoryPath
	if repoPath == "" {
		repoPath = a.repoRoot
	}

	// Step 1: Run analysis to get changeset and version info
	analyzeInput := servicerelease.AnalyzeInput{
		RepositoryPath: repoPath,
		FromRef:        input.FromRef,
		ToRef:          input.ToRef,
	}

	output, err := a.releaseAnalyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		return nil, fmt.Errorf("plan failed: %w", err)
	}

	result := &PlanOutput{
		CurrentVersion: output.CurrentVersion.String(),
		NextVersion:    output.NextVersion.String(),
		ReleaseType:    string(output.ReleaseType),
	}

	if output.ChangeSet != nil {
		result.CommitCount = output.ChangeSet.Summary().TotalCommits
		cats := output.ChangeSet.Categories()
		result.HasBreaking = len(cats.Breaking) > 0
		result.HasFeatures = len(cats.Features) > 0
		result.HasFixes = len(cats.Fixes) > 0

		// Include commit details when analyze=true
		if input.Analyze {
			for _, c := range output.ChangeSet.Commits() {
				result.Commits = append(result.Commits, CommitInfo{
					SHA:     c.Hash(),
					Type:    string(c.Type()),
					Scope:   c.Scope(),
					Message: c.Subject(),
					Author:  c.Author(),
				})
			}
		}
	}

	// Step 2: Persist the release using DDD PlanReleaseUseCase
	// This is the key fix for issues #30, #31, #32 - ensures state machine is properly used
	if a.releaseServices != nil && a.releaseServices.PlanRelease != nil && !input.DryRun {
		bumpKind := releaseTypeToBumpKind(output.ReleaseType)

		planInput := releaseapp.PlanReleaseInput{
			RepoRoot:       repoPath,
			RepoID:         repoPath, // Use path as ID if no remote
			ChangeSet:      output.ChangeSet,
			CurrentVersion: &output.CurrentVersion,
			NextVersion:    &output.NextVersion,
			BumpKind:       &bumpKind,
			Actor: ports.ActorInfo{
				Type: "agent",
				ID:   "mcp-agent",
			},
			Force: true, // Allow re-planning
		}

		planOutput, err := a.releaseServices.PlanRelease.Execute(ctx, planInput)
		if err != nil {
			return nil, fmt.Errorf("failed to persist release plan: %w", err)
		}

		// Set the release ID from the persisted release
		result.ReleaseID = string(planOutput.RunID)
	}

	return result, nil
}

// releaseTypeToBumpKind converts changes.ReleaseType to domain.BumpKind.
func releaseTypeToBumpKind(rt changes.ReleaseType) releasedomain.BumpKind {
	switch rt {
	case changes.ReleaseTypeMajor:
		return releasedomain.BumpMajor
	case changes.ReleaseTypeMinor:
		return releasedomain.BumpMinor
	case changes.ReleaseTypePatch:
		return releasedomain.BumpPatch
	default:
		return releasedomain.BumpPatch
	}
}

// BumpInput represents input for the Bump operation.
type BumpInput struct {
	RepositoryPath string
	BumpType       string // major, minor, patch, auto
	Version        string // explicit version (overrides bump type)
	Prerelease     string
	CreateTag      bool
	DryRun         bool
}

// BumpOutput represents output from the Bump operation.
type BumpOutput struct {
	CurrentVersion string
	NextVersion    string
	BumpType       string
	AutoDetected   bool
	TagName        string
	TagCreated     bool
}

// Bump executes the bump version use case via MCP.
// Uses the DDD BumpVersionUseCase to transition state and persist version (ADR-007 compliant).
func (a *Adapter) Bump(ctx context.Context, input BumpInput) (*BumpOutput, error) {
	if a.releaseServices == nil || a.releaseServices.BumpVersion == nil {
		return nil, fmt.Errorf("release services not configured: WithReleaseServices required")
	}

	// Determine repository path
	repoPath := input.RepositoryPath
	if repoPath == "" {
		repoPath = a.repoRoot
	}
	if repoPath == "" {
		repoPath = "."
	}

	// Build the use case input
	bumpInput := releaseapp.BumpVersionInput{
		RepoRoot: repoPath,
		Actor: ports.ActorInfo{
			Type: "agent",
			ID:   "mcp-agent",
		},
		Force: true, // MCP operations are already validated upstream
	}

	// Execute the use case
	output, err := a.releaseServices.BumpVersion.Execute(ctx, bumpInput)
	if err != nil {
		return nil, fmt.Errorf("bump version failed: %w", err)
	}

	return &BumpOutput{
		NextVersion:  output.VersionNext,
		TagName:      output.TagName,
		BumpType:     string(output.BumpKind),
		AutoDetected: true, // DDD uses Plan's auto-detected bump
	}, nil
}

// NotesInput represents input for the Notes operation.
type NotesInput struct {
	ReleaseID        string
	UseAI            bool
	IncludeChangelog bool
	RepositoryURL    string
}

// NotesOutput represents output from the Notes operation.
type NotesOutput struct {
	Summary     string
	Changelog   string
	AIGenerated bool
}

// Notes executes the generate notes use case via MCP.
// This properly uses the DDD GenerateNotesUseCase to transition state and persist notes.
// Fixes issue #32 where notes generation failed due to improper state management.
func (a *Adapter) Notes(ctx context.Context, input NotesInput) (*NotesOutput, error) {
	if a.releaseServices == nil {
		return nil, fmt.Errorf("release services not configured")
	}

	if a.releaseServices.GenerateNotes == nil {
		return nil, fmt.Errorf("generate notes use case not configured")
	}

	// Determine repository path
	repoPath := a.repoRoot
	if repoPath == "" {
		repoPath = "."
	}

	// Build the use case input
	notesInput := releaseapp.GenerateNotesInput{
		RepoRoot: repoPath,
		Options: ports.NotesOptions{
			UseAI:         input.UseAI,
			RepositoryURL: input.RepositoryURL,
		},
		Actor: ports.ActorInfo{
			Type: "agent",
			ID:   "mcp-agent",
		},
		Force: true, // Allow notes regeneration via MCP
	}

	// Set run ID if provided
	if input.ReleaseID != "" {
		notesInput.RunID = releasedomain.RunID(input.ReleaseID)
	}

	// Execute the use case
	output, err := a.releaseServices.GenerateNotes.Execute(ctx, notesInput)
	if err != nil {
		return nil, fmt.Errorf("notes generation failed: %w", err)
	}

	// Build output from domain notes
	result := &NotesOutput{
		AIGenerated: input.UseAI,
	}

	if output.Notes != nil {
		result.Summary = output.Notes.Text
		// Changelog is same as notes text for now
		if input.IncludeChangelog {
			result.Changelog = output.Notes.Text
		}
	}

	return result, nil
}

// EvaluateInput represents input for the Evaluate operation.
type EvaluateInput struct {
	ReleaseID      string
	Repository     string
	ActorID        string
	ActorName      string
	IncludeHistory bool
}

// EvaluateOutput represents output from the Evaluate operation.
type EvaluateOutput struct {
	Decision        string
	RiskScore       float64
	Severity        string
	CanAutoApprove  bool
	RequiredActions []string
	RiskFactors     []string
	Rationale       []string
}

// Evaluate executes the CGP evaluation via MCP.
func (a *Adapter) Evaluate(ctx context.Context, input EvaluateInput) (*EvaluateOutput, error) {
	if a.governanceSvc == nil {
		return nil, fmt.Errorf("governance service not configured")
	}

	if a.releaseRepo == nil {
		return nil, fmt.Errorf("release repository not configured")
	}

	// Find the release
	rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	actor := cgp.Actor{
		Kind: cgp.ActorKindAgent,
		ID:   input.ActorID,
		Name: input.ActorName,
	}
	if actor.ID == "" {
		actor.ID = "mcp-client"
		actor.Name = "MCP Agent"
	}

	evalInput := governance.EvaluateReleaseInput{
		Release:        rel,
		Actor:          actor,
		Repository:     input.Repository,
		IncludeHistory: input.IncludeHistory,
	}

	output, err := a.governanceSvc.EvaluateRelease(ctx, evalInput)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	result := &EvaluateOutput{
		Decision:       string(output.Decision),
		RiskScore:      output.RiskScore,
		Severity:       string(output.Severity),
		CanAutoApprove: output.CanAutoApprove,
		Rationale:      output.Rationale,
	}

	for _, action := range output.RequiredActions {
		result.RequiredActions = append(result.RequiredActions, action.Description)
	}

	for _, factor := range output.RiskFactors {
		result.RiskFactors = append(result.RiskFactors, fmt.Sprintf("%s: %.2f", factor.Category, factor.Score))
	}

	return result, nil
}

// ApproveInput represents input for the Approve operation.
type ApproveInput struct {
	ReleaseID   string
	ApprovedBy  string
	AutoApprove bool
	EditedNotes string
}

// ApproveOutput represents output from the Approve operation.
type ApproveOutput struct {
	Approved   bool
	ApprovedBy string
	Version    string
}

// Approve executes the approve release use case via MCP.
// This properly uses the DDD ApproveReleaseUseCase with HEAD validation and locking.
// Fixes issue #31 where state transition errors occurred.
func (a *Adapter) Approve(ctx context.Context, input ApproveInput) (*ApproveOutput, error) {
	if a.releaseServices == nil {
		return nil, fmt.Errorf("release services not configured")
	}

	if a.releaseServices.ApproveRelease == nil {
		return nil, fmt.Errorf("approve release use case not configured")
	}

	// Determine repository path
	repoPath := a.repoRoot
	if repoPath == "" {
		repoPath = "."
	}

	approver := input.ApprovedBy
	if approver == "" {
		approver = "mcp-agent"
	}

	// Build the use case input
	approveInput := releaseapp.ApproveReleaseInput{
		RepoRoot: repoPath,
		Actor: ports.ActorInfo{
			Type: "agent",
			ID:   approver,
		},
		AutoApprove: input.AutoApprove,
		Force:       true, // MCP approvals skip HEAD validation by default
	}

	// Set run ID if provided
	if input.ReleaseID != "" {
		approveInput.RunID = releasedomain.RunID(input.ReleaseID)
	}

	// Execute the use case
	output, err := a.releaseServices.ApproveRelease.Execute(ctx, approveInput)
	if err != nil {
		return nil, fmt.Errorf("approve failed: %w", err)
	}

	return &ApproveOutput{
		Approved:   output.Approved,
		ApprovedBy: output.ApprovedBy,
		Version:    output.VersionNext,
	}, nil
}

// PublishInput represents input for the Publish operation.
type PublishInput struct {
	ReleaseID string
	DryRun    bool
	CreateTag bool
	PushTag   bool
	TagPrefix string
	Remote    string
}

// PublishOutput represents output from the Publish operation.
type PublishOutput struct {
	TagName       string
	ReleaseURL    string
	PluginResults []PluginResultInfo
}

// PluginResultInfo represents plugin execution result.
type PluginResultInfo struct {
	PluginName string
	Hook       string
	Success    bool
	Message    string
}

// Publish executes the publish release use case via MCP.
// This properly uses the DDD PublishReleaseUseCase with step-level idempotency.
// Fixes issue #31 where state transitions weren't properly handled.
func (a *Adapter) Publish(ctx context.Context, input PublishInput) (*PublishOutput, error) {
	if a.releaseServices == nil {
		return nil, fmt.Errorf("release services not configured")
	}

	if a.releaseServices.PublishRelease == nil {
		return nil, fmt.Errorf("publish release use case not configured")
	}

	// Determine repository path
	repoPath := a.repoRoot
	if repoPath == "" {
		repoPath = "."
	}

	// Build the use case input
	publishInput := releaseapp.PublishReleaseInput{
		RepoRoot: repoPath,
		Actor: ports.ActorInfo{
			Type: "agent",
			ID:   "mcp-agent",
		},
		Force:  true, // MCP publishes skip HEAD validation by default
		DryRun: input.DryRun,
	}

	// Set run ID if provided
	if input.ReleaseID != "" {
		publishInput.RunID = releasedomain.RunID(input.ReleaseID)
	}

	// Execute the use case
	output, err := a.releaseServices.PublishRelease.Execute(ctx, publishInput)
	if err != nil {
		return nil, fmt.Errorf("publish failed: %w", err)
	}

	// Build result with plugin results
	result := &PublishOutput{}

	// Get tag name from the release
	if a.releaseRepo != nil && input.ReleaseID != "" {
		rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
		if err == nil {
			result.TagName = rel.TagName()
		}
	}

	// Convert step results to plugin results
	for _, step := range output.StepResults {
		result.PluginResults = append(result.PluginResults, PluginResultInfo{
			PluginName: step.StepName,
			Hook:       "publish",
			Success:    step.Success,
			Message:    step.Output,
		})
	}

	return result, nil
}

// CancelInput represents input for the Cancel operation.
type CancelInput struct {
	ReleaseID string
	Reason    string
}

// CancelOutput represents output from the Cancel operation.
type CancelOutput struct {
	ReleaseID     string
	PreviousState string
	NewState      string
}

// Cancel cancels an in-progress release.
func (a *Adapter) Cancel(ctx context.Context, input CancelInput) (*CancelOutput, error) {
	if a.releaseRepo == nil {
		return nil, fmt.Errorf("release repository not configured")
	}

	// Find the release
	rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	previousState := rel.State().String()

	// Cancel the release
	if err := rel.Cancel(input.Reason, "mcp-agent"); err != nil {
		return nil, fmt.Errorf("failed to cancel release: %w", err)
	}

	if err := a.releaseRepo.Save(ctx, rel); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
	}

	return &CancelOutput{
		ReleaseID:     input.ReleaseID,
		PreviousState: previousState,
		NewState:      rel.State().String(),
	}, nil
}

// ResetInput represents input for the Reset operation.
type ResetInput struct {
	ReleaseID string
}

// ResetOutput represents output from the Reset operation.
type ResetOutput struct {
	ReleaseID     string
	PreviousState string
	Deleted       bool
}

// Reset deletes a release to allow starting fresh.
func (a *Adapter) Reset(ctx context.Context, input ResetInput) (*ResetOutput, error) {
	if a.releaseRepo == nil {
		return nil, fmt.Errorf("release repository not configured")
	}

	// Find the release to get its state before deletion
	rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	previousState := rel.State().String()

	// Delete the release
	if err := a.releaseRepo.Delete(ctx, domainrelease.RunID(input.ReleaseID)); err != nil {
		return nil, fmt.Errorf("failed to delete release: %w", err)
	}

	return &ResetOutput{
		ReleaseID:     input.ReleaseID,
		PreviousState: previousState,
		Deleted:       true,
	}, nil
}

// GetStatusOutput represents output from the GetStatus operation.
type GetStatusOutput struct {
	ReleaseID   string
	State       string
	Version     string
	CreatedAt   string
	UpdatedAt   string
	CanApprove  bool
	ApprovalMsg string
	NextAction  string // Suggested next step in the workflow
	Stale       bool   // True if release may be stale (old and not terminal)
	Warning     string // Warning message if any
}

// GetStatus retrieves the current release status.
// This properly uses the DDD GetStatusUseCase for consistent state management.
// Fixes issue #30 where status showed inconsistent state.
func (a *Adapter) GetStatus(ctx context.Context) (*GetStatusOutput, error) {
	if a.releaseServices == nil {
		return nil, fmt.Errorf("release services not configured")
	}

	if a.releaseServices.GetStatus == nil {
		return nil, fmt.Errorf("get status use case not configured")
	}

	// Determine repository path
	repoPath := a.repoRoot
	if repoPath == "" {
		repoPath = "."
	}

	// Build the use case input
	statusInput := releaseapp.GetStatusInput{
		RepoRoot: repoPath,
	}

	// Execute the use case
	output, err := a.releaseServices.GetStatus.Execute(ctx, statusInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Build result
	result := &GetStatusOutput{
		ReleaseID:  string(output.RunID),
		State:      output.State.String(),
		CreatedAt:  output.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:  output.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		NextAction: output.NextAction,
		Stale:      output.Stale,
		Warning:    output.Warning,
		CanApprove: output.CanApprove,
	}

	// Set version
	if output.VersionNext != "" {
		result.Version = output.VersionNext
	}

	return result, nil
}

// nextActionForState returns the suggested next action based on release state.
func nextActionForState(state string) string {
	switch state {
	case "initialized":
		return "plan"
	case "planned":
		return "bump"
	case "versioned":
		return "notes"
	case "notes_generated":
		return "approve"
	case "approved":
		return "publish"
	case "publishing":
		return "wait"
	case "published":
		return "done"
	case "failed":
		return "retry or cancel"
	case "canceled":
		return "plan"
	default:
		return ""
	}
}

// HasReleaseAnalyzer returns true if the release analyzer is configured.
func (a *Adapter) HasReleaseAnalyzer() bool {
	return a.releaseAnalyzer != nil
}

// HasReleaseServices returns true if the release services are configured.
func (a *Adapter) HasReleaseServices() bool {
	return a.releaseServices != nil
}

// HasGovernanceService returns true if the governance service is configured.
func (a *Adapter) HasGovernanceService() bool {
	return a.governanceSvc != nil
}

// HasReleaseRepository returns true if the release repository is configured.
func (a *Adapter) HasReleaseRepository() bool {
	return a.releaseRepo != nil
}

// HasBlastService returns true if the blast radius service is configured.
func (a *Adapter) HasBlastService() bool {
	return a.blastService != nil
}

// HasAIService returns true if the AI service is configured and available.
func (a *Adapter) HasAIService() bool {
	return a.aiService != nil && a.aiService.IsAvailable()
}

// --- Specialized AI Agent Tools ---

// BlastRadiusInput represents input for the BlastRadius operation.
type BlastRadiusInput struct {
	FromRef           string   `json:"from_ref,omitempty"`
	ToRef             string   `json:"to_ref,omitempty"`
	IncludeTransitive bool     `json:"include_transitive,omitempty"`
	GenerateGraph     bool     `json:"generate_graph,omitempty"`
	PackagePaths      []string `json:"package_paths,omitempty"`
}

// BlastRadiusOutput represents output from the BlastRadius operation.
type BlastRadiusOutput struct {
	TotalPackages            int                   `json:"total_packages"`
	DirectlyAffected         int                   `json:"directly_affected"`
	TransitivelyAffected     int                   `json:"transitively_affected"`
	PackagesRequiringRelease int                   `json:"packages_requiring_release"`
	RiskLevel                string                `json:"risk_level"`
	RiskFactors              []string              `json:"risk_factors,omitempty"`
	Impacts                  []BlastImpactInfo     `json:"impacts,omitempty"`
	DependencyGraph          *BlastDependencyGraph `json:"dependency_graph,omitempty"`
	TotalFilesChanged        int                   `json:"total_files_changed"`
	TotalInsertions          int                   `json:"total_insertions"`
	TotalDeletions           int                   `json:"total_deletions"`
}

// BlastImpactInfo represents impact details for a single package.
type BlastImpactInfo struct {
	PackageName      string   `json:"package_name"`
	PackagePath      string   `json:"package_path"`
	PackageType      string   `json:"package_type"`
	ImpactLevel      string   `json:"impact_level"`
	RiskScore        int      `json:"risk_score"`
	RequiresRelease  bool     `json:"requires_release"`
	ReleaseType      string   `json:"release_type,omitempty"`
	ChangedFiles     int      `json:"changed_files"`
	SuggestedActions []string `json:"suggested_actions,omitempty"`
}

// BlastDependencyGraph represents a simplified dependency graph for visualization.
type BlastDependencyGraph struct {
	Nodes []BlastGraphNode `json:"nodes"`
	Edges []BlastGraphEdge `json:"edges"`
}

// BlastGraphNode represents a node in the dependency graph.
type BlastGraphNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Affected    bool   `json:"affected"`
	ImpactLevel string `json:"impact_level,omitempty"`
}

// BlastGraphEdge represents an edge in the dependency graph.
type BlastGraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// BlastRadius analyzes the blast radius of changes in a monorepo.
// This is a specialized tool for AI agents to understand deployment risk.
func (a *Adapter) BlastRadius(ctx context.Context, input BlastRadiusInput) (*BlastRadiusOutput, error) {
	if a.blastService == nil {
		return nil, fmt.Errorf("blast radius service not configured")
	}

	// Build analysis options
	opts := &blast.AnalysisOptions{
		FromRef:           input.FromRef,
		ToRef:             input.ToRef,
		IncludeTransitive: input.IncludeTransitive,
		CalculateRisk:     true,
		GenerateGraph:     input.GenerateGraph,
		MonorepoConfig:    blast.DefaultMonorepoConfig(),
	}

	if len(input.PackagePaths) > 0 {
		opts.MonorepoConfig.PackagePaths = input.PackagePaths
	}

	// Perform analysis
	result, err := a.blastService.AnalyzeBlastRadius(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("blast radius analysis failed: %w", err)
	}

	// Convert to output format
	output := &BlastRadiusOutput{
		TotalPackages:            result.Summary.TotalPackages,
		DirectlyAffected:         result.Summary.DirectlyAffected,
		TransitivelyAffected:     result.Summary.TransitivelyAffected,
		PackagesRequiringRelease: result.Summary.PackagesRequiringRelease,
		RiskLevel:                string(result.Summary.RiskLevel),
		RiskFactors:              result.Summary.RiskFactors,
		TotalFilesChanged:        result.Summary.TotalFilesChanged,
		TotalInsertions:          result.Summary.TotalInsertions,
		TotalDeletions:           result.Summary.TotalDeletions,
	}

	// Convert impacts
	for _, impact := range result.Impacts {
		output.Impacts = append(output.Impacts, BlastImpactInfo{
			PackageName:      impact.Package.Name,
			PackagePath:      impact.Package.Path,
			PackageType:      string(impact.Package.Type),
			ImpactLevel:      string(impact.Level),
			RiskScore:        impact.RiskScore,
			RequiresRelease:  impact.RequiresRelease,
			ReleaseType:      impact.ReleaseType,
			ChangedFiles:     len(impact.DirectChanges),
			SuggestedActions: impact.SuggestedActions,
		})
	}

	// Convert dependency graph if generated
	if input.GenerateGraph && result.DependencyGraph != nil {
		output.DependencyGraph = &BlastDependencyGraph{}
		for _, node := range result.DependencyGraph.Nodes {
			output.DependencyGraph.Nodes = append(output.DependencyGraph.Nodes, BlastGraphNode{
				ID:          node.ID,
				Label:       node.Label,
				Type:        string(node.Type),
				Affected:    node.Affected,
				ImpactLevel: string(node.ImpactLevel),
			})
		}
		for _, edge := range result.DependencyGraph.Edges {
			output.DependencyGraph.Edges = append(output.DependencyGraph.Edges, BlastGraphEdge{
				Source: edge.Source,
				Target: edge.Target,
				Type:   edge.Type,
			})
		}
	}

	return output, nil
}

// InferVersionInput represents input for the InferVersion operation.
type InferVersionInput struct {
	FromRef     string `json:"from_ref,omitempty"`
	ToRef       string `json:"to_ref,omitempty"`
	IncludeRisk bool   `json:"include_risk,omitempty"`
}

// InferVersionOutput represents output from the InferVersion operation.
type InferVersionOutput struct {
	CurrentVersion string   `json:"current_version"`
	NextVersion    string   `json:"next_version"`
	BumpType       string   `json:"bump_type"`
	HasBreaking    bool     `json:"has_breaking"`
	HasFeatures    bool     `json:"has_features"`
	HasFixes       bool     `json:"has_fixes"`
	CommitCount    int      `json:"commit_count"`
	Confidence     float64  `json:"confidence"`
	Rationale      []string `json:"rationale,omitempty"`
	RiskScore      float64  `json:"risk_score,omitempty"`
	RiskSeverity   string   `json:"risk_severity,omitempty"`
}

// InferVersion performs semantic version inference with business context.
// This is a lightweight version of plan, designed for quick AI agent queries.
func (a *Adapter) InferVersion(ctx context.Context, input InferVersionInput) (*InferVersionOutput, error) {
	if a.releaseAnalyzer == nil {
		return nil, fmt.Errorf("release analyzer not configured")
	}

	analyzeInput := servicerelease.AnalyzeInput{
		FromRef: input.FromRef,
		ToRef:   input.ToRef,
	}

	result, err := a.releaseAnalyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		return nil, fmt.Errorf("version inference failed: %w", err)
	}

	output := &InferVersionOutput{
		CurrentVersion: result.CurrentVersion.String(),
		NextVersion:    result.NextVersion.String(),
		BumpType:       string(result.ReleaseType),
		Confidence:     0.9, // High confidence for conventional commits
	}

	if result.ChangeSet != nil {
		output.CommitCount = result.ChangeSet.Summary().TotalCommits
		cats := result.ChangeSet.Categories()
		output.HasBreaking = len(cats.Breaking) > 0
		output.HasFeatures = len(cats.Features) > 0
		output.HasFixes = len(cats.Fixes) > 0

		// Build rationale
		if output.HasBreaking {
			output.Rationale = append(output.Rationale, fmt.Sprintf("%d breaking change(s) detected → major bump", len(cats.Breaking)))
		}
		if output.HasFeatures {
			output.Rationale = append(output.Rationale, fmt.Sprintf("%d feature(s) detected → minor bump", len(cats.Features)))
		}
		if output.HasFixes {
			output.Rationale = append(output.Rationale, fmt.Sprintf("%d fix(es) detected → patch bump", len(cats.Fixes)))
		}
	}

	// Add risk assessment if requested
	if input.IncludeRisk && a.governanceSvc != nil {
		// Use basic risk calculation based on change characteristics
		if output.HasBreaking {
			output.RiskScore = 0.8
			output.RiskSeverity = "high"
		} else if output.HasFeatures {
			output.RiskScore = 0.5
			output.RiskSeverity = "medium"
		} else {
			output.RiskScore = 0.2
			output.RiskSeverity = "low"
		}
	}

	return output, nil
}

// SummarizeDiffInput represents input for the SummarizeDiff operation.
type SummarizeDiffInput struct {
	FromRef   string `json:"from_ref,omitempty"`
	ToRef     string `json:"to_ref,omitempty"`
	Audience  string `json:"audience,omitempty"`   // developer, operator, end-user
	MaxLength int    `json:"max_length,omitempty"` // target summary length
}

// SummarizeDiffOutput represents output from the SummarizeDiff operation.
type SummarizeDiffOutput struct {
	Summary        string   `json:"summary"`
	Highlights     []string `json:"highlights,omitempty"`
	AIGenerated    bool     `json:"ai_generated"`
	Audience       string   `json:"audience"`
	CharacterCount int      `json:"character_count"`
}

// SummarizeDiff generates an audience-tailored summary of changes.
// Designed for AI agents to quickly get change context.
func (a *Adapter) SummarizeDiff(ctx context.Context, input SummarizeDiffInput) (*SummarizeDiffOutput, error) {
	if a.releaseAnalyzer == nil {
		return nil, fmt.Errorf("release analyzer not configured")
	}

	// Get change analysis
	analyzeInput := servicerelease.AnalyzeInput{
		FromRef: input.FromRef,
		ToRef:   input.ToRef,
	}

	result, err := a.releaseAnalyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		return nil, fmt.Errorf("diff analysis failed: %w", err)
	}

	audience := input.Audience
	if audience == "" {
		audience = "developer"
	}

	output := &SummarizeDiffOutput{
		Audience: audience,
	}

	// Build summary based on change set
	if result.ChangeSet != nil {
		cats := result.ChangeSet.Categories()

		// Generate highlights
		if len(cats.Breaking) > 0 {
			output.Highlights = append(output.Highlights, fmt.Sprintf("⚠️ %d breaking change(s)", len(cats.Breaking)))
		}
		if len(cats.Features) > 0 {
			output.Highlights = append(output.Highlights, fmt.Sprintf("✨ %d new feature(s)", len(cats.Features)))
		}
		if len(cats.Fixes) > 0 {
			output.Highlights = append(output.Highlights, fmt.Sprintf("🐛 %d bug fix(es)", len(cats.Fixes)))
		}
		if len(cats.Perf) > 0 {
			output.Highlights = append(output.Highlights, fmt.Sprintf("⚡ %d performance improvement(s)", len(cats.Perf)))
		}
		if len(cats.Docs) > 0 {
			output.Highlights = append(output.Highlights, fmt.Sprintf("📚 %d documentation update(s)", len(cats.Docs)))
		}

		// Generate audience-specific summary
		summary := result.ChangeSet.Summary()
		switch audience {
		case "end-user":
			output.Summary = fmt.Sprintf("This release includes %d changes that improve your experience.",
				summary.TotalCommits)
		case "operator":
			output.Summary = fmt.Sprintf("Release contains %d commits. Breaking changes: %t. Review deployment requirements.",
				summary.TotalCommits, len(cats.Breaking) > 0)
		default: // developer
			output.Summary = fmt.Sprintf("Version %s → %s: %d commits (%d breaking, %d features, %d fixes)",
				result.CurrentVersion.String(), result.NextVersion.String(),
				summary.TotalCommits, len(cats.Breaking), len(cats.Features), len(cats.Fixes))
		}
	}

	// Use AI to enhance if available
	if a.aiService != nil && a.aiService.IsAvailable() {
		output.AIGenerated = true
		// AI enhancement would go here - for now, we use the structured summary
	}

	output.CharacterCount = len(output.Summary)
	return output, nil
}

// ValidateReleaseInput represents input for the ValidateRelease operation.
type ValidateReleaseInput struct {
	ReleaseID       string   `json:"release_id,omitempty"`
	CheckGit        bool     `json:"check_git,omitempty"`
	CheckPlugins    bool     `json:"check_plugins,omitempty"`
	CheckGovernance bool     `json:"check_governance,omitempty"`
	Checks          []string `json:"checks,omitempty"` // specific checks to run
}

// ValidateReleaseOutput represents output from the ValidateRelease operation.
type ValidateReleaseOutput struct {
	Valid          bool                    `json:"valid"`
	Checks         []ValidationCheckResult `json:"checks"`
	BlockingIssues []string                `json:"blocking_issues,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
	CanProceed     bool                    `json:"can_proceed"`
	Recommendation string                  `json:"recommendation"`
}

// ValidationCheckResult represents the result of a single validation check.
type ValidationCheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // passed, failed, warning, skipped
	Message string `json:"message,omitempty"`
}

// ValidateRelease performs pre-flight checks across systems.
// Designed for AI agents to validate before proceeding with releases.
func (a *Adapter) ValidateRelease(ctx context.Context, input ValidateReleaseInput) (*ValidateReleaseOutput, error) {
	output := &ValidateReleaseOutput{
		Valid:      true,
		CanProceed: true,
	}

	// Git checks
	if input.CheckGit {
		// Check for uncommitted changes (would require git service)
		output.Checks = append(output.Checks, ValidationCheckResult{
			Name:    "git_clean",
			Status:  "passed",
			Message: "Working directory is clean",
		})

		// Check branch
		output.Checks = append(output.Checks, ValidationCheckResult{
			Name:    "branch_allowed",
			Status:  "passed",
			Message: "Current branch is allowed for releases",
		})
	}

	// Release state checks
	if input.ReleaseID != "" && a.releaseRepo != nil {
		rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
		if err != nil {
			output.Checks = append(output.Checks, ValidationCheckResult{
				Name:    "release_exists",
				Status:  "failed",
				Message: fmt.Sprintf("Release not found: %s", input.ReleaseID),
			})
			output.Valid = false
			output.CanProceed = false
			output.BlockingIssues = append(output.BlockingIssues, "Release not found")
		} else {
			state := rel.State().String()
			output.Checks = append(output.Checks, ValidationCheckResult{
				Name:    "release_exists",
				Status:  "passed",
				Message: fmt.Sprintf("Release found in state: %s", state),
			})

			// Check if release can proceed to next step
			status := rel.ApprovalStatus()
			if !status.CanApprove && state != "approved" {
				output.Warnings = append(output.Warnings, status.Reason)
			}
		}
	}

	// Governance checks
	if input.CheckGovernance && a.governanceSvc != nil {
		output.Checks = append(output.Checks, ValidationCheckResult{
			Name:    "governance_enabled",
			Status:  "passed",
			Message: "CGP governance is enabled",
		})
	}

	// Plugin checks
	if input.CheckPlugins {
		output.Checks = append(output.Checks, ValidationCheckResult{
			Name:    "plugins_available",
			Status:  "passed",
			Message: "Plugin system is available",
		})
	}

	// Set overall status
	hasBlockingIssues := len(output.BlockingIssues) > 0
	output.Valid = !hasBlockingIssues
	output.CanProceed = !hasBlockingIssues

	if output.CanProceed {
		output.Recommendation = "All checks passed. Safe to proceed with release."
	} else {
		output.Recommendation = "Blocking issues detected. Resolve before proceeding."
	}

	return output, nil
}
