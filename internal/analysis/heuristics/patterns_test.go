package heuristics

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

func TestPatternDetector_ShouldSkip(t *testing.T) {
	detector := NewPatternDetector()

	tests := []struct {
		name       string
		commit     analysis.CommitInfo
		shouldSkip bool
		reason     string
	}{
		// Merge commits
		{
			name: "merge commit by parents",
			commit: analysis.CommitInfo{
				Subject:     "Update feature",
				ParentCount: 2,
			},
			shouldSkip: true,
			reason:     "merge commit",
		},
		{
			name: "merge commit by flag",
			commit: analysis.CommitInfo{
				Subject: "Update feature",
				IsMerge: true,
			},
			shouldSkip: true,
			reason:     "merge commit",
		},
		{
			name: "merge branch message",
			commit: analysis.CommitInfo{
				Subject: "Merge branch 'feature' into main",
			},
			shouldSkip: true,
			reason:     "merge branch",
		},
		{
			name: "merge pull request",
			commit: analysis.CommitInfo{
				Subject: "Merge pull request #123 from feature/auth",
			},
			shouldSkip: true,
			reason:     "merge pull request",
		},
		{
			name: "merged with different case",
			commit: analysis.CommitInfo{
				Subject: "Merged feature into main",
			},
			shouldSkip: true,
			reason:     "merge commit",
		},

		// Revert commits
		{
			name: "revert commit",
			commit: analysis.CommitInfo{
				Subject: `Revert "Add new feature"`,
			},
			shouldSkip: true,
			reason:     "revert commit",
		},

		// Initial commits
		{
			name: "initial commit",
			commit: analysis.CommitInfo{
				Subject: "Initial commit",
			},
			shouldSkip: true,
			reason:     "initial commit",
		},
		{
			name: "init only",
			commit: analysis.CommitInfo{
				Subject: "init",
			},
			shouldSkip: true,
			reason:     "initial commit",
		},
		{
			name: "first commit",
			commit: analysis.CommitInfo{
				Subject: "first commit",
			},
			shouldSkip: true,
			reason:     "initial commit",
		},

		// WIP commits
		{
			name: "wip lowercase",
			commit: analysis.CommitInfo{
				Subject: "wip",
			},
			shouldSkip: true,
			reason:     "work in progress",
		},
		{
			name: "wip uppercase",
			commit: analysis.CommitInfo{
				Subject: "WIP",
			},
			shouldSkip: true,
			reason:     "work in progress",
		},
		{
			name: "wip with colon",
			commit: analysis.CommitInfo{
				Subject: "wip: working on feature",
			},
			shouldSkip: true,
			reason:     "work in progress",
		},
		{
			name: "wip in brackets",
			commit: analysis.CommitInfo{
				Subject: "[WIP] Not ready yet",
			},
			shouldSkip: true,
			reason:     "work in progress",
		},
		{
			name: "fixup commit",
			commit: analysis.CommitInfo{
				Subject: "fixup! Previous commit",
			},
			shouldSkip: true,
			reason:     "fixup commit",
		},
		{
			name: "squash commit",
			commit: analysis.CommitInfo{
				Subject: "squash! Combine changes",
			},
			shouldSkip: true,
			reason:     "squash commit",
		},

		// Auto-generated commits
		{
			name: "auto-merge",
			commit: analysis.CommitInfo{
				Subject: "Auto-merge of feature branch",
			},
			shouldSkip: true,
			reason:     "auto-generated",
		},
		{
			name: "code review suggestion",
			commit: analysis.CommitInfo{
				Subject: "Apply suggestions from code review",
			},
			shouldSkip: true,
			reason:     "code review suggestion",
		},

		// Version/release commits
		{
			name: "version only",
			commit: analysis.CommitInfo{
				Subject: "1.2.3",
			},
			shouldSkip: true,
			reason:     "version tag",
		},
		{
			name: "version with v prefix",
			commit: analysis.CommitInfo{
				Subject: "v1.2.3",
			},
			shouldSkip: true,
			reason:     "version tag",
		},
		{
			name: "release commit",
			commit: analysis.CommitInfo{
				Subject: "Release v1.2.3",
			},
			shouldSkip: true,
			reason:     "release commit",
		},
		{
			name: "bump version",
			commit: analysis.CommitInfo{
				Subject: "Bump version to 2.0.0",
			},
			shouldSkip: true,
			reason:     "version bump",
		},
		{
			name: "release tag",
			commit: analysis.CommitInfo{
				Subject: "[release] Version 1.5.0",
			},
			shouldSkip: true,
			reason:     "release commit",
		},
		{
			name: "chore release",
			commit: analysis.CommitInfo{
				Subject: "chore(release): 1.2.3",
			},
			shouldSkip: true,
			reason:     "release commit",
		},

		// Bot commits
		{
			name: "bot tag",
			commit: analysis.CommitInfo{
				Subject: "[bot] Automated update",
			},
			shouldSkip: true,
			reason:     "bot commit",
		},
		{
			name: "dependabot",
			commit: analysis.CommitInfo{
				Subject: "dependabot[bot]: Bump lodash",
			},
			shouldSkip: true,
			reason:     "dependabot",
		},
		{
			name: "renovate",
			commit: analysis.CommitInfo{
				Subject: "renovate[bot]: Update dependencies",
			},
			shouldSkip: true,
			reason:     "renovate",
		},

		// Non-meaningful messages
		{
			name: "just update",
			commit: analysis.CommitInfo{
				Subject: "update",
			},
			shouldSkip: true,
			reason:     "non-descriptive",
		},
		{
			name: "just stuff",
			commit: analysis.CommitInfo{
				Subject: "stuff",
			},
			shouldSkip: true,
			reason:     "non-descriptive",
		},
		{
			name: "keyboard mash",
			commit: analysis.CommitInfo{
				Subject: "asdfff", // Matches ^asdf+$ pattern
			},
			shouldSkip: true,
			reason:     "non-descriptive",
		},
		{
			name: "temp",
			commit: analysis.CommitInfo{
				Subject: "temp",
			},
			shouldSkip: true,
			reason:     "non-descriptive",
		},
		{
			name: "dots only",
			commit: analysis.CommitInfo{
				Subject: "...",
			},
			shouldSkip: true,
			reason:     "non-descriptive",
		},
		{
			name: "dashes only",
			commit: analysis.CommitInfo{
				Subject: "---",
			},
			shouldSkip: true,
			reason:     "non-descriptive",
		},

		// Should NOT skip
		{
			name: "regular fix",
			commit: analysis.CommitInfo{
				Subject: "Fix authentication bug",
			},
			shouldSkip: false,
		},
		{
			name: "regular feature",
			commit: analysis.CommitInfo{
				Subject: "Add user dashboard",
			},
			shouldSkip: false,
		},
		{
			name: "update with context",
			commit: analysis.CommitInfo{
				Subject: "Update user service to handle edge cases",
			},
			shouldSkip: false,
		},
		{
			name: "descriptive commit",
			commit: analysis.CommitInfo{
				Subject: "Implement caching for API responses",
			},
			shouldSkip: false,
		},
		{
			name: "fix with details",
			commit: analysis.CommitInfo{
				Subject: "fix: resolve login timeout issue",
			},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip, reason := detector.ShouldSkip(tt.commit)

			if skip != tt.shouldSkip {
				t.Errorf("ShouldSkip() = %v, want %v for subject: %q",
					skip, tt.shouldSkip, tt.commit.Subject)
			}

			if tt.shouldSkip && tt.reason != "" && reason != tt.reason {
				t.Errorf("ShouldSkip() reason = %q, want %q for subject: %q",
					reason, tt.reason, tt.commit.Subject)
			}
		})
	}
}

