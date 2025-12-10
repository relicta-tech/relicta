// Package changes provides domain types for analyzing commit changes.
package changes

import (
	"regexp"
	"strings"
	"time"
)

// ConventionalCommit represents a parsed conventional commit.
// This is an entity in DDD terms - it has identity (hash) and state.
type ConventionalCommit struct {
	// Identity
	hash string

	// Conventional commit components
	commitType CommitType
	scope      string
	subject    string
	body       string
	footer     string

	// Metadata
	breaking    bool
	breakingMsg string
	author      string
	authorEmail string
	date        time.Time

	// Original raw message
	rawMessage string
}

// ConventionalCommitOption is a functional option for creating commits.
type ConventionalCommitOption func(*ConventionalCommit)

// WithScope sets the commit scope.
func WithScope(scope string) ConventionalCommitOption {
	return func(c *ConventionalCommit) {
		c.scope = scope
	}
}

// WithBody sets the commit body.
func WithBody(body string) ConventionalCommitOption {
	return func(c *ConventionalCommit) {
		c.body = body
	}
}

// WithFooter sets the commit footer.
func WithFooter(footer string) ConventionalCommitOption {
	return func(c *ConventionalCommit) {
		c.footer = footer
	}
}

// WithBreaking marks the commit as a breaking change.
func WithBreaking(msg string) ConventionalCommitOption {
	return func(c *ConventionalCommit) {
		c.breaking = true
		c.breakingMsg = msg
	}
}

// WithAuthor sets the commit author.
func WithAuthor(name, email string) ConventionalCommitOption {
	return func(c *ConventionalCommit) {
		c.author = name
		c.authorEmail = email
	}
}

// WithDate sets the commit date.
func WithDate(date time.Time) ConventionalCommitOption {
	return func(c *ConventionalCommit) {
		c.date = date
	}
}

