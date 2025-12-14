// Package git provides git operations for Relicta.
package git

import (
	"strings"

	rperrors "github.com/relicta-tech/relicta/internal/errors"
)

// ParseConventionalCommit parses a commit message as a conventional commit.
func ParseConventionalCommit(message string) (*ConventionalCommit, error) {
	commit := Commit{
		Message: message,
	}
	commit.Subject, commit.Body = splitCommitMessage(message)

	return ParseConventionalCommitWithOptions(commit, DefaultParseOptions())
}

// ParseConventionalCommitWithOptions parses a commit with options.
func ParseConventionalCommitWithOptions(commit Commit, opts ParseOptions) (*ConventionalCommit, error) {
	const op = "git.ParseConventionalCommit"

	cc := &ConventionalCommit{
		Commit: commit,
	}

	// Parse the subject line
	matches := ConventionalCommitRegex.FindStringSubmatch(commit.Subject)
	if matches == nil {
		if opts.StrictMode {
			return nil, rperrors.Validation(op, "commit message does not follow conventional commit format")
		}
		cc.Type = CommitTypeUnknown
		cc.Description = commit.Subject
		cc.Body = commit.Body
		cc.IsConventional = false
		return cc, nil
	}

	// Extract named groups
	result := make(map[string]string)
	for i, name := range ConventionalCommitRegex.SubexpNames() {
		if i != 0 && name != "" && i < len(matches) {
			result[name] = matches[i]
		}
	}

	cc.Type = CommitType(result["type"])
	cc.Scope = result["scope"]
	cc.Description = result["description"]
	cc.Breaking = result["breaking"] == "!"
	cc.IsConventional = true

	// Parse body and footer
	if commit.Body != "" {
		cc.Body, cc.Footer = parseBodyAndFooter(commit.Body)

		// Check for BREAKING CHANGE in footer
		if !cc.Breaking && cc.Footer != "" {
			breakingMatches := BreakingChangeRegex.FindStringSubmatch(cc.Footer)
			if breakingMatches != nil {
				cc.Breaking = true
				cc.BreakingDescription = breakingMatches[1]
			}
		}

		// Parse references
		if opts.ParseReferences {
			cc.References = parseReferences(commit.Message)
		}
	}

	return cc, nil
}

// splitCommitMessage splits a commit message into subject and body.
func splitCommitMessage(message string) (subject, body string) {
	message = strings.TrimSpace(message)
	parts := strings.SplitN(message, "\n\n", 2)
	subject = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		body = strings.TrimSpace(parts[1])
	}
	return subject, body
}

// parseBodyAndFooter separates the body from the footer.
// Footer starts with a line that matches footer patterns (e.g., "BREAKING CHANGE:", "Fixes #123").
func parseBodyAndFooter(text string) (body, footer string) {
	lines := strings.Split(text, "\n")
	var bodyLines, footerLines []string
	inFooter := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line starts a footer
		if !inFooter && isFooterLine(trimmed) {
			inFooter = true
		}

		if inFooter {
			footerLines = append(footerLines, line)
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	footer = strings.TrimSpace(strings.Join(footerLines, "\n"))
	return body, footer
}

// footerPatternsMap provides O(1) lookup for footer patterns by their prefix.
// The map key is the pattern prefix (e.g., "BREAKING", "FIXES") for quick filtering.
var footerPatternsMap = map[string][]string{
	"BREAKING":    {"BREAKING CHANGE:", "BREAKING-CHANGE:"},
	"FIXES":       {"FIXES:"},
	"CLOSES":      {"CLOSES:"},
	"RESOLVES":    {"RESOLVES:"},
	"REFS":        {"REFS:"},
	"CO-AUTHORED": {"CO-AUTHORED-BY:"},
	"SIGNED-OFF":  {"SIGNED-OFF-BY:"},
	"REVIEWED-BY": {"REVIEWED-BY:"},
	"ACKED-BY":    {"ACKED-BY:"},
}

// footerPatternPrefixes are all unique prefixes for quick initial filtering.
// Pre-sorted by length descending for efficient early match.
var footerPatternPrefixes = []string{
	"BREAKING",
	"CO-AUTHORED",
	"SIGNED-OFF",
	"REVIEWED-BY",
	"ACKED-BY",
	"RESOLVES",
	"CLOSES",
	"FIXES",
	"REFS",
}

// isFooterLine checks if a line is a footer line.
func isFooterLine(line string) bool {
	// Check for common footer patterns using prefix map lookup (O(1) instead of O(n))
	upperLine := strings.ToUpper(line)

	// Try to match known footer patterns via prefix lookup
	for _, prefix := range footerPatternPrefixes {
		if strings.HasPrefix(upperLine, prefix) {
			// Found a potential prefix match, verify full pattern
			patterns := footerPatternsMap[prefix]
			for _, pattern := range patterns {
				if strings.HasPrefix(upperLine, pattern) {
					return true
				}
			}
		}
	}

	// Also check for "token: value" or "token #value" format
	if len(line) > 0 {
		// Must start with a word character and contain ": " or " #"
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], " ") {
			return true
		}
		parts = strings.SplitN(line, " #", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], " ") {
			return true
		}
	}

	return false
}

