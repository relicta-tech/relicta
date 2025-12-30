package monorepo

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewPackageRelease(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	if pkg.PackagePath != "packages/core" {
		t.Errorf("PackagePath = %v, want packages/core", pkg.PackagePath)
	}
	if pkg.PackageName != "core" {
		t.Errorf("PackageName = %v, want core", pkg.PackageName)
	}
	if pkg.PackageType != PackageTypeNPM {
		t.Errorf("PackageType = %v, want npm", pkg.PackageType)
	}
	if pkg.State != PackageStatePending {
		t.Errorf("State = %v, want pending", pkg.State)
	}
	if !pkg.CurrentVersion.Equal(version.NewSemanticVersion(1, 0, 0)) {
		t.Errorf("CurrentVersion = %v, want 1.0.0", pkg.CurrentVersion)
	}
}

func TestPackageRelease_Include(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	if err := pkg.Include(); err != nil {
		t.Fatalf("Include() error = %v", err)
	}
	if pkg.State != PackageStateIncluded {
		t.Errorf("State = %v, want included", pkg.State)
	}

	// Cannot include from included state
	if err := pkg.Include(); err == nil {
		t.Error("Include() should fail from included state")
	}
}

func TestPackageRelease_Exclude(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	if err := pkg.Exclude(); err != nil {
		t.Fatalf("Exclude() error = %v", err)
	}
	if pkg.State != PackageStateExcluded {
		t.Errorf("State = %v, want excluded", pkg.State)
	}

	// Cannot exclude from excluded state
	if err := pkg.Exclude(); err == nil {
		t.Error("Exclude() should fail from excluded state")
	}
}

func TestPackageRelease_Skip(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	if err := pkg.Skip(); err != nil {
		t.Fatalf("Skip() error = %v", err)
	}
	if pkg.State != PackageStateSkipped {
		t.Errorf("State = %v, want skipped", pkg.State)
	}

	// Cannot skip from skipped state
	if err := pkg.Skip(); err == nil {
		t.Error("Skip() should fail from skipped state")
	}
}

func TestPackageRelease_MarkReleased(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = pkg.SetVersion(version.NewSemanticVersion(1, 1, 0), BumpTypeMinor)

	if err := pkg.MarkReleased(); err != nil {
		t.Fatalf("MarkReleased() error = %v", err)
	}
	if pkg.State != PackageStateReleased {
		t.Errorf("State = %v, want released", pkg.State)
	}
	if !pkg.CurrentVersion.Equal(version.NewSemanticVersion(1, 1, 0)) {
		t.Errorf("CurrentVersion should be updated to NextVersion")
	}
}

func TestPackageRelease_SetVersion(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	nextVer := version.NewSemanticVersion(2, 0, 0)
	if err := pkg.SetVersion(nextVer, BumpTypeMajor); err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}

	if !pkg.NextVersion.Equal(nextVer) {
		t.Errorf("NextVersion = %v, want %v", pkg.NextVersion, nextVer)
	}
	if pkg.BumpType != BumpTypeMajor {
		t.Errorf("BumpType = %v, want major", pkg.BumpType)
	}
	// Should auto-include when setting version
	if pkg.State != PackageStateIncluded {
		t.Errorf("State = %v, want included (auto-included on version set)", pkg.State)
	}
}

func TestPackageRelease_SetNotes(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	pkg.SetNotes("## Changes\n- New feature")
	if pkg.Notes != "## Changes\n- New feature" {
		t.Errorf("Notes = %v", pkg.Notes)
	}
}

func TestPackageRelease_SetTagName(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	pkg.SetTagName("core-v1.1.0")
	if pkg.TagName != "core-v1.1.0" {
		t.Errorf("TagName = %v, want core-v1.1.0", pkg.TagName)
	}
}

func TestPackageRelease_AddChangedFile(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	pkg.AddChangedFile("packages/core/src/index.ts")
	pkg.AddChangedFile("packages/core/package.json")

	if len(pkg.ChangedFiles) != 2 {
		t.Errorf("ChangedFiles length = %d, want 2", len(pkg.ChangedFiles))
	}
}

