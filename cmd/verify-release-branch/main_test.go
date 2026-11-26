package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestAllowedRemoteBranchesAndConfiguredPatterns(t *testing.T) {
	repo := t.TempDir()
	t.Chdir(repo)
	runGit(t, "init", "-q")
	runGit(t, "config", "user.name", "Relicta Test")
	runGit(t, "config", "user.email", "relicta-test@example.invalid")
	runGit(t, "commit", "--allow-empty", "-qm", "base")
	base, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("resolve base commit: %v", err)
	}
	baseSHA := strings.TrimSpace(string(base))
	runGit(t, "update-ref", "refs/remotes/origin/main", baseSHA)
	runGit(t, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	runGit(t, "commit", "--allow-empty", "-qm", "feature-only")
	feature, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("resolve feature commit: %v", err)
	}
	featureSHA := strings.TrimSpace(string(feature))
	runGit(t, "update-ref", "refs/remotes/origin/feature/only", featureSHA)

	branches, err := allowedRemoteBranches(context.Background(), baseSHA, "origin")
	if err != nil {
		t.Fatalf("branches containing base: %v", err)
	}
	if !contains(branches, "main") || !contains(branches, "feature/only") {
		t.Fatalf("branches containing base = %v, want main and feature/only", branches)
	}
	if contains(branches, "HEAD") {
		t.Fatal("the origin/HEAD symbolic ref must not be treated as an allowed branch")
	}
	if !branchAllowed([]string{"main", "release/*"}, "main") {
		t.Fatal("main should match configured branch")
	}
	if branchAllowed([]string{"main", "release/*"}, "feature/only") {
		t.Fatal("feature branch must not match configured allowed branches")
	}
	if branchAllowed([]string{"release/*"}, "release/1.2/hotfix") {
		t.Fatal("single-level release glob must not match nested branch")
	}
	branches, err = allowedRemoteBranches(context.Background(), featureSHA, "origin")
	if err != nil {
		t.Fatalf("branches containing feature: %v", err)
	}
	if len(branches) != 1 || branches[0] != "feature/only" {
		t.Fatalf("branches containing feature = %v, want only feature/only", branches)
	}
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
