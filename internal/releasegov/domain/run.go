// Package domain provides the core domain model for release governance.
// This is the bounded context for managing release runs with DDD principles.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// RunID uniquely identifies a release run. It is derived from the plan_hash.
type RunID string

// String returns the string representation of the RunID.
func (id RunID) String() string {
	return string(id)
}

// Short returns the first 8 characters of the RunID for display.
func (id RunID) Short() string {
	if len(id) > 8 {
		return string(id[:8])
	}
	return string(id)
}

// CommitSHA represents a git commit hash.
type CommitSHA string

// String returns the string representation.
func (c CommitSHA) String() string {
	return string(c)
}

// Short returns the first 7 characters of the commit SHA.
func (c CommitSHA) Short() string {
	if len(c) >= 7 {
		return string(c[:7])
	}
	return string(c)
}

// ActorType represents who or what initiated the release.
type ActorType string

const (
	ActorHuman ActorType = "human"
	ActorCI    ActorType = "ci"
	ActorAgent ActorType = "agent"
)

// BumpKind represents the type of version bump.
type BumpKind string

const (
	BumpMajor      BumpKind = "major"
	BumpMinor      BumpKind = "minor"
	BumpPatch      BumpKind = "patch"
	BumpPrerelease BumpKind = "prerelease"
	BumpNone       BumpKind = "none"
)

// ReleaseRun is the aggregate root for the release governance bounded context.
// It encapsulates all state and invariants for a single release run.
type ReleaseRun struct {
	// Identity
	id       RunID
	planHash string

	// Immutable planning facts (pinned at plan time)
	repoID         string      // Remote URL or derived stable ID
	repoRoot       string      // Local repository root path
	baseRef        string      // Base reference (tag or commit)
	headSHA        CommitSHA   // Exact SHA pinned at plan time - IMMUTABLE after planning
	commits        []CommitSHA // Explicit list of commit SHAs in this release
	configHash     string      // Hash of relevant config snapshot
	pluginPlanHash string      // Hash of plugin configuration

	// Release proposal
	versionCurrent version.SemanticVersion
	versionNext    version.SemanticVersion
	bumpKind       BumpKind
	confidence     float64 // 0..1 confidence in the version calculation

	// Policy evaluation
	riskScore  float64
	reasons    []string
	actorType  ActorType
	actorID    string
	thresholds PolicyThresholds

	// Version tracking
	tagName string // The tag name for the release (e.g., "v1.2.3")

	// Notes
	notes           *ReleaseNotes
	notesInputsHash string

	// Execution plan
	steps      []StepPlan
	stepStatus map[string]*StepStatus

	// State machine
	state       RunState
	history     []TransitionRecord
	lastError   string
	changesetID string // Reference to the changeset aggregate

	// Timestamps
	createdAt   time.Time
	updatedAt   time.Time
	publishedAt *time.Time

	// Domain events (collected for publication)
	domainEvents []DomainEvent
}

// PolicyThresholds captures the policy thresholds at plan time.
type PolicyThresholds struct {
	AutoApproveRiskThreshold float64
	RequireApprovalAbove     float64
	BlockReleaseAbove        float64
}

// ReleaseNotes holds the generated release notes.
type ReleaseNotes struct {
	Text           string
	AudiencePreset string
	TonePreset     string
	Provider       string
	Model          string
	GeneratedAt    time.Time
}

// StepType represents the type of publishing step.
type StepType string

const (
	StepTypeTag       StepType = "tag"
	StepTypeBuild     StepType = "build"
	StepTypeArtifact  StepType = "artifact"
	StepTypeNotify    StepType = "notify"
	StepTypeFinalize  StepType = "finalize"
	StepTypePlugin    StepType = "plugin"
	StepTypeChangelog StepType = "changelog"
)

// StepPlan describes a single step in the publishing execution plan.
type StepPlan struct {
	Name           string
	Type           StepType
	ConfigHash     string
	IdempotencyKey string // Derived from run_id + step_name + config_hash
	PluginName     string // For plugin steps
	Hook           string // For plugin steps
	Unsafe         bool   // If true, requires explicit approval
}

// StepState represents the execution state of a step.
type StepState string

const (
	StepPending StepState = "pending"
	StepRunning StepState = "running"
	StepDone    StepState = "done"
	StepFailed  StepState = "failed"
	StepSkipped StepState = "skipped"
)

