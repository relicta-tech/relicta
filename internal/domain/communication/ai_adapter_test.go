package communication

import (
	"strings"
	"testing"
)

func TestBuildAudienceSystemPrompt_Engineering(t *testing.T) {
	audience := DefaultAudiences()[AudienceEngineering]
	prompt := buildAudienceSystemPrompt(audience, OutputMarkdown)

	expectations := []string{
		"engineers",
		"Technical",
		"breaking changes",
		"migration",
		"Markdown",
	}

	for _, exp := range expectations {
		if !strings.Contains(prompt, exp) {
			t.Errorf("engineering system prompt should contain %q", exp)
		}
	}
}

func TestBuildAudienceSystemPrompt_Product(t *testing.T) {
	audience := DefaultAudiences()[AudienceProduct]
	prompt := buildAudienceSystemPrompt(audience, OutputMarkdown)

	if !strings.Contains(prompt, "Product managers") {
		t.Error("product prompt should reference product managers")
	}
	if !strings.Contains(prompt, "business") || !strings.Contains(prompt, "impact") {
		t.Error("product prompt should mention business impact")
	}
}

func TestBuildAudienceSystemPrompt_Executive(t *testing.T) {
	audience := DefaultAudiences()[AudienceExecutive]
	prompt := buildAudienceSystemPrompt(audience, OutputMarkdown)

	if !strings.Contains(prompt, "executive") || !strings.Contains(prompt, "C-level") {
		t.Error("executive prompt should reference C-level executives")
	}
	if !strings.Contains(prompt, "strategic") {
		t.Error("executive prompt should mention strategic alignment")
	}
}

func TestBuildAudienceSystemPrompt_External(t *testing.T) {
	audience := DefaultAudiences()[AudienceExternal]
	prompt := buildAudienceSystemPrompt(audience, OutputMarkdown)

	if !strings.Contains(prompt, "End users") {
		t.Error("external prompt should reference end users")
	}
	if !strings.Contains(prompt, "simple") || !strings.Contains(prompt, "clear") {
		t.Error("external prompt should mention simple, clear language")
	}
}

func TestBuildAudienceSystemPrompt_CustomPrompt(t *testing.T) {
	audience := Audience{
		Type:         AudienceEngineering,
		Name:         "Custom",
		Tone:         CommToneTechnical,
		DetailLevel:  DetailFull,
		Sections:     []Section{SectionFeatures},
		CustomPrompt: "You are a custom prompt writer for engineers.",
	}

	prompt := buildAudienceSystemPrompt(audience, OutputMarkdown)
	if prompt != "You are a custom prompt writer for engineers." {
		t.Errorf("custom prompt should be used verbatim, got %q", prompt)
	}
}

func TestBuildAudienceSystemPrompt_OutputFormats(t *testing.T) {
	audience := DefaultAudiences()[AudienceEngineering]

	tests := []struct {
		format   OutputFormat
		contains string
	}{
		{OutputMarkdown, "Markdown"},
		{OutputHTML, "HTML"},
		{OutputPlainText, "Plain text"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			prompt := buildAudienceSystemPrompt(audience, tt.format)
			if !strings.Contains(prompt, tt.contains) {
				t.Errorf("prompt for format %q should contain %q", tt.format, tt.contains)
			}
		})
	}
}

func TestBuildAudienceSystemPrompt_DetailLevels(t *testing.T) {
	tests := []struct {
		level    DetailLevel
		contains string
	}{
		{DetailFull, "comprehensive"},
		{DetailSummary, "concise summary"},
		{DetailHighlights, "key highlights"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			audience := Audience{
				Type:        AudienceEngineering,
				Name:        "Test",
				Tone:        CommToneTechnical,
				DetailLevel: tt.level,
				Sections:    []Section{SectionFeatures},
			}
			prompt := buildAudienceSystemPrompt(audience, OutputMarkdown)
			if !strings.Contains(prompt, tt.contains) {
				t.Errorf("detail level %q prompt should contain %q", tt.level, tt.contains)
			}
		})
	}
}

func TestBuildAudienceUserPrompt(t *testing.T) {
	input := NarrativeInput{
		Version:         "2.0.0",
		ProductName:     "TestApp",
		PreviousVersion: "1.5.0",
		Bundles: []Bundle{
			{
				Type:  BundleTypeFeature,
				Theme: "Search",
				Changes: []BundledChange{
					{Description: "add full-text search", Scope: "api"},
					{Description: "remove legacy search", Breaking: true},
				},
			},
		},
	}

	audience := DefaultAudiences()[AudienceEngineering]
	prompt := buildAudienceUserPrompt(input, audience)

	expectations := []string{
		"TestApp",
		"2.0.0",
		"1.5.0",
		"add full-text search",
		"(api)",
		"[BREAKING]",
	}

	for _, exp := range expectations {
		if !strings.Contains(prompt, exp) {
			t.Errorf("user prompt should contain %q; got:\n%s", exp, prompt)
		}
	}
}

func TestBuildAudienceUserPrompt_NoProduct(t *testing.T) {
	input := NarrativeInput{
		Version: "1.0.0",
		Bundles: []Bundle{
			{
				Type:    BundleTypeFeature,
				Changes: []BundledChange{{Description: "test change"}},
			},
		},
	}

	audience := DefaultAudiences()[AudienceEngineering]
	prompt := buildAudienceUserPrompt(input, audience)

	if !strings.Contains(prompt, "this project") {
		t.Error("should use 'this project' when no product name is set")
	}
}

func TestSectionDisplayName(t *testing.T) {
	tests := []struct {
		section Section
		want    string
	}{
		{SectionBreakingChanges, "Breaking Changes"},
		{SectionFeatures, "Features"},
		{SectionFixes, "Bug Fixes"},
		{SectionPerformance, "Performance Improvements"},
		{SectionSecurity, "Security"},
		{SectionMigration, "Migration Guide"},
		{SectionMetrics, "Release Metrics"},
		{SectionRiskAssessment, "Risk Assessment"},
		{SectionBusinessValue, "Business Value"},
		{SectionUserImpact, "User Impact"},
		{SectionUpgradeGuide, "Upgrade Guide"},
		{SectionContributors, "Contributors"},
		{SectionDocumentation, "Documentation"},
		{SectionStrategicAlign, "Strategic Alignment"},
		{Section("unknown"), "Unknown (unknown)"},
	}

	for _, tt := range tests {
		t.Run(string(tt.section), func(t *testing.T) {
			got := sectionDisplayName(tt.section)
			if got != tt.want {
				t.Errorf("sectionDisplayName(%q) = %q, want %q", tt.section, got, tt.want)
			}
		})
	}
}
