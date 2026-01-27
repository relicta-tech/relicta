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
