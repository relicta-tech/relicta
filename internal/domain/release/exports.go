// Package release provides the release governance bounded context.
// This file re-exports domain types for clean imports.
package release

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// ReleasePlan holds the planned release information.
// It captures version transitions and the associated changeset.
type ReleasePlan struct {
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	ReleaseType    changes.ReleaseType
	ChangeSetID    changes.ChangeSetID
	changeSet      *changes.ChangeSet
	DryRun         bool
}

// NewReleasePlan creates a new release plan.
func NewReleasePlan(
	currentVersion, nextVersion version.SemanticVersion,
	releaseType changes.ReleaseType,
	changeSet *changes.ChangeSet,
	dryRun bool,
) *ReleasePlan {
	var changeSetID changes.ChangeSetID
	if changeSet != nil {
		changeSetID = changeSet.ID()
	}
	return &ReleasePlan{
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		ReleaseType:    releaseType,
		ChangeSetID:    changeSetID,
		changeSet:      changeSet,
		DryRun:         dryRun,
	}
}

// GetChangeSet returns the cached changeset if available.
func (p *ReleasePlan) GetChangeSet() *changes.ChangeSet {
	return p.changeSet
}

// SetChangeSet sets the cached changeset.
func (p *ReleasePlan) SetChangeSet(cs *changes.ChangeSet) {
	p.changeSet = cs
}

// HasChangeSet returns true if a changeset is cached.
func (p *ReleasePlan) HasChangeSet() bool {
	return p.changeSet != nil
}

// CommitCount returns the number of commits in the changeset.
func (p *ReleasePlan) CommitCount() int {
	if p.changeSet == nil {
		return 0
	}
	return p.changeSet.CommitCount()
}

// SetPlan sets the release plan on a ReleaseRun.
// It extracts version info from the ReleasePlan and stores the changeset in the aggregate.
func SetPlan(r *ReleaseRun, plan *ReleasePlan) error {
	if plan == nil {
		return domain.ErrVersionNotSet
	}

	bumpKind := domain.BumpKindFromReleaseType(plan.ReleaseType)

	// Set version proposal
	if err := r.SetVersionProposal(
		plan.CurrentVersion,
		plan.NextVersion,
		bumpKind,
		1.0, // confidence
	); err != nil {
		return err
	}

	// Transition to planned state
	if err := r.Plan("system"); err != nil {
		return err
	}

	// Store the changeset in the aggregate
	if plan.changeSet != nil {
		r.SetChangeSet(plan.changeSet)
	}

	return nil
}

// GetPlan extracts a ReleasePlan from a ReleaseRun.
// It reconstructs the plan from the aggregate's stored data.
func GetPlan(r *ReleaseRun) *ReleasePlan {
	if r == nil {
		return nil
	}

	// Reconstruct plan from aggregate data
	plan := &ReleasePlan{
		CurrentVersion: r.VersionCurrent(),
		NextVersion:    r.VersionNext(),
		ReleaseType:    changes.ReleaseType(r.BumpKind()),
		DryRun:         false,
	}

	// Include changeset if available
	if r.HasChangeSet() {
		plan.changeSet = r.ChangeSet()
		plan.ChangeSetID = r.ChangeSet().ID()
	}

	return plan
}

