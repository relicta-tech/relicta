// Package release provides the release governance bounded context.
// This file re-exports domain types for clean imports.
package release

import (
	"context"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// planCache stores release plans with their changesets by run ID.
// This provides a bridge between the old API (which passed changesets through plans)
// and the new DDD model (which doesn't store changesets in the aggregate).
var planCache sync.Map

// ReleasePlan holds the planned release information.
// This type provides backwards compatibility with the old Release aggregate.
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

// NewRelease creates a new Release (ReleaseRun) for backwards compatibility.
// The provided id parameter is used to maintain compatibility with existing code.
func NewRelease(id ReleaseID, branch, repoPath string) *ReleaseRun {
	run := domain.NewReleaseRun(
		repoPath, // repoID - use path as ID
		repoPath, // repoRoot
		branch,   // baseRef
		"",       // headSHA - will be set later
		nil,      // commits - will be set later
		"",       // configHash
		"",       // pluginPlanHash
	)

	// Override the auto-generated ID with the provided ID for backwards compatibility
	run.ReconstructState(
		domain.RunID(id),                    // use provided id
		"",                                  // planHash
		repoPath,                            // repoID
		repoPath,                            // repoRoot
		branch,                              // baseRef
		"",                                  // headSHA
		nil,                                 // commits
		"",                                  // configHash
		"",                                  // pluginPlanHash
		version.SemanticVersion{},           // versionCurrent
		version.SemanticVersion{},           // versionNext
		domain.BumpNone,                     // bumpKind
		0.0,                                 // confidence
		0.0,                                 // riskScore
		nil,                                 // reasons
		domain.ActorHuman,                   // actorType
		"",                                  // actorID
		domain.PolicyThresholds{},           // thresholds
		"",                                  // tagName
		nil,                                 // notes
		"",                                  // notesInputsHash
		nil,                                 // approval
		nil,                                 // steps
		make(map[string]*domain.StepStatus), // stepStatus
		domain.StateDraft,                   // state
		nil,                                 // history
		"",                                  // lastError
		"",                                  // changesetID
		run.CreatedAt(),                     // createdAt
		run.UpdatedAt(),                     // updatedAt
		nil,                                 // publishedAt
	)

	// Emit creation event (ReconstructState clears events, so we emit after)
	run.EmitCreatedEvent()

	return run
}

// SetPlan sets the release plan on a ReleaseRun for backwards compatibility.
// It extracts version info from the ReleasePlan and calls the appropriate methods.
// The plan (including its changeset) is cached for later retrieval by GetPlan.
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

	// Cache the plan with its changeset for later retrieval
	// The run ID is generated during Plan(), so we cache after planning
	planCache.Store(string(r.ID()), plan)

	return nil
}

// GetPlan extracts a ReleasePlan from a ReleaseRun for backwards compatibility.
// If the plan was previously set via SetPlan, it returns the cached plan with its changeset.
// Otherwise, it reconstructs a plan from the run's version info (without changeset).
func GetPlan(r *ReleaseRun) *ReleasePlan {
	if r == nil {
		return nil
	}

	// Try to retrieve cached plan with changeset
	if cached, ok := planCache.Load(string(r.ID())); ok {
		if plan, ok := cached.(*ReleasePlan); ok {
			return plan
		}
	}

	// Fall back to reconstructing plan from run data (without changeset)
	return &ReleasePlan{
		CurrentVersion: r.VersionCurrent(),
		NextVersion:    r.VersionNext(),
		ReleaseType:    changes.ReleaseType(r.BumpKind()),
		DryRun:         false,
	}
}

// ClearPlanCache clears the plan cache. Used for testing.
func ClearPlanCache() {
	planCache.Range(func(key, value interface{}) bool {
		planCache.Delete(key)
		return true
	})
}

