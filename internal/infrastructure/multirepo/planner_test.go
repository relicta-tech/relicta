package multirepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The coordinator consults a ReleaseExecutor to answer what each member would release next,
// and the CLI passed nil — so `relicta group plan` reported which repositories had changes
// and how many, with the NEXT column blank. That column is the question the command exists to
// answer, and the arithmetic already existed for the single-repository path.

// commitIn adds a commit with the given message to a repository.
func commitIn(t *testing.T, dir, message string) {
	t.Helper()

	name := filepath.Join(dir, strings.NewReplacer(" ", "-", ":", "", "!", "", "(", "", ")", "").Replace(message)+".txt")
	if err := os.WriteFile(name, []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestThePlannedVersionFollowsTheCommits(t *testing.T) {
	for name, tc := range map[string]struct {
		commit string
		want   string
	}{
		"a fix is a patch":             {"fix: correct the thing", "1.2.1"},
		"a feature is a minor":         {"feat: add the thing", "1.3.0"},
		"a breaking change is a major": {"feat!: replace the thing", "2.0.0"},
	} {
		t.Run(name, func(t *testing.T) {
			repo := gitRepo(t, "v1.2.0", 0)
			commitIn(t, repo, tc.commit)

			result, err := NewPlanner("v").Plan(context.Background(), repo)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if result.CurrentVersion != "1.2.0" {
				t.Errorf("CurrentVersion = %q, want 1.2.0", result.CurrentVersion)
			}
			if result.NextVersion != tc.want {
				t.Errorf("NextVersion = %q, want %q for %q — this is the column that was "+
					"blank, and a wrong answer in it is worse than none",
					result.NextVersion, tc.want, tc.commit)
			}
		})
	}
}

// A chore plans a patch, which is what the single-repository path does: DetectReleaseType
// treats any commit that is not feat, fix or perf as still worth a patch, and only an empty
// commit set as no release.
//
// I asserted the opposite here first — that chores produce no version — and it was my
// opinion rather than this tool's rule. A group whose members planned by different rules than
// a single repository would be the worse outcome by far: the same commits would produce
// different versions depending on which command was run, and neither answer would be wrong on
// its own terms. The planner defers to DetectReleaseType for exactly that reason, and this
// test now records the rule instead of contradicting it.
func TestAChorePlansAPatchJustAsItDoesForOneRepository(t *testing.T) {
	repo := gitRepo(t, "v1.2.0", 0)
	commitIn(t, repo, "chore: tidy the makefile")

	result, err := NewPlanner("v").Plan(context.Background(), repo)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if result.NextVersion != "1.2.1" {
		t.Errorf("NextVersion = %q, want 1.2.1: a group must plan by the same rule as a "+
			"single repository, or the same commits produce different versions depending on "+
			"which command ran", result.NextVersion)
	}
}

// An empty commit set is the case that produces nothing, and the coordinator skips such a
// repository before the planner is consulted at all.
func TestNoCommitsPlansNoVersion(t *testing.T) {
	repo := gitRepo(t, "v1.2.0", 0)

	result, err := NewPlanner("v").Plan(context.Background(), repo)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if result.HasChanges {
		t.Error("HasChanges = true for a repository sitting on its tag")
	}
	if result.NextVersion != "" {
		t.Errorf("NextVersion = %q for a repository with nothing to release", result.NextVersion)
	}
}

// A member that has never released starts at 0.0.0 rather than failing the group.
func TestAnUnreleasedMemberIsPlannedFromZero(t *testing.T) {
	repo := gitRepo(t, "", 0)
	commitIn(t, repo, "feat: the first thing")

	result, err := NewPlanner("v").Plan(context.Background(), repo)
	if err != nil {
		t.Fatalf("Plan on an unreleased repository: %v", err)
	}
	if result.CurrentVersion != "0.0.0" {
		t.Errorf("CurrentVersion = %q, want 0.0.0", result.CurrentVersion)
	}
	if result.NextVersion != "0.1.0" {
		t.Errorf("NextVersion = %q, want 0.1.0", result.NextVersion)
	}
}

// The configured prefix has to reach the planner too, or a group whose members tag
// "release-1.2.0" is planned from 0.0.0 — silently, and every member at once.
func TestThePlannerHonorsTheConfiguredPrefix(t *testing.T) {
	repo := gitRepo(t, "release-2.4.0", 0)
	commitIn(t, repo, "fix: correct the thing")

	result, err := NewPlanner("release-").Plan(context.Background(), repo)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if result.CurrentVersion != "2.4.0" || result.NextVersion != "2.4.1" {
		t.Errorf("planned %s -> %s, want 2.4.0 -> 2.4.1", result.CurrentVersion, result.NextVersion)
	}
}

// Releasing is refused, and the refusal has to say what is undecided rather than reading as a
// bug. Coordinator.Execute surfaces it.
func TestReleasingFromAGroupIsRefusedWithItsReason(t *testing.T) {
	_, err := NewPlanner("v").Release(context.Background(), "/repos/service-a")
	if err == nil {
		t.Fatal("Release returned no error; the pipeline is not implemented for another checkout")
	}
	if !strings.Contains(err.Error(), "approve") {
		t.Errorf("refusal %q does not name the decision that is missing, so it reads as a "+
			"defect rather than an unfinished feature", err)
	}
}
