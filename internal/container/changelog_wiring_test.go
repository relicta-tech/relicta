package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/communication"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Every changelog.* rendering setting shipped with a default and was read by nothing outside
// the config package. The defaults describe a Keep a Changelog renderer — format
// keep-a-changelog, exclude [chore ci docs style test], categories mapping feat to "Features",
// include_commit_hash, include_date — while a release wrote a flat list of commit subjects
// with no version heading and no grouping.
//
// This is the translation that makes them mean something, so it is tested for carrying each
// setting across rather than for the renderer's own behavior.

func appWithChangelog(t *testing.T, mutate func(*config.ChangelogConfig)) *App {
	t.Helper()

	cfg := config.DefaultConfig()
	if mutate != nil {
		mutate(&cfg.Changelog)
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return app
}

func TestTheChangelogConfigurationReachesTheRenderer(t *testing.T) {
	app := appWithChangelog(t, func(c *config.ChangelogConfig) {
		c.Exclude = []string{"chore"}
		c.Categories = map[string]string{"feat": "Neuerungen"}
		c.IncludeCommitHash = false
		c.IncludeAuthor = true
		c.IncludeDate = false
		c.LinkCommits = true
		c.RepositoryURL = "https://github.com/owner/repo/"
	})

	opts := app.changelogRenderOptions()

	if len(opts.Exclude) != 1 || opts.Exclude[0] != "chore" {
		t.Errorf("Exclude = %v, want [chore]", opts.Exclude)
	}
	if opts.Categories["feat"] != "Neuerungen" {
		t.Errorf("Categories[feat] = %q, want the configured name", opts.Categories["feat"])
	}
	if opts.IncludeCommitHash {
		t.Error("IncludeCommitHash is on despite include_commit_hash: false")
	}
	if !opts.IncludeAuthor {
		t.Error("IncludeAuthor is off despite include_author: true")
	}
	if opts.IncludeDate {
		t.Error("IncludeDate is on despite include_date: false")
	}
	if !opts.LinkCommits {
		t.Error("LinkCommits is off despite link_commits: true")
	}
	// Trailing slash removed, or every link would carry a double slash.
	if opts.RepositoryURL != "https://github.com/owner/repo" {
		t.Errorf("RepositoryURL = %q, want it without the trailing slash", opts.RepositoryURL)
	}
}

// An empty exclude list is a real choice — "put everything in the changelog" — and must not be
// silently replaced by the defaults.
func TestAnEmptyExcludeListIsHonored(t *testing.T) {
	app := appWithChangelog(t, func(c *config.ChangelogConfig) { c.Exclude = []string{} })

	if got := app.changelogRenderOptions().Exclude; len(got) != 0 {
		t.Errorf("Exclude = %v, want empty: the operator asked for every type to appear", got)
	}
}

func TestAnInvalidFormatFallsBackRatherThanBreakingTheRelease(t *testing.T) {
	app := appWithChangelog(t, func(c *config.ChangelogConfig) { c.Format = "not-a-format" })

	if got := app.changelogRenderOptions().Format; got != communication.FormatKeepAChangelog {
		t.Errorf("Format = %q, want the default: a typo in the config should not stop a "+
			"release, and validation reports it separately", got)
	}
}

// The options are passed to the notes generator as a constructor argument rather than set
// afterwards, because an option nobody calls is exactly how this configuration came to be
// unread in the first place.
func TestTheNotesGeneratorIsBuiltWithTheChangelogOptions(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("container.go"))
	if err != nil {
		t.Fatalf("read container.go: %v", err)
	}

	if !strings.Contains(string(source), "NewNotesGeneratorAdapter(c.aiService, c.gitAdapter, c.changelogRenderOptions())") {
		t.Error("the notes generator is not constructed with the changelog options, so the " +
			"non-AI path — the default path — renders a flat list of commit subjects again")
	}
}

// The behavioral half of the wiring: the default path must produce grouped sections rather
// than the flat "- subject" list it emitted for every release before this.
func TestTheNonAIPathRendersGroupedSections(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil, communication.DefaultRenderOptions())

	run := createTestReleaseRunWithChangeset(t)
	notes, err := adapter.Generate(context.Background(), run, ports.NotesOptions{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(notes.Text, "### Features") {
		t.Errorf("notes = %q, want a Features section.\nThis path runs whenever no AI "+
			"provider is configured — the default — so it is what most changelogs are "+
			"made of", notes.Text)
	}
	if !strings.Contains(notes.Text, "add feature") {
		t.Errorf("notes = %q, want the commit subject", notes.Text)
	}
}
