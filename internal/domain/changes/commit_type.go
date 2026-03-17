// Package changes provides domain types for analyzing commit changes.
package changes

import "strings"

// CommitType represents the type of a conventional commit.
type CommitType string

// Standard conventional commit types.
const (
	CommitTypeFeat     CommitType = "feat"
	CommitTypeFix      CommitType = "fix"
	CommitTypeDocs     CommitType = "docs"
	CommitTypeStyle    CommitType = "style"
	CommitTypeRefactor CommitType = "refactor"
	CommitTypePerf     CommitType = "perf"
	CommitTypeTest     CommitType = "test"
	CommitTypeBuild    CommitType = "build"
	CommitTypeCI       CommitType = "ci"
	CommitTypeChore    CommitType = "chore"
	CommitTypeRevert   CommitType = "revert"
)

// AllCommitTypes returns all standard commit types.
func AllCommitTypes() []CommitType {
	return []CommitType{
		CommitTypeFeat,
		CommitTypeFix,
		CommitTypeDocs,
		CommitTypeStyle,
		CommitTypeRefactor,
		CommitTypePerf,
		CommitTypeTest,
		CommitTypeBuild,
		CommitTypeCI,
		CommitTypeChore,
		CommitTypeRevert,
	}
}

// IsValid returns true if the commit type is a recognized type.
func (t CommitType) IsValid() bool {
	switch t {
	case CommitTypeFeat, CommitTypeFix, CommitTypeDocs, CommitTypeStyle,
		CommitTypeRefactor, CommitTypePerf, CommitTypeTest, CommitTypeBuild,
		CommitTypeCI, CommitTypeChore, CommitTypeRevert:
		return true
	default:
		return false
	}
}

// String returns the string representation of the commit type.
func (t CommitType) String() string {
	return string(t)
}

// Description returns a human-readable description of the commit type.
func (t CommitType) Description() string {
	switch t {
	case CommitTypeFeat:
		return "A new feature"
	case CommitTypeFix:
		return "A bug fix"
	case CommitTypeDocs:
		return "Documentation only changes"
	case CommitTypeStyle:
		return "Changes that do not affect the meaning of the code"
	case CommitTypeRefactor:
		return "A code change that neither fixes a bug nor adds a feature"
	case CommitTypePerf:
		return "A code change that improves performance"
	case CommitTypeTest:
		return "Adding missing tests or correcting existing tests"
	case CommitTypeBuild:
		return "Changes that affect the build system or external dependencies"
	case CommitTypeCI:
		return "Changes to CI configuration files and scripts"
	case CommitTypeChore:
		return "Other changes that don't modify src or test files"
	case CommitTypeRevert:
		return "Reverts a previous commit"
	default:
		return "Unknown commit type"
	}
}

// AffectsChangelog returns true if this commit type should appear in changelog.
func (t CommitType) AffectsChangelog() bool {
	switch t {
	case CommitTypeFeat, CommitTypeFix, CommitTypePerf, CommitTypeRevert:
		return true
	default:
		return false
	}
}

// ParseCommitType parses a string into a CommitType.
// Returns the commit type and true if valid, or empty string and false if invalid.
func ParseCommitType(s string) (CommitType, bool) {
	t := CommitType(strings.ToLower(strings.TrimSpace(s)))
	if t.IsValid() {
		return t, true
	}
	return "", false
}

// ChangelogCategory returns the changelog category for this commit type.
func (t CommitType) ChangelogCategory() string {
	switch t {
	case CommitTypeFeat:
		return "Features"
	case CommitTypeFix:
		return "Bug Fixes"
	case CommitTypePerf:
		return "Performance Improvements"
	case CommitTypeDocs:
		return "Documentation"
	case CommitTypeRefactor:
		return "Code Refactoring"
	case CommitTypeTest:
		return "Tests"
	case CommitTypeBuild:
		return "Build System"
	case CommitTypeCI:
		return "Continuous Integration"
	case CommitTypeChore:
		return "Chores"
	case CommitTypeRevert:
		return "Reverts"
	case CommitTypeStyle:
		return "Styles"
	default:
		return "Other Changes"
	}
}