// Re-export core aggregate and value objects
type (
	// ReleaseRun is the aggregate root for release governance.
	ReleaseRun = domain.ReleaseRun

	// RunID uniquely identifies a release run.
	RunID = domain.RunID

	// RunSnapshot contains all data needed to reconstruct a ReleaseRun.
	RunSnapshot = domain.RunSnapshot

	// RunState represents the current state of a release run.
	RunState = domain.RunState

	// CommitSHA represents a git commit hash.
	CommitSHA = domain.CommitSHA

	// ActorType represents who or what initiated the release.
	ActorType = domain.ActorType

	// BumpKind represents the type of version bump.
	BumpKind = domain.BumpKind

	// PolicyThresholds captures the policy thresholds at plan time.
	PolicyThresholds = domain.PolicyThresholds

	// Approval holds release approval information.
	Approval = domain.Approval

	// ApprovalLevel represents the type/level of approval required.
	ApprovalLevel = domain.ApprovalLevel

	// ApprovalRequirement defines a single approval requirement.
	ApprovalRequirement = domain.ApprovalRequirement

	// ApprovalPolicy defines the multi-level approval requirements.
	ApprovalPolicy = domain.ApprovalPolicy

	// MultiLevelApproval tracks multiple approvals for a release.
	MultiLevelApproval = domain.MultiLevelApproval

	// ReleaseNotes holds the generated release notes.
	ReleaseNotes = domain.ReleaseNotes

	// StepType represents the type of publishing step.
	StepType = domain.StepType

	// StepPlan describes a single step in the publishing execution plan.
	StepPlan = domain.StepPlan

	// StepState represents the execution state of a step.
	StepState = domain.StepState

	// StepStatus tracks the execution status of a step.
	StepStatus = domain.StepStatus

	// TransitionRecord records a state transition for audit.
	TransitionRecord = domain.TransitionRecord

	// RunSummary is a summary of the release run.
	RunSummary = domain.RunSummary

	// Invariant provides information about aggregate invariant validation.
	Invariant = domain.Invariant

	// ApprovalStatus represents the approval readiness of a release.
	ApprovalStatus = domain.ApprovalStatus
)

// Re-export domain events
type (
	DomainEvent               = domain.DomainEvent
	RunCreatedEvent           = domain.RunCreatedEvent
	StateTransitionedEvent    = domain.StateTransitionedEvent
	RunPlannedEvent           = domain.RunPlannedEvent
	RunVersionedEvent         = domain.RunVersionedEvent
	RunNotesGeneratedEvent    = domain.RunNotesGeneratedEvent
	RunNotesUpdatedEvent      = domain.RunNotesUpdatedEvent
	RunApprovedEvent          = domain.RunApprovedEvent
	RunPublishingStartedEvent = domain.RunPublishingStartedEvent
	RunPublishedEvent         = domain.RunPublishedEvent
	RunFailedEvent            = domain.RunFailedEvent
	RunCanceledEvent          = domain.RunCanceledEvent
	RunRetriedEvent           = domain.RunRetriedEvent
	StepCompletedEvent        = domain.StepCompletedEvent
	PluginExecutedEvent       = domain.PluginExecutedEvent
)

// Re-export specifications
type (
	Specification = domain.Specification
)

// Re-export errors
var (
	ErrInvalidState        = domain.ErrInvalidState
	ErrHeadSHAChanged      = domain.ErrHeadSHAChanged
	ErrAlreadyPublished    = domain.ErrAlreadyPublished
	ErrNotApproved         = domain.ErrNotApproved
	ErrStepNotFound        = domain.ErrStepNotFound
	ErrStepAlreadyDone     = domain.ErrStepAlreadyDone
	ErrNilNotes            = domain.ErrNilNotes
	ErrRunNotFound         = domain.ErrRunNotFound
	ErrPlanHashMismatch    = domain.ErrPlanHashMismatch
	ErrApprovalBoundToHash = domain.ErrApprovalBoundToHash
	ErrNoChanges           = domain.ErrNoChanges
	ErrCannotCancel        = domain.ErrCannotCancel
	ErrCannotRetry         = domain.ErrCannotRetry
	ErrVersionNotSet       = domain.ErrVersionNotSet
	ErrRiskTooHigh         = domain.ErrRiskTooHigh
	ErrDuplicateRun        = domain.ErrDuplicateRun
)

// State constants
const (
	StateDraft      = domain.StateDraft
	StatePlanned    = domain.StatePlanned
	StateVersioned  = domain.StateVersioned
	StateNotesReady = domain.StateNotesReady
	StateApproved   = domain.StateApproved
	StatePublishing = domain.StatePublishing
	StatePublished  = domain.StatePublished
	StateFailed     = domain.StateFailed
	StateCanceled   = domain.StateCanceled
)

// Actor type constants
const (
	ActorHuman = domain.ActorHuman
	ActorCI    = domain.ActorCI
	ActorAgent = domain.ActorAgent
)

// Bump kind constants
const (
	BumpMajor      = domain.BumpMajor
	BumpMinor      = domain.BumpMinor
	BumpPatch      = domain.BumpPatch
	BumpPrerelease = domain.BumpPrerelease
	BumpNone       = domain.BumpNone
)

