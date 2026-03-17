package git

import (
	"testing"
)

// Benchmark data representing typical commit messages.
var benchCommitMessages = []string{
	"feat(auth): add OAuth2 support",
	"fix: resolve null pointer exception in user service",
	"feat!: redesign API with breaking changes",
	"docs: update README with installation instructions",
	"chore(deps): bump dependencies",
	"feat(ui): add dark mode support\n\nThis adds a new dark mode theme that can be toggled in settings.\n\nFixes #123",
	"fix(api): handle edge case in pagination\n\nBREAKING CHANGE: pagination now uses cursor-based approach",
	"refactor: simplify error handling logic",
	"perf: optimize database queries",
	"test: add unit tests for auth module",
}

func BenchmarkParseConventionalCommit(b *testing.B) {
	b.ReportAllocs()

	b.Run("simple", func(b *testing.B) {
		msg := "feat(auth): add OAuth2 support"
		for i := 0; i < b.N; i++ {
			_, _ = ParseConventionalCommit(msg)
		}
	})

	b.Run("with_body", func(b *testing.B) {
		msg := "feat(ui): add dark mode support\n\nThis adds a new dark mode theme that can be toggled in settings.\n\nFixes #123"
		for i := 0; i < b.N; i++ {
			_, _ = ParseConventionalCommit(msg)
		}
	})

	b.Run("breaking", func(b *testing.B) {
		msg := "feat!: redesign API with breaking changes"
		for i := 0; i < b.N; i++ {
			_, _ = ParseConventionalCommit(msg)
		}
	})

	b.Run("breaking_footer", func(b *testing.B) {
		msg := "fix(api): handle edge case in pagination\n\nBREAKING CHANGE: pagination now uses cursor-based approach"
		for i := 0; i < b.N; i++ {
			_, _ = ParseConventionalCommit(msg)
		}
	})

	b.Run("non_conventional", func(b *testing.B) {
		msg := "Updated the user service to fix bug"
		for i := 0; i < b.N; i++ {
			_, _ = ParseConventionalCommit(msg)
		}
	})

	b.Run("mixed_batch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			msg := benchCommitMessages[i%len(benchCommitMessages)]
			_, _ = ParseConventionalCommit(msg)
		}
	})
}

func BenchmarkDetectReleaseType(b *testing.B) {
	b.ReportAllocs()

	// Create test commit sets
	featureCommits := []ConventionalCommit{
		{Type: CommitTypeFeat, Description: "add new feature 1"},
		{Type: CommitTypeFeat, Description: "add new feature 2"},
		{Type: CommitTypeFix, Description: "fix bug"},
	}

	breakingCommits := []ConventionalCommit{
		{Type: CommitTypeFeat, Breaking: true, Description: "breaking change"},
		{Type: CommitTypeFix, Description: "fix bug"},
	}

	patchCommits := []ConventionalCommit{
		{Type: CommitTypeFix, Description: "fix bug 1"},
		{Type: CommitTypeFix, Description: "fix bug 2"},
		{Type: CommitTypePerf, Description: "performance improvement"},
	}

	mixedCommits := []ConventionalCommit{
		{Type: CommitTypeFeat, Description: "add feature"},
		{Type: CommitTypeFix, Description: "fix bug"},
		{Type: CommitTypeDocs, Description: "update docs"},
		{Type: CommitTypeChore, Description: "update deps"},
		{Type: CommitTypeTest, Description: "add tests"},
		{Type: CommitTypeRefactor, Description: "refactor code"},
	}

	// Large batch for stress testing
	largeCommits := make([]ConventionalCommit, 100)
	for i := 0; i < 100; i++ {
		if i%5 == 0 {
			largeCommits[i] = ConventionalCommit{Type: CommitTypeFeat, Description: "feature"}
		} else if i%3 == 0 {
			largeCommits[i] = ConventionalCommit{Type: CommitTypeFix, Description: "fix"}
		} else {
			largeCommits[i] = ConventionalCommit{Type: CommitTypeChore, Description: "chore"}
		}
	}

	b.Run("feature_release", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DetectReleaseType(featureCommits)
		}
	})

	b.Run("breaking_release", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DetectReleaseType(breakingCommits)
		}
	})

	b.Run("patch_release", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DetectReleaseType(patchCommits)
		}
	})

	b.Run("mixed_commits", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DetectReleaseType(mixedCommits)
		}
	})

	b.Run("large_batch_100", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DetectReleaseType(largeCommits)
		}
	})

	b.Run("empty", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DetectReleaseType(nil)
		}
	})
}

func BenchmarkCategorizedChanges_DetermineReleaseType(b *testing.B) {
	b.ReportAllocs()

	b.Run("with_breaking", func(b *testing.B) {
		changes := &CategorizedChanges{
			Breaking: []ConventionalCommit{{Description: "breaking change"}},
		}
		for i := 0; i < b.N; i++ {
			_ = changes.DetermineReleaseType()
		}
	})

	b.Run("with_features", func(b *testing.B) {
		changes := &CategorizedChanges{
			Features: []ConventionalCommit{{Description: "feature 1"}, {Description: "feature 2"}},
		}
		for i := 0; i < b.N; i++ {
			_ = changes.DetermineReleaseType()
		}
	})

	b.Run("with_fixes", func(b *testing.B) {
		changes := &CategorizedChanges{
			Fixes: []ConventionalCommit{{Description: "fix 1"}, {Description: "fix 2"}},
		}
		for i := 0; i < b.N; i++ {
			_ = changes.DetermineReleaseType()
		}
	})

	b.Run("mixed_full", func(b *testing.B) {
		changes := &CategorizedChanges{
			Features:      []ConventionalCommit{{Description: "feature"}},
			Fixes:         []ConventionalCommit{{Description: "fix"}},
			Documentation: []ConventionalCommit{{Description: "docs"}},
			Refactoring:   []ConventionalCommit{{Description: "refactor"}},
		}
		for i := 0; i < b.N; i++ {
			_ = changes.DetermineReleaseType()
		}
	})
}
