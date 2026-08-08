package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// WriteConfig used to hand Go structs straight to viper, whose writer lowercases
// field names and never consults mapstructure tags — the only names the loader
// reads. So `relicta init` produced `tagprefix`, `apikey`, `includecommithash`,
// and 51 of the 74 keys it wrote matched nothing in the schema. Everything a user
// configured through those keys was silently ignored: `tag_prefix: "rel-"` was
// honored, and `tagprefix: "rel-"` — the form init itself wrote — was not.
//
// A round-trip test is the right shape here. Asserting specific key spellings
// would pin today's names; asserting that written config reads back unchanged
// catches any future divergence between writer and loader, whichever side moves.

// TestWriteConfigRoundTrips is the core guarantee: what relicta writes, relicta
// reads.
func TestWriteConfigRoundTrips(t *testing.T) {
	cfg := DefaultConfig()

	// Values deliberately different from the defaults, and specifically on fields
	// whose mapstructure names contain underscores — those are the ones the old
	// writer mangled. A test using single-word keys would have passed throughout.
	cfg.Versioning.TagPrefix = "rel-"
	cfg.Versioning.GitTag = false
	cfg.Versioning.GitPush = false
	cfg.Versioning.BumpFrom = "file"
	cfg.Versioning.PrereleaseSuffix = "beta"
	cfg.Versioning.BuildMetadata = "ci42"
	cfg.Changelog.IncludeCommitHash = false
	cfg.Changelog.IncludeAuthor = true
	cfg.Changelog.LinkCommits = true
	cfg.Changelog.RepositoryURL = "https://example.invalid/org/repo"
	cfg.AI.MaxTokens = 4096
	cfg.AI.RetryAttempts = 7
	cfg.AI.IncludeEmoji = true
	cfg.AI.Timeout = 45 * time.Second

	path := filepath.Join(t.TempDir(), ".relicta.yaml")
	if err := WriteConfig(cfg, path); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	assert := func(name string, got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("%s did not survive the round trip: wrote %v, read back %v", name, want, got)
		}
	}

	assert("versioning.tag_prefix", loaded.Versioning.TagPrefix, cfg.Versioning.TagPrefix)
	assert("versioning.git_tag", loaded.Versioning.GitTag, cfg.Versioning.GitTag)
	assert("versioning.git_push", loaded.Versioning.GitPush, cfg.Versioning.GitPush)
	assert("versioning.bump_from", loaded.Versioning.BumpFrom, cfg.Versioning.BumpFrom)
	assert("versioning.prerelease_suffix", loaded.Versioning.PrereleaseSuffix, cfg.Versioning.PrereleaseSuffix)
	assert("versioning.build_metadata", loaded.Versioning.BuildMetadata, cfg.Versioning.BuildMetadata)
	assert("changelog.include_commit_hash", loaded.Changelog.IncludeCommitHash, cfg.Changelog.IncludeCommitHash)
	assert("changelog.include_author", loaded.Changelog.IncludeAuthor, cfg.Changelog.IncludeAuthor)
	assert("changelog.link_commits", loaded.Changelog.LinkCommits, cfg.Changelog.LinkCommits)
	assert("changelog.repository_url", loaded.Changelog.RepositoryURL, cfg.Changelog.RepositoryURL)
	assert("ai.max_tokens", loaded.AI.MaxTokens, cfg.AI.MaxTokens)
	assert("ai.retry_attempts", loaded.AI.RetryAttempts, cfg.AI.RetryAttempts)
	assert("ai.include_emoji", loaded.AI.IncludeEmoji, cfg.AI.IncludeEmoji)
	assert("ai.timeout", loaded.AI.Timeout, cfg.AI.Timeout)
}

// TestWrittenKeysUseMapstructureNames checks the file itself, so a failure points
// at the writer rather than at whichever setting happened to be asserted above.
func TestWrittenKeysUseMapstructureNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".relicta.yaml")
	if err := WriteDefaultConfig(path); err != nil {
		t.Fatalf("WriteDefaultConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	// The exact spellings the old writer produced. Each is a key the loader
	// cannot see, so finding one means configuration is being silently dropped.
	mangled := []string{
		"tagprefix:", "gitpush:", "gittag:", "gitsign:",
		"bumpfrom:", "versionfile:", "buildmetadata:", "prereleasesuffix:",
		"apikey:", "baseurl:", "maxtokens:", "retryattempts:",
		"includeemoji:", "includecommithash:", "includeauthor:",
		"linkcommits:", "repositoryurl:", "customprompts:",
	}
	for _, key := range mangled {
		if strings.Contains(content, key) {
			t.Errorf("generated config contains %q — a lowercased Go field name the "+
				"loader does not read; the writer is not using mapstructure tags", key)
		}
	}

	// And the correct forms should be present.
	for _, key := range []string{"tag_prefix:", "git_push:", "include_commit_hash:"} {
		if !strings.Contains(content, key) {
			t.Errorf("generated config is missing %q", key)
		}
	}
}

// TestGeneratedConfigCarriesSafetyNotes guards the comments that explain which
// settings reach the outside world.
//
// configAnnotations matches on key name, so when the writer's key names changed
// these silently stopped matching and every generated config lost the warning
// that git_push starts a real, irreversible public release (issue #194). Nothing
// failed — the file was simply less safe.
func TestGeneratedConfigCarriesSafetyNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".relicta.yaml")
	if err := WriteDefaultConfig(path); err != nil {
		t.Fatalf("WriteDefaultConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "IRREVERSIBLE") {
		t.Error("the git_push warning is missing — a configAnnotations key no longer " +
			"matches a key the writer emits")
	}

	// Every annotation must land, not just the one checked above.
	for _, ann := range configAnnotations {
		if !strings.Contains(content, ann.key+":") {
			t.Errorf("annotation keyed on %q matches nothing in the generated file", ann.key)
			continue
		}
		if !strings.Contains(content, "# "+ann.lines[0]) {
			t.Errorf("annotation for %q was not inserted", ann.key)
		}
	}
}

// Durations must stay human-editable. A plain encode writes time.Duration as
// integer nanoseconds, and nobody should have to recognize 30000000000 in a file
// they are expected to edit.
func TestDurationsAreWrittenReadably(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Timeout = 90 * time.Second

	path := filepath.Join(t.TempDir(), ".relicta.yaml")
	if err := WriteConfig(cfg, path); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "timeout: 1m30s") {
		t.Errorf("expected a readable duration, got:\n%s",
			firstMatchingLine(content, "timeout"))
	}
	if strings.Contains(content, "90000000000") {
		t.Error("duration was written as raw nanoseconds")
	}
}

func firstMatchingLine(content, needle string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return "(no matching line)"
}
