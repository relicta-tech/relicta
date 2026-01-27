package release

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

func TestNewReleaseRunForTest(t *testing.T) {
	run := NewReleaseRunForTest("test-run-id", "main", "/tmp/repo")

	if run == nil {
		t.Fatal("NewReleaseRunForTest returned nil")
	}

	if string(run.ID()) != "test-run-id" {
		t.Errorf("ID() = %s, want test-run-id", run.ID())
	}

	if run.RepoRoot() != "/tmp/repo" {
		t.Errorf("RepoRoot() = %s, want /tmp/repo", run.RepoRoot())
	}

	if run.State() != domain.StateDraft {
		t.Errorf("State() = %s, want %s", run.State(), domain.StateDraft)
	}

	// Should have emitted a creation event
	events := run.DomainEvents()
	if len(events) == 0 {
		t.Error("Expected creation event to be emitted")
	}
}

func TestNewReleaseRunForTestWithCommits(t *testing.T) {
	run := NewReleaseRunForTestWithCommits("test-run-with-commits", "main", "/tmp/repo")

	if run == nil {
		t.Fatal("NewReleaseRunForTestWithCommits returned nil")
	}

	if string(run.ID()) != "test-run-with-commits" {
		t.Errorf("ID() = %s, want test-run-with-commits", run.ID())
	}

	// Should have commits
	commits := run.Commits()
	if len(commits) == 0 {
		t.Error("Expected commits to be set")
	}

	// Should have a changeset
	if !run.HasChangeSet() {
		t.Error("Expected changeset to be set")
	}

	// Should be in planned state (SetPlan was called)
	if run.State() != domain.StatePlanned {
		t.Errorf("State() = %s, want %s", run.State(), domain.StatePlanned)
	}
}

func TestTestCommit_Constant(t *testing.T) {
	if TestCommit == "" {
		t.Error("TestCommit should not be empty")
	}

	if string(TestCommit) != "abc123def456" {
		t.Errorf("TestCommit = %s, want abc123def456", TestCommit)
	}
}