// StepStatus tracks the execution status of a step.
type StepStatus struct {
	State       StepState
	Attempts    int
	LastError   string
	StartedAt   *time.Time
	CompletedAt *time.Time
	Output      string // Step output/result
}

// TransitionRecord records a state transition for audit.
type TransitionRecord struct {
	At       time.Time
	From     RunState
	To       RunState
	Event    string
	Actor    string
	Reason   string
	Metadata map[string]string
}

// Domain errors
var (
	ErrInvalidState        = errors.New("invalid state for this operation")
	ErrHeadSHAChanged      = errors.New("repository HEAD has changed since planning")
	ErrAlreadyPublished    = errors.New("release is already published")
	ErrNotApproved         = errors.New("release is not approved")
	ErrPlanHashMismatch    = errors.New("plan hash does not match approved plan")
	ErrStepAlreadyDone     = errors.New("step is already completed")
	ErrStepNotFound        = errors.New("step not found in execution plan")
	ErrRunNotFound         = errors.New("release run not found")
	ErrApprovalBoundToHash = errors.New("approval is bound to a different plan hash")
)

// NewReleaseRun creates a new ReleaseRun aggregate in Draft state.
func NewReleaseRun(
	repoID, repoRoot string,
	baseRef string,
	headSHA CommitSHA,
	commits []CommitSHA,
	configHash, pluginPlanHash string,
) *ReleaseRun {
	now := time.Now()
	r := &ReleaseRun{
		repoID:         repoID,
		repoRoot:       repoRoot,
		baseRef:        baseRef,
		headSHA:        headSHA,
		commits:        commits,
		configHash:     configHash,
		pluginPlanHash: pluginPlanHash,
		state:          StateDraft,
		stepStatus:     make(map[string]*StepStatus),
		history:        make([]TransitionRecord, 0, 10),
		createdAt:      now,
		updatedAt:      now,
		domainEvents:   make([]DomainEvent, 0, 5),
	}

	// Compute plan hash and run ID
	r.planHash = r.computePlanHash()
	r.id = RunID("run-" + r.planHash[:16])

	r.addEvent(&RunCreatedEvent{
		RunID:   r.id,
		RepoID:  repoID,
		HeadSHA: headSHA,
		At:      now,
	})

	return r
}

// computePlanHash computes a deterministic hash of the planning facts.
func (r *ReleaseRun) computePlanHash() string {
	h := sha256.New()

	// Include all immutable planning facts
	h.Write([]byte(r.repoID))
	h.Write([]byte(r.baseRef))
	h.Write([]byte(r.headSHA))

	// Sort commits for deterministic ordering
	sortedCommits := make([]string, len(r.commits))
	for i, c := range r.commits {
		sortedCommits[i] = string(c)
	}
	sort.Strings(sortedCommits)
	for _, c := range sortedCommits {
		h.Write([]byte(c))
	}

	h.Write([]byte(r.versionNext.String()))
	h.Write([]byte(r.configHash))
	h.Write([]byte(r.pluginPlanHash))

	return hex.EncodeToString(h.Sum(nil))
}

// ID returns the run ID.
func (r *ReleaseRun) ID() RunID {
	return r.id
}

// PlanHash returns the plan hash.
func (r *ReleaseRun) PlanHash() string {
	return r.planHash
}

// RepoID returns the repository ID.
func (r *ReleaseRun) RepoID() string {
	return r.repoID
}

// RepoRoot returns the repository root path.
func (r *ReleaseRun) RepoRoot() string {
	return r.repoRoot
}

// BaseRef returns the base reference.
func (r *ReleaseRun) BaseRef() string {
	return r.baseRef
}

// HeadSHA returns the pinned HEAD SHA.
func (r *ReleaseRun) HeadSHA() CommitSHA {
	return r.headSHA
}

// Commits returns the list of commit SHAs in this release.
func (r *ReleaseRun) Commits() []CommitSHA {
	return r.commits
}

// VersionCurrent returns the current version.
func (r *ReleaseRun) VersionCurrent() version.SemanticVersion {
	return r.versionCurrent
}

// VersionNext returns the next version.
func (r *ReleaseRun) VersionNext() version.SemanticVersion {
	return r.versionNext
}

// BumpKind returns the type of version bump.
func (r *ReleaseRun) BumpKind() BumpKind {
	return r.bumpKind
}

// RiskScore returns the calculated risk score.
func (r *ReleaseRun) RiskScore() float64 {
	return r.riskScore
}

// Reasons returns the reasons for the risk assessment.
func (r *ReleaseRun) Reasons() []string {
	return r.reasons
}

