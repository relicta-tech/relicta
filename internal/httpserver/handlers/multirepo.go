package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/domain/multirepo"
)

// GroupDTO is the API representation of a repository group.
type GroupDTO struct {
	Name         string                `json:"name"`
	Strategy     string                `json:"strategy"`
	Repositories []GroupRepoDTOSummary `json:"repositories"`
	RepoCount    int                   `json:"repo_count"`
}

// GroupRepoDTOSummary is a summary of a repository in a group.
type GroupRepoDTOSummary struct {
	Name              string   `json:"name"`
	URL               string   `json:"url"`
	Path              string   `json:"path"`
	Dependencies      []string `json:"dependencies,omitempty"`
	VersionConstraint string   `json:"version_constraint,omitempty"`
}

// GroupStatusDTO is the API representation of a group's release status.
type GroupStatusDTO struct {
	GroupName    string          `json:"group_name"`
	Strategy     string          `json:"strategy"`
	Repositories []RepoStatusDTO `json:"repositories"`
}

// RepoStatusDTO is the status of a single repository.
type RepoStatusDTO struct {
	Name           string   `json:"name"`
	State          string   `json:"state"`
	CurrentVersion string   `json:"current_version,omitempty"`
	HasChanges     bool     `json:"has_changes"`
	Dependencies   []string `json:"dependencies,omitempty"`
}

// GraphDTO is the API representation of a dependency graph.
type GraphDTO struct {
	GroupName string      `json:"group_name"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
}

// GraphNode is a node in the dependency graph.
type GraphNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GraphEdge is an edge in the dependency graph.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ListGroups returns all configured repository groups.
// GET /api/v1/groups
func ListGroups(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil {
		respondJSON(w, http.StatusOK, []GroupDTO{})
		return
	}

	groups := getGroupsFromConfig()
	dtos := make([]GroupDTO, 0, len(groups))
	for _, g := range groups {
		dtos = append(dtos, mapGroupToDTO(&g))
	}

	respondJSON(w, http.StatusOK, dtos)
}

// GetGroupStatus returns the release state of all repos in a group.
// GET /api/v1/groups/:name/status
func GetGroupStatus(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	group := findGroupByName(name)
	if group == nil {
		writeError(w, r, http.StatusNotFound, ErrCodeNotFound,
			"repository group not found", nil)
		return
	}

	status := GroupStatusDTO{
		GroupName:    group.Name,
		Strategy:     string(group.Strategy),
		Repositories: make([]RepoStatusDTO, 0, len(group.Repositories)),
	}

	for _, repo := range group.Repositories {
		repoStatus := RepoStatusDTO{
			Name:         repo.Name,
			State:        "unknown",
			Dependencies: repo.Dependencies,
		}
		status.Repositories = append(status.Repositories, repoStatus)
	}

	respondJSON(w, http.StatusOK, status)
}

// GetGroupGraph returns the dependency graph for visualization.
// GET /api/v1/groups/:name/graph
func GetGroupGraph(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	group := findGroupByName(name)
	if group == nil {
		writeError(w, r, http.StatusNotFound, ErrCodeNotFound,
			"repository group not found", nil)
		return
	}

	graph := GraphDTO{
		GroupName: group.Name,
		Nodes:     make([]GraphNode, 0, len(group.Repositories)),
		Edges:     make([]GraphEdge, 0),
	}

	for _, repo := range group.Repositories {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:   repo.Name,
			Name: repo.Name,
		})
		for _, dep := range repo.Dependencies {
			graph.Edges = append(graph.Edges, GraphEdge{
				From: repo.Name,
				To:   dep,
			})
		}
	}

	respondJSON(w, http.StatusOK, graph)
}

// mapGroupToDTO converts a RepositoryGroup to its API representation.
func mapGroupToDTO(g *multirepo.RepositoryGroup) GroupDTO {
	repos := make([]GroupRepoDTOSummary, 0, len(g.Repositories))
	for _, repo := range g.Repositories {
		repos = append(repos, GroupRepoDTOSummary{
			Name:              repo.Name,
			URL:               repo.URL,
			Path:              repo.Path,
			Dependencies:      repo.Dependencies,
			VersionConstraint: repo.VersionConstraint,
		})
	}

	return GroupDTO{
		Name:         g.Name,
		Strategy:     string(g.Strategy),
		Repositories: repos,
		RepoCount:    len(g.Repositories),
	}
}

// getGroupsFromConfig returns repository groups from the global config.
// This is a package-level function to avoid coupling handlers to config directly.
var getGroupsFromConfig = func() []multirepo.RepositoryGroup {
	return nil
}

// SetGroupsProvider sets the function that provides repository groups.
func SetGroupsProvider(fn func() []multirepo.RepositoryGroup) {
	getGroupsFromConfig = fn
}

// findGroupByName looks up a group by name from the config.
func findGroupByName(name string) *multirepo.RepositoryGroup {
	groups := getGroupsFromConfig()
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i]
		}
	}
	return nil
}

// respondJSONMultirepo is a helper to avoid import cycle with the existing respondJSON.
// Since respondJSON is already defined in releases.go, we reuse it.
var _ = json.NewEncoder // Ensure json import is used.