// parseReferences extracts issue/PR references from the commit message.
func parseReferences(message string) []Reference {
	var refs []Reference
	matches := ReferenceRegex.FindAllStringSubmatch(message, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			refType := "ref"
			raw := match[0]

			// Determine reference type
			lowerRaw := strings.ToLower(raw)
			switch {
			case strings.Contains(lowerRaw, "close"):
				refType = "closes"
			case strings.Contains(lowerRaw, "fix"):
				refType = "fixes"
			case strings.Contains(lowerRaw, "resolve"):
				refType = "resolves"
			}

			refs = append(refs, Reference{
				Type: refType,
				ID:   match[1],
				Raw:  raw,
			})
		}
	}

	return refs
}

// DetectReleaseType determines the release type based on conventional commits.
func DetectReleaseType(commits []ConventionalCommit) ReleaseType {
	var hasBreaking, hasFeature, hasFix, hasPerf bool

	for _, commit := range commits {
		if commit.Breaking {
			hasBreaking = true
		}
		switch commit.Type {
		case CommitTypeFeat:
			hasFeature = true
		case CommitTypeFix:
			hasFix = true
		case CommitTypePerf:
			hasPerf = true
		}
	}

	switch {
	case hasBreaking:
		return ReleaseTypeMajor
	case hasFeature:
		return ReleaseTypeMinor
	case hasFix || hasPerf:
		return ReleaseTypePatch
	default:
		if len(commits) > 0 {
			return ReleaseTypePatch
		}
		return ReleaseTypeNone
	}
}

// CategorizeCommits groups commits by their type.
func CategorizeCommits(commits []ConventionalCommit) *CategorizedChanges {
	changes := &CategorizedChanges{
		All: commits,
	}

	for _, commit := range commits {
		// Add to breaking if applicable
		if commit.Breaking {
			changes.Breaking = append(changes.Breaking, commit)
		}

		// Categorize by type
		switch commit.Type {
		case CommitTypeFeat:
			changes.Features = append(changes.Features, commit)
		case CommitTypeFix:
			changes.Fixes = append(changes.Fixes, commit)
		case CommitTypePerf:
			changes.Performance = append(changes.Performance, commit)
		case CommitTypeDocs:
			changes.Documentation = append(changes.Documentation, commit)
		case CommitTypeRefactor:
			changes.Refactoring = append(changes.Refactoring, commit)
		default:
			changes.Other = append(changes.Other, commit)
		}
	}

	return changes
}

// compiledFilter is a pre-compiled filter with map lookups for O(1) matching.
type compiledFilter struct {
	types          map[CommitType]struct{}
	authors        map[string]struct{}
	excludeAuthors map[string]struct{}
	scopes         map[string]struct{}
	excludeScopes  map[string]struct{}
	filter         CommitFilter // original filter for non-map fields
}

// compileFilter pre-compiles filter criteria into maps for efficient O(1) lookups.
func compileFilter(filter CommitFilter) *compiledFilter {
	cf := &compiledFilter{filter: filter}

	// Build type lookup map
	if len(filter.Types) > 0 {
		cf.types = make(map[CommitType]struct{}, len(filter.Types))
		for _, t := range filter.Types {
			cf.types[t] = struct{}{}
		}
	}

	// Build author lookup map
	if len(filter.Authors) > 0 {
		cf.authors = make(map[string]struct{}, len(filter.Authors))
		for _, author := range filter.Authors {
			cf.authors[author] = struct{}{}
		}
	}

	// Build exclude authors lookup map
	if len(filter.ExcludeAuthors) > 0 {
		cf.excludeAuthors = make(map[string]struct{}, len(filter.ExcludeAuthors))
		for _, author := range filter.ExcludeAuthors {
			cf.excludeAuthors[author] = struct{}{}
		}
	}

	// Build scope lookup map
	if len(filter.Scopes) > 0 {
		cf.scopes = make(map[string]struct{}, len(filter.Scopes))
		for _, scope := range filter.Scopes {
			cf.scopes[scope] = struct{}{}
		}
	}

	// Build exclude scopes lookup map
	if len(filter.ExcludeScopes) > 0 {
		cf.excludeScopes = make(map[string]struct{}, len(filter.ExcludeScopes))
		for _, scope := range filter.ExcludeScopes {
			cf.excludeScopes[scope] = struct{}{}
		}
	}

	return cf
}

