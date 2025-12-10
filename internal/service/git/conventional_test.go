// Package git provides tests for conventional commit parsing.
package git

import (
	"testing"
	"time"
)

func TestParseConventionalCommit(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		wantType     CommitType
		wantScope    string
		wantDesc     string
		wantBreaking bool
		wantConv     bool
	}{
		{
			name:         "simple feat",
			message:      "feat: add new feature",
			wantType:     CommitTypeFeat,
			wantScope:    "",
			wantDesc:     "add new feature",
			wantBreaking: false,
			wantConv:     true,
		},
		{
			name:         "feat with scope",
			message:      "feat(auth): add login",
			wantType:     CommitTypeFeat,
			wantScope:    "auth",
			wantDesc:     "add login",
			wantBreaking: false,
			wantConv:     true,
		},
		{
			name:         "breaking change with bang",
			message:      "feat!: breaking change",
			wantType:     CommitTypeFeat,
			wantScope:    "",
			wantDesc:     "breaking change",
			wantBreaking: true,
			wantConv:     true,
		},
		{
			name:         "breaking with scope and bang",
			message:      "feat(api)!: breaking API change",
			wantType:     CommitTypeFeat,
			wantScope:    "api",
			wantDesc:     "breaking API change",
			wantBreaking: true,
			wantConv:     true,
		},
		{
			name:         "fix commit",
			message:      "fix: resolve bug",
			wantType:     CommitTypeFix,
			wantScope:    "",
			wantDesc:     "resolve bug",
			wantBreaking: false,
			wantConv:     true,
		},
		{
			name:         "docs commit",
			message:      "docs: update readme",
			wantType:     CommitTypeDocs,
			wantScope:    "",
			wantDesc:     "update readme",
			wantBreaking: false,
			wantConv:     true,
		},
		{
			name:         "chore commit",
			message:      "chore(deps): update dependencies",
			wantType:     CommitTypeChore,
			wantScope:    "deps",
			wantDesc:     "update dependencies",
			wantBreaking: false,
			wantConv:     true,
		},
		{
			name:         "non-conventional commit",
			message:      "Update the thing",
			wantType:     CommitTypeUnknown,
			wantScope:    "",
			wantDesc:     "Update the thing",
			wantBreaking: false,
			wantConv:     false,
		},
		{
			name:         "perf commit",
			message:      "perf: improve query performance",
			wantType:     CommitTypePerf,
			wantScope:    "",
			wantDesc:     "improve query performance",
			wantBreaking: false,
			wantConv:     true,
		},
		{
			name:         "refactor commit",
			message:      "refactor(core): restructure modules",
			wantType:     CommitTypeRefactor,
			wantScope:    "core",
			wantDesc:     "restructure modules",
			wantBreaking: false,
			wantConv:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc, err := ParseConventionalCommit(tt.message)
			if err != nil {
				t.Fatalf("ParseConventionalCommit() error = %v", err)
			}

			if cc.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", cc.Type, tt.wantType)
			}
			if cc.Scope != tt.wantScope {
				t.Errorf("Scope = %v, want %v", cc.Scope, tt.wantScope)
			}
			if cc.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", cc.Description, tt.wantDesc)
			}
			if cc.Breaking != tt.wantBreaking {
				t.Errorf("Breaking = %v, want %v", cc.Breaking, tt.wantBreaking)
			}
			if cc.IsConventional != tt.wantConv {
				t.Errorf("IsConventional = %v, want %v", cc.IsConventional, tt.wantConv)
			}
		})
	}
}

func TestParseConventionalCommit_WithBody(t *testing.T) {
	message := `feat(auth): add OAuth2 support

This adds OAuth2 authentication support with multiple providers.

BREAKING CHANGE: removed legacy auth methods`

	cc, err := ParseConventionalCommit(message)
	if err != nil {
		t.Fatalf("ParseConventionalCommit() error = %v", err)
	}

	if cc.Type != CommitTypeFeat {
		t.Errorf("Type = %v, want feat", cc.Type)
	}
	if cc.Scope != "auth" {
		t.Errorf("Scope = %v, want auth", cc.Scope)
	}
	if !cc.Breaking {
		t.Error("Expected breaking change to be detected from footer")
	}
}

