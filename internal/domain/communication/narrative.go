package communication

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
)

// Narrative represents audience-specific release communication.
type Narrative struct {
	// Audience identifies which audience this narrative targets.
	Audience AudienceType
	// Title is the narrative heading.
	Title string
	// Body is the main narrative content.
	Body string
	// Format is the output format (markdown, plaintext, html).
	Format OutputFormat
	// GeneratedAt is when the narrative was created.
	GeneratedAt time.Time
	// Provider indicates how the narrative was generated (ai or template).
	Provider string
	// Model is the AI model used, if applicable.
	Model string
}

// NarrativeInput holds everything needed to generate a narrative.
type NarrativeInput struct {
	// Version is the release version string (e.g., "1.2.3").
	Version string
	// ProductName is the product name for branding.
	ProductName string
	// Bundles contains changes grouped into thematic bundles.
	Bundles []Bundle
	// ChangeSet is the raw changeset, used for template fallback.
	ChangeSet *changes.ChangeSet
	// Format is the desired output format.
	Format OutputFormat
	// PreviousVersion is the previous release version, if known.
	PreviousVersion string
}

// AICompleter is the interface the narrative generator requires from AI.
// This is a focused interface to avoid importing the full AI service.
type AICompleter interface {
	// Complete generates a raw completion from system and user prompts.
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	// IsAvailable returns true if the AI service is ready.
	IsAvailable() bool
}

// AIStructuredCompleter is an opt-in extension implemented by AI services
// supporting provider-native structured output. When the wired completer
// satisfies this interface, the generator gets a typed AudienceNarrative
// payload (headline + body + optional CTA) instead of free-form prose.
//
// schemaProvider is the minimal contract a JSON schema must satisfy; it
// matches `internal/infrastructure/ai/schemas.Schema` structurally without
// importing that package (avoids cross-cutting domain → infrastructure deps).
type AIStructuredCompleter interface {
	CompleteStructured(ctx context.Context, systemPrompt, userPrompt string, schema schemaProvider) ([]byte, error)
}

// schemaProvider is the structural contract a JSON Schema must satisfy.
// Concrete `schemas.Schema` values from the AI infrastructure package
// satisfy this implicitly.
type schemaProvider interface {
	json.Marshaler
	Name() string
	Description() string
	Strict() bool
}

// NarrativeGenerator generates audience-specific narratives from changes.
type NarrativeGenerator struct {
	ai AICompleter
}

// NewNarrativeGenerator creates a new narrative generator.
// ai may be nil; in that case template-based generation is used.
func NewNarrativeGenerator(ai AICompleter) *NarrativeGenerator {
	return &NarrativeGenerator{ai: ai}
}

// GenerateNarrative produces a narrative for the given audience.
func (g *NarrativeGenerator) GenerateNarrative(ctx context.Context, input NarrativeInput, audience Audience) (*Narrative, error) {
	if err := audience.Validate(); err != nil {
		return nil, fmt.Errorf("invalid audience: %w", err)
	}

	format := input.Format
	if format == "" {
		format = OutputMarkdown
	}

	// Try AI-powered generation first
	if g.ai != nil && g.ai.IsAvailable() {
		narrative, err := g.generateWithAI(ctx, input, audience, format)
		if err == nil {
			return narrative, nil
		}
		// Fall through to template-based generation on AI failure
	}

	// Template-based fallback
	return g.generateWithTemplate(input, audience, format)
}

// GenerateAll produces narratives for all provided audiences.
func (g *NarrativeGenerator) GenerateAll(ctx context.Context, input NarrativeInput, audiences []Audience) ([]*Narrative, error) {
	results := make([]*Narrative, 0, len(audiences))
	for _, aud := range audiences {
		narrative, err := g.GenerateNarrative(ctx, input, aud)
		if err != nil {
			return nil, fmt.Errorf("failed to generate narrative for audience %q: %w", aud.Type, err)
		}
		results = append(results, narrative)
	}
	return results, nil
}

