// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"sort"
	"testing"
	"time"
)

func TestNewTag(t *testing.T) {
	tag := NewTag("v1.0.0", CommitHash("abc123"))

	if tag.Name() != "v1.0.0" {
		t.Errorf("Name() = %v, want v1.0.0", tag.Name())
	}
	if tag.Hash() != CommitHash("abc123") {
		t.Errorf("Hash() = %v, want abc123", tag.Hash())
	}
	if tag.Message() != "" {
		t.Errorf("Message() should be empty for lightweight tag, got %v", tag.Message())
	}
	if !tag.IsVersionTag() {
		t.Error("IsVersionTag() should be true for v1.0.0")
	}
}

func TestNewTag_NonVersionTag(t *testing.T) {
	tag := NewTag("release-candidate", CommitHash("abc123"))

	if tag.Name() != "release-candidate" {
		t.Errorf("Name() = %v, want release-candidate", tag.Name())
	}
	if tag.IsVersionTag() {
		t.Error("IsVersionTag() should be false for non-version tag")
	}
	if tag.Version() != nil {
		t.Error("Version() should be nil for non-version tag")
	}
}

func TestNewAnnotatedTag(t *testing.T) {
	tagger := Author{Name: "John Doe", Email: "john@example.com"}
	tag := NewAnnotatedTag("v2.0.0", CommitHash("def456"), "Release v2.0.0", tagger)

	if tag.Name() != "v2.0.0" {
		t.Errorf("Name() = %v, want v2.0.0", tag.Name())
	}
	if tag.Hash() != CommitHash("def456") {
		t.Errorf("Hash() = %v, want def456", tag.Hash())
	}
	if tag.Message() != "Release v2.0.0" {
		t.Errorf("Message() = %v, want Release v2.0.0", tag.Message())
	}
	if tag.Tagger() != tagger {
		t.Errorf("Tagger() = %v, want %v", tag.Tagger(), tagger)
	}
	if !tag.IsVersionTag() {
		t.Error("IsVersionTag() should be true for v2.0.0")
	}
	if tag.IsLightweight() {
		t.Error("IsLightweight() should be false for annotated tag")
	}
}

func TestTag_SetMessage(t *testing.T) {
	tag := NewTag("v1.0.0", CommitHash("abc123"))

	// Initially lightweight
	if !tag.IsLightweight() {
		t.Error("Tag should be lightweight initially")
	}

	// Set message makes it annotated
	tag.SetMessage("This is a release")
	if tag.Message() != "This is a release" {
		t.Errorf("Message() = %v, want This is a release", tag.Message())
	}
	if tag.IsLightweight() {
		t.Error("Tag should not be lightweight after SetMessage")
	}
}

func TestTag_SetTagger(t *testing.T) {
	tag := NewTag("v1.0.0", CommitHash("abc123"))

	tagger := Author{Name: "Jane Doe", Email: "jane@example.com"}
	tag.SetTagger(tagger)

	if tag.Tagger() != tagger {
		t.Errorf("Tagger() = %v, want %v", tag.Tagger(), tagger)
	}
}

func TestTag_SetDate(t *testing.T) {
	tag := NewTag("v1.0.0", CommitHash("abc123"))

	newDate := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	tag.SetDate(newDate)

	if !tag.Date().Equal(newDate) {
		t.Errorf("Date() = %v, want %v", tag.Date(), newDate)
	}
}

