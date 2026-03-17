package communication

import (
	"fmt"
	"strings"
)

// buildAudienceSystemPrompt constructs the system prompt for AI narrative generation.
func buildAudienceSystemPrompt(audience Audience, format OutputFormat) string {
	// Use custom prompt if provided
	if audience.CustomPrompt != "" {
		return audience.CustomPrompt
	}

	var sb strings.Builder
	sb.Grow(1024)

	// Base role instruction
	sb.WriteString("You are a release communication specialist. ")
	sb.WriteString("Your task is to transform software release changes into clear, audience-appropriate narratives.\n\n")

	// Audience-specific instructions
	switch audience.Type {
	case AudienceEngineering:
		sb.WriteString("TARGET AUDIENCE: Software engineers and developers.\n")
		sb.WriteString("TONE: Technical, precise, and detailed.\n")
		sb.WriteString("FOCUS: Include technical details, breaking changes with migration steps, API changes, ")
		sb.WriteString("performance metrics, and implementation notes.\n")
		sb.WriteString("DETAIL LEVEL: Full technical detail. Include code references and specific change descriptions.\n")
		sb.WriteString("Use precise technical language. Developers need to understand exactly what changed and why.\n")

	case AudienceProduct:
		sb.WriteString("TARGET AUDIENCE: Product managers and designers.\n")
		sb.WriteString("TONE: Business-oriented, clear, and impact-focused.\n")
		sb.WriteString("FOCUS: Feature highlights, user impact, business value, and customer-facing improvements.\n")
		sb.WriteString("DETAIL LEVEL: Summary of key changes. Avoid implementation details.\n")
		sb.WriteString("Frame changes in terms of user benefits and business outcomes.\n")

	case AudienceExecutive:
		sb.WriteString("TARGET AUDIENCE: C-level executives and leadership.\n")
		sb.WriteString("TONE: Executive, concise, and strategic.\n")
		sb.WriteString("FOCUS: Summary metrics, risk assessment, strategic alignment, and high-level impact.\n")
		sb.WriteString("DETAIL LEVEL: Highlights only. Keep it brief and decision-oriented.\n")
		sb.WriteString("Use business language. Emphasize risk, ROI, and strategic value.\n")

	case AudienceExternal:
		sb.WriteString("TARGET AUDIENCE: End users and the public.\n")
		sb.WriteString("TONE: Friendly, accessible, and user-focused.\n")
		sb.WriteString("FOCUS: User-facing features, bug fixes, upgrade instructions.\n")
		sb.WriteString("DETAIL LEVEL: Summary of visible changes. No internal implementation details.\n")
		sb.WriteString("Use simple, clear language. Focus on what users can now do differently.\n")
	}

	// Detail level instruction
	sb.WriteString("\nDETAIL LEVEL GUIDANCE:\n")
	switch audience.DetailLevel {
	case DetailFull:
		sb.WriteString("Provide comprehensive detail for each change.\n")
	case DetailSummary:
		sb.WriteString("Provide a concise summary. Group related changes together.\n")
	case DetailHighlights:
		sb.WriteString("Provide only the key highlights. Be extremely concise.\n")
	}

	// Sections instruction
	sb.WriteString("\nINCLUDE THESE SECTIONS:\n")
	for _, s := range audience.Sections {
		sb.WriteString("- ")
		sb.WriteString(sectionDisplayName(s))
		sb.WriteString("\n")
	}

	// Format instruction
	sb.WriteString("\nOUTPUT FORMAT: ")
	switch format {
	case OutputMarkdown:
		sb.WriteString("Markdown with proper headings (##), bullet points, and formatting.\n")
	case OutputHTML:
		sb.WriteString("Clean HTML with semantic tags (h2, ul, li, p).\n")
	case OutputPlainText:
		sb.WriteString("Plain text with clear section headers and bullet points using * or -.\n")
	default:
		sb.WriteString("Markdown.\n")
	}

	sb.WriteString("\nDo not include a top-level title/heading - that will be added separately.")

	return sb.String()
}

// buildAudienceUserPrompt constructs the user prompt with change details.
func buildAudienceUserPrompt(input NarrativeInput, audience Audience) string {
	var sb strings.Builder
	sb.Grow(2048)

	sb.WriteString("Generate release communication for ")
	if input.ProductName != "" {
		sb.WriteString(input.ProductName)
	} else {
		sb.WriteString("this project")
	}
	sb.WriteString(" version ")
	sb.WriteString(input.Version)

	if input.PreviousVersion != "" {
		sb.WriteString(" (previous: ")
		sb.WriteString(input.PreviousVersion)
		sb.WriteString(")")
	}
	sb.WriteString(".\n\n")

	// Include bundled changes
	if len(input.Bundles) > 0 {
		sb.WriteString("CHANGES:\n\n")
		for _, bundle := range input.Bundles {
			sb.WriteString("## ")
			sb.WriteString(string(bundle.Type))
			if bundle.Theme != "" {
				sb.WriteString(" - ")
				sb.WriteString(bundle.Theme)
			}
			sb.WriteString("\n")

			if bundle.Summary != "" {
				sb.WriteString(bundle.Summary)
				sb.WriteString("\n")
			}

			for _, c := range bundle.Changes {
				sb.WriteString("- ")
				sb.WriteString(c.Description)
				if c.Scope != "" {
					sb.WriteString(" (")
					sb.WriteString(c.Scope)
					sb.WriteString(")")
				}
				if c.Breaking {
					sb.WriteString(" [BREAKING]")
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// Fallback: include raw changeset data
	if len(input.Bundles) == 0 && input.ChangeSet != nil {
		sb.WriteString("COMMITS:\n\n")
		for _, c := range input.ChangeSet.Commits() {
			sb.WriteString("- ")
			sb.WriteString(c.String())
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// sectionDisplayName returns a human-readable name for a section.
func sectionDisplayName(s Section) string {
	switch s {
	case SectionBreakingChanges:
		return "Breaking Changes"
	case SectionFeatures:
		return "Features"
	case SectionFixes:
		return "Bug Fixes"
	case SectionPerformance:
		return "Performance Improvements"
	case SectionSecurity:
		return "Security"
	case SectionMigration:
		return "Migration Guide"
	case SectionMetrics:
		return "Release Metrics"
	case SectionRiskAssessment:
		return "Risk Assessment"
	case SectionBusinessValue:
		return "Business Value"
	case SectionUserImpact:
		return "User Impact"
	case SectionUpgradeGuide:
		return "Upgrade Guide"
	case SectionContributors:
		return "Contributors"
	case SectionDocumentation:
		return "Documentation"
	case SectionStrategicAlign:
		return "Strategic Alignment"
	default:
		return fmt.Sprintf("Unknown (%s)", s)
	}
}
