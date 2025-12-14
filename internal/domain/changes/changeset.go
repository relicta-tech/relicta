// Package changes provides domain types for analyzing commit changes.
package changes

import (
	"sort"
	"sync"
	"time"
)

// ChangeSetID uniquely identifies a changeset.
type ChangeSetID string

// ChangeSet is an aggregate root representing a collection of commits
// that form a potential release.
type ChangeSet struct {
	id      ChangeSetID
	commits []*ConventionalCommit

	// Cached categorization (protected by mu for commits, categorizeOnce for categories)
	mu             sync.RWMutex
	categorizeOnce sync.Once
	categories     *Categories

	// Metadata
	fromRef   string
	toRef     string
	createdAt time.Time
}

// Categories holds commits organized by type.
type Categories struct {
	Features  []*ConventionalCommit
	Fixes     []*ConventionalCommit
	Breaking  []*ConventionalCommit
	Perf      []*ConventionalCommit
	Docs      []*ConventionalCommit
	Refactors []*ConventionalCommit
	Tests     []*ConventionalCommit
	Build     []*ConventionalCommit
	CI        []*ConventionalCommit
	Chores    []*ConventionalCommit
	Reverts   []*ConventionalCommit
	Other     []*ConventionalCommit
}

// NewChangeSet creates a new ChangeSet aggregate.
func NewChangeSet(id ChangeSetID, fromRef, toRef string) *ChangeSet {
	return &ChangeSet{
		id:        id,
		fromRef:   fromRef,
		toRef:     toRef,
		commits:   make([]*ConventionalCommit, 0),
		createdAt: time.Now(),
	}
}

// ID returns the changeset ID.
func (cs *ChangeSet) ID() ChangeSetID {
	return cs.id
}

// FromRef returns the starting git reference.
func (cs *ChangeSet) FromRef() string {
	return cs.fromRef
}

// ToRef returns the ending git reference.
func (cs *ChangeSet) ToRef() string {
	return cs.toRef
}

// CreatedAt returns when the changeset was created.
func (cs *ChangeSet) CreatedAt() time.Time {
	return cs.createdAt
}

// AddCommit adds a commit to the changeset.
// This invalidates any cached categorization to ensure consistency.
func (cs *ChangeSet) AddCommit(commit *ConventionalCommit) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.commits = append(cs.commits, commit)
	cs.invalidateCategoriesLocked()
}

// AddCommits adds multiple commits to the changeset.
// This invalidates any cached categorization to ensure consistency.
func (cs *ChangeSet) AddCommits(commits []*ConventionalCommit) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.commits = append(cs.commits, commits...)
	cs.invalidateCategoriesLocked()
}

// InvalidateCategories resets the cached categorization.
// This allows re-categorization on the next call to Categories().
func (cs *ChangeSet) InvalidateCategories() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.invalidateCategoriesLocked()
}

// invalidateCategoriesLocked resets categorization cache.
// Thread-safety: Caller MUST hold cs.mu lock before calling (indicated by _Locked suffix).
// Note: Resetting sync.Once by assigning a zero value is an intentional pattern
// for cache invalidation. This allows Categories() to re-run categorize() on the
// next call after commits are added.
func (cs *ChangeSet) invalidateCategoriesLocked() {
	cs.categorizeOnce = sync.Once{} // Reset to allow re-categorization
	cs.categories = nil             // Clear cached categories
}

// Commits returns all commits in the changeset.
// This method is safe for concurrent access.
func (cs *ChangeSet) Commits() []*ConventionalCommit {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	// Return a copy to prevent external mutation
	result := make([]*ConventionalCommit, len(cs.commits))
	copy(result, cs.commits)
	return result
}

// CommitCount returns the number of commits.
// This method is safe for concurrent access.
func (cs *ChangeSet) CommitCount() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.commits)
}

// IsEmpty returns true if there are no commits.
// This method is safe for concurrent access.
func (cs *ChangeSet) IsEmpty() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.commits) == 0
}

// Categories returns commits organized by type.
// This method is safe for concurrent access and uses sync.Once for efficient initialization.
// Note: Categorization is done once; adding commits after this is called won't update categories.
func (cs *ChangeSet) Categories() *Categories {
	cs.categorizeOnce.Do(func() {
		cs.categorize()
	})
	return cs.categories
}

