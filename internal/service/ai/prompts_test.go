// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
	"github.com/felixgeelhaar/release-pilot/internal/service/version"
)

func TestBuildSystemPrompt_AdditionalCases(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		opts        GenerateOptions
		contains    []string
		notContains []string
	}{
		{
			name:     "basic template",
			template: "You are a technical writer.",
			opts:     GenerateOptions{},
			contains: []string{"You are a technical writer", "Do not include emojis"},
		},
		{
			name:     "technical tone",
			template: "Write content.",
			opts:     GenerateOptions{Tone: ToneTechnical},
			contains: []string{"technical", "developer-focused"},
		},
		{
			name:     "friendly tone",
			template: "Write content.",
			opts:     GenerateOptions{Tone: ToneFriendly},
			contains: []string{"friendly", "casual"},
		},
		{
			name:     "professional tone",
			template: "Write content.",
			opts:     GenerateOptions{Tone: ToneProfessional},
			contains: []string{"professional", "business-like"},
		},
		{
			name:     "excited tone",
			template: "Write content.",
			opts:     GenerateOptions{Tone: ToneExcited},
			contains: []string{"enthusiastic", "excited"},
		},
		{
			name:     "developers audience",
			template: "Write content.",
			opts:     GenerateOptions{Audience: AudienceDevelopers},
			contains: []string{"developers", "technical details"},
		},
		{
			name:     "users audience",
			template: "Write content.",
			opts:     GenerateOptions{Audience: AudienceUsers},
			contains: []string{"end users", "user-facing"},
		},
		{
			name:     "public audience",
			template: "Write content.",
			opts:     GenerateOptions{Audience: AudiencePublic},
			contains: []string{"general public", "simple"},
		},
		{
			name:     "marketing audience",
			template: "Write content.",
			opts:     GenerateOptions{Audience: AudienceMarketing},
			contains: []string{"marketing", "value propositions"},
		},
		{
			name:     "with language",
			template: "Write content.",
			opts:     GenerateOptions{Language: "German"},
			contains: []string{"Write the output in German"},
		},
		{
			name:        "english language not mentioned",
			template:    "Write content.",
			opts:        GenerateOptions{Language: "English"},
			notContains: []string{"Write the output in English"},
		},
		{
			name:        "with emoji",
			template:    "Write content.",
			opts:        GenerateOptions{IncludeEmoji: true},
			contains:    []string{"Include relevant emojis"},
			notContains: []string{"Do not include emojis"},
		},
		{
			name:        "without emoji",
			template:    "Write content.",
			opts:        GenerateOptions{IncludeEmoji: false},
			contains:    []string{"Do not include emojis"},
			notContains: []string{"Include relevant emojis"},
		},
		{
			name:     "with max length",
			template: "Write content.",
			opts:     GenerateOptions{MaxLength: 500},
			contains: []string{"Keep the output under 500 characters"},
		},
		{
			name:        "without max length",
			template:    "Write content.",
			opts:        GenerateOptions{MaxLength: 0},
			notContains: []string{"Keep the output under"},
		},
		{
			name:     "combined options",
			template: "Write content.",
			opts: GenerateOptions{
				Tone:         ToneFriendly,
				Audience:     AudienceDevelopers,
				Language:     "Spanish",
				IncludeEmoji: true,
				MaxLength:    1000,
			},
			contains: []string{
				"friendly",
				"developers",
				"Spanish",
				"Include relevant emojis",
				"Keep the output under 1000 characters",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSystemPrompt(tt.template, tt.opts)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("buildSystemPrompt() missing expected content: %q\nGot: %s", expected, result)
				}
			}

			for _, unexpected := range tt.notContains {
				if strings.Contains(result, unexpected) {
					t.Errorf("buildSystemPrompt() contains unexpected content: %q\nGot: %s", unexpected, result)
				}
			}
		})
	}
}

func TestGetToneInstruction(t *testing.T) {
	tests := []struct {
		name     string
		tone     Tone
		contains string
	}{
		{"technical", ToneTechnical, "technical"},
		{"friendly", ToneFriendly, "friendly"},
		{"professional", ToneProfessional, "professional"},
		{"excited", ToneExcited, "enthusiastic"},
		{"default", Tone("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getToneInstruction(tt.tone)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("getToneInstruction(%v) should contain %q, got %q", tt.tone, tt.contains, result)
			}
			if tt.contains == "" && result != "" {
				t.Errorf("getToneInstruction(%v) should return empty string, got %q", tt.tone, result)
			}
		})
	}
}

