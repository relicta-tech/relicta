package versioning

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// versioningConfig builds a VersioningConfig with the fields these tests care
// about, so each case reads as the config a user would actually write.
func versioningConfig(bumpFrom string, targets ...config.VersionTarget) config.VersioningConfig {
	return config.VersioningConfig{
		Strategy:     "conventional",
		TagPrefix:    "v",
		BumpFrom:     bumpFrom,
		VersionFiles: targets,
	}
}

// TestTheVersionComesFromTheFileWhenBumpFromNamesOne is the case the setting
// exists for: the tag and the manifest disagree, and bump_from says which wins.
func TestTheVersionComesFromTheFileWhenBumpFromNamesOne(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", "{\n  \"name\": \"demo\",\n  \"version\": \"2.5.0\"\n}\n")

	cfg := versioningConfig("file", config.VersionTarget{
		Path:   "package.json",
		Key:    "version",
		Format: config.VersionFormatSemver,
	})

	target, ok, err := CurrentVersionTarget(cfg)
	if err != nil {
		t.Fatalf("CurrentVersionTarget() error = %v", err)
	}
	if !ok {
		t.Fatal("bump_from 'file' left git tags authoritative; the manifest a user configured would be ignored and the release would carry the tag's version instead")
	}

	got, err := ReadCurrentVersion(dir, target)
	if err != nil {
		t.Fatalf("ReadCurrentVersion() error = %v", err)
	}
	if got.String() != "2.5.0" {
		t.Errorf("current version = %q, want %q; the release would bump from the wrong starting point and write a version the manifest never held", got.String(), "2.5.0")
	}
}

// TestPackageJSONNeedsNoVersionFilesToBeRead: bump_from "package.json" names the
// file and the field on its own, so requiring version_files as well would make
// the value unusable.
func TestPackageJSONNeedsNoVersionFilesToBeRead(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", "{\n  \"name\": \"demo\",\n  \"version\": \"4.2.1\"\n}\n")

	target, ok, err := CurrentVersionTarget(versioningConfig("package.json"))
	if err != nil {
		t.Fatalf("CurrentVersionTarget() error = %v", err)
	}
	if !ok {
		t.Fatal("bump_from 'package.json' resolved to no target, so the manifest would never be read")
	}

	got, err := ReadCurrentVersion(dir, target)
	if err != nil {
		t.Fatalf("ReadCurrentVersion() error = %v", err)
	}
	if got.String() != "4.2.1" {
		t.Errorf("current version = %q, want %q", got.String(), "4.2.1")
	}
}

// TestTheGitTagStaysAuthoritativeUnderTheDefault guards the blast radius: every
// project that never set bump_from must keep asking git.
func TestTheGitTagStaysAuthoritativeUnderTheDefault(t *testing.T) {
	for _, bumpFrom := range []string{"tag", ""} {
		_, ok, err := CurrentVersionTarget(versioningConfig(bumpFrom, config.VersionTarget{
			Path: "package.json", Key: "version",
		}))
		if err != nil {
			t.Fatalf("CurrentVersionTarget(%q) error = %v", bumpFrom, err)
		}
		if ok {
			t.Errorf("bump_from %q resolved to a file target; every project on the default would silently start reading its manifest instead of its tags", bumpFrom)
		}
	}
}

// TestACounterTargetIsNotMistakenForTheCurrentVersion: version_files routinely
// holds shapes derived from the version — an Android versionCode, a four-part
// platform manifest — and reading one back as the version would be circular.
func TestACounterTargetIsNotMistakenForTheCurrentVersion(t *testing.T) {
	cfg := versioningConfig("file",
		config.VersionTarget{
			Path:     "build.gradle",
			Key:      "versionCode",
			Format:   config.VersionFormatInteger,
			Strategy: config.StrategyIncrement,
		},
		config.VersionTarget{
			Path:   "manifest.json",
			Key:    "Version",
			Format: config.VersionFormatSemverBuild,
		},
		config.VersionTarget{
			Path:   "package.json",
			Key:    "version",
			Format: config.VersionFormatSemver,
		},
	)

	target, ok, err := CurrentVersionTarget(cfg)
	if err != nil {
		t.Fatalf("CurrentVersionTarget() error = %v", err)
	}
	if !ok {
		t.Fatal("no target selected despite a semver entry being configured")
	}
	if target.Path != "package.json" {
		t.Errorf("selected %q, want package.json; a counter or four-part target is derived from the version, so reading the version back out of one yields a number that is not the version at all", target.Path)
	}
}