// categorize organizes commits into categories.
// Pre-allocates slices based on typical commit distributions to reduce allocations.
func (cs *ChangeSet) categorize() {
	cs.mu.RLock()
	commitCount := len(cs.commits)
	cs.mu.RUnlock()

	// Pre-allocate based on typical distribution:
	// ~40% features, ~30% fixes, ~5% breaking, ~5% perf, ~5% docs, ~5% refactors, ~10% other
	cs.categories = &Categories{
		Features:  make([]*ConventionalCommit, 0, commitCount*4/10+1),
		Fixes:     make([]*ConventionalCommit, 0, commitCount*3/10+1),
		Breaking:  make([]*ConventionalCommit, 0, commitCount/20+1),
		Perf:      make([]*ConventionalCommit, 0, commitCount/20+1),
		Docs:      make([]*ConventionalCommit, 0, commitCount/20+1),
		Refactors: make([]*ConventionalCommit, 0, commitCount/20+1),
		Tests:     make([]*ConventionalCommit, 0, commitCount/20+1),
		Build:     make([]*ConventionalCommit, 0, commitCount/40+1),
		CI:        make([]*ConventionalCommit, 0, commitCount/40+1),
		Chores:    make([]*ConventionalCommit, 0, commitCount/20+1),
		Reverts:   make([]*ConventionalCommit, 0, commitCount/40+1),
		Other:     make([]*ConventionalCommit, 0, commitCount/20+1),
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, c := range cs.commits {
		// Breaking changes get their own category
		if c.IsBreaking() {
			cs.categories.Breaking = append(cs.categories.Breaking, c)
		}

		// Also categorize by type
		switch c.Type() {
		case CommitTypeFeat:
			cs.categories.Features = append(cs.categories.Features, c)
		case CommitTypeFix:
			cs.categories.Fixes = append(cs.categories.Fixes, c)
		case CommitTypePerf:
			cs.categories.Perf = append(cs.categories.Perf, c)
		case CommitTypeDocs:
			cs.categories.Docs = append(cs.categories.Docs, c)
		case CommitTypeRefactor:
			cs.categories.Refactors = append(cs.categories.Refactors, c)
		case CommitTypeTest:
			cs.categories.Tests = append(cs.categories.Tests, c)
		case CommitTypeBuild:
			cs.categories.Build = append(cs.categories.Build, c)
		case CommitTypeCI:
			cs.categories.CI = append(cs.categories.CI, c)
		case CommitTypeChore:
			cs.categories.Chores = append(cs.categories.Chores, c)
		case CommitTypeRevert:
			cs.categories.Reverts = append(cs.categories.Reverts, c)
		default:
			cs.categories.Other = append(cs.categories.Other, c)
		}
	}
}

// ReleaseType determines the release type based on all commits.
// This method is safe for concurrent access.
func (cs *ChangeSet) ReleaseType() ReleaseType {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := ReleaseTypeNone

	for _, c := range cs.commits {
		result = MaxReleaseType(result, c.ReleaseType())
		if result == ReleaseTypeMajor {
			break // Can't go higher
		}
	}

	return result
}

// HasBreakingChanges returns true if any commit has breaking changes.
// This method is safe for concurrent access.
func (cs *ChangeSet) HasBreakingChanges() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, c := range cs.commits {
		if c.IsBreaking() {
			return true
		}
	}
	return false
}

// HasFeatures returns true if any commit adds features.
// This method is safe for concurrent access.
func (cs *ChangeSet) HasFeatures() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, c := range cs.commits {
		if c.Type() == CommitTypeFeat {
			return true
		}
	}
	return false
}

// HasFixes returns true if any commit fixes bugs.
// This method is safe for concurrent access.
func (cs *ChangeSet) HasFixes() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for _, c := range cs.commits {
		if c.Type() == CommitTypeFix {
			return true
		}
	}
	return false
}

// ChangelogCommits returns commits that should appear in the changelog.
// This method is safe for concurrent access.
func (cs *ChangeSet) ChangelogCommits() []*ConventionalCommit {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]*ConventionalCommit, 0, len(cs.commits)/2)
	for _, c := range cs.commits {
		if c.AffectsChangelog() {
			result = append(result, c)
		}
	}
	return result
}

// SortByDate sorts commits by date (newest first).
// Note: Categorization is based on commit properties, not order, so sorting is safe after Categories().
func (cs *ChangeSet) SortByDate() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	sort.Slice(cs.commits, func(i, j int) bool {
		return cs.commits[i].Date().After(cs.commits[j].Date())
	})
}

// SortByType sorts commits by type priority (breaking first, then features, etc.).
// Note: Categorization is based on commit properties, not order, so sorting is safe after Categories().
func (cs *ChangeSet) SortByType() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	sort.Slice(cs.commits, func(i, j int) bool {
		// Breaking changes first
		if cs.commits[i].IsBreaking() != cs.commits[j].IsBreaking() {
			return cs.commits[i].IsBreaking()
		}
		// Then by release type priority
		return cs.commits[i].ReleaseType() > cs.commits[j].ReleaseType()
	})
}

// FilterByScope returns a new changeset containing only commits with the given scope.
// This method is safe for concurrent access.
func (cs *ChangeSet) FilterByScope(scope string) *ChangeSet {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	filtered := NewChangeSet(cs.id+"-filtered", cs.fromRef, cs.toRef)
	for _, c := range cs.commits {
		if c.Scope() == scope {
			filtered.AddCommit(c)
		}
	}
	return filtered
}

// Scopes returns all unique scopes in the changeset.
// This method is safe for concurrent access.
func (cs *ChangeSet) Scopes() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	scopeSet := make(map[string]struct{})
	for _, c := range cs.commits {
		if c.Scope() != "" {
			scopeSet[c.Scope()] = struct{}{}
		}
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes
}

// Summary returns a summary of the changeset.
type ChangeSetSummary struct {
	TotalCommits  int
	Features      int
	Fixes         int
	Breaking      int
	Performance   int
	Documentation int
	Refactoring   int
	Tests         int
	Other         int
	ReleaseType   ReleaseType
	Scopes        []string
}

// Summary returns a summary of the changeset.
// This method is safe for concurrent access.
func (cs *ChangeSet) Summary() ChangeSetSummary {
	cats := cs.Categories()
	commitCount := cs.CommitCount()
	releaseType := cs.ReleaseType()
	scopes := cs.Scopes()

	return ChangeSetSummary{
		TotalCommits:  commitCount,
		Features:      len(cats.Features),
		Fixes:         len(cats.Fixes),
		Breaking:      len(cats.Breaking),
		Performance:   len(cats.Perf),
		Documentation: len(cats.Docs),
		Refactoring:   len(cats.Refactors),
		Tests:         len(cats.Tests),
		Other:         len(cats.Chores) + len(cats.CI) + len(cats.Build) + len(cats.Other),
		ReleaseType:   releaseType,
		Scopes:        scopes,
	}
}
