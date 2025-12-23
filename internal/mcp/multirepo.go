package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
)

// MultiRepoManager manages multiple repository contexts for MCP operations.
type MultiRepoManager struct {
	repos     map[string]*RepoContext
	primary   string
	mu        sync.RWMutex
	gitLoader GitServiceLoader
}

// GitServiceLoader creates git service instances for repositories.
type GitServiceLoader func(path string) (GitService, error)

// GitService interface for git operations (subset needed for multi-repo).
type GitService interface {
	GetCurrentBranch() (string, error)
	GetLatestTag() (string, error)
}

// RepoContext holds state for a single repository.
type RepoContext struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Branch     string     `json:"branch,omitempty"`
	LatestTag  string     `json:"latest_tag,omitempty"`
	ReleaseID  string     `json:"release_id,omitempty"`
	State      string     `json:"state,omitempty"`
	Version    string     `json:"version,omitempty"`
	gitService GitService `json:"-"`
}

// NewMultiRepoManager creates a new multi-repo manager.
func NewMultiRepoManager(gitLoader GitServiceLoader) *MultiRepoManager {
	return &MultiRepoManager{
		repos:     make(map[string]*RepoContext),
		gitLoader: gitLoader,
	}
}

// AddRepository adds a repository to the manager.
func (m *MultiRepoManager) AddRepository(path string) (*RepoContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check if already added
	for _, repo := range m.repos {
		if repo.Path == absPath {
			return repo, nil
		}
	}

	// Create git service if loader is available
	var gitSvc GitService
	if m.gitLoader != nil {
		gitSvc, err = m.gitLoader(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize git service: %w", err)
		}
	}

	// Generate unique ID
	id := generateRepoID(absPath)
	name := filepath.Base(absPath)

	repo := &RepoContext{
		ID:         id,
		Path:       absPath,
		Name:       name,
		gitService: gitSvc,
	}

	// Load initial state
	if gitSvc != nil {
		if branch, err := gitSvc.GetCurrentBranch(); err == nil {
			repo.Branch = branch
		}
		if tag, err := gitSvc.GetLatestTag(); err == nil {
			repo.LatestTag = tag
		}
	}

	m.repos[id] = repo

	// Set as primary if first repo
	if m.primary == "" {
		m.primary = id
	}

	return repo, nil
}

// RemoveRepository removes a repository from the manager.
func (m *MultiRepoManager) RemoveRepository(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.repos[id]; !ok {
		return errors.New("repository not found")
	}

	delete(m.repos, id)

	// Update primary if removed
	if m.primary == id {
		m.primary = ""
		for newID := range m.repos {
			m.primary = newID
			break
		}
	}

	return nil
}

// GetRepository returns a repository by ID.
func (m *MultiRepoManager) GetRepository(id string) (*RepoContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	repo, ok := m.repos[id]
	if !ok {
		return nil, errors.New("repository not found")
	}

	return repo, nil
}

// GetPrimaryRepository returns the primary repository.
func (m *MultiRepoManager) GetPrimaryRepository() (*RepoContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.primary == "" {
		return nil, errors.New("no primary repository set")
	}

	return m.repos[m.primary], nil
}

// SetPrimaryRepository sets the primary repository.
func (m *MultiRepoManager) SetPrimaryRepository(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.repos[id]; !ok {
		return errors.New("repository not found")
	}

	m.primary = id
	return nil
}

// ListRepositories returns all repositories.
func (m *MultiRepoManager) ListRepositories() []*RepoContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	repos := make([]*RepoContext, 0, len(m.repos))
	for _, repo := range m.repos {
		repos = append(repos, repo)
	}
	return repos
}

// UpdateRepositoryState updates the release state for a repository.
func (m *MultiRepoManager) UpdateRepositoryState(id, releaseID, state, version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	repo, ok := m.repos[id]
	if !ok {
		return errors.New("repository not found")
	}

	repo.ReleaseID = releaseID
	repo.State = state
	repo.Version = version
	return nil
}

// RefreshRepository refreshes git state for a repository.
func (m *MultiRepoManager) RefreshRepository(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	repo, ok := m.repos[id]
	if !ok {
		return errors.New("repository not found")
	}

	if repo.gitService != nil {
		if branch, err := repo.gitService.GetCurrentBranch(); err == nil {
			repo.Branch = branch
		}
		if tag, err := repo.gitService.GetLatestTag(); err == nil {
			repo.LatestTag = tag
		}
	}

	return nil
}

