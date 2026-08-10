// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Tag represents a git tag entity.
type Tag struct {
	name          string
	hash          CommitHash
	message       string
	tagger        Author
	date          time.Time
	isLightweight bool
	version       *version.SemanticVersion
}

// NewTag creates a new Tag entity.
func NewTag(name string, hash CommitHash) *Tag {
	t := &Tag{
		name: name,
		hash: hash,
		date: time.Now(),
	}

	// Try to parse as version
	if ver, err := version.Parse(name); err == nil {
		t.version = &ver
	}

	return t
}

// NewAnnotatedTag creates a new annotated Tag entity.
func NewAnnotatedTag(name string, hash CommitHash, message string, tagger Author) *Tag {
	t := &Tag{
		name:    name,
		hash:    hash,
		message: message,
		tagger:  tagger,
		date:    time.Now(),
	}

	// Try to parse as version
	if ver, err := version.Parse(name); err == nil {
		t.version = &ver
	}

	return t
}

// Name returns the tag name.
func (t *Tag) Name() string {
	return t.name
}

// Hash returns the commit hash the tag points to.
func (t *Tag) Hash() CommitHash {
	return t.hash
}

// Message returns the tag message (for annotated tags).
func (t *Tag) Message() string {
	return t.message
}

// SetMessage sets the tag message.
func (t *Tag) SetMessage(msg string) {
	t.message = msg
	t.isLightweight = false
}

// Tagger returns the tagger (for annotated tags).
func (t *Tag) Tagger() Author {
	return t.tagger
}

// SetTagger sets the tagger.
func (t *Tag) SetTagger(tagger Author) {
	t.tagger = tagger
}

// Date returns the tag date.
func (t *Tag) Date() time.Time {
	return t.date
}

// SetDate sets the tag date.
func (t *Tag) SetDate(date time.Time) {
	t.date = date
}

// IsLightweight returns true if this is a lightweight tag.
func (t *Tag) IsLightweight() bool {
	return t.isLightweight || t.message == ""
}

// IsVersionTag returns true if this tag represents a version.
func (t *Tag) IsVersionTag() bool {
	return t.version != nil
}

// Version returns the semantic version if this is a version tag.
func (t *Tag) Version() *version.SemanticVersion {
	return t.version
}

// HasPrefix returns true if the tag has the specified prefix.
func (t *Tag) HasPrefix(prefix string) bool {
	return strings.HasPrefix(t.name, prefix)
}

// WithoutPrefix returns the tag name without the specified prefix.
func (t *Tag) WithoutPrefix(prefix string) string {
	return strings.TrimPrefix(t.name, prefix)
}

// TagList represents a sorted list of tags.
type TagList []*Tag

// Len returns the number of tags.
func (tl TagList) Len() int {
	return len(tl)
}

// Less compares tags by version (if both are version tags) or by date.
func (tl TagList) Less(i, j int) bool {
	if tl[i].IsVersionTag() && tl[j].IsVersionTag() {
		return tl[i].version.LessThan(*tl[j].version)
	}
	return tl[i].date.Before(tl[j].date)
}

// Swap swaps two tags.
func (tl TagList) Swap(i, j int) {
	tl[i], tl[j] = tl[j], tl[i]
}

// Latest returns the latest version tag.
func (tl TagList) Latest() *Tag {
	var latest *Tag
	for _, t := range tl {
		if t.IsVersionTag() {
			if latest == nil || t.version.GreaterThan(*latest.version) {
				latest = t
			}
		}
	}
	return latest
}

// VersionWithPrefix resolves the tag's version after removing a known prefix.
//
// Version() reflects what NewTag could parse from the whole tag name, and
// version.Parse accepts only bare semver or a leading "v". So "app-v1.2.3" and
// "release-1.5.0" are not version tags at all, and every caller that filtered by
// a configured prefix and then asked for version tags got nothing: the second step
// discarded exactly what the first step selected. Nothing reported that the
// prefix had been ignored, so a project using one appeared to have no releases.
//
// Returns nil when the tag does not carry the prefix, or when what remains is not
// a semantic version.
func (t *Tag) VersionWithPrefix(prefix string) *version.SemanticVersion {
	if !strings.HasPrefix(t.name, prefix) {
		return nil
	}
	ver, err := version.Parse(strings.TrimPrefix(t.name, prefix))
	if err != nil {
		return nil
	}
	return &ver
}

// IsVersionTagWithPrefix reports whether the tag names a version under prefix.
func (t *Tag) IsVersionTagWithPrefix(prefix string) bool {
	return t.VersionWithPrefix(prefix) != nil
}

// withVersion returns a copy of the tag carrying a resolved version, so callers
// reading Version() after a prefix-aware filter see the version that filter
// matched on rather than the one NewTag guessed from the whole name.
func (t *Tag) withVersion(ver *version.SemanticVersion) *Tag {
	clone := *t
	clone.version = ver
	return &clone
}

// VersionTagsWithPrefix returns the tags that name a version under prefix, each
// carrying its resolved version.
//
// This replaces FilterByPrefix(prefix).VersionTags(), which cannot work for any
// prefix outside what version.Parse already accepts. Doing both steps together is
// also the only way to keep them consistent: the prefix that selects a tag is the
// prefix that must be removed to read its version.
func (tl TagList) VersionTagsWithPrefix(prefix string) TagList {
	result := make(TagList, 0, len(tl))
	for _, t := range tl {
		if ver := t.VersionWithPrefix(prefix); ver != nil {
			result = append(result, t.withVersion(ver))
		}
	}
	return result
}

// LatestWithPrefix returns the highest-versioned tag under prefix, or nil.
func (tl TagList) LatestWithPrefix(prefix string) *Tag {
	return tl.VersionTagsWithPrefix(prefix).Latest()
}

// FilterByPrefix returns tags with the specified prefix.
//
// Prefer VersionTagsWithPrefix when the intent is "version tags under this
// prefix": chaining this with VersionTags() drops every tag whose prefix is not
// one version.Parse already understands.
func (tl TagList) FilterByPrefix(prefix string) TagList {
	// Pre-allocate assuming ~25% match rate to reduce reallocations
	result := make(TagList, 0, len(tl)/4+1)
	for _, t := range tl {
		if t.HasPrefix(prefix) {
			result = append(result, t)
		}
	}
	return result
}

// VersionTags returns only version tags.
func (tl TagList) VersionTags() TagList {
	// Pre-allocate assuming most tags are version tags
	result := make(TagList, 0, len(tl))
	for _, t := range tl {
		if t.IsVersionTag() {
			result = append(result, t)
		}
	}
	return result
}
