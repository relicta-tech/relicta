package changes

import (
	"testing"
	"time"
)

func TestParseConventionalCommit(t *testing.T) {
	tests := []struct {
		name        string
		hash        string
		message     string
		wantType    CommitType
		wantScope   string
		wantSubject string
		wantBreak   bool
		wantNil     bool
	}{
		{
			name:        "simple feat",
			hash:        "abc1234",
			message:     "feat: add new feature",
			wantType:    CommitTypeFeat,
			wantScope:   "",
			wantSubject: "add new feature",
			wantBreak:   false,
		},
		{
			name:        "feat with scope",
			hash:        "def5678",
			message:     "feat(api): add new endpoint",
			wantType:    CommitTypeFeat,
			wantScope:   "api",
			wantSubject: "add new endpoint",
			wantBreak:   false,
		},
		{
			name:        "fix",
			hash:        "ghi9012",
			message:     "fix: resolve null pointer",
			wantType:    CommitTypeFix,
			wantSubject: "resolve null pointer",
		},
		{
			name:        "breaking change with exclamation",
			hash:        "jkl3456",
			message:     "feat!: redesign API",
			wantType:    CommitTypeFeat,
			wantSubject: "redesign API",
			wantBreak:   true,
		},
		{
			name:        "breaking change with scope and exclamation",
			hash:        "mno7890",
			message:     "feat(api)!: change response format",
			wantType:    CommitTypeFeat,
			wantScope:   "api",
			wantSubject: "change response format",
			wantBreak:   true,
		},
		{
			name:        "chore commit",
			hash:        "pqr1234",
			message:     "chore: update dependencies",
			wantType:    CommitTypeChore,
			wantSubject: "update dependencies",
		},
		{
			name:        "docs commit",
			hash:        "stu5678",
			message:     "docs: improve README",
			wantType:    CommitTypeDocs,
			wantSubject: "improve README",
		},
		{
			name:    "empty message",
			hash:    "xyz",
			message: "",
			wantNil: true,
		},
		{
			name:    "non-conventional message",
			hash:    "abc",
			message: "This is not a conventional commit",
			wantNil: true,
		},
		{
			name:    "no colon",
			hash:    "def",
			message: "feat add new feature",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseConventionalCommit(tt.hash, tt.message)

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseConventionalCommit() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("ParseConventionalCommit() = nil, want non-nil")
			}

			if got.Type() != tt.wantType {
				t.Errorf("Type() = %v, want %v", got.Type(), tt.wantType)
			}
			if got.Scope() != tt.wantScope {
				t.Errorf("Scope() = %v, want %v", got.Scope(), tt.wantScope)
			}
			if got.Subject() != tt.wantSubject {
				t.Errorf("Subject() = %v, want %v", got.Subject(), tt.wantSubject)
			}
			if got.IsBreaking() != tt.wantBreak {
				t.Errorf("IsBreaking() = %v, want %v", got.IsBreaking(), tt.wantBreak)
			}
			if got.Hash() != tt.hash {
				t.Errorf("Hash() = %v, want %v", got.Hash(), tt.hash)
			}
		})
	}
}

func TestParseConventionalCommit_WithBreakingChangeFooter(t *testing.T) {
	message := `feat: add new feature

This is the body of the commit.

BREAKING CHANGE: this breaks the API`

	got := ParseConventionalCommit("abc123", message)
	if got == nil {
		t.Fatal("ParseConventionalCommit() = nil, want non-nil")
	}

	if !got.IsBreaking() {
		t.Error("IsBreaking() = false, want true")
	}

	if got.BreakingMessage() != "this breaks the API" {
		t.Errorf("BreakingMessage() = %v, want 'this breaks the API'", got.BreakingMessage())
	}
}

func TestParseConventionalCommit_WithBody(t *testing.T) {
	message := `feat(api): add new endpoint

This is the body describing the change in detail.
It can span multiple lines.`

	got := ParseConventionalCommit("abc123", message)
	if got == nil {
		t.Fatal("ParseConventionalCommit() = nil, want non-nil")
	}

	expectedBody := `This is the body describing the change in detail.
It can span multiple lines.`
	if got.Body() != expectedBody {
		t.Errorf("Body() = %q, want %q", got.Body(), expectedBody)
	}
}

