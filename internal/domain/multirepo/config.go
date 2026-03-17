// Package multirepo provides domain model for multi-repository governance.
//
// It enables coordinated release management across multiple repositories,
// supporting independent and coordinated release strategies with
// dependency-aware ordering and CGP federation.
package multirepo

import (
	"fmt"
	"strings"
)

// ReleaseStrategy defines how repositories in a group are released.
type ReleaseStrategy string

const (
	// StrategyIndependent allows each repository to be released independently.
	StrategyIndependent ReleaseStrategy = "independent"
	// StrategyCoordinated releases repositories together in dependency order.
	StrategyCoordinated ReleaseStrategy = "coordinated"
)

// IsValid returns true if the release strategy is recognized.
func (s ReleaseStrategy) IsValid() bool {
	switch s {
	case StrategyIndependent, StrategyCoordinated:
		return true
	default:
		return false
	}
}

// RepositoryGroup defines a collection of related repositories that are
// managed together for governance and release coordination.
type RepositoryGroup struct {
	// Name is the unique identifier for this group.
	Name string `mapstructure:"name" json:"name" yaml:"name"`
	// Repositories is the list of repositories in this group.
	Repositories []RepoConfig `mapstructure:"repositories" json:"repositories" yaml:"repositories"`
	// Strategy defines how repositories are released (independent or coordinated).
	Strategy ReleaseStrategy `mapstructure:"strategy" json:"strategy" yaml:"strategy"`
}

// RepoConfig defines configuration for a single repository within a group.
type RepoConfig struct {
	// Name is the short identifier for this repository (e.g., "auth-service").
	Name string `mapstructure:"name" json:"name" yaml:"name"`
	// URL is the remote git URL (e.g., "https://github.com/org/repo.git").
	URL string `mapstructure:"url" json:"url" yaml:"url"`
	// Path is the local filesystem path to the repository checkout.
	Path string `mapstructure:"path" json:"path" yaml:"path"`
	// Dependencies lists names of other repos in this group that this repo depends on.
	Dependencies []string `mapstructure:"dependencies" json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	// VersionConstraint is an optional semver constraint for dependency compatibility.
	VersionConstraint string `mapstructure:"version_constraint" json:"version_constraint,omitempty" yaml:"version_constraint,omitempty"`
}

// Validate checks the repository group configuration for correctness.
// It verifies:
//   - Group name is not empty
//   - At least one repository is defined
//   - Strategy is valid
//   - All repo names are unique
//   - All dependency references point to existing repos in the group
//   - No circular dependencies exist
func (g *RepositoryGroup) Validate() error {
	if strings.TrimSpace(g.Name) == "" {
		return fmt.Errorf("repository group name is required")
	}
	if len(g.Repositories) == 0 {
		return fmt.Errorf("repository group %q must contain at least one repository", g.Name)
	}
	if !g.Strategy.IsValid() {
		return fmt.Errorf("repository group %q has invalid strategy %q", g.Name, g.Strategy)
	}

	// Build set of known repo names and check uniqueness.
	names := make(map[string]bool, len(g.Repositories))
	for _, repo := range g.Repositories {
		if strings.TrimSpace(repo.Name) == "" {
			return fmt.Errorf("repository in group %q has empty name", g.Name)
		}
		if names[repo.Name] {
			return fmt.Errorf("duplicate repository name %q in group %q", repo.Name, g.Name)
		}
		names[repo.Name] = true
	}

	// Validate all dependency references exist.
	for _, repo := range g.Repositories {
		for _, dep := range repo.Dependencies {
			if !names[dep] {
				return fmt.Errorf("repository %q in group %q depends on unknown repository %q", repo.Name, g.Name, dep)
			}
			if dep == repo.Name {
				return fmt.Errorf("repository %q in group %q cannot depend on itself", repo.Name, g.Name)
			}
		}
	}

	// Check for circular dependencies.
	return g.detectCycles()
}

// detectCycles checks for circular dependencies using DFS.
func (g *RepositoryGroup) detectCycles() error {
	// Build adjacency list.
	adj := make(map[string][]string, len(g.Repositories))
	for _, repo := range g.Repositories {
		adj[repo.Name] = repo.Dependencies
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int, len(g.Repositories))
	path := make([]string, 0)

	var dfs func(node string) error
	dfs = func(node string) error {
		state[node] = visiting
		path = append(path, node)

		for _, dep := range adj[node] {
			switch state[dep] {
			case visiting:
				// Found a cycle. Build the cycle path for a clear error message.
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				cycle := append(path[cycleStart:], dep)
				return fmt.Errorf("circular dependency detected in group %q: %s",
					g.Name, strings.Join(cycle, " -> "))
			case unvisited:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}

		state[node] = visited
		path = path[:len(path)-1]
		return nil
	}

	for _, repo := range g.Repositories {
		if state[repo.Name] == unvisited {
			if err := dfs(repo.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRepo returns the repository config by name, or nil if not found.
func (g *RepositoryGroup) GetRepo(name string) *RepoConfig {
	for i := range g.Repositories {
		if g.Repositories[i].Name == name {
			return &g.Repositories[i]
		}
	}
	return nil
}

// RepoNames returns all repository names in the group.
func (g *RepositoryGroup) RepoNames() []string {
	names := make([]string, len(g.Repositories))
	for i, repo := range g.Repositories {
		names[i] = repo.Name
	}
	return names
}