func TestPatternDetector_ClassifyByDiffSize(t *testing.T) {
	detector := NewPatternDetector()

	tests := []struct {
		name          string
		stats         analysis.DiffStats
		expectedType  changes.CommitType
		minConfidence float64
		shouldMatch   bool
	}{
		// Very small changes
		{
			name:          "tiny fix (2 lines)",
			stats:         analysis.DiffStats{Additions: 1, Deletions: 1, FilesChanged: 1},
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.45,
			shouldMatch:   true,
		},
		{
			name:          "small fix (5 lines)",
			stats:         analysis.DiffStats{Additions: 3, Deletions: 2, FilesChanged: 1},
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.45,
			shouldMatch:   true,
		},

		// Small changes
		{
			name:          "small change (10 lines)",
			stats:         analysis.DiffStats{Additions: 6, Deletions: 4, FilesChanged: 1},
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.40,
			shouldMatch:   true,
		},
		{
			name:          "small change (15 lines)",
			stats:         analysis.DiffStats{Additions: 10, Deletions: 5, FilesChanged: 2},
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.40,
			shouldMatch:   true,
		},

		// Large additions - feature
		{
			name:          "large additions, minimal deletions",
			stats:         analysis.DiffStats{Additions: 75, Deletions: 5, FilesChanged: 3},
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.50,
			shouldMatch:   true,
		},
		{
			name:          "very large additions",
			stats:         analysis.DiffStats{Additions: 250, Deletions: 30, FilesChanged: 5},
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.60,
			shouldMatch:   true,
		},

		// Balanced changes - refactor
		{
			name:          "balanced changes",
			stats:         analysis.DiffStats{Additions: 50, Deletions: 45, FilesChanged: 4},
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.45,
			shouldMatch:   true,
		},
		{
			name:          "more deletions than additions",
			stats:         analysis.DiffStats{Additions: 15, Deletions: 50, FilesChanged: 3},
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.40,
			shouldMatch:   true,
		},

		// Edge cases
		{
			name:        "no changes",
			stats:       analysis.DiffStats{Additions: 0, Deletions: 0, FilesChanged: 0},
			shouldMatch: false,
		},
		{
			name:          "single file moderate change",
			stats:         analysis.DiffStats{Additions: 20, Deletions: 10, FilesChanged: 1},
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.35,
			shouldMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.ClassifyByDiffSize(tt.stats)

			if tt.shouldMatch {
				if result == nil {
					t.Errorf("expected match but got nil for stats: %+v", tt.stats)
					return
				}

				if result.Type != tt.expectedType {
					t.Errorf("expected type %s, got %s for stats: %+v",
						tt.expectedType, result.Type, tt.stats)
				}

				if result.Confidence < tt.minConfidence {
					t.Errorf("expected confidence >= %.2f, got %.2f for stats: %+v",
						tt.minConfidence, result.Confidence, tt.stats)
				}
			} else {
				if result != nil {
					t.Errorf("expected no match but got %s (%.2f) for stats: %+v",
						result.Type, result.Confidence, tt.stats)
				}
			}
		})
	}
}

