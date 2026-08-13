// Package multirepo provides application services for multi-repository release coordination.
package multirepo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/multirepo"
)

// RepoReleaseState tracks the release state of an individual repository.
type RepoReleaseState string

const (
	// StatePending indicates the repo has not started releasing.
	StatePending RepoReleaseState = "pending"
	// StatePlanning indicates the repo is being analyzed.
	StatePlanning RepoReleaseState = "planning"
	// StateReleasing indicates the repo is being released.
	StateReleasing RepoReleaseState = "releasing"
	// StateReleased indicates the repo has been successfully released.
	StateReleased RepoReleaseState = "released"
	// StateSkipped indicates the repo had no changes and was skipped.
	StateSkipped RepoReleaseState = "skipped"
	// StateFailed indicates the repo release failed.
	StateFailed RepoReleaseState = "failed"
)

// RepoResult holds the result of planning or releasing a single repository.
type RepoResult struct {
	// Name is the repository name.
	Name string `json:"name"`
	// State is the current release state.
	State RepoReleaseState `json:"state"`
	// CurrentVersion is the version before release.
	CurrentVersion string `json:"current_version,omitempty"`
	// NextVersion is the calculated next version.
	NextVersion string `json:"next_version,omitempty"`
	// HasChanges indicates whether the repo has unreleased changes.
	HasChanges bool `json:"has_changes"`
	// ChangeCount is the number of changes detected.
	ChangeCount int `json:"change_count"`
	// Error contains any error message if the release failed.
	Error string `json:"error,omitempty"`
	// ReleasedAt is when the repo was released.
	ReleasedAt *time.Time `json:"released_at,omitempty"`
}

// MultiRepoPlan is the output of planning a multi-repo release.
type MultiRepoPlan struct {
	// GroupName is the repository group being released.
	GroupName string `json:"group_name"`
	// Strategy is the release strategy.
	Strategy multirepo.ReleaseStrategy `json:"strategy"`
	// ReleaseOrder is the topological order for releasing repos.
	ReleaseOrder []string `json:"release_order"`
	// Results maps repo names to their plan results.
	Results map[string]*RepoResult `json:"results"`
	// TotalChanges is the total number of changes across all repos.
	TotalChanges int `json:"total_changes"`
	// ReposWithChanges is the count of repos that have changes.
	ReposWithChanges int `json:"repos_with_changes"`
	// CreatedAt is when the plan was created.
	CreatedAt time.Time `json:"created_at"`
}

// GitAdapter abstracts git operations for a repository.
type GitAdapter interface {
	// HasChanges checks if the repository has unreleased changes.
	HasChanges(ctx context.Context, repoPath string) (bool, error)
	// GetCurrentVersion returns the current version of the repository.
	GetCurrentVersion(ctx context.Context, repoPath string) (string, error)
	// GetChangeCount returns the number of unreleased changes.
	GetChangeCount(ctx context.Context, repoPath string) (int, error)
}

// ReleaseExecutor executes a release for a single repository.
type ReleaseExecutor interface {
	// Plan analyzes a repository and returns what would be released.
	Plan(ctx context.Context, repoPath string) (*RepoResult, error)
	// Release performs the full release cycle: plan -> bump -> notes -> approve -> publish.
	Release(ctx context.Context, repoPath string) (*RepoResult, error)
}

// Coordinator orchestrates releases across multiple repositories in a group.
// It respects dependency ordering for coordinated releases and tracks
// cross-repo release state.
type Coordinator struct {
	gitAdapter      GitAdapter
	releaseExecutor ReleaseExecutor
	logger          *slog.Logger
}

// NewCoordinator creates a new multi-repo release coordinator.
func NewCoordinator(
	gitAdapter GitAdapter,
	releaseExecutor ReleaseExecutor,
) *Coordinator {
	return &Coordinator{
		gitAdapter:      gitAdapter,
		releaseExecutor: releaseExecutor,
		logger:          slog.Default().With("service", "multirepo_coordinator"),
	}
}

// Plan analyzes changes across all repositories in a group and creates
// a release plan. For coordinated strategy, repos are ordered by dependencies.
func (c *Coordinator) Plan(
	ctx context.Context,
	group *multirepo.RepositoryGroup,
	targetRepos ...string,
) (*MultiRepoPlan, error) {
	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("invalid repository group: %w", err)
	}

	graph, err := multirepo.NewDependencyGraph(group)
	if err != nil {
		return nil, fmt.Errorf("building dependency graph: %w", err)
	}

	releaseOrder, err := graph.ReleaseOrder()
	if err != nil {
		return nil, fmt.Errorf("computing release order: %w", err)
	}

	// Filter to target repos if specified.
	targetSet := makeTargetSet(targetRepos, group)

	plan := &MultiRepoPlan{
		GroupName:    group.Name,
		Strategy:     group.Strategy,
		ReleaseOrder: releaseOrder,
		Results:      make(map[string]*RepoResult, len(group.Repositories)),
		CreatedAt:    time.Now().UTC(),
	}

	for _, repoName := range releaseOrder {
		if len(targetSet) > 0 && !targetSet[repoName] {
			plan.Results[repoName] = &RepoResult{
				Name:  repoName,
				State: StateSkipped,
			}
			continue
		}

		repo := group.GetRepo(repoName)
		if repo == nil {
			continue
		}

		result, err := c.planRepo(ctx, repo)
		if err != nil {
			c.logger.Warn("failed to plan repository",
				"repo", repoName,
				"error", err,
			)
			result = &RepoResult{
				Name:  repoName,
				State: StateFailed,
				Error: err.Error(),
			}
		}

		plan.Results[repoName] = result

		if result.HasChanges {
			plan.ReposWithChanges++
			plan.TotalChanges += result.ChangeCount
		}
	}

	return plan, nil
}

