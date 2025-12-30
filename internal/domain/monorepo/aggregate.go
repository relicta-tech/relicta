// Package monorepo provides domain model for multi-package/monorepo versioning.
package monorepo

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

// MonorepoReleaseID is a unique identifier for a monorepo release.
type MonorepoReleaseID string

// NewMonorepoReleaseID creates a new unique release ID.
func NewMonorepoReleaseID() MonorepoReleaseID {
	return MonorepoReleaseID(uuid.New().String())
}

// MonorepoReleaseState represents the state of a monorepo release.
type MonorepoReleaseState string

const (
	// StateDraft is the initial state when a release is created.
	StateDraft MonorepoReleaseState = "draft"
	// StatePlanned is when packages have been analyzed and planned.
	StatePlanned MonorepoReleaseState = "planned"
	// StateVersioned is when package versions have been calculated.
	StateVersioned MonorepoReleaseState = "versioned"
	// StateNotesReady is when release notes have been generated.
	StateNotesReady MonorepoReleaseState = "notes_ready"
	// StateApproved is when the release has been approved.
	StateApproved MonorepoReleaseState = "approved"
	// StatePublishing is when the release is being published.
	StatePublishing MonorepoReleaseState = "publishing"
	// StatePublished is when the release has been successfully published.
	StatePublished MonorepoReleaseState = "published"
	// StateFailed is when the release has failed.
	StateFailed MonorepoReleaseState = "failed"
	// StateCanceled is when the release has been canceled.
	StateCanceled MonorepoReleaseState = "canceled"
)

// MonorepoRelease is the aggregate root for multi-package releases.
// It coordinates the release of multiple packages within a monorepo,
// managing their versions, changelogs, and publishing lifecycle.
type MonorepoRelease struct {
	// ID is the unique identifier for this release.
	ID MonorepoReleaseID
	// RepoID identifies the repository (e.g., "github.com/owner/repo").
	RepoID string
	// BaseRef is the git reference to compare against (e.g., previous release tag).
	BaseRef string
	// HeadRef is the target git reference for this release (e.g., HEAD, branch).
	HeadRef string
	// Packages contains the individual package releases.
	Packages []*PackageRelease
	// State is the current state of the release.
	State MonorepoReleaseState
	// Strategy is the versioning strategy used.
	Strategy MonorepoStrategy
	// CreatedAt is when the release was created.
	CreatedAt time.Time
	// UpdatedAt is when the release was last updated.
	UpdatedAt time.Time
	// ApprovedAt is when the release was approved (if applicable).
	ApprovedAt *time.Time
	// ApprovedBy is the actor who approved the release.
	ApprovedBy string
	// PublishedAt is when the release was published (if applicable).
	PublishedAt *time.Time
	// FailureReason contains the reason for failure (if failed).
	FailureReason string
	// Events contains domain events that have occurred.
	Events []DomainEvent
}

// MonorepoStrategy defines how packages are versioned together.
type MonorepoStrategy string

const (
	// StrategyIndependent allows each package to have its own version.
	StrategyIndependent MonorepoStrategy = "independent"
	// StrategyLockstep keeps all packages at the same version.
	StrategyLockstep MonorepoStrategy = "lockstep"
	// StrategyHybrid allows groups of packages with different strategies.
	StrategyHybrid MonorepoStrategy = "hybrid"
)

// NewMonorepoRelease creates a new monorepo release.
func NewMonorepoRelease(repoID, baseRef, headRef string, strategy MonorepoStrategy) *MonorepoRelease {
	now := time.Now()
	rel := &MonorepoRelease{
		ID:        NewMonorepoReleaseID(),
		RepoID:    repoID,
		BaseRef:   baseRef,
		HeadRef:   headRef,
		Packages:  make([]*PackageRelease, 0),
		State:     StateDraft,
		Strategy:  strategy,
		CreatedAt: now,
		UpdatedAt: now,
		Events:    make([]DomainEvent, 0),
	}
	rel.addEvent(MonorepoReleaseCreated{
		ReleaseID: rel.ID,
		RepoID:    repoID,
		BaseRef:   baseRef,
		HeadRef:   headRef,
		Strategy:  strategy,
		Timestamp: now,
	})
	return rel
}

