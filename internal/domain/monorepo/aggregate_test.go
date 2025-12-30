package monorepo

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewMonorepoRelease(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)

	if rel.ID == "" {
		t.Error("ID should not be empty")
	}
	if rel.RepoID != "github.com/owner/repo" {
		t.Errorf("RepoID = %v, want github.com/owner/repo", rel.RepoID)
	}
	if rel.BaseRef != "v1.0.0" {
		t.Errorf("BaseRef = %v, want v1.0.0", rel.BaseRef)
	}
	if rel.HeadRef != "HEAD" {
		t.Errorf("HeadRef = %v, want HEAD", rel.HeadRef)
	}
	if rel.State != StateDraft {
		t.Errorf("State = %v, want draft", rel.State)
	}
	if rel.Strategy != StrategyIndependent {
		t.Errorf("Strategy = %v, want independent", rel.Strategy)
	}
	if len(rel.Events) != 1 {
		t.Errorf("Events length = %d, want 1", len(rel.Events))
	}
	if rel.Events[0].EventName() != "monorepo.release.created" {
		t.Errorf("First event = %v, want monorepo.release.created", rel.Events[0].EventName())
	}
}

func TestMonorepoRelease_AddPackage(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)

	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	err := rel.AddPackage(pkg)
	if err != nil {
		t.Fatalf("AddPackage() error = %v", err)
	}

	if len(rel.Packages) != 1 {
		t.Errorf("Packages length = %d, want 1", len(rel.Packages))
	}

	// Adding duplicate should fail
	pkg2 := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	err = rel.AddPackage(pkg2)
	if err == nil {
		t.Error("AddPackage() should fail for duplicate package")
	}
}

func TestMonorepoRelease_StateTransitions(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)

	// Add a package
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = rel.AddPackage(pkg)

	// Plan
	if err := rel.Plan(); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if rel.State != StatePlanned {
		t.Errorf("State after Plan() = %v, want planned", rel.State)
	}

	// Set version on package
	pkg.SetVersion(version.NewSemanticVersion(1, 1, 0), BumpTypeMinor)

	// Set versions
	if err := rel.SetVersions(); err != nil {
		t.Fatalf("SetVersions() error = %v", err)
	}
	if rel.State != StateVersioned {
		t.Errorf("State after SetVersions() = %v, want versioned", rel.State)
	}

	// Generate notes
	if err := rel.GenerateNotes(); err != nil {
		t.Fatalf("GenerateNotes() error = %v", err)
	}
	if rel.State != StateNotesReady {
		t.Errorf("State after GenerateNotes() = %v, want notes_ready", rel.State)
	}

	// Approve
	if err := rel.Approve("test-user"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if rel.State != StateApproved {
		t.Errorf("State after Approve() = %v, want approved", rel.State)
	}
	if rel.ApprovedBy != "test-user" {
		t.Errorf("ApprovedBy = %v, want test-user", rel.ApprovedBy)
	}

	// Start publish
	if err := rel.StartPublish(); err != nil {
		t.Fatalf("StartPublish() error = %v", err)
	}
	if rel.State != StatePublishing {
		t.Errorf("State after StartPublish() = %v, want publishing", rel.State)
	}

	// Complete
	if err := rel.Complete(); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if rel.State != StatePublished {
		t.Errorf("State after Complete() = %v, want published", rel.State)
	}
	if rel.PublishedAt == nil {
		t.Error("PublishedAt should be set")
	}
}

func TestMonorepoRelease_InvalidTransitions(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)

	// Cannot plan without packages
	if err := rel.Plan(); err == nil {
		t.Error("Plan() should fail without packages")
	}

	// Add package and plan
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = rel.AddPackage(pkg)
	_ = rel.Plan()

	// Cannot plan again
	if err := rel.Plan(); err == nil {
		t.Error("Plan() should fail in planned state")
	}

	// Cannot approve from planned state
	if err := rel.Approve("user"); err == nil {
		t.Error("Approve() should fail from planned state")
	}
}

