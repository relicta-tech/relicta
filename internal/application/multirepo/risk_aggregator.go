// Package multirepo provides application services for multi-repository release coordination.
package multirepo

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/multirepo"
)

// concurrentPenaltyPerRelease is added for each concurrent release with risk > 0.3.
const concurrentPenaltyPerRelease = 0.05

// dependencyPenalty is added when releasing a package and its dependents simultaneously.
const dependencyPenalty = 0.1

// highRiskThreshold is the minimum risk score for a release to incur concurrent penalty.
const highRiskThreshold = 0.3

// OrgRiskSnapshot captures the org-wide risk state at a point in time.
type OrgRiskSnapshot struct {
	// Timestamp is when the snapshot was taken.
	Timestamp time.Time `json:"timestamp"`
	// TotalRepos is the total number of repositories in the org.
	TotalRepos int `json:"total_repos"`
	// ActiveReleases is the count of repos currently releasing.
	ActiveReleases int `json:"active_releases"`
	// AggregateRisk is the combined risk score across the org.
	AggregateRisk float64 `json:"aggregate_risk"`
	// ConcurrentPenalty is the penalty applied for concurrent releases.
	ConcurrentPenalty float64 `json:"concurrent_penalty"`
	// RepoRisks lists risk entries for each tracked repo.
	RepoRisks []RepoRiskEntry `json:"repo_risks"`
	// Warnings contains advisory messages about the org risk state.
	Warnings []string `json:"warnings,omitempty"`
}

// RepoRiskEntry captures the risk state of a single repository.
type RepoRiskEntry struct {
	// Repository is the name of the repository.
	Repository string `json:"repository"`
	// Version is the version being released.
	Version string `json:"version"`
	// RiskScore is the assessed risk (0.0-1.0).
	RiskScore float64 `json:"risk_score"`
	// State is the release state (e.g., "planned", "releasing", "released").
	State string `json:"state"`
	// Dependencies lists the repo names this repo depends on.
	Dependencies []string `json:"dependencies,omitempty"`
}

// OrgRiskAggregatorOption configures the OrgRiskAggregator.
type OrgRiskAggregatorOption func(*OrgRiskAggregator)

// WithAggregatorLogger sets a custom logger.
func WithAggregatorLogger(logger *slog.Logger) OrgRiskAggregatorOption {
	return func(a *OrgRiskAggregator) {
		a.logger = logger
	}
}

// OrgRiskAggregator computes org-level risk from individual repo risks.
// It accounts for concurrent release penalties and dependency chain risks.
type OrgRiskAggregator struct {
	groups []multirepo.RepositoryGroup
	store  memory.Store
	logger *slog.Logger
}

