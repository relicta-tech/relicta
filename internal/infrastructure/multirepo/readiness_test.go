package multirepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// `relicta group release` refused on the first member with "no release executor is
// configured" — a message about something being unimplemented, saying nothing about the
// operator's group. This reports what they were asking: what is blocking this release, and in
// which repository.
//
// It reads stored runs and nothing else. No container, no git service, no configuration —
// each of those resolves against the process working directory somewhere, and a component
// that silently answered for the invoking repository instead of the member would be worse
// than the refusal it replaced.

// runInState stores a release run at repoRoot and drives it to the given state.
func runInState(t *testing.T, repoRoot string, state domain.RunState) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoRoot, ".relicta"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	run := domain.NewReleaseRun(
		"acme/member", repoRoot, "main",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"},
		"config-hash", "plugin-hash",
	)

	// Reaching a state through the aggregate rather than by setting a field, so the stored
	// run is one the loader would accept.
	switch state {
	case domain.StateCanceled:
		if err := run.Cancel("changed my mind", "human:alice"); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	case domain.StateFailed:
		if err := run.MarkFailed("publish rejected the tarball", "human:alice"); err != nil {
			t.Fatalf("MarkFailed: %v", err)
		}
	}

	repo := adapters.NewFileReleaseRunRepository()
	ctx := context.Background()
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.SetLatest(ctx, repoRoot, run.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}
}

func checkOneMember(t *testing.T, repoRoot string) appmultirepo.MemberState {
	t.Helper()

	states := NewReadiness().Check(context.Background(), []appmultirepo.Member{
		{Name: "member", Path: repoRoot},
	})
	if len(states) != 1 {
		t.Fatalf("got %d states, want 1", len(states))
	}
	return states[0]
}

// The ordinary starting point: nobody has planned a release in that repository. It is not a
// failure, and saying so is the difference between an actionable message and an error.
func TestAMemberWithNoRunIsNotReadyAndSaysWhy(t *testing.T) {
	state := checkOneMember(t, t.TempDir())

	if state.Ready {
		t.Error("a repository with no planned release reported ready")
	}
	if !strings.Contains(state.Blocker, "relicta plan") {
		t.Errorf("blocker %q does not tell the operator what to run", state.Blocker)
	}
	// The path matters: in a group, "run relicta plan" without saying where is half an
	// instruction.
	if !strings.Contains(state.Blocker, t.TempDir()) && state.Path == "" {
		t.Error("the blocker names no repository, so an operator with six members cannot act on it")
	}
}

// Every state, including ones an aggregate cannot be driven into from a fresh run. What
// matters is that each sends the operator to the right command — "not ready" without saying
// which command is half an instruction — and that an unrecognized state blocks rather than
// permits.
func TestEveryStateNamesTheRightRemedy(t *testing.T) {
	const path = "../service-a"

	for name, tc := range map[string]struct {
		state     domain.RunState
		wantReady bool
		mentions  string
	}{
		"approved is ready":           {domain.StateApproved, true, ""},
		"publishing resumes":          {domain.StatePublishing, true, ""},
		"draft needs approval":        {domain.StateDraft, false, "approve"},
		"planned needs approval":      {domain.StatePlanned, false, "approve"},
		"published needs a new plan":  {domain.StatePublished, false, "plan"},
		"canceled needs a reset":      {domain.StateCanceled, false, "reset"},
		"failed needs retry or reset": {domain.StateFailed, false, "retry"},
		"an unknown state blocks":     {domain.RunState("invented-later"), false, "approve"},
	} {
		t.Run(name, func(t *testing.T) {
			ready, blocker := blockerFor(tc.state, path)

			if ready != tc.wantReady {
				t.Fatalf("ready = %v for state %q, want %v", ready, tc.state, tc.wantReady)
			}
			if ready {
				if blocker != "" {
					t.Errorf("a ready member carries a blocker: %q", blocker)
				}
				return
			}
			if !strings.Contains(blocker, tc.mentions) {
				t.Errorf("blocker for %q is %q, which does not mention %q",
					tc.state, blocker, tc.mentions)
			}
			// In a group, an instruction without a repository is half an instruction.
			if !strings.Contains(blocker, path) {
				t.Errorf("blocker for %q does not name the repository: %q", tc.state, blocker)
			}
		})
	}
}

// The store read itself: a real stored run is found and reported, rather than the command
// reporting "no plan" for a repository that has one.
func TestAStoredRunIsFoundAndReported(t *testing.T) {
	repoRoot := t.TempDir()
	runInState(t, repoRoot, domain.StateDraft)

	state := checkOneMember(t, repoRoot)
	if state.State == "" {
		t.Fatal("no state read back for a repository with a stored run; the readiness check " +
			"is not finding runs where they live")
	}
	if state.Ready {
		t.Errorf("a run in state %q reported ready", state.State)
	}
}

// AllReady is what the command gates on, and it has to report every blocked member rather
// than the first: an operator with six members wants one list, not six runs.
func TestAllReadyReportsEveryBlockedMember(t *testing.T) {
	states := []appmultirepo.MemberState{
		{Name: "a", Ready: true},
		{Name: "b", Ready: false, Blocker: "needs approval"},
		{Name: "c", Ready: false, Blocker: "no plan"},
	}

	ready, blocked := appmultirepo.AllReady(states)
	if ready {
		t.Error("AllReady = true with two blocked members")
	}
	if len(blocked) != 2 {
		t.Fatalf("got %d blocked members, want 2 — reporting only the first means as many "+
			"runs as there are problems", len(blocked))
	}

	allReady, none := appmultirepo.AllReady([]appmultirepo.MemberState{{Ready: true}, {Ready: true}})
	if !allReady || len(none) != 0 {
		t.Errorf("AllReady on two ready members = (%v, %d blocked), want (true, 0)", allReady, len(none))
	}
}