func TestNewConventionalCommit(t *testing.T) {
	now := time.Now()
	c := NewConventionalCommit(
		"abc123",
		CommitTypeFeat,
		"add new feature",
		WithScope("api"),
		WithBody("detailed description"),
		WithBreaking("breaks API"),
		WithAuthor("John Doe", "john@example.com"),
		WithDate(now),
	)

	if c.Hash() != "abc123" {
		t.Errorf("Hash() = %v, want abc123", c.Hash())
	}
	if c.Type() != CommitTypeFeat {
		t.Errorf("Type() = %v, want feat", c.Type())
	}
	if c.Subject() != "add new feature" {
		t.Errorf("Subject() = %v, want 'add new feature'", c.Subject())
	}
	if c.Scope() != "api" {
		t.Errorf("Scope() = %v, want api", c.Scope())
	}
	if c.Body() != "detailed description" {
		t.Errorf("Body() = %v, want 'detailed description'", c.Body())
	}
	if !c.IsBreaking() {
		t.Error("IsBreaking() = false, want true")
	}
	if c.BreakingMessage() != "breaks API" {
		t.Errorf("BreakingMessage() = %v, want 'breaks API'", c.BreakingMessage())
	}
	if c.Author() != "John Doe" {
		t.Errorf("Author() = %v, want 'John Doe'", c.Author())
	}
	if c.AuthorEmail() != "john@example.com" {
		t.Errorf("AuthorEmail() = %v, want 'john@example.com'", c.AuthorEmail())
	}
	if !c.Date().Equal(now) {
		t.Errorf("Date() = %v, want %v", c.Date(), now)
	}
}

func TestConventionalCommit_ShortHash(t *testing.T) {
	tests := []struct {
		hash     string
		expected string
	}{
		{"abcdefghij", "abcdefg"},
		{"abc", "abc"},
		{"1234567890", "1234567"},
	}

	for _, tt := range tests {
		c := NewConventionalCommit(tt.hash, CommitTypeFeat, "test")
		if got := c.ShortHash(); got != tt.expected {
			t.Errorf("ShortHash() = %v, want %v", got, tt.expected)
		}
	}
}

func TestConventionalCommit_ReleaseType(t *testing.T) {
	tests := []struct {
		name        string
		commitType  CommitType
		breaking    bool
		wantRelease ReleaseType
	}{
		{"feat", CommitTypeFeat, false, ReleaseTypeMinor},
		{"fix", CommitTypeFix, false, ReleaseTypePatch},
		{"perf", CommitTypePerf, false, ReleaseTypePatch},
		{"breaking feat", CommitTypeFeat, true, ReleaseTypeMajor},
		{"breaking fix", CommitTypeFix, true, ReleaseTypeMajor},
		{"docs", CommitTypeDocs, false, ReleaseTypeNone},
		{"chore", CommitTypeChore, false, ReleaseTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []ConventionalCommitOption
			if tt.breaking {
				opts = append(opts, WithBreaking("breaking change"))
			}
			c := NewConventionalCommit("abc123", tt.commitType, "test", opts...)
			if got := c.ReleaseType(); got != tt.wantRelease {
				t.Errorf("ReleaseType() = %v, want %v", got, tt.wantRelease)
			}
		})
	}
}

func TestConventionalCommit_AffectsChangelog(t *testing.T) {
	tests := []struct {
		name       string
		commitType CommitType
		breaking   bool
		want       bool
	}{
		{"feat", CommitTypeFeat, false, true},
		{"fix", CommitTypeFix, false, true},
		{"perf", CommitTypePerf, false, true},
		{"docs", CommitTypeDocs, false, false},
		{"chore", CommitTypeChore, false, false},
		{"breaking docs", CommitTypeDocs, true, true}, // Breaking changes always affect
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []ConventionalCommitOption
			if tt.breaking {
				opts = append(opts, WithBreaking("breaking"))
			}
			c := NewConventionalCommit("abc", tt.commitType, "test", opts...)
			if got := c.AffectsChangelog(); got != tt.want {
				t.Errorf("AffectsChangelog() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConventionalCommit_String(t *testing.T) {
	tests := []struct {
		name     string
		commit   *ConventionalCommit
		expected string
	}{
		{
			name:     "simple",
			commit:   NewConventionalCommit("abc", CommitTypeFeat, "add feature"),
			expected: "feat: add feature",
		},
		{
			name:     "with scope",
			commit:   NewConventionalCommit("abc", CommitTypeFeat, "add feature", WithScope("api")),
			expected: "feat(api): add feature",
		},
		{
			name:     "breaking",
			commit:   NewConventionalCommit("abc", CommitTypeFeat, "add feature", WithBreaking("breaks")),
			expected: "feat!: add feature",
		},
		{
			name:     "breaking with scope",
			commit:   NewConventionalCommit("abc", CommitTypeFeat, "add feature", WithScope("api"), WithBreaking("breaks")),
			expected: "feat(api)!: add feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.commit.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConventionalCommit_FormattedSubject(t *testing.T) {
	tests := []struct {
		name     string
		commit   *ConventionalCommit
		expected string
	}{
		{
			name:     "without scope",
			commit:   NewConventionalCommit("abc", CommitTypeFeat, "add feature"),
			expected: "add feature",
		},
		{
			name:     "with scope",
			commit:   NewConventionalCommit("abc", CommitTypeFeat, "add feature", WithScope("api")),
			expected: "**api:** add feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.commit.FormattedSubject(); got != tt.expected {
				t.Errorf("FormattedSubject() = %v, want %v", got, tt.expected)
			}
		})
	}
}