// generateWithAI uses the AI provider to create an audience-specific narrative.
//
// Prefers provider-native structured output when the configured AI service
// supports it: returns a typed (headline, body, call_to_action) payload via
// the AudienceNarrative schema. Falls back to free-form Complete otherwise.
func (g *NarrativeGenerator) generateWithAI(ctx context.Context, input NarrativeInput, audience Audience, format OutputFormat) (*Narrative, error) {
	systemPrompt := buildAudienceSystemPrompt(audience, format)
	userPrompt := buildAudienceUserPrompt(input, audience)

	if structured, ok := g.ai.(AIStructuredCompleter); ok {
		schema := audienceNarrativeSchema()
		bytes, err := structured.CompleteStructured(ctx, systemPrompt, userPrompt, schema)
		if err == nil && len(bytes) > 0 {
			n, parseErr := buildNarrativeFromStructured(bytes, input, audience, format)
			if parseErr == nil {
				return n, nil
			}
			// Parse failure → fall through to free-form path so we still
			// emit *something* rather than blocking the release narrative.
		}
	}

	body, err := g.ai.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed for audience %q: %w", audience.Type, err)
	}

	return &Narrative{
		Audience:    audience.Type,
		Title:       buildTitle(input, audience),
		Body:        body,
		Format:      format,
		GeneratedAt: time.Now(),
		Provider:    "ai",
	}, nil
}

// buildNarrativeFromStructured parses an AudienceNarrative JSON payload and
// renders it into a Narrative. Body composition: headline → body → CTA.
func buildNarrativeFromStructured(bytes []byte, input NarrativeInput, audience Audience, format OutputFormat) (*Narrative, error) {
	var payload struct {
		Audience     string `json:"audience"`
		Headline     string `json:"headline"`
		Body         string `json:"body"`
		CallToAction string `json:"call_to_action,omitempty"`
	}
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return nil, fmt.Errorf("decode AudienceNarrative: %w", err)
	}
	if payload.Body == "" {
		return nil, fmt.Errorf("AudienceNarrative payload has empty body")
	}

	var sb strings.Builder
	if payload.Headline != "" && payload.Headline != buildTitle(input, audience) {
		sb.WriteString(payload.Headline)
		sb.WriteString("\n\n")
	}
	sb.WriteString(payload.Body)
	if payload.CallToAction != "" {
		sb.WriteString("\n\n")
		sb.WriteString(payload.CallToAction)
	}

	title := payload.Headline
	if title == "" {
		title = buildTitle(input, audience)
	}

	return &Narrative{
		Audience:    audience.Type,
		Title:       title,
		Body:        sb.String(),
		Format:      format,
		GeneratedAt: time.Now(),
		Provider:    "ai-structured",
	}, nil
}

// audienceNarrativeSchema is a local schema definition that matches
// `infrastructure/ai/schemas.AudienceNarrativeSchema()` structurally.
// Defined inline here to keep the domain layer free of infrastructure imports.
type narrativeSchema struct{}

