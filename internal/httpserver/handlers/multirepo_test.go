package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/domain/multirepo"
)

func setupMultirepoTestGroups() func() {
	// Save original provider.
	original := getGroupsFromConfig

	testGroups := []multirepo.RepositoryGroup{
		{
			Name:     "platform",
			Strategy: multirepo.StrategyCoordinated,
			Repositories: []multirepo.RepoConfig{
				{Name: "core-lib", URL: "https://github.com/org/core.git", Path: "/repos/core"},
				{Name: "auth-service", URL: "https://github.com/org/auth.git", Path: "/repos/auth", Dependencies: []string{"core-lib"}},
				{Name: "api-gateway", URL: "https://github.com/org/gateway.git", Path: "/repos/gateway", Dependencies: []string{"auth-service"}},
			},
		},
		{
			Name:     "frontend",
			Strategy: multirepo.StrategyIndependent,
			Repositories: []multirepo.RepoConfig{
				{Name: "web-app", URL: "https://github.com/org/web.git", Path: "/repos/web"},
				{Name: "mobile-app", URL: "https://github.com/org/mobile.git", Path: "/repos/mobile"},
			},
		},
	}

	SetGroupsProvider(func() []multirepo.RepositoryGroup {
		return testGroups
	})

	// Set handler context.
	SetContext(&Context{})

	return func() {
		getGroupsFromConfig = original
	}
}

func TestListGroups(t *testing.T) {
	cleanup := setupMultirepoTestGroups()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	w := httptest.NewRecorder()

	ListGroups(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var groups []GroupDTO
	if err := json.Unmarshal(w.Body.Bytes(), &groups); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Verify first group.
	platform := groups[0]
	if platform.Name != "platform" {
		t.Errorf("expected name 'platform', got %q", platform.Name)
	}
	if platform.Strategy != "coordinated" {
		t.Errorf("expected strategy 'coordinated', got %q", platform.Strategy)
	}
	if platform.RepoCount != 3 {
		t.Errorf("expected 3 repos, got %d", platform.RepoCount)
	}
	if len(platform.Repositories) != 3 {
		t.Errorf("expected 3 repository summaries, got %d", len(platform.Repositories))
	}
}

func TestListGroups_NoContext(t *testing.T) {
	// Save and restore.
	saved := DefaultContext
	DefaultContext = nil
	defer func() { DefaultContext = saved }()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	w := httptest.NewRecorder()

	ListGroups(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var groups []GroupDTO
	if err := json.Unmarshal(w.Body.Bytes(), &groups); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected empty list, got %d groups", len(groups))
	}
}

func TestGetGroupStatus(t *testing.T) {
	cleanup := setupMultirepoTestGroups()
	defer cleanup()

	// Create a chi router to properly extract URL params.
	r := chi.NewRouter()
	r.Get("/api/v1/groups/{name}/status", GetGroupStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/platform/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var status GroupStatusDTO
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if status.GroupName != "platform" {
		t.Errorf("expected group name 'platform', got %q", status.GroupName)
	}
	if len(status.Repositories) != 3 {
		t.Errorf("expected 3 repos, got %d", len(status.Repositories))
	}
}

func TestGetGroupStatus_NotFound(t *testing.T) {
	cleanup := setupMultirepoTestGroups()
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/api/v1/groups/{name}/status", GetGroupStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/nonexistent/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetGroupGraph(t *testing.T) {
	cleanup := setupMultirepoTestGroups()
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/api/v1/groups/{name}/graph", GetGroupGraph)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/platform/graph", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var graph GraphDTO
	if err := json.Unmarshal(w.Body.Bytes(), &graph); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if graph.GroupName != "platform" {
		t.Errorf("expected group name 'platform', got %q", graph.GroupName)
	}
	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	// auth-service depends on core-lib, api-gateway depends on auth-service = 2 edges.
	if len(graph.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(graph.Edges))
	}
}

func TestGetGroupGraph_NotFound(t *testing.T) {
	cleanup := setupMultirepoTestGroups()
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/api/v1/groups/{name}/graph", GetGroupGraph)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups/nonexistent/graph", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestMapGroupToDTO(t *testing.T) {
	group := &multirepo.RepositoryGroup{
		Name:     "test",
		Strategy: multirepo.StrategyCoordinated,
		Repositories: []multirepo.RepoConfig{
			{
				Name:              "core",
				URL:               "https://github.com/org/core.git",
				Path:              "/repos/core",
				VersionConstraint: ">=1.0.0",
			},
			{
				Name:         "auth",
				URL:          "https://github.com/org/auth.git",
				Path:         "/repos/auth",
				Dependencies: []string{"core"},
			},
		},
	}

	dto := mapGroupToDTO(group)

	if dto.Name != "test" {
		t.Errorf("expected name 'test', got %q", dto.Name)
	}
	if dto.Strategy != "coordinated" {
		t.Errorf("expected strategy 'coordinated', got %q", dto.Strategy)
	}
	if dto.RepoCount != 2 {
		t.Errorf("expected repo count 2, got %d", dto.RepoCount)
	}
	if len(dto.Repositories) != 2 {
		t.Errorf("expected 2 repos, got %d", len(dto.Repositories))
	}

	// Verify first repo.
	core := dto.Repositories[0]
	if core.VersionConstraint != ">=1.0.0" {
		t.Errorf("expected constraint '>=1.0.0', got %q", core.VersionConstraint)
	}

	// Verify second repo has dependency.
	auth := dto.Repositories[1]
	if len(auth.Dependencies) != 1 || auth.Dependencies[0] != "core" {
		t.Errorf("expected auth to depend on core, got %v", auth.Dependencies)
	}
}
