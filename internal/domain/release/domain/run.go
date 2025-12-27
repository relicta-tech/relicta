// Package domain provides the core domain model for release governance.
// This is the bounded context for managing release runs with DDD principles.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
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

	// Approval
	approval           *Approval
	multiLevelApproval *MultiLevelApproval // Optional multi-level approval tracking

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

// ApprovalLevel represents the type/level of approval required.
type ApprovalLevel string

const (
	ApprovalLevelTechnical ApprovalLevel = "technical" // Technical review (code quality)
	ApprovalLevelSecurity  ApprovalLevel = "security"  // Security review
	ApprovalLevelManager   ApprovalLevel = "manager"   // Management approval
	ApprovalLevelRelease   ApprovalLevel = "release"   // Release manager approval
	ApprovalLevelAuto      ApprovalLevel = "auto"      // Auto-approval (low risk)
)

// Approval holds release approval information.
type Approval struct {
	ApprovedBy    string        // Who approved the release
	ApprovedAt    time.Time     // When it was approved
	AutoApproved  bool          // If auto-approved (risk below threshold)
	PlanHash      string        // The plan hash that was approved
	RiskScore     float64       // Risk score at time of approval
	ApproverType  ActorType     // Type of approver (human, ci, agent)
	Justification string        // Optional justification for approval
	Level         ApprovalLevel // The level of this approval
}

// IsManual returns true if this was a manual approval.
func (a *Approval) IsManual() bool {
	return !a.AutoApproved
}

// ApprovalRequirement defines required approvals for a release.
type ApprovalRequirement struct {
	Level       ApprovalLevel // Required approval level
	Description string        // Human-readable description
	Required    bool          // Whether this approval is mandatory
	AllowedBy   []string      // List of allowed approvers (empty = any)
}

// ApprovalPolicy defines the multi-level approval requirements for a release.
type ApprovalPolicy struct {
	Requirements []ApprovalRequirement // List of required approvals
	Sequential   bool                  // If true, approvals must be in order
}

// MultiLevelApproval tracks multiple approvals for a release.
type MultiLevelApproval struct {
	Policy    ApprovalPolicy              // The approval policy in effect
	Approvals map[ApprovalLevel]*Approval // Approvals granted at each level
}

// NewMultiLevelApproval creates a new multi-level approval tracker.
func NewMultiLevelApproval(policy ApprovalPolicy) *MultiLevelApproval {
	return &MultiLevelApproval{
		Policy:    policy,
		Approvals: make(map[ApprovalLevel]*Approval),
	}
}

// Grant grants an approval at a specific level.
func (m *MultiLevelApproval) Grant(level ApprovalLevel, approval *Approval) error {
	if m.Approvals == nil {
		m.Approvals = make(map[ApprovalLevel]*Approval)
	}
	approval.Level = level
	m.Approvals[level] = approval
	return nil
}

// IsLevelApproved returns true if the given level has been approved.
func (m *MultiLevelApproval) IsLevelApproved(level ApprovalLevel) bool {
	if m.Approvals == nil {
		return false
	}
	_, ok := m.Approvals[level]
	return ok
}

// IsFullyApproved returns true if all required approvals have been granted.
func (m *MultiLevelApproval) IsFullyApproved() bool {
	for _, req := range m.Policy.Requirements {
		if req.Required && !m.IsLevelApproved(req.Level) {
			return false
		}
	}
	return true
}

// PendingApprovals returns the list of approvals still needed.
func (m *MultiLevelApproval) PendingApprovals() []ApprovalRequirement {
	var pending []ApprovalRequirement
	for _, req := range m.Policy.Requirements {
		if req.Required && !m.IsLevelApproved(req.Level) {
			pending = append(pending, req)
		}
	}
	return pending
}

// GetApproval returns the approval for a specific level, or nil if not approved.
func (m *MultiLevelApproval) GetApproval(level ApprovalLevel) *Approval {
	if m.Approvals == nil {
		return nil
	}
	return m.Approvals[level]
}