// NewConventionalCommit creates a new ConventionalCommit entity.
func NewConventionalCommit(hash string, commitType CommitType, subject string, opts ...ConventionalCommitOption) *ConventionalCommit {
	c := &ConventionalCommit{
		hash:       hash,
		commitType: commitType,
		subject:    subject,
		date:       time.Now(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Regex patterns for parsing conventional commits.
var (
	// Matches: type(scope)!: subject or type!: subject or type(scope): subject or type: subject
	conventionalCommitRegex = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?\s*:\s*(.+)$`)

	// Matches BREAKING CHANGE: or BREAKING-CHANGE: in footer
	breakingChangeRegex = regexp.MustCompile(`(?i)^BREAKING[ -]CHANGE:\s*(.+)$`)
)

// ParseConventionalCommit parses a commit message into a ConventionalCommit.
// Returns nil if the message is not a valid conventional commit.
func ParseConventionalCommit(hash, message string, opts ...ConventionalCommitOption) *ConventionalCommit {
	if message == "" {
		return nil
	}

	// Split into lines
	lines := strings.Split(strings.TrimSpace(message), "\n")
	if len(lines) == 0 {
		return nil
	}

	// Parse first line (subject line)
	matches := conventionalCommitRegex.FindStringSubmatch(strings.TrimSpace(lines[0]))
	if matches == nil {
		return nil
	}

	commitType, valid := ParseCommitType(matches[1])
	if !valid {
		// Allow unknown types but mark them
		commitType = CommitType(matches[1])
	}

	scope := matches[2]
	breaking := matches[3] == "!"
	subject := strings.TrimSpace(matches[4])

	// Parse body and footer
	var body, footer string
	var breakingMsg string

	if len(lines) > 1 {
		// Skip empty line after subject
		bodyStart := 1
		if bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) == "" {
			bodyStart++
		}

		// Collect body and footer
		var bodyLines, footerLines []string
		inFooter := false

		for i := bodyStart; i < len(lines); i++ {
			line := lines[i]

			// Check for breaking change in footer
			if bcMatch := breakingChangeRegex.FindStringSubmatch(line); bcMatch != nil {
				breaking = true
				breakingMsg = bcMatch[1]
				inFooter = true
				footerLines = append(footerLines, line)
				continue
			}

			// Simple footer detection: lines starting with token:
			if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && isFooterToken(parts[0]) {
					inFooter = true
					footerLines = append(footerLines, line)
					continue
				}
			}

			if inFooter {
				footerLines = append(footerLines, line)
			} else {
				bodyLines = append(bodyLines, line)
			}
		}

		body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		footer = strings.TrimSpace(strings.Join(footerLines, "\n"))
	}

	c := &ConventionalCommit{
		hash:        hash,
		commitType:  commitType,
		scope:       scope,
		subject:     subject,
		body:        body,
		footer:      footer,
		breaking:    breaking,
		breakingMsg: breakingMsg,
		rawMessage:  message,
		date:        time.Now(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// isFooterToken checks if a string looks like a git trailer token.
func isFooterToken(s string) bool {
	s = strings.TrimSpace(s)
	// Common footer tokens
	switch strings.ToLower(s) {
	case "breaking change", "breaking-change", "closes", "fixes", "resolves",
		"refs", "see", "co-authored-by", "signed-off-by", "reviewed-by",
		"acked-by", "tested-by":
		return true
	}
	return false
}

// Hash returns the commit hash.
func (c *ConventionalCommit) Hash() string {
	return c.hash
}

// ShortHash returns the short (7 character) commit hash.
func (c *ConventionalCommit) ShortHash() string {
	if len(c.hash) > 7 {
		return c.hash[:7]
	}
	return c.hash
}

// Type returns the commit type.
func (c *ConventionalCommit) Type() CommitType {
	return c.commitType
}

// Scope returns the commit scope.
func (c *ConventionalCommit) Scope() string {
	return c.scope
}

// Subject returns the commit subject (description).
func (c *ConventionalCommit) Subject() string {
	return c.subject
}

// Body returns the commit body.
func (c *ConventionalCommit) Body() string {
	return c.body
}

// Footer returns the commit footer.
func (c *ConventionalCommit) Footer() string {
	return c.footer
}

// IsBreaking returns true if this is a breaking change.
func (c *ConventionalCommit) IsBreaking() bool {
	return c.breaking
}

// BreakingMessage returns the breaking change message if any.
func (c *ConventionalCommit) BreakingMessage() string {
	return c.breakingMsg
}

// Author returns the commit author name.
func (c *ConventionalCommit) Author() string {
	return c.author
}

// AuthorEmail returns the commit author email.
func (c *ConventionalCommit) AuthorEmail() string {
	return c.authorEmail
}

// Date returns the commit date.
func (c *ConventionalCommit) Date() time.Time {
	return c.date
}

// RawMessage returns the original commit message.
func (c *ConventionalCommit) RawMessage() string {
	return c.rawMessage
}

// AffectsChangelog returns true if this commit should appear in changelog.
func (c *ConventionalCommit) AffectsChangelog() bool {
	return c.commitType.AffectsChangelog() || c.breaking
}

// ReleaseType returns the release type this commit requires.
func (c *ConventionalCommit) ReleaseType() ReleaseType {
	return ReleaseTypeFromCommitType(c.commitType, c.breaking)
}

// FormattedSubject returns the subject formatted for changelog display.
func (c *ConventionalCommit) FormattedSubject() string {
	if c.scope != "" {
		return "**" + c.scope + ":** " + c.subject
	}
	return c.subject
}

// String returns a string representation of the commit.
func (c *ConventionalCommit) String() string {
	var sb strings.Builder
	// Pre-allocate: type(10) + scope(20) + subject + extras(10)
	sb.Grow(10 + len(c.scope) + 10 + len(c.subject) + 10)
	sb.WriteString(string(c.commitType))
	if c.scope != "" {
		sb.WriteString("(")
		sb.WriteString(c.scope)
		sb.WriteString(")")
	}
	if c.breaking {
		sb.WriteString("!")
	}
	sb.WriteString(": ")
	sb.WriteString(c.subject)
	return sb.String()
}