// FilterCommits filters commits based on criteria.
// Uses pre-compiled maps for O(1) lookups instead of O(n) linear searches.
func FilterCommits(commits []ConventionalCommit, filter CommitFilter) []ConventionalCommit {
	// Pre-allocate assuming most commits pass filter (can shrink if needed)
	filtered := make([]ConventionalCommit, 0, len(commits))

	// Compile filter once for efficient lookups
	cf := compileFilter(filter)

	for _, commit := range commits {
		if cf.matches(commit) {
			filtered = append(filtered, commit)
		}
	}

	return filtered
}

// matches checks if a commit matches the compiled filter criteria.
// Uses O(1) map lookups instead of O(n) linear searches.
func (cf *compiledFilter) matches(commit ConventionalCommit) bool {
	// Filter by types (O(1) lookup)
	if cf.types != nil {
		if _, ok := cf.types[commit.Type]; !ok {
			return false
		}
	}

	// Filter by authors (O(1) lookup)
	if cf.authors != nil {
		if _, ok := cf.authors[commit.Commit.Author.Email]; !ok {
			return false
		}
	}

	// Exclude authors (O(1) lookup)
	if cf.excludeAuthors != nil {
		if _, ok := cf.excludeAuthors[commit.Commit.Author.Email]; ok {
			return false
		}
	}

	// Filter by scopes (O(1) lookup)
	if cf.scopes != nil {
		if _, ok := cf.scopes[commit.Scope]; !ok {
			return false
		}
	}

	// Exclude scopes (O(1) lookup)
	if cf.excludeScopes != nil {
		if _, ok := cf.excludeScopes[commit.Scope]; ok {
			return false
		}
	}

	// Include non-conventional
	if !cf.filter.IncludeNonConventional && !commit.IsConventional {
		return false
	}

	// Only breaking
	if cf.filter.OnlyBreaking && !commit.Breaking {
		return false
	}

	// Filter by date
	if cf.filter.Since != nil && commit.Commit.Date.Before(*cf.filter.Since) {
		return false
	}
	if cf.filter.Until != nil && commit.Commit.Date.After(*cf.filter.Until) {
		return false
	}

	return true
}

// FormatConventionalCommit formats a conventional commit as a string.
func FormatConventionalCommit(cc *ConventionalCommit) string {
	// Estimate capacity: type(10) + scope(20) + desc(80) + body + footer
	estimatedSize := 10 + len(cc.Scope) + 10 + len(cc.Description) + len(cc.Body) + len(cc.Footer) + 20
	var sb strings.Builder
	sb.Grow(estimatedSize)

	// Type and scope
	sb.WriteString(string(cc.Type))
	if cc.Scope != "" {
		sb.WriteString("(")
		sb.WriteString(cc.Scope)
		sb.WriteString(")")
	}
	if cc.Breaking {
		sb.WriteString("!")
	}
	sb.WriteString(": ")
	sb.WriteString(cc.Description)

	// Body
	if cc.Body != "" {
		sb.WriteString("\n\n")
		sb.WriteString(cc.Body)
	}

	// Footer
	if cc.Footer != "" {
		sb.WriteString("\n\n")
		sb.WriteString(cc.Footer)
	}

	return sb.String()
}

// CommitTypeDisplayName returns a human-readable name for a commit type.
func CommitTypeDisplayName(t CommitType) string {
	switch t {
	case CommitTypeFeat:
		return "Features"
	case CommitTypeFix:
		return "Bug Fixes"
	case CommitTypeDocs:
		return "Documentation"
	case CommitTypeStyle:
		return "Styles"
	case CommitTypeRefactor:
		return "Code Refactoring"
	case CommitTypePerf:
		return "Performance Improvements"
	case CommitTypeTest:
		return "Tests"
	case CommitTypeBuild:
		return "Build System"
	case CommitTypeCI:
		return "Continuous Integration"
	case CommitTypeChore:
		return "Chores"
	case CommitTypeRevert:
		return "Reverts"
	default:
		return "Other Changes"
	}
}

// CommitTypeEmoji returns an emoji for a commit type.
func CommitTypeEmoji(t CommitType) string {
	switch t {
	case CommitTypeFeat:
		return "✨"
	case CommitTypeFix:
		return "🐛"
	case CommitTypeDocs:
		return "📚"
	case CommitTypeStyle:
		return "💄"
	case CommitTypeRefactor:
		return "♻️"
	case CommitTypePerf:
		return "⚡"
	case CommitTypeTest:
		return "✅"
	case CommitTypeBuild:
		return "📦"
	case CommitTypeCI:
		return "👷"
	case CommitTypeChore:
		return "🔧"
	case CommitTypeRevert:
		return "⏪"
	default:
		return "📝"
	}
}
