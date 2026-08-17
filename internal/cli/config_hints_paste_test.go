package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The hints exist so a reader does not have to work out the shape of a setting from the schema.
// That only holds if the block works when pasted, and this hint has now been wrong twice: once
// with `repositories:` as a list of bare strings, which fails to load, and once with a two-space
// indent that made it a child of whatever key came before it.
//
// The second was the worse failure. Indented, the block parses only in an empty config file — and
// a hint is printed precisely because a config already exists, so pasting it after an existing
// `ai:` block made `repository_groups` a child of `ai`, and the command that printed the hint
// then told the operator "no repository groups are declared". They had followed the instructions
// exactly. Verified against the shipped binary in both arrangements.
//
// So this asserts what the earlier tests did not: that each hint, appended to a config that
// already has content, actually reaches the field it names.

// baseConfig is what a repository that has run `relicta init` already has. The hint is appended
// to it, because that is what an operator does.
const baseConfig = "ai:\n  provider: none\nworkflow:\n  require_approval: false\n"

func loadWithHint(t *testing.T, hint configHint) *config.Config {
	t.Helper()

	dir := t.TempDir()
	body := baseConfig + hint.yaml + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".relicta.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.LoadFromDirectory(dir)
	if err != nil {
		t.Fatalf("a config with this hint pasted in does not load: %v\n\n%s", err, body)
	}
	return cfg
}

func TestEachHintReachesTheSettingItNames(t *testing.T) {
	for _, c := range []struct {
		name string
		hint configHint
		// reached reports whether the setting the hint is about is populated.
		reached func(*config.Config) bool
	}{
		{
			name:    "repository_groups",
			hint:    hintRepositoryGroups,
			reached: func(c *config.Config) bool { return len(c.RepositoryGroups) > 0 },
		},
		{
			name:    "environments",
			hint:    hintEnvironments,
			reached: func(c *config.Config) bool { return len(c.Environments) > 0 },
		},
		{
			name:    "dashboard auth",
			hint:    hintDashboardAuth,
			reached: func(c *config.Config) bool { return len(c.Dashboard.Auth.APIKeys) > 0 },
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := loadWithHint(t, c.hint)

			if !c.reached(cfg) {
				t.Errorf("the hint loaded but %s is still empty.\nThe block was accepted as "+
					"YAML and landed somewhere else — an indented block becomes a child of "+
					"the key above it — so the operator follows the instructions and the "+
					"command still refuses.\n\npasted:\n%s", c.name, c.hint.yaml)
			}
		})
	}
}

// A block that only works in an empty file is the trap this pair of tests exists for: the
// indented version passed a naive "does it parse" check.
func TestNoHintIsIndentedAtItsFirstLine(t *testing.T) {
	for _, c := range []struct {
		name string
		hint configHint
	}{
		{"repository_groups", hintRepositoryGroups},
		{"environments", hintEnvironments},
		{"dashboard auth", hintDashboardAuth},
	} {
		t.Run(c.name, func(t *testing.T) {
			first := strings.SplitN(c.hint.yaml, "\n", 2)[0]

			if strings.HasPrefix(first, " ") || strings.HasPrefix(first, "\t") {
				t.Errorf("the block starts with whitespace: %q.\nPasted into a config that "+
					"already has keys, it becomes a child of the last one rather than a "+
					"top-level setting", first)
			}
		})
	}
}

// The specific shape that was wrong the first time. A group whose repositories are bare strings
// loads as nothing, and the note in the hint exists because of it — so the note has to keep
// describing the block above it.
func TestTheGroupHintDeclaresRepositoriesAsEntries(t *testing.T) {
	cfg := loadWithHint(t, hintRepositoryGroups)

	if len(cfg.RepositoryGroups) == 0 {
		t.Fatal("no groups loaded")
	}
	group := cfg.RepositoryGroups[0]
	if len(group.Repositories) == 0 {
		t.Fatal("the group has no repositories: 'repositories: [owner/repo]' as bare strings " +
			"is what the note warns against, and it loads as an empty list")
	}
	if group.Repositories[0].Name == "" || group.Repositories[0].Path == "" {
		t.Errorf("the first repository is %+v, want both a name and a path", group.Repositories[0])
	}
}
