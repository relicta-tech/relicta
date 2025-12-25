// Package mcp provides MCP server implementation for Relicta.
package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/cgp"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// Adapter bridges MCP server to application use cases.
type Adapter struct {
	planUC           *release.PlanReleaseUseCase
	calculateUC      *versioning.CalculateVersionUseCase
	setVersionUC     *versioning.SetVersionUseCase
	notesUC          *release.GenerateNotesUseCase
	approveUC        *release.ApproveReleaseUseCase
	getForApprovalUC *release.GetReleaseForApprovalUseCase
	publishUC        *release.PublishReleaseUseCase
	governanceSvc    *governance.Service
	releaseRepo      domainrelease.Repository
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

// WithPlanUseCase sets the plan release use case.
func WithPlanUseCase(uc *release.PlanReleaseUseCase) AdapterOption {
	return func(a *Adapter) {
		a.planUC = uc
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

// WithGenerateNotesUseCase sets the generate notes use case.
func WithGenerateNotesUseCase(uc *release.GenerateNotesUseCase) AdapterOption {
	return func(a *Adapter) {
		a.notesUC = uc
	}
}

// WithApproveUseCase sets the approve release use case.
func WithApproveUseCase(uc *release.ApproveReleaseUseCase) AdapterOption {
	return func(a *Adapter) {
		a.approveUC = uc
	}
}

// WithGetForApprovalUseCase sets the get release for approval use case.
func WithGetForApprovalUseCase(uc *release.GetReleaseForApprovalUseCase) AdapterOption {
	return func(a *Adapter) {
		a.getForApprovalUC = uc
	}
}

// WithPublishUseCase sets the publish release use case.
func WithPublishUseCase(uc *release.PublishReleaseUseCase) AdapterOption {
	return func(a *Adapter) {
		a.publishUC = uc
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
	if a.planUC == nil {
		return nil, fmt.Errorf("plan use case not configured")
	}

	ucInput := release.PlanReleaseInput{
		RepositoryPath: input.RepositoryPath,
		FromRef:        input.FromRef,
		ToRef:          input.ToRef,
		DryRun:         input.DryRun,
	}

	output, err := a.planUC.Execute(ctx, ucInput)
	if err != nil {
		return nil, fmt.Errorf("plan failed: %w", err)
	}

	result := &PlanOutput{
		ReleaseID:      string(output.ReleaseID),
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

	// Create tag if requested and set version UC is available
	if input.CreateTag && a.setVersionUC != nil && !input.DryRun {
		setInput := versioning.SetVersionInput{
			Version:   calcOutput.NextVersion,
			CreateTag: true,
			DryRun:    input.DryRun,
		}

		setOutput, err := a.setVersionUC.Execute(ctx, setInput)
		if err != nil {
			return nil, fmt.Errorf("set version failed: %w", err)
		}

		result.TagName = setOutput.TagName
		result.TagCreated = setOutput.TagCreated
	}

	return result, nil
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
	if a.notesUC == nil {
		return nil, fmt.Errorf("generate notes use case not configured")
	}

	ucInput := release.GenerateNotesInput{
		ReleaseID:        domainrelease.ReleaseID(input.ReleaseID),
		UseAI:            input.UseAI,
		IncludeChangelog: input.IncludeChangelog,
		RepositoryURL:    input.RepositoryURL,
	}

	output, err := a.notesUC.Execute(ctx, ucInput)
	if err != nil {
		return nil, fmt.Errorf("generate notes failed: %w", err)
	}

	result := &NotesOutput{
		Summary:     output.ReleaseNotes.Summary(),
		AIGenerated: output.ReleaseNotes.IsAIGenerated(),
	}

	if output.Changelog != nil {
		result.Changelog = output.Changelog.Render()
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
	rel, err := a.releaseRepo.FindByID(ctx, domainrelease.ReleaseID(input.ReleaseID))
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
	if a.approveUC == nil {
		return nil, fmt.Errorf("approve use case not configured")
	}

	ucInput := release.ApproveReleaseInput{
		ReleaseID:   domainrelease.ReleaseID(input.ReleaseID),
		ApprovedBy:  input.ApprovedBy,
		AutoApprove: input.AutoApprove,
	}

	if input.EditedNotes != "" {
		ucInput.EditedNotes = &input.EditedNotes
	}

	output, err := a.approveUC.Execute(ctx, ucInput)
	if err != nil {
		return nil, fmt.Errorf("approve failed: %w", err)
	}

	result := &ApproveOutput{
		Approved:   output.Approved,
		ApprovedBy: output.ApprovedBy,
	}

	if output.ReleasePlan != nil {
		result.Version = output.ReleasePlan.NextVersion.String()
	}

	return result, nil
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
	if a.publishUC == nil {
		return nil, fmt.Errorf("publish use case not configured")
	}

	ucInput := release.PublishReleaseInput{
		ReleaseID: domainrelease.ReleaseID(input.ReleaseID),
		DryRun:    input.DryRun,
		CreateTag: input.CreateTag,
		PushTag:   input.PushTag,
		TagPrefix: input.TagPrefix,
		Remote:    input.Remote,
	}

	output, err := a.publishUC.Execute(ctx, ucInput)
	if err != nil {
		return nil, fmt.Errorf("publish failed: %w", err)
	}

	result := &PublishOutput{
		TagName:    output.TagName,
		ReleaseURL: output.ReleaseURL,
	}

	for _, pr := range output.PluginResults {
		result.PluginResults = append(result.PluginResults, PluginResultInfo{
			PluginName: pr.PluginName,
			Hook:       string(pr.Hook),
			Success:    pr.Success,
			Message:    pr.Message,
		})
	}

	return result, nil
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

	// Get version - prefer direct version, fall back to plan's next version
	if rel.Version() != nil {
		result.Version = rel.Version().String()
	} else if rel.Plan() != nil {
		result.Version = rel.Plan().NextVersion.String()
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

// HasPlanUseCase returns true if the plan use case is configured.
func (a *Adapter) HasPlanUseCase() bool {
	return a.planUC != nil
}

// HasCalculateVersionUseCase returns true if the calculate version use case is configured.
func (a *Adapter) HasCalculateVersionUseCase() bool {
	return a.calculateUC != nil
}

// HasGenerateNotesUseCase returns true if the generate notes use case is configured.
func (a *Adapter) HasGenerateNotesUseCase() bool {
	return a.notesUC != nil
}

// HasApproveUseCase returns true if the approve use case is configured.
func (a *Adapter) HasApproveUseCase() bool {
	return a.approveUC != nil
}

// HasPublishUseCase returns true if the publish use case is configured.
func (a *Adapter) HasPublishUseCase() bool {
	return a.publishUC != nil
}

// HasGovernanceService returns true if the governance service is configured.
func (a *Adapter) HasGovernanceService() bool {
	return a.governanceSvc != nil
}

// HasReleaseRepository returns true if the release repository is configured.
func (a *Adapter) HasReleaseRepository() bool {
	return a.releaseRepo != nil
}