// ActorType returns the type of actor who initiated the release.
func (r *ReleaseRun) ActorType() ActorType {
	return r.actorType
}

// ActorID returns the ID of the actor.
func (r *ReleaseRun) ActorID() string {
	return r.actorID
}

// TagName returns the tag name for the release.
func (r *ReleaseRun) TagName() string {
	return r.tagName
}

// Notes returns the release notes if generated.
func (r *ReleaseRun) Notes() *ReleaseNotes {
	return r.notes
}

// Steps returns the execution plan steps.
func (r *ReleaseRun) Steps() []StepPlan {
	return r.steps
}

// StepStatus returns the status of a specific step.
func (r *ReleaseRun) StepStatus(stepName string) *StepStatus {
	return r.stepStatus[stepName]
}

// AllStepStatuses returns all step statuses.
func (r *ReleaseRun) AllStepStatuses() map[string]*StepStatus {
	result := make(map[string]*StepStatus)
	for k, v := range r.stepStatus {
		result[k] = v
	}
	return result
}

// State returns the current state.
func (r *ReleaseRun) State() RunState {
	return r.state
}

// History returns the transition history.
func (r *ReleaseRun) History() []TransitionRecord {
	return r.history
}

// LastError returns the last error message.
func (r *ReleaseRun) LastError() string {
	return r.lastError
}

// ChangesetID returns the changeset ID reference.
func (r *ReleaseRun) ChangesetID() string {
	return r.changesetID
}

// CreatedAt returns when the run was created.
func (r *ReleaseRun) CreatedAt() time.Time {
	return r.createdAt
}

// UpdatedAt returns when the run was last updated.
func (r *ReleaseRun) UpdatedAt() time.Time {
	return r.updatedAt
}

// PublishedAt returns when the run was published.
func (r *ReleaseRun) PublishedAt() *time.Time {
	return r.publishedAt
}

// DomainEvents returns all uncommitted domain events.
func (r *ReleaseRun) DomainEvents() []DomainEvent {
	return r.domainEvents
}

// ClearDomainEvents clears the domain events after publication.
func (r *ReleaseRun) ClearDomainEvents() {
	r.domainEvents = make([]DomainEvent, 0, 5)
}

// addEvent adds a domain event.
func (r *ReleaseRun) addEvent(event DomainEvent) {
	r.domainEvents = append(r.domainEvents, event)
}

// recordTransition records a state transition in history.
func (r *ReleaseRun) recordTransition(from, to RunState, event, actor, reason string, metadata map[string]string) {
	r.history = append(r.history, TransitionRecord{
		At:       time.Now(),
		From:     from,
		To:       to,
		Event:    event,
		Actor:    actor,
		Reason:   reason,
		Metadata: metadata,
	})
	r.updatedAt = time.Now()
}

// SetVersionProposal sets the version proposal during planning.
func (r *ReleaseRun) SetVersionProposal(current, next version.SemanticVersion, bumpKind BumpKind, confidence float64) error {
	if r.state != StateDraft {
		return fmt.Errorf("%w: cannot set version proposal in state %s", ErrInvalidState, r.state)
	}

	r.versionCurrent = current
	r.versionNext = next
	r.bumpKind = bumpKind
	r.confidence = confidence

	// Recompute plan hash after version is set
	r.planHash = r.computePlanHash()
	r.id = RunID("run-" + r.planHash[:16])

	r.updatedAt = time.Now()
	return nil
}

// SetPolicyEvaluation sets the policy evaluation results.
func (r *ReleaseRun) SetPolicyEvaluation(riskScore float64, reasons []string, thresholds PolicyThresholds) {
	r.riskScore = riskScore
	r.reasons = reasons
	r.thresholds = thresholds
	r.updatedAt = time.Now()
}

// SetActor sets the actor information.
func (r *ReleaseRun) SetActor(actorType ActorType, actorID string) {
	r.actorType = actorType
	r.actorID = actorID
	r.updatedAt = time.Now()
}

// SetChangesetID sets the changeset reference.
func (r *ReleaseRun) SetChangesetID(id string) {
	r.changesetID = id
	r.updatedAt = time.Now()
}

// SetExecutionPlan sets the publishing execution plan.
func (r *ReleaseRun) SetExecutionPlan(steps []StepPlan) {
	r.steps = steps
	r.stepStatus = make(map[string]*StepStatus, len(steps))
	for _, step := range steps {
		r.stepStatus[step.Name] = &StepStatus{
			State:    StepPending,
			Attempts: 0,
		}
	}
	r.updatedAt = time.Now()
}

