package monorepo

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// A monorepo has no single version, so it cannot have a single `v1.2.3`. The tag has to be
// derivable in both directions: publish forms it from a package and a version, and bump reads
// a package's last release back out of the repository's tags.

func TestTheDefaultTagIsThePackageDirectoryAndAVersion(t *testing.T) {
	ver := version.MustParse("1.5.0")

	if got := TagNameFor("packages/api", nil, ver); got != "api-v1.5.0" {
		t.Errorf("TagNameFor = %q, want api-v1.5.0", got)
	}
	if got := TagNameFor("api", nil, ver); got != "api-v1.5.0" {
		t.Errorf("a package at the repository root got %q, want api-v1.5.0", got)
	}
}

func TestAConfiguredPrefixReplacesTheDefault(t *testing.T) {
	overrides := map[string]string{"packages/web": "webapp-v"}

	if got := TagNameFor("packages/web", overrides, version.MustParse("2.1.4")); got != "webapp-v2.1.4" {
		t.Errorf("TagNameFor = %q, want webapp-v2.1.4: a project already tagging webapp-v* "+
			"would otherwise have its history split in two", got)
	}
	if got := TagNameFor("packages/api", overrides, version.MustParse("1.0.0")); got != "api-v1.0.0" {
		t.Errorf("an override for one package changed another: %q", got)
	}
}

// Reading a version back out is what lets bump measure a package from its own last release.
func TestAVersionIsReadBackOutOfItsOwnTagOnly(t *testing.T) {
	ver, ok := VersionFromTag("api-v1.4.0", "api-v")
	if !ok || ver.String() != "1.4.0" {
		t.Errorf("VersionFromTag = %v, %v; want 1.4.0, true", ver, ok)
	}

	// The `-v` is part of the prefix, which is what keeps neighbors apart.
	if _, ok := VersionFromTag("api-server-v2.0.0", "api-v"); ok {
		t.Error("api-server-v2.0.0 was read as a tag of the api package, so a sibling's " +
			"release would be taken as this package's base")
	}
	if _, ok := VersionFromTag("v1.0.0", "api-v"); ok {
		t.Error("the repository's own tag was read as a package tag")
	}
	if _, ok := VersionFromTag("api-vnot-a-version", "api-v"); ok {
		t.Error("a tag that only looks like one was accepted")
	}
	if _, ok := VersionFromTag("anything", ""); ok {
		t.Error("an empty prefix matched, which would claim every tag in the repository")
	}
}