// Execute releases repositories according to the plan, respecting dependency order.
// For coordinated strategy, it stops on failure. For independent strategy,
// it continues releasing other repos on failure.
func (c *Coordinator) Execute(
	ctx context.Context,
	group *multirepo.RepositoryGroup,
	plan *MultiRepoPlan,
) (*MultiRepoPlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("release plan is required")
	}

	for _, repoName := range plan.ReleaseOrder {
		result := plan.Results[repoName]
		if result == nil || result.State == StateSkipped || !result.HasChanges {
			continue
		}

		repo := group.GetRepo(repoName)
		if repo == nil {
			continue
		}

		c.logger.Info("releasing repository",
			"repo", repoName,
			"version", result.NextVersion,
		)

		result.State = StateReleasing

		if c.releaseExecutor == nil {
			// Refused rather than dereferenced. Plan works with a git adapter alone, so
			// a coordinator built for planning would otherwise panic here the moment
			// someone ran `group release` against it.
			return nil, fmt.Errorf("cannot release %q: no release executor is configured, "+
				"so this coordinator can plan a group but not release one", repo.Name)
		}
		releaseResult, err := c.releaseExecutor.Release(ctx, repo.Path)
		if err != nil {
			result.State = StateFailed
			result.Error = err.Error()

			c.logger.Error("repository release failed",
				"repo", repoName,
				"error", err,
			)

			// For coordinated releases, stop on failure since downstream
			// repos depend on this one.
			if group.Strategy == multirepo.StrategyCoordinated {
				return plan, fmt.Errorf("coordinated release failed at repository %q: %w", repoName, err)
			}
			continue
		}

		// Update result from release executor.
		if releaseResult != nil {
			result.State = releaseResult.State
			result.NextVersion = releaseResult.NextVersion
			result.ReleasedAt = releaseResult.ReleasedAt
		} else {
			result.State = StateReleased
			now := time.Now().UTC()
			result.ReleasedAt = &now
		}

		c.logger.Info("repository released successfully",
			"repo", repoName,
			"version", result.NextVersion,
		)
	}

	return plan, nil
}

// planRepo plans a release for a single repository.
func (c *Coordinator) planRepo(ctx context.Context, repo *multirepo.RepoConfig) (*RepoResult, error) {
	result := &RepoResult{
		Name:  repo.Name,
		State: StatePlanning,
	}

	// Check for changes.
	hasChanges, err := c.gitAdapter.HasChanges(ctx, repo.Path)
	if err != nil {
		return nil, fmt.Errorf("checking changes for %q: %w", repo.Name, err)
	}
	result.HasChanges = hasChanges

	if !hasChanges {
		result.State = StateSkipped
		return result, nil
	}

	// Get current version.
	currentVersion, err := c.gitAdapter.GetCurrentVersion(ctx, repo.Path)
	if err != nil {
		return nil, fmt.Errorf("getting version for %q: %w", repo.Name, err)
	}
	result.CurrentVersion = currentVersion

	// Get change count.
	changeCount, err := c.gitAdapter.GetChangeCount(ctx, repo.Path)
	if err != nil {
		return nil, fmt.Errorf("counting changes for %q: %w", repo.Name, err)
	}
	result.ChangeCount = changeCount

	// Use release executor for detailed planning if available.
	if c.releaseExecutor != nil {
		planResult, err := c.releaseExecutor.Plan(ctx, repo.Path)
		if err != nil {
			return nil, fmt.Errorf("planning release for %q: %w", repo.Name, err)
		}
		if planResult != nil {
			result.NextVersion = planResult.NextVersion
		}
	}

	result.State = StatePending
	return result, nil
}

// makeTargetSet creates a set of target repo names from the filter list.
// If targetRepos is empty, returns nil (meaning all repos are targeted).
func makeTargetSet(targetRepos []string, group *multirepo.RepositoryGroup) map[string]bool {
	if len(targetRepos) == 0 {
		return nil
	}

	set := make(map[string]bool, len(targetRepos))
	for _, name := range targetRepos {
		set[name] = true
	}

	// For coordinated releases, include all upstream dependencies of targeted repos.
	if group.Strategy == multirepo.StrategyCoordinated {
		graph, err := multirepo.NewDependencyGraph(group)
		if err != nil {
			return set // Fallback to just the target repos.
		}

		for _, name := range targetRepos {
			deps := graph.DirectDependencies(name)
			for _, dep := range deps {
				set[dep] = true
			}
		}
	}

	return set
}