func TestPackageRelease_Dependencies(t *testing.T) {
	pkg := NewPackageRelease("packages/cli", "cli", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	pkg.AddDependency("packages/core")
	pkg.AddDependency("packages/utils")
	pkg.AddDependency("packages/core") // Duplicate

	if len(pkg.Dependencies) != 2 {
		t.Errorf("Dependencies length = %d, want 2", len(pkg.Dependencies))
	}
}

func TestPackageRelease_Dependents(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	pkg.AddDependent("packages/cli")
	pkg.AddDependent("packages/api")
	pkg.AddDependent("packages/cli") // Duplicate

	if len(pkg.Dependents) != 2 {
		t.Errorf("Dependents length = %d, want 2", len(pkg.Dependents))
	}
	if !pkg.HasDependents() {
		t.Error("HasDependents() should return true")
	}
}

func TestPackageRelease_HasChanges(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	if pkg.HasChanges() {
		t.Error("HasChanges() should return false initially")
	}

	pkg.AddChangedFile("some/file.txt")
	if !pkg.HasChanges() {
		t.Error("HasChanges() should return true after adding file")
	}

	pkg2 := NewPackageRelease("packages/utils", "utils", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	pkg2.CommitCount = 3
	if !pkg2.HasChanges() {
		t.Error("HasChanges() should return true with commit count > 0")
	}
}

func TestPackageRelease_IsIncluded(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	if pkg.IsIncluded() {
		t.Error("IsIncluded() should return false initially")
	}

	_ = pkg.Include()
	if !pkg.IsIncluded() {
		t.Error("IsIncluded() should return true after Include()")
	}
}

func TestPackageRelease_IsReleased(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))
	_ = pkg.SetVersion(version.NewSemanticVersion(1, 1, 0), BumpTypeMinor)

	if pkg.IsReleased() {
		t.Error("IsReleased() should return false initially")
	}

	_ = pkg.MarkReleased()
	if !pkg.IsReleased() {
		t.Error("IsReleased() should return true after MarkReleased()")
	}
}

func TestPackageRelease_GetVersionDiff(t *testing.T) {
	pkg := NewPackageRelease("packages/core", "core", PackageTypeNPM, version.NewSemanticVersion(1, 0, 0))

	// Without next version
	diff := pkg.GetVersionDiff()
	if diff != "1.0.0" {
		t.Errorf("GetVersionDiff() = %v, want 1.0.0", diff)
	}

	// With next version
	_ = pkg.SetVersion(version.NewSemanticVersion(1, 1, 0), BumpTypeMinor)
	diff = pkg.GetVersionDiff()
	if diff != "1.0.0 -> 1.1.0" {
		t.Errorf("GetVersionDiff() = %v, want '1.0.0 -> 1.1.0'", diff)
	}
}

func TestPackageTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected PackageType
	}{
		{"npm", PackageTypeNPM},
		{"cargo", PackageTypeCargo},
		{"python", PackageTypePython},
		{"go_module", PackageTypeGoModule},
		{"go", PackageTypeGoModule},
		{"maven", PackageTypeMaven},
		{"gradle", PackageTypeGradle},
		{"composer", PackageTypeComposer},
		{"gem", PackageTypeGem},
		{"ruby", PackageTypeGem},
		{"nuget", PackageTypeNuGet},
		{"unknown", PackageTypeDirectory},
		{"", PackageTypeDirectory},
	}

	for _, tt := range tests {
		result := PackageTypeFromString(tt.input)
		if result != tt.expected {
			t.Errorf("PackageTypeFromString(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestPackageReleaseState_Values(t *testing.T) {
	tests := []struct {
		state    PackageReleaseState
		expected string
	}{
		{PackageStatePending, "pending"},
		{PackageStateIncluded, "included"},
		{PackageStateExcluded, "excluded"},
		{PackageStateReleased, "released"},
		{PackageStateSkipped, "skipped"},
	}

	for _, tt := range tests {
		if string(tt.state) != tt.expected {
			t.Errorf("PackageReleaseState %v = %q, want %q", tt.state, string(tt.state), tt.expected)
		}
	}
}
