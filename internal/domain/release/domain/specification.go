// Package domain provides the core domain model for release governance.
package domain

// Specification defines the interface for domain query specifications.
// Specifications encapsulate business rules for querying aggregates.
type Specification interface {
	// IsSatisfiedBy checks if the given release run satisfies this specification.
	IsSatisfiedBy(r *ReleaseRun) bool
}

// CompositeSpecification provides base functionality for composite specifications.
type CompositeSpecification struct {
	specs []Specification
}

// AndSpecification represents a logical AND of multiple specifications.
type AndSpecification struct {
	CompositeSpecification
}

// And creates a new AndSpecification combining the given specifications.
func And(specs ...Specification) *AndSpecification {
	return &AndSpecification{
		CompositeSpecification: CompositeSpecification{specs: specs},
	}
}

// IsSatisfiedBy returns true if all child specifications are satisfied.
func (s *AndSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	for _, spec := range s.specs {
		if !spec.IsSatisfiedBy(r) {
			return false
		}
	}
	return true
}

// OrSpecification represents a logical OR of multiple specifications.
type OrSpecification struct {
	CompositeSpecification
}

// Or creates a new OrSpecification combining the given specifications.
func Or(specs ...Specification) *OrSpecification {
	return &OrSpecification{
		CompositeSpecification: CompositeSpecification{specs: specs},
	}
}

// IsSatisfiedBy returns true if any child specification is satisfied.
func (s *OrSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	if len(s.specs) == 0 {
		return true
	}
	for _, spec := range s.specs {
		if spec.IsSatisfiedBy(r) {
			return true
		}
	}
	return false
}

// NotSpecification represents a logical NOT of a specification.
type NotSpecification struct {
	spec Specification
}

// Not creates a new NotSpecification negating the given specification.
func Not(spec Specification) *NotSpecification {
	return &NotSpecification{spec: spec}
}

// IsSatisfiedBy returns the inverse of the wrapped specification.
func (s *NotSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return !s.spec.IsSatisfiedBy(r)
}

// StateSpecification matches runs in a specific state.
type StateSpecification struct {
	state RunState
}

// ByState creates a specification for runs in the given state.
func ByState(state RunState) *StateSpecification {
	return &StateSpecification{state: state}
}

// IsSatisfiedBy returns true if the run is in the specified state.
func (s *StateSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.State() == s.state
}

// ActiveSpecification matches runs that are not in a final state.
type ActiveSpecification struct{}

// Active creates a specification for active (non-final) runs.
func Active() *ActiveSpecification {
	return &ActiveSpecification{}
}

// IsSatisfiedBy returns true if the run is not in a final state.
func (s *ActiveSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return !r.State().IsFinal()
}

// FinalSpecification matches runs that are in a final state.
type FinalSpecification struct{}

// Final creates a specification for final (completed/failed/canceled) runs.
func Final() *FinalSpecification {
	return &FinalSpecification{}
}

// IsSatisfiedBy returns true if the run is in a final state.
func (s *FinalSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.State().IsFinal()
}

// RepositoryPathSpecification matches runs for a specific repository.
type RepositoryPathSpecification struct {
	path string
}

// ByRepositoryPath creates a specification for runs in the given repository.
func ByRepositoryPath(path string) *RepositoryPathSpecification {
	return &RepositoryPathSpecification{path: path}
}

// IsSatisfiedBy returns true if the run is for the specified repository.
func (s *RepositoryPathSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.RepoRoot() == s.path
}

// RepoIDSpecification matches runs for a specific repository ID.
type RepoIDSpecification struct {
	repoID string
}

// ByRepoID creates a specification for runs with the given repository ID.
func ByRepoID(repoID string) *RepoIDSpecification {
	return &RepoIDSpecification{repoID: repoID}
}

// IsSatisfiedBy returns true if the run is for the specified repository ID.
func (s *RepoIDSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.RepoID() == s.repoID
}

// ReadyForPublishSpecification matches runs ready for publishing.
type ReadyForPublishSpecification struct{}

// ReadyForPublish creates a specification for runs ready to be published.
func ReadyForPublish() *ReadyForPublishSpecification {
	return &ReadyForPublishSpecification{}
}

// IsSatisfiedBy returns true if the run can proceed to publish.
func (s *ReadyForPublishSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.State() == StateApproved &&
		r.VersionNext().String() != "" &&
		r.VersionNext().String() != "0.0.0" &&
		r.Notes() != nil
}

