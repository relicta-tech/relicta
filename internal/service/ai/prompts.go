// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"strconv"
	"strings"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// promptTemplates holds prompt templates for different generation tasks.
type promptTemplates struct {
	changelogSystem    string
	changelogUser      string
	releaseNotesSystem string
	releaseNotesUser   string
	marketingSystem    string
	marketingUser      string
	summarySystem      string
	summaryUser        string
}

// newDefaultPromptTemplates creates prompt templates with default values.
func newDefaultPromptTemplates() promptTemplates {
	return promptTemplates{
		changelogSystem:    defaultChangelogSystemPrompt,
		changelogUser:      defaultChangelogUserPrompt,
		releaseNotesSystem: defaultReleaseNotesSystemPrompt,
		releaseNotesUser:   defaultReleaseNotesUserPrompt,
		marketingSystem:    defaultMarketingSystemPrompt,
		marketingUser:      defaultMarketingUserPrompt,
		summarySystem:      defaultSummarySystemPrompt,
		summaryUser:        defaultSummaryUserPrompt,
	}
}

// applyCustomPrompts applies custom prompts from configuration.
func (p *promptTemplates) applyCustomPrompts(custom CustomPrompts) {
	if custom.ChangelogSystem != "" {
		p.changelogSystem = custom.ChangelogSystem
	}
	if custom.ChangelogUser != "" {
		p.changelogUser = custom.ChangelogUser
	}
	if custom.ReleaseNotesSystem != "" {
		p.releaseNotesSystem = custom.ReleaseNotesSystem
	}
	if custom.ReleaseNotesUser != "" {
		p.releaseNotesUser = custom.ReleaseNotesUser
	}
	if custom.MarketingSystem != "" {
		p.marketingSystem = custom.MarketingSystem
	}
	if custom.MarketingUser != "" {
		p.marketingUser = custom.MarketingUser
	}
}

// buildSystemPrompt builds the system prompt with options.
// This is shared across all AI service implementations.
func buildSystemPrompt(template string, opts GenerateOptions) string {
	// Pre-allocate capacity: template + estimated additions (~200 chars for tone/audience/etc)
	var b strings.Builder
	b.Grow(len(template) + 300)
	b.WriteString(template)

	// Add tone instructions
	b.WriteString(getToneInstruction(opts.Tone))

	// Add audience instructions
	b.WriteString(getAudienceInstruction(opts.Audience))

	// Add language instruction
	if opts.Language != "" && opts.Language != "English" {
		b.WriteString("\n\nWrite the output in ")
		b.WriteString(opts.Language)
		b.WriteByte('.')
	}

	// Add emoji instruction
	if opts.IncludeEmoji {
		b.WriteString("\n\nInclude relevant emojis to make the content more engaging.")
	} else {
		b.WriteString("\n\nDo not include emojis.")
	}

	// Add max length instruction
	if opts.MaxLength > 0 {
		b.WriteString("\n\nKeep the output under ")
		b.WriteString(strconv.Itoa(opts.MaxLength))
		b.WriteString(" characters.")
	}

	return b.String()
}

// getToneInstruction returns the instruction string for a given tone.
func getToneInstruction(tone Tone) string {
	switch tone {
	case ToneTechnical:
		return "\n\nUse a technical, developer-focused tone. Include technical details and be precise."
	case ToneFriendly:
		return "\n\nUse a friendly, casual tone. Be approachable and easy to understand."
	case ToneProfessional:
		return "\n\nUse a professional, business-like tone. Be clear and formal."
	case ToneExcited:
		return "\n\nUse an enthusiastic, excited tone. Highlight exciting improvements and new capabilities."
	default:
		return ""
	}
}

// getAudienceInstruction returns the instruction string for a given audience.
func getAudienceInstruction(audience Audience) string {
	switch audience {
	case AudienceDevelopers:
		return "\n\nTarget audience: software developers. Include technical details they care about."
	case AudienceUsers:
		return "\n\nTarget audience: end users. Focus on user-facing changes and benefits."
	case AudiencePublic:
		return "\n\nTarget audience: general public. Keep it simple and accessible."
	case AudienceMarketing:
		return "\n\nTarget audience: marketing teams. Emphasize value propositions and key highlights."
	default:
		return ""
	}
}

// buildUserPrompt builds the user prompt with content and options.
// This is shared across all AI service implementations.
func buildUserPrompt(template, content string, opts GenerateOptions) string {
	prompt := strings.ReplaceAll(template, "{{CONTENT}}", content)

	if opts.ProductName != "" {
		prompt = strings.ReplaceAll(prompt, "{{PRODUCT_NAME}}", opts.ProductName)
	} else {
		prompt = strings.ReplaceAll(prompt, "{{PRODUCT_NAME}}", "the project")
	}

	if opts.Version != nil {
		prompt = strings.ReplaceAll(prompt, "{{VERSION}}", opts.Version.String())
	} else {
		prompt = strings.ReplaceAll(prompt, "{{VERSION}}", "")
	}

	if opts.Context != "" {
		var b strings.Builder
		b.Grow(len(prompt) + len(opts.Context) + 25)
		b.WriteString(prompt)
		b.WriteString("\n\nAdditional context: ")
		b.WriteString(opts.Context)
		return b.String()
	}

	return prompt
}