// TestNoReadableVersionFileIsReportedRatherThanGuessed: when every configured
// target is derived from the version, there is nothing to read, and saying so
// beats falling back to the tag the user told us not to trust.
func TestNoReadableVersionFileIsReportedRatherThanGuessed(t *testing.T) {
	cfg := versioningConfig("file", config.VersionTarget{
		Path:     "build.gradle",
		Key:      "versionCode",
		Format:   config.VersionFormatInteger,
		Strategy: config.StrategyIncrement,
	})

	_, _, err := CurrentVersionTarget(cfg)
	if err == nil {
		t.Fatal("no error for a bump_from 'file' config with no readable version; the release would proceed from a version nobody chose")
	}
	if !strings.Contains(err.Error(), "bump_from") {
		t.Errorf("error = %q, want it to name bump_from so the user knows which setting to change", err.Error())
	}
}

// TestAManifestThatIsNotASemanticVersionFailsTheRead. Falling back to the tag
// here would resurrect the exact divergence bump_from exists to settle, and do
// it without telling anyone.
func TestAManifestThatIsNotASemanticVersionFailsTheRead(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", "{\n  \"name\": \"demo\",\n  \"version\": \"not-a-version\"\n}\n")

	_, err := ReadCurrentVersion(dir, config.VersionTarget{
		Path: "package.json", Key: "version", Format: config.VersionFormatSemver,
	})
	if err == nil {
		t.Fatal("no error for an unparseable manifest version; the release would silently continue from the git tag the user configured away from")
	}
	if !strings.Contains(err.Error(), "not-a-version") {
		t.Errorf("error = %q, want it to quote the offending value so the user can find it", err.Error())
	}
}

// TestAMissingManifestFailsTheRead: a configured file that is not there is a
// config error, not a reason to fall back.
func TestAMissingManifestFailsTheRead(t *testing.T) {
	_, err := ReadCurrentVersion(t.TempDir(), config.VersionTarget{
		Path: "package.json", Key: "version",
	})
	if err == nil {
		t.Fatal("no error for a missing manifest; the release would proceed from the git tag instead of failing")
	}
}

// TestAPlainVersionFileToleratesALeadingV, because that is how plenty of VERSION
// files are written and failing a release over punctuation helps nobody.
func TestAPlainVersionFileToleratesALeadingV(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "VERSION", "v3.1.0\n")

	got, err := ReadCurrentVersion(dir, config.VersionTarget{Path: "VERSION"})
	if err != nil {
		t.Fatalf("ReadCurrentVersion() error = %v", err)
	}
	if got.String() != "3.1.0" {
		t.Errorf("current version = %q, want %q", got.String(), "3.1.0")
	}
}

// TestReadingRefusesAPathOutsideTheRepository holds the same line the writer
// holds: a version file is part of the project.
func TestReadingRefusesAPathOutsideTheRepository(t *testing.T) {
	_, err := ReadCurrentVersion(t.TempDir(), config.VersionTarget{
		Path: "../outside/package.json", Key: "version",
	})
	if err == nil {
		t.Fatal("a path escaping the repository root was read; config could point the version source at any file on the machine")
	}
}

// TestTheCalculatedVersionStartsFromTheConfiguredCurrentVersion checks the wiring
// end of it: the use case must bump from the supplied version, not from the tag
// its git repository reports.
func TestTheCalculatedVersionStartsFromTheConfiguredCurrentVersion(t *testing.T) {
	fromFile := mustVersion(t, "2.5.0")

	gitRepo := &mockGitRepository{
		commits: []*sourcecontrol.Commit{
			createTestCommit("abc123", "feat: add a feature"),
		},
		// The repository's own answer is 1.0.0, and it must lose to the manifest.
		latestTagErr: errors.New("no tags found"),
	}

	uc := NewCalculateVersionUseCase(gitRepo, &mockVersionCalculator{})
	out, err := uc.Execute(context.Background(), CalculateVersionInput{
		Auto:           true,
		CurrentVersion: &fromFile,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if out.CurrentVersion.String() != "2.5.0" {
		t.Errorf("current version = %q, want 2.5.0; the configured source was ignored", out.CurrentVersion.String())
	}
	if out.NextVersion.String() != "2.6.0" {
		t.Errorf("next version = %q, want 2.6.0; a release cut here would carry a version below the one the manifest already publishes", out.NextVersion.String())
	}
}

// TestTheCalculatedVersionFallsBackToGitWhenNoneIsConfigured is the other half of
// the guard: nil must leave the existing discovery untouched.
func TestTheCalculatedVersionFallsBackToGitWhenNoneIsConfigured(t *testing.T) {
	gitRepo := &mockGitRepository{
		commits: []*sourcecontrol.Commit{
			createTestCommit("abc123", "feat: add a feature"),
		},
		latestTagErr: errors.New("no tags found"),
	}

	uc := NewCalculateVersionUseCase(gitRepo, &mockVersionCalculator{})
	out, err := uc.Execute(context.Background(), CalculateVersionInput{Auto: true})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if out.CurrentVersion.String() != version.Zero.String() {
		t.Errorf("current version = %q, want %q; the default path no longer asks git", out.CurrentVersion.String(), version.Zero.String())
	}
}