func generateRepoID(path string) string {
	// Simple hash-like ID from path
	name := filepath.Base(path)
	return fmt.Sprintf("repo-%s", name)
}

// MultiRepoResource represents the multi-repo resource data.
type MultiRepoResource struct {
	Primary      string                  `json:"primary"`
	Repositories map[string]*RepoContext `json:"repositories"`
	Count        int                     `json:"count"`
}

// ToResource converts the manager state to a resource.
func (m *MultiRepoManager) ToResource() *MultiRepoResource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	repos := make(map[string]*RepoContext, len(m.repos))
	for id, repo := range m.repos {
		repos[id] = repo
	}

	return &MultiRepoResource{
		Primary:      m.primary,
		Repositories: repos,
		Count:        len(repos),
	}
}

// MultiRepoServer extends Server with multi-repo capabilities.
type MultiRepoServer struct {
	*Server
	repoManager *MultiRepoManager
}

// NewMultiRepoServer creates a server with multi-repo support.
func NewMultiRepoServer(version string, gitLoader GitServiceLoader, opts ...ServerOption) (*MultiRepoServer, error) {
	server, err := NewServer(version, opts...)
	if err != nil {
		return nil, err
	}

	return &MultiRepoServer{
		Server:      server,
		repoManager: NewMultiRepoManager(gitLoader),
	}, nil
}

// RepoManager returns the multi-repo manager.
func (s *MultiRepoServer) RepoManager() *MultiRepoManager {
	return s.repoManager
}

// RegisterMultiRepoTools registers multi-repo specific tools.
func (s *MultiRepoServer) RegisterMultiRepoTools() {
	// Tool: repos/list - List all repositories
	s.tools["relicta.repos.list"] = s.toolReposList

	// Tool: repos/add - Add a repository
	s.tools["relicta.repos.add"] = s.toolReposAdd

	// Tool: repos/remove - Remove a repository
	s.tools["relicta.repos.remove"] = s.toolReposRemove

	// Tool: repos/switch - Switch primary repository
	s.tools["relicta.repos.switch"] = s.toolReposSwitch

	// Tool: repos/refresh - Refresh repository state
	s.tools["relicta.repos.refresh"] = s.toolReposRefresh

	// Resource: relicta://repos - Multi-repo state
	s.resources["relicta://repos"] = s.resourceRepos
}

func (s *MultiRepoServer) toolReposList(_ context.Context, _ map[string]any) (*CallToolResult, error) {
	resource := s.repoManager.ToResource()
	return NewToolResultJSON(resource)
}

func (s *MultiRepoServer) toolReposAdd(_ context.Context, args map[string]any) (*CallToolResult, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return NewToolResultError("path is required"), nil
	}

	repo, err := s.repoManager.AddRepository(path)
	if err != nil {
		return NewToolResultError(fmt.Sprintf("failed to add repository: %v", err)), nil
	}

	return NewToolResultJSON(map[string]any{
		"added": true,
		"id":    repo.ID,
		"name":  repo.Name,
		"path":  repo.Path,
	})
}

func (s *MultiRepoServer) toolReposRemove(_ context.Context, args map[string]any) (*CallToolResult, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return NewToolResultError("repository id is required"), nil
	}

	if err := s.repoManager.RemoveRepository(id); err != nil {
		return NewToolResultError(fmt.Sprintf("failed to remove repository: %v", err)), nil
	}

	return NewToolResultJSON(map[string]any{
		"removed": true,
		"id":      id,
	})
}

func (s *MultiRepoServer) toolReposSwitch(_ context.Context, args map[string]any) (*CallToolResult, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return NewToolResultError("repository id is required"), nil
	}

	if err := s.repoManager.SetPrimaryRepository(id); err != nil {
		return NewToolResultError(fmt.Sprintf("failed to switch repository: %v", err)), nil
	}

	repo, _ := s.repoManager.GetRepository(id)
	return NewToolResultJSON(map[string]any{
		"switched":        true,
		"primary":         id,
		"primary_name":    repo.Name,
		"primary_path":    repo.Path,
		"primary_version": repo.Version,
	})
}