// TransitionTo attempts to transition to a new state.
// This is the core state machine method.
func (r *ReleaseRun) TransitionTo(to RunState, event, actor, reason string, metadata map[string]string) error {
	from := r.state

	// Validate transition is allowed
	if !r.state.CanTransitionTo(to) {
		return fmt.Errorf("%w: cannot transition from %s to %s via %s", ErrInvalidState, from, to, event)
	}

	r.state = to
	r.recordTransition(from, to, event, actor, reason, metadata)

	r.addEvent(&StateTransitionedEvent{
		RunID: r.id,
		From:  from,
		To:    to,
		Event: event,
		Actor: actor,
		At:    time.Now(),
	})

	return nil
}

// Plan transitions from Draft to Planned state.
func (r *ReleaseRun) Plan(actor string) error {
	if r.state != StateDraft {
		return fmt.Errorf("%w: can only plan from Draft state, current: %s", ErrInvalidState, r.state)
	}

	return r.TransitionTo(StatePlanned, "PLAN", actor, "Release planned", nil)
}

// SetVersion sets the calculated version for the release.
// This should be called while in Planned state, before calling Bump().
func (r *ReleaseRun) SetVersion(next version.SemanticVersion, tagName string) error {
	if r.state != StatePlanned {
		return fmt.Errorf("%w: can only set version in Planned state, current: %s", ErrInvalidState, r.state)
	}

	r.versionNext = next
	r.tagName = tagName

	// Recompute plan hash now that version is finalized
	r.planHash = r.computePlanHash()
	r.id = RunID("run-" + r.planHash[:16])

	r.updatedAt = time.Now()
	return nil
}

// Bump transitions from Planned to Versioned state.
// The version should already be set via SetVersion().
func (r *ReleaseRun) Bump(actor string) error {
	if r.state != StatePlanned {
		return fmt.Errorf("%w: can only bump from Planned state, current: %s", ErrInvalidState, r.state)
	}

	if r.versionNext.String() == "" || r.versionNext.String() == "0.0.0" {
		return fmt.Errorf("%w: version must be set before bumping", ErrInvalidState)
	}

	r.addEvent(&RunVersionedEvent{
		RunID:       r.id,
		VersionNext: r.versionNext,
		BumpKind:    r.bumpKind,
		TagName:     r.tagName,
		Actor:       actor,
		At:          time.Now(),
	})

	return r.TransitionTo(StateVersioned, "BUMP", actor, "Version applied", map[string]string{
		"version":  r.versionNext.String(),
		"tag_name": r.tagName,
	})
}

// GenerateNotes sets the release notes and transitions to NotesReady.
func (r *ReleaseRun) GenerateNotes(notes *ReleaseNotes, inputsHash, actor string) error {
	if r.state != StateVersioned {
		return fmt.Errorf("%w: can only generate notes from Versioned state, current: %s", ErrInvalidState, r.state)
	}

	r.notes = notes
	r.notesInputsHash = inputsHash

	return r.TransitionTo(StateNotesReady, "GENERATE_NOTES", actor, "Notes generated", nil)
}

// Approve approves the release and transitions to Approved state.
// The approval is bound to the current plan hash.
func (r *ReleaseRun) Approve(actor string, autoApproved bool) error {
	if r.state != StateNotesReady {
		return fmt.Errorf("%w: can only approve from NotesReady state, current: %s", ErrInvalidState, r.state)
	}

	metadata := map[string]string{
		"plan_hash":     r.planHash,
		"auto_approved": fmt.Sprintf("%t", autoApproved),
	}

	r.addEvent(&RunApprovedEvent{
		RunID:        r.id,
		PlanHash:     r.planHash,
		ApprovedBy:   actor,
		AutoApproved: autoApproved,
		At:           time.Now(),
	})

	return r.TransitionTo(StateApproved, "APPROVE", actor, "Release approved", metadata)
}

// StartPublishing transitions to Publishing state.
func (r *ReleaseRun) StartPublishing(actor string) error {
	if r.state != StateApproved {
		return fmt.Errorf("%w: can only start publishing from Approved state, current: %s", ErrInvalidState, r.state)
	}

	return r.TransitionTo(StatePublishing, "START_PUBLISH", actor, "Publishing started", nil)
}

