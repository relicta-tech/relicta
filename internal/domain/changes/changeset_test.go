// Package changes provides domain types for analyzing commit changes.
package changes

import (
	"testing"
	"time"
)

func TestNewChangeSet(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")

	if cs.ID() != "changeset-1" {
		t.Errorf("ID() = %v, want changeset-1", cs.ID())
	}
	if cs.FromRef() != "v1.0.0" {
		t.Errorf("FromRef() = %v, want v1.0.0", cs.FromRef())
	}
	if cs.ToRef() != "HEAD" {
		t.Errorf("ToRef() = %v, want HEAD", cs.ToRef())
	}
	if cs.CommitCount() != 0 {
		t.Errorf("CommitCount() = %d, want 0", cs.CommitCount())
	}
	if !cs.IsEmpty() {
		t.Error("IsEmpty() = false, want true")
	}
	if cs.CreatedAt().IsZero() {
		t.Error("CreatedAt() is zero, want non-zero")
	}
}

func TestChangeSet_AddCommit(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	commit := NewConventionalCommit("abc123", CommitTypeFeat, "add feature")

	cs.AddCommit(commit)

	if cs.CommitCount() != 1 {
		t.Errorf("CommitCount() = %d, want 1", cs.CommitCount())
	}
	if cs.IsEmpty() {
		t.Error("IsEmpty() = true, want false")
	}

	commits := cs.Commits()
	if len(commits) != 1 {
		t.Fatalf("Commits() length = %d, want 1", len(commits))
	}
	if commits[0].Hash() != "abc123" {
		t.Errorf("Commits()[0].Hash() = %v, want abc123", commits[0].Hash())
	}
}

func TestChangeSet_AddCommits(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	commits := []*ConventionalCommit{
		NewConventionalCommit("abc123", CommitTypeFeat, "add feature"),
		NewConventionalCommit("def456", CommitTypeFix, "fix bug"),
		NewConventionalCommit("ghi789", CommitTypeDocs, "update docs"),
	}

	cs.AddCommits(commits)

	if cs.CommitCount() != 3 {
		t.Errorf("CommitCount() = %d, want 3", cs.CommitCount())
	}
}

func TestChangeSet_Categories(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeFeat, "feature 1"),
		NewConventionalCommit("2", CommitTypeFeat, "feature 2"),
		NewConventionalCommit("3", CommitTypeFix, "fix 1"),
		NewConventionalCommit("4", CommitTypePerf, "perf 1"),
		NewConventionalCommit("5", CommitTypeDocs, "docs 1"),
		NewConventionalCommit("6", CommitTypeRefactor, "refactor 1"),
		NewConventionalCommit("7", CommitTypeTest, "test 1"),
		NewConventionalCommit("8", CommitTypeBuild, "build 1"),
		NewConventionalCommit("9", CommitTypeCI, "ci 1"),
		NewConventionalCommit("10", CommitTypeChore, "chore 1"),
		NewConventionalCommit("11", CommitTypeRevert, "revert 1"),
		NewConventionalCommit("12", CommitTypeFeat, "breaking", WithBreaking("breaks API")),
	})

	cats := cs.Categories()

	if len(cats.Features) != 3 {
		t.Errorf("Features length = %d, want 3", len(cats.Features))
	}
	if len(cats.Fixes) != 1 {
		t.Errorf("Fixes length = %d, want 1", len(cats.Fixes))
	}
	if len(cats.Perf) != 1 {
		t.Errorf("Perf length = %d, want 1", len(cats.Perf))
	}
	if len(cats.Docs) != 1 {
		t.Errorf("Docs length = %d, want 1", len(cats.Docs))
	}
	if len(cats.Refactors) != 1 {
		t.Errorf("Refactors length = %d, want 1", len(cats.Refactors))
	}
	if len(cats.Tests) != 1 {
		t.Errorf("Tests length = %d, want 1", len(cats.Tests))
	}
	if len(cats.Build) != 1 {
		t.Errorf("Build length = %d, want 1", len(cats.Build))
	}
	if len(cats.CI) != 1 {
		t.Errorf("CI length = %d, want 1", len(cats.CI))
	}
	if len(cats.Chores) != 1 {
		t.Errorf("Chores length = %d, want 1", len(cats.Chores))
	}
	if len(cats.Reverts) != 1 {
		t.Errorf("Reverts length = %d, want 1", len(cats.Reverts))
	}
	if len(cats.Breaking) != 1 {
		t.Errorf("Breaking length = %d, want 1", len(cats.Breaking))
	}
}

