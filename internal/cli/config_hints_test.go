package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// `relicta init` writes 7 of the schema's 21 top-level sections, deliberately: all 21 would
// be several hundred lines of advanced settings sitting at their defaults, and the generated
// file is currently short enough to read. The cost is that a message telling someone to
// configure a setting can name a section their file does not contain — they open it, the
// section is absent, and nothing indicates the nesting to add.
//
// The rule this enforces: a user-facing message that names a config section must either name
// one `init` writes, or carry the YAML to add. errGovernanceDisabled established the shape;
// hintEnvironments, hintDashboardAPIKeys and hintRepositoryGroups follow it.
//
// This is the invariant the backlog asked for, and it is worth having as a test rather than
// a convention because the failure is silent: the message looks helpful, and only someone
// following it discovers there is nothing to edit.

// sectionsWrittenByInit returns the top-level keys `relicta init` puts in a new config.
//
// Read from a file WriteDefaultConfig actually produces, not from a hand-written list, so a
// section added to or removed from the generated file changes this automatically.
func sectionsWrittenByInit(t *testing.T) map[string]bool {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".relicta.yaml")
	if err := config.WriteDefaultConfig(path); err != nil {
		t.Fatalf("WriteDefaultConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}

	sections := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "#") {
			continue
		}
		if key, _, found := strings.Cut(line, ":"); found {
			sections[strings.TrimSpace(key)] = true
		}
	}
	if len(sections) == 0 {
		t.Fatal("parsed no sections out of the generated config, so this test would pass " +
			"while asserting nothing")
	}
	return sections
}

// configKeyMention is a section named in a user-facing string, and the hint expected to
// accompany it.
//
// Enumerated rather than discovered, because a scan of every string containing
// ".relicta.yaml" also matches prose in long help text and comments, where naming a section
// is fine. What matters is that each *actionable* message has a remedy, so each is listed
// with the hint that must exist for it.
var configKeyMentions = []struct {
	section string
	file    string
	hint    func() string
}{
	{section: "governance", file: "governance_disabled.go", hint: func() string { return governanceEnableHint }},
	{section: "environments", file: "deploy.go", hint: hintEnvironments.String},
	{section: "dashboard", file: "serve.go", hint: hintDashboardAPIKeys.String},
	{section: "repository_groups", file: "multirepo.go", hint: hintRepositoryGroups.String},
}

func TestEveryConfigKeyMentionedHasSomewhereToLook(t *testing.T) {
	written := sectionsWrittenByInit(t)

	for _, m := range configKeyMentions {
		t.Run(m.section, func(t *testing.T) {
			hint := m.hint()

			// A hint must show the section nested as it appears in the file, or the
			// reader still has to guess where it goes.
			if !strings.Contains(hint, m.section+":") {
				t.Errorf("the hint for %s never shows %q as a YAML key, so a reader cannot "+
					"tell what to add or where", m.file, m.section)
			}

			// And it must point at the file, so the remedy is complete rather than a
			// fragment of YAML with no home.
			if !strings.Contains(hint, ".relicta.yaml") {
				t.Errorf("the hint for %s does not say which file to add it to", m.file)
			}

			if written[m.section] {
				// Written by init, so naming it would have been enough. The hint is not
				// wrong, but this records which case each mention is.
				t.Logf("%s is written by init, so the hint is a convenience rather than a "+
					"requirement", m.section)
			}
		})
	}
}

// The messages themselves must not name a key without printing the remedy. This catches the
// regression directly: someone adding "configure X in .relicta.yaml" to an error string,
// which is what all three of these used to be.
func TestActionableMessagesDoNotJustNameAKey(t *testing.T) {
	// Matches a string that tells the reader to configure something in the config file
	// without a newline — a one-line instruction, which cannot contain a YAML block.
	nameOnly := regexp.MustCompile(`"[^"\n]*(configure|enable|add)[^"\n]*\.relicta\.yaml[^"\n]*"`)

	for _, name := range []string{"deploy.go", "serve.go", "multirepo.go", "evaluate.go"} {
		source, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		for _, match := range nameOnly.FindAllString(string(source), -1) {
			t.Errorf("%s contains a one-line instruction naming the config file:\n  %s\n"+
				"Point at a configHint instead, so the reader gets the YAML and its nesting "+
				"rather than a key they then have to locate in a file that may not contain it.",
				name, match)
		}
	}
}