// MarkStepStarted marks a step as started.
func (r *ReleaseRun) MarkStepStarted(stepName string) error {
	status, ok := r.stepStatus[stepName]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStepNotFound, stepName)
	}

	if status.State == StepDone {
		return fmt.Errorf("%w: %s", ErrStepAlreadyDone, stepName)
	}

	now := time.Now()
	status.State = StepRunning
	status.Attempts++
	status.StartedAt = &now
	r.updatedAt = now

	return nil
}

// MarkStepDone marks a step as completed successfully.
func (r *ReleaseRun) MarkStepDone(stepName string, output string) error {
	status, ok := r.stepStatus[stepName]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStepNotFound, stepName)
	}

	now := time.Now()
	status.State = StepDone
	status.CompletedAt = &now
	status.Output = output
	status.LastError = ""
	r.updatedAt = now

	r.addEvent(&StepCompletedEvent{
		RunID:    r.id,
		StepName: stepName,
		Success:  true,
		At:       now,
	})

	return nil
}

// MarkStepFailed marks a step as failed.
func (r *ReleaseRun) MarkStepFailed(stepName string, err error) error {
	status, ok := r.stepStatus[stepName]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStepNotFound, stepName)
	}

	now := time.Now()
	status.State = StepFailed
	status.CompletedAt = &now
	status.LastError = err.Error()
	r.updatedAt = now
	r.lastError = fmt.Sprintf("step %s failed: %s", stepName, err.Error())

	r.addEvent(&StepCompletedEvent{
		RunID:    r.id,
		StepName: stepName,
		Success:  false,
		Error:    err.Error(),
		At:       now,
	})

	return nil
}

// MarkStepSkipped marks a step as skipped (e.g., already done externally).
func (r *ReleaseRun) MarkStepSkipped(stepName string, reason string) error {
	status, ok := r.stepStatus[stepName]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStepNotFound, stepName)
	}

	now := time.Now()
	status.State = StepSkipped
	status.CompletedAt = &now
	status.Output = reason
	r.updatedAt = now

	return nil
}

// AllStepsDone returns true if all steps are in a terminal state (done/skipped/failed).
func (r *ReleaseRun) AllStepsDone() bool {
	for _, status := range r.stepStatus {
		if status.State == StepPending || status.State == StepRunning {
			return false
		}
	}
	return true
}

// AllStepsSucceeded returns true if all steps completed successfully.
func (r *ReleaseRun) AllStepsSucceeded() bool {
	for _, status := range r.stepStatus {
		if status.State != StepDone && status.State != StepSkipped {
			return false
		}
	}
	return true
}

// NextPendingStep returns the next step that needs to be executed.
func (r *ReleaseRun) NextPendingStep() *StepPlan {
	for i := range r.steps {
		status := r.stepStatus[r.steps[i].Name]
		if status.State == StepPending || status.State == StepFailed {
			return &r.steps[i]
		}
	}
	return nil
}

// MarkPublished transitions to Published state.
func (r *ReleaseRun) MarkPublished(actor string) error {
	if r.state != StatePublishing {
		return fmt.Errorf("%w: can only mark published from Publishing state, current: %s", ErrInvalidState, r.state)
	}

	if !r.AllStepsSucceeded() {
		return fmt.Errorf("%w: not all steps succeeded", ErrInvalidState)
	}

	now := time.Now()
	r.publishedAt = &now

	r.addEvent(&RunPublishedEvent{
		RunID:   r.id,
		Version: r.versionNext,
		At:      now,
	})

	return r.TransitionTo(StatePublished, "PUBLISH_COMPLETE", actor, "Release published", nil)
}

// MarkFailed transitions to Failed state.
func (r *ReleaseRun) MarkFailed(reason, actor string) error {
	r.lastError = reason

	r.addEvent(&RunFailedEvent{
		RunID:  r.id,
		Reason: reason,
		At:     time.Now(),
	})

	return r.TransitionTo(StateFailed, "FAIL", actor, reason, nil)
}

// Cancel cancels the release run.
func (r *ReleaseRun) Cancel(reason, actor string) error {
	if r.state == StatePublished {
		return ErrAlreadyPublished
	}

	r.lastError = reason

	r.addEvent(&RunCancelledEvent{
		RunID:  r.id,
		Reason: reason,
		By:     actor,
		At:     time.Now(),
	})

	return r.TransitionTo(StateCancelled, "CANCEL", actor, reason, nil)
}

