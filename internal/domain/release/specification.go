// Package release provides domain types for release management.
package release

// Specification defines the interface for domain query specifications.
// Specifications encapsulate business rules for querying aggregates.
type Specification interface {
	// IsSatisfiedBy checks if the given release satisfies this specification.
	IsSatisfiedBy(r *Release) bool
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
func (s *AndSpecification) IsSatisfiedBy(r *Release) bool {
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
func (s *OrSpecification) IsSatisfiedBy(r *Release) bool {
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
func (s *NotSpecification) IsSatisfiedBy(r *Release) bool {
	return !s.spec.IsSatisfiedBy(r)
}

// StateSpecification matches releases in a specific state.
type StateSpecification struct {
	state ReleaseState
}

// ByState creates a specification for releases in the given state.
func ByState(state ReleaseState) *StateSpecification {
	return &StateSpecification{state: state}
}

// IsSatisfiedBy returns true if the release is in the specified state.
func (s *StateSpecification) IsSatisfiedBy(r *Release) bool {
	return r.State() == s.state
}

// ActiveSpecification matches releases that are not in a final state.
type ActiveSpecification struct{}

// Active creates a specification for active (non-final) releases.
func Active() *ActiveSpecification {
	return &ActiveSpecification{}
}

// IsSatisfiedBy returns true if the release is not in a final state.
func (s *ActiveSpecification) IsSatisfiedBy(r *Release) bool {
	return !r.State().IsFinal()
}

// FinalSpecification matches releases that are in a final state.
type FinalSpecification struct{}

// Final creates a specification for final (completed/failed/canceled) releases.
func Final() *FinalSpecification {
	return &FinalSpecification{}
}

// IsSatisfiedBy returns true if the release is in a final state.
func (s *FinalSpecification) IsSatisfiedBy(r *Release) bool {
	return r.State().IsFinal()
}

// RepositoryPathSpecification matches releases for a specific repository.
type RepositoryPathSpecification struct {
	path string
}

// ByRepositoryPath creates a specification for releases in the given repository.
func ByRepositoryPath(path string) *RepositoryPathSpecification {
	return &RepositoryPathSpecification{path: path}
}

// IsSatisfiedBy returns true if the release is for the specified repository.
func (s *RepositoryPathSpecification) IsSatisfiedBy(r *Release) bool {
	return r.RepositoryPath() == s.path
}

// BranchSpecification matches releases on a specific branch.
type BranchSpecification struct {
	branch string
}

// ByBranch creates a specification for releases on the given branch.
func ByBranch(branch string) *BranchSpecification {
	return &BranchSpecification{branch: branch}
}

// IsSatisfiedBy returns true if the release is on the specified branch.
func (s *BranchSpecification) IsSatisfiedBy(r *Release) bool {
	return r.Branch() == s.branch
}

// ReadyForPublishSpecification matches releases ready for publishing.
type ReadyForPublishSpecification struct{}

// ReadyForPublish creates a specification for releases ready to be published.
func ReadyForPublish() *ReadyForPublishSpecification {
	return &ReadyForPublishSpecification{}
}

// IsSatisfiedBy returns true if the release can proceed to publish.
func (s *ReadyForPublishSpecification) IsSatisfiedBy(r *Release) bool {
	return r.CanProceedToPublish()
}

// HasPlanSpecification matches releases that have a plan set.
type HasPlanSpecification struct{}

// HasPlan creates a specification for releases with a plan.
func HasPlan() *HasPlanSpecification {
	return &HasPlanSpecification{}
}

// IsSatisfiedBy returns true if the release has a plan.
func (s *HasPlanSpecification) IsSatisfiedBy(r *Release) bool {
	return r.Plan() != nil
}

// HasNotesSpecification matches releases that have notes set.
type HasNotesSpecification struct{}

// HasNotes creates a specification for releases with notes.
func HasNotes() *HasNotesSpecification {
	return &HasNotesSpecification{}
}

// IsSatisfiedBy returns true if the release has notes.
func (s *HasNotesSpecification) IsSatisfiedBy(r *Release) bool {
	return r.Notes() != nil
}

// IsApprovedSpecification matches releases that have been approved.
type IsApprovedSpecification struct{}

// IsApproved creates a specification for approved releases.
func IsApproved() *IsApprovedSpecification {
	return &IsApprovedSpecification{}
}

// IsSatisfiedBy returns true if the release has been approved.
func (s *IsApprovedSpecification) IsSatisfiedBy(r *Release) bool {
	return r.Approval() != nil
}