// AddPackage adds a package to this release.
func (r *MonorepoRelease) AddPackage(pkg *PackageRelease) error {
	if r.State != StateDraft && r.State != StatePlanned {
		return fmt.Errorf("cannot add package in state %s", r.State)
	}

	// Check for duplicate package paths
	for _, existing := range r.Packages {
		if existing.PackagePath == pkg.PackagePath {
			return fmt.Errorf("package %s already exists in release", pkg.PackagePath)
		}
	}

	r.Packages = append(r.Packages, pkg)
	r.UpdatedAt = time.Now()
	r.addEvent(PackageAddedToRelease{
		ReleaseID:   r.ID,
		PackagePath: pkg.PackagePath,
		PackageType: pkg.PackageType,
		Timestamp:   r.UpdatedAt,
	})
	return nil
}

// Plan transitions the release to planned state.
func (r *MonorepoRelease) Plan() error {
	if r.State != StateDraft {
		return fmt.Errorf("cannot plan release in state %s (expected draft)", r.State)
	}
	if len(r.Packages) == 0 {
		return errors.New("cannot plan release with no packages")
	}

	r.State = StatePlanned
	r.UpdatedAt = time.Now()
	r.addEvent(MonorepoReleasePlanned{
		ReleaseID:    r.ID,
		PackageCount: len(r.Packages),
		Timestamp:    r.UpdatedAt,
	})
	return nil
}

// SetVersions sets versions for all packages and transitions to versioned state.
func (r *MonorepoRelease) SetVersions() error {
	if r.State != StatePlanned {
		return fmt.Errorf("cannot set versions in state %s (expected planned)", r.State)
	}

	// Verify all included packages have a next version
	for _, pkg := range r.Packages {
		if pkg.State == PackageStateIncluded && pkg.NextVersion.IsZero() {
			return fmt.Errorf("package %s is included but has no next version", pkg.PackagePath)
		}
	}

	r.State = StateVersioned
	r.UpdatedAt = time.Now()
	r.addEvent(MonorepoReleaseVersioned{
		ReleaseID: r.ID,
		Packages:  r.getVersionSummary(),
		Timestamp: r.UpdatedAt,
	})
	return nil
}

// GenerateNotes marks that release notes have been generated.
func (r *MonorepoRelease) GenerateNotes() error {
	if r.State != StateVersioned {
		return fmt.Errorf("cannot generate notes in state %s (expected versioned)", r.State)
	}

	r.State = StateNotesReady
	r.UpdatedAt = time.Now()
	r.addEvent(MonorepoReleaseNotesReady{
		ReleaseID: r.ID,
		Timestamp: r.UpdatedAt,
	})
	return nil
}

// Approve approves the release for publishing.
func (r *MonorepoRelease) Approve(approver string) error {
	if r.State != StateNotesReady {
		return fmt.Errorf("cannot approve release in state %s (expected notes_ready)", r.State)
	}

	now := time.Now()
	r.State = StateApproved
	r.ApprovedAt = &now
	r.ApprovedBy = approver
	r.UpdatedAt = now
	r.addEvent(MonorepoReleaseApproved{
		ReleaseID:  r.ID,
		ApprovedBy: approver,
		Timestamp:  now,
	})
	return nil
}

// StartPublish transitions to publishing state.
func (r *MonorepoRelease) StartPublish() error {
	if r.State != StateApproved {
		return fmt.Errorf("cannot publish release in state %s (expected approved)", r.State)
	}

	r.State = StatePublishing
	r.UpdatedAt = time.Now()
	r.addEvent(MonorepoReleasePublishing{
		ReleaseID: r.ID,
		Timestamp: r.UpdatedAt,
	})
	return nil
}

// Complete marks the release as successfully published.
func (r *MonorepoRelease) Complete() error {
	if r.State != StatePublishing {
		return fmt.Errorf("cannot complete release in state %s (expected publishing)", r.State)
	}

	now := time.Now()
	r.State = StatePublished
	r.PublishedAt = &now
	r.UpdatedAt = now

	// Mark all included packages as released
	for _, pkg := range r.Packages {
		if pkg.State == PackageStateIncluded {
			_ = pkg.MarkReleased() // Error ignored: state is validated above
		}
	}

	r.addEvent(MonorepoReleasePublished{
		ReleaseID: r.ID,
		Packages:  r.getVersionSummary(),
		Timestamp: now,
	})
	return nil
}