// formatChangesForPrompt formats categorized changes for use in prompts.
// This is shared across all AI service implementations.
func formatChangesForPrompt(changes *git.CategorizedChanges) string {
	// Estimate capacity based on change counts (avg ~50 chars per change)
	totalChanges := len(changes.Breaking) + len(changes.Features) + len(changes.Fixes) +
		len(changes.Performance) + len(changes.Documentation) + len(changes.Other)
	var sb strings.Builder
	sb.Grow(totalChanges*60 + 200) // 60 chars per change + headers

	// Helper to write a change entry
	writeChange := func(c *git.ConventionalCommit) {
		sb.WriteString("- ")
		sb.WriteString(c.Description)
		if c.Scope != "" {
			sb.WriteString(" (")
			sb.WriteString(c.Scope)
			sb.WriteByte(')')
		}
		sb.WriteByte('\n')
	}

	if len(changes.Breaking) > 0 {
		sb.WriteString("BREAKING CHANGES:\n")
		for i := range changes.Breaking {
			writeChange(&changes.Breaking[i])
		}
		sb.WriteByte('\n')
	}

	if len(changes.Features) > 0 {
		sb.WriteString("NEW FEATURES:\n")
		for i := range changes.Features {
			if !changes.Features[i].Breaking {
				writeChange(&changes.Features[i])
			}
		}
		sb.WriteByte('\n')
	}

	if len(changes.Fixes) > 0 {
		sb.WriteString("BUG FIXES:\n")
		for i := range changes.Fixes {
			writeChange(&changes.Fixes[i])
		}
		sb.WriteByte('\n')
	}

	if len(changes.Performance) > 0 {
		sb.WriteString("PERFORMANCE IMPROVEMENTS:\n")
		for i := range changes.Performance {
			writeChange(&changes.Performance[i])
		}
		sb.WriteByte('\n')
	}

	if len(changes.Documentation) > 0 {
		sb.WriteString("DOCUMENTATION:\n")
		for i := range changes.Documentation {
			writeChange(&changes.Documentation[i])
		}
		sb.WriteByte('\n')
	}

	if len(changes.Other) > 0 {
		sb.WriteString("OTHER CHANGES:\n")
		for i := range changes.Other {
			writeChange(&changes.Other[i])
		}
	}

	return sb.String()
}

// Default prompt templates

const defaultChangelogSystemPrompt = `You are a technical writer specializing in software changelogs.
Your task is to transform commit information into a well-structured, human-readable changelog entry.
Follow the Keep a Changelog format (https://keepachangelog.com/).
Group changes by type: Added, Changed, Deprecated, Removed, Fixed, Security.
Be concise but informative. Each entry should clearly explain what changed and why it matters.`

const defaultChangelogUserPrompt = `Generate a changelog entry for {{PRODUCT_NAME}} version {{VERSION}} based on these changes:

{{CONTENT}}

Format the output as a proper changelog entry with date and version header.`

const defaultReleaseNotesSystemPrompt = `You are a technical writer creating release notes for software releases.
Your task is to create engaging, informative release notes that highlight the most important changes.
Structure the notes with a brief overview followed by detailed sections.
Focus on user impact and benefits rather than just listing changes.`

const defaultReleaseNotesUserPrompt = `Create release notes for {{PRODUCT_NAME}} version {{VERSION}} based on this changelog:

{{CONTENT}}

Start with a brief summary of the release, then detail the key changes.`

const defaultMarketingSystemPrompt = `You are a marketing copywriter for software products.
Your task is to create compelling marketing content that highlights the value of new releases.
Focus on benefits, not features. Use clear, engaging language.
Create content suitable for social media, newsletters, or blog posts.`

const defaultMarketingUserPrompt = `Create a marketing blurb for {{PRODUCT_NAME}} version {{VERSION}} based on these release notes:

{{CONTENT}}

Make it engaging and highlight the most impactful improvements.`

const defaultSummarySystemPrompt = `You are a technical writer who excels at summarizing software changes.
Your task is to create a concise summary of changes that captures the essence of a release.
Be brief but comprehensive. Highlight the most important changes first.`

const defaultSummaryUserPrompt = `Summarize the following changes for {{PRODUCT_NAME}} in 2-3 sentences:

{{CONTENT}}`
