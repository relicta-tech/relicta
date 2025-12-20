package heuristics

import (
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// KeywordDetector detects commit types from message keywords.
type KeywordDetector struct {
	patterns map[changes.CommitType][]keywordPattern
	breaking []keywordPattern
}

// keywordPattern represents a keyword matching pattern.
type keywordPattern struct {
	// pattern is the keyword or phrase to match.
	pattern string

	// regex is a compiled regex for complex patterns.
	regex *regexp.Regexp

	// confidence is the confidence when this pattern matches.
	confidence float64

	// requireWordBoundary requires the pattern to be a complete word.
	requireWordBoundary bool

	// position specifies where the pattern should appear (start, anywhere).
	position string // "start", "anywhere"
}

// NewKeywordDetector creates a new keyword detector.
func NewKeywordDetector() *KeywordDetector {
	return &KeywordDetector{
		patterns: initKeywordPatterns(),
		breaking: initBreakingPatterns(),
	}
}

// Detect attempts to classify a commit message by keywords.
func (d *KeywordDetector) Detect(message string) *DetectionResult {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return nil
	}

	// Check for breaking change indicators first
	isBreaking := d.detectBreaking(message)

	// Check each commit type
	var bestMatch *DetectionResult
	var bestConfidence float64

	for commitType, patterns := range d.patterns {
		for _, p := range patterns {
			if confidence := d.matchPattern(message, p); confidence > bestConfidence {
				bestConfidence = confidence
				bestMatch = &DetectionResult{
					Type:       commitType,
					Confidence: confidence,
					Reasoning:  "matched keyword pattern: " + p.pattern,
					IsBreaking: isBreaking,
				}
			}
		}
	}

	return bestMatch
}

// detectBreaking checks if the message indicates a breaking change.
func (d *KeywordDetector) detectBreaking(message string) bool {
	for _, p := range d.breaking {
		if d.matchPattern(message, p) > 0 {
			return true
		}
	}
	return false
}

// matchPattern checks if a message matches a pattern.
func (d *KeywordDetector) matchPattern(message string, p keywordPattern) float64 {
	// Use regex if available
	if p.regex != nil {
		if p.regex.MatchString(message) {
			return p.confidence
		}
		return 0
	}

	// Simple string matching
	pattern := strings.ToLower(p.pattern)

	switch p.position {
	case "start":
		if strings.HasPrefix(message, pattern) {
			return p.confidence
		}
	default: // "anywhere"
		if p.requireWordBoundary {
			if containsWord(message, pattern) {
				return p.confidence
			}
		} else if strings.Contains(message, pattern) {
			return p.confidence
		}
	}

	return 0
}

// containsWord checks if message contains pattern as a complete word.
func containsWord(message, word string) bool {
	// Check for word at start
	if strings.HasPrefix(message, word) {
		if len(message) == len(word) {
			return true
		}
		next := message[len(word)]
		if !isAlphanumeric(next) {
			return true
		}
	}

	// Check for word in middle or end
	index := strings.Index(message, word)
	for index > 0 {
		prev := message[index-1]
		endIndex := index + len(word)

		if !isAlphanumeric(prev) {
			if endIndex >= len(message) {
				return true
			}
			next := message[endIndex]
			if !isAlphanumeric(next) {
				return true
			}
		}

		// Keep searching
		remaining := message[index+1:]
		nextIndex := strings.Index(remaining, word)
		if nextIndex == -1 {
			break
		}
		index = index + 1 + nextIndex
	}

	return false
}

