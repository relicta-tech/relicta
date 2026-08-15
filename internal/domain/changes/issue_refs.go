package changes

import (
	"regexp"
	"strings"
)

var (
	// issueRefPattern matches a numeric issue reference, "#123". The digits are required:
	// a bare "#" is a markdown heading, a color, or a comment far more often than it is an
	// issue.
	issueRefPattern = regexp.MustCompile(`#(\d+)`)

	// issueTrailerPattern matches a git trailer that says this commit acts on an issue,
	// with or without the colon the convention makes optional: "Closes: #123", "Fixes #45",
	// "Refs #7, #8". The tokens are the issue-related half of isFooterToken, so the parser
	// and this agree about what a trailer is.
	issueTrailerPattern = regexp.MustCompile(`(?i)^[ \t]*(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?|refs?|see)\b[ \t]*:?[ \t]*(.*)$`)
)

// IssueRefs returns the issue references this commit declares, as "#123" strings, in the order
// they appear and without duplicates.
//
// Only trailer lines are read — "Closes: #123", "Fixes #45", "Refs #7, #8" — because a trailer
// is the one place an author states that this commit acts on that issue. Two nearer sources
// are deliberately left alone:
//
//   - Free-form body prose. A "#99" in a paragraph is usually discussion — "unlike #99, this
//     keeps the old flag" — and listing it claims the release resolved an issue it only
//     mentioned. A missing reference is invisible; a wrong one sends a reader to the wrong
//     ticket.
//   - The subject line. The "(#45)" a squash merge leaves there is a pull request number, and
//     issue_url points at an issue tracker, which is not always the same place — GitHub
//     redirects one to the other, a project tracking work in Jira does not. It is also
//     already in front of the reader as text, so linking it would print the number twice on
//     one line for the sake of a hyperlink. ChangelogItem keeps PRRefs for that concept.
//
// Empty when the commit references nothing, which is the common case.
func (c *ConventionalCommit) IssueRefs() []string {
	if c == nil {
		return nil
	}

	// The raw message is the whole truth when it is there, and it usually is: both analyzer
	// paths pass WithRawMessage and it survives the state file. Commits assembled
	// field-by-field — the older state entries, and tests — fall back to the parsed parts.
	// The subject is dropped either way; only what follows it can hold a trailer.
	body := strings.Join([]string{c.body, c.footer}, "\n")
	if raw := strings.TrimSpace(c.rawMessage); raw != "" {
		_, body, _ = strings.Cut(raw, "\n")
	}

	refs := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)

	for _, line := range strings.Split(body, "\n") {
		match := issueTrailerPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for _, ref := range issueRefPattern.FindAllStringSubmatch(match[1], -1) {
			number := "#" + ref[1]
			if _, duplicate := seen[number]; duplicate {
				continue
			}
			seen[number] = struct{}{}
			refs = append(refs, number)
		}
	}

	if len(refs) == 0 {
		return nil
	}
	return refs
}