// NewOrgRiskAggregator creates a new org risk aggregator.
func NewOrgRiskAggregator(
	groups []multirepo.RepositoryGroup,
	store memory.Store,
	opts ...OrgRiskAggregatorOption,
) *OrgRiskAggregator {
	a := &OrgRiskAggregator{
		groups: groups,
		store:  store,
		logger: slog.Default().With("service", "org_risk_aggregator"),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Snapshot returns the current org-wide risk state by querying the memory store
// for recent releases across all configured repository groups.
func (a *OrgRiskAggregator) Snapshot(ctx context.Context) (*OrgRiskSnapshot, error) {
	snapshot := &OrgRiskSnapshot{
		Timestamp: time.Now().UTC(),
		RepoRisks: make([]RepoRiskEntry, 0),
	}

	for _, group := range a.groups {
		for _, repo := range group.Repositories {
			snapshot.TotalRepos++

			releases, err := a.store.GetReleaseHistory(ctx, repo.Name, 1)
			if err != nil {
				a.logger.Debug("no release history for repo",
					"repo", repo.Name,
					"error", err,
				)
				continue
			}
			if len(releases) == 0 {
				continue
			}

			latest := releases[0]
			entry := RepoRiskEntry{
				Repository:   repo.Name,
				Version:      latest.Version,
				RiskScore:    latest.RiskScore,
				State:        string(latest.Outcome),
				Dependencies: repo.Dependencies,
			}

			// Consider releases within the last hour as "active".
			if time.Since(latest.ReleasedAt) < time.Hour {
				entry.State = "releasing"
				snapshot.ActiveReleases++
			} else {
				entry.State = "released"
			}

			snapshot.RepoRisks = append(snapshot.RepoRisks, entry)
		}
	}

	snapshot.AggregateRisk = a.AggregateRisk(snapshot.RepoRisks)
	snapshot.ConcurrentPenalty = a.computeConcurrentPenalty(snapshot.RepoRisks)

	// Generate warnings for elevated risk states.
	if snapshot.AggregateRisk > 0.7 {
		snapshot.Warnings = append(snapshot.Warnings,
			fmt.Sprintf("org aggregate risk is high: %.2f", snapshot.AggregateRisk))
	}
	if snapshot.ActiveReleases > 3 {
		snapshot.Warnings = append(snapshot.Warnings,
			fmt.Sprintf("%d concurrent releases in progress", snapshot.ActiveReleases))
	}

	return snapshot, nil
}

// AggregateRisk computes the org-wide aggregate risk score from individual repo risks.
//
// The algorithm:
//  1. Base score = max of individual repo risk scores.
//  2. Concurrent penalty: +0.05 per concurrent release with risk > 0.3.
//  3. Dependency penalty: +0.1 when releasing a package AND its dependents simultaneously.
//
// The result is clamped to [0.0, 1.0].
func (a *OrgRiskAggregator) AggregateRisk(repoRisks []RepoRiskEntry) float64 {
	if len(repoRisks) == 0 {
		return 0.0
	}

	// 1. Base: max of individual risks.
	maxRisk := 0.0
	for _, entry := range repoRisks {
		if entry.RiskScore > maxRisk {
			maxRisk = entry.RiskScore
		}
	}

	// 2. Concurrent penalty.
	concurrentPenalty := a.computeConcurrentPenalty(repoRisks)

	// 3. Dependency penalty.
	depPenalty := a.computeDependencyPenalty(repoRisks)

	total := maxRisk + concurrentPenalty + depPenalty
	return clampFloat(total, 0.0, 1.0)
}

// computeConcurrentPenalty returns +0.05 for each active release with risk > 0.3.
func (a *OrgRiskAggregator) computeConcurrentPenalty(repoRisks []RepoRiskEntry) float64 {
	activeHighRisk := 0
	for _, entry := range repoRisks {
		if isActiveState(entry.State) && entry.RiskScore > highRiskThreshold {
			activeHighRisk++
		}
	}
	// Penalty starts at the second concurrent high-risk release.
	if activeHighRisk <= 1 {
		return 0.0
	}
	return float64(activeHighRisk-1) * concurrentPenaltyPerRelease
}

// computeDependencyPenalty returns +0.1 for each case where a repo and one of
// its dependents are both actively releasing.
func (a *OrgRiskAggregator) computeDependencyPenalty(repoRisks []RepoRiskEntry) float64 {
	activeRepos := make(map[string]bool)
	for _, entry := range repoRisks {
		if isActiveState(entry.State) {
			activeRepos[entry.Repository] = true
		}
	}

	penaltyCount := 0
	seen := make(map[string]bool)
	for _, entry := range repoRisks {
		if !activeRepos[entry.Repository] {
			continue
		}
		for _, dep := range entry.Dependencies {
			if activeRepos[dep] {
				// Use a sorted pair key to avoid double-counting.
				pair := depPairKey(entry.Repository, dep)
				if !seen[pair] {
					seen[pair] = true
					penaltyCount++
				}
			}
		}
	}

	return float64(penaltyCount) * dependencyPenalty
}

// CheckOrgBudget evaluates whether a new release can proceed given the current
// org-level risk state and the configured risk budget.
//
// It returns true if the release is allowed, along with a reason string.
func (a *OrgRiskAggregator) CheckOrgBudget(
	ctx context.Context,
	newRelease RepoRiskEntry,
	budget *config.RiskBudgetConfig,
) (bool, string) {
	if budget == nil {
		return true, "no risk budget configured"
	}

	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		a.logger.Warn("failed to get org snapshot for budget check", "error", err)
		return true, "could not assess org risk; allowing release"
	}

	// Check concurrent release limit.
	if budget.ConcurrentLimit > 0 {
		// Count current active releases plus this new one.
		activeCount := snapshot.ActiveReleases + 1
		if activeCount > budget.ConcurrentLimit {
			return false, fmt.Sprintf(
				"concurrent release limit exceeded: %d active (limit %d)",
				activeCount, budget.ConcurrentLimit,
			)
		}
	}

	// Check weekly risk budget by projecting the new aggregate.
	if budget.WeeklyLimit > 0 {
		projected := make([]RepoRiskEntry, len(snapshot.RepoRisks), len(snapshot.RepoRisks)+1)
		copy(projected, snapshot.RepoRisks)
		projected = append(projected, newRelease)

		projectedRisk := a.AggregateRisk(projected)
		if projectedRisk > budget.WeeklyLimit {
			return false, fmt.Sprintf(
				"org aggregate risk %.2f would exceed weekly limit %.2f",
				projectedRisk, budget.WeeklyLimit,
			)
		}
	}

	return true, "within org risk budget"
}

// isActiveState returns true if the state represents an in-progress release.
func isActiveState(state string) bool {
	switch state {
	case "planned", "releasing", "planning":
		return true
	default:
		return false
	}
}

// depPairKey returns a deterministic key for a dependency pair.
func depPairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}

// clampFloat restricts v to the range [minVal, maxVal].
func clampFloat(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// SortRepoRisks sorts entries by repository name for deterministic output.
func SortRepoRisks(entries []RepoRiskEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Repository < entries[j].Repository
	})
}
