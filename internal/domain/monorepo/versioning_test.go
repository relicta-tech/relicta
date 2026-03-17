package monorepo

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestPackageVersionInfo_FormatTag(t *testing.T) {
	tests := []struct {
		name    string
		info    PackageVersionInfo
		version version.SemanticVersion
		want    string
	}{
		{
			name: "slash pattern - Go module style",
			info: PackageVersionInfo{
				PackagePath: "pkg/core",
				PackageName: "github.com/test/pkg/core",
				TagPattern:  TagPatternSlash,
			},
			version: version.NewSemanticVersion(1, 2, 3),
			want:    "pkg/core/v1.2.3",
		},
		{
			name: "at pattern - npm style",
			info: PackageVersionInfo{
				PackagePath: "packages/core",
				PackageName: "@scope/core",
				TagPattern:  TagPatternAt,
			},
			version: version.NewSemanticVersion(2, 0, 0),
			want:    "@scope/core@2.0.0",
		},
		{
			name: "prefix pattern - generic style",
			info: PackageVersionInfo{
				PackagePath: "packages/core",
				PackageName: "core",
				TagPattern:  TagPatternPrefix,
			},
			version: version.NewSemanticVersion(0, 5, 1),
			want:    "core-v0.5.1",
		},
		{
			name: "default pattern (falls back to slash)",
			info: PackageVersionInfo{
				PackagePath: "services/api",
				PackageName: "api",
				TagPattern:  "",
			},
			version: version.NewSemanticVersion(1, 0, 0),
			want:    "services/api/v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.FormatTag(tt.version)
			if got != tt.want {
				t.Errorf("FormatTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTagVersion(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		pattern TagPattern
		want    version.SemanticVersion
		wantErr bool
	}{
		{
			name:    "slash pattern",
			tag:     "pkg/core/v1.2.3",
			pattern: TagPatternSlash,
			want:    version.NewSemanticVersion(1, 2, 3),
		},
		{
			name:    "at pattern",
			tag:     "@scope/core@2.0.0",
			pattern: TagPatternAt,
			want:    version.NewSemanticVersion(2, 0, 0),
		},
		{
			name:    "prefix pattern",
			tag:     "core-v0.5.1",
			pattern: TagPatternPrefix,
			want:    version.NewSemanticVersion(0, 5, 1),
		},
		{
			name:    "invalid slash tag",
			tag:     "invalid",
			pattern: TagPatternSlash,
			want:    version.NewSemanticVersion(0, 0, 0),
			wantErr: true,
		},
		{
			name:    "invalid at tag - no @",
			tag:     "noseparator",
			pattern: TagPatternAt,
			want:    version.NewSemanticVersion(0, 0, 0),
			wantErr: true,
		},
		{
			name:    "invalid prefix tag",
			tag:     "no-version-here",
			pattern: TagPatternPrefix,
			want:    version.NewSemanticVersion(0, 0, 0),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTagVersion(tt.tag, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTagVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseTagVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionPlan_Basic(t *testing.T) {
	plan := NewVersionPlan(StrategyIndependent)

	entry1 := &VersionPlanEntry{
		PackagePath:    "packages/core",
		PackageName:    "core",
		CurrentVersion: version.NewSemanticVersion(1, 0, 0),
		NextVersion:    version.NewSemanticVersion(1, 1, 0),
		BumpType:       BumpTypeMinor,
		IsDirectChange: true,
	}
	entry2 := &VersionPlanEntry{
		PackagePath:    "packages/utils",
		PackageName:    "utils",
		CurrentVersion: version.NewSemanticVersion(0, 5, 0),
		BumpType:       BumpTypeNone,
	}

	plan.AddEntry(entry1)
	plan.AddEntry(entry2)

	// GetEntry
	if got := plan.GetEntry("packages/core"); got != entry1 {
		t.Error("GetEntry did not return expected entry")
	}
	if got := plan.GetEntry("nonexistent"); got != nil {
		t.Error("GetEntry for nonexistent should return nil")
	}

	// AffectedPackages
	affected := plan.AffectedPackages()
	if len(affected) != 1 {
		t.Errorf("AffectedPackages() len = %d, want 1", len(affected))
	}
	if affected[0] != "packages/core" {
		t.Errorf("AffectedPackages()[0] = %s, want packages/core", affected[0])
	}
}

func TestVersionCalculator_IndependentStrategy(t *testing.T) {
	vc := NewVersionCalculator(TagPatternSlash)

	packages := []*PackageRelease{
		{
			PackagePath:    "packages/core",
			PackageName:    "core",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypeMinor,
			ChangedFiles:   []string{"packages/core/src/main.go"},
			CommitCount:    3,
		},
		{
			PackagePath:    "packages/utils",
			PackageName:    "utils",
			CurrentVersion: version.NewSemanticVersion(0, 5, 0),
			BumpType:       BumpTypePatch,
			ChangedFiles:   []string{"packages/utils/helper.go"},
			CommitCount:    1,
		},
		{
			PackagePath:    "packages/docs",
			PackageName:    "docs",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypeNone,
		},
	}

	plan := vc.CalculateVersionPlan(packages, StrategyIndependent, nil)

	// Core should get minor bump
	coreEntry := plan.GetEntry("packages/core")
	if coreEntry == nil {
		t.Fatal("core entry not found")
	}
	if coreEntry.NextVersion != version.NewSemanticVersion(1, 1, 0) {
		t.Errorf("core NextVersion = %s, want 1.1.0", coreEntry.NextVersion)
	}
	if coreEntry.TagName != "packages/core/v1.1.0" {
		t.Errorf("core TagName = %s, want packages/core/v1.1.0", coreEntry.TagName)
	}

	// Utils should get patch bump
	utilsEntry := plan.GetEntry("packages/utils")
	if utilsEntry == nil {
		t.Fatal("utils entry not found")
	}
	if utilsEntry.NextVersion != version.NewSemanticVersion(0, 5, 1) {
		t.Errorf("utils NextVersion = %s, want 0.5.1", utilsEntry.NextVersion)
	}

	// Docs should not be affected
	docsEntry := plan.GetEntry("packages/docs")
	if docsEntry == nil {
		t.Fatal("docs entry not found")
	}
	if docsEntry.BumpType != BumpTypeNone {
		t.Errorf("docs BumpType = %s, want none", docsEntry.BumpType)
	}

	// Check affected packages
	affected := plan.AffectedPackages()
	if len(affected) != 2 {
		t.Errorf("affected packages = %d, want 2", len(affected))
	}
}

func TestVersionCalculator_DependencyPropagation(t *testing.T) {
	vc := NewVersionCalculator(TagPatternSlash)

	// core has changes, utils depends on core but has no direct changes
	packages := []*PackageRelease{
		{
			PackagePath:    "packages/core",
			PackageName:    "core",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypeMinor,
			ChangedFiles:   []string{"packages/core/src/main.go"},
			CommitCount:    2,
			Dependencies:   []string{},
			Dependents:     []string{"packages/utils"},
		},
		{
			PackagePath:    "packages/utils",
			PackageName:    "utils",
			CurrentVersion: version.NewSemanticVersion(0, 5, 0),
			BumpType:       BumpTypeNone,
			Dependencies:   []string{"packages/core"},
			Dependents:     []string{},
		},
	}

	plan := vc.CalculateVersionPlan(packages, StrategyIndependent, nil)

	// Utils should get a patch bump due to dependency on core
	utilsEntry := plan.GetEntry("packages/utils")
	if utilsEntry == nil {
		t.Fatal("utils entry not found")
	}
	if utilsEntry.BumpType != BumpTypePatch {
		t.Errorf("utils BumpType = %s, want patch (propagated from core)", utilsEntry.BumpType)
	}
	if !utilsEntry.IsDependencyChange {
		t.Error("utils should be marked as dependency change")
	}
	if len(utilsEntry.AffectedDependencies) != 1 || utilsEntry.AffectedDependencies[0] != "packages/core" {
		t.Errorf("utils AffectedDependencies = %v, want [packages/core]", utilsEntry.AffectedDependencies)
	}
}

func TestVersionCalculator_LockstepStrategy(t *testing.T) {
	vc := NewVersionCalculator(TagPatternSlash)

	packages := []*PackageRelease{
		{
			PackagePath:    "packages/core",
			PackageName:    "core",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypeMinor,
			ChangedFiles:   []string{"packages/core/main.go"},
			CommitCount:    1,
		},
		{
			PackagePath:    "packages/utils",
			PackageName:    "utils",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypePatch,
			ChangedFiles:   []string{"packages/utils/helper.go"},
			CommitCount:    1,
		},
		{
			PackagePath:    "packages/docs",
			PackageName:    "docs",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypeNone,
		},
	}

	plan := vc.CalculateVersionPlan(packages, StrategyLockstep, nil)

	// All packages should have the same version (minor bump = highest)
	expectedVersion := version.NewSemanticVersion(1, 1, 0)
	for _, entry := range plan.Entries {
		if entry.NextVersion != expectedVersion {
			t.Errorf("package %s NextVersion = %s, want %s (lockstep)", entry.PackagePath, entry.NextVersion, expectedVersion)
		}
		if entry.BumpType != BumpTypeMinor {
			t.Errorf("package %s BumpType = %s, want minor (lockstep)", entry.PackagePath, entry.BumpType)
		}
	}
}

func TestVersionCalculator_HybridStrategy(t *testing.T) {
	vc := NewVersionCalculator(TagPatternSlash)

	packages := []*PackageRelease{
		{
			PackagePath:    "packages/core",
			PackageName:    "core",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypeMajor,
			ChangedFiles:   []string{"packages/core/main.go"},
			CommitCount:    1,
		},
		{
			PackagePath:    "packages/utils",
			PackageName:    "utils",
			CurrentVersion: version.NewSemanticVersion(1, 0, 0),
			BumpType:       BumpTypePatch,
			ChangedFiles:   []string{"packages/utils/helper.go"},
			CommitCount:    1,
		},
		{
			PackagePath:    "plugins/auth",
			PackageName:    "auth",
			CurrentVersion: version.NewSemanticVersion(0, 5, 0),
			BumpType:       BumpTypeMinor,
			ChangedFiles:   []string{"plugins/auth/handler.go"},
			CommitCount:    1,
		},
	}

	releaseGroups := map[string][]string{
		"core-group": {"packages/core", "packages/utils"},
	}

	plan := vc.CalculateVersionPlan(packages, StrategyHybrid, releaseGroups)

	// Core and utils should share the highest bump in their group (major)
	coreEntry := plan.GetEntry("packages/core")
	utilsEntry := plan.GetEntry("packages/utils")

	if coreEntry.NextVersion != utilsEntry.NextVersion {
		t.Errorf("core (%s) and utils (%s) should have same version in hybrid mode",
			coreEntry.NextVersion, utilsEntry.NextVersion)
	}

	expectedGroupVersion := version.NewSemanticVersion(2, 0, 0)
	if coreEntry.NextVersion != expectedGroupVersion {
		t.Errorf("core-group version = %s, want %s", coreEntry.NextVersion, expectedGroupVersion)
	}

	// Auth plugin should be independent (not in any group)
	authEntry := plan.GetEntry("plugins/auth")
	if authEntry.NextVersion != version.NewSemanticVersion(0, 6, 0) {
		t.Errorf("auth NextVersion = %s, want 0.6.0 (independent)", authEntry.NextVersion)
	}
}

func TestReleaseOrder_NoDependencies(t *testing.T) {
	packages := []*PackageRelease{
		{PackagePath: "packages/b", Dependencies: []string{}},
		{PackagePath: "packages/a", Dependencies: []string{}},
		{PackagePath: "packages/c", Dependencies: []string{}},
	}

	order := ReleaseOrder(packages)

	if len(order) != 3 {
		t.Fatalf("order length = %d, want 3", len(order))
	}

	// Should be sorted alphabetically when no dependencies
	expected := []string{"packages/a", "packages/b", "packages/c"}
	for i, pkg := range order {
		if pkg != expected[i] {
			t.Errorf("order[%d] = %s, want %s", i, pkg, expected[i])
		}
	}
}

func TestReleaseOrder_WithDependencies(t *testing.T) {
	// app depends on utils, utils depends on core
	packages := []*PackageRelease{
		{PackagePath: "packages/app", Dependencies: []string{"packages/utils"}},
		{PackagePath: "packages/utils", Dependencies: []string{"packages/core"}},
		{PackagePath: "packages/core", Dependencies: []string{}},
	}

	order := ReleaseOrder(packages)

	if len(order) != 3 {
		t.Fatalf("order length = %d, want 3", len(order))
	}

	// Core must come before utils, utils must come before app
	coreIdx, utilsIdx, appIdx := -1, -1, -1
	for i, pkg := range order {
		switch pkg {
		case "packages/core":
			coreIdx = i
		case "packages/utils":
			utilsIdx = i
		case "packages/app":
			appIdx = i
		}
	}

	if coreIdx >= utilsIdx {
		t.Errorf("core (idx %d) should come before utils (idx %d)", coreIdx, utilsIdx)
	}
	if utilsIdx >= appIdx {
		t.Errorf("utils (idx %d) should come before app (idx %d)", utilsIdx, appIdx)
	}
}

func TestReleaseOrder_DiamondDependency(t *testing.T) {
	// app depends on both a and b, both depend on core
	packages := []*PackageRelease{
		{PackagePath: "packages/app", Dependencies: []string{"packages/a", "packages/b"}},
		{PackagePath: "packages/a", Dependencies: []string{"packages/core"}},
		{PackagePath: "packages/b", Dependencies: []string{"packages/core"}},
		{PackagePath: "packages/core", Dependencies: []string{}},
	}

	order := ReleaseOrder(packages)

	if len(order) != 4 {
		t.Fatalf("order length = %d, want 4", len(order))
	}

	// Core must come first, app must come last
	if order[0] != "packages/core" {
		t.Errorf("first package = %s, want packages/core", order[0])
	}
	if order[3] != "packages/app" {
		t.Errorf("last package = %s, want packages/app", order[3])
	}
}

func TestCompareBumpTypes(t *testing.T) {
	tests := []struct {
		name string
		a, b BumpType
		want int
	}{
		{"none < patch", BumpTypeNone, BumpTypePatch, -1},
		{"patch < minor", BumpTypePatch, BumpTypeMinor, -1},
		{"minor < major", BumpTypeMinor, BumpTypeMajor, -1},
		{"major > minor", BumpTypeMajor, BumpTypeMinor, 1},
		{"patch == patch", BumpTypePatch, BumpTypePatch, 0},
		{"none == none", BumpTypeNone, BumpTypeNone, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareBumpTypes(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareBumpTypes(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