// AllApprovals returns all granted approvals.
func (m *MultiLevelApproval) AllApprovals() []*Approval {
	var approvals []*Approval
	for _, a := range m.Approvals {
		approvals = append(approvals, a)
	}
	return approvals
}

// NextRequiredLevel returns the next approval level needed (for sequential policies).
func (m *MultiLevelApproval) NextRequiredLevel() *ApprovalRequirement {
	for _, req := range m.Policy.Requirements {
		if req.Required && !m.IsLevelApproved(req.Level) {
			return &req
		}
	}
	return nil
}

// DefaultApprovalPolicy returns a simple single-approval policy.
func DefaultApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		Requirements: []ApprovalRequirement{
			{
				Level:       ApprovalLevelRelease,
				Description: "Release approval",
				Required:    true,
			},
		},
		Sequential: false,
	}
}

// HighRiskApprovalPolicy returns a policy requiring technical and security review.
func HighRiskApprovalPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		Requirements: []ApprovalRequirement{
			{
				Level:       ApprovalLevelTechnical,
				Description: "Technical review",
				Required:    true,
			},
			{
				Level:       ApprovalLevelSecurity,
				Description: "Security review",
				Required:    true,
			},
			{
				Level:       ApprovalLevelRelease,
				Description: "Final release approval",
				Required:    true,
			},
		},
		Sequential: true,
	}
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
	hashPrefix := r.planHash
	if len(hashPrefix) > 16 {
		hashPrefix = hashPrefix[:16]
	}
	r.id = RunID("run-" + hashPrefix)

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