func (s *MultiRepoServer) toolReposRefresh(_ context.Context, args map[string]any) (*CallToolResult, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		// Refresh all
		for _, repo := range s.repoManager.ListRepositories() {
			_ = s.repoManager.RefreshRepository(repo.ID)
		}
		return NewToolResultJSON(map[string]any{
			"refreshed": "all",
			"count":     len(s.repoManager.ListRepositories()),
		})
	}

	if err := s.repoManager.RefreshRepository(id); err != nil {
		return NewToolResultError(fmt.Sprintf("failed to refresh repository: %v", err)), nil
	}

	repo, _ := s.repoManager.GetRepository(id)
	return NewToolResultJSON(map[string]any{
		"refreshed":  true,
		"id":         id,
		"branch":     repo.Branch,
		"latest_tag": repo.LatestTag,
		"release_id": repo.ReleaseID,
		"state":      repo.State,
		"version":    repo.Version,
	})
}

func (s *MultiRepoServer) resourceRepos(_ context.Context, _ string) (*ReadResourceResult, error) {
	resource := s.repoManager.ToResource()
	data, err := json.MarshalIndent(resource, "", "  ")
	if err != nil {
		return nil, err
	}

	return &ReadResourceResult{
		Contents: []ResourceContent{
			{
				URI:      "relicta://repos",
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

// MultiRepoClient extends Client with multi-repo operations.
type MultiRepoClient struct {
	*Client
}

// NewMultiRepoClient creates a client with multi-repo support.
func NewMultiRepoClient(transport ClientTransport, opts ...ClientOption) *MultiRepoClient {
	return &MultiRepoClient{
		Client: NewClient(transport, opts...),
	}
}

// ListRepos lists all repositories.
func (c *MultiRepoClient) ListRepos(ctx context.Context) (*MultiRepoResource, error) {
	var result MultiRepoResource
	if err := c.CallToolTyped(ctx, "relicta.repos.list", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AddRepo adds a repository.
func (c *MultiRepoClient) AddRepo(ctx context.Context, path string) (*RepoContext, error) {
	args := map[string]any{"path": path}

	var result struct {
		Added bool   `json:"added"`
		ID    string `json:"id"`
		Name  string `json:"name"`
		Path  string `json:"path"`
	}

	if err := c.CallToolTyped(ctx, "relicta.repos.add", args, &result); err != nil {
		return nil, err
	}

	return &RepoContext{
		ID:   result.ID,
		Name: result.Name,
		Path: result.Path,
	}, nil
}

// RemoveRepo removes a repository.
func (c *MultiRepoClient) RemoveRepo(ctx context.Context, id string) error {
	args := map[string]any{"id": id}

	var result struct {
		Removed bool   `json:"removed"`
		ID      string `json:"id"`
	}

	if err := c.CallToolTyped(ctx, "relicta.repos.remove", args, &result); err != nil {
		return err
	}

	if !result.Removed {
		return errors.New("failed to remove repository")
	}

	return nil
}

// SwitchRepo switches the primary repository.
func (c *MultiRepoClient) SwitchRepo(ctx context.Context, id string) error {
	args := map[string]any{"id": id}

	var result struct {
		Switched bool `json:"switched"`
	}

	if err := c.CallToolTyped(ctx, "relicta.repos.switch", args, &result); err != nil {
		return err
	}

	if !result.Switched {
		return errors.New("failed to switch repository")
	}

	return nil
}

// RefreshRepo refreshes repository state.
func (c *MultiRepoClient) RefreshRepo(ctx context.Context, id string) (*RepoContext, error) {
	args := map[string]any{}
	if id != "" {
		args["id"] = id
	}

	var result struct {
		Refreshed any    `json:"refreshed"`
		ID        string `json:"id,omitempty"`
		Branch    string `json:"branch,omitempty"`
		LatestTag string `json:"latest_tag,omitempty"`
		ReleaseID string `json:"release_id,omitempty"`
		State     string `json:"state,omitempty"`
		Version   string `json:"version,omitempty"`
		Count     int    `json:"count,omitempty"`
	}

	if err := c.CallToolTyped(ctx, "relicta.repos.refresh", args, &result); err != nil {
		return nil, err
	}

	// If refreshing all, return nil
	if result.ID == "" {
		return nil, nil
	}

	return &RepoContext{
		ID:        result.ID,
		Branch:    result.Branch,
		LatestTag: result.LatestTag,
		ReleaseID: result.ReleaseID,
		State:     result.State,
		Version:   result.Version,
	}, nil
}

// GetRepos reads the repos resource.
func (c *MultiRepoClient) GetRepos(ctx context.Context) (*MultiRepoResource, error) {
	var result MultiRepoResource
	if err := c.ReadResourceTyped(ctx, "relicta://repos", &result); err != nil {
		return nil, err
	}
	return &result, nil
}