func TestChangeSet_CategoriesWithOther(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")

	// Parse a commit message that will have an unknown type
	// Use ParseConventionalCommit to create a commit with an invalid type
	// We'll test by having a commit with style type, then filtering for Other
	// Actually, style is valid - let's test using a parsed unknown type commit

	// Create a commit by parsing - unknown types get parsed but aren't in the switch
	commit := ParseConventionalCommit("abc123", "unknown: some commit message")
	if commit != nil {
		cs.AddCommit(commit)
	}

	cats := cs.Categories()
	// Unknown types go to Other
	if len(cats.Other) != 1 {
		t.Errorf("Other length = %d, want 1", len(cats.Other))
	}
}

func TestChangeSet_CategoriesCaching(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommit(NewConventionalCommit("1", CommitTypeFeat, "feature"))

	// First call should categorize
	cats1 := cs.Categories()
	if len(cats1.Features) != 1 {
		t.Errorf("Features length = %d, want 1", len(cats1.Features))
	}

	// Second call should return cached result (same pointer)
	cats2 := cs.Categories()
	if cats1 != cats2 {
		t.Error("Categories() should return cached result")
	}

	// Adding commit after Categories() is called will NOT update the categorization
	// (as documented in AddCommit). This is by design for thread-safety with sync.Once.
	cs.AddCommit(NewConventionalCommit("2", CommitTypeFeat, "feature 2"))
	cats3 := cs.Categories()
	// Categories remain the same - only the original commit is categorized
	if cats3 != cats1 {
		t.Error("Categories() should return the same cached result")
	}
	if len(cats3.Features) != 1 {
		t.Errorf("Features length = %d, want 1 (commit added after categorization)", len(cats3.Features))
	}

	// Commits() still returns all commits including the one added later
	if cs.CommitCount() != 2 {
		t.Errorf("CommitCount = %d, want 2", cs.CommitCount())
	}
}

func TestChangeSet_ReleaseType(t *testing.T) {
	tests := []struct {
		name     string
		commits  []*ConventionalCommit
		expected ReleaseType
	}{
		{
			name:     "empty changeset",
			commits:  nil,
			expected: ReleaseTypeNone,
		},
		{
			name: "only docs",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeDocs, "docs"),
			},
			expected: ReleaseTypeNone,
		},
		{
			name: "patch release",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeFix, "fix"),
			},
			expected: ReleaseTypePatch,
		},
		{
			name: "minor release",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeFeat, "feature"),
			},
			expected: ReleaseTypeMinor,
		},
		{
			name: "major release",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeFeat, "feature", WithBreaking("breaks")),
			},
			expected: ReleaseTypeMajor,
		},
		{
			name: "mixed - highest wins",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeDocs, "docs"),
				NewConventionalCommit("2", CommitTypeFix, "fix"),
				NewConventionalCommit("3", CommitTypeFeat, "feature"),
			},
			expected: ReleaseTypeMinor,
		},
		{
			name: "breaking overrides all",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeFeat, "feature"),
				NewConventionalCommit("2", CommitTypeFix, "breaking fix", WithBreaking("breaks")),
			},
			expected: ReleaseTypeMajor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
			if tt.commits != nil {
				cs.AddCommits(tt.commits)
			}

			got := cs.ReleaseType()
			if got != tt.expected {
				t.Errorf("ReleaseType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChangeSet_HasBreakingChanges(t *testing.T) {
	tests := []struct {
		name     string
		commits  []*ConventionalCommit
		expected bool
	}{
		{
			name:     "empty",
			commits:  nil,
			expected: false,
		},
		{
			name: "no breaking",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeFeat, "feature"),
			},
			expected: false,
		},
		{
			name: "has breaking",
			commits: []*ConventionalCommit{
				NewConventionalCommit("1", CommitTypeFeat, "feature", WithBreaking("breaks")),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
			if tt.commits != nil {
				cs.AddCommits(tt.commits)
			}

			if got := cs.HasBreakingChanges(); got != tt.expected {
				t.Errorf("HasBreakingChanges() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChangeSet_HasFeatures(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")

	if cs.HasFeatures() {
		t.Error("HasFeatures() = true for empty changeset, want false")
	}

	cs.AddCommit(NewConventionalCommit("1", CommitTypeFix, "fix"))
	if cs.HasFeatures() {
		t.Error("HasFeatures() = true for fix-only changeset, want false")
	}

	cs.AddCommit(NewConventionalCommit("2", CommitTypeFeat, "feature"))
	if !cs.HasFeatures() {
		t.Error("HasFeatures() = false, want true")
	}
}

func TestChangeSet_HasFixes(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")

	if cs.HasFixes() {
		t.Error("HasFixes() = true for empty changeset, want false")
	}

	cs.AddCommit(NewConventionalCommit("1", CommitTypeFeat, "feat"))
	if cs.HasFixes() {
		t.Error("HasFixes() = true for feat-only changeset, want false")
	}

	cs.AddCommit(NewConventionalCommit("2", CommitTypeFix, "fix"))
	if !cs.HasFixes() {
		t.Error("HasFixes() = false, want true")
	}
}

func TestChangeSet_ChangelogCommits(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeFeat, "feature"),
		NewConventionalCommit("2", CommitTypeFix, "fix"),
		NewConventionalCommit("3", CommitTypeDocs, "docs"),
		NewConventionalCommit("4", CommitTypePerf, "perf"),
		NewConventionalCommit("5", CommitTypeChore, "chore"),
	})

	changelogCommits := cs.ChangelogCommits()

	// feat, fix, and perf should be included
	if len(changelogCommits) != 3 {
		t.Errorf("ChangelogCommits() length = %d, want 3", len(changelogCommits))
	}

	types := make(map[CommitType]bool)
	for _, c := range changelogCommits {
		types[c.Type()] = true
	}

	if !types[CommitTypeFeat] {
		t.Error("ChangelogCommits() should include feat")
	}
	if !types[CommitTypeFix] {
		t.Error("ChangelogCommits() should include fix")
	}
	if !types[CommitTypePerf] {
		t.Error("ChangelogCommits() should include perf")
	}
	if types[CommitTypeDocs] {
		t.Error("ChangelogCommits() should not include docs")
	}
	if types[CommitTypeChore] {
		t.Error("ChangelogCommits() should not include chore")
	}
}

