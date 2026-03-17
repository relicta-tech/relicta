package heuristics

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

func TestKeywordDetector_Detect(t *testing.T) {
	detector := NewKeywordDetector()

	tests := []struct {
		name          string
		message       string
		expectedType  changes.CommitType
		expectedBreak bool
		minConfidence float64
		shouldMatch   bool
	}{
		// Fix patterns
		{
			name:          "fix at start",
			message:       "fix bug in authentication",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "fix in middle",
			message:       "authentication fix for token expiry",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "bugfix",
			message:       "bugfix: resolve login issue",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "hotfix",
			message:       "hotfix for production crash",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "resolve issue",
			message:       "resolve issue with database connection",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "crash fix",
			message:       "prevent crash on null input",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.70,
			shouldMatch:   true,
		},

		// Feature patterns
		{
			name:          "add at start",
			message:       "add user authentication",
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "feature keyword",
			message:       "feature: dark mode support",
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "implement",
			message:       "implement caching layer",
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "new keyword",
			message:       "new command for exports",
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "introduce",
			message:       "introduce error handling middleware",
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "create",
			message:       "create API endpoints for users",
			expectedType:  changes.CommitTypeFeat,
			minConfidence: 0.65,
			shouldMatch:   true,
		},

		// Refactor patterns
		{
			name:          "refactor at start",
			message:       "refactor authentication module",
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "simplify",
			message:       "simplify error handling logic",
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "cleanup",
			message:       "cleanup unused imports",
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "rename",
			message:       "rename User to Account",
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.65,
			shouldMatch:   true,
		},
		{
			name:          "extract",
			message:       "extract common logic to helper",
			expectedType:  changes.CommitTypeRefactor,
			minConfidence: 0.65,
			shouldMatch:   true,
		},

		// Docs patterns
		{
			name:          "docs at start",
			message:       "docs: update README",
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "readme",
			message:       "update readme with examples",
			expectedType:  changes.CommitTypeDocs,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "typo",
			message:       "fix typo in documentation",
			expectedType:  changes.CommitTypeFix, // "fix" has higher confidence than "typo"
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "jsdoc",
			message:       "add jsdoc comments to functions",
			expectedType:  changes.CommitTypeDocs, // "jsdoc" matches docs at 0.85, same as "add" for feat
			minConfidence: 0.70,
			shouldMatch:   true,
		},

		// Chore patterns
		{
			name:          "chore at start",
			message:       "chore: update dependencies",
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "bump",
			message:       "bump version to 1.2.3",
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "deps",
			message:       "deps: upgrade lodash",
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "dependency update",
			message:       "update dependency versions",
			expectedType:  changes.CommitTypeChore,
			minConfidence: 0.75,
			shouldMatch:   true,
		},

		// Build patterns
		{
			name:          "build at start",
			message:       "build: update webpack config",
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "makefile",
			message:       "update Makefile targets",
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "webpack",
			message:       "configure webpack for production",
			expectedType:  changes.CommitTypeBuild,
			minConfidence: 0.75,
			shouldMatch:   true,
		},

		// CI patterns
		{
			name:          "ci at start",
			message:       "ci: add github actions workflow",
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "github action",
			message:       "update github action for tests",
			expectedType:  changes.CommitTypeCI,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "workflow",
			message:       "fix workflow for releases",
			expectedType:  changes.CommitTypeFix, // "fix" has higher confidence than "workflow"
			minConfidence: 0.75,
			shouldMatch:   true,
		},

		// Test patterns
		{
			name:          "test at start",
			message:       "test: add unit tests for auth",
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "tests anywhere",
			message:       "add tests for user service",
			expectedType:  changes.CommitTypeFeat, // "add" has higher confidence than "tests"
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "coverage",
			message:       "improve test coverage",
			expectedType:  changes.CommitTypeTest,
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "e2e tests",
			message:       "add e2e tests for checkout",
			expectedType:  changes.CommitTypeFeat, // "add" has higher confidence than "e2e"
			minConfidence: 0.70,
			shouldMatch:   true,
		},

		// Perf patterns
		{
			name:          "perf at start",
			message:       "perf: optimize database queries",
			expectedType:  changes.CommitTypePerf,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "performance",
			message:       "improve performance of search",
			expectedType:  changes.CommitTypePerf,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:          "faster",
			message:       "make startup faster",
			expectedType:  changes.CommitTypePerf,
			minConfidence: 0.65,
			shouldMatch:   true,
		},

		// Revert patterns
		{
			name:          "revert at start",
			message:       "revert: undo last change",
			expectedType:  changes.CommitTypeRevert,
			minConfidence: 0.90,
			shouldMatch:   true,
		},
		{
			name:          "rollback",
			message:       "rollback database migration",
			expectedType:  changes.CommitTypeRevert,
			minConfidence: 0.80,
			shouldMatch:   true,
		},

		// Style patterns
		{
			name:          "style at start",
			message:       "style: format code",
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.85,
			shouldMatch:   true,
		},
		{
			name:          "lint",
			message:       "lint: fix eslint warnings",
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "formatting",
			message:       "apply formatting to all files",
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.70,
			shouldMatch:   true,
		},
		{
			name:          "prettier",
			message:       "run prettier on codebase",
			expectedType:  changes.CommitTypeStyle,
			minConfidence: 0.75,
			shouldMatch:   true,
		},

		// Breaking change detection
		// Note: Some messages may not match a type but still be detected as breaking
		{
			name:          "breaking with refactor",
			message:       "refactor auth with breaking change",
			expectedType:  changes.CommitTypeRefactor,
			expectedBreak: true,
			minConfidence: 0.80,
			shouldMatch:   true,
		},
		{
			name:          "breaking with remove",
			message:       "remove deprecated UserV1 type",
			expectedType:  changes.CommitTypeRefactor, // "remove" matches refactor patterns
			expectedBreak: true,                       // "remove" is a breaking indicator
			minConfidence: 0.50,
			shouldMatch:   true,
		},

		// Edge cases
		{
			name:        "empty message",
			message:     "",
			shouldMatch: false,
		},
		{
			name:        "whitespace only",
			message:     "   ",
			shouldMatch: false,
		},
		{
			name:          "mixed case",
			message:       "FIX authentication BUG",
			expectedType:  changes.CommitTypeFix,
			minConfidence: 0.75,
			shouldMatch:   true,
		},
		{
			name:        "partial word should not match (fixed boundary)",
			message:     "prefix something",
			shouldMatch: false, // "fix" should not match in "prefix"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.message)

			if tt.shouldMatch {
				if result == nil {
					t.Errorf("expected match but got nil for message: %q", tt.message)
					return
				}

				if result.Type != tt.expectedType {
					t.Errorf("expected type %s, got %s for message: %q",
						tt.expectedType, result.Type, tt.message)
				}

				if result.Confidence < tt.minConfidence {
					t.Errorf("expected confidence >= %.2f, got %.2f for message: %q",
						tt.minConfidence, result.Confidence, tt.message)
				}

				if tt.expectedBreak && !result.IsBreaking {
					t.Errorf("expected breaking change for message: %q", tt.message)
				}
			} else {
				if result != nil {
					t.Errorf("expected no match but got %s (%.2f) for message: %q",
						result.Type, result.Confidence, tt.message)
				}
			}
		})
	}
}