func TestGetAudienceInstruction(t *testing.T) {
	tests := []struct {
		name     string
		audience Audience
		contains string
	}{
		{"developers", AudienceDevelopers, "developers"},
		{"users", AudienceUsers, "end users"},
		{"public", AudiencePublic, "general public"},
		{"marketing", AudienceMarketing, "marketing"},
		{"default", Audience("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAudienceInstruction(tt.audience)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("getAudienceInstruction(%v) should contain %q, got %q", tt.audience, tt.contains, result)
			}
			if tt.contains == "" && result != "" {
				t.Errorf("getAudienceInstruction(%v) should return empty string, got %q", tt.audience, result)
			}
		})
	}
}

func TestBuildUserPrompt_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		content  string
		opts     GenerateOptions
		contains []string
	}{
		{
			name:     "basic replacement",
			template: "Generate changelog for {{CONTENT}}",
			content:  "commit messages here",
			opts:     GenerateOptions{},
			contains: []string{"Generate changelog for commit messages here"},
		},
		{
			name:     "with product name",
			template: "Generate changelog for {{PRODUCT_NAME}} version {{VERSION}}",
			content:  "changes",
			opts:     GenerateOptions{ProductName: "MyApp"},
			contains: []string{"MyApp"},
		},
		{
			name:     "without product name",
			template: "Generate changelog for {{PRODUCT_NAME}}",
			content:  "changes",
			opts:     GenerateOptions{},
			contains: []string{"the project"},
		},
		{
			name:     "with context",
			template: "Generate content: {{CONTENT}}",
			content:  "changes",
			opts:     GenerateOptions{Context: "This is a major release with breaking changes"},
			contains: []string{"Additional context: This is a major release with breaking changes"},
		},
		{
			name:     "with version",
			template: "Generate notes for version {{VERSION}}",
			content:  "changes",
			opts: GenerateOptions{
				Version: func() *version.Version { v, _ := version.Parse("1.2.3"); return v }(),
			},
			contains: []string{"1.2.3"},
		},
		{
			name:     "without version leaves placeholder empty",
			template: "Version {{VERSION}} changelog",
			content:  "changes",
			opts:     GenerateOptions{},
			contains: []string{"Version  changelog"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildUserPrompt(tt.template, tt.content, tt.opts)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("buildUserPrompt() missing expected content: %q\nGot: %s", expected, result)
				}
			}
		})
	}
}

func TestFormatChangesForPrompt(t *testing.T) {
	tests := []struct {
		name     string
		changes  *git.CategorizedChanges
		contains []string
	}{
		{
			name: "breaking changes",
			changes: &git.CategorizedChanges{
				Breaking: []git.ConventionalCommit{
					{Type: "feat", Scope: "api", Description: "remove deprecated endpoint", Breaking: true},
				},
			},
			contains: []string{"BREAKING CHANGES:", "remove deprecated endpoint", "(api)"},
		},
		{
			name: "features",
			changes: &git.CategorizedChanges{
				Features: []git.ConventionalCommit{
					{Type: "feat", Scope: "auth", Description: "add OAuth support"},
					{Type: "feat", Description: "add dark mode"},
				},
			},
			contains: []string{"NEW FEATURES:", "add OAuth support", "(auth)", "add dark mode"},
		},
		{
			name: "bug fixes",
			changes: &git.CategorizedChanges{
				Fixes: []git.ConventionalCommit{
					{Type: "fix", Scope: "ui", Description: "button alignment"},
				},
			},
			contains: []string{"BUG FIXES:", "button alignment", "(ui)"},
		},
		{
			name: "performance improvements",
			changes: &git.CategorizedChanges{
				Performance: []git.ConventionalCommit{
					{Type: "perf", Description: "optimize database queries"},
				},
			},
			contains: []string{"PERFORMANCE IMPROVEMENTS:", "optimize database queries"},
		},
		{
			name: "documentation",
			changes: &git.CategorizedChanges{
				Documentation: []git.ConventionalCommit{
					{Type: "docs", Description: "update API documentation"},
				},
			},
			contains: []string{"DOCUMENTATION:", "update API documentation"},
		},
		{
			name: "other changes",
			changes: &git.CategorizedChanges{
				Other: []git.ConventionalCommit{
					{Type: "chore", Description: "update dependencies"},
				},
			},
			contains: []string{"OTHER CHANGES:", "update dependencies"},
		},
		{
			name: "all categories",
			changes: &git.CategorizedChanges{
				Breaking: []git.ConventionalCommit{
					{Type: "feat", Description: "breaking change", Breaking: true},
				},
				Features: []git.ConventionalCommit{
					{Type: "feat", Description: "new feature"},
				},
				Fixes: []git.ConventionalCommit{
					{Type: "fix", Description: "bug fix"},
				},
				Performance: []git.ConventionalCommit{
					{Type: "perf", Description: "performance"},
				},
				Documentation: []git.ConventionalCommit{
					{Type: "docs", Description: "documentation"},
				},
				Other: []git.ConventionalCommit{
					{Type: "chore", Description: "other"},
				},
			},
			contains: []string{
				"BREAKING CHANGES:", "breaking change",
				"NEW FEATURES:", "new feature",
				"BUG FIXES:", "bug fix",
				"PERFORMANCE IMPROVEMENTS:", "performance",
				"DOCUMENTATION:", "documentation",
				"OTHER CHANGES:", "other",
			},
		},
		{
			name: "features with breaking excluded from new features",
			changes: &git.CategorizedChanges{
				Features: []git.ConventionalCommit{
					{Type: "feat", Description: "regular feature", Breaking: false},
					{Type: "feat", Description: "breaking feature", Breaking: true},
				},
			},
			contains: []string{"NEW FEATURES:", "regular feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatChangesForPrompt(tt.changes)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("formatChangesForPrompt() missing expected content: %q\nGot: %s", expected, result)
				}
			}
		})
	}
}

