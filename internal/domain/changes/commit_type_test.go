// Package changes provides domain types for analyzing commit changes.
package changes

import (
	"testing"
)

func TestCommitType_String(t *testing.T) {
	tests := []struct {
		ct       CommitType
		expected string
	}{
		{CommitTypeFeat, "feat"},
		{CommitTypeFix, "fix"},
		{CommitTypeDocs, "docs"},
		{CommitTypeStyle, "style"},
		{CommitTypeRefactor, "refactor"},
		{CommitTypePerf, "perf"},
		{CommitTypeTest, "test"},
		{CommitTypeBuild, "build"},
		{CommitTypeCI, "ci"},
		{CommitTypeChore, "chore"},
		{CommitTypeRevert, "revert"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCommitType_IsValid(t *testing.T) {
	validTypes := []CommitType{
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

	for _, ct := range validTypes {
		if !ct.IsValid() {
			t.Errorf("IsValid() = false for %s, want true", ct)
		}
	}

	invalidTypes := []CommitType{
		"invalid",
		"",
		"FEAT",
		"unknown",
		"feature",
	}

	for _, ct := range invalidTypes {
		if ct.IsValid() {
			t.Errorf("IsValid() = true for %q, want false", ct)
		}
	}
}

func TestCommitType_Description(t *testing.T) {
	tests := []struct {
		ct       CommitType
		expected string
	}{
		{CommitTypeFeat, "A new feature"},
		{CommitTypeFix, "A bug fix"},
		{CommitTypeDocs, "Documentation only changes"},
		{CommitTypeStyle, "Changes that do not affect the meaning of the code"},
		{CommitTypeRefactor, "A code change that neither fixes a bug nor adds a feature"},
		{CommitTypePerf, "A code change that improves performance"},
		{CommitTypeTest, "Adding missing tests or correcting existing tests"},
		{CommitTypeBuild, "Changes that affect the build system or external dependencies"},
		{CommitTypeCI, "Changes to CI configuration files and scripts"},
		{CommitTypeChore, "Other changes that don't modify src or test files"},
		{CommitTypeRevert, "Reverts a previous commit"},
	}

	for _, tt := range tests {
		t.Run(string(tt.ct), func(t *testing.T) {
			if got := tt.ct.Description(); got != tt.expected {
				t.Errorf("Description() = %q, want %q", got, tt.expected)
			}
		})
	}

	// Test unknown type
	unknown := CommitType("unknown")
	if got := unknown.Description(); got != "Unknown commit type" {
		t.Errorf("Description() for unknown = %q, want 'Unknown commit type'", got)
	}
}

func TestCommitType_AffectsChangelog(t *testing.T) {
	affectsChangelog := []CommitType{
		CommitTypeFeat,
		CommitTypeFix,
		CommitTypePerf,
		CommitTypeRevert,
	}

	for _, ct := range affectsChangelog {
		if !ct.AffectsChangelog() {
			t.Errorf("AffectsChangelog() = false for %s, want true", ct)
		}
	}

	doesNotAffectChangelog := []CommitType{
		CommitTypeDocs,
		CommitTypeStyle,
		CommitTypeRefactor,
		CommitTypeTest,
		CommitTypeBuild,
		CommitTypeCI,
		CommitTypeChore,
	}

	for _, ct := range doesNotAffectChangelog {
		if ct.AffectsChangelog() {
			t.Errorf("AffectsChangelog() = true for %s, want false", ct)
		}
	}
}

func TestCommitType_ChangelogCategory(t *testing.T) {
	tests := []struct {
		ct       CommitType
		expected string
	}{
		{CommitTypeFeat, "Features"},
		{CommitTypeFix, "Bug Fixes"},
		{CommitTypePerf, "Performance Improvements"},
		{CommitTypeDocs, "Documentation"},
		{CommitTypeRefactor, "Code Refactoring"},
		{CommitTypeTest, "Tests"},
		{CommitTypeBuild, "Build System"},
		{CommitTypeCI, "Continuous Integration"},
		{CommitTypeChore, "Chores"},
		{CommitTypeRevert, "Reverts"},
		{CommitTypeStyle, "Styles"},
	}

	for _, tt := range tests {
		t.Run(string(tt.ct), func(t *testing.T) {
			if got := tt.ct.ChangelogCategory(); got != tt.expected {
				t.Errorf("ChangelogCategory() = %q, want %q", got, tt.expected)
			}
		})
	}

	// Test unknown type
	unknown := CommitType("unknown")
	if got := unknown.ChangelogCategory(); got != "Other Changes" {
		t.Errorf("ChangelogCategory() for unknown = %q, want 'Other Changes'", got)
	}
}

func TestParseCommitType(t *testing.T) {
	tests := []struct {
		input  string
		wantCT CommitType
		wantOK bool
	}{
		{"feat", CommitTypeFeat, true},
		{"fix", CommitTypeFix, true},
		{"docs", CommitTypeDocs, true},
		{"FEAT", CommitTypeFeat, true},       // Case insensitive
		{"  fix  ", CommitTypeFix, true},     // Trim spaces
		{"  CHORE  ", CommitTypeChore, true}, // Both
		{"invalid", "", false},
		{"", "", false},
		{"feature", "", false}, // Not a valid type
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ct, ok := ParseCommitType(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseCommitType(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ct != tt.wantCT {
				t.Errorf("ParseCommitType(%q) = %v, want %v", tt.input, ct, tt.wantCT)
			}
		})
	}
}

func TestAllCommitTypes(t *testing.T) {
	types := AllCommitTypes()

	expectedCount := 11
	if len(types) != expectedCount {
		t.Errorf("AllCommitTypes() length = %d, want %d", len(types), expectedCount)
	}

	// Verify all returned types are valid
	for _, ct := range types {
		if !ct.IsValid() {
			t.Errorf("AllCommitTypes() contains invalid type: %v", ct)
		}
	}

	// Verify all expected types are present
	expected := map[CommitType]bool{
		CommitTypeFeat:     false,
		CommitTypeFix:      false,
		CommitTypeDocs:     false,
		CommitTypeStyle:    false,
		CommitTypeRefactor: false,
		CommitTypePerf:     false,
		CommitTypeTest:     false,
		CommitTypeBuild:    false,
		CommitTypeCI:       false,
		CommitTypeChore:    false,
		CommitTypeRevert:   false,
	}

	for _, ct := range types {
		expected[ct] = true
	}

	for ct, found := range expected {
		if !found {
			t.Errorf("AllCommitTypes() missing type: %s", ct)
		}
	}
}
