package communication

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// mockAICompleter implements AICompleter for testing.
type mockAICompleter struct {
	response  string
	err       error
	available bool
	calls     int
}

func (m *mockAICompleter) Complete(_ context.Context, _, _ string) (string, error) {
	m.calls++
	return m.response, m.err
}

func (m *mockAICompleter) IsAvailable() bool {
	return m.available
}

func TestNarrativeGenerator_GenerateNarrative_WithAI(t *testing.T) {
	mock := &mockAICompleter{
		response:  "## Features\n\n- Added widget support\n- Improved performance",
		available: true,
	}

	gen := NewNarrativeGenerator(mock)
	audience := DefaultAudiences()[AudienceEngineering]

	input := NarrativeInput{
		Version:     "1.2.0",
		ProductName: "TestApp",
		Bundles: []Bundle{
			{
				Type:  BundleTypeFeature,
				Theme: "Widget support",
				Changes: []BundledChange{
					{Description: "add widget rendering", Scope: "ui"},
				},
			},
		},
		Format: OutputMarkdown,
	}

	narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if narrative.Provider != "ai" {
		t.Errorf("provider = %q, want %q", narrative.Provider, "ai")
	}
	if narrative.Audience != AudienceEngineering {
		t.Errorf("audience = %q, want %q", narrative.Audience, AudienceEngineering)
	}
	if !strings.Contains(narrative.Body, "widget") {
		t.Error("narrative body should contain AI response content")
	}
	if mock.calls != 1 {
		t.Errorf("AI Complete called %d times, want 1", mock.calls)
	}
}

func TestNarrativeGenerator_GenerateNarrative_FallbackOnAIError(t *testing.T) {
	mock := &mockAICompleter{
		err:       errors.New("API rate limit exceeded"),
		available: true,
	}

	gen := NewNarrativeGenerator(mock)
	audience := DefaultAudiences()[AudienceEngineering]

	input := NarrativeInput{
		Version:     "1.0.0",
		ProductName: "TestApp",
		Bundles: []Bundle{
			{
				Type:    BundleTypeFeature,
				Theme:   "Auth",
				Summary: "Added OAuth support",
				Changes: []BundledChange{
					{Description: "implement OAuth2 flow", Scope: "auth"},
				},
			},
		},
		Format: OutputMarkdown,
	}

	narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if narrative.Provider != "template" {
		t.Errorf("provider = %q, want %q (should fallback)", narrative.Provider, "template")
	}
}

func TestNarrativeGenerator_GenerateNarrative_FallbackWhenNoAI(t *testing.T) {
	gen := NewNarrativeGenerator(nil)
	audience := DefaultAudiences()[AudienceProduct]

	input := NarrativeInput{
		Version:     "2.0.0",
		ProductName: "MyProduct",
		Bundles: []Bundle{
			{
				Type:    BundleTypeFeature,
				Theme:   "Dashboard",
				Summary: "New analytics dashboard",
				Changes: []BundledChange{
					{Description: "add analytics dashboard", Scope: "dashboard"},
				},
			},
			{
				Type:  BundleTypeBreaking,
				Theme: "API v2",
				Changes: []BundledChange{
					{Description: "rename /users endpoint to /accounts", Breaking: true},
				},
			},
		},
		Format: OutputMarkdown,
	}

	narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if narrative.Provider != "template" {
		t.Errorf("provider = %q, want %q", narrative.Provider, "template")
	}
	if narrative.Format != OutputMarkdown {
		t.Errorf("format = %q, want %q", narrative.Format, OutputMarkdown)
	}

	// Product audience should have feature highlights
	if !strings.Contains(narrative.Body, "Feature Highlights") {
		t.Error("product narrative should contain 'Feature Highlights' section")
	}
	// Should include breaking changes
	if !strings.Contains(narrative.Body, "Breaking Changes") {
		t.Error("product narrative should include breaking changes")
	}
}

func TestNarrativeGenerator_GenerateNarrative_InvalidAudience(t *testing.T) {
	gen := NewNarrativeGenerator(nil)

	input := NarrativeInput{Version: "1.0.0"}
	_, err := gen.GenerateNarrative(context.Background(), input, Audience{})
	if err == nil {
		t.Error("expected error for invalid audience")
	}
}

