package multirepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/multirepo"
)

// mockGitAdapter implements GitAdapter for testing.
type mockGitAdapter struct {
	changes  map[string]bool
	versions map[string]string
	counts   map[string]int
	failFor  map[string]bool
}

func newMockGitAdapter() *mockGitAdapter {
	return &mockGitAdapter{
		changes:  make(map[string]bool),
		versions: make(map[string]string),
		counts:   make(map[string]int),
		failFor:  make(map[string]bool),
	}
}

func (m *mockGitAdapter) HasChanges(_ context.Context, repoPath string) (bool, error) {
	if m.failFor[repoPath] {
		return false, fmt.Errorf("git error for %s", repoPath)
	}
	return m.changes[repoPath], nil
}

func (m *mockGitAdapter) GetCurrentVersion(_ context.Context, repoPath string) (string, error) {
	if m.failFor[repoPath] {
		return "", fmt.Errorf("git error for %s", repoPath)
	}
	v, ok := m.versions[repoPath]
	if !ok {
		return "0.0.0", nil
	}
	return v, nil
}

func (m *mockGitAdapter) GetChangeCount(_ context.Context, repoPath string) (int, error) {
	if m.failFor[repoPath] {
		return 0, fmt.Errorf("git error for %s", repoPath)
	}
	return m.counts[repoPath], nil
}

// mockReleaseExecutor implements ReleaseExecutor for testing.
type mockReleaseExecutor struct {
	planResults    map[string]*RepoResult
	releaseResults map[string]*RepoResult
	failPlan       map[string]bool
	failRelease    map[string]bool
}

func newMockReleaseExecutor() *mockReleaseExecutor {
	return &mockReleaseExecutor{
		planResults:    make(map[string]*RepoResult),
		releaseResults: make(map[string]*RepoResult),
		failPlan:       make(map[string]bool),
		failRelease:    make(map[string]bool),
	}
}

func (m *mockReleaseExecutor) Plan(_ context.Context, repoPath string) (*RepoResult, error) {
	if m.failPlan[repoPath] {
		return nil, fmt.Errorf("plan failed for %s", repoPath)
	}
	if result, ok := m.planResults[repoPath]; ok {
		return result, nil
	}
	return &RepoResult{NextVersion: "1.0.0"}, nil
}

func (m *mockReleaseExecutor) Release(_ context.Context, repoPath string) (*RepoResult, error) {
	if m.failRelease[repoPath] {
		return nil, fmt.Errorf("release failed for %s", repoPath)
	}
	if result, ok := m.releaseResults[repoPath]; ok {
		return result, nil
	}
	return &RepoResult{State: StateReleased, NextVersion: "1.0.0"}, nil
}

func makeCoordinatedGroup() *multirepo.RepositoryGroup {
	return &multirepo.RepositoryGroup{
		Name:     "platform",
		Strategy: multirepo.StrategyCoordinated,
		Repositories: []multirepo.RepoConfig{
			{Name: "core-lib", Path: "/repos/core"},
			{Name: "auth-service", Path: "/repos/auth", Dependencies: []string{"core-lib"}},
			{Name: "api-gateway", Path: "/repos/api", Dependencies: []string{"auth-service"}},
		},
	}
}

func makeIndependentGroup() *multirepo.RepositoryGroup {
	return &multirepo.RepositoryGroup{
		Name:     "services",
		Strategy: multirepo.StrategyIndependent,
		Repositories: []multirepo.RepoConfig{
			{Name: "service-a", Path: "/repos/svc-a"},
			{Name: "service-b", Path: "/repos/svc-b"},
			{Name: "service-c", Path: "/repos/svc-c"},
		},
	}
}