func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// initKeywordPatterns initializes the keyword patterns for each commit type.
func initKeywordPatterns() map[changes.CommitType][]keywordPattern {
	return map[changes.CommitType][]keywordPattern{
		changes.CommitTypeFix: {
			// High confidence - explicit fix keywords at start
			{pattern: "fix", position: "start", confidence: 0.90, requireWordBoundary: true},
			{pattern: "bugfix", position: "start", confidence: 0.92},
			{pattern: "hotfix", position: "start", confidence: 0.92},
			{pattern: "patch", position: "start", confidence: 0.85, requireWordBoundary: true},

			// Medium-high confidence - fix-related words
			{pattern: "fix", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "bug", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "resolve", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "fixed", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "fixes", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "fixing", position: "anywhere", confidence: 0.80, requireWordBoundary: true},

			// Medium confidence - problem-related words
			{pattern: "issue", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
			{pattern: "error", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "crash", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "broken", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "repair", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "correct", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
		},

		changes.CommitTypeFeat: {
			// High confidence - explicit feature keywords at start
			{pattern: "add", position: "start", confidence: 0.85, requireWordBoundary: true},
			{pattern: "feature", position: "start", confidence: 0.92},
			{pattern: "implement", position: "start", confidence: 0.88},
			{pattern: "new", position: "start", confidence: 0.80, requireWordBoundary: true},

			// Medium-high confidence - feature-related words
			{pattern: "add", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "added", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "adding", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "adds", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "implement", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "implemented", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "introduce", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "support", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "create", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "enable", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
		},

		changes.CommitTypeRefactor: {
			// High confidence
			{pattern: "refactor", position: "start", confidence: 0.92},
			{pattern: "restructure", position: "start", confidence: 0.90},
			{pattern: "reorganize", position: "start", confidence: 0.88},
			{pattern: "remove", position: "start", confidence: 0.75, requireWordBoundary: true},

			// Medium-high confidence
			{pattern: "refactor", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "refactored", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "refactoring", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "simplify", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "clean", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
			{pattern: "cleanup", position: "anywhere", confidence: 0.75},
			{pattern: "clean up", position: "anywhere", confidence: 0.75},
			{pattern: "improve", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
			{pattern: "optimize", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
			{pattern: "rename", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "move", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
			{pattern: "extract", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "remove", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
			{pattern: "delete", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
		},

		changes.CommitTypeDocs: {
			// High confidence
			{pattern: "doc", position: "start", confidence: 0.90, requireWordBoundary: true},
			{pattern: "docs", position: "start", confidence: 0.92},
			{pattern: "readme", position: "start", confidence: 0.90},
			{pattern: "documentation", position: "start", confidence: 0.92},

			// Medium-high confidence
			{pattern: "doc", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "docs", position: "anywhere", confidence: 0.82, requireWordBoundary: true},
			{pattern: "readme", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "comment", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "comments", position: "anywhere", confidence: 0.75, requireWordBoundary: true},

			// Medium confidence
			{pattern: "typo", position: "anywhere", confidence: 0.86, requireWordBoundary: true},
			{pattern: "spelling", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "grammar", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "jsdoc", position: "anywhere", confidence: 0.86},
			{pattern: "godoc", position: "anywhere", confidence: 0.86},
			{pattern: "docstring", position: "anywhere", confidence: 0.86},
		},

		changes.CommitTypeChore: {
			// High confidence
			{pattern: "chore", position: "start", confidence: 0.92},
			{pattern: "deps", position: "start", confidence: 0.88},
			{pattern: "bump", position: "start", confidence: 0.85},

			// Medium-high confidence
			{pattern: "chore", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "dependency", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "dependencies", position: "anywhere", confidence: 0.80},
			{pattern: "upgrade", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "update", position: "start", confidence: 0.60, requireWordBoundary: true},
			{pattern: "bump", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "config", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
		},

		changes.CommitTypeBuild: {
			{pattern: "build", position: "start", confidence: 0.88, requireWordBoundary: true},
			{pattern: "build", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "make", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
			{pattern: "makefile", position: "anywhere", confidence: 0.80},
			{pattern: "webpack", position: "anywhere", confidence: 0.80},
			{pattern: "vite", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "rollup", position: "anywhere", confidence: 0.80},
			{pattern: "esbuild", position: "anywhere", confidence: 0.80},
		},

		changes.CommitTypeCI: {
			// High confidence
			{pattern: "ci", position: "start", confidence: 0.90, requireWordBoundary: true},
			{pattern: "ci:", position: "start", confidence: 0.92},

			// Medium-high confidence
			{pattern: "ci", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "github action", position: "anywhere", confidence: 0.85},
			{pattern: "github-action", position: "anywhere", confidence: 0.85},
			{pattern: "workflow", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "pipeline", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "travis", position: "anywhere", confidence: 0.80},
			{pattern: "circleci", position: "anywhere", confidence: 0.80},
			{pattern: "gitlab-ci", position: "anywhere", confidence: 0.85},
			{pattern: "jenkins", position: "anywhere", confidence: 0.80},
		},

		changes.CommitTypeTest: {
			// High confidence
			{pattern: "test", position: "start", confidence: 0.90, requireWordBoundary: true},
			{pattern: "tests", position: "start", confidence: 0.90},
			{pattern: "spec", position: "start", confidence: 0.85, requireWordBoundary: true},

			// Medium-high confidence
			{pattern: "test", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "tests", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "testing", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "spec", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "coverage", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "e2e", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "unit test", position: "anywhere", confidence: 0.85},
			{pattern: "integration test", position: "anywhere", confidence: 0.85},
		},

		changes.CommitTypePerf: {
			// High confidence
			{pattern: "perf", position: "start", confidence: 0.90, requireWordBoundary: true},
			{pattern: "performance", position: "start", confidence: 0.90},

			// Medium-high confidence
			{pattern: "perf", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "performance", position: "anywhere", confidence: 0.80, requireWordBoundary: true},
			{pattern: "speed", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
			{pattern: "faster", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "slow", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
			{pattern: "optimize", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "cache", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
			{pattern: "memory", position: "anywhere", confidence: 0.55, requireWordBoundary: true},
		},

		changes.CommitTypeRevert: {
			{pattern: "revert", position: "start", confidence: 0.95},
			{pattern: "revert", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "rollback", position: "anywhere", confidence: 0.85, requireWordBoundary: true},
			{pattern: "undo", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
		},

		changes.CommitTypeStyle: {
			{pattern: "style", position: "start", confidence: 0.88, requireWordBoundary: true},
			{pattern: "format", position: "start", confidence: 0.80, requireWordBoundary: true},
			{pattern: "lint", position: "start", confidence: 0.85, requireWordBoundary: true},

			{pattern: "style", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "format", position: "anywhere", confidence: 0.70, requireWordBoundary: true},
			{pattern: "formatting", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "lint", position: "anywhere", confidence: 0.75, requireWordBoundary: true},
			{pattern: "prettier", position: "anywhere", confidence: 0.80},
			{pattern: "eslint", position: "anywhere", confidence: 0.80},
			{pattern: "gofmt", position: "anywhere", confidence: 0.85},
			{pattern: "whitespace", position: "anywhere", confidence: 0.75},
			{pattern: "indentation", position: "anywhere", confidence: 0.75},
		},
	}
}

// initBreakingPatterns initializes patterns that indicate breaking changes.
func initBreakingPatterns() []keywordPattern {
	return []keywordPattern{
		// Explicit breaking indicators
		{pattern: "breaking", position: "anywhere", confidence: 0.95, requireWordBoundary: true},
		{pattern: "breaking change", position: "anywhere", confidence: 0.98},
		{pattern: "breaking-change", position: "anywhere", confidence: 0.98},
		{pattern: "break:", position: "anywhere", confidence: 0.95},
		{pattern: "!:", position: "anywhere", confidence: 0.95}, // conventional commits breaking

		// Actions that often indicate breaking changes
		{pattern: "remove", position: "start", confidence: 0.70, requireWordBoundary: true},
		{pattern: "removed", position: "anywhere", confidence: 0.65, requireWordBoundary: true},
		{pattern: "delete", position: "start", confidence: 0.65, requireWordBoundary: true},
		{pattern: "deprecated", position: "anywhere", confidence: 0.60, requireWordBoundary: true},
		{pattern: "drop support", position: "anywhere", confidence: 0.85},
		{pattern: "incompatible", position: "anywhere", confidence: 0.80, requireWordBoundary: true},

		// Regex patterns for common breaking change formats
		{regex: regexp.MustCompile(`^[a-z]+\([^)]*\)!:`), confidence: 0.98}, // feat(api)!: ...
	}
}
