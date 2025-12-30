// Package mcp provides MCP server implementation for Relicta.
package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/cgp"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

// Adapter bridges MCP server to application use cases.
type Adapter struct {
	releaseAnalyzer *servicerelease.Analyzer
	calculateUC     *versioning.CalculateVersionUseCase
	setVersionUC    *versioning.SetVersionUseCase
	releaseServices *domainrelease.Services
	governanceSvc   *governance.Service
	releaseRepo     domainrelease.Repository
}

// AdapterOption configures the Adapter.
type AdapterOption func(*Adapter)

// NewAdapter creates a new MCP adapter.
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

// WithCalculateVersionUseCase sets the calculate version use case.
func WithCalculateVersionUseCase(uc *versioning.CalculateVersionUseCase) AdapterOption {
	return func(a *Adapter) {
		a.calculateUC = uc
	}
}

// WithSetVersionUseCase sets the set version use case.
func WithSetVersionUseCase(uc *versioning.SetVersionUseCase) AdapterOption {
	return func(a *Adapter) {
		a.setVersionUC = uc
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

// WithAdapterReleaseRepository sets the release repository.
func WithAdapterReleaseRepository(repo domainrelease.Repository) AdapterOption {
	return func(a *Adapter) {
		a.releaseRepo = repo
	}
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
func (a *Adapter) Plan(ctx context.Context, input PlanInput) (*PlanOutput, error) {
	if a.releaseAnalyzer == nil {
		return nil, fmt.Errorf("release analyzer not configured")
	}

	analyzeInput := servicerelease.AnalyzeInput{
		RepositoryPath: input.RepositoryPath,
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

	return result, nil
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
// This mirrors the CLI's runReleaseBump pattern exactly:
// 1. Calculate version using CalculateVersionUseCase
// 2. Set version using SetVersionUseCase (handles git tag)
// 3. Update release aggregate state using updateReleaseVersion
func (a *Adapter) Bump(ctx context.Context, input BumpInput) (*BumpOutput, error) {
	// Parse and validate bump type first (fail-fast on invalid input)
	var bumpType version.BumpType
	auto := false
	switch input.BumpType {
	case "major":
		bumpType = version.BumpMajor
	case "minor":
		bumpType = version.BumpMinor
	case "patch":
		bumpType = version.BumpPatch
	case "auto", "":
		auto = true
	default:
		return nil, fmt.Errorf("invalid bump type: %s", input.BumpType)
	}

	if a.calculateUC == nil {
		return nil, fmt.Errorf("calculate version use case not configured")
	}

	// Step 1: Calculate version (same as CLI)
	calcInput := versioning.CalculateVersionInput{
		RepositoryPath: input.RepositoryPath,
		BumpType:       bumpType,
		Prerelease:     version.Prerelease(input.Prerelease),
		Auto:           auto,
	}

	calcOutput, err := a.calculateUC.Execute(ctx, calcInput)
	if err != nil {
		return nil, fmt.Errorf("calculate version failed: %w", err)
	}

	result := &BumpOutput{
		CurrentVersion: calcOutput.CurrentVersion.String(),
		NextVersion:    calcOutput.NextVersion.String(),
		BumpType:       string(calcOutput.BumpType),
		AutoDetected:   calcOutput.AutoDetected,
	}

	// For dry run, return early without making changes
	if input.DryRun {
		result.TagName = "v" + calcOutput.NextVersion.String()
		return result, nil
	}

	// Step 2: Set version using SetVersionUseCase (same as CLI's SetVersion().Execute())
	// This handles git tag creation - mirrors runReleaseBump pattern
	if a.setVersionUC != nil {
		setInput := versioning.SetVersionInput{
			Version:    calcOutput.NextVersion,
			TagPrefix:  "v",
			CreateTag:  input.CreateTag,
			TagMessage: fmt.Sprintf("Release %s", calcOutput.NextVersion.String()),
			DryRun:     input.DryRun,
		}

		setOutput, err := a.setVersionUC.Execute(ctx, setInput)
		if err != nil {
			return nil, fmt.Errorf("set version failed: %w", err)
		}

		result.TagName = setOutput.TagName
		result.TagCreated = setOutput.TagCreated
	} else {
		result.TagName = "v" + calcOutput.NextVersion.String()
	}

	// Step 3: Update release aggregate state (same as CLI's updateReleaseVersion)
	// This transitions the release from StatePlanned to StateVersioned
	if a.releaseRepo != nil {
		if err := a.updateReleaseVersion(ctx, input.RepositoryPath, calcOutput.NextVersion); err != nil {
			return nil, fmt.Errorf("failed to update release state: %w", err)
		}
	}

	return result, nil
}

// updateReleaseVersion updates the latest release with the bumped version.
// This is the same logic as CLI's updateReleaseVersion function in bump.go:357-376.
// It transitions the release aggregate from StatePlanned to StateVersioned.
func (a *Adapter) updateReleaseVersion(ctx context.Context, repoPath string, ver version.SemanticVersion) error {
	// Use FindLatest like CLI does, not FindActive
	rel, err := a.releaseRepo.FindLatest(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to find latest release: %w", err)
	}

	tagName := "v" + ver.String()

	// Same as rel.SetVersion() call in CLI's updateReleaseVersion
	if err := rel.SetVersion(ver, tagName); err != nil {
		return fmt.Errorf("failed to set version on release: %w", err)
	}

	// Same as releaseRepo.Save() call in CLI's updateReleaseVersion
	if err := a.releaseRepo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	return nil
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
func (a *Adapter) Notes(ctx context.Context, input NotesInput) (*NotesOutput, error) {
	if a.releaseServices == nil {
		return nil, fmt.Errorf("release services not configured")
	}

	// Use DDD services for notes generation
	// For now, return a stub since the DDD layer handles this through the state machine
	return &NotesOutput{
		Summary:     "Release notes generated via MCP",
		AIGenerated: input.UseAI,
	}, nil
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
func (a *Adapter) Approve(ctx context.Context, input ApproveInput) (*ApproveOutput, error) {
	if a.releaseRepo == nil {
		return nil, fmt.Errorf("release repository not configured")
	}

	// Find and approve the release directly
	rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	approver := input.ApprovedBy
	if approver == "" {
		approver = "mcp-agent"
	}

	if err := rel.Approve(approver, true); err != nil { // auto-approved via MCP
		return nil, fmt.Errorf("approve failed: %w", err)
	}

	if err := a.releaseRepo.Save(ctx, rel); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
	}

	return &ApproveOutput{
		Approved:   true,
		ApprovedBy: approver,
		Version:    rel.VersionNext().String(),
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
func (a *Adapter) Publish(ctx context.Context, input PublishInput) (*PublishOutput, error) {
	if a.releaseRepo == nil {
		return nil, fmt.Errorf("release repository not configured")
	}

	// Find the release
	rel, err := a.releaseRepo.FindByID(ctx, domainrelease.RunID(input.ReleaseID))
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	// Start publishing
	if err := rel.StartPublishing("mcp-agent"); err != nil {
		return nil, fmt.Errorf("failed to start publishing: %w", err)
	}

	// Mark as published
	if err := rel.MarkPublished("mcp-agent"); err != nil {
		return nil, fmt.Errorf("failed to mark as published: %w", err)
	}

	if err := a.releaseRepo.Save(ctx, rel); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
	}

	return &PublishOutput{
		TagName: rel.TagName(),
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
func (a *Adapter) GetStatus(ctx context.Context) (*GetStatusOutput, error) {
	if a.releaseRepo == nil {
		return nil, fmt.Errorf("release repository not configured")
	}

	releases, err := a.releaseRepo.FindActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find active releases: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no active release found")
	}

	rel := releases[0]
	result := &GetStatusOutput{
		ReleaseID: string(rel.ID()),
		State:     rel.State().String(),
		CreatedAt: rel.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: rel.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}

	// Get version from aggregate
	if !rel.VersionNext().IsZero() {
		result.Version = rel.VersionNext().String()
	}

	status := rel.ApprovalStatus()
	result.CanApprove = status.CanApprove
	result.ApprovalMsg = status.Reason

	// Set next action based on current state
	result.NextAction = nextActionForState(rel.State().String())

	// Check for stale release (not updated in over 1 hour and not in terminal state)
	stateStr := rel.State().String()
	isTerminal := stateStr == "published" || stateStr == "canceled"
	if !isTerminal {
		staleThreshold := time.Now().Add(-1 * time.Hour)
		if rel.UpdatedAt().Before(staleThreshold) {
			result.Stale = true
			result.Warning = "Release was last updated over 1 hour ago. Consider running 'relicta plan' to refresh state."
		}
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

// HasCalculateVersionUseCase returns true if the calculate version use case is configured.
func (a *Adapter) HasCalculateVersionUseCase() bool {
	return a.calculateUC != nil
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

// Deprecated: Use HasReleaseAnalyzer instead. Kept for backwards compatibility.
func (a *Adapter) HasPlanUseCase() bool {
	return a.releaseAnalyzer != nil
}

// Deprecated: Use HasReleaseServices instead. Kept for backwards compatibility.
func (a *Adapter) HasGenerateNotesUseCase() bool {
	return a.releaseServices != nil
}

// Deprecated: Use HasReleaseRepository instead. Kept for backwards compatibility.
func (a *Adapter) HasApproveUseCase() bool {
	return a.releaseRepo != nil
}

// Deprecated: Use HasReleaseRepository instead. Kept for backwards compatibility.
func (a *Adapter) HasPublishUseCase() bool {
	return a.releaseRepo != nil
}
