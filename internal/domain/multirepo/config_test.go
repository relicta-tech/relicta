package multirepo

import (
	"strings"
	"testing"
)

func TestReleaseStrategy_IsValid(t *testing.T) {
	tests := []struct {
		strategy ReleaseStrategy
		want     bool
	}{
		{StrategyIndependent, true},
		{StrategyCoordinated, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			if got := tt.strategy.IsValid(); got != tt.want {
				t.Errorf("ReleaseStrategy(%q).IsValid() = %v, want %v", tt.strategy, got, tt.want)
			}
		})
	}
}

func TestRepositoryGroup_Validate(t *testing.T) {
	tests := []struct {
		name    string
		group   RepositoryGroup
		wantErr string
	}{
		{
			name: "valid independent group",
			group: RepositoryGroup{
				Name:     "platform",
				Strategy: StrategyIndependent,
				Repositories: []RepoConfig{
					{Name: "auth-service", URL: "https://github.com/org/auth.git", Path: "/tmp/auth"},
					{Name: "api-gateway", URL: "https://github.com/org/gateway.git", Path: "/tmp/gateway"},
				},
			},
		},
		{
			name: "valid coordinated group with deps",
			group: RepositoryGroup{
				Name:     "platform",
				Strategy: StrategyCoordinated,
				Repositories: []RepoConfig{
					{Name: "core-lib", URL: "https://github.com/org/core.git", Path: "/tmp/core"},
					{Name: "auth-service", URL: "https://github.com/org/auth.git", Path: "/tmp/auth", Dependencies: []string{"core-lib"}},
					{Name: "api-gateway", URL: "https://github.com/org/gateway.git", Path: "/tmp/gateway", Dependencies: []string{"auth-service"}},
				},
			},
		},
		{
			name: "empty name",
			group: RepositoryGroup{
				Name:     "",
				Strategy: StrategyIndependent,
				Repositories: []RepoConfig{
					{Name: "repo1", Path: "/tmp/repo1"},
				},
			},
			wantErr: "repository group name is required",
		},
		{
			name: "no repositories",
			group: RepositoryGroup{
				Name:         "empty",
				Strategy:     StrategyIndependent,
				Repositories: []RepoConfig{},
			},
			wantErr: "must contain at least one repository",
		},
		{
			name: "invalid strategy",
			group: RepositoryGroup{
				Name:     "bad-strategy",
				Strategy: "invalid",
				Repositories: []RepoConfig{
					{Name: "repo1", Path: "/tmp/repo1"},
				},
			},
			wantErr: "invalid strategy",
		},
		{
			name: "duplicate repo names",
			group: RepositoryGroup{
				Name:     "dupes",
				Strategy: StrategyIndependent,
				Repositories: []RepoConfig{
					{Name: "repo1", Path: "/tmp/repo1"},
					{Name: "repo1", Path: "/tmp/repo1-copy"},
				},
			},
			wantErr: "duplicate repository name",
		},
		{
			name: "empty repo name",
			group: RepositoryGroup{
				Name:     "empty-name",
				Strategy: StrategyIndependent,
				Repositories: []RepoConfig{
					{Name: "", Path: "/tmp/repo1"},
				},
			},
			wantErr: "has empty name",
		},
		{
			name: "unknown dependency reference",
			group: RepositoryGroup{
				Name:     "bad-dep",
				Strategy: StrategyCoordinated,
				Repositories: []RepoConfig{
					{Name: "auth", Path: "/tmp/auth", Dependencies: []string{"nonexistent"}},
				},
			},
			wantErr: "depends on unknown repository",
		},
		{
			name: "self dependency",
			group: RepositoryGroup{
				Name:     "self-dep",
				Strategy: StrategyCoordinated,
				Repositories: []RepoConfig{
					{Name: "repo1", Path: "/tmp/repo1", Dependencies: []string{"repo1"}},
				},
			},
			wantErr: "cannot depend on itself",
		},
		{
			name: "circular dependency - direct cycle",
			group: RepositoryGroup{
				Name:     "cycle",
				Strategy: StrategyCoordinated,
				Repositories: []RepoConfig{
					{Name: "a", Path: "/tmp/a", Dependencies: []string{"b"}},
					{Name: "b", Path: "/tmp/b", Dependencies: []string{"a"}},
				},
			},
			wantErr: "circular dependency",
		},
		{
			name: "circular dependency - transitive cycle",
			group: RepositoryGroup{
				Name:     "transitive-cycle",
				Strategy: StrategyCoordinated,
				Repositories: []RepoConfig{
					{Name: "a", Path: "/tmp/a", Dependencies: []string{"b"}},
					{Name: "b", Path: "/tmp/b", Dependencies: []string{"c"}},
					{Name: "c", Path: "/tmp/c", Dependencies: []string{"a"}},
				},
			},
			wantErr: "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRepositoryGroup_GetRepo(t *testing.T) {
	group := RepositoryGroup{
		Name:     "test",
		Strategy: StrategyIndependent,
		Repositories: []RepoConfig{
			{Name: "auth", Path: "/tmp/auth"},
			{Name: "gateway", Path: "/tmp/gateway"},
		},
	}

	// Found
	repo := group.GetRepo("auth")
	if repo == nil {
		t.Fatal("expected to find repo 'auth'")
	}
	if repo.Path != "/tmp/auth" {
		t.Errorf("got path %q, want %q", repo.Path, "/tmp/auth")
	}

	// Not found
	repo = group.GetRepo("nonexistent")
	if repo != nil {
		t.Error("expected nil for nonexistent repo")
	}
}

func TestRepositoryGroup_RepoNames(t *testing.T) {
	group := RepositoryGroup{
		Name:     "test",
		Strategy: StrategyIndependent,
		Repositories: []RepoConfig{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
	}

	names := group.RepoNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	expected := map[string]bool{"a": true, "b": true, "c": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected name %q", n)
		}
	}
}
