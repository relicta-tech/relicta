// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/version"
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

// FilterByPrefix returns tags with the specified prefix.
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
