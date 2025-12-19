package heuristics

import (
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// PatternDetector detects commit types from patterns and provides skip detection.
type PatternDetector struct {
	skipPatterns []*regexp.Regexp
	skipReasons  []string
}

// NewPatternDetector creates a new pattern detector.
func NewPatternDetector() *PatternDetector {
	patterns, reasons := initSkipPatterns()
	return &PatternDetector{
		skipPatterns: patterns,
		skipReasons:  reasons,
	}
}

// ShouldSkip checks if a commit should be skipped.
func (d *PatternDetector) ShouldSkip(commit analysis.CommitInfo) (bool, string) {
	// Always skip merge commits
	if commit.IsMerge || commit.ParentCount > 1 {
		return true, "merge commit"
	}

	// Check message patterns
	subject := strings.TrimSpace(commit.Subject)
	for i, pattern := range d.skipPatterns {
		if pattern.MatchString(subject) {
			return true, d.skipReasons[i]
		}
	}

	return false, ""
}

// ClassifyByDiffSize attempts to classify based on diff statistics.
func (d *PatternDetector) ClassifyByDiffSize(stats analysis.DiffStats) *DetectionResult {
	additions := stats.Additions
	deletions := stats.Deletions
	total := additions + deletions

	// Skip very small changes (likely typos or minor fixes)
	if total == 0 {
		return nil
	}

	// Very small changes (1-5 lines) - likely a fix or typo
	if total <= 5 {
		return &DetectionResult{
			Type:       changes.CommitTypeFix,
			Confidence: 0.50,
			Reasoning:  "very small change suggests minor fix",
		}
	}

	// Small changes (6-15 lines) - likely a fix
	if total <= 15 {
		return &DetectionResult{
			Type:       changes.CommitTypeFix,
			Confidence: 0.45,
			Reasoning:  "small change suggests fix",
		}
	}

	// Large additions with few deletions - likely a feature
	if additions > 50 && deletions < 10 {
		return &DetectionResult{
			Type:       changes.CommitTypeFeat,
			Confidence: 0.55,
			Reasoning:  "large additions with minimal deletions suggests new feature",
		}
	}

	// Very large additions - definitely a feature
	if additions > 200 && deletions < additions/4 {
		return &DetectionResult{
			Type:       changes.CommitTypeFeat,
			Confidence: 0.65,
			Reasoning:  "substantial additions suggests new feature or major enhancement",
		}
	}

	// Balanced changes (similar additions/deletions) - likely refactor
	if additions > 20 && deletions > 20 {
		ratio := float64(additions) / float64(deletions)
		if ratio > 0.6 && ratio < 1.7 {
			return &DetectionResult{
				Type:       changes.CommitTypeRefactor,
				Confidence: 0.50,
				Reasoning:  "balanced additions and deletions suggests refactoring",
			}
		}
	}

	// More deletions than additions - could be cleanup/refactor
	if deletions > additions*2 && deletions > 20 {
		return &DetectionResult{
			Type:       changes.CommitTypeRefactor,
			Confidence: 0.45,
			Reasoning:  "significant deletions suggests cleanup or refactoring",
		}
	}

	// Single file change with moderate size
	if stats.FilesChanged == 1 && total >= 10 && total <= 50 {
		return &DetectionResult{
			Type:       changes.CommitTypeFix,
			Confidence: 0.40,
			Reasoning:  "single file moderate change",
		}
	}

	// Unable to determine from diff size alone
	return nil
}

// initSkipPatterns initializes patterns for commits that should be skipped.
func initSkipPatterns() ([]*regexp.Regexp, []string) {
	patterns := []struct {
		pattern string
		reason  string
	}{
		// Merge commits - more specific patterns first
		{`^Merge pull request`, "merge pull request"},
		{`^Merge branch`, "merge branch"},
		{`^Merge remote-tracking branch`, "merge remote branch"},
		{`^Merge `, "merge commit"},
		{`^Merged `, "merge commit"},

		// Revert commits (already parsed by conventional commit parser)
		{`^Revert "`, "revert commit"},

		// Initial commit
		{`^Initial commit$`, "initial commit"},
		{`^init$`, "initial commit"},
		{`^first commit$`, "initial commit"},

		// WIP commits (should not be released)
		{`^wip$`, "work in progress"},
		{`^WIP$`, "work in progress"},
		{`^wip:`, "work in progress"},
		{`^WIP:`, "work in progress"},
		{`^\[WIP\]`, "work in progress"},
		{`^fixup!`, "fixup commit"},
		{`^squash!`, "squash commit"},
		{`^amend!`, "amend commit"},

		// Auto-generated commits
		{`^Auto-merge`, "auto-generated"},
		{`^Automatic merge`, "auto-generated"},
		{`^Apply suggestions from code review`, "code review suggestion"},

		// Version bumps (already handled separately)
		{`^v?\d+\.\d+\.\d+$`, "version tag"},
		{`^Release v?\d+`, "release commit"},
		{`^Bump version`, "version bump"},
		{`^\[release\]`, "release commit"},
		{`^chore\(release\)`, "release commit"},

		// Bot commits
		{`^\[bot\]`, "bot commit"},
		{`^dependabot\[bot\]`, "dependabot"},
		{`^renovate\[bot\]`, "renovate"},
		{`^github-actions\[bot\]`, "github actions"},

		// Non-meaningful messages
		{`^update$`, "non-descriptive"},
		{`^updates$`, "non-descriptive"},
		{`^fix$`, "non-descriptive"}, // Note: "fix" at start is kept for keyword detection, this is exact match
		{`^stuff$`, "non-descriptive"},
		{`^asdf+$`, "non-descriptive"},
		{`^temp$`, "non-descriptive"},
		{`^tmp$`, "non-descriptive"},
		{`^test$`, "non-descriptive"},
		{`^\.+$`, "non-descriptive"},
		{`^-+$`, "non-descriptive"},
		{`^_+$`, "non-descriptive"},
	}

	compiled := make([]*regexp.Regexp, len(patterns))
	reasons := make([]string, len(patterns))

	for i, p := range patterns {
		compiled[i] = regexp.MustCompile(`(?i)` + p.pattern) // Case insensitive
		reasons[i] = p.reason
	}

	return compiled, reasons
}

// CommitType represents the final change type for compatibility.
// Note: This is kept for potential future use but currently uses changes.CommitType.
type ChangeType = changes.CommitType