func TestHeuristicsAnalyzer_Classify(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	tests := []struct {
		name          string
		commit        analysis.CommitInfo
		expectedType  changes.CommitType
		expectedSkip  bool
		minConfidence float64
		shouldMatch   bool
	}{
		// Keywords should win for good commit messages
		{
			name: "fix in message",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "fix authentication bug",
				Files:   []string{"auth.go"},
			},
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name: "feature in message",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "add user dashboard",
				Files:   []string{"dashboard.tsx"},
			},
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.80,
			shouldMatch:   true,
		},

		// Path detection when message doesn't match keywords
		// Note: "update", "stuff", etc. are skip patterns, so use different vague messages
		{
			name: "vague message but test files",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "misc work on tests", // Has "test" keyword, matches files
				Files:   []string{"user_test.go", "auth_test.go"},
			},
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.70, // "tests" keyword detection
			shouldMatch:   true,
		},
		{
			name: "vague message but docs",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "various changes", // Not a skip pattern
				Files:   []string{"README.md", "CHANGELOG.md"},
			},
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name: "vague message but CI files",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "minor adjustments", // Not a skip pattern
				Files:   []string{".github/workflows/ci.yml"},
			},
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.85,
			shouldMatch:   true,
		},

		// Skip patterns
		{
			name: "merge commit",
			commit: analysis.CommitInfo{
				Hash:        sourcecontrol.CommitHash("abc123"),
				Subject:     "Merge branch 'feature'",
				ParentCount: 2,
			},
			expectedSkip: true,
			shouldMatch:  true,
		},
		{
			name: "wip commit",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "WIP",
			},
			expectedSkip: true,
			shouldMatch:  true,
		},

		// Diff size fallback
		{
			name: "large additions with vague message",
			commit: analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: "changes",
				Files:   []string{"feature.go"},
				Stats:   analysis.DiffStats{Additions: 100, Deletions: 5, FilesChanged: 1},
			},
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.45,
			shouldMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Classify(tt.commit)

			if !tt.shouldMatch {
				if result != nil && result.Confidence > 0.5 {
					t.Errorf("expected no confident match but got %s (%.2f)",
						result.Type, result.Confidence)
				}
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if tt.expectedSkip {
				if !result.ShouldSkip {
					t.Errorf("expected skip=true but got skip=false for: %q",
						tt.commit.Subject)
				}
				return
			}

			if result.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s for: %q",
					tt.expectedType, result.Type, tt.commit.Subject)
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("expected confidence >= %.2f, got %.2f for: %q",
					tt.minConfidence, result.Confidence, tt.commit.Subject)
			}
		})
	}
}

func TestHeuristicsAnalyzer_CustomKeywords(t *testing.T) {
	customKeywords := map[changes.CommitType][]string{
		changes.CommitTypeFeat: {"JIRA-123", "story"},
		changes.CommitTypeFix:  {"ticket", "resolve-issue"},
	}

	analyzer := NewAnalyzer(customKeywords)

	tests := []struct {
		name         string
		message      string
		expectedType changes.CommitType
	}{
		{
			name:         "custom feature keyword",
			message:      "JIRA-123: implement user login",
			expectedType: changes.CommitTypeFeat,
		},
		{
			name:         "custom story keyword",
			message:      "story: add dashboard feature",
			expectedType: changes.CommitTypeFeat,
		},
		{
			name:         "custom fix keyword",
			message:      "ticket fix for login issue",
			expectedType: changes.CommitTypeFix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commit := analysis.CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123"),
				Subject: tt.message,
			}

			result := analyzer.Classify(commit)
			if result == nil {
				t.Errorf("expected match but got nil for: %q", tt.message)
				return
			}

			if result.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s for: %q",
					tt.expectedType, result.Type, tt.message)
			}
		})
	}
}
