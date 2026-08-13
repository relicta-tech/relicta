package multirepo

import (
	"context"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/multirepo"
)

// The CLI built this coordinator with NewCoordinator(nil, nil), and both nils were
// dereferenced: planRepo on the git adapter, Release on the executor. So `relicta group
// plan` panicked on the first repository in the group rather than reporting anything.
//
// The git adapter is now supplied. The release executor is deliberately still absent —
// running a full release inside another checkout is separate work — so the coordinator must
// refuse a release it cannot perform instead of crashing on it. A panic in a governance tool
// is worse than a refusal: it leaves no record of what was attempted.

type stubGitAdapter struct{}

func (stubGitAdapter) HasChanges(context.Context, string) (bool, error)          { return true, nil }
func (stubGitAdapter) GetCurrentVersion(context.Context, string) (string, error) { return "1.0.0", nil }
func (stubGitAdapter) GetChangeCount(context.Context, string) (int, error)       { return 2, nil }

func groupOfOne() *multirepo.RepositoryGroup {
	return &multirepo.RepositoryGroup{
		Name:     "platform",
		Strategy: multirepo.StrategyIndependent,
		Repositories: []multirepo.RepoConfig{
			{Name: "service-a", Path: "../service-a"},
		},
	}
}

func TestExecuteRefusesWithoutAnExecutorRatherThanPanicking(t *testing.T) {
	coordinator := NewCoordinator(stubGitAdapter{}, nil)

	group := groupOfOne()
	ctx := context.Background()

	// Execute takes the plan Plan produced, which is the real sequence: planning works
	// with the git adapter alone, and only executing needs the executor.
	plan, err := coordinator.Plan(ctx, group)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	_, err = coordinator.Execute(ctx, group, plan)
	if err == nil {
		t.Fatal("Execute with no executor returned no error; it used to dereference nil and " +
			"take the process down instead")
	}

	// The message has to say what is missing, or the operator cannot tell a
	// misconfiguration from a bug.
	if !strings.Contains(err.Error(), "executor") {
		t.Errorf("error %q does not explain that no release executor is configured", err)
	}
}

// Planning must work with the git adapter alone — that is the case the CLI actually runs,
// and the one that panicked.
func TestPlanWorksWithTheGitAdapterAlone(t *testing.T) {
	coordinator := NewCoordinator(stubGitAdapter{}, nil)

	plan, err := coordinator.Plan(context.Background(), groupOfOne())
	if err != nil {
		t.Fatalf("Plan with a git adapter and no executor: %v", err)
	}
	if plan == nil {
		t.Fatal("Plan returned no plan and no error")
	}
	if plan.ReposWithChanges != 1 {
		t.Errorf("ReposWithChanges = %d, want 1", plan.ReposWithChanges)
	}
}
