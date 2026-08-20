package monorepo

import (
	"os"
	"path/filepath"

	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
)

// packageTypeMarkers maps a manifest file to the package type that owns it, in the order
// they are tried. Order matters where a directory carries more than one: a Go module with a
// package.json for its tooling is a Go module, because go.mod is the file its version is
// released from.
var packageTypeMarkers = []struct {
	file string
	typ  monorepo.PackageType
}{
	{"package.json", monorepo.PackageTypeNPM},
	{"Cargo.toml", monorepo.PackageTypeCargo},
	{"pyproject.toml", monorepo.PackageTypePython},
	{"setup.py", monorepo.PackageTypePython},
	{"go.mod", monorepo.PackageTypeGoModule},
	{"pom.xml", monorepo.PackageTypeMaven},
	{"build.gradle", monorepo.PackageTypeGradle},
	{"build.gradle.kts", monorepo.PackageTypeGradle},
	{"composer.json", monorepo.PackageTypeComposer},
	{"Gemfile", monorepo.PackageTypeGem},
}

// DetectPackageType identifies a package from the manifest lying in its directory.
//
// The workspace model carries one PackageManager for the whole workspace, which is right for
// a pnpm or Cargo workspace and wrong for the layout `monorepo.package_paths` exists to
// describe: `packages/*` may hold an npm package beside a Go module, and versioning each from
// the workspace-wide type would read one manifest and write another.
//
// Returns PackageTypeDirectory when no manifest is recognized, which is also a real answer —
// DirectoryVersionWriter keeps a plain VERSION file — so callers that want to fall back to the
// workspace type should test for it explicitly.
func DetectPackageType(pkgPath string) monorepo.PackageType {
	for _, marker := range packageTypeMarkers {
		if _, err := os.Stat(filepath.Join(pkgPath, marker.file)); err == nil {
			return marker.typ
		}
	}
	return monorepo.PackageTypeDirectory
}