func TestChangeSet_SortByDate(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")

	now := time.Now()
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeFeat, "oldest", WithDate(now.Add(-2*time.Hour))),
		NewConventionalCommit("2", CommitTypeFeat, "newest", WithDate(now)),
		NewConventionalCommit("3", CommitTypeFeat, "middle", WithDate(now.Add(-1*time.Hour))),
	})

	cs.SortByDate()

	commits := cs.Commits()
	if commits[0].Subject() != "newest" {
		t.Errorf("First commit should be newest, got %s", commits[0].Subject())
	}
	if commits[1].Subject() != "middle" {
		t.Errorf("Second commit should be middle, got %s", commits[1].Subject())
	}
	if commits[2].Subject() != "oldest" {
		t.Errorf("Third commit should be oldest, got %s", commits[2].Subject())
	}
}

func TestChangeSet_SortByType(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeDocs, "docs"),
		NewConventionalCommit("2", CommitTypeFix, "fix"),
		NewConventionalCommit("3", CommitTypeFeat, "feature"),
		NewConventionalCommit("4", CommitTypeFeat, "breaking", WithBreaking("breaks")),
	})

	cs.SortByType()

	commits := cs.Commits()

	// Breaking should be first (release type Major)
	if !commits[0].IsBreaking() {
		t.Errorf("First commit should be breaking, got %s", commits[0].Subject())
	}

	// Verify that higher release types come before lower ones
	// Breaking (Major) > feat (Minor) > fix (Patch) > docs (None)
	for i := 0; i < len(commits)-1; i++ {
		curr := commits[i]
		next := commits[i+1]

		// If current is breaking and next is not, that's correct
		if curr.IsBreaking() && !next.IsBreaking() {
			continue
		}
		// If neither is breaking, check release type priority
		if !curr.IsBreaking() && !next.IsBreaking() {
			if curr.ReleaseType() < next.ReleaseType() {
				t.Errorf("Commit %d (%s, %s) should not come before commit %d (%s, %s)",
					i, curr.Type(), curr.ReleaseType(),
					i+1, next.Type(), next.ReleaseType())
			}
		}
	}
}

func TestChangeSet_FilterByScope(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeFeat, "api feature", WithScope("api")),
		NewConventionalCommit("2", CommitTypeFeat, "ui feature", WithScope("ui")),
		NewConventionalCommit("3", CommitTypeFix, "api fix", WithScope("api")),
		NewConventionalCommit("4", CommitTypeFix, "no scope"),
	})

	apiChanges := cs.FilterByScope("api")

	if apiChanges.CommitCount() != 2 {
		t.Errorf("FilterByScope('api') count = %d, want 2", apiChanges.CommitCount())
	}

	for _, c := range apiChanges.Commits() {
		if c.Scope() != "api" {
			t.Errorf("FilterByScope('api') returned commit with scope %q", c.Scope())
		}
	}

	// Verify new changeset has modified ID
	if apiChanges.ID() != "changeset-1-filtered" {
		t.Errorf("Filtered changeset ID = %s, want changeset-1-filtered", apiChanges.ID())
	}
}

