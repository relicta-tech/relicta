package cli

import (
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

// A monorepo release has two levels: `relicta approve` decides the release, and
// `relicta approve --package <name>` decides what is in it. Publish tags and writes a changelog
// for exactly the packages that were decided, so these are the functions that keep a held
// package out of a release.

func withMonorepoConfig(t *testing.T) *config.Config {
	t.Helper()

	orig := cfg
	t.Cleanup(func() { cfg = orig })
	cfg = config.DefaultConfig()
	cfg.Monorepo.Enabled = true
	cfg.Monorepo.PackagePaths = []string{"packages/*"}
	return cfg
}

// A tag is the release: anything watching for `api-v1.5.0` starts on it. Only a decision in
// favor of releasing may let one be created.
func TestOnlyADecidedPackageShips(t *testing.T) {
	shipping := map[release.RunState]bool{
		release.StateApproved:   true,
		release.StatePublishing: true,
		release.StatePublished:  true,
		release.StateDraft:      false,
		release.StatePlanned:    false,
		release.StateVersioned:  false,
		release.StateNotesReady: false,
		release.StateFailed:     false,
		release.StateCanceled:   false,
	}

	for state, want := range shipping {
		if got := packageWillShip(state); got != want {
			t.Errorf("packageWillShip(%q) = %v, want %v", state, got, want)
		}
	}
}

// A package's changelog lives beside its manifest, where a reader of that package looks.
func TestAPackageChangelogSitsBesideItsManifest(t *testing.T) {
	withMonorepoConfig(t)

	got := packageChangelogFile("/repo", filepath.Join("packages", "api"))
	want := filepath.Join("/repo", "packages", "api", "CHANGELOG.md")
	if got != want {
		t.Errorf("packageChangelogFile = %q, want %q", got, want)
	}
}

func TestAConfiguredChangelogFileWins(t *testing.T) {
	c := withMonorepoConfig(t)
	relPath := filepath.Join("packages", "api")
	c.Monorepo.PackageOverrides = map[string]config.PackageOverrideConfig{
		relPath: {ChangelogFile: "NEWS.md"},
	}

	got := packageChangelogFile("/repo", relPath)
	want := filepath.Join("/repo", relPath, "NEWS.md")
	if got != want {
		t.Errorf("packageChangelogFile = %q, want %q", got, want)
	}

	// An absolute override is taken as given: a project collecting every package's changelog
	// in one directory means that path, not one under the package.
	c.Monorepo.PackageOverrides[relPath] = config.PackageOverrideConfig{ChangelogFile: "/docs/api.md"}
	if got := packageChangelogFile("/repo", relPath); got != "/docs/api.md" {
		t.Errorf("packageChangelogFile = %q, want the absolute path as given", got)
	}
}

// Tag prefixes reach the application layer as a plain map: naming a tag should not require
// importing the configuration package.
func TestTagPrefixesCarryOnlyWhatIsConfigured(t *testing.T) {
	c := withMonorepoConfig(t)

	if got := monorepoTagPrefixes(); got != nil {
		t.Errorf("prefixes = %v, want nil when nothing is overridden", got)
	}

	c.Monorepo.PackageOverrides = map[string]config.PackageOverrideConfig{
		"packages/web": {TagPrefix: "webapp-v"},
		"packages/api": {ChangelogFile: "NEWS.md"}, // no prefix
	}

	got := monorepoTagPrefixes()
	if len(got) != 1 || got["packages/web"] != "webapp-v" {
		t.Errorf("prefixes = %v, want only the package that configured one", got)
	}
}

// Each package reports the ref it was measured from, so the heading must not claim one for all
// of them.
func TestAnAbsentBaseRefReadsAsTheStartOfHistory(t *testing.T) {
	if got := refDisplay(""); got != "the start of history" {
		t.Errorf("refDisplay(\"\") = %q", got)
	}
	if got := refDisplay("api-v1.4.0"); got != "api-v1.4.0" {
		t.Errorf("refDisplay = %q, want the ref itself", got)
	}
}

// Paths are shown relative to the repository: absolute ones are what the writers need and not
// what an operator comparing a table wants to read.
func TestPathsAreShownRelativeToTheRepository(t *testing.T) {
	if got := displayPath(filepath.Join("/repo", "packages", "api"), "/repo"); got != filepath.Join("packages", "api") {
		t.Errorf("displayPath = %q", got)
	}
	if got := displayPath("/elsewhere/pkg", "/repo"); got == "" {
		t.Error("a path outside the repository rendered as empty")
	}
}

func TestPluralAgreesWithItsCount(t *testing.T) {
	if plural(1) != "" || plural(0) != "s" || plural(2) != "s" {
		t.Errorf("plural: %q %q %q", plural(1), plural(0), plural(2))
	}
}
