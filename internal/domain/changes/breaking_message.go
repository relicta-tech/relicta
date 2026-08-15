package changes

import "strings"

// BreakingMessageFromMessage extracts the text of a BREAKING CHANGE footer, or "" when the
// commit marks a break without describing one (the `feat!:` form).
//
// The release analyzer passed the literal string "breaking change" for every breaking commit,
// so a changelog's BREAKING CHANGES section read "- breaking change" however carefully the
// author had written the footer. The parser already recognizes the footer — it just never saw
// the full message, because the analyzer hands NewConventionalCommit a one-line subject.
//
// An empty result is meaningful: it tells a renderer to fall back to the subject rather than
// print a placeholder.
func BreakingMessageFromMessage(message string) string {
	for _, line := range strings.Split(message, "\n") {
		if match := breakingChangeRegex.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}