func TestChangeSet_Scopes(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeFeat, "api feature", WithScope("api")),
		NewConventionalCommit("2", CommitTypeFeat, "ui feature", WithScope("ui")),
		NewConventionalCommit("3", CommitTypeFix, "api fix", WithScope("api")), // Duplicate
		NewConventionalCommit("4", CommitTypeFix, "no scope"),
	})

	scopes := cs.Scopes()

	// Should have 2 unique scopes (api, ui), sorted
	if len(scopes) != 2 {
		t.Errorf("Scopes() length = %d, want 2", len(scopes))
	}

	// Should be sorted alphabetically
	if len(scopes) >= 2 {
		if scopes[0] != "api" || scopes[1] != "ui" {
			t.Errorf("Scopes() = %v, want [api, ui]", scopes)
		}
	}
}

func TestChangeSet_ScopesEmpty(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommit(NewConventionalCommit("1", CommitTypeFeat, "no scope"))

	scopes := cs.Scopes()
	if len(scopes) != 0 {
		t.Errorf("Scopes() length = %d for commits without scope, want 0", len(scopes))
	}
}

func TestChangeSet_Summary(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommits([]*ConventionalCommit{
		NewConventionalCommit("1", CommitTypeFeat, "feature 1", WithScope("api")),
		NewConventionalCommit("2", CommitTypeFeat, "feature 2", WithScope("ui")),
		NewConventionalCommit("3", CommitTypeFix, "fix 1"),
		NewConventionalCommit("4", CommitTypePerf, "perf 1"),
		NewConventionalCommit("5", CommitTypeDocs, "docs 1"),
		NewConventionalCommit("6", CommitTypeRefactor, "refactor 1"),
		NewConventionalCommit("7", CommitTypeTest, "test 1"),
		NewConventionalCommit("8", CommitTypeChore, "chore 1"),
		NewConventionalCommit("9", CommitTypeFeat, "breaking", WithBreaking("breaks")),
	})

	summary := cs.Summary()

	if summary.TotalCommits != 9 {
		t.Errorf("TotalCommits = %d, want 9", summary.TotalCommits)
	}
	if summary.Features != 3 {
		t.Errorf("Features = %d, want 3", summary.Features)
	}
	if summary.Fixes != 1 {
		t.Errorf("Fixes = %d, want 1", summary.Fixes)
	}
	if summary.Breaking != 1 {
		t.Errorf("Breaking = %d, want 1", summary.Breaking)
	}
	if summary.Performance != 1 {
		t.Errorf("Performance = %d, want 1", summary.Performance)
	}
	if summary.Documentation != 1 {
		t.Errorf("Documentation = %d, want 1", summary.Documentation)
	}
	if summary.Refactoring != 1 {
		t.Errorf("Refactoring = %d, want 1", summary.Refactoring)
	}
	if summary.Tests != 1 {
		t.Errorf("Tests = %d, want 1", summary.Tests)
	}
	if summary.Other != 1 { // chore
		t.Errorf("Other = %d, want 1", summary.Other)
	}
	if summary.ReleaseType != ReleaseTypeMajor {
		t.Errorf("ReleaseType = %v, want major", summary.ReleaseType)
	}
	if len(summary.Scopes) != 2 {
		t.Errorf("Scopes length = %d, want 2", len(summary.Scopes))
	}
}

func TestChangeSet_SummaryEmpty(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	summary := cs.Summary()

	if summary.TotalCommits != 0 {
		t.Errorf("TotalCommits = %d, want 0", summary.TotalCommits)
	}
	if summary.ReleaseType != ReleaseTypeNone {
		t.Errorf("ReleaseType = %v, want none", summary.ReleaseType)
	}
}

func TestChangeSet_SortDoesNotAffectCategories(t *testing.T) {
	cs := NewChangeSet("changeset-1", "v1.0.0", "HEAD")
	cs.AddCommit(NewConventionalCommit("1", CommitTypeFeat, "feature"))
	cs.AddCommit(NewConventionalCommit("2", CommitTypeFix, "fix"))

	// Categorize first
	cats1 := cs.Categories()
	if len(cats1.Features) != 1 || len(cats1.Fixes) != 1 {
		t.Fatal("Initial categorization should have 1 feature and 1 fix")
	}

	// Sort - categories should remain the same (based on commit types, not order)
	cs.SortByDate()
	cats2 := cs.Categories()
	if len(cats2.Features) != 1 || len(cats2.Fixes) != 1 {
		t.Error("SortByDate should not affect categories")
	}

	cs.SortByType()
	cats3 := cs.Categories()
	if len(cats3.Features) != 1 || len(cats3.Fixes) != 1 {
		t.Error("SortByType should not affect categories")
	}
}