func TestContainsWord(t *testing.T) {
	tests := []struct {
		message  string
		word     string
		expected bool
	}{
		// Positive cases
		{"fix bug", "fix", true},
		{"this is a fix", "fix", true},
		{"fix", "fix", true},
		{"fix: something", "fix", true},
		{"the fix worked", "fix", true},

		// Negative cases - word is part of larger word
		{"prefix something", "fix", false},
		{"fixup commit", "fix", false},

		// Positive - word boundaries respected
		{"suffix fix attached", "fix", true}, // "fix" is a separate word here

		// Boundary cases
		{"", "fix", false},
		{"fix", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.message+"_"+tt.word, func(t *testing.T) {
			result := containsWord(tt.message, tt.word)
			if result != tt.expected {
				t.Errorf("containsWord(%q, %q) = %v, want %v",
					tt.message, tt.word, result, tt.expected)
			}
		})
	}
}

func TestKeywordDetector_BreakingDetection(t *testing.T) {
	detector := NewKeywordDetector()

	// Messages that should be detected as breaking (when they also match a type)
	breakingMessages := []struct {
		message string
		hasType bool // whether it should also match a commit type
	}{
		{"refactor with breaking change to API", true},     // "refactor" + "breaking"
		{"remove deprecated UserV1 type", true},            // "remove" is breaking
		{"fix issue and drop support for v1", true},        // "drop support" is breaking
		{"add feature with incompatible changes", true},    // "incompatible" is breaking
		{"implement new auth with breaking changes", true}, // "implement" + "breaking"
	}

	for _, tc := range breakingMessages {
		t.Run(tc.message, func(t *testing.T) {
			result := detector.Detect(tc.message)
			if tc.hasType {
				if result == nil {
					t.Errorf("expected match for: %q", tc.message)
					return
				}
				if !result.IsBreaking {
					t.Errorf("expected breaking=true for message: %q", tc.message)
				}
			}
		})
	}

	nonBreakingMessages := []string{
		"add new feature",
		"fix bug in auth",
		"refactor code",
		"update documentation",
	}

	for _, msg := range nonBreakingMessages {
		t.Run(msg, func(t *testing.T) {
			result := detector.Detect(msg)
			if result != nil && result.IsBreaking {
				t.Errorf("expected breaking=false for message: %q", msg)
			}
		})
	}
}