func (narrativeSchema) Name() string        { return "AudienceNarrative" }
func (narrativeSchema) Description() string { return "Audience-tailored release narrative." }
func (narrativeSchema) Strict() bool        { return false }
func (narrativeSchema) MarshalJSON() ([]byte, error) {
	return []byte(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["audience", "headline", "body"],
		"properties": {
			"audience": {"type": "string", "enum": ["engineering", "product", "executive", "external"]},
			"headline": {"type": "string"},
			"body":     {"type": "string"},
			"call_to_action": {"type": "string", "description": "Optional next-step CTA."}
		}
	}`), nil
}

func audienceNarrativeSchema() schemaProvider { return narrativeSchema{} }

// generateWithTemplate uses Go templates to create a narrative without AI.
func (g *NarrativeGenerator) generateWithTemplate(input NarrativeInput, audience Audience, format OutputFormat) (*Narrative, error) {
	var sb strings.Builder

	title := buildTitle(input, audience)

	switch format {
	case OutputMarkdown:
		sb.WriteString("# ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	case OutputHTML:
		sb.WriteString("<h1>")
		sb.WriteString(title)
		sb.WriteString("</h1>\n\n")
	default:
		sb.WriteString(title)
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("=", len(title)))
		sb.WriteString("\n\n")
	}

	// Write sections based on audience configuration
	for _, section := range audience.Sections {
		content := renderSection(input, section, audience, format)
		if content != "" {
			sb.WriteString(content)
			sb.WriteString("\n\n")
		}
	}

	return &Narrative{
		Audience:    audience.Type,
		Title:       title,
		Body:        strings.TrimRight(sb.String(), "\n") + "\n",
		Format:      format,
		GeneratedAt: time.Now(),
		Provider:    "template",
	}, nil
}

// buildTitle creates a narrative title for the audience.
func buildTitle(input NarrativeInput, audience Audience) string {
	product := input.ProductName
	if product == "" {
		product = "Release"
	}

	switch audience.Type {
	case AudienceEngineering:
		return fmt.Sprintf("%s v%s - Engineering Release Notes", product, input.Version)
	case AudienceProduct:
		return fmt.Sprintf("%s v%s - Product Update", product, input.Version)
	case AudienceExecutive:
		return fmt.Sprintf("%s v%s - Executive Summary", product, input.Version)
	case AudienceExternal:
		return fmt.Sprintf("%s v%s - What's New", product, input.Version)
	default:
		return fmt.Sprintf("%s v%s - Release Notes", product, input.Version)
	}
}

// renderSection renders a single section for the template-based fallback.
func renderSection(input NarrativeInput, section Section, audience Audience, format OutputFormat) string {
	var sb strings.Builder
	bundles := input.Bundles

	writeSectionHeader := func(title string) {
		switch format {
		case OutputMarkdown:
			sb.WriteString("## ")
			sb.WriteString(title)
			sb.WriteString("\n\n")
		case OutputHTML:
			sb.WriteString("<h2>")
			sb.WriteString(title)
			sb.WriteString("</h2>\n\n")
		default:
			sb.WriteString(title)
			sb.WriteString("\n")
			sb.WriteString(strings.Repeat("-", len(title)))
			sb.WriteString("\n\n")
		}
	}

	writeBullet := func(text string) {
		switch format {
		case OutputMarkdown:
			sb.WriteString("- ")
			sb.WriteString(text)
			sb.WriteString("\n")
		case OutputHTML:
			sb.WriteString("<li>")
			sb.WriteString(text)
			sb.WriteString("</li>\n")
		default:
			sb.WriteString("* ")
			sb.WriteString(text)
			sb.WriteString("\n")
		}
	}

	switch section {
	case SectionBreakingChanges:
		items := collectBundleItems(bundles, BundleTypeBreaking)
		if len(items) == 0 {
			return ""
		}
		writeSectionHeader("Breaking Changes")
		for _, item := range items {
			writeBullet(item)
		}

	case SectionFeatures:
		items := collectBundleItems(bundles, BundleTypeFeature)
		if len(items) == 0 {
			return ""
		}
		switch audience.Type {
		case AudienceProduct:
			writeSectionHeader("Feature Highlights")
		case AudienceExternal:
			writeSectionHeader("New Features")
		default:
			writeSectionHeader("Features")
		}
		for _, item := range items {
			writeBullet(item)
		}

	case SectionFixes:
		items := collectBundleItems(bundles, BundleTypeBugfix)
		if len(items) == 0 {
			return ""
		}
		writeSectionHeader("Bug Fixes")
		for _, item := range items {
			writeBullet(item)
		}

	case SectionPerformance:
		items := collectBundleItems(bundles, BundleTypePerformance)
		if len(items) == 0 {
			return ""
		}
		writeSectionHeader("Performance Improvements")
		for _, item := range items {
			writeBullet(item)
		}

	case SectionSecurity:
		items := collectBundleItems(bundles, BundleTypeSecurity)
		if len(items) == 0 {
			return ""
		}
		writeSectionHeader("Security")
		for _, item := range items {
			writeBullet(item)
		}

	case SectionMigration:
		breaking := collectBundleItems(bundles, BundleTypeBreaking)
		if len(breaking) == 0 {
			return ""
		}
		writeSectionHeader("Migration Guide")
		sb.WriteString("The following breaking changes require action:\n\n")
		for _, item := range breaking {
			writeBullet(item)
		}

	case SectionMetrics:
		writeSectionHeader("Release Metrics")
		totalChanges := 0
		for _, b := range bundles {
			totalChanges += len(b.Changes)
		}
		writeBullet(fmt.Sprintf("Total changes: %d", totalChanges))
		writeBullet(fmt.Sprintf("Bundles: %d", len(bundles)))
		if input.PreviousVersion != "" {
			writeBullet(fmt.Sprintf("Previous version: %s", input.PreviousVersion))
		}

	case SectionRiskAssessment:
		breaking := collectBundleItems(bundles, BundleTypeBreaking)
		writeSectionHeader("Risk Assessment")
		if len(breaking) > 0 {
			writeBullet(fmt.Sprintf("Breaking changes: %d (requires customer communication)", len(breaking)))
		} else {
			writeBullet("No breaking changes")
		}

	case SectionBusinessValue:
		features := collectBundleItems(bundles, BundleTypeFeature)
		if len(features) == 0 {
			return ""
		}
		writeSectionHeader("Business Value")
		for _, item := range features {
			writeBullet(item)
		}

	case SectionUserImpact:
		features := collectBundleItems(bundles, BundleTypeFeature)
		fixes := collectBundleItems(bundles, BundleTypeBugfix)
		if len(features) == 0 && len(fixes) == 0 {
			return ""
		}
		writeSectionHeader("User Impact")
		for _, item := range features {
			writeBullet(item)
		}
		for _, item := range fixes {
			writeBullet(item)
		}

	case SectionUpgradeGuide:
		breaking := collectBundleItems(bundles, BundleTypeBreaking)
		if len(breaking) == 0 {
			writeSectionHeader("Upgrade Guide")
			writeBullet("No breaking changes. Standard upgrade applies.")
			return sb.String()
		}
		writeSectionHeader("Upgrade Guide")
		sb.WriteString("Please review the following changes before upgrading:\n\n")
		for _, item := range breaking {
			writeBullet(item)
		}

	case SectionContributors:
		authors := collectAuthors(bundles)
		if len(authors) == 0 {
			return ""
		}
		writeSectionHeader("Contributors")
		for _, author := range authors {
			writeBullet(author)
		}

	case SectionDocumentation:
		items := collectBundleItems(bundles, BundleTypeDocs)
		if len(items) == 0 {
			return ""
		}
		writeSectionHeader("Documentation")
		for _, item := range items {
			writeBullet(item)
		}

	case SectionStrategicAlign:
		writeSectionHeader("Strategic Alignment")
		features := collectBundleItems(bundles, BundleTypeFeature)
		writeBullet(fmt.Sprintf("New capabilities: %d", len(features)))

	default:
		return ""
	}

	return sb.String()
}

// collectBundleItems collects human-readable descriptions from bundles of the given type.
func collectBundleItems(bundles []Bundle, bundleType BundleType) []string {
	var items []string
	for _, b := range bundles {
		if b.Type == bundleType {
			if b.Summary != "" {
				items = append(items, b.Summary)
			} else {
				for _, c := range b.Changes {
					items = append(items, c.Description)
				}
			}
		}
	}
	return items
}

// collectAuthors collects unique author names from bundles.
func collectAuthors(bundles []Bundle) []string {
	seen := make(map[string]struct{})
	var authors []string
	for _, b := range bundles {
		for _, c := range b.Changes {
			if c.Author != "" {
				if _, ok := seen[c.Author]; !ok {
					seen[c.Author] = struct{}{}
					authors = append(authors, c.Author)
				}
			}
		}
	}
	return authors
}
