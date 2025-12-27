package git

import (
	"context"
	"testing"
	"time"
)

func TestTimeoutHelpers(t *testing.T) {
	ctx := context.Background()
	localCtx, cancelLocal := withLocalTimeout(ctx)
	defer cancelLocal()

	if dl, ok := localCtx.Deadline(); !ok {
		t.Fatal("expected local context to have deadline")
	} else if time.Until(dl) > DefaultLocalTimeout {
		t.Fatalf("deadlines should not exceed %v", DefaultLocalTimeout)
	}

	shortCtx, shortCancel := context.WithTimeout(ctx, 1*time.Second)
	defer shortCancel()
	withShort, cancelShort := withLocalTimeout(shortCtx)
	defer cancelShort()
	dl, _ := withShort.Deadline()
	if diff := time.Until(dl); diff > 2*time.Second {
		t.Fatalf("expected short deadline to remain under 2s, got %v", diff)
	}
}

func TestPathHelpers(t *testing.T) {
	if got := extractRepoName("/Users/alice/projects/repo"); got != "repo" {
		t.Fatalf("expected repo name 'repo', got %q", got)
	}
	if got := extractOwner("git@github.com:owner/repo.git"); got != "owner" {
		t.Fatalf("expected owner 'owner', got %q", got)
	}
	if got := extractOwner("https://gitlab.com/team/project.git"); got != "team" {
		t.Fatalf("expected owner 'team', got %q", got)
	}
	if parts := splitPath("a/b/c"); len(parts) != 3 {
		t.Fatalf("expected splitPath to return 3 parts, got %d", len(parts))
	}
}

func TestConvertCommitHelper(t *testing.T) {
	commit := Commit{
		Hash:    "abc123",
		Message: "subject\nbody",
		Author:  Author{Name: "Alice", Email: "alice@example.com"},
		Committer: Author{
			Name:  "Bob",
			Email: "bob@example.com",
		},
		Date:    time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
		Parents: []string{"parent"},
	}

	got := convertCommit(&commit)
	if got == nil {
		t.Fatal("convertCommit returned nil for valid commit")
	}
	if got.Hash().String() != "abc123" {
		t.Fatalf("expected hash abc123, got %s", got.Hash())
	}
	if got.Author().Name != "Alice" || got.Committer().Name != "Bob" {
		t.Fatalf("unexpected commit author/committer")
	}
	if len(got.Parents()) != 1 || got.Parents()[0].String() != "parent" {
		t.Fatalf("unexpected parents: %#v", got.Parents())
	}
}

func TestConvertDiffStatsHelper(t *testing.T) {
	if convertDiffStats(nil) != nil {
		t.Fatal("expected nil diff stats to return nil")
	}

	stats := &DiffStats{
		FilesChanged: 1,
		Insertions:   2,
		Deletions:    1,
		Files: []FileStats{
			{
				Path:       "file.txt",
				Insertions: 2,
				Deletions:  1,
				Status:     "modified",
				OldPath:    "",
			},
		},
	}

	converted := convertDiffStats(stats)
	if converted == nil || len(converted.Files) != 1 {
		t.Fatal("expected converted stats with file details")
	}
	if converted.Files[0].Path != "file.txt" || converted.Files[0].Additions != 2 {
		t.Fatalf("unexpected file stats: %#v", converted.Files[0])
	}
}

func TestServiceOptions_AuthAndFallback(t *testing.T) {
	cfg := DefaultServiceConfig()

	WithCLIFallback(false)(&cfg)
	WithAuthToken("token")(&cfg)
	WithAuthUsername("user")(&cfg)

	if cfg.UseCLIFallback {
		t.Fatal("expected UseCLIFallback to be false")
	}
	if cfg.AuthToken != "token" {
		t.Fatalf("unexpected AuthToken: %s", cfg.AuthToken)
	}
	if cfg.AuthUsername != "user" {
		t.Fatalf("unexpected AuthUsername: %s", cfg.AuthUsername)
	}
}