// RetryPublish prepares the run for retry by resetting failed steps.
func (r *ReleaseRun) RetryPublish(actor string) error {
	if r.state != StateFailed {
		return fmt.Errorf("%w: can only retry from Failed state, current: %s", ErrInvalidState, r.state)
	}

	// Reset failed steps to pending
	for name, status := range r.stepStatus {
		if status.State == StepFailed {
			r.stepStatus[name] = &StepStatus{
				State:    StepPending,
				Attempts: status.Attempts, // Keep the attempt count
			}
		}
	}

	r.lastError = ""

	r.addEvent(&RunRetriedEvent{
		RunID: r.id,
		By:    actor,
		At:    time.Now(),
	})

	return r.TransitionTo(StatePublishing, "RETRY_PUBLISH", actor, "Retrying publish", nil)
}

// ValidateHeadMatch checks if the current HEAD matches the pinned head_sha.
func (r *ReleaseRun) ValidateHeadMatch(currentHead CommitSHA) error {
	if r.headSHA != currentHead {
		return fmt.Errorf("%w: expected %s, got %s", ErrHeadSHAChanged, r.headSHA.Short(), currentHead.Short())
	}
	return nil
}

// CanAutoApprove returns true if the release can be auto-approved based on policy.
func (r *ReleaseRun) CanAutoApprove() bool {
	return r.riskScore <= r.thresholds.AutoApproveRiskThreshold
}

// RequiresApproval returns true if manual approval is required.
func (r *ReleaseRun) RequiresApproval() bool {
	return r.riskScore > r.thresholds.AutoApproveRiskThreshold
}

// IsBlocked returns true if the release is blocked by policy.
func (r *ReleaseRun) IsBlocked() bool {
	return r.riskScore >= r.thresholds.BlockReleaseAbove
}

// Summary returns a summary of the release run for display.
type RunSummary struct {
	ID             RunID
	State          RunState
	HeadSHA        CommitSHA
	VersionCurrent string
	VersionNext    string
	BumpKind       BumpKind
	RiskScore      float64
	CommitCount    int
	StepsTotal     int
	StepsDone      int
	StepsFailed    int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Summary returns a summary of the run.
func (r *ReleaseRun) Summary() RunSummary {
	stepsDone := 0
	stepsFailed := 0
	for _, status := range r.stepStatus {
		if status.State == StepDone || status.State == StepSkipped {
			stepsDone++
		} else if status.State == StepFailed {
			stepsFailed++
		}
	}

	return RunSummary{
		ID:             r.id,
		State:          r.state,
		HeadSHA:        r.headSHA,
		VersionCurrent: r.versionCurrent.String(),
		VersionNext:    r.versionNext.String(),
		BumpKind:       r.bumpKind,
		RiskScore:      r.riskScore,
		CommitCount:    len(r.commits),
		StepsTotal:     len(r.steps),
		StepsDone:      stepsDone,
		StepsFailed:    stepsFailed,
		CreatedAt:      r.createdAt,
		UpdatedAt:      r.updatedAt,
	}
}

// BuildIdempotencyKey creates an idempotency key for a step.
func BuildIdempotencyKey(runID RunID, stepName, configHash string) string {
	h := sha256.New()
	h.Write([]byte(runID))
	h.Write([]byte(stepName))
	h.Write([]byte(configHash))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ParseBumpKind parses a string into a BumpKind.
func ParseBumpKind(s string) (BumpKind, error) {
	switch strings.ToLower(s) {
	case "major":
		return BumpMajor, nil
	case "minor":
		return BumpMinor, nil
	case "patch":
		return BumpPatch, nil
	case "prerelease":
		return BumpPrerelease, nil
	case "none", "":
		return BumpNone, nil
	default:
		return BumpNone, fmt.Errorf("invalid bump kind: %s", s)
	}
}

// ToBumpType converts BumpKind to the version package's BumpType.
func (bk BumpKind) ToBumpType() version.BumpType {
	switch bk {
	case BumpMajor:
		return version.BumpMajor
	case BumpMinor:
		return version.BumpMinor
	case BumpPatch:
		return version.BumpPatch
	default:
		return version.BumpPatch
	}
}

// FromReleaseType converts a changes.ReleaseType to BumpKind.
func BumpKindFromReleaseType(rt changes.ReleaseType) BumpKind {
	switch rt {
	case changes.ReleaseTypeMajor:
		return BumpMajor
	case changes.ReleaseTypeMinor:
		return BumpMinor
	case changes.ReleaseTypePatch:
		return BumpPatch
	default:
		return BumpPatch
	}
}