func TestNarrativeGenerator_GenerateAll(t *testing.T) {
	gen := NewNarrativeGenerator(nil)
	audiences := []Audience{
		DefaultAudiences()[AudienceEngineering],
		DefaultAudiences()[AudienceExternal],
	}

	input := NarrativeInput{
		Version: "1.0.0",
		Bundles: []Bundle{
			{
				Type:    BundleTypeFeature,
				Theme:   "Core",
				Summary: "Initial release",
				Changes: []BundledChange{
					{Description: "initial implementation"},
				},
			},
		},
	}

	narratives, err := gen.GenerateAll(context.Background(), input, audiences)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(narratives) != 2 {
		t.Fatalf("got %d narratives, want 2", len(narratives))
	}
	if narratives[0].Audience != AudienceEngineering {
		t.Errorf("first narrative audience = %q, want %q", narratives[0].Audience, AudienceEngineering)
	}
	if narratives[1].Audience != AudienceExternal {
		t.Errorf("second narrative audience = %q, want %q", narratives[1].Audience, AudienceExternal)
	}
}

func TestNarrativeGenerator_OutputFormats(t *testing.T) {
	gen := NewNarrativeGenerator(nil)
	audience := DefaultAudiences()[AudienceEngineering]

	input := NarrativeInput{
		Version:     "1.0.0",
		ProductName: "TestApp",
		Bundles: []Bundle{
			{
				Type:    BundleTypeFeature,
				Theme:   "Core",
				Summary: "New feature",
				Changes: []BundledChange{
					{Description: "add feature X"},
				},
			},
		},
	}

	tests := []struct {
		format   OutputFormat
		contains string
	}{
		{OutputMarkdown, "# "},
		{OutputHTML, "<h1>"},
		{OutputPlainText, "==="},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			input.Format = tt.format
			narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(narrative.Body, tt.contains) {
				t.Errorf("output should contain %q for format %q; got:\n%s", tt.contains, tt.format, narrative.Body)
			}
		})
	}
}

func TestNarrativeGenerator_TemplateWithChangeSet(t *testing.T) {
	gen := NewNarrativeGenerator(nil)
	audience := DefaultAudiences()[AudienceEngineering]

	// Create a changeset with real commits
	cs := changes.NewChangeSet("test-cs", "v0.9.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc1234", changes.CommitTypeFeat, "add search API",
		changes.WithScope("api"),
		changes.WithAuthor("Alice", "alice@example.com"),
	))
	cs.AddCommit(changes.NewConventionalCommit("def5678", changes.CommitTypeFix, "fix null pointer in handler",
		changes.WithScope("api"),
	))

	bundler := NewBundler()
	bundles := bundler.BundleChanges(cs)

	input := NarrativeInput{
		Version:     "1.0.0",
		ProductName: "TestApp",
		Bundles:     bundles,
		ChangeSet:   cs,
		Format:      OutputMarkdown,
	}

	narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(narrative.Body, "search API") {
		t.Error("narrative should contain commit description 'search API'")
	}
	if !strings.Contains(narrative.Body, "null pointer") {
		t.Error("narrative should contain commit description 'null pointer'")
	}
}

func TestNarrativeGenerator_ExecutiveAudience(t *testing.T) {
	gen := NewNarrativeGenerator(nil)
	audience := DefaultAudiences()[AudienceExecutive]

	input := NarrativeInput{
		Version:     "3.0.0",
		ProductName: "Enterprise Suite",
		Bundles: []Bundle{
			{
				Type:  BundleTypeBreaking,
				Theme: "API Migration",
				Changes: []BundledChange{
					{Description: "migrate to v3 API", Breaking: true},
				},
			},
			{
				Type:    BundleTypeFeature,
				Theme:   "Analytics",
				Summary: "Real-time analytics dashboard",
				Changes: []BundledChange{
					{Description: "add real-time analytics"},
				},
			},
		},
		PreviousVersion: "2.5.0",
		Format:          OutputMarkdown,
	}

	narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(narrative.Body, "Executive Summary") {
		t.Error("executive narrative title should contain 'Executive Summary'")
	}
	if !strings.Contains(narrative.Body, "Risk Assessment") {
		t.Error("executive narrative should contain risk assessment section")
	}
	if !strings.Contains(narrative.Body, "Metrics") {
		t.Error("executive narrative should contain metrics section")
	}
}

func TestNarrativeGenerator_ExternalAudience(t *testing.T) {
	gen := NewNarrativeGenerator(nil)
	audience := DefaultAudiences()[AudienceExternal]

	input := NarrativeInput{
		Version:     "2.1.0",
		ProductName: "MyApp",
		Bundles: []Bundle{
			{
				Type:    BundleTypeFeature,
				Theme:   "Search",
				Summary: "Full-text search",
				Changes: []BundledChange{
					{Description: "add full-text search"},
				},
			},
		},
		Format: OutputMarkdown,
	}

	narrative, err := gen.GenerateNarrative(context.Background(), input, audience)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(narrative.Title, "What's New") {
		t.Errorf("external title = %q, should contain \"What's New\"", narrative.Title)
	}
	// External should have upgrade guide section
	if !strings.Contains(narrative.Body, "Upgrade Guide") {
		t.Error("external narrative should contain upgrade guide")
	}
}