func TestTag_HasPrefix(t *testing.T) {
	tag := NewTag("v1.0.0", CommitHash("abc123"))

	tests := []struct {
		prefix string
		want   bool
	}{
		{"v", true},
		{"v1", true},
		{"v1.0", true},
		{"v1.0.0", true},
		{"v2", false},
		{"release-", false},
		{"", true}, // empty prefix matches everything
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := tag.HasPrefix(tt.prefix); got != tt.want {
				t.Errorf("HasPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestTag_WithoutPrefix(t *testing.T) {
	tests := []struct {
		name    string
		tagName string
		prefix  string
		want    string
	}{
		{"remove v prefix", "v1.0.0", "v", "1.0.0"},
		{"no matching prefix", "v1.0.0", "release-", "v1.0.0"},
		{"full tag as prefix", "v1.0.0", "v1.0.0", ""},
		{"empty prefix", "v1.0.0", "", "v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := NewTag(tt.tagName, CommitHash("abc123"))
			if got := tag.WithoutPrefix(tt.prefix); got != tt.want {
				t.Errorf("WithoutPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestTag_Version(t *testing.T) {
	// Version tag
	vTag := NewTag("v1.2.3", CommitHash("abc123"))
	if vTag.Version() == nil {
		t.Error("Version() should not be nil for version tag")
	}
	if vTag.Version().String() != "1.2.3" {
		t.Errorf("Version().String() = %v, want 1.2.3", vTag.Version().String())
	}

	// Non-version tag
	nonVTag := NewTag("latest", CommitHash("def456"))
	if nonVTag.Version() != nil {
		t.Error("Version() should be nil for non-version tag")
	}
}

func TestTagList_Len(t *testing.T) {
	tl := TagList{
		NewTag("v1.0.0", CommitHash("a")),
		NewTag("v2.0.0", CommitHash("b")),
		NewTag("v3.0.0", CommitHash("c")),
	}

	if tl.Len() != 3 {
		t.Errorf("Len() = %v, want 3", tl.Len())
	}
}

func TestTagList_Less(t *testing.T) {
	tag1 := NewTag("v1.0.0", CommitHash("a"))
	tag2 := NewTag("v2.0.0", CommitHash("b"))

	tl := TagList{tag1, tag2}

	// v1.0.0 < v2.0.0
	if !tl.Less(0, 1) {
		t.Error("Less(0, 1) should be true: v1.0.0 < v2.0.0")
	}
	if tl.Less(1, 0) {
		t.Error("Less(1, 0) should be false: v2.0.0 > v1.0.0")
	}
}

func TestTagList_Less_NonVersionTags(t *testing.T) {
	tag1 := NewTag("release-a", CommitHash("a"))
	tag1.SetDate(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	tag2 := NewTag("release-b", CommitHash("b"))
	tag2.SetDate(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

	tl := TagList{tag1, tag2}

	// Earlier date should be less
	if !tl.Less(0, 1) {
		t.Error("Less(0, 1) should be true for earlier date")
	}
	if tl.Less(1, 0) {
		t.Error("Less(1, 0) should be false for later date")
	}
}

func TestTagList_Swap(t *testing.T) {
	tag1 := NewTag("v1.0.0", CommitHash("a"))
	tag2 := NewTag("v2.0.0", CommitHash("b"))

	tl := TagList{tag1, tag2}
	tl.Swap(0, 1)

	if tl[0].Name() != "v2.0.0" {
		t.Errorf("After Swap, tl[0].Name() = %v, want v2.0.0", tl[0].Name())
	}
	if tl[1].Name() != "v1.0.0" {
		t.Errorf("After Swap, tl[1].Name() = %v, want v1.0.0", tl[1].Name())
	}
}

func TestTagList_Sort(t *testing.T) {
	tag1 := NewTag("v3.0.0", CommitHash("c"))
	tag2 := NewTag("v1.0.0", CommitHash("a"))
	tag3 := NewTag("v2.0.0", CommitHash("b"))

	tl := TagList{tag1, tag2, tag3}
	sort.Sort(tl)

	expected := []string{"v1.0.0", "v2.0.0", "v3.0.0"}
	for i, want := range expected {
		if tl[i].Name() != want {
			t.Errorf("After Sort, tl[%d].Name() = %v, want %v", i, tl[i].Name(), want)
		}
	}
}

func TestTagList_Latest(t *testing.T) {
	tag1 := NewTag("v1.0.0", CommitHash("a"))
	tag2 := NewTag("v3.0.0", CommitHash("c"))
	tag3 := NewTag("v2.0.0", CommitHash("b"))

	tl := TagList{tag1, tag2, tag3}
	latest := tl.Latest()

	if latest == nil {
		t.Fatal("Latest() should not be nil")
	}
	if latest.Name() != "v3.0.0" {
		t.Errorf("Latest().Name() = %v, want v3.0.0", latest.Name())
	}
}

func TestTagList_Latest_Empty(t *testing.T) {
	tl := TagList{}
	if tl.Latest() != nil {
		t.Error("Latest() should be nil for empty list")
	}
}

func TestTagList_Latest_NoVersionTags(t *testing.T) {
	tl := TagList{
		NewTag("release-a", CommitHash("a")),
		NewTag("release-b", CommitHash("b")),
	}
	if tl.Latest() != nil {
		t.Error("Latest() should be nil when no version tags exist")
	}
}

func TestTagList_FilterByPrefix(t *testing.T) {
	tl := TagList{
		NewTag("v1.0.0", CommitHash("a")),
		NewTag("v2.0.0", CommitHash("b")),
		NewTag("release-1.0", CommitHash("c")),
		NewTag("v3.0.0", CommitHash("d")),
	}

	filtered := tl.FilterByPrefix("v")

	if len(filtered) != 3 {
		t.Errorf("FilterByPrefix(\"v\") length = %v, want 3", len(filtered))
	}

	for _, tag := range filtered {
		if !tag.HasPrefix("v") {
			t.Errorf("Filtered tag %v should have prefix 'v'", tag.Name())
		}
	}
}

func TestTagList_FilterByPrefix_NoMatch(t *testing.T) {
	tl := TagList{
		NewTag("v1.0.0", CommitHash("a")),
		NewTag("v2.0.0", CommitHash("b")),
	}

	filtered := tl.FilterByPrefix("release-")
	if len(filtered) != 0 {
		t.Errorf("FilterByPrefix(\"release-\") should return empty list, got %d", len(filtered))
	}
}

func TestTagList_VersionTags(t *testing.T) {
	tl := TagList{
		NewTag("v1.0.0", CommitHash("a")),
		NewTag("latest", CommitHash("b")),
		NewTag("v2.0.0", CommitHash("c")),
		NewTag("release-candidate", CommitHash("d")),
	}

	versionTags := tl.VersionTags()

	if len(versionTags) != 2 {
		t.Errorf("VersionTags() length = %v, want 2", len(versionTags))
	}

	for _, tag := range versionTags {
		if !tag.IsVersionTag() {
			t.Errorf("Tag %v should be a version tag", tag.Name())
		}
	}
}

func TestTagList_VersionTags_Empty(t *testing.T) {
	tl := TagList{
		NewTag("latest", CommitHash("a")),
		NewTag("stable", CommitHash("b")),
	}

	versionTags := tl.VersionTags()
	if len(versionTags) != 0 {
		t.Errorf("VersionTags() should return empty list when no version tags, got %d", len(versionTags))
	}
}
