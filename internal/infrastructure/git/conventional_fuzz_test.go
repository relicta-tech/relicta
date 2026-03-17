package git

import (
	"testing"
)

// FuzzParseConventionalCommit tests the conventional commit parser with fuzzing.
// Run with: go test -fuzz=FuzzParseConventionalCommit -fuzztime=30s
func FuzzParseConventionalCommit(f *testing.F) {
	// Add seed corpus from various valid and invalid commit messages
	seeds := []string{
		"feat: add new feature",
		"fix: resolve bug",
		"feat(auth): add OAuth2 support",
		"fix(api): handle null pointer",
		"feat!: breaking change",
		"feat(ui)!: redesign dashboard",
		"docs: update README",
		"chore(deps): bump dependencies",
		"refactor: simplify logic",
		"perf: optimize query",
		"test: add unit tests",
		"ci: update workflow",
		"build: change compiler settings",
		"revert: revert previous commit",
		"style: fix formatting",
		// With body and footer
		"feat: feature\n\nBody text here",
		"fix: fix\n\nDescription\n\nFixes #123",
		"feat: feature\n\nBREAKING CHANGE: changes API",
		"fix: fix\n\nBREAKING-CHANGE: different format",
		// Edge cases
		"",
		"feat:",
		"feat: ",
		"feat():",
		"feat(scope):",
		"FEAT: uppercase",
		"fix (space): with space",
		"feat\n: newline before colon",
		"no colon at all",
		"feat:no space after colon",
		// Unicode
		"feat: 新機能を追加",
		"fix: исправление ошибки",
		"feat(🚀): emoji scope",
		// Long inputs
		"feat: " + string(make([]byte, 1000)),
		// Special characters
		"feat: test <script>alert('xss')</script>",
		"feat: test $(whoami)",
		"feat: test `ls -la`",
		"feat: test; rm -rf /",
		"feat: test && cat /etc/passwd",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, message string) {
		// The primary goal of this fuzz test is to ensure the parser doesn't panic
		// on any input. We don't strictly validate parse results since the parser
		// may return partial results for edge cases (by design).
		result, _ := ParseConventionalCommit(message)

		// If we got a result, verify it's internally consistent
		if result != nil {
			// Access all fields to ensure no nil pointer panics
			_ = result.Type
			_ = result.Scope
			_ = result.Description
			_ = result.Breaking
			_ = result.Body
			_ = result.Footer
		}
	})
}

// FuzzDetectReleaseType tests the release type detection with fuzzing.
func FuzzDetectReleaseType(f *testing.F) {
	// Seed with various commit type combinations
	f.Add("feat", "fix", "docs", false)
	f.Add("feat", "", "", true)
	f.Add("fix", "fix", "fix", false)
	f.Add("", "", "", false)
	f.Add("chore", "docs", "test", false)
	f.Add("feat", "feat", "feat", true)

	f.Fuzz(func(t *testing.T, type1, type2, type3 string, hasBreaking bool) {
		commits := []ConventionalCommit{}

		if type1 != "" {
			commits = append(commits, ConventionalCommit{
				Type:        CommitType(type1),
				Breaking:    hasBreaking,
				Description: "test",
			})
		}
		if type2 != "" {
			commits = append(commits, ConventionalCommit{
				Type:        CommitType(type2),
				Description: "test",
			})
		}
		if type3 != "" {
			commits = append(commits, ConventionalCommit{
				Type:        CommitType(type3),
				Description: "test",
			})
		}

		// Should not panic
		result := DetectReleaseType(commits)

		// Result should be a valid release type
		switch result {
		case ReleaseTypeMajor, ReleaseTypeMinor, ReleaseTypePatch, ReleaseTypeNone:
			// Valid
		default:
			t.Errorf("unexpected release type: %v", result)
		}

		// If any commit is breaking, result should be Major
		if hasBreaking && len(commits) > 0 && commits[0].Breaking {
			if result != ReleaseTypeMajor {
				t.Errorf("expected Major for breaking change, got %v", result)
			}
		}
	})
}