// Version returns the next version (for backwards compatibility).
func (r *ReleaseRun) Version() *version.SemanticVersion {
	ver := r.versionNext
	if ver.String() == "" || ver.String() == "0.0.0" {
		return nil
	}
	return &ver
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

// EmitCreatedEvent emits a RunCreatedEvent.
// This is used when a run is created with a custom ID via ReconstructState.
func (r *ReleaseRun) EmitCreatedEvent() {
	r.addEvent(&RunCreatedEvent{
		RunID:   r.id,
		RepoID:  r.repoID,
		HeadSHA: r.headSHA,
		At:      r.createdAt,
	})
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

	// Update plan hash but keep ID stable
	// Aggregate identity is immutable after creation
	r.planHash = r.computePlanHash()

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

	// Update plan hash to reflect version, but keep ID stable
	// Aggregate identity should not change after creation
	r.planHash = r.computePlanHash()

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

	return r.ApproveWithOptions(actor, autoApproved, r.actorType, "")
}

// ApproveWithOptions approves the release with additional options.
func (r *ReleaseRun) ApproveWithOptions(actor string, autoApproved bool, approverType ActorType, justification string) error {
	if r.state != StateNotesReady {
		return fmt.Errorf("%w: can only approve from NotesReady state, current: %s", ErrInvalidState, r.state)
	}

	now := time.Now()

	// Create the approval record
	r.approval = &Approval{
		ApprovedBy:    actor,
		ApprovedAt:    now,
		AutoApproved:  autoApproved,
		PlanHash:      r.planHash,
		RiskScore:     r.riskScore,
		ApproverType:  approverType,
		Justification: justification,
	}

	metadata := map[string]string{
		"plan_hash":     r.planHash,
		"auto_approved": fmt.Sprintf("%t", autoApproved),
		"risk_score":    fmt.Sprintf("%.2f", r.riskScore),
	}

	r.addEvent(&RunApprovedEvent{
		RunID:        r.id,
		PlanHash:     r.planHash,
		ApprovedBy:   actor,
		AutoApproved: autoApproved,
		At:           now,
	})

	return r.TransitionTo(StateApproved, "APPROVE", actor, "Release approved", metadata)
}

// Approval returns a copy of the approval details if approved.
// Returns nil if not approved. A copy is returned to preserve aggregate encapsulation.
func (r *ReleaseRun) Approval() *Approval {
	if r.approval == nil {
		return nil
	}
	// Return a copy to preserve aggregate boundary
	return &Approval{
		ApprovedBy:    r.approval.ApprovedBy,
		ApprovedAt:    r.approval.ApprovedAt,
		AutoApproved:  r.approval.AutoApproved,
		PlanHash:      r.approval.PlanHash,
		RiskScore:     r.approval.RiskScore,
		ApproverType:  r.approval.ApproverType,
		Justification: r.approval.Justification,
	}
}

// IsApproved returns true if the release has been approved.
func (r *ReleaseRun) IsApproved() bool {
	return r.approval != nil
}

// ApprovalStatus represents the approval readiness of a release.
type ApprovalStatus struct {
	CanApprove bool
	Reason     string
}

// ApprovalStatus returns the approval readiness status for the release.
// This encapsulates the domain logic for determining if a release can be approved.
func (r *ReleaseRun) ApprovalStatus() ApprovalStatus {
	switch r.state {
	case StateNotesReady:
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

// CanApprove returns true if the release can be approved.
func (r *ReleaseRun) CanApprove() bool {
	return r.state == StateNotesReady
}

// CanProceedToPublish returns true if the release can proceed to publishing.
func (r *ReleaseRun) CanProceedToPublish() bool {
	return r.IsApproved() && r.state == StateApproved
}

// ValidateApprovalPlanHash validates that the approval is bound to the current plan hash.
// This prevents executing a release if the plan has been modified after approval.
// Returns nil if valid, ErrApprovalBoundToHash if mismatched.
func (r *ReleaseRun) ValidateApprovalPlanHash() error {
	if r.approval == nil {
		return ErrNotApproved
	}
	if r.approval.PlanHash != r.planHash {
		return fmt.Errorf("%w: approved hash %s, current hash %s",
			ErrApprovalBoundToHash, r.approval.PlanHash[:8], r.planHash[:8])
	}
	return nil
}

// =============================================================================
// Multi-Level Approval Methods
// =============================================================================

// SetApprovalPolicy sets the multi-level approval policy for this release.
// Should be called before any approvals are granted.
func (r *ReleaseRun) SetApprovalPolicy(policy ApprovalPolicy) {
	r.multiLevelApproval = NewMultiLevelApproval(policy)
}

// ApproveAtLevel grants an approval at a specific level for multi-level workflows.
// This does not transition state - use CompleteMultiLevelApproval after all required
// approvals are granted.
func (r *ReleaseRun) ApproveAtLevel(level ApprovalLevel, actor string, approverType ActorType, justification string) error {
	if r.state != StateNotesReady {
		return fmt.Errorf("%w: can only approve from NotesReady state, current: %s", ErrInvalidState, r.state)
	}

	// Initialize multi-level approval if not set
	if r.multiLevelApproval == nil {
		r.multiLevelApproval = NewMultiLevelApproval(DefaultApprovalPolicy())
	}

	// Check if this level is required and in order (for sequential policies)
	if r.multiLevelApproval.Policy.Sequential {
		next := r.multiLevelApproval.NextRequiredLevel()
		if next != nil && next.Level != level {
			return fmt.Errorf("sequential approval required: expecting %s approval, got %s",
				next.Level, level)
		}
	}

	// Create and grant the approval
	approval := &Approval{
		ApprovedBy:    actor,
		ApprovedAt:    time.Now(),
		AutoApproved:  level == ApprovalLevelAuto,
		PlanHash:      r.planHash,
		RiskScore:     r.riskScore,
		ApproverType:  approverType,
		Justification: justification,
		Level:         level,
	}

	if err := r.multiLevelApproval.Grant(level, approval); err != nil {
		return err
	}

	r.updatedAt = time.Now()
	return nil
}

// CompleteMultiLevelApproval transitions to Approved state if all required approvals are granted.
// Returns an error if any required approvals are still pending.
func (r *ReleaseRun) CompleteMultiLevelApproval(actor string) error {
	if r.state != StateNotesReady {
		return fmt.Errorf("%w: can only approve from NotesReady state, current: %s", ErrInvalidState, r.state)
	}

	if r.multiLevelApproval == nil {
		return fmt.Errorf("no approval policy set; use Approve() for single-level approval")
	}

	if !r.multiLevelApproval.IsFullyApproved() {
		pending := r.multiLevelApproval.PendingApprovals()
		var levels []string
		for _, p := range pending {
			levels = append(levels, string(p.Level))
		}
		return fmt.Errorf("pending approvals required: %v", levels)
	}

	// Set the main approval to the final level for backwards compatibility
	finalApproval := r.multiLevelApproval.GetApproval(ApprovalLevelRelease)
	if finalApproval == nil {
		// Use the last granted approval if no release-level approval
		allApprovals := r.multiLevelApproval.AllApprovals()
		if len(allApprovals) > 0 {
			finalApproval = allApprovals[len(allApprovals)-1]
		}
	}
	r.approval = finalApproval

	metadata := map[string]string{
		"plan_hash":       r.planHash,
		"auto_approved":   "false",
		"risk_score":      fmt.Sprintf("%.2f", r.riskScore),
		"approval_levels": fmt.Sprintf("%d", len(r.multiLevelApproval.Approvals)),
	}

	r.addEvent(&RunApprovedEvent{
		RunID:        r.id,
		PlanHash:     r.planHash,
		ApprovedBy:   actor,
		AutoApproved: false,
		At:           time.Now(),
	})

	return r.TransitionTo(StateApproved, "APPROVE", actor, "Multi-level approval complete", metadata)
}

// MultiLevelApprovalStatus returns the current multi-level approval status.
func (r *ReleaseRun) MultiLevelApprovalStatus() *MultiLevelApproval {
	return r.multiLevelApproval
}

// IsMultiLevelApprovalEnabled returns true if multi-level approval is configured.
func (r *ReleaseRun) IsMultiLevelApprovalEnabled() bool {
	return r.multiLevelApproval != nil
}

// PendingApprovalLevels returns the approval levels still needed.
func (r *ReleaseRun) PendingApprovalLevels() []ApprovalRequirement {
	if r.multiLevelApproval == nil {
		return nil
	}
	return r.multiLevelApproval.PendingApprovals()
}

// RepositoryPath returns the repository path (alias for RepoRoot).
func (r *ReleaseRun) RepositoryPath() string {
	return r.repoRoot
}

// RepositoryName returns the repository name (alias for RepoID).
func (r *ReleaseRun) RepositoryName() string {
	return r.repoID
}

// Branch returns the branch name (derived from baseRef if available).
func (r *ReleaseRun) Branch() string {
	return r.baseRef
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

	r.addEvent(&RunCanceledEvent{
		RunID:  r.id,
		Reason: reason,
		By:     actor,
		At:     time.Now(),
	})

	return r.TransitionTo(StateCanceled, "CANCEL", actor, reason, nil)
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
		switch status.State {
		case StepDone, StepSkipped:
			stepsDone++
		case StepFailed:
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

// Invariant provides information about aggregate invariant validation.
type Invariant struct {
	Name        string
	Description string
	Valid       bool
	Message     string
}

// ValidateInvariants checks all aggregate invariants and returns any violations.
// This is useful for debugging and ensuring aggregate consistency.
func (r *ReleaseRun) ValidateInvariants() []Invariant {
	invariants := make([]Invariant, 0, 10)

	// Invariant 1: ID must be non-empty
	invariants = append(invariants, Invariant{
		Name:        "NonEmptyID",
		Description: "Release run must have a non-empty ID",
		Valid:       r.id != "",
		Message:     conditionalMessage(r.id == "", "Run ID is empty"),
	})

	// Invariant 2: State must be valid
	invariants = append(invariants, Invariant{
		Name:        "ValidState",
		Description: "Release run state must be valid",
		Valid:       r.state.IsValid(),
		Message:     conditionalMessage(!r.state.IsValid(), "State is invalid: "+string(r.state)),
	})

	// Invariant 3: HEAD SHA must be set
	invariants = append(invariants, Invariant{
		Name:        "HeadSHASet",
		Description: "Release run must have a pinned HEAD SHA",
		Valid:       r.headSHA != "",
		Message:     conditionalMessage(r.headSHA == "", "HEAD SHA is empty"),
	})

	// Invariant 4: If state is beyond Planned, must have version
	hasVersionIfRequired := r.state == StateDraft || r.state == StatePlanned ||
		(r.versionNext.String() != "" && r.versionNext.String() != "0.0.0")
	invariants = append(invariants, Invariant{
		Name:        "VersionRequired",
		Description: "Release run must have version if state is beyond Planned",
		Valid:       hasVersionIfRequired,
		Message:     conditionalMessage(!hasVersionIfRequired, "Version is not set but state is "+string(r.state)),
	})

	// Invariant 5: If state is beyond NotesReady, must have notes
	hasNotesIfRequired := r.state == StateDraft || r.state == StatePlanned ||
		r.state == StateVersioned || r.notes != nil
	invariants = append(invariants, Invariant{
		Name:        "NotesRequired",
		Description: "Release run must have notes if state is beyond NotesReady",
		Valid:       hasNotesIfRequired,
		Message:     conditionalMessage(!hasNotesIfRequired, "Notes are nil but state is "+string(r.state)),
	})

	// Invariant 6: If published, must have publishedAt timestamp
	hasPublishedAtIfRequired := r.state != StatePublished || r.publishedAt != nil
	invariants = append(invariants, Invariant{
		Name:        "PublishedAtRequired",
		Description: "Published run must have publishedAt timestamp",
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

	// Invariant 8: RepoRoot must be non-empty
	invariants = append(invariants, Invariant{
		Name:        "NonEmptyRepoRoot",
		Description: "Release run must have a non-empty repository root",
		Valid:       r.repoRoot != "",
		Message:     conditionalMessage(r.repoRoot == "", "RepoRoot is empty"),
	})

	// Invariant 9: PlanHash must be set for non-draft states
	planHashRequired := r.state == StateDraft || r.planHash != ""
	invariants = append(invariants, Invariant{
		Name:        "PlanHashRequired",
		Description: "Release run must have plan hash if state is beyond Draft",
		Valid:       planHashRequired,
		Message:     conditionalMessage(!planHashRequired, "PlanHash is empty but state is "+string(r.state)),
	})

	// Invariant 10: If in Publishing state, must have execution plan
	hasStepsIfPublishing := r.state != StatePublishing || len(r.steps) > 0
	invariants = append(invariants, Invariant{
		Name:        "StepsRequiredForPublishing",
		Description: "Publishing run must have execution steps",
		Valid:       hasStepsIfPublishing,
		Message:     conditionalMessage(!hasStepsIfPublishing, "No execution steps but state is publishing"),
	})

	// Invariant 11: If state is Approved or beyond, must have approval
	hasApprovalIfRequired := r.state == StateDraft || r.state == StatePlanned ||
		r.state == StateVersioned || r.state == StateNotesReady || r.approval != nil
	invariants = append(invariants, Invariant{
		Name:        "ApprovalRequired",
		Description: "Release run must have approval if state is approved or beyond",
		Valid:       hasApprovalIfRequired,
		Message:     conditionalMessage(!hasApprovalIfRequired, "Approval is nil but state is "+string(r.state)),
	})

	return invariants
}

// IsValid checks if all aggregate invariants are satisfied.
func (r *ReleaseRun) IsValid() bool {
	for _, inv := range r.ValidateInvariants() {
		if !inv.Valid {
			return false
		}
	}
	return true
}

// InvariantViolations returns only the violated invariants.
func (r *ReleaseRun) InvariantViolations() []Invariant {
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

// ReconstructState reconstructs the release run state from persisted data without
// triggering domain events. This is used by repositories when loading aggregates.
// It should only be called by repository implementations.
func (r *ReleaseRun) ReconstructState(
	id RunID,
	planHash string,
	repoID, repoRoot string,
	baseRef string,
	headSHA CommitSHA,
	commits []CommitSHA,
	configHash, pluginPlanHash string,
	versionCurrent, versionNext version.SemanticVersion,
	bumpKind BumpKind,
	confidence float64,
	riskScore float64,
	reasons []string,
	actorType ActorType,
	actorID string,
	thresholds PolicyThresholds,
	tagName string,
	notes *ReleaseNotes,
	notesInputsHash string,
	approval *Approval,
	steps []StepPlan,
	stepStatus map[string]*StepStatus,
	state RunState,
	history []TransitionRecord,
	lastError string,
	changesetID string,
	createdAt, updatedAt time.Time,
	publishedAt *time.Time,
) {
	r.id = id
	r.planHash = planHash
	r.repoID = repoID
	r.repoRoot = repoRoot
	r.baseRef = baseRef
	r.headSHA = headSHA
	r.commits = commits
	r.configHash = configHash
	r.pluginPlanHash = pluginPlanHash
	r.versionCurrent = versionCurrent
	r.versionNext = versionNext
	r.bumpKind = bumpKind
	r.confidence = confidence
	r.riskScore = riskScore
	r.reasons = reasons
	r.actorType = actorType
	r.actorID = actorID
	r.thresholds = thresholds
	r.tagName = tagName
	r.notes = notes
	r.notesInputsHash = notesInputsHash
	r.approval = approval
	r.steps = steps
	r.stepStatus = stepStatus
	r.state = state
	r.history = history
	r.lastError = lastError
	r.changesetID = changesetID
	r.createdAt = createdAt
	r.updatedAt = updatedAt
	r.publishedAt = publishedAt
	// Clear domain events - we're reconstructing, not creating
	r.domainEvents = make([]DomainEvent, 0, 5)
}

// UpdateNotes updates the release notes in NotesReady state.
// This allows manual editing of notes before approval.
func (r *ReleaseRun) UpdateNotes(notes *ReleaseNotes, actor string) error {
	if r.state != StateNotesReady {
		return NewStateTransitionError(r.state, "update notes")
	}

	if notes == nil {
		return ErrNilNotes
	}

	r.notes = notes
	r.updatedAt = time.Now()

	r.addEvent(&RunNotesUpdatedEvent{
		RunID:       r.id,
		NotesLength: len(notes.Text),
		Actor:       actor,
		At:          time.Now(),
	})

	return nil
}

// UpdateNotesText updates the release notes with just the text content.
// This is a convenience method that creates a ReleaseNotes struct internally.
func (r *ReleaseRun) UpdateNotesText(text string) error {
	notes := &ReleaseNotes{
		Text: text,
	}
	return r.UpdateNotes(notes, "system")
}

// SetNotes sets the release notes (alias for GenerateNotes for backwards compatibility).
func (r *ReleaseRun) SetNotes(notes *ReleaseNotes) error {
	return r.GenerateNotes(notes, "", "system")
}

// RecordPluginExecution records a plugin execution result.
func (r *ReleaseRun) RecordPluginExecution(pluginName, hook string, success bool, msg string, duration time.Duration) {
	r.addEvent(&PluginExecutedEvent{
		RunID:      r.id,
		PluginName: pluginName,
		Hook:       hook,
		Success:    success,
		Message:    msg,
		Duration:   duration,
		At:         time.Now(),
	})
	r.updatedAt = time.Now()
}
