package monorepo

import "time"

// DomainEvent is the interface for all domain events.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// MonorepoReleaseCreated is emitted when a new monorepo release is created.
type MonorepoReleaseCreated struct {
	ReleaseID MonorepoReleaseID
	RepoID    string
	BaseRef   string
	HeadRef   string
	Strategy  MonorepoStrategy
	Timestamp time.Time
}

func (e MonorepoReleaseCreated) EventName() string     { return "monorepo.release.created" }
func (e MonorepoReleaseCreated) OccurredAt() time.Time { return e.Timestamp }

// PackageAddedToRelease is emitted when a package is added to a release.
type PackageAddedToRelease struct {
	ReleaseID   MonorepoReleaseID
	PackagePath string
	PackageType PackageType
	Timestamp   time.Time
}

func (e PackageAddedToRelease) EventName() string     { return "monorepo.package.added" }
func (e PackageAddedToRelease) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleasePlanned is emitted when a release is planned.
type MonorepoReleasePlanned struct {
	ReleaseID    MonorepoReleaseID
	PackageCount int
	Timestamp    time.Time
}

func (e MonorepoReleasePlanned) EventName() string     { return "monorepo.release.planned" }
func (e MonorepoReleasePlanned) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleaseVersioned is emitted when versions are set.
type MonorepoReleaseVersioned struct {
	ReleaseID MonorepoReleaseID
	Packages  []PackageVersionSummary
	Timestamp time.Time
}

func (e MonorepoReleaseVersioned) EventName() string     { return "monorepo.release.versioned" }
func (e MonorepoReleaseVersioned) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleaseNotesReady is emitted when release notes are ready.
type MonorepoReleaseNotesReady struct {
	ReleaseID MonorepoReleaseID
	Timestamp time.Time
}

func (e MonorepoReleaseNotesReady) EventName() string     { return "monorepo.release.notes_ready" }
func (e MonorepoReleaseNotesReady) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleaseApproved is emitted when a release is approved.
type MonorepoReleaseApproved struct {
	ReleaseID  MonorepoReleaseID
	ApprovedBy string
	Timestamp  time.Time
}

func (e MonorepoReleaseApproved) EventName() string     { return "monorepo.release.approved" }
func (e MonorepoReleaseApproved) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleasePublishing is emitted when publishing starts.
type MonorepoReleasePublishing struct {
	ReleaseID MonorepoReleaseID
	Timestamp time.Time
}

func (e MonorepoReleasePublishing) EventName() string     { return "monorepo.release.publishing" }
func (e MonorepoReleasePublishing) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleasePublished is emitted when a release is successfully published.
type MonorepoReleasePublished struct {
	ReleaseID MonorepoReleaseID
	Packages  []PackageVersionSummary
	Timestamp time.Time
}

func (e MonorepoReleasePublished) EventName() string     { return "monorepo.release.published" }
func (e MonorepoReleasePublished) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleaseFailed is emitted when a release fails.
type MonorepoReleaseFailed struct {
	ReleaseID MonorepoReleaseID
	Reason    string
	Timestamp time.Time
}

func (e MonorepoReleaseFailed) EventName() string     { return "monorepo.release.failed" }
func (e MonorepoReleaseFailed) OccurredAt() time.Time { return e.Timestamp }

// MonorepoReleaseCanceled is emitted when a release is canceled.
type MonorepoReleaseCanceled struct {
	ReleaseID MonorepoReleaseID
	Reason    string
	Timestamp time.Time
}

func (e MonorepoReleaseCanceled) EventName() string     { return "monorepo.release.canceled" }
func (e MonorepoReleaseCanceled) OccurredAt() time.Time { return e.Timestamp }

// PackageVersionBumped is emitted when a package version is bumped.
type PackageVersionBumped struct {
	ReleaseID      MonorepoReleaseID
	PackagePath    string
	CurrentVersion string
	NextVersion    string
	BumpType       BumpType
	Timestamp      time.Time
}

func (e PackageVersionBumped) EventName() string     { return "monorepo.package.version_bumped" }
func (e PackageVersionBumped) OccurredAt() time.Time { return e.Timestamp }

// PackageNotesGenerated is emitted when package notes are generated.
type PackageNotesGenerated struct {
	ReleaseID   MonorepoReleaseID
	PackagePath string
	Timestamp   time.Time
}

func (e PackageNotesGenerated) EventName() string     { return "monorepo.package.notes_generated" }
func (e PackageNotesGenerated) OccurredAt() time.Time { return e.Timestamp }

// PackagePublished is emitted when an individual package is published.
type PackagePublished struct {
	ReleaseID   MonorepoReleaseID
	PackagePath string
	Version     string
	TagName     string
	Timestamp   time.Time
}

func (e PackagePublished) EventName() string     { return "monorepo.package.published" }
func (e PackagePublished) OccurredAt() time.Time { return e.Timestamp }

// DependencyVersionUpdated is emitted when internal dependency versions are updated.
type DependencyVersionUpdated struct {
	ReleaseID      MonorepoReleaseID
	PackagePath    string
	DependencyPath string
	OldVersion     string
	NewVersion     string
	Timestamp      time.Time
}

func (e DependencyVersionUpdated) EventName() string     { return "monorepo.dependency.updated" }
func (e DependencyVersionUpdated) OccurredAt() time.Time { return e.Timestamp }
