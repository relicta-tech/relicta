package multirepo

import (
	"strings"
	"testing"
)

func makeTestGroup(repos []RepoConfig) *RepositoryGroup {
	return &RepositoryGroup{
		Name:         "test-group",
		Strategy:     StrategyCoordinated,
		Repositories: repos,
	}
}

func TestNewDependencyGraph_Success(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "core"},
		{Name: "auth", Dependencies: []string{"core"}},
		{Name: "api", Dependencies: []string{"auth", "core"}},
	})

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestNewDependencyGraph_Cycle(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "a", Dependencies: []string{"b"}},
		{Name: "b", Dependencies: []string{"a"}},
	})

	_, err := NewDependencyGraph(group)
	if err == nil {
		t.Fatal("expected error for cyclic graph")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("error %q should mention circular dependency", err.Error())
	}
}

func TestDependencyGraph_ReleaseOrder(t *testing.T) {
	tests := []struct {
		name     string
		repos    []RepoConfig
		wantLen  int
		validate func(order []string) bool
	}{
		{
			name: "linear chain",
			repos: []RepoConfig{
				{Name: "core"},
				{Name: "auth", Dependencies: []string{"core"}},
				{Name: "api", Dependencies: []string{"auth"}},
			},
			wantLen: 3,
			validate: func(order []string) bool {
				return indexOf(order, "core") < indexOf(order, "auth") &&
					indexOf(order, "auth") < indexOf(order, "api")
			},
		},
		{
			name: "diamond dependency",
			repos: []RepoConfig{
				{Name: "core"},
				{Name: "auth", Dependencies: []string{"core"}},
				{Name: "cache", Dependencies: []string{"core"}},
				{Name: "api", Dependencies: []string{"auth", "cache"}},
			},
			wantLen: 4,
			validate: func(order []string) bool {
				return indexOf(order, "core") < indexOf(order, "auth") &&
					indexOf(order, "core") < indexOf(order, "cache") &&
					indexOf(order, "auth") < indexOf(order, "api") &&
					indexOf(order, "cache") < indexOf(order, "api")
			},
		},
		{
			name: "no dependencies - independent repos",
			repos: []RepoConfig{
				{Name: "service-a"},
				{Name: "service-b"},
				{Name: "service-c"},
			},
			wantLen: 3,
			validate: func(order []string) bool {
				return len(order) == 3
			},
		},
		{
			name: "single repo",
			repos: []RepoConfig{
				{Name: "standalone"},
			},
			wantLen: 1,
			validate: func(order []string) bool {
				return order[0] == "standalone"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := makeTestGroup(tt.repos)
			graph, err := NewDependencyGraph(group)
			if err != nil {
				t.Fatalf("unexpected error building graph: %v", err)
			}

			order, err := graph.ReleaseOrder()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(order) != tt.wantLen {
				t.Errorf("got %d repos, want %d", len(order), tt.wantLen)
			}

			if !tt.validate(order) {
				t.Errorf("invalid release order: %v", order)
			}
		})
	}
}

func TestDependencyGraph_AffectedRepos(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "core"},
		{Name: "auth", Dependencies: []string{"core"}},
		{Name: "cache", Dependencies: []string{"core"}},
		{Name: "api", Dependencies: []string{"auth", "cache"}},
		{Name: "frontend", Dependencies: []string{"api"}},
		{Name: "standalone"},
	})

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name        string
		changedRepo string
		wantRepos   map[string]bool
		wantErr     bool
	}{
		{
			name:        "core change affects all downstream",
			changedRepo: "core",
			wantRepos:   map[string]bool{"auth": true, "cache": true, "api": true, "frontend": true},
		},
		{
			name:        "auth change affects api and frontend",
			changedRepo: "auth",
			wantRepos:   map[string]bool{"api": true, "frontend": true},
		},
		{
			name:        "frontend change affects nothing",
			changedRepo: "frontend",
			wantRepos:   map[string]bool{},
		},
		{
			name:        "standalone change affects nothing",
			changedRepo: "standalone",
			wantRepos:   map[string]bool{},
		},
		{
			name:        "unknown repo returns error",
			changedRepo: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			affected, err := graph.AffectedRepos(tt.changedRepo)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(affected) != len(tt.wantRepos) {
				t.Errorf("got %d affected repos %v, want %d %v",
					len(affected), affected, len(tt.wantRepos), tt.wantRepos)
			}

			for _, repo := range affected {
				if !tt.wantRepos[repo] {
					t.Errorf("unexpected affected repo: %s", repo)
				}
			}

			// Verify changed repo is not included.
			for _, repo := range affected {
				if repo == tt.changedRepo {
					t.Error("changed repo should not be in affected list")
				}
			}
		})
	}
}

func TestDependencyGraph_Roots(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "core"},
		{Name: "utils"},
		{Name: "auth", Dependencies: []string{"core"}},
		{Name: "api", Dependencies: []string{"auth", "utils"}},
	})

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roots := graph.Roots()
	rootSet := make(map[string]bool)
	for _, r := range roots {
		rootSet[r] = true
	}

	if !rootSet["core"] || !rootSet["utils"] {
		t.Errorf("expected roots [core, utils], got %v", roots)
	}
	if rootSet["auth"] || rootSet["api"] {
		t.Errorf("auth and api should not be roots, got %v", roots)
	}
}

func TestDependencyGraph_Leaves(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "core"},
		{Name: "auth", Dependencies: []string{"core"}},
		{Name: "api", Dependencies: []string{"auth"}},
	})

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	leaves := graph.Leaves()
	if len(leaves) != 1 || leaves[0] != "api" {
		t.Errorf("expected leaves [api], got %v", leaves)
	}
}

func TestDependencyGraph_DirectDependencies(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "core"},
		{Name: "auth", Dependencies: []string{"core"}},
		{Name: "api", Dependencies: []string{"auth", "core"}},
	})

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deps := graph.DirectDependencies("api")
	if len(deps) != 2 {
		t.Errorf("expected 2 deps for api, got %d: %v", len(deps), deps)
	}

	deps = graph.DirectDependencies("core")
	if len(deps) != 0 {
		t.Errorf("expected 0 deps for core, got %d", len(deps))
	}
}

func TestDependencyGraph_DirectDependents(t *testing.T) {
	group := makeTestGroup([]RepoConfig{
		{Name: "core"},
		{Name: "auth", Dependencies: []string{"core"}},
		{Name: "api", Dependencies: []string{"core"}},
	})

	graph, err := NewDependencyGraph(group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dependents := graph.DirectDependents("core")
	if len(dependents) != 2 {
		t.Errorf("expected 2 dependents for core, got %d: %v", len(dependents), dependents)
	}
}

// indexOf returns the index of a string in a slice, or -1 if not found.
func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
