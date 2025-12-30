package monorepo

import (
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

// PackageReleaseState represents the state of an individual package release.
type PackageReleaseState string

const (
	// PackageStatePending means the package is pending analysis.
	PackageStatePending PackageReleaseState = "pending"
	// PackageStateIncluded means the package will be included in this release.
	PackageStateIncluded PackageReleaseState = "included"
	// PackageStateExcluded means the package will not be included in this release.
	PackageStateExcluded PackageReleaseState = "excluded"
	// PackageStateReleased means the package has been released.
	PackageStateReleased PackageReleaseState = "released"
	// PackageStateSkipped means the package was skipped (no changes).
	PackageStateSkipped PackageReleaseState = "skipped"
)

// PackageType identifies the type of package for versioning purposes.
type PackageType string

const (
	PackageTypeNPM       PackageType = "npm"
	PackageTypeCargo     PackageType = "cargo"
	PackageTypePython    PackageType = "python"
	PackageTypeGoModule  PackageType = "go_module"
	PackageTypeMaven     PackageType = "maven"
	PackageTypeGradle    PackageType = "gradle"
	PackageTypeComposer  PackageType = "composer"
	PackageTypeGem       PackageType = "gem"
	PackageTypeNuGet     PackageType = "nuget"
	PackageTypeDirectory PackageType = "directory"
)

// PackageRelease represents an individual package within a monorepo release.
type PackageRelease struct {
	// PackagePath is the path to the package relative to repo root.
	PackagePath string
	// PackageName is the human-readable name of the package.
	PackageName string
	// PackageType identifies the package type (npm, cargo, etc.).
	PackageType PackageType
	// CurrentVersion is the current version of the package.
	CurrentVersion version.SemanticVersion
	// NextVersion is the version this package will be released as.
	NextVersion version.SemanticVersion
	// BumpType is the type of version bump (patch, minor, major).
	BumpType BumpType
	// State is the current state of this package in the release.
	State PackageReleaseState
	// ChangedFiles lists files changed in this package.
	ChangedFiles []string
	// CommitCount is the number of commits affecting this package.
	CommitCount int
	// Notes contains release notes for this package.
	Notes string
	// ReleaseGroup is the name of the release group this package belongs to.
	ReleaseGroup string
	// TagName is the git tag that will be created for this package.
	TagName string
	// Private indicates if this package should not be published.
	Private bool
	// Dependencies lists internal package dependencies.
	Dependencies []string
	// Dependents lists packages that depend on this one.
	Dependents []string
	// RiskScore is the CGP risk assessment score (0.0-1.0).
	RiskScore float64
	// UpdatedAt is when this package was last updated.
	UpdatedAt time.Time
}

// NewPackageRelease creates a new package release.
func NewPackageRelease(path, name string, pkgType PackageType, currentVersion version.SemanticVersion) *PackageRelease {
	return &PackageRelease{
		PackagePath:    path,
		PackageName:    name,
		PackageType:    pkgType,
		CurrentVersion: currentVersion,
		NextVersion:    version.SemanticVersion{},
		BumpType:       BumpTypeNone,
		State:          PackageStatePending,
		ChangedFiles:   make([]string, 0),
		Dependencies:   make([]string, 0),
		Dependents:     make([]string, 0),
		UpdatedAt:      time.Now(),
	}
}

// Include marks the package for inclusion in the release.
func (p *PackageRelease) Include() error {
	if p.State != PackageStatePending && p.State != PackageStateExcluded {
		return fmt.Errorf("cannot include package in state %s", p.State)
	}
	p.State = PackageStateIncluded
	p.UpdatedAt = time.Now()
	return nil
}

// Exclude marks the package for exclusion from the release.
func (p *PackageRelease) Exclude() error {
	if p.State != PackageStatePending && p.State != PackageStateIncluded {
		return fmt.Errorf("cannot exclude package in state %s", p.State)
	}
	p.State = PackageStateExcluded
	p.UpdatedAt = time.Now()
	return nil
}

// Skip marks the package as skipped (no changes to release).
func (p *PackageRelease) Skip() error {
	if p.State != PackageStatePending {
		return fmt.Errorf("cannot skip package in state %s", p.State)
	}
	p.State = PackageStateSkipped
	p.UpdatedAt = time.Now()
	return nil
}

// MarkReleased marks the package as released.
func (p *PackageRelease) MarkReleased() error {
	if p.State != PackageStateIncluded {
		return fmt.Errorf("cannot mark package as released in state %s", p.State)
	}
	p.State = PackageStateReleased
	p.CurrentVersion = p.NextVersion
	p.UpdatedAt = time.Now()
	return nil
}

// SetVersion sets the next version for this package.
func (p *PackageRelease) SetVersion(next version.SemanticVersion, bump BumpType) error {
	if p.State != PackageStatePending && p.State != PackageStateIncluded {
		return fmt.Errorf("cannot set version for package in state %s", p.State)
	}

	p.NextVersion = next
	p.BumpType = bump
	p.UpdatedAt = time.Now()

	// Automatically include if we're setting a version
	if p.State == PackageStatePending && bump != BumpTypeNone {
		p.State = PackageStateIncluded
	}

	return nil
}

// SetNotes sets release notes for this package.
func (p *PackageRelease) SetNotes(notes string) {
	p.Notes = notes
	p.UpdatedAt = time.Now()
}

// SetTagName sets the git tag name for this package.
func (p *PackageRelease) SetTagName(tagName string) {
	p.TagName = tagName
	p.UpdatedAt = time.Now()
}

// AddChangedFile adds a changed file to this package.
func (p *PackageRelease) AddChangedFile(file string) {
	p.ChangedFiles = append(p.ChangedFiles, file)
	p.UpdatedAt = time.Now()
}

// AddDependency adds an internal dependency.
func (p *PackageRelease) AddDependency(dep string) {
	for _, existing := range p.Dependencies {
		if existing == dep {
			return
		}
	}
	p.Dependencies = append(p.Dependencies, dep)
	p.UpdatedAt = time.Now()
}

// AddDependent adds a package that depends on this one.
func (p *PackageRelease) AddDependent(dependent string) {
	for _, existing := range p.Dependents {
		if existing == dependent {
			return
		}
	}
	p.Dependents = append(p.Dependents, dependent)
	p.UpdatedAt = time.Now()
}

// HasChanges returns true if the package has any changes.
func (p *PackageRelease) HasChanges() bool {
	return len(p.ChangedFiles) > 0 || p.CommitCount > 0
}

// IsIncluded returns true if the package is included in the release.
func (p *PackageRelease) IsIncluded() bool {
	return p.State == PackageStateIncluded
}

// IsReleased returns true if the package has been released.
func (p *PackageRelease) IsReleased() bool {
	return p.State == PackageStateReleased
}

// HasDependents returns true if other packages depend on this one.
func (p *PackageRelease) HasDependents() bool {
	return len(p.Dependents) > 0
}

// GetVersionDiff returns the version change as a string.
func (p *PackageRelease) GetVersionDiff() string {
	if p.NextVersion.IsZero() {
		return p.CurrentVersion.String()
	}
	return fmt.Sprintf("%s -> %s", p.CurrentVersion.String(), p.NextVersion.String())
}

// PackageTypeFromString converts a string to PackageType.
func PackageTypeFromString(s string) PackageType {
	switch s {
	case "npm":
		return PackageTypeNPM
	case "cargo":
		return PackageTypeCargo
	case "python":
		return PackageTypePython
	case "go_module", "go":
		return PackageTypeGoModule
	case "maven":
		return PackageTypeMaven
	case "gradle":
		return PackageTypeGradle
	case "composer":
		return PackageTypeComposer
	case "gem", "ruby":
		return PackageTypeGem
	case "nuget":
		return PackageTypeNuGet
	default:
		return PackageTypeDirectory
	}
}
