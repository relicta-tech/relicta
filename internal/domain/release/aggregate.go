// Package release provides domain types for release management.
package release

import (
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// ReleaseID uniquely identifies a release.
type ReleaseID string

// Release is the aggregate root for the release management bounded context.
// It encapsulates all invariants and business rules for the release workflow.
type Release struct {
	// Identity
	id ReleaseID

	// State
	state    ReleaseState
	plan     *ReleasePlan
	version  *version.SemanticVersion
	notes    *ReleaseNotes
	approval *Approval

	// Context
	branch         string
	repositoryPath string
	repositoryName string
	tagName        string

	// Timestamps
	createdAt   time.Time
	updatedAt   time.Time
	publishedAt *time.Time

	// Domain events (for event sourcing / event publishing)
	domainEvents []DomainEvent

	// Error tracking
	lastError string
}

// ReleasePlan holds the planned release information.
type ReleasePlan struct {
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	ReleaseType    changes.ReleaseType
	// ChangeSetID references the changeset by identity (DDD aggregate boundary).
	// Use GetChangeSet() to retrieve the full changeset when needed.
	ChangeSetID changes.ChangeSetID
	// changeSet is the cached changeset (transient, not persisted with the aggregate).
	// This allows efficient access during the same use case execution.
	changeSet *changes.ChangeSet
	DryRun    bool
}

// NewReleasePlan creates a new release plan with proper aggregate references.
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
// Returns nil if the changeset was not loaded or has been cleared.
func (p *ReleasePlan) GetChangeSet() *changes.ChangeSet {
	return p.changeSet
}

// SetChangeSet sets the cached changeset (used when loading from repository).
func (p *ReleasePlan) SetChangeSet(cs *changes.ChangeSet) {
	p.changeSet = cs
}

// HasChangeSet returns true if a changeset is cached.
func (p *ReleasePlan) HasChangeSet() bool {
	return p.changeSet != nil
}

// CommitCount returns the number of commits in the changeset.
// Returns 0 if no changeset is loaded.
func (p *ReleasePlan) CommitCount() int {
	if p.changeSet == nil {
		return 0
	}
	return p.changeSet.CommitCount()
}

// ReleaseNotes holds generated release documentation.
type ReleaseNotes struct {
	Changelog   string
	Summary     string
	AIGenerated bool
	GeneratedAt time.Time
}

// Approval holds release approval information.
type Approval struct {
	ApprovedBy   string
	ApprovedAt   time.Time
	AutoApproved bool
}

// NewRelease creates a new Release aggregate.
func NewRelease(id ReleaseID, branch, repoPath string) *Release {
	r := &Release{
		id:             id,
		state:          StateInitialized,
		branch:         branch,
		repositoryPath: repoPath,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		domainEvents:   make([]DomainEvent, 0, 5), // Typical release generates 3-5 events
	}

	r.addEvent(NewReleaseInitializedEvent(id, branch, repoPath))
	return r
}

// ID returns the release ID.
func (r *Release) ID() ReleaseID {
	return r.id
}

// State returns the current release state.
func (r *Release) State() ReleaseState {
	return r.state
}

// Plan returns the release plan if set.
func (r *Release) Plan() *ReleasePlan {
	return r.plan
}

// Version returns the release version if set.
func (r *Release) Version() *version.SemanticVersion {
	return r.version
}

// Notes returns the release notes if generated.
func (r *Release) Notes() *ReleaseNotes {
	return r.notes
}

// Branch returns the release branch.
func (r *Release) Branch() string {
	return r.branch
}

// RepositoryPath returns the repository path.
func (r *Release) RepositoryPath() string {
	return r.repositoryPath
}

// RepositoryName returns the repository name.
func (r *Release) RepositoryName() string {
	return r.repositoryName
}

// TagName returns the tag name for this release.
func (r *Release) TagName() string {
	return r.tagName
}

// CreatedAt returns when the release was created.
func (r *Release) CreatedAt() time.Time {
	return r.createdAt
}

// UpdatedAt returns when the release was last updated.
func (r *Release) UpdatedAt() time.Time {
	return r.updatedAt
}

// PublishedAt returns when the release was published (nil if not published).
func (r *Release) PublishedAt() *time.Time {
	return r.publishedAt
}

// LastError returns the last error message if any.
func (r *Release) LastError() string {
	return r.lastError
}

// IsApproved returns true if the release is approved.
func (r *Release) IsApproved() bool {
	return r.approval != nil
}

// Approval returns a copy of the approval details if approved.
// Returns nil if not approved. A copy is returned to preserve aggregate encapsulation.
func (r *Release) Approval() *Approval {
	if r.approval == nil {
		return nil
	}
	// Return a copy to preserve aggregate boundary
	return &Approval{
		ApprovedBy:   r.approval.ApprovedBy,
		ApprovedAt:   r.approval.ApprovedAt,
		AutoApproved: r.approval.AutoApproved,
	}
}

// DomainEvents returns all uncommitted domain events.
func (r *Release) DomainEvents() []DomainEvent {
	return r.domainEvents
}

// ClearDomainEvents clears the domain events after they've been published.
func (r *Release) ClearDomainEvents() {
	r.domainEvents = make([]DomainEvent, 0, 5) // Typical release generates 3-5 events
}

// addEvent adds a domain event.
func (r *Release) addEvent(event DomainEvent) {
	r.domainEvents = append(r.domainEvents, event)
}

// SetRepositoryName sets the repository name.
func (r *Release) SetRepositoryName(name string) {
	r.repositoryName = name
	r.updatedAt = time.Now()
}

// SetPlan sets the release plan and transitions to StatePlanned.
func (r *Release) SetPlan(plan *ReleasePlan) error {
	if !r.state.CanTransitionTo(StatePlanned) {
		return NewStateTransitionError(r.state, "set plan")
	}

	if plan == nil {
		return ErrNilPlan
	}

	r.plan = plan
	r.state = StatePlanned
	r.updatedAt = time.Now()

	// Get commit count using the new method
	commitCount := plan.CommitCount()

	r.addEvent(NewReleasePlannedEvent(
		r.id,
		plan.CurrentVersion,
		plan.NextVersion,
		plan.ReleaseType.String(),
		commitCount,
	))

	return nil
}

// SetVersion sets the release version and transitions to StateVersioned.
func (r *Release) SetVersion(ver version.SemanticVersion, tagName string) error {
	if !r.state.CanTransitionTo(StateVersioned) {
		return NewStateTransitionError(r.state, "set version")
	}

	r.version = &ver
	r.tagName = tagName
	r.state = StateVersioned
	r.updatedAt = time.Now()

	r.addEvent(NewReleaseVersionedEvent(r.id, ver, tagName))

	return nil
}

// SetNotes sets the release notes and transitions to StateNotesGenerated.
func (r *Release) SetNotes(notes *ReleaseNotes) error {
	if !r.state.CanTransitionTo(StateNotesGenerated) {
		return NewStateTransitionError(r.state, "generate notes")
	}

	if notes == nil {
		return ErrNilNotes
	}

	r.notes = notes
	r.state = StateNotesGenerated
	r.updatedAt = time.Now()

	r.addEvent(NewReleaseNotesGeneratedEvent(r.id, true, len(notes.Changelog)))

	return nil
}

// UpdateNotes updates the release notes without changing state.
// This can only be called in StateNotesGenerated to allow editing before approval.
func (r *Release) UpdateNotes(changelog string) error {
	if r.state != StateNotesGenerated {
		return NewStateTransitionError(r.state, "update notes")
	}

	if r.notes == nil {
		return ErrNilNotes
	}

	r.notes = &ReleaseNotes{
		Changelog:   changelog,
		Summary:     r.notes.Summary,
		AIGenerated: false, // Mark as manually edited
		GeneratedAt: r.notes.GeneratedAt,
	}
	r.updatedAt = time.Now()

	r.addEvent(NewReleaseNotesUpdatedEvent(r.id, len(changelog)))

	return nil
}

// Approve approves the release and transitions to StateApproved.
func (r *Release) Approve(approvedBy string, autoApproved bool) error {
	if !r.state.CanTransitionTo(StateApproved) {
		return NewStateTransitionError(r.state, "approve")
	}

	r.approval = &Approval{
		ApprovedBy:   approvedBy,
		ApprovedAt:   time.Now(),
		AutoApproved: autoApproved,
	}
	r.state = StateApproved
	r.updatedAt = time.Now()

	r.addEvent(NewReleaseApprovedEvent(r.id, approvedBy))

	return nil
}

// StartPublishing transitions to StatePublishing.
func (r *Release) StartPublishing(plugins []string) error {
	if !r.state.CanTransitionTo(StatePublishing) {
		return NewStateTransitionError(r.state, "start publishing")
	}

	r.state = StatePublishing
	r.updatedAt = time.Now()

	r.addEvent(NewReleasePublishingStartedEvent(r.id, plugins))

	return nil
}

// MarkPublished marks the release as published.
func (r *Release) MarkPublished(releaseURL string) error {
	if !r.state.CanTransitionTo(StatePublished) {
		return NewStateTransitionError(r.state, "publish")
	}

	now := time.Now()
	r.state = StatePublished
	r.publishedAt = &now
	r.updatedAt = now

	r.addEvent(NewReleasePublishedEvent(r.id, *r.version, r.tagName, releaseURL))

	return nil
}

// MarkFailed marks the release as failed.
func (r *Release) MarkFailed(reason string, recoverable bool) error {
	if !r.state.CanTransitionTo(StateFailed) {
		return NewStateTransitionError(r.state, "mark as failed")
	}

	previousState := r.state
	r.state = StateFailed
	r.lastError = reason
	r.updatedAt = time.Now()

	r.addEvent(NewReleaseFailedEvent(r.id, reason, previousState, recoverable))

	return nil
}

// Cancel cancels the release.
func (r *Release) Cancel(reason, canceledBy string) error {
	if !r.state.CanTransitionTo(StateCanceled) {
		return NewStateTransitionError(r.state, "cancel")
	}

	r.state = StateCanceled
	r.lastError = reason
	r.updatedAt = time.Now()

	r.addEvent(NewReleaseCanceledEvent(r.id, reason, canceledBy))

	return nil
}

// Retry resets the release to allow retrying from a failed state.
func (r *Release) Retry() error {
	if r.state != StateFailed && r.state != StateCanceled {
		return NewStateTransitionError(r.state, "retry")
	}

	previousState := r.state
	newState := r.determineRetryState()

	r.resetToState(newState)
	r.addEvent(NewReleaseRetriedEvent(r.id, previousState, newState))

	return nil
}

// determineRetryState determines the appropriate state to reset to when retrying.
// Returns StatePlanned if a plan exists, otherwise StateInitialized.
func (r *Release) determineRetryState() ReleaseState {
	if r.plan != nil {
		return StatePlanned
	}
	return StateInitialized
}

// resetToState resets the release to the given state, clearing errors.
// This is a helper method that does not generate domain events; the caller
// (e.g., Retry) is responsible for generating the appropriate event.
func (r *Release) resetToState(state ReleaseState) {
	r.state = state
	r.lastError = ""
	r.updatedAt = time.Now()
}

// RecordPluginExecution records a plugin execution result.
func (r *Release) RecordPluginExecution(pluginName, hook string, success bool, msg string, duration time.Duration) {
	r.addEvent(NewPluginExecutedEvent(r.id, pluginName, hook, success, msg, duration))
	r.updatedAt = time.Now()
}

// ReconstructState reconstructs the release state from persisted data without
// triggering domain events. This is used by repositories when loading aggregates.
// It should only be called by repository implementations.
func (r *Release) ReconstructState(
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
	r.state = state
	r.plan = plan
	r.version = ver
	r.tagName = tagName
	r.notes = notes
	r.approval = approval
	r.createdAt = createdAt
	r.updatedAt = updatedAt
	r.publishedAt = publishedAt
	r.lastError = lastError
	// Clear domain events - we're reconstructing, not creating
	r.domainEvents = make([]DomainEvent, 0, 5) // Typical release generates 3-5 events
}

// CanProceedToPublish returns true if the release can proceed to publishing.
func (r *Release) CanProceedToPublish() bool {
	return r.state == StateApproved &&
		r.version != nil &&
		r.notes != nil &&
		r.plan != nil
}

// CanApprove returns true if the release can be approved.
func (r *Release) CanApprove() bool {
	return r.state.CanTransitionTo(StateApproved)
}

// ApprovalStatus represents the approval readiness of a release.
type ApprovalStatus struct {
	CanApprove bool
	Reason     string
}

// ApprovalStatus returns the approval readiness status for the release.
// This encapsulates the domain logic for determining if a release can be approved.
func (r *Release) ApprovalStatus() ApprovalStatus {
	switch r.state {
	case StateNotesGenerated:
		return ApprovalStatus{
			CanApprove: true,
			Reason:     "Release is ready for approval",
		}
	case StateApproved:
		return ApprovalStatus{
			CanApprove: false,
			Reason:     "Release is already approved",
		}
	case StatePublishing, StatePublished:
		return ApprovalStatus{
			CanApprove: false,
			Reason:     "Release has already progressed past approval",
		}
	case StateFailed, StateCanceled:
		return ApprovalStatus{
			CanApprove: false,
			Reason:     "Release is in a terminal state: " + string(r.state),
		}
	default:
		return ApprovalStatus{
			CanApprove: false,
			Reason:     "Release is not ready for approval, current state: " + string(r.state),
		}
	}
}

// Invariant provides information about aggregate invariant validation.
type Invariant struct {
	Name        string
	Description string
	Valid       bool
	Message     string
}

// ValidateInvariants checks all aggregate invariants and returns any violations.
// This is useful for debugging and ensuring aggregate consistency.
func (r *Release) ValidateInvariants() []Invariant {
	invariants := make([]Invariant, 0, 8)

	// Invariant 1: ID must be non-empty
	invariants = append(invariants, Invariant{
		Name:        "NonEmptyID",
		Description: "Release must have a non-empty ID",
		Valid:       r.id != "",
		Message:     conditionalMessage(r.id == "", "Release ID is empty"),
	})

	// Invariant 2: State must be valid
	invariants = append(invariants, Invariant{
		Name:        "ValidState",
		Description: "Release state must be a valid state",
		Valid:       r.state.IsValid(),
		Message:     conditionalMessage(!r.state.IsValid(), "State is invalid: "+string(r.state)),
	})

	// Invariant 3: If state is beyond Planned, must have plan
	hasPlanIfRequired := r.state == StateInitialized || r.plan != nil
	invariants = append(invariants, Invariant{
		Name:        "PlanRequired",
		Description: "Release must have plan if state is beyond initialized",
		Valid:       hasPlanIfRequired,
		Message:     conditionalMessage(!hasPlanIfRequired, "Plan is nil but state is "+string(r.state)),
	})

	// Invariant 4: If state is beyond NotesGenerated, must have notes
	hasNotesIfRequired := r.state == StateInitialized || r.state == StatePlanned ||
		r.state == StateVersioned || r.notes != nil
	invariants = append(invariants, Invariant{
		Name:        "NotesRequired",
		Description: "Release must have notes if state is beyond notes_generated",
		Valid:       hasNotesIfRequired,
		Message:     conditionalMessage(!hasNotesIfRequired, "Notes are nil but state is "+string(r.state)),
	})

	// Invariant 5: If state is Approved or beyond, must have approval
	hasApprovalIfRequired := r.state == StateInitialized || r.state == StatePlanned ||
		r.state == StateVersioned || r.state == StateNotesGenerated || r.approval != nil
	invariants = append(invariants, Invariant{
		Name:        "ApprovalRequired",
		Description: "Release must have approval if state is approved or beyond",
		Valid:       hasApprovalIfRequired,
		Message:     conditionalMessage(!hasApprovalIfRequired, "Approval is nil but state is "+string(r.state)),
	})

	// Invariant 6: If published, must have publishedAt timestamp
	hasPublishedAtIfRequired := r.state != StatePublished || r.publishedAt != nil
	invariants = append(invariants, Invariant{
		Name:        "PublishedAtRequired",
		Description: "Published release must have publishedAt timestamp",
		Valid:       hasPublishedAtIfRequired,
		Message:     conditionalMessage(!hasPublishedAtIfRequired, "publishedAt is nil but state is published"),
	})

	// Invariant 7: CreatedAt must be before or equal to UpdatedAt
	createdBeforeUpdated := !r.createdAt.After(r.updatedAt)
	invariants = append(invariants, Invariant{
		Name:        "CreatedBeforeUpdated",
		Description: "createdAt must not be after updatedAt",
		Valid:       createdBeforeUpdated,
		Message:     conditionalMessage(!createdBeforeUpdated, "createdAt is after updatedAt"),
	})

	// Invariant 8: Branch must be non-empty
	invariants = append(invariants, Invariant{
		Name:        "NonEmptyBranch",
		Description: "Release must have a non-empty branch",
		Valid:       r.branch != "",
		Message:     conditionalMessage(r.branch == "", "Branch is empty"),
	})

	return invariants
}

// IsValid checks if all aggregate invariants are satisfied.
func (r *Release) IsValid() bool {
	for _, inv := range r.ValidateInvariants() {
		if !inv.Valid {
			return false
		}
	}
	return true
}

// InvariantViolations returns only the violated invariants.
func (r *Release) InvariantViolations() []Invariant {
	violations := make([]Invariant, 0)
	for _, inv := range r.ValidateInvariants() {
		if !inv.Valid {
			violations = append(violations, inv)
		}
	}
	return violations
}

// conditionalMessage returns msg if condition is true, empty string otherwise.
func conditionalMessage(condition bool, msg string) string {
	if condition {
		return msg
	}
	return ""
}

// Summary returns a summary of the release.
type ReleaseSummary struct {
	ID             ReleaseID
	State          ReleaseState
	Branch         string
	Repository     string
	CurrentVersion string
	NextVersion    string
	ReleaseType    string
	CommitCount    int
	IsApproved     bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Summary returns a summary of the release.
func (r *Release) Summary() ReleaseSummary {
	summary := ReleaseSummary{
		ID:         r.id,
		State:      r.state,
		Branch:     r.branch,
		Repository: r.repositoryName,
		IsApproved: r.IsApproved(),
		CreatedAt:  r.createdAt,
		UpdatedAt:  r.updatedAt,
	}

	if r.plan != nil {
		summary.CurrentVersion = r.plan.CurrentVersion.String()
		summary.NextVersion = r.plan.NextVersion.String()
		summary.ReleaseType = r.plan.ReleaseType.String()
		summary.CommitCount = r.plan.CommitCount()
	}

	return summary
}
