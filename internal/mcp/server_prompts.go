package mcp

import (
	"context"

	"go.klarlabs.de/mcp"
)

func (s *Server) handlePromptReleaseSummary(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	style := "brief"
	if v, ok := args["style"]; ok && v != "" {
		style = v
	}

	var content string
	switch style {
	case "detailed":
		content = `You are a release manager. Provide a detailed summary of the upcoming release including:
- All changes categorized by type (features, fixes, breaking changes)
- Impact analysis
- Risk assessment
- Recommended actions before release`
	case "technical":
		content = `You are a technical writer. Provide a technical summary of the release including:
- API changes and their signatures
- Configuration changes
- Migration requirements
- Performance implications`
	default:
		content = `You are a release manager. Provide a brief summary of the upcoming release including:
- Key highlights (1-3 bullet points)
- Version number
- Release readiness status`
	}

	return &mcp.PromptResult{
		Description: "Release summary prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptRiskAnalysis(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	content := `You are a release risk analyst using the Change Governance Protocol (CGP).

Analyze the current release and provide:
1. Overall risk score (0.0 - 1.0) with justification
2. Individual risk factors:
   - API changes impact
   - Dependency changes
   - Blast radius (scope of changes)
   - Security implications
   - Historical patterns
3. Recommendations:
   - Approval recommendation (approve, review, block)
   - Required actions before release
   - Suggested reviewers

Base your analysis on the commit history, change analysis, and CGP policies.`

	return &mcp.PromptResult{
		Description: "Risk analysis prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptCommitReview(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	focus := "compliance"
	if v, ok := args["focus"]; ok && v != "" {
		focus = v
	}

	var content string
	switch focus {
	case "quality":
		content = `You are a code review expert analyzing commit messages and changes.

Review the commits in this release for quality:
1. Commit message clarity and completeness
2. Logical grouping of changes (atomic commits)
3. Code change quality indicators
4. Documentation updates for significant changes
5. Test coverage implications

For each issue found, suggest specific improvements.`
	case "security":
		content = `You are a security analyst reviewing commits for potential security implications.

Analyze commits for security concerns:
1. Sensitive data handling changes
2. Authentication/authorization modifications
3. Input validation changes
4. Dependency updates with known vulnerabilities
5. Configuration changes affecting security posture
6. Cryptographic code changes

Flag any commits that require security review before release.`
	default: // compliance
		content = `You are a release compliance officer reviewing commits for conventional commit standards.

Analyze each commit for compliance with Conventional Commits specification:
1. Format: <type>(<scope>): <subject>
2. Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore
3. Breaking changes properly marked with ! or BREAKING CHANGE footer
4. Scope relevance and consistency
5. Subject line length and imperative mood

List non-compliant commits with specific corrections needed.`
	}

	return &mcp.PromptResult{
		Description: "Commit review prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptBreakingChanges(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	content := `You are a technical writer documenting breaking changes for users.

For each breaking change in this release, provide:

1. **Change Summary**: One-line description of what changed
2. **Reason**: Why this breaking change was necessary
3. **Impact**: Who is affected and how
4. **Migration Path**: Step-by-step instructions to adapt
5. **Code Examples**: Before/after code snippets where applicable

Format the output as a structured breaking changes document suitable for inclusion in release notes.

If there are no breaking changes, confirm this and explain what safeguards prevented them.`

	return &mcp.PromptResult{
		Description: "Breaking changes documentation prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptMigrationGuide(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	audience := "developer"
	if v, ok := args["audience"]; ok && v != "" {
		audience = v
	}

	var content string
	switch audience {
	case "operator":
		content = `You are a DevOps engineer writing migration instructions for system operators.

Create a migration guide covering:
1. **Pre-migration Checklist**
   - Backup procedures
   - Downtime requirements
   - Rollback plan

2. **Infrastructure Changes**
   - Configuration file updates
   - Environment variable changes
   - Database migrations

3. **Deployment Steps**
   - Ordered deployment sequence
   - Health check verification
   - Monitoring updates

4. **Post-migration Verification**
   - Smoke tests to run
   - Metrics to monitor
   - Common issues and solutions

Use clear, actionable language suitable for runbooks.`
	case "end-user":
		content = `You are a product manager writing migration notes for end users.

Create user-friendly upgrade instructions:
1. **What's New**: Key benefits of upgrading
2. **What's Changed**: User-facing changes to expect
3. **Action Required**: Steps users need to take
4. **Getting Help**: Support resources and FAQ

Keep technical jargon minimal. Focus on the user experience impact.`
	default: // developer
		content = `You are a senior developer writing migration instructions for other developers.

Create a developer migration guide covering:
1. **Dependency Updates**
   - Version requirements
   - Package manager commands

2. **API Changes**
   - Changed endpoints/methods
   - Parameter modifications
   - Response format changes
   - Deprecation timeline

3. **Code Migration**
   - Find/replace patterns
   - Refactoring steps
   - Type definition updates

4. **Testing Updates**
   - Test changes needed
   - New test patterns

Include code examples for all significant changes.`
	}

	return &mcp.PromptResult{
		Description: "Migration guide prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptReleaseAnnouncement(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	channel := "github"
	if v, ok := args["channel"]; ok && v != "" {
		channel = v
	}

	var content string
	switch channel {
	case "blog":
		content = `You are a technical writer crafting a blog post for a software release.

Write a blog post announcement including:
1. **Headline**: Compelling title highlighting the main theme
2. **Introduction**: Context and significance of this release
3. **Key Features**: Detailed coverage of major additions
4. **Improvements**: Notable fixes and enhancements
5. **Technical Deep-dive**: One feature explained in depth
6. **Getting Started**: How to upgrade or try it
7. **What's Next**: Roadmap preview
8. **Acknowledgments**: Contributor recognition

Tone: Professional but engaging, 800-1200 words.`
	case "social":
		content = `You are a developer advocate crafting social media announcements.

Create announcements for different platforms:

**Twitter/X (280 chars)**:
- Hook + key feature + link

**LinkedIn (longer form)**:
- Professional summary
- 3 key highlights
- Call to action

**Mastodon/Dev Community**:
- Technical focus
- Code snippet if relevant
- Community engagement

Use appropriate hashtags and emoji sparingly.`
	case "email":
		content = `You are writing a release announcement email for subscribers.

Structure the email:
1. **Subject Line**: Clear, action-oriented
2. **Preview Text**: Compelling summary
3. **Header**: Version number and release date
4. **Executive Summary**: 2-3 sentence overview
5. **Highlights**: Bullet points of key changes
6. **Upgrade Instructions**: Brief steps
7. **Full Changelog Link**: For detailed information
8. **Feedback Request**: How to provide input

Keep it scannable with clear visual hierarchy.`
	default: // github
		content = `You are writing release notes for a GitHub release.

Structure the release notes:
1. **Title**: Version number with optional codename
2. **Summary**: 2-3 sentence release overview
3. **Highlights**: Top 3-5 changes as bullet points
4. **What's Changed**: Categorized changes
   - ✨ Features
   - 🐛 Bug Fixes
   - 📚 Documentation
   - ⚠️ Breaking Changes
5. **Upgrade Notes**: Critical information for upgrading
6. **Contributors**: @mention contributors

Use GitHub-flavored markdown with appropriate emoji.`
	}

	return &mcp.PromptResult{
		Description: "Release announcement prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

func (s *Server) handlePromptApprovalDecision(ctx context.Context, args map[string]string) (*mcp.PromptResult, error) {
	content := `You are a release governance advisor helping make approval decisions.

Based on the Change Governance Protocol (CGP) analysis, provide:

1. **Decision Recommendation**: APPROVE, REQUEST_CHANGES, or BLOCK
   - Clear rationale for your recommendation
   - Confidence level (high/medium/low)

2. **Risk Assessment Summary**
   - Overall risk level and score
   - Top 3 risk factors to consider
   - Mitigating factors present

3. **Approval Conditions** (if recommending approval)
   - Required reviewers and their focus areas
   - Pre-release checks to complete
   - Monitoring requirements post-release

4. **Blocking Issues** (if recommending block)
   - Specific issues that must be resolved
   - Suggested remediation steps
   - Criteria for re-evaluation

5. **Audit Trail Entry**
   - Structured summary for governance records
   - Key decision factors
   - Timestamp and context

Provide actionable guidance that enables confident decision-making.`

	return &mcp.PromptResult{
		Description: "Approval decision prompt",
		Messages: []mcp.PromptMessage{
			{Role: "user", Content: mcp.TextContent{Type: "text", Text: content}},
		},
	}, nil
}

// =============================================================================
// CGP Protocol Wire Format Handlers
// =============================================================================