func TestDetectReleaseType(t *testing.T) {
	tests := []struct {
		name    string
		commits []ConventionalCommit
		want    ReleaseType
	}{
		{
			name:    "empty commits",
			commits: []ConventionalCommit{},
			want:    ReleaseTypeNone,
		},
		{
			name: "breaking change",
			commits: []ConventionalCommit{
				{Type: CommitTypeFeat, Breaking: true},
			},
			want: ReleaseTypeMajor,
		},
		{
			name: "feature without breaking",
			commits: []ConventionalCommit{
				{Type: CommitTypeFeat, Breaking: false},
			},
			want: ReleaseTypeMinor,
		},
		{
			name: "fix only",
			commits: []ConventionalCommit{
				{Type: CommitTypeFix},
			},
			want: ReleaseTypePatch,
		},
		{
			name: "perf only",
			commits: []ConventionalCommit{
				{Type: CommitTypePerf},
			},
			want: ReleaseTypePatch,
		},
		{
			name: "mixed - breaking takes precedence",
			commits: []ConventionalCommit{
				{Type: CommitTypeFix},
				{Type: CommitTypeFeat},
				{Type: CommitTypeFeat, Breaking: true},
			},
			want: ReleaseTypeMajor,
		},
		{
			name: "mixed - feat over fix",
			commits: []ConventionalCommit{
				{Type: CommitTypeFix},
				{Type: CommitTypeFeat},
				{Type: CommitTypeDocs},
			},
			want: ReleaseTypeMinor,
		},
		{
			name: "docs only still triggers patch",
			commits: []ConventionalCommit{
				{Type: CommitTypeDocs},
			},
			want: ReleaseTypePatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectReleaseType(tt.commits)
			if got != tt.want {
				t.Errorf("DetectReleaseType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategorizeCommits(t *testing.T) {
	commits := []ConventionalCommit{
		{Type: CommitTypeFeat, Description: "feature 1"},
		{Type: CommitTypeFeat, Description: "feature 2"},
		{Type: CommitTypeFix, Description: "fix 1"},
		{Type: CommitTypePerf, Description: "perf 1"},
		{Type: CommitTypeDocs, Description: "docs 1"},
		{Type: CommitTypeRefactor, Description: "refactor 1"},
		{Type: CommitTypeFeat, Breaking: true, Description: "breaking feat"},
		{Type: CommitTypeChore, Description: "chore 1"},
	}

	changes := CategorizeCommits(commits)

	if len(changes.Features) != 3 {
		t.Errorf("Features count = %d, want 3", len(changes.Features))
	}
	if len(changes.Fixes) != 1 {
		t.Errorf("Fixes count = %d, want 1", len(changes.Fixes))
	}
	if len(changes.Performance) != 1 {
		t.Errorf("Performance count = %d, want 1", len(changes.Performance))
	}
	if len(changes.Documentation) != 1 {
		t.Errorf("Documentation count = %d, want 1", len(changes.Documentation))
	}
	if len(changes.Refactoring) != 1 {
		t.Errorf("Refactoring count = %d, want 1", len(changes.Refactoring))
	}
	if len(changes.Breaking) != 1 {
		t.Errorf("Breaking count = %d, want 1", len(changes.Breaking))
	}
	if len(changes.Other) != 1 {
		t.Errorf("Other count = %d, want 1", len(changes.Other))
	}
	if len(changes.All) != 8 {
		t.Errorf("All count = %d, want 8", len(changes.All))
	}
}

func TestFilterCommits(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	commits := []ConventionalCommit{
		{Type: CommitTypeFeat, Scope: "api", Commit: Commit{Author: Author{Email: "dev@example.com"}, Date: now}},
		{Type: CommitTypeFix, Scope: "ui", Commit: Commit{Author: Author{Email: "dev@example.com"}, Date: now}},
		{Type: CommitTypeFeat, Scope: "api", Breaking: true, Commit: Commit{Author: Author{Email: "lead@example.com"}, Date: now}},
		{Type: CommitTypeChore, Commit: Commit{Author: Author{Email: "bot@example.com"}, Date: now}, IsConventional: true},
	}

	tests := []struct {
		name   string
		filter CommitFilter
		want   int
	}{
		{
			name:   "no filter",
			filter: CommitFilter{IncludeNonConventional: true},
			want:   4,
		},
		{
			name:   "filter by type feat",
			filter: CommitFilter{Types: []CommitType{CommitTypeFeat}, IncludeNonConventional: true},
			want:   2,
		},
		{
			name:   "filter by scope api",
			filter: CommitFilter{Scopes: []string{"api"}, IncludeNonConventional: true},
			want:   2,
		},
		{
			name:   "exclude author bot",
			filter: CommitFilter{ExcludeAuthors: []string{"bot@example.com"}, IncludeNonConventional: true},
			want:   3,
		},
		{
			name:   "only breaking",
			filter: CommitFilter{OnlyBreaking: true, IncludeNonConventional: true},
			want:   1,
		},
		{
			name:   "filter by author",
			filter: CommitFilter{Authors: []string{"dev@example.com"}, IncludeNonConventional: true},
			want:   2,
		},
		{
			name:   "filter since yesterday",
			filter: CommitFilter{Since: &yesterday, IncludeNonConventional: true},
			want:   4,
		},
		{
			name:   "filter until tomorrow",
			filter: CommitFilter{Until: &tomorrow, IncludeNonConventional: true},
			want:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterCommits(commits, tt.filter)
			if len(filtered) != tt.want {
				t.Errorf("FilterCommits() returned %d commits, want %d", len(filtered), tt.want)
			}
		})
	}
}

func TestIsFooterLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"BREAKING CHANGE: something", true},
		{"BREAKING-CHANGE: something", true},
		{"breaking change: something", true},
		{"Fixes: #123", true},
		{"Closes: #456", true},
		{"Co-authored-by: Name <email>", true},
		{"Signed-off-by: Name <email>", true},
		{"Refs: #789", true},
		{"token: value", true},
		{"token #123", true},
		{"This is just text", false},
		{"", false},
		{"some text with: colon", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := isFooterLine(tt.line)
			if result != tt.expected {
				t.Errorf("isFooterLine(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestFooterPatternsUpperPrecomputed(t *testing.T) {
	// Verify that footer patterns are pre-computed and uppercase
	// Test prefixes
	for _, prefix := range footerPatternPrefixes {
		for _, r := range prefix {
			if r >= 'a' && r <= 'z' {
				t.Errorf("footerPatternPrefixes contains lowercase character in %q", prefix)
				break
			}
		}
	}

	// Test patterns in map
	for prefix, patterns := range footerPatternsMap {
		for _, r := range prefix {
			if r >= 'a' && r <= 'z' {
				t.Errorf("footerPatternsMap key contains lowercase character in %q", prefix)
				break
			}
		}
		for _, pattern := range patterns {
			for _, r := range pattern {
				if r >= 'a' && r <= 'z' {
					t.Errorf("footerPatternsMap value contains lowercase character in %q", pattern)
					break
				}
			}
		}
	}
}

func TestCommitTypeDisplayName(t *testing.T) {
	tests := []struct {
		commitType CommitType
		expected   string
	}{
		{CommitTypeFeat, "Features"},
		{CommitTypeFix, "Bug Fixes"},
		{CommitTypeDocs, "Documentation"},
		{CommitTypePerf, "Performance Improvements"},
		{CommitTypeRefactor, "Code Refactoring"},
		{CommitTypeUnknown, "Other Changes"},
	}

	for _, tt := range tests {
		t.Run(string(tt.commitType), func(t *testing.T) {
			result := CommitTypeDisplayName(tt.commitType)
			if result != tt.expected {
				t.Errorf("CommitTypeDisplayName(%v) = %q, want %q", tt.commitType, result, tt.expected)
			}
		})
	}
}

func TestFormatConventionalCommit(t *testing.T) {
	cc := &ConventionalCommit{
		Type:        CommitTypeFeat,
		Scope:       "api",
		Description: "add new endpoint",
		Body:        "This adds a new endpoint.",
		Breaking:    true,
	}

	result := FormatConventionalCommit(cc)
	expected := "feat(api)!: add new endpoint\n\nThis adds a new endpoint."

	if result != expected {
		t.Errorf("FormatConventionalCommit() = %q, want %q", result, expected)
	}
}

func TestCategorizedChanges_Methods(t *testing.T) {
	t.Run("HasChanges", func(t *testing.T) {
		empty := &CategorizedChanges{}
		if empty.HasChanges() {
			t.Error("empty changes should return false")
		}

		withChanges := &CategorizedChanges{All: []ConventionalCommit{{}}}
		if !withChanges.HasChanges() {
			t.Error("non-empty changes should return true")
		}
	})

	t.Run("HasBreakingChanges", func(t *testing.T) {
		noBreaking := &CategorizedChanges{}
		if noBreaking.HasBreakingChanges() {
			t.Error("should return false with no breaking changes")
		}

		withBreaking := &CategorizedChanges{Breaking: []ConventionalCommit{{}}}
		if !withBreaking.HasBreakingChanges() {
			t.Error("should return true with breaking changes")
		}
	})

	t.Run("TotalCount", func(t *testing.T) {
		changes := &CategorizedChanges{
			All: []ConventionalCommit{{}, {}, {}},
		}
		if changes.TotalCount() != 3 {
			t.Errorf("TotalCount() = %d, want 3", changes.TotalCount())
		}
	})

	t.Run("DetermineReleaseType", func(t *testing.T) {
		tests := []struct {
			name    string
			changes *CategorizedChanges
			want    ReleaseType
		}{
			{
				name:    "empty",
				changes: &CategorizedChanges{},
				want:    ReleaseTypeNone,
			},
			{
				name:    "breaking",
				changes: &CategorizedChanges{Breaking: []ConventionalCommit{{}}},
				want:    ReleaseTypeMajor,
			},
			{
				name:    "features",
				changes: &CategorizedChanges{Features: []ConventionalCommit{{}}},
				want:    ReleaseTypeMinor,
			},
			{
				name:    "fixes",
				changes: &CategorizedChanges{Fixes: []ConventionalCommit{{}}, All: []ConventionalCommit{{}}},
				want:    ReleaseTypePatch,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := tt.changes.DetermineReleaseType()
				if got != tt.want {
					t.Errorf("DetermineReleaseType() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

// TestValidateGitRef tests the git reference validation function for security.
func TestValidateGitRef(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		// Valid refs
		{name: "empty string", ref: "", wantErr: false},
		{name: "HEAD", ref: "HEAD", wantErr: false},
		{name: "simple branch", ref: "main", wantErr: false},
		{name: "feature branch", ref: "feature/my-feature", wantErr: false},
		{name: "release branch", ref: "release-1.0", wantErr: false},
		{name: "version tag", ref: "v1.0.0", wantErr: false},
		{name: "semver tag", ref: "v2.3.4-beta.1", wantErr: false},
		{name: "short sha", ref: "abc1234", wantErr: false},
		{name: "full sha", ref: "abc1234567890def1234567890abc1234567890de", wantErr: false},
		{name: "relative ref HEAD~1", ref: "HEAD~1", wantErr: false},
		{name: "relative ref HEAD^2", ref: "HEAD^2", wantErr: false},
		{name: "relative ref main~5", ref: "main~5", wantErr: false},
		{name: "remote ref", ref: "origin/main", wantErr: false},
		{name: "upstream ref", ref: "upstream/feature", wantErr: false},
		{name: "underscores", ref: "feature_branch", wantErr: false},
		{name: "dots", ref: "v1.2.3", wantErr: false},

		// Invalid refs - command injection attempts
		{name: "option prefix", ref: "--exec=whoami", wantErr: true},
		{name: "semicolon injection", ref: "main;rm -rf /", wantErr: true},
		{name: "pipe injection", ref: "main|cat /etc/passwd", wantErr: true},
		{name: "ampersand injection", ref: "main&whoami", wantErr: true},
		{name: "backtick injection", ref: "main`whoami`", wantErr: true},
		{name: "command substitution $(...)", ref: "main$(whoami)", wantErr: true},
		{name: "variable expansion ${...}", ref: "main${HOME}", wantErr: true},
		{name: "newline injection", ref: "main\nwhoami", wantErr: true},
		{name: "carriage return injection", ref: "main\rwhoami", wantErr: true},
		{name: "empty command substitution", ref: "main$()", wantErr: true},

		// Invalid refs - path traversal
		{name: "path traversal ..", ref: "..", wantErr: true},
		{name: "path traversal ../etc", ref: "../etc/passwd", wantErr: true},

		// Invalid refs - length
		{name: "too long ref", ref: string(make([]byte, 251)), wantErr: true},

		// Invalid refs - invalid characters
		{name: "space in ref", ref: "my branch", wantErr: true},
		{name: "starts with dot", ref: ".hidden", wantErr: true},
		{name: "starts with dash", ref: "-invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			}
		})
	}
}

// TestValidateGitRef_DangerousPatterns tests all dangerous patterns are caught.
func TestValidateGitRef_DangerousPatterns(t *testing.T) {
	// These are the dangerous patterns from types.go
	dangerousPatterns := []string{
		"--",  // Option prefix
		";",   // Command separator
		"|",   // Pipe
		"&",   // Background/AND
		"`",   // Command substitution
		"$(",  // Command substitution
		"${",  // Variable expansion
		"\n",  // Newline
		"\r",  // Carriage return
		"$()", // Command substitution
		"..",  // Path traversal
	}

	for _, pattern := range dangerousPatterns {
		testRef := "prefix" + pattern + "suffix"
		t.Run("pattern_"+pattern, func(t *testing.T) {
			err := ValidateGitRef(testRef)
			if err == nil {
				t.Errorf("ValidateGitRef(%q) should reject dangerous pattern %q", testRef, pattern)
			}
		})
	}
}

// TestMustValidateGitRef tests the panic behavior of MustValidateGitRef.
func TestMustValidateGitRef(t *testing.T) {
	t.Run("valid ref returns unchanged", func(t *testing.T) {
		result := MustValidateGitRef("main")
		if result != "main" {
			t.Errorf("MustValidateGitRef(\"main\") = %q, want \"main\"", result)
		}
	})

	t.Run("empty ref returns unchanged", func(t *testing.T) {
		result := MustValidateGitRef("")
		if result != "" {
			t.Errorf("MustValidateGitRef(\"\") = %q, want \"\"", result)
		}
	})

	t.Run("invalid ref panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustValidateGitRef should panic for invalid ref")
			}
		}()
		MustValidateGitRef("--exec=whoami")
	})
}

func TestIsVersionBump(t *testing.T) {
	tests := []struct {
		commitType CommitType
		want       bool
	}{
		{CommitTypeFeat, true},
		{CommitTypeFix, true},
		{CommitTypePerf, true},
		{CommitTypeDocs, false},
		{CommitTypeStyle, false},
		{CommitTypeRefactor, false},
		{CommitTypeTest, false},
		{CommitTypeBuild, false},
		{CommitTypeCI, false},
		{CommitTypeChore, false},
		{CommitTypeRevert, false},
		{CommitTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.commitType), func(t *testing.T) {
			got := IsVersionBump(tt.commitType)
			if got != tt.want {
				t.Errorf("IsVersionBump(%v) = %v, want %v", tt.commitType, got, tt.want)
			}
		})
	}
}

func TestIsMajorTrigger(t *testing.T) {
	tests := []struct {
		commitType CommitType
		want       bool
	}{
		{CommitTypeFeat, true},
		{CommitTypeFix, false},
		{CommitTypePerf, false},
		{CommitTypeDocs, false},
		{CommitTypeChore, false},
		{CommitTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.commitType), func(t *testing.T) {
			got := IsMajorTrigger(tt.commitType)
			if got != tt.want {
				t.Errorf("IsMajorTrigger(%v) = %v, want %v", tt.commitType, got, tt.want)
			}
		})
	}
}

func TestPriority(t *testing.T) {
	tests := []struct {
		releaseType ReleaseType
		want        int
	}{
		{ReleaseTypeMajor, 3},
		{ReleaseTypeMinor, 2},
		{ReleaseTypePatch, 1},
		{ReleaseTypeNone, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.releaseType), func(t *testing.T) {
			got := Priority(tt.releaseType)
			if got != tt.want {
				t.Errorf("Priority(%v) = %v, want %v", tt.releaseType, got, tt.want)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	t.Run("DefaultTagOptions", func(t *testing.T) {
		opts := DefaultTagOptions()
		if !opts.Annotated {
			t.Error("DefaultTagOptions: Annotated should be true")
		}
		if opts.Sign {
			t.Error("DefaultTagOptions: Sign should be false")
		}
		if opts.Force {
			t.Error("DefaultTagOptions: Force should be false")
		}
		if opts.Ref != "HEAD" {
			t.Errorf("DefaultTagOptions: Ref = %q, want \"HEAD\"", opts.Ref)
		}
	})

	t.Run("DefaultPushOptions", func(t *testing.T) {
		opts := DefaultPushOptions()
		if opts.Remote != "origin" {
			t.Errorf("DefaultPushOptions: Remote = %q, want \"origin\"", opts.Remote)
		}
		if opts.Force {
			t.Error("DefaultPushOptions: Force should be false")
		}
		if opts.Tags {
			t.Error("DefaultPushOptions: Tags should be false")
		}
		if opts.DryRun {
			t.Error("DefaultPushOptions: DryRun should be false")
		}
	})

	t.Run("DefaultFetchOptions", func(t *testing.T) {
		opts := DefaultFetchOptions()
		if opts.Remote != "origin" {
			t.Errorf("DefaultFetchOptions: Remote = %q, want \"origin\"", opts.Remote)
		}
		if !opts.Tags {
			t.Error("DefaultFetchOptions: Tags should be true")
		}
		if opts.Prune {
			t.Error("DefaultFetchOptions: Prune should be false")
		}
		if opts.Depth != 0 {
			t.Errorf("DefaultFetchOptions: Depth = %d, want 0", opts.Depth)
		}
	})

	t.Run("DefaultPullOptions", func(t *testing.T) {
		opts := DefaultPullOptions()
		if opts.Remote != "origin" {
			t.Errorf("DefaultPullOptions: Remote = %q, want \"origin\"", opts.Remote)
		}
		if opts.Rebase {
			t.Error("DefaultPullOptions: Rebase should be false")
		}
		if opts.Depth != 0 {
			t.Errorf("DefaultPullOptions: Depth = %d, want 0", opts.Depth)
		}
	})

	t.Run("DefaultServiceConfig", func(t *testing.T) {
		cfg := DefaultServiceConfig()
		if cfg.RepoPath != "." {
			t.Errorf("DefaultServiceConfig: RepoPath = %q, want \".\"", cfg.RepoPath)
		}
		if cfg.DefaultRemote != "origin" {
			t.Errorf("DefaultServiceConfig: DefaultRemote = %q, want \"origin\"", cfg.DefaultRemote)
		}
		if cfg.GPGSign {
			t.Error("DefaultServiceConfig: GPGSign should be false")
		}
	})
}

func TestServiceOptions(t *testing.T) {
	t.Run("WithRepoPath", func(t *testing.T) {
		cfg := DefaultServiceConfig()
		opt := WithRepoPath("/custom/path")
		opt(&cfg)
		if cfg.RepoPath != "/custom/path" {
			t.Errorf("WithRepoPath: RepoPath = %q, want \"/custom/path\"", cfg.RepoPath)
		}
	})

	t.Run("WithDefaultRemote", func(t *testing.T) {
		cfg := DefaultServiceConfig()
		opt := WithDefaultRemote("upstream")
		opt(&cfg)
		if cfg.DefaultRemote != "upstream" {
			t.Errorf("WithDefaultRemote: DefaultRemote = %q, want \"upstream\"", cfg.DefaultRemote)
		}
	})

	t.Run("WithGPGSign", func(t *testing.T) {
		cfg := DefaultServiceConfig()
		opt := WithGPGSign("ABCD1234")
		opt(&cfg)
		if !cfg.GPGSign {
			t.Error("WithGPGSign: GPGSign should be true")
		}
		if cfg.GPGKeyID != "ABCD1234" {
			t.Errorf("WithGPGSign: GPGKeyID = %q, want \"ABCD1234\"", cfg.GPGKeyID)
		}
	})
}

func TestCategorizedChanges_DetermineReleaseType_PerformancePath(t *testing.T) {
	// Test the performance path specifically (fixes + performance triggers patch)
	changes := &CategorizedChanges{
		Performance: []ConventionalCommit{{}},
		All:         []ConventionalCommit{{}},
	}
	got := changes.DetermineReleaseType()
	if got != ReleaseTypePatch {
		t.Errorf("DetermineReleaseType() with performance = %v, want %v", got, ReleaseTypePatch)
	}
}

// TestCommitTypeEmoji tests the emoji function for all commit types.
func TestCommitTypeEmoji(t *testing.T) {
	tests := []struct {
		commitType CommitType
		expected   string
	}{
		{CommitTypeFeat, "✨"},
		{CommitTypeFix, "🐛"},
		{CommitTypeDocs, "📚"},
		{CommitTypeStyle, "💄"},
		{CommitTypeRefactor, "♻️"},
		{CommitTypePerf, "⚡"},
		{CommitTypeTest, "✅"},
		{CommitTypeBuild, "📦"},
		{CommitTypeCI, "👷"},
		{CommitTypeChore, "🔧"},
		{CommitTypeRevert, "⏪"},
		{CommitTypeUnknown, "📝"},
	}

	for _, tt := range tests {
		t.Run(string(tt.commitType), func(t *testing.T) {
			result := CommitTypeEmoji(tt.commitType)
			if result != tt.expected {
				t.Errorf("CommitTypeEmoji(%v) = %q, want %q", tt.commitType, result, tt.expected)
			}
		})
	}
}

// TestCommitTypeDisplayName_AllTypes tests all commit type display names.
func TestCommitTypeDisplayName_AllTypes(t *testing.T) {
	tests := []struct {
		commitType CommitType
		expected   string
	}{
		{CommitTypeFeat, "Features"},
		{CommitTypeFix, "Bug Fixes"},
		{CommitTypeDocs, "Documentation"},
		{CommitTypeStyle, "Styles"},
		{CommitTypeRefactor, "Code Refactoring"},
		{CommitTypePerf, "Performance Improvements"},
		{CommitTypeTest, "Tests"},
		{CommitTypeBuild, "Build System"},
		{CommitTypeCI, "Continuous Integration"},
		{CommitTypeChore, "Chores"},
		{CommitTypeRevert, "Reverts"},
		{CommitTypeUnknown, "Other Changes"},
	}

	for _, tt := range tests {
		t.Run(string(tt.commitType), func(t *testing.T) {
			result := CommitTypeDisplayName(tt.commitType)
			if result != tt.expected {
				t.Errorf("CommitTypeDisplayName(%v) = %q, want %q", tt.commitType, result, tt.expected)
			}
		})
	}
}

// TestParseReferences tests reference parsing in commit messages.
func TestParseReferences(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		wantRefs int
		wantType string
		wantID   string
	}{
		{
			name:     "simple issue reference",
			message:  "fix: resolve bug #123",
			wantRefs: 1,
			wantType: "ref",
			wantID:   "123",
		},
		{
			name:     "closes reference",
			message:  "fix: bug\n\nCloses #456",
			wantRefs: 1,
			wantType: "closes",
			wantID:   "456",
		},
		{
			name:     "fixes reference",
			message:  "fix: bug\n\nFixes #789",
			wantRefs: 1,
			wantType: "fixes",
			wantID:   "789",
		},
		{
			name:     "resolves reference",
			message:  "fix: bug\n\nResolves #101",
			wantRefs: 1,
			wantType: "resolves",
			wantID:   "101",
		},
		{
			name:     "multiple references",
			message:  "fix: bugs\n\nCloses #123\nFixes #456",
			wantRefs: 2,
			wantType: "closes",
			wantID:   "123",
		},
		{
			name:     "GH- prefix",
			message:  "fix: bug GH-999",
			wantRefs: 1,
			wantType: "ref",
			wantID:   "999",
		},
		{
			name:     "no references",
			message:  "fix: bug with no refs",
			wantRefs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := parseReferences(tt.message)
			if len(refs) != tt.wantRefs {
				t.Errorf("parseReferences() returned %d refs, want %d", len(refs), tt.wantRefs)
			}
			if tt.wantRefs > 0 {
				if refs[0].Type != tt.wantType {
					t.Errorf("Reference type = %v, want %v", refs[0].Type, tt.wantType)
				}
				if refs[0].ID != tt.wantID {
					t.Errorf("Reference ID = %v, want %v", refs[0].ID, tt.wantID)
				}
			}
		})
	}
}

// TestParseConventionalCommit_WithReferences tests parsing with references enabled.
func TestParseConventionalCommit_WithReferences(t *testing.T) {
	message := `fix: resolve authentication bug

This fixes the login issue.

Closes #123
Fixes #456`

	commit := Commit{
		Message: message,
		Subject: "fix: resolve authentication bug",
		Body: `This fixes the login issue.

Closes #123
Fixes #456`,
	}

	opts := DefaultParseOptions()
	opts.ParseReferences = true

	cc, err := ParseConventionalCommitWithOptions(commit, opts)
	if err != nil {
		t.Fatalf("ParseConventionalCommitWithOptions() error = %v", err)
	}

	if len(cc.References) != 2 {
		t.Errorf("References count = %d, want 2", len(cc.References))
	}
}

// TestFormatConventionalCommit_EdgeCases tests formatting edge cases.
func TestFormatConventionalCommit_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		cc       *ConventionalCommit
		expected string
	}{
		{
			name: "minimal commit",
			cc: &ConventionalCommit{
				Type:        CommitTypeFix,
				Description: "bug fix",
			},
			expected: "fix: bug fix",
		},
		{
			name: "with scope no breaking",
			cc: &ConventionalCommit{
				Type:        CommitTypeFeat,
				Scope:       "api",
				Description: "new endpoint",
			},
			expected: "feat(api): new endpoint",
		},
		{
			name: "with body and footer",
			cc: &ConventionalCommit{
				Type:        CommitTypeFeat,
				Description: "feature",
				Body:        "Body text",
				Footer:      "BREAKING CHANGE: API changed",
			},
			expected: "feat: feature\n\nBody text\n\nBREAKING CHANGE: API changed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatConventionalCommit(tt.cc)
			if result != tt.expected {
				t.Errorf("FormatConventionalCommit() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestCompileFilter_EdgeCases tests filter compilation edge cases.
func TestCompileFilter_EdgeCases(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		filter := CommitFilter{}
		cf := compileFilter(filter)
		if cf == nil {
			t.Error("compileFilter() should return non-nil for empty filter")
		}
	})

	t.Run("filter with all fields", func(t *testing.T) {
		filter := CommitFilter{
			Types:          []CommitType{CommitTypeFeat, CommitTypeFix},
			Authors:        []string{"dev@example.com"},
			ExcludeAuthors: []string{"bot@example.com"},
			Scopes:         []string{"api", "ui"},
			ExcludeScopes:  []string{"test"},
		}
		cf := compileFilter(filter)
		if cf == nil {
			t.Fatal("compileFilter() should return non-nil")
		}
		if len(cf.types) != 2 {
			t.Errorf("types map size = %d, want 2", len(cf.types))
		}
		if len(cf.authors) != 1 {
			t.Errorf("authors map size = %d, want 1", len(cf.authors))
		}
		if len(cf.excludeAuthors) != 1 {
			t.Errorf("excludeAuthors map size = %d, want 1", len(cf.excludeAuthors))
		}
		if len(cf.scopes) != 2 {
			t.Errorf("scopes map size = %d, want 2", len(cf.scopes))
		}
		if len(cf.excludeScopes) != 1 {
			t.Errorf("excludeScopes map size = %d, want 1", len(cf.excludeScopes))
		}
	})
}

// TestMatches_EdgeCases tests the matches function with edge cases.
func TestMatches_EdgeCases(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	commit := ConventionalCommit{
		Type:           CommitTypeFeat,
		Scope:          "api",
		IsConventional: true,
		Commit: Commit{
			Author: Author{Email: "dev@example.com"},
			Date:   now,
		},
	}

	tests := []struct {
		name    string
		filter  CommitFilter
		matches bool
	}{
		{
			name:    "exclude scope match",
			filter:  CommitFilter{ExcludeScopes: []string{"api"}, IncludeNonConventional: true},
			matches: false,
		},
		{
			name:    "date since future",
			filter:  CommitFilter{Since: &tomorrow, IncludeNonConventional: true},
			matches: false,
		},
		{
			name:    "date until past",
			filter:  CommitFilter{Until: &yesterday, IncludeNonConventional: true},
			matches: false,
		},
		{
			name:    "non-conventional excluded",
			filter:  CommitFilter{IncludeNonConventional: false},
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := compileFilter(tt.filter)
			result := cf.matches(commit)
			if result != tt.matches {
				t.Errorf("matches() = %v, want %v", result, tt.matches)
			}
		})
	}
}

// TestParseConventionalCommit_StrictMode tests strict mode parsing.
func TestParseConventionalCommit_StrictMode(t *testing.T) {
	commit := Commit{
		Message: "Invalid commit message",
		Subject: "Invalid commit message",
	}

	opts := DefaultParseOptions()
	opts.StrictMode = true

	_, err := ParseConventionalCommitWithOptions(commit, opts)
	if err == nil {
		t.Error("ParseConventionalCommitWithOptions() should return error in strict mode for invalid commit")
	}
}

// TestValidateGitRef_EdgeCases tests edge cases and boundary conditions.
func TestValidateGitRef_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		// Boundary for length
		{name: "exactly 250 chars", ref: string(make([]byte, 250)), wantErr: true},     // Invalid chars
		{name: "250 valid chars", ref: "a" + string(make([]byte, 249)), wantErr: true}, // Has null bytes

		// Valid edge cases
		{name: "single char", ref: "a", wantErr: false},
		{name: "numbers only", ref: "123456", wantErr: false},
		{name: "version with rc", ref: "v1.0.0-rc.1", wantErr: false},
		{name: "complex path", ref: "refs/heads/feature/test", wantErr: false},
		{name: "HEAD with caret", ref: "HEAD^", wantErr: false},
		{name: "HEAD with tilde", ref: "HEAD~", wantErr: false},
		{name: "double caret", ref: "HEAD^^", wantErr: false},
		{name: "double tilde", ref: "HEAD~~", wantErr: false},

		// Common attack vectors that should fail
		{name: "null byte", ref: "main\x00rm", wantErr: true},
		{name: "url encoded", ref: "main%20rm", wantErr: true}, // % is invalid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			}
		})
	}
}
