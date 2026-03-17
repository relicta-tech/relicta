package benchmark

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestMockGitRepo_AllMethods(t *testing.T) {
	ctx := context.Background()
	repo := NewMockGitRepo(10)

	if _, err := repo.GetCommitDiffStats(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetRemotes(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetBranches(ctx); err != nil {
		t.Fatal(err)
	}
	if b, err := repo.GetCurrentBranch(ctx); err != nil || b != "main" {
		t.Fatalf("GetCurrentBranch = %q, err = %v", b, err)
	}
	if _, err := repo.GetCommit(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetCommitsSince(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetLatestCommit(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetCommitPatch(ctx, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetFileAtRef(ctx, "HEAD", "file.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetTag(ctx, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetLatestVersionTag(ctx, "v"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag(ctx, "v2.0.0", "abc", "release"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteTag(ctx, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := repo.PushTag(ctx, "v2.0.0", "origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.IsDirty(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetStatus(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(ctx, "origin"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pull(ctx, "origin", "main"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(ctx, "origin", "main"); err != nil {
		t.Fatal(err)
	}
}

func TestMockGitRepo_GetBatchCommitDiffStats(t *testing.T) {
	ctx := context.Background()
	repo := NewMockGitRepo(5)
	hashes := []sourcecontrol.CommitHash{"abc", "def"}
	stats, err := repo.GetBatchCommitDiffStats(ctx, hashes)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(stats))
	}
}

func TestMockVersionCalc(t *testing.T) {
	calc := NewMockVersionCalc()
	v := calc.CalculateNextVersion(calc.NextVersion, version.BumpPatch)
	if v.String() != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", v.String())
	}

	if calc.DetermineRequiredBump(true, false, false) != version.BumpMajor {
		t.Error("breaking should be major")
	}
	if calc.DetermineRequiredBump(false, true, false) != version.BumpMinor {
		t.Error("feature should be minor")
	}
	if calc.DetermineRequiredBump(false, false, true) != version.BumpPatch {
		t.Error("fix should be patch")
	}
}
