package app

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Planning failed outright in a repository with no version tags — every project's
// first release. The baseline resolution sets baseRef to "" when no tag is found,
// which the git service then tried to resolve as a reference. Fixed there; these
// tests cover the use case's half: recognizing the situation and reporting it.
//
// Two situations produce no tag and both are legitimately a first release: no tags
// at all, and tags that all carry a different prefix — a monorepo at its first
// `web-v` release while `app-v` tags already exist.

func planUseCaseWithInspector(t *testing.T, inspector ports.RepoInspector) *PlanReleaseUseCase {
	t.Helper()

	// nil state machine, matching plan_idempotency_test.go: Execute does not use it
	// on this path, and supplying one would only add setup.
	return NewPlanReleaseUseCase(newMockRepository(), inspector, nil)
}

func firstReleaseInput(t *testing.T) PlanReleaseInput {
	t.Helper()

	return PlanReleaseInput{
		RepoRoot: t.TempDir(),
		RepoID:   "owner/repo",
		Actor:    ports.ActorInfo{Type: "user", ID: "cli"},
	}
}

func TestPlanReportsAFirstReleaseWhenNoTagExists(t *testing.T) {
	inspector := &mockRepoInspector{
		headSHA: "abc123",
		commits: []domain.CommitSHA{"abc123", "def456"},
		// No tag, reported as an error — how the git inspector answers when nothing
		// matches.
		latestTagErr: errors.New("no version tags found"),
	}

	uc := planUseCaseWithInspector(t, inspector)
	out, err := uc.Execute(context.Background(), firstReleaseInput(t))
	if err != nil {
		t.Fatalf("planning a first release must succeed: %v", err)
	}
	if !out.FirstRelease {
		t.Error("FirstRelease must be true when no previous release was found; a caller " +
			"cannot otherwise tell a whole-history changeset from an ordinary one")
	}
}

// A tag lookup that succeeds with an empty string is the same situation. Treating
// only the error case as "first release" would leave this one setting baseRef to ""
// without saying so.
func TestPlanReportsAFirstReleaseWhenTheTagIsEmpty(t *testing.T) {
	inspector := &mockRepoInspector{
		headSHA:   "abc123",
		commits:   []domain.CommitSHA{"abc123"},
		latestTag: "",
	}

	uc := planUseCaseWithInspector(t, inspector)
	out, err := uc.Execute(context.Background(), firstReleaseInput(t))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.FirstRelease {
		t.Error("an empty tag is no previous release")
	}
}

func TestPlanDoesNotReportAFirstReleaseWhenATagExists(t *testing.T) {
	inspector := &mockRepoInspector{
		headSHA:   "abc123",
		commits:   []domain.CommitSHA{"abc123"},
		latestTag: "v1.2.3",
	}

	uc := planUseCaseWithInspector(t, inspector)
	out, err := uc.Execute(context.Background(), firstReleaseInput(t))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.FirstRelease {
		t.Error("v1.2.3 is a previous release, so this is not a first release")
	}
}

// An explicit BaseRef is the caller's answer and must not be overridden, nor
// reported as a first release.
func TestPlanWithAnExplicitBaseRefIsNotAFirstRelease(t *testing.T) {
	inspector := &mockRepoInspector{
		headSHA:      "abc123",
		commits:      []domain.CommitSHA{"abc123"},
		latestTagErr: errors.New("no version tags found"),
	}

	in := firstReleaseInput(t)
	in.BaseRef = "v1.0.0"

	uc := planUseCaseWithInspector(t, inspector)
	out, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.FirstRelease {
		t.Error("the caller supplied a baseline, so this is not a first release")
	}
}
