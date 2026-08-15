package versioning

// current_version.go: reads the version a project is already on out of one of
// its own manifests, for versioning.bump_from = "file" or "package.json".
//
// The default, bump_from = "tag", asks git what the last release was. That is
// the right answer for a repository whose tags are the record of what shipped.
// It is the wrong answer for one whose manifest is: a project that publishes to
// npm or crates.io from a manifest, or that imported its history from another
// tool, has a package.json saying 2.5.0 and no tag anywhere near it. bump_from
// exists to say which of the two is authoritative, and until this file nothing
// read it — so `bump` took the tag regardless and then *wrote* its answer into
// the manifest, silently walking 2.5.0 back to 1.1.0.
//
// Only the current version is taken from the file. The commit range is still
// delimited by tags, because a manifest records a version and not the point in
// history it was cut at; there is nothing else to ask.

import (
	"fmt"
	"os"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// BumpFrom values, as validated by config.Validator.validateVersioning.
const (
	bumpFromTag         = "tag"
	bumpFromFile        = "file"
	bumpFromPackageJSON = "package.json"
)

// packageJSONTarget is the manifest bump_from = "package.json" names. It is
// spelled out rather than looked up in version_files, because "package.json" is
// a complete instruction on its own — the file, and the field within it, are
// both fixed by npm.
func packageJSONTarget() config.VersionTarget {
	return config.VersionTarget{
		Path:   "package.json",
		Key:    "version",
		Format: config.VersionFormatSemver,
	}
}

// CurrentVersionTarget returns the manifest the current version should be read
// from, and false when bump_from leaves git tags authoritative.
//
// An unknown bump_from also returns false. Validation rejects those at load
// time; treating one as "not a file" here means a config that somehow slipped
// past falls back to the documented default rather than failing a release.
func CurrentVersionTarget(cfg config.VersioningConfig) (config.VersionTarget, bool, error) {
	switch cfg.BumpFrom {
	case bumpFromPackageJSON:
		return packageJSONTarget(), true, nil

	case bumpFromFile:
		target, err := semverBearingTarget(cfg.ResolvedVersionFiles())
		if err != nil {
			return config.VersionTarget{}, false, err
		}
		return target, true, nil

	case bumpFromTag, "":
		return config.VersionTarget{}, false, nil

	default:
		return config.VersionTarget{}, false, nil
	}
}

// semverBearingTarget picks the version_files entry to read the current version
// from: the first one that carries a plain semantic version.
//
// Not simply the first entry. version_files exists so that several manifests
// stay in step, and they routinely hold shapes that are not versions to read
// back — an Android versionCode is a monotonic counter, a Stream Deck manifest
// wants four components, a template can render anything at all. Those are
// derived *from* the version, so deriving the version from them would be
// circular. The first plain-semver entry is the one that round-trips.
func semverBearingTarget(targets []config.VersionTarget) (config.VersionTarget, error) {
	for _, t := range targets {
		if t.Strategy == config.StrategyIncrement {
			continue
		}
		if t.Format == config.VersionFormatSemver || t.Format == "" {
			return t, nil
		}
	}

	if len(targets) == 0 {
		return config.VersionTarget{}, fmt.Errorf(
			"versioning.bump_from is %q but no versioning.version_files are configured; "+
				"name the manifest that holds the current version, or set bump_from to %q",
			bumpFromFile, bumpFromTag)
	}
	return config.VersionTarget{}, fmt.Errorf(
		"versioning.bump_from is %q but none of the %d configured version_files holds a plain "+
			"semantic version to read back (every entry is an increment, integer, semver.build or "+
			"template target, all of which are derived from the version rather than a source for it); "+
			"add a semver entry, or set bump_from to \"tag\"",
		bumpFromFile, len(targets))
}

// ReadCurrentVersion reads the semantic version a manifest currently carries.
//
// Every failure is returned rather than absorbed. Falling back to the git tag
// when the manifest cannot be read would resurrect exactly the divergence
// bump_from exists to settle, and do it silently: the release would proceed
// from a version the user did not ask for and would not be told about.
func ReadCurrentVersion(repoRoot string, t config.VersionTarget) (version.SemanticVersion, error) {
	abs, err := resolveInRepo(repoRoot, t.Path)
	if err != nil {
		return version.SemanticVersion{}, fmt.Errorf("version file %s: %w", t.Path, err)
	}

	data, err := os.ReadFile(abs) //nolint:gosec // abs is confined to repoRoot by resolveInRepo
	if err != nil {
		return version.SemanticVersion{}, fmt.Errorf("reading version file %s: %w", t.Path, err)
	}

	raw, err := rawVersionIn(t, data)
	if err != nil {
		return version.SemanticVersion{}, fmt.Errorf("version file %s: %w", t.Path, err)
	}

	// A leading "v" is how the version is spelled in plenty of VERSION files, and
	// rejecting it would fail a release over punctuation.
	parsed, err := version.Parse(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if err != nil {
		return version.SemanticVersion{}, fmt.Errorf(
			"version file %s holds %q, which is not a semantic version: %w", t.Path, raw, err)
	}
	return parsed, nil
}

// rawVersionIn extracts the version string a target addresses, using the same
// per-format readers the writer uses, so read and write always address the same
// place in the same file.
func rawVersionIn(t config.VersionTarget, data []byte) (string, error) {
	kind := structuredKind(t.Path)

	switch kind {
	case kindPlain:
		if t.Key != "" {
			return "", fmt.Errorf("key %q given for a plain-text file; keys apply to json, yaml, toml and gradle targets", t.Key)
		}
		return strings.TrimSpace(string(data)), nil

	case kindGradle:
		if t.Key == "" {
			return "", fmt.Errorf("key is required for gradle files; name the property that holds the version, e.g. versionName")
		}
		return getGradle(data, t.Key)

	default:
		if t.Key == "" {
			return "", fmt.Errorf("key is required for %s files; naming the field avoids reading the wrong one", kind)
		}
		return getStructured(kind, data, t.Key)
	}
}