// Fail marks the release as failed.
func (r *MonorepoRelease) Fail(reason string) error {
	if r.State == StatePublished || r.State == StateFailed || r.State == StateCanceled {
		return fmt.Errorf("cannot fail release in terminal state %s", r.State)
	}

	r.State = StateFailed
	r.FailureReason = reason
	r.UpdatedAt = time.Now()
	r.addEvent(MonorepoReleaseFailed{
		ReleaseID: r.ID,
		Reason:    reason,
		Timestamp: r.UpdatedAt,
	})
	return nil
}

// Cancel cancels the release.
func (r *MonorepoRelease) Cancel(reason string) error {
	if r.State == StatePublished || r.State == StateFailed || r.State == StateCanceled {
		return fmt.Errorf("cannot cancel release in terminal state %s", r.State)
	}

	r.State = StateCanceled
	r.FailureReason = reason
	r.UpdatedAt = time.Now()
	r.addEvent(MonorepoReleaseCanceled{
		ReleaseID: r.ID,
		Reason:    reason,
		Timestamp: r.UpdatedAt,
	})
	return nil
}

// GetIncludedPackages returns packages that are included in this release.
func (r *MonorepoRelease) GetIncludedPackages() []*PackageRelease {
	var included []*PackageRelease
	for _, pkg := range r.Packages {
		if pkg.State == PackageStateIncluded {
			included = append(included, pkg)
		}
	}
	return included
}

// GetPackageByPath returns a package by its path.
func (r *MonorepoRelease) GetPackageByPath(path string) *PackageRelease {
	for _, pkg := range r.Packages {
		if pkg.PackagePath == path {
			return pkg
		}
	}
	return nil
}

// FlushEvents returns and clears all pending domain events.
func (r *MonorepoRelease) FlushEvents() []DomainEvent {
	events := r.Events
	r.Events = make([]DomainEvent, 0)
	return events
}

// addEvent adds a domain event.
func (r *MonorepoRelease) addEvent(event DomainEvent) {
	r.Events = append(r.Events, event)
}

// getVersionSummary returns a summary of package versions for events.
func (r *MonorepoRelease) getVersionSummary() []PackageVersionSummary {
	var summary []PackageVersionSummary
	for _, pkg := range r.Packages {
		if pkg.State == PackageStateIncluded || pkg.State == PackageStateReleased {
			summary = append(summary, PackageVersionSummary{
				PackagePath:    pkg.PackagePath,
				CurrentVersion: pkg.CurrentVersion.String(),
				NextVersion:    pkg.NextVersion.String(),
				BumpType:       string(pkg.BumpType),
			})
		}
	}
	return summary
}

// PackageVersionSummary provides version info for events.
type PackageVersionSummary struct {
	PackagePath    string
	CurrentVersion string
	NextVersion    string
	BumpType       string
}

// IsTerminal returns true if the release is in a terminal state.
func (r *MonorepoRelease) IsTerminal() bool {
	return r.State == StatePublished || r.State == StateFailed || r.State == StateCanceled
}

// CanModifyPackages returns true if packages can be added/modified.
func (r *MonorepoRelease) CanModifyPackages() bool {
	return r.State == StateDraft || r.State == StatePlanned
}

// BumpType represents the type of version bump.
type BumpType string

const (
	BumpTypeNone  BumpType = "none"
	BumpTypePatch BumpType = "patch"
	BumpTypeMinor BumpType = "minor"
	BumpTypeMajor BumpType = "major"
)

// ParseBumpType parses a string into a BumpType.
func ParseBumpType(s string) (BumpType, error) {
	switch s {
	case "none", "":
		return BumpTypeNone, nil
	case "patch":
		return BumpTypePatch, nil
	case "minor":
		return BumpTypeMinor, nil
	case "major":
		return BumpTypeMajor, nil
	default:
		return BumpTypeNone, fmt.Errorf("invalid bump type: %s", s)
	}
}

// CalculateNextVersion calculates the next version based on bump type.
func CalculateNextVersion(current version.SemanticVersion, bump BumpType) version.SemanticVersion {
	switch bump {
	case BumpTypeMajor:
		return version.NewSemanticVersion(current.Major()+1, 0, 0)
	case BumpTypeMinor:
		return version.NewSemanticVersion(current.Major(), current.Minor()+1, 0)
	case BumpTypePatch:
		return version.NewSemanticVersion(current.Major(), current.Minor(), current.Patch()+1)
	default:
		return current
	}
}
