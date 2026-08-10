package sourcecontrol

import "testing"

// versioning.tag_prefix was configurable, documented, and had no effect beyond its
// default. NewTag decided version-ness by parsing the whole tag name, and
// version.Parse accepts only bare semver or a leading "v" — so "release-1.5.0",
// "rel/1.5.0" and the monorepo-style "app-v1.2.3" were not version tags at all.
// VersionTags() dropped them before FilterByPrefix could select them, and the
// usual call order was FilterByPrefix(prefix).VersionTags(): the second step
// discarded exactly what the first had chosen. Seven call sites used it.

func tagAt(name string) *Tag {
	return NewTag(name, CommitHash("aaaaaaaaaaaa"))
}

func TestVersionWithPrefix(t *testing.T) {
	cases := []struct {
		name    string
		tag     string
		prefix  string
		want    string
		wantNil bool
	}{
		{"the prefix that always worked", "v1.5.0", "v", "1.5.0", false},
		{"a word prefix", "release-1.5.0", "release-", "1.5.0", false},
		{"a path-like prefix", "rel/1.5.0", "rel/", "1.5.0", false},
		{"a monorepo component prefix", "app-v1.2.3", "app-v", "1.2.3", false},
		{"no prefix configured, bare semver", "1.5.0", "", "1.5.0", false},
		{"no prefix configured, v tag still parses", "v1.5.0", "", "1.5.0", false},
		{"prerelease survives", "app-v2.0.0-rc.1", "app-v", "2.0.0-rc.1", false},

		// A different component's tag is not this component's release. Without
		// this, widening prefix support would make every component in a monorepo
		// see every other component's versions.
		{"a different component", "web-v2.0.0", "app-v", "", true},
		{"prefix absent entirely", "1.5.0", "app-v", "", true},
		{"prefix present but no version after it", "app-vlatest", "app-v", "", true},
		{"prefix is the whole tag", "app-v", "app-v", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tagAt(tc.tag).VersionWithPrefix(tc.prefix)
			if tc.wantNil {
				if got != nil {
					t.Errorf("VersionWithPrefix(%q) on %q = %s, want nil", tc.prefix, tc.tag, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("VersionWithPrefix(%q) on %q = nil, want %s", tc.prefix, tc.tag, tc.want)
			}
			if got.String() != tc.want {
				t.Errorf("VersionWithPrefix(%q) on %q = %s, want %s", tc.prefix, tc.tag, got, tc.want)
			}
		})
	}
}

// The defect in one line: the two-step sequence returns nothing for a prefix
// version.Parse cannot read, and the combined method returns the tag.
func TestVersionTagsWithPrefixSucceedsWhereTheOldSequenceFailed(t *testing.T) {
	tags := TagList{tagAt("release-1.4.0"), tagAt("release-1.5.0"), tagAt("not-a-release")}

	if old := tags.FilterByPrefix("release-").VersionTags(); len(old) != 0 {
		t.Errorf("precondition: the old sequence found %d tags; this test describes why it found none", len(old))
	}

	got := tags.VersionTagsWithPrefix("release-")
	if len(got) != 2 {
		t.Fatalf("VersionTagsWithPrefix found %d version tags, want 2", len(got))
	}

	// And the version has to be readable from the returned tag, or callers that
	// filter correctly still get nil from Version().
	for _, tag := range got {
		if tag.Version() == nil {
			t.Errorf("%s came back without a resolved version; every caller reads Version() next", tag.Name())
		}
	}
}

func TestLatestWithPrefixPicksTheHighest(t *testing.T) {
	tags := TagList{
		tagAt("app-v1.9.0"),
		tagAt("app-v2.10.0"),
		tagAt("app-v2.9.0"),
		tagAt("web-v9.9.9"), // another component, must not win
	}

	latest := tags.LatestWithPrefix("app-v")
	if latest == nil {
		t.Fatal("expected a latest tag")
	}
	if latest.Name() != "app-v2.10.0" {
		t.Errorf("latest = %s, want app-v2.10.0 (2.10.0 > 2.9.0 numerically, not lexically)", latest.Name())
	}
}

func TestLatestWithPrefixReturnsNilWhenNothingMatches(t *testing.T) {
	tags := TagList{tagAt("web-v1.0.0"), tagAt("random-tag")}
	if latest := tags.LatestWithPrefix("app-v"); latest != nil {
		t.Errorf("no app-v tag exists, got %s", latest.Name())
	}
}

// The resolved version must not leak back into the shared tag: VersionTagsWithPrefix
// returns copies, so reading it under one prefix cannot change what another
// prefix sees. In a monorepo both are asked in the same process.
func TestVersionTagsWithPrefixDoesNotMutateItsInput(t *testing.T) {
	original := tagAt("app-v1.2.3")
	if original.Version() != nil {
		t.Fatalf("precondition: NewTag should not parse app-v1.2.3, got %s", original.Version())
	}

	tags := TagList{original}
	if got := tags.VersionTagsWithPrefix("app-v"); len(got) != 1 || got[0].Version() == nil {
		t.Fatal("expected the tag to resolve under its own prefix")
	}

	if original.Version() != nil {
		t.Errorf("the input tag was mutated; it now reports version %s", original.Version())
	}
}