// HasNotesSpecification matches runs that have notes generated.
type HasNotesSpecification struct{}

// HasNotes creates a specification for runs with notes.
func HasNotes() *HasNotesSpecification {
	return &HasNotesSpecification{}
}

// IsSatisfiedBy returns true if the run has notes.
func (s *HasNotesSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.Notes() != nil
}

// IsApprovedSpecification matches runs that have been approved.
type IsApprovedSpecification struct{}

// IsApproved creates a specification for approved runs.
func IsApproved() *IsApprovedSpecification {
	return &IsApprovedSpecification{}
}

// IsSatisfiedBy returns true if the run is in Approved state or beyond.
func (s *IsApprovedSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	state := r.State()
	return state == StateApproved || state == StatePublishing || state == StatePublished
}

// HeadSHAMatchesSpecification matches runs where the HEAD SHA matches.
type HeadSHAMatchesSpecification struct {
	headSHA CommitSHA
}

// HeadSHAMatches creates a specification for runs with the given HEAD SHA.
func HeadSHAMatches(sha CommitSHA) *HeadSHAMatchesSpecification {
	return &HeadSHAMatchesSpecification{headSHA: sha}
}

// IsSatisfiedBy returns true if the run's pinned HEAD SHA matches.
func (s *HeadSHAMatchesSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.HeadSHA() == s.headSHA
}

// CanBumpSpecification matches runs that can proceed to version bump.
type CanBumpSpecification struct{}

// CanBump creates a specification for runs that can be bumped.
func CanBump() *CanBumpSpecification {
	return &CanBumpSpecification{}
}

// IsSatisfiedBy returns true if the run can proceed to version bump.
func (s *CanBumpSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.State() == StatePlanned
}

// CanGenerateNotesSpecification matches runs that can generate notes.
type CanGenerateNotesSpecification struct{}

// CanGenerateNotes creates a specification for runs that can generate notes.
func CanGenerateNotes() *CanGenerateNotesSpecification {
	return &CanGenerateNotesSpecification{}
}

// IsSatisfiedBy returns true if the run can generate notes.
func (s *CanGenerateNotesSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.State() == StateVersioned
}

// CanApproveSpecification matches runs that can be approved.
type CanApproveSpecification struct{}

// CanApprove creates a specification for runs that can be approved.
func CanApprove() *CanApproveSpecification {
	return &CanApproveSpecification{}
}

// IsSatisfiedBy returns true if the run can be approved.
func (s *CanApproveSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.State() == StateNotesReady && r.notes != nil
}

// RiskBelowThresholdSpecification matches runs with risk below a threshold.
type RiskBelowThresholdSpecification struct {
	threshold float64
}

// RiskBelowThreshold creates a specification for runs with risk below the threshold.
func RiskBelowThreshold(threshold float64) *RiskBelowThresholdSpecification {
	return &RiskBelowThresholdSpecification{threshold: threshold}
}

// IsSatisfiedBy returns true if the run's risk score is below the threshold.
func (s *RiskBelowThresholdSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.RiskScore() <= s.threshold
}

// CanAutoApproveSpecification matches runs that can be auto-approved.
type CanAutoApproveSpecification struct{}

// CanAutoApprove creates a specification for runs that can be auto-approved.
func CanAutoApprove() *CanAutoApproveSpecification {
	return &CanAutoApproveSpecification{}
}

// IsSatisfiedBy returns true if the run can be auto-approved.
func (s *CanAutoApproveSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.CanAutoApprove()
}

// AllStepsSucceededSpecification matches runs where all steps succeeded.
type AllStepsSucceededSpecification struct{}

// AllStepsSucceeded creates a specification for runs with all steps succeeded.
func AllStepsSucceeded() *AllStepsSucceededSpecification {
	return &AllStepsSucceededSpecification{}
}

// IsSatisfiedBy returns true if all steps in the run succeeded.
func (s *AllStepsSucceededSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	return r.AllStepsSucceeded()
}

// HasFailedStepsSpecification matches runs that have failed steps.
type HasFailedStepsSpecification struct{}

// HasFailedSteps creates a specification for runs with failed steps.
func HasFailedSteps() *HasFailedStepsSpecification {
	return &HasFailedStepsSpecification{}
}

// IsSatisfiedBy returns true if the run has any failed steps.
func (s *HasFailedStepsSpecification) IsSatisfiedBy(r *ReleaseRun) bool {
	for _, status := range r.AllStepStatuses() {
		if status.State == StepFailed {
			return true
		}
	}
	return false
}
