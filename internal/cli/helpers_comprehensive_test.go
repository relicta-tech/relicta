// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
)

// Test validateEditor edge cases
func TestValidateEditor_SafetyChecks(t *testing.T) {
	// Test that dangerous patterns are rejected
	dangerousEditors := []string{
		"rm", // rm is not in whitelist
		"cat",
		"unknown-editor-xyz-123",
		"", // empty string
	}

	for _, editor := range dangerousEditors {
		t.Run(editor, func(t *testing.T) {
			_, err := validateEditor(editor)
			if err == nil {
				t.Errorf("validateEditor(%s) should not be allowed", editor)
			}
		})
	}
}

// Test filterNonBreaking with various inputs
func TestFilterNonBreaking_WithBreakingCommits(t *testing.T) {
	commits := []*changes.ConventionalCommit{
		changes.NewConventionalCommit(
			"abc123",
			changes.CommitTypeFeat,
			"regular feature",
		),
		changes.NewConventionalCommit(
			"def456",
			changes.CommitTypeFeat,
			"breaking feature",
			changes.WithBreaking("API changed"),
		),
		changes.NewConventionalCommit(
			"ghi789",
			changes.CommitTypeFeat,
			"another feature",
		),
	}

	result := filterNonBreaking(commits)

	// Should filter out the breaking commit
	if len(result) != 2 {
		t.Errorf("filterNonBreaking() returned %d commits, want 2", len(result))
	}

	// Verify none are breaking
	for _, commit := range result {
		if commit.IsBreaking() {
			t.Error("filterNonBreaking() should not include breaking commits")
		}
	}
}

// Test getNonCoreCategorizedCommits
func TestGetNonCoreCategorizedCommits_WithNonCoreCommits(t *testing.T) {
	cats := &changes.Categories{
		Features: []*changes.ConventionalCommit{},
		Fixes:    []*changes.ConventionalCommit{},
		Perf:     []*changes.ConventionalCommit{},
		Docs: []*changes.ConventionalCommit{
			changes.NewConventionalCommit(
				"doc1",
				changes.CommitTypeDocs,
				"update docs",
			),
		},
		Refactors: []*changes.ConventionalCommit{
			changes.NewConventionalCommit(
				"ref1",
				changes.CommitTypeRefactor,
				"refactor code",
			),
		},
		Tests: []*changes.ConventionalCommit{
			changes.NewConventionalCommit(
				"test1",
				changes.CommitTypeTest,
				"add tests",
			),
		},
		Chores:   []*changes.ConventionalCommit{},
		Build:    []*changes.ConventionalCommit{},
		CI:       []*changes.ConventionalCommit{},
		Other:    []*changes.ConventionalCommit{},
		Breaking: []*changes.ConventionalCommit{},
	}

	result := getNonCoreCategorizedCommits(cats)

	// Should have 3 non-core commits (docs, refactors, tests)
	if len(result) != 3 {
		t.Errorf("getNonCoreCategorizedCommits() returned %d commits, want 3", len(result))
	}
}
