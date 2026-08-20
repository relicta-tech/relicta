package monorepo

import (
	"path"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Tag naming for per-package releases.
//
// A monorepo has no single version, so it cannot have a single `v1.2.3`. Each package is
// released under its own tag, and the name has to be derivable in both directions: publish
// forms it from a package and a version, and bump reads a package's last release back out of
// the repository's tags.

// TagPrefixFor returns the prefix a package's tags carry.
//
// The default is the package's directory name followed by `-v`, which gives `api-v1.5.0` — the
// convention npm and Cargo monorepos use, and one that sorts the packages together in
// `git tag`. `monorepo.package_overrides.<path>.tag_prefix` replaces it outright, so a project
// already tagging `core-v*` or `@scope/pkg@` keeps its history readable.
//
// The override is looked up by the path as written in the configuration, which is relative to
// the repository root, so the caller passes the relative path rather than the absolute one it
// uses for file access.
func TagPrefixFor(relPath string, overrides map[string]string) string {
	if prefix, ok := overrides[relPath]; ok && prefix != "" {
		return prefix
	}
	return path.Base(relPath) + "-v"
}

// TagNameFor is the tag a package release carries.
func TagNameFor(relPath string, overrides map[string]string, ver version.SemanticVersion) string {
	return TagPrefixFor(relPath, overrides) + ver.String()
}

// VersionFromTag reads a version back out of a tag carrying the given prefix.
//
// Reports false for a tag belonging to another package or to the repository as a whole, which
// is what makes it usable as a filter over every tag in the repository: `api-v1.4.0` and
// `api-server-v2.0.0` share no prefix by accident, because the `-v` is part of it.
func VersionFromTag(tag, prefix string) (version.SemanticVersion, bool) {
	if prefix == "" || !strings.HasPrefix(tag, prefix) {
		return version.Zero, false
	}
	ver, err := version.Parse(strings.TrimPrefix(tag, prefix))
	if err != nil {
		return version.Zero, false
	}
	return ver, true
}