// Re-export core aggregate and value objects
type (
	// Backwards-compatible aliases (old names -> new DDD names)
	// These allow gradual migration of existing code
	Release      = domain.ReleaseRun // Old name -> new aggregate
	ReleaseID    = domain.RunID      // Old name -> new ID type
	ReleaseState = domain.RunState   // Old name -> new state type

	// New DDD names (preferred)
	// ReleaseRun is the aggregate root for release governance.
	ReleaseRun = domain.ReleaseRun

	// RunID uniquely identifies a release run.
	RunID = domain.RunID

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

// Re-export domain events (new DDD names)
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

	// ReleaseSummary alias for backwards compatibility
	ReleaseSummary = domain.RunSummary
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

	// Backwards-compatible error aliases
	ErrReleaseNotFound = domain.ErrRunNotFound
	ErrNilPlan         = domain.ErrVersionNotSet // Maps to version not set
)

// State constants (new DDD names)
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

// Backwards-compatible state constants (old names -> new names)
const (
	StateInitialized    = domain.StateDraft      // Old name for StateDraft
	StateNotesGenerated = domain.StateNotesReady // Old name for StateNotesReady
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
// This follows the DDD repository pattern.
type Repository interface {
	// Save persists a release.
	Save(ctx context.Context, release *Release) error

	// FindByID retrieves a release by its ID.
	FindByID(ctx context.Context, id ReleaseID) (*Release, error)

	// FindLatest retrieves the latest release for a repository.
	FindLatest(ctx context.Context, repoPath string) (*Release, error)

	// FindByState retrieves releases in a specific state.
	FindByState(ctx context.Context, state ReleaseState) ([]*Release, error)

	// FindActive retrieves all active (non-final) releases.
	FindActive(ctx context.Context) ([]*Release, error)

	// FindBySpecification retrieves releases matching the given specification.
	FindBySpecification(ctx context.Context, spec Specification) ([]*Release, error)

	// Delete removes a release.
	Delete(ctx context.Context, id ReleaseID) error
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

// ReconstructFromLegacy reconstructs a ReleaseRun from legacy persisted data.
// This allows the persistence layer to restore aggregates without needing to know
// the full internal structure of the new domain model.
func ReconstructFromLegacy(
	rel *ReleaseRun,
	state ReleaseState,
	plan *ReleasePlan,
	ver *version.SemanticVersion,
	tagName string,
	notes *ReleaseNotes,
	approval *Approval,
	createdAt, updatedAt time.Time,
	publishedAt *time.Time,
	lastError string,
) {
	// Extract version info from plan if available
	var versionCurrent, versionNext version.SemanticVersion
	bumpKind := BumpNone
	if plan != nil {
		versionCurrent = plan.CurrentVersion
		versionNext = plan.NextVersion
		bumpKind = BumpKindFromReleaseType(plan.ReleaseType)
		// Cache the plan with its changeset for later retrieval by GetPlan
		planCache.Store(string(rel.ID()), plan)
	}

	// Use explicitly provided version if available (may have build metadata)
	if ver != nil {
		versionNext = *ver
	}

	// Use ReconstructState with defaults for new fields
	rel.ReconstructState(
		rel.ID(),             // id
		"",                   // planHash
		rel.RepositoryPath(), // repoID
		rel.RepositoryPath(), // repoRoot
		rel.Branch(),         // baseRef
		"",                   // headSHA
		nil,                  // commits
		"",                   // configHash
		"",                   // pluginPlanHash
		versionCurrent,       // versionCurrent
		versionNext,          // versionNext
		bumpKind,             // bumpKind
		1.0,                  // confidence
		0.0,                  // riskScore
		nil,                  // reasons
		ActorHuman,           // actorType
		"",                   // actorID
		PolicyThresholds{},   // thresholds
		tagName,              // tagName
		notes,                // notes
		"",                   // notesInputsHash
		approval,             // approval
		nil,                  // steps
		nil,                  // stepStatus
		state,                // state
		nil,                  // history
		lastError,            // lastError
		"",                   // changesetID
		createdAt,            // createdAt
		updatedAt,            // updatedAt
		publishedAt,          // publishedAt
	)
}