func TestMonorepoRelease_Fail(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = rel.AddPackage(pkg)
	_ = rel.Plan()

	if err := rel.Fail("something went wrong"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	if rel.State != StateFailed {
		t.Errorf("State = %v, want failed", rel.State)
	}
	if rel.FailureReason != "something went wrong" {
		t.Errorf("FailureReason = %v, want 'something went wrong'", rel.FailureReason)
	}

	// Cannot fail already failed release
	if err := rel.Fail("another reason"); err == nil {
		t.Error("Fail() should fail on already failed release")
	}
}

func TestMonorepoRelease_Cancel(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = rel.AddPackage(pkg)
	_ = rel.Plan()

	if err := rel.Cancel("user canceled"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if rel.State != StateCanceled {
		t.Errorf("State = %v, want canceled", rel.State)
	}
}

func TestMonorepoRelease_GetIncludedPackages(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)

	pkg1 := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	pkg2 := NewPackageRelease("packages/cli", "cli", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	pkg3 := NewPackageRelease("packages/utils", "utils", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	_ = rel.AddPackage(pkg1)
	_ = rel.AddPackage(pkg2)
	_ = rel.AddPackage(pkg3)

	// Mark some as included
	_ = pkg1.Include()
	_ = pkg2.Include()
	_ = pkg3.Skip()

	included := rel.GetIncludedPackages()
	if len(included) != 2 {
		t.Errorf("GetIncludedPackages() length = %d, want 2", len(included))
	}
}

func TestMonorepoRelease_GetPackageByPath(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)

	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = rel.AddPackage(pkg)

	found := rel.GetPackageByPath("packages/core")
	if found == nil {
		t.Fatal("GetPackageByPath() should find existing package")
	}
	if found.PackageName != "core" {
		t.Errorf("PackageName = %v, want core", found.PackageName)
	}

	notFound := rel.GetPackageByPath("packages/nonexistent")
	if notFound != nil {
		t.Error("GetPackageByPath() should return nil for nonexistent package")
	}
}

func TestMonorepoRelease_FlushEvents(t *testing.T) {
	rel := NewMonorepoRelease("github.com/owner/repo", "v1.0.0", "HEAD", StrategyIndependent)
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = rel.AddPackage(pkg)

	events := rel.FlushEvents()
	if len(events) != 2 { // Created + PackageAdded
		t.Errorf("FlushEvents() returned %d events, want 2", len(events))
	}

	// After flush, events should be empty
	events = rel.FlushEvents()
	if len(events) != 0 {
		t.Errorf("FlushEvents() returned %d events after flush, want 0", len(events))
	}
}

func TestMonorepoRelease_IsTerminal(t *testing.T) {
	tests := []struct {
		state    MonorepoReleaseState
		terminal bool
	}{
		{StateDraft, false},
		{StatePlanned, false},
		{StateVersioned, false},
		{StateNotesReady, false},
		{StateApproved, false},
		{StatePublishing, false},
		{StatePublished, true},
		{StateFailed, true},
		{StateCanceled, true},
	}

	for _, tt := range tests {
		rel := NewMonorepoRelease("repo", "v1", "HEAD", StrategyIndependent)
		rel.State = tt.state

		if rel.IsTerminal() != tt.terminal {
			t.Errorf("IsTerminal() for %v = %v, want %v", tt.state, rel.IsTerminal(), tt.terminal)
		}
	}
}

func TestCalculateNextVersion(t *testing.T) {
	current := version.NewSemanticVersion(1, 2, 3)

	tests := []struct {
		bump     BumpType
		expected version.SemanticVersion
	}{
		{BumpTypeMajor, version.NewSemanticVersion(2, 0, 0)},
		{BumpTypeMinor, version.NewSemanticVersion(1, 3, 0)},
		{BumpTypePatch, version.NewSemanticVersion(1, 2, 4)},
		{BumpTypeNone, version.NewSemanticVersion(1, 2, 3)},
	}

	for _, tt := range tests {
		result := CalculateNextVersion(current, tt.bump)
		if !result.Equal(tt.expected) {
			t.Errorf("CalculateNextVersion(%v, %v) = %v, want %v", current, tt.bump, result, tt.expected)
		}
	}
}

func TestParseBumpType(t *testing.T) {
	tests := []struct {
		input    string
		expected BumpType
		hasError bool
	}{
		{"none", BumpTypeNone, false},
		{"", BumpTypeNone, false},
		{"patch", BumpTypePatch, false},
		{"minor", BumpTypeMinor, false},
		{"major", BumpTypeMajor, false},
		{"invalid", BumpTypeNone, true},
	}

	for _, tt := range tests {
		result, err := ParseBumpType(tt.input)
		if tt.hasError && err == nil {
			t.Errorf("ParseBumpType(%q) should return error", tt.input)
		}
		if !tt.hasError && err != nil {
			t.Errorf("ParseBumpType(%q) error = %v", tt.input, err)
		}
		if !tt.hasError && result != tt.expected {
			t.Errorf("ParseBumpType(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
