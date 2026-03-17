package multirepo

import (
	"fmt"
	"strings"
)

// DependencyGraph represents a directed acyclic graph (DAG) of repository
// dependencies within a repository group. It provides topological ordering
// for coordinated releases and affected-repo analysis for change propagation.
type DependencyGraph struct {
	// nodes maps repo name to its direct dependencies (outgoing edges).
	nodes map[string][]string
	// reverse maps repo name to repos that depend on it (incoming edges).
	reverse map[string][]string
}

// NewDependencyGraph builds a dependency graph from a repository group.
// Returns an error if the group contains cycles (use Validate first to
// get a more descriptive error message).
func NewDependencyGraph(group *RepositoryGroup) (*DependencyGraph, error) {
	g := &DependencyGraph{
		nodes:   make(map[string][]string, len(group.Repositories)),
		reverse: make(map[string][]string, len(group.Repositories)),
	}

	for _, repo := range group.Repositories {
		g.nodes[repo.Name] = repo.Dependencies
		for _, dep := range repo.Dependencies {
			g.reverse[dep] = append(g.reverse[dep], repo.Name)
		}
		// Ensure every node exists in both maps.
		if _, ok := g.reverse[repo.Name]; !ok {
			g.reverse[repo.Name] = nil
		}
	}

	// Verify no cycles exist.
	if _, err := g.topologicalSort(); err != nil {
		return nil, err
	}

	return g, nil
}

// ReleaseOrder returns the repositories in topological order, meaning
// dependencies come before dependents. This is the correct order for
// coordinated releases: release upstream repos first.
func (g *DependencyGraph) ReleaseOrder() ([]string, error) {
	return g.topologicalSort()
}

// AffectedRepos returns all repositories that are transitively downstream
// of the given repository. When changedRepo releases a new version,
// all returned repos may need updates or re-releases.
// The changedRepo itself is not included in the result.
func (g *DependencyGraph) AffectedRepos(changedRepo string) ([]string, error) {
	if _, ok := g.nodes[changedRepo]; !ok {
		return nil, fmt.Errorf("repository %q not found in dependency graph", changedRepo)
	}

	affected := make(map[string]bool)
	g.collectDownstream(changedRepo, affected)

	// Remove the changed repo itself.
	delete(affected, changedRepo)

	result := make([]string, 0, len(affected))
	for name := range affected {
		result = append(result, name)
	}

	// Return in topological order for consistency.
	order, err := g.topologicalSort()
	if err != nil {
		return result, nil // Fallback to unordered if sort fails.
	}

	ordered := make([]string, 0, len(affected))
	for _, name := range order {
		if affected[name] {
			ordered = append(ordered, name)
		}
	}

	return ordered, nil
}

// DirectDependencies returns the repositories that the given repo directly depends on.
func (g *DependencyGraph) DirectDependencies(repo string) []string {
	return g.nodes[repo]
}

// DirectDependents returns the repositories that directly depend on the given repo.
func (g *DependencyGraph) DirectDependents(repo string) []string {
	return g.reverse[repo]
}

// Roots returns repositories that have no dependencies (the starting points
// for a coordinated release).
func (g *DependencyGraph) Roots() []string {
	var roots []string
	for name, deps := range g.nodes {
		if len(deps) == 0 {
			roots = append(roots, name)
		}
	}
	return roots
}

// Leaves returns repositories that nothing depends on (the end points
// of the dependency chain).
func (g *DependencyGraph) Leaves() []string {
	var leaves []string
	for name, dependents := range g.reverse {
		if len(dependents) == 0 {
			leaves = append(leaves, name)
		}
	}
	return leaves
}

// Nodes returns all repository names in the graph.
func (g *DependencyGraph) Nodes() []string {
	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	return names
}

// topologicalSort performs Kahn's algorithm to produce a topological ordering.
func (g *DependencyGraph) topologicalSort() ([]string, error) {
	// Calculate in-degree for each node.
	inDegree := make(map[string]int, len(g.nodes))
	for name := range g.nodes {
		inDegree[name] = 0
	}
	for _, deps := range g.nodes {
		for _, dep := range deps {
			inDegree[dep]++ // Inverse: dep is depended upon, but we track who depends on what.
		}
	}

	// Actually, in a dependency graph where A depends on B:
	// A -> B (A's dependencies list includes B)
	// For topological sort, B must come before A.
	// In-degree should count how many repos depend on this one.
	inDegree = make(map[string]int, len(g.nodes))
	for name := range g.nodes {
		inDegree[name] = len(g.nodes[name]) // How many deps this node has.
	}

	// Start with nodes that have no dependencies (in-degree 0).
	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// For each repo that depends on this node, decrement its count.
		for _, dependent := range g.reverse[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(g.nodes) {
		// Find the cycle for error reporting.
		var unresolved []string
		for name, degree := range inDegree {
			if degree > 0 {
				unresolved = append(unresolved, name)
			}
		}
		return nil, fmt.Errorf("circular dependency detected among repositories: %s",
			strings.Join(unresolved, ", "))
	}

	return order, nil
}

// collectDownstream performs BFS to find all transitive dependents.
func (g *DependencyGraph) collectDownstream(repo string, visited map[string]bool) {
	if visited[repo] {
		return
	}
	visited[repo] = true

	for _, dependent := range g.reverse[repo] {
		g.collectDownstream(dependent, visited)
	}
}