func TestIsConnectionError_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", &testError{"connection refused"}, true},
		{"no such host", &testError{"no such host"}, true},
		{"network unreachable", &testError{"network is unreachable"}, true},
		{"connection reset", &testError{"connection reset"}, true},
		{"i/o timeout", &testError{"i/o timeout"}, true},
		{"other error", &testError{"something else failed"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			if result != tt.expected {
				t.Errorf("isConnectionError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestNewDefaultPromptTemplates(t *testing.T) {
	templates := newDefaultPromptTemplates()

	if templates.changelogSystem == "" {
		t.Error("changelogSystem should not be empty")
	}
	if templates.changelogUser == "" {
		t.Error("changelogUser should not be empty")
	}
	if templates.releaseNotesSystem == "" {
		t.Error("releaseNotesSystem should not be empty")
	}
	if templates.releaseNotesUser == "" {
		t.Error("releaseNotesUser should not be empty")
	}
	if templates.marketingSystem == "" {
		t.Error("marketingSystem should not be empty")
	}
	if templates.marketingUser == "" {
		t.Error("marketingUser should not be empty")
	}
	if templates.summarySystem == "" {
		t.Error("summarySystem should not be empty")
	}
	if templates.summaryUser == "" {
		t.Error("summaryUser should not be empty")
	}
}

func TestApplyCustomPrompts(t *testing.T) {
	templates := newDefaultPromptTemplates()
	original := templates

	custom := CustomPrompts{
		ChangelogSystem:    "Custom changelog system",
		ChangelogUser:      "Custom changelog user",
		ReleaseNotesSystem: "Custom release notes system",
		ReleaseNotesUser:   "Custom release notes user",
		MarketingSystem:    "Custom marketing system",
		MarketingUser:      "Custom marketing user",
	}

	templates.applyCustomPrompts(custom)

	if templates.changelogSystem != "Custom changelog system" {
		t.Errorf("changelogSystem = %v, want Custom changelog system", templates.changelogSystem)
	}
	if templates.changelogUser != "Custom changelog user" {
		t.Errorf("changelogUser = %v, want Custom changelog user", templates.changelogUser)
	}
	if templates.releaseNotesSystem != "Custom release notes system" {
		t.Errorf("releaseNotesSystem = %v, want Custom release notes system", templates.releaseNotesSystem)
	}
	if templates.releaseNotesUser != "Custom release notes user" {
		t.Errorf("releaseNotesUser = %v, want Custom release notes user", templates.releaseNotesUser)
	}
	if templates.marketingSystem != "Custom marketing system" {
		t.Errorf("marketingSystem = %v, want Custom marketing system", templates.marketingSystem)
	}
	if templates.marketingUser != "Custom marketing user" {
		t.Errorf("marketingUser = %v, want Custom marketing user", templates.marketingUser)
	}

	// Summary prompts should remain unchanged
	if templates.summarySystem != original.summarySystem {
		t.Error("summarySystem should not change when not provided in custom prompts")
	}
	if templates.summaryUser != original.summaryUser {
		t.Error("summaryUser should not change when not provided in custom prompts")
	}
}

func TestApplyCustomPrompts_Partial(t *testing.T) {
	templates := newDefaultPromptTemplates()
	original := templates

	// Apply only some custom prompts
	custom := CustomPrompts{
		ChangelogSystem: "Custom changelog system only",
	}

	templates.applyCustomPrompts(custom)

	if templates.changelogSystem != "Custom changelog system only" {
		t.Errorf("changelogSystem = %v, want Custom changelog system only", templates.changelogSystem)
	}

	// All others should remain unchanged
	if templates.changelogUser != original.changelogUser {
		t.Error("changelogUser should not change")
	}
	if templates.releaseNotesSystem != original.releaseNotesSystem {
		t.Error("releaseNotesSystem should not change")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