// Step type constants
const (
	StepTypeTag       = domain.StepTypeTag
	StepTypeBuild     = domain.StepTypeBuild
	StepTypeArtifact  = domain.StepTypeArtifact
	StepTypeNotify    = domain.StepTypeNotify
	StepTypeFinalize  = domain.StepTypeFinalize
	StepTypePlugin    = domain.StepTypePlugin
	StepTypeChangelog = domain.StepTypeChangelog
)

// Step state constants
const (
	StepPending = domain.StepPending
	StepRunning = domain.StepRunning
	StepDone    = domain.StepDone
	StepFailed  = domain.StepFailed
	StepSkipped = domain.StepSkipped
)

// Approval level constants
const (
	ApprovalLevelTechnical = domain.ApprovalLevelTechnical
	ApprovalLevelSecurity  = domain.ApprovalLevelSecurity
	ApprovalLevelManager   = domain.ApprovalLevelManager
	ApprovalLevelRelease   = domain.ApprovalLevelRelease
	ApprovalLevelAuto      = domain.ApprovalLevelAuto
)

// Constructor functions
var (
	NewReleaseRun           = domain.NewReleaseRun
	BuildIdempotencyKey     = domain.BuildIdempotencyKey
	ParseBumpKind           = domain.ParseBumpKind
	BumpKindFromReleaseType = domain.BumpKindFromReleaseType
	AllStates               = domain.AllStates
	ParseRunState           = domain.ParseRunState
	NewStateTransitionError = domain.NewStateTransitionError

	// Approval policy helpers
	DefaultApprovalPolicy  = domain.DefaultApprovalPolicy
	HighRiskApprovalPolicy = domain.HighRiskApprovalPolicy
)

// Specification constructors
var (
	ByState            = domain.ByState
	Active             = domain.Active
	Final              = domain.Final
	ByRepositoryPath   = domain.ByRepositoryPath
	ByRepoID           = domain.ByRepoID
	ReadyForPublish    = domain.ReadyForPublish
	HasNotes           = domain.HasNotes
	IsApproved         = domain.IsApproved
	HeadSHAMatches     = domain.HeadSHAMatches
	CanBump            = domain.CanBump
	CanGenerateNotes   = domain.CanGenerateNotes
	CanApprove         = domain.CanApprove
	RiskBelowThreshold = domain.RiskBelowThreshold
	CanAutoApprove     = domain.CanAutoApprove
	AllStepsSucceeded  = domain.AllStepsSucceeded
	HasFailedSteps     = domain.HasFailedSteps
	And                = domain.And
	Or                 = domain.Or
	Not                = domain.Not
)

// Repository defines the interface for persisting and retrieving releases.
type Repository interface {
	// Save persists a release run.
	Save(ctx context.Context, run *ReleaseRun) error

	// FindByID retrieves a release run by its ID.
	FindByID(ctx context.Context, id RunID) (*ReleaseRun, error)

	// FindLatest retrieves the latest release run for a repository.
	FindLatest(ctx context.Context, repoPath string) (*ReleaseRun, error)

	// FindByState retrieves release runs in a specific state.
	FindByState(ctx context.Context, state RunState) ([]*ReleaseRun, error)

	// FindActive retrieves all active (non-final) release runs.
	FindActive(ctx context.Context) ([]*ReleaseRun, error)

	// FindBySpecification retrieves release runs matching the given specification.
	FindBySpecification(ctx context.Context, spec Specification) ([]*ReleaseRun, error)

	// Delete removes a release run.
	Delete(ctx context.Context, id RunID) error
}

// EventPublisher defines the interface for publishing domain events.
type EventPublisher interface {
	// Publish publishes domain events.
	Publish(ctx context.Context, events ...DomainEvent) error
}

// UnitOfWork defines the interface for transactional operations.
type UnitOfWork interface {
	// Commit commits the unit of work.
	Commit(ctx context.Context) error

	// Rollback rolls back the unit of work.
	Rollback() error

	// ReleaseRepository returns the release repository within this unit of work.
	ReleaseRepository() Repository
}

// UnitOfWorkFactory creates new UnitOfWork instances.
type UnitOfWorkFactory interface {
	// Begin starts a new unit of work.
	Begin(ctx context.Context) (UnitOfWork, error)
}
