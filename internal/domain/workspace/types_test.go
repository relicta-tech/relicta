package workspace

import (
	"testing"
)

func TestWorkspaceType_String(t *testing.T) {
	tests := []struct {
		wt   WorkspaceType
		want string
	}{
		{WorkspaceTypeNone, "none"},
		{WorkspaceTypePnpm, "pnpm"},
		{WorkspaceTypeNpm, "npm"},
		{WorkspaceTypeYarn, "yarn"},
		{WorkspaceTypeLerna, "lerna"},
		{WorkspaceTypeNx, "nx"},
		{WorkspaceTypeTurborepo, "turborepo"},
		{WorkspaceTypeGoModule, "go_module"},
		{WorkspaceTypeCargo, "cargo"},
		{WorkspaceTypeMaven, "maven"},
		{WorkspaceTypeGradle, "gradle"},
		{WorkspaceTypeCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.wt.String(); got != tt.want {
				t.Errorf("WorkspaceType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspaceType_IsMonorepo(t *testing.T) {
	tests := []struct {
		wt   WorkspaceType
		want bool
	}{
		{WorkspaceTypeNone, false},
		{WorkspaceTypePnpm, true},
		{WorkspaceTypeNpm, true},
		{WorkspaceTypeYarn, true},
		{WorkspaceTypeLerna, true},
		{WorkspaceTypeNx, true},
		{WorkspaceTypeTurborepo, true},
		{WorkspaceTypeGoModule, true},
		{WorkspaceTypeCargo, true},
		{WorkspaceTypeMaven, true},
		{WorkspaceTypeGradle, true},
		{WorkspaceTypeCustom, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.wt), func(t *testing.T) {
			if got := tt.wt.IsMonorepo(); got != tt.want {
				t.Errorf("WorkspaceType.IsMonorepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPackageManagerType_String(t *testing.T) {
	tests := []struct {
		pm   PackageManagerType
		want string
	}{
		{PackageManagerUnknown, "unknown"},
		{PackageManagerNpm, "npm"},
		{PackageManagerPnpm, "pnpm"},
		{PackageManagerYarn, "yarn"},
		{PackageManagerBun, "bun"},
		{PackageManagerGo, "go"},
		{PackageManagerCargo, "cargo"},
		{PackageManagerMaven, "maven"},
		{PackageManagerGradle, "gradle"},
		{PackageManagerPip, "pip"},
		{PackageManagerPoetry, "poetry"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.pm.String(); got != tt.want {
				t.Errorf("PackageManagerType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspaceStrategy_String(t *testing.T) {
	tests := []struct {
		ws   WorkspaceStrategy
		want string
	}{
		{StrategyIndependent, "independent"},
		{StrategyLockstep, "lockstep"},
		{StrategyHybrid, "hybrid"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ws.String(); got != tt.want {
				t.Errorf("WorkspaceStrategy.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspace_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		w    *Workspace
		want bool
	}{
		{"nil workspace", nil, true},
		{"none type", &Workspace{Type: WorkspaceTypeNone}, true},
		{"pnpm type", &Workspace{Type: WorkspaceTypePnpm}, false},
		{"npm type", &Workspace{Type: WorkspaceTypeNpm}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.IsEmpty(); got != tt.want {
				t.Errorf("Workspace.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspace_PackageCount(t *testing.T) {
	tests := []struct {
		name string
		w    *Workspace
		want int
	}{
		{"nil workspace", nil, 0},
		{"empty packages", &Workspace{}, 0},
		{"one package", &Workspace{Packages: []*Package{{Name: "pkg1"}}}, 1},
		{"multiple packages", &Workspace{Packages: []*Package{{Name: "pkg1"}, {Name: "pkg2"}, {Name: "pkg3"}}}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.PackageCount(); got != tt.want {
				t.Errorf("Workspace.PackageCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspace_FindPackage(t *testing.T) {
	pkg1 := &Package{Name: "@scope/pkg1", Path: "packages/pkg1"}
	pkg2 := &Package{Name: "pkg2", Path: "packages/pkg2"}
	w := &Workspace{Packages: []*Package{pkg1, pkg2}}

	tests := []struct {
		name    string
		w       *Workspace
		pkgName string
		wantPkg *Package
		wantNil bool
	}{
		{"nil workspace", nil, "pkg1", nil, true},
		{"found scoped package", w, "@scope/pkg1", pkg1, false},
		{"found simple package", w, "pkg2", pkg2, false},
		{"not found", w, "nonexistent", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.w.FindPackage(tt.pkgName)
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindPackage() = %v, want nil", got)
				}
			} else {
				if got != tt.wantPkg {
					t.Errorf("FindPackage() = %v, want %v", got, tt.wantPkg)
				}
			}
		})
	}
}

func TestWorkspace_FindPackageByPath(t *testing.T) {
	pkg1 := &Package{Name: "pkg1", Path: "packages/pkg1"}
	pkg2 := &Package{Name: "pkg2", Path: "apps/pkg2"}
	w := &Workspace{Packages: []*Package{pkg1, pkg2}}

	tests := []struct {
		name    string
		w       *Workspace
		path    string
		wantPkg *Package
		wantNil bool
	}{
		{"nil workspace", nil, "packages/pkg1", nil, true},
		{"found in packages", w, "packages/pkg1", pkg1, false},
		{"found in apps", w, "apps/pkg2", pkg2, false},
		{"not found", w, "libs/nonexistent", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.w.FindPackageByPath(tt.path)
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindPackageByPath() = %v, want nil", got)
				}
			} else {
				if got != tt.wantPkg {
					t.Errorf("FindPackageByPath() = %v, want %v", got, tt.wantPkg)
				}
			}
		})
	}
}

func TestDetectionResult_Success(t *testing.T) {
	tests := []struct {
		name string
		r    *DetectionResult
		want bool
	}{
		{"success", &DetectionResult{Workspace: &Workspace{Type: WorkspaceTypePnpm}}, true},
		{"error set", &DetectionResult{Error: &testError{}}, false},
		{"nil workspace", &DetectionResult{}, false},
		{"error and workspace", &DetectionResult{Workspace: &Workspace{}, Error: &testError{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Success(); got != tt.want {
				t.Errorf("DetectionResult.Success() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestDefaultDetectionOptions(t *testing.T) {
	opts := DefaultDetectionOptions()

	if opts.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want 3", opts.MaxDepth)
	}
	if !opts.IncludePackages {
		t.Error("IncludePackages should be true")
	}
	if opts.FollowSymlinks {
		t.Error("FollowSymlinks should be false")
	}
	if opts.PreferredTypes != nil {
		t.Error("PreferredTypes should be nil")
	}
	if opts.CustomMarkers != nil {
		t.Error("CustomMarkers should be nil")
	}
}
