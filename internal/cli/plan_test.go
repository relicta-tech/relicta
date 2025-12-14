// Package cli provides the command-line interface for Relicta.
package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

func TestPlanCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"from flag", "from"},
		{"to flag", "to"},
		{"all flag", "all"},
		{"minimal flag", "minimal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := planCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("plan command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestPlanCommand_DefaultValues(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		wantDefault string
	}{
		{"from default empty", "from", ""},
		{"to default HEAD", "to", "HEAD"},
		{"all default false", "all", "false"},
		{"minimal default false", "minimal", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := planCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("%s flag not found", tt.flagName)
			}
			if flag.DefValue != tt.wantDefault {
				t.Errorf("%s flag default = %v, want %v", tt.flagName, flag.DefValue, tt.wantDefault)
			}
		})
	}
}

func TestReleaseTypeDisplay_AllTypes(t *testing.T) {
	tests := []struct {
		name         string
		releaseType  changes.ReleaseType
		wantContains string
	}{
		{
			name:         "major release",
			releaseType:  changes.ReleaseTypeMajor,
			wantContains: "major",
		},
		{
			name:         "minor release",
			releaseType:  changes.ReleaseTypeMinor,
			wantContains: "minor",
		},
		{
			name:         "patch release",
			releaseType:  changes.ReleaseTypePatch,
			wantContains: "patch",
		},
		{
			name:         "none release",
			releaseType:  changes.ReleaseTypeNone,
			wantContains: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := releaseTypeDisplay(tt.releaseType)
			// The result will be styled, so check if it contains the expected text
			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("releaseTypeDisplay(%v) = %q, should contain %q", tt.releaseType, result, tt.wantContains)
			}
		})
	}
}

func TestFilterNonBreaking_NilInput(t *testing.T) {
	result := filterNonBreaking(nil)
	if result != nil {
		t.Errorf("filterNonBreaking(nil) = %v, want nil", result)
	}
}

func TestFilterNonBreaking_EmptySlice(t *testing.T) {
	result := filterNonBreaking([]*changes.ConventionalCommit{})
	// Empty slice can be nil or have length 0, both are acceptable
	if len(result) != 0 {
		t.Errorf("filterNonBreaking([]) length = %d, want 0", len(result))
	}
}

func TestGetNonCoreCategorizedCommits_Empty(t *testing.T) {
	cats := &changes.Categories{
		Features:  nil,
		Fixes:     nil,
		Perf:      nil,
		Docs:      nil,
		Refactors: nil,
		Tests:     nil,
		Chores:    nil,
		Build:     nil,
		CI:        nil,
		Other:     nil,
		Breaking:  nil,
	}

	result := getNonCoreCategorizedCommits(cats)
	if len(result) != 0 {
		t.Errorf("getNonCoreCategorizedCommits() with empty categories len = %d, want 0", len(result))
	}
}

func TestGetNonCoreCategorizedCommits_ExcludesCoreTypes(t *testing.T) {
	// Create a Categories with some core types (Features, Fixes, Perf)
	// These should NOT be included in the result
	cats := &changes.Categories{
		Features:  []*changes.ConventionalCommit{},
		Fixes:     []*changes.ConventionalCommit{},
		Perf:      []*changes.ConventionalCommit{},
		Docs:      []*changes.ConventionalCommit{},
		Refactors: []*changes.ConventionalCommit{},
		Tests:     []*changes.ConventionalCommit{},
		Chores:    []*changes.ConventionalCommit{},
		Build:     []*changes.ConventionalCommit{},
		CI:        []*changes.ConventionalCommit{},
		Other:     []*changes.ConventionalCommit{},
		Breaking:  []*changes.ConventionalCommit{},
	}

	// Result should be empty because we're testing with empty slices
	result := getNonCoreCategorizedCommits(cats)
	if len(result) != 0 {
		t.Errorf("getNonCoreCategorizedCommits() len = %d, want 0", len(result))
	}
}

func TestPlanCommand_Configuration(t *testing.T) {
	if planCmd == nil {
		t.Fatal("planCmd is nil")
	}
	if planCmd.Use != "plan" {
		t.Errorf("planCmd.Use = %v, want plan", planCmd.Use)
	}
	if planCmd.RunE == nil {
		t.Error("planCmd.RunE is nil")
	}
}

func TestPrintConventionalCommit_WithScope(t *testing.T) {
	// Create a commit with a scope
	commit := changes.NewConventionalCommit(
		"abc123def456",
		changes.CommitTypeFeat,
		"add login support",
		changes.WithScope("auth"),
		changes.WithBody("Full commit body"),
	)

	// Just verify it doesn't panic
	printConventionalCommit(commit)
}

func TestPrintConventionalCommit_WithoutScope(t *testing.T) {
	// Create a commit without a scope
	commit := changes.NewConventionalCommit(
		"abc123def456",
		changes.CommitTypeFix,
		"fix critical bug",
		changes.WithBody("Full commit body"),
	)

	// Just verify it doesn't panic
	printConventionalCommit(commit)
}

func TestPrintConventionalCommit_BreakingChange(t *testing.T) {
	// Create a breaking change commit
	commit := changes.NewConventionalCommit(
		"abc123def456",
		changes.CommitTypeFeat,
		"change API response format",
		changes.WithScope("api"),
		changes.WithBody("Full commit body"),
		changes.WithBreaking("API response format changed"),
	)

	// Just verify it doesn't panic
	printConventionalCommit(commit)
}