func TestCoordinator_Plan_AllWithChanges(t *testing.T) {
	git := newMockGitAdapter()
	git.changes["/repos/core"] = true
	git.changes["/repos/auth"] = true
	git.changes["/repos/api"] = true
	git.versions["/repos/core"] = "1.0.0"
	git.versions["/repos/auth"] = "2.0.0"
	git.versions["/repos/api"] = "3.0.0"
	git.counts["/repos/core"] = 5
	git.counts["/repos/auth"] = 3
	git.counts["/repos/api"] = 2

	executor := newMockReleaseExecutor()
	coord := NewCoordinator(git, executor)

	plan, err := coord.Plan(context.Background(), makeCoordinatedGroup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.GroupName != "platform" {
		t.Errorf("expected group name 'platform', got %q", plan.GroupName)
	}
	if plan.ReposWithChanges != 3 {
		t.Errorf("expected 3 repos with changes, got %d", plan.ReposWithChanges)
	}
	if plan.TotalChanges != 10 {
		t.Errorf("expected 10 total changes, got %d", plan.TotalChanges)
	}

	// Verify release order respects dependencies.
	coreIdx := indexOf(plan.ReleaseOrder, "core-lib")
	authIdx := indexOf(plan.ReleaseOrder, "auth-service")
	apiIdx := indexOf(plan.ReleaseOrder, "api-gateway")

	if coreIdx >= authIdx || authIdx >= apiIdx {
		t.Errorf("invalid release order: %v", plan.ReleaseOrder)
	}
}

func TestCoordinator_Plan_NoChanges(t *testing.T) {
	git := newMockGitAdapter()
	// No changes in any repo.
	executor := newMockReleaseExecutor()
	coord := NewCoordinator(git, executor)

	plan, err := coord.Plan(context.Background(), makeIndependentGroup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ReposWithChanges != 0 {
		t.Errorf("expected 0 repos with changes, got %d", plan.ReposWithChanges)
	}
}

func TestCoordinator_Plan_PartialChanges(t *testing.T) {
	git := newMockGitAdapter()
	git.changes["/repos/svc-a"] = true
	git.versions["/repos/svc-a"] = "1.0.0"
	git.counts["/repos/svc-a"] = 3
	// svc-b and svc-c have no changes.

	executor := newMockReleaseExecutor()
	coord := NewCoordinator(git, executor)

	plan, err := coord.Plan(context.Background(), makeIndependentGroup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ReposWithChanges != 1 {
		t.Errorf("expected 1 repo with changes, got %d", plan.ReposWithChanges)
	}
	if plan.TotalChanges != 3 {
		t.Errorf("expected 3 total changes, got %d", plan.TotalChanges)
	}
}

func TestCoordinator_Plan_TargetRepos(t *testing.T) {
	git := newMockGitAdapter()
	git.changes["/repos/svc-a"] = true
	git.changes["/repos/svc-b"] = true
	git.changes["/repos/svc-c"] = true
	git.versions["/repos/svc-a"] = "1.0.0"
	git.versions["/repos/svc-b"] = "2.0.0"
	git.versions["/repos/svc-c"] = "3.0.0"
	git.counts["/repos/svc-a"] = 1
	git.counts["/repos/svc-b"] = 2
	git.counts["/repos/svc-c"] = 3

	executor := newMockReleaseExecutor()
	coord := NewCoordinator(git, executor)

	plan, err := coord.Plan(context.Background(), makeIndependentGroup(), "service-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only service-a should be planned.
	if plan.ReposWithChanges != 1 {
		t.Errorf("expected 1 repo with changes, got %d", plan.ReposWithChanges)
	}

	result := plan.Results["service-a"]
	if result == nil || !result.HasChanges {
		t.Error("service-a should have changes")
	}

	// Other repos should be skipped.
	for _, name := range []string{"service-b", "service-c"} {
		result := plan.Results[name]
		if result == nil {
			t.Errorf("expected result for %q", name)
			continue
		}
		if result.State != StateSkipped {
			t.Errorf("expected %q to be skipped, got %s", name, result.State)
		}
	}
}

func TestCoordinator_Plan_InvalidGroup(t *testing.T) {
	coord := NewCoordinator(newMockGitAdapter(), newMockReleaseExecutor())

	_, err := coord.Plan(context.Background(), &multirepo.RepositoryGroup{
		Name:         "",
		Strategy:     "invalid",
		Repositories: nil,
	})
	if err == nil {
		t.Fatal("expected error for invalid group")
	}
}

func TestCoordinator_Plan_GitError(t *testing.T) {
	git := newMockGitAdapter()
	git.failFor["/repos/svc-a"] = true

	coord := NewCoordinator(git, newMockReleaseExecutor())

	plan, err := coord.Plan(context.Background(), makeIndependentGroup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The failed repo should have an error state.
	result := plan.Results["service-a"]
	if result == nil {
		t.Fatal("expected result for service-a")
	}
	if result.State != StateFailed {
		t.Errorf("expected failed state, got %s", result.State)
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestCoordinator_Execute_IndependentContinuesOnFailure(t *testing.T) {
	git := newMockGitAdapter()
	git.changes["/repos/svc-a"] = true
	git.changes["/repos/svc-b"] = true
	git.versions["/repos/svc-a"] = "1.0.0"
	git.versions["/repos/svc-b"] = "2.0.0"
	git.counts["/repos/svc-a"] = 1
	git.counts["/repos/svc-b"] = 1

	executor := newMockReleaseExecutor()
	executor.failRelease["/repos/svc-a"] = true

	coord := NewCoordinator(git, executor)
	group := makeIndependentGroup()

	plan, err := coord.Plan(context.Background(), group)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	plan, err = coord.Execute(context.Background(), group, plan)
	// Independent strategy should not return error.
	if err != nil {
		t.Fatalf("unexpected error for independent release: %v", err)
	}

	// service-a should be failed.
	resultA := plan.Results["service-a"]
	if resultA.State != StateFailed {
		t.Errorf("expected service-a to be failed, got %s", resultA.State)
	}

	// service-b should be released (or pending if it was ordered after).
	resultB := plan.Results["service-b"]
	if resultB.State != StateReleased && resultB.State != StatePending {
		t.Errorf("expected service-b state Released or Pending, got %s", resultB.State)
	}
}

func TestCoordinator_Execute_CoordinatedStopsOnFailure(t *testing.T) {
	git := newMockGitAdapter()
	git.changes["/repos/core"] = true
	git.changes["/repos/auth"] = true
	git.changes["/repos/api"] = true
	git.versions["/repos/core"] = "1.0.0"
	git.versions["/repos/auth"] = "2.0.0"
	git.versions["/repos/api"] = "3.0.0"
	git.counts["/repos/core"] = 1
	git.counts["/repos/auth"] = 1
	git.counts["/repos/api"] = 1

	executor := newMockReleaseExecutor()
	executor.failRelease["/repos/core"] = true

	coord := NewCoordinator(git, executor)
	group := makeCoordinatedGroup()

	plan, err := coord.Plan(context.Background(), group)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	_, err = coord.Execute(context.Background(), group, plan)
	if err == nil {
		t.Fatal("expected error for coordinated release failure")
	}
	if !strings.Contains(err.Error(), "core-lib") {
		t.Errorf("error should mention failing repo, got: %v", err)
	}
}

func TestCoordinator_Execute_NilPlan(t *testing.T) {
	coord := NewCoordinator(newMockGitAdapter(), newMockReleaseExecutor())

	_, err := coord.Execute(context.Background(), makeIndependentGroup(), nil)
	if err == nil {
		t.Fatal("expected error for nil plan")
	}
}

// indexOf returns the index of s in slice, or -1.
func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
