package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitService implements GitService for testing.
type mockGitService struct {
	branch    string
	tag       string
	branchErr error
	tagErr    error
}

func (m *mockGitService) GetCurrentBranch() (string, error) {
	if m.branchErr != nil {
		return "", m.branchErr
	}
	return m.branch, nil
}

func (m *mockGitService) GetLatestTag() (string, error) {
	if m.tagErr != nil {
		return "", m.tagErr
	}
	return m.tag, nil
}

func TestNewMultiRepoManager(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.repos)
	assert.Empty(t, manager.primary)
}

func TestMultiRepoManager_AddRepository(t *testing.T) {
	tempDir := t.TempDir()

	loader := func(path string) (GitService, error) {
		return &mockGitService{branch: "main", tag: "v1.0.0"}, nil
	}
	manager := NewMultiRepoManager(loader)

	repo, err := manager.AddRepository(tempDir)

	require.NoError(t, err)
	assert.NotNil(t, repo)
	assert.NotEmpty(t, repo.ID)
	assert.Equal(t, filepath.Base(tempDir), repo.Name)
	assert.Equal(t, "main", repo.Branch)
	assert.Equal(t, "v1.0.0", repo.LatestTag)
	assert.Equal(t, repo.ID, manager.primary) // First repo becomes primary
}

func TestMultiRepoManager_AddRepository_Duplicate(t *testing.T) {
	tempDir := t.TempDir()

	manager := NewMultiRepoManager(nil)

	repo1, err := manager.AddRepository(tempDir)
	require.NoError(t, err)

	repo2, err := manager.AddRepository(tempDir)
	require.NoError(t, err)

	// Should return the same repo
	assert.Equal(t, repo1.ID, repo2.ID)
	assert.Equal(t, repo1.Path, repo2.Path)
}

func TestMultiRepoManager_AddRepository_GitLoaderError(t *testing.T) {
	tempDir := t.TempDir()

	loader := func(path string) (GitService, error) {
		return nil, errors.New("git init failed")
	}
	manager := NewMultiRepoManager(loader)

	repo, err := manager.AddRepository(tempDir)

	assert.Error(t, err)
	assert.Nil(t, repo)
	assert.Contains(t, err.Error(), "failed to initialize git service")
}

func TestMultiRepoManager_AddRepository_NoLoader(t *testing.T) {
	tempDir := t.TempDir()

	manager := NewMultiRepoManager(nil)

	repo, err := manager.AddRepository(tempDir)

	require.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Empty(t, repo.Branch) // No git service to load branch
	assert.Empty(t, repo.LatestTag)
}

func TestMultiRepoManager_RemoveRepository(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo, err := manager.AddRepository(tempDir)
	require.NoError(t, err)

	err = manager.RemoveRepository(repo.ID)
	require.NoError(t, err)

	_, err = manager.GetRepository(repo.ID)
	assert.Error(t, err)
}

func TestMultiRepoManager_RemoveRepository_NotFound(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	err := manager.RemoveRepository("nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository not found")
}

func TestMultiRepoManager_RemoveRepository_UpdatesPrimary(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo1, _ := manager.AddRepository(tempDir1)
	repo2, _ := manager.AddRepository(tempDir2)

	// First repo is primary
	assert.Equal(t, repo1.ID, manager.primary)

	// Remove primary
	err := manager.RemoveRepository(repo1.ID)
	require.NoError(t, err)

	// Second repo should now be primary
	assert.Equal(t, repo2.ID, manager.primary)
}

func TestMultiRepoManager_GetRepository(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewMultiRepoManager(nil)

	added, err := manager.AddRepository(tempDir)
	require.NoError(t, err)

	repo, err := manager.GetRepository(added.ID)
	require.NoError(t, err)
	assert.Equal(t, added.ID, repo.ID)
	assert.Equal(t, added.Path, repo.Path)
}

func TestMultiRepoManager_GetRepository_NotFound(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	repo, err := manager.GetRepository("nonexistent")

	assert.Error(t, err)
	assert.Nil(t, repo)
	assert.Contains(t, err.Error(), "repository not found")
}

func TestMultiRepoManager_GetPrimaryRepository(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewMultiRepoManager(nil)

	added, err := manager.AddRepository(tempDir)
	require.NoError(t, err)

	primary, err := manager.GetPrimaryRepository()
	require.NoError(t, err)
	assert.Equal(t, added.ID, primary.ID)
}

func TestMultiRepoManager_GetPrimaryRepository_NoPrimary(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	primary, err := manager.GetPrimaryRepository()

	assert.Error(t, err)
	assert.Nil(t, primary)
	assert.Contains(t, err.Error(), "no primary repository set")
}

func TestMultiRepoManager_SetPrimaryRepository(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo1, _ := manager.AddRepository(tempDir1)
	repo2, _ := manager.AddRepository(tempDir2)

	// First repo is primary by default
	assert.Equal(t, repo1.ID, manager.primary)

	// Switch to second repo
	err := manager.SetPrimaryRepository(repo2.ID)
	require.NoError(t, err)
	assert.Equal(t, repo2.ID, manager.primary)
}

func TestMultiRepoManager_SetPrimaryRepository_NotFound(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	err := manager.SetPrimaryRepository("nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository not found")
}

func TestMultiRepoManager_ListRepositories(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo1, _ := manager.AddRepository(tempDir1)
	repo2, _ := manager.AddRepository(tempDir2)

	repos := manager.ListRepositories()

	assert.Len(t, repos, 2)

	ids := make(map[string]bool)
	for _, r := range repos {
		ids[r.ID] = true
	}
	assert.True(t, ids[repo1.ID])
	assert.True(t, ids[repo2.ID])
}

func TestMultiRepoManager_ListRepositories_Empty(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	repos := manager.ListRepositories()

	assert.Empty(t, repos)
}

func TestMultiRepoManager_UpdateRepositoryState(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo, _ := manager.AddRepository(tempDir)

	err := manager.UpdateRepositoryState(repo.ID, "rel-123", "approved", "1.2.0")
	require.NoError(t, err)

	updated, _ := manager.GetRepository(repo.ID)
	assert.Equal(t, "rel-123", updated.ReleaseID)
	assert.Equal(t, "approved", updated.State)
	assert.Equal(t, "1.2.0", updated.Version)
}

func TestMultiRepoManager_UpdateRepositoryState_NotFound(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	err := manager.UpdateRepositoryState("nonexistent", "rel-123", "approved", "1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository not found")
}

func TestMultiRepoManager_RefreshRepository(t *testing.T) {
	tempDir := t.TempDir()

	gitSvc := &mockGitService{branch: "main", tag: "v1.0.0"}
	loader := func(path string) (GitService, error) {
		return gitSvc, nil
	}
	manager := NewMultiRepoManager(loader)

	repo, _ := manager.AddRepository(tempDir)
	assert.Equal(t, "main", repo.Branch)

	// Update mock values
	gitSvc.branch = "feature-branch"
	gitSvc.tag = "v2.0.0"

	err := manager.RefreshRepository(repo.ID)
	require.NoError(t, err)

	refreshed, _ := manager.GetRepository(repo.ID)
	assert.Equal(t, "feature-branch", refreshed.Branch)
	assert.Equal(t, "v2.0.0", refreshed.LatestTag)
}

func TestMultiRepoManager_RefreshRepository_NotFound(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	err := manager.RefreshRepository("nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository not found")
}

func TestMultiRepoManager_RefreshRepository_NoGitService(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo, _ := manager.AddRepository(tempDir)

	// Should not error even without git service
	err := manager.RefreshRepository(repo.ID)
	require.NoError(t, err)
}

func TestMultiRepoManager_ToResource(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	manager := NewMultiRepoManager(nil)

	repo1, _ := manager.AddRepository(tempDir1)
	manager.AddRepository(tempDir2)

	resource := manager.ToResource()

	assert.Equal(t, repo1.ID, resource.Primary)
	assert.Equal(t, 2, resource.Count)
	assert.Len(t, resource.Repositories, 2)
}

func TestGenerateRepoID(t *testing.T) {
	id1 := generateRepoID("/path/to/repo1")
	id2 := generateRepoID("/path/to/repo2")
	id3 := generateRepoID("/other/path/to/repo1")

	assert.Equal(t, "repo-repo1", id1)
	assert.Equal(t, "repo-repo2", id2)
	assert.Equal(t, "repo-repo1", id3) // Same base name = same ID
}

func TestNewMultiRepoServer(t *testing.T) {
	server, err := NewMultiRepoServer("1.0.0", nil)

	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.Server)
	assert.NotNil(t, server.repoManager)
}

func TestMultiRepoServer_RepoManager(t *testing.T) {
	server, _ := NewMultiRepoServer("1.0.0", nil)

	manager := server.RepoManager()

	assert.NotNil(t, manager)
	assert.Same(t, server.repoManager, manager)
}

func TestMultiRepoServer_RegisterMultiRepoTools(t *testing.T) {
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	// Verify tools are registered
	assert.Contains(t, server.tools, "relicta.repos.list")
	assert.Contains(t, server.tools, "relicta.repos.add")
	assert.Contains(t, server.tools, "relicta.repos.remove")
	assert.Contains(t, server.tools, "relicta.repos.switch")
	assert.Contains(t, server.tools, "relicta.repos.refresh")

	// Verify resource is registered
	assert.Contains(t, server.resources, "relicta://repos")
}

func TestMultiRepoServer_ToolReposList(t *testing.T) {
	tempDir := t.TempDir()
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	server.repoManager.AddRepository(tempDir)

	result, err := server.toolReposList(context.Background(), nil)

	require.NoError(t, err)
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)

	var resource MultiRepoResource
	err = json.Unmarshal([]byte(result.Content[0].Text), &resource)
	require.NoError(t, err)
	assert.Equal(t, 1, resource.Count)
}

func TestMultiRepoServer_ToolReposAdd(t *testing.T) {
	tempDir := t.TempDir()
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	result, err := server.toolReposAdd(context.Background(), map[string]any{
		"path": tempDir,
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var response struct {
		Added bool   `json:"added"`
		ID    string `json:"id"`
		Name  string `json:"name"`
		Path  string `json:"path"`
	}
	err = json.Unmarshal([]byte(result.Content[0].Text), &response)
	require.NoError(t, err)
	assert.True(t, response.Added)
	assert.NotEmpty(t, response.ID)
}

func TestMultiRepoServer_ToolReposAdd_MissingPath(t *testing.T) {
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	result, err := server.toolReposAdd(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "path is required")
}

func TestMultiRepoServer_ToolReposRemove(t *testing.T) {
	tempDir := t.TempDir()
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	repo, _ := server.repoManager.AddRepository(tempDir)

	result, err := server.toolReposRemove(context.Background(), map[string]any{
		"id": repo.ID,
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var response struct {
		Removed bool   `json:"removed"`
		ID      string `json:"id"`
	}
	err = json.Unmarshal([]byte(result.Content[0].Text), &response)
	require.NoError(t, err)
	assert.True(t, response.Removed)
}

func TestMultiRepoServer_ToolReposRemove_MissingID(t *testing.T) {
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	result, err := server.toolReposRemove(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "repository id is required")
}

func TestMultiRepoServer_ToolReposSwitch(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	server.repoManager.AddRepository(tempDir1)
	repo2, _ := server.repoManager.AddRepository(tempDir2)

	result, err := server.toolReposSwitch(context.Background(), map[string]any{
		"id": repo2.ID,
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var response struct {
		Switched    bool   `json:"switched"`
		Primary     string `json:"primary"`
		PrimaryName string `json:"primary_name"`
	}
	err = json.Unmarshal([]byte(result.Content[0].Text), &response)
	require.NoError(t, err)
	assert.True(t, response.Switched)
	assert.Equal(t, repo2.ID, response.Primary)
}

func TestMultiRepoServer_ToolReposSwitch_MissingID(t *testing.T) {
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	result, err := server.toolReposSwitch(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "repository id is required")
}

func TestMultiRepoServer_ToolReposRefresh_All(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	server.repoManager.AddRepository(tempDir1)
	server.repoManager.AddRepository(tempDir2)

	result, err := server.toolReposRefresh(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var response struct {
		Refreshed string `json:"refreshed"`
		Count     int    `json:"count"`
	}
	err = json.Unmarshal([]byte(result.Content[0].Text), &response)
	require.NoError(t, err)
	assert.Equal(t, "all", response.Refreshed)
	assert.Equal(t, 2, response.Count)
}

func TestMultiRepoServer_ToolReposRefresh_Single(t *testing.T) {
	tempDir := t.TempDir()

	gitSvc := &mockGitService{branch: "main", tag: "v1.0.0"}
	loader := func(path string) (GitService, error) {
		return gitSvc, nil
	}
	server, _ := NewMultiRepoServer("1.0.0", loader)
	server.RegisterMultiRepoTools()

	repo, _ := server.repoManager.AddRepository(tempDir)

	// Update mock values before refresh
	gitSvc.branch = "develop"
	gitSvc.tag = "v2.0.0"

	result, err := server.toolReposRefresh(context.Background(), map[string]any{
		"id": repo.ID,
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var response struct {
		Refreshed bool   `json:"refreshed"`
		ID        string `json:"id"`
		Branch    string `json:"branch"`
		LatestTag string `json:"latest_tag"`
	}
	err = json.Unmarshal([]byte(result.Content[0].Text), &response)
	require.NoError(t, err)
	assert.True(t, response.Refreshed)
	assert.Equal(t, "develop", response.Branch)
	assert.Equal(t, "v2.0.0", response.LatestTag)
}

func TestMultiRepoServer_ResourceRepos(t *testing.T) {
	tempDir := t.TempDir()
	server, _ := NewMultiRepoServer("1.0.0", nil)
	server.RegisterMultiRepoTools()

	server.repoManager.AddRepository(tempDir)

	result, err := server.resourceRepos(context.Background(), "relicta://repos")

	require.NoError(t, err)
	require.Len(t, result.Contents, 1)
	assert.Equal(t, "relicta://repos", result.Contents[0].URI)
	assert.Equal(t, "application/json", result.Contents[0].MIMEType)

	var resource MultiRepoResource
	err = json.Unmarshal([]byte(result.Contents[0].Text), &resource)
	require.NoError(t, err)
	assert.Equal(t, 1, resource.Count)
}

func TestNewMultiRepoClient(t *testing.T) {
	transport := newMockTransport()
	client := NewMultiRepoClient(transport)

	assert.NotNil(t, client)
	assert.NotNil(t, client.Client)
}

func TestMultiRepoClient_ListRepos(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"primary": "repo-test",
			"repositories": {
				"repo-test": {"id": "repo-test", "name": "test", "path": "/tmp/test"}
			},
			"count": 1
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	resource, err := client.ListRepos(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "repo-test", resource.Primary)
	assert.Equal(t, 1, resource.Count)
}

func TestMultiRepoClient_AddRepo(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"added": true,
			"id": "repo-myrepo",
			"name": "myrepo",
			"path": "/tmp/myrepo"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	repo, err := client.AddRepo(context.Background(), "/tmp/myrepo")

	require.NoError(t, err)
	assert.Equal(t, "repo-myrepo", repo.ID)
	assert.Equal(t, "myrepo", repo.Name)
	assert.Equal(t, "/tmp/myrepo", repo.Path)
}

func TestMultiRepoClient_RemoveRepo(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"removed": true,
			"id": "repo-test"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	err := client.RemoveRepo(context.Background(), "repo-test")

	require.NoError(t, err)
}

func TestMultiRepoClient_RemoveRepo_Failed(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"removed": false,
			"id": "repo-test"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	err := client.RemoveRepo(context.Background(), "repo-test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove repository")
}

func TestMultiRepoClient_SwitchRepo(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"switched": true,
			"primary": "repo-other"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	err := client.SwitchRepo(context.Background(), "repo-other")

	require.NoError(t, err)
}

func TestMultiRepoClient_SwitchRepo_Failed(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"switched": false
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	err := client.SwitchRepo(context.Background(), "repo-other")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to switch repository")
}

func TestMultiRepoClient_RefreshRepo_Single(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"refreshed": true,
			"id": "repo-test",
			"branch": "main",
			"latest_tag": "v1.0.0",
			"release_id": "rel-123",
			"state": "approved",
			"version": "1.1.0"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	repo, err := client.RefreshRepo(context.Background(), "repo-test")

	require.NoError(t, err)
	require.NotNil(t, repo)
	assert.Equal(t, "repo-test", repo.ID)
	assert.Equal(t, "main", repo.Branch)
	assert.Equal(t, "v1.0.0", repo.LatestTag)
	assert.Equal(t, "rel-123", repo.ReleaseID)
	assert.Equal(t, "approved", repo.State)
	assert.Equal(t, "1.1.0", repo.Version)
}

func TestMultiRepoClient_RefreshRepo_All(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"refreshed": "all",
			"count": 3
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	repo, err := client.RefreshRepo(context.Background(), "")

	require.NoError(t, err)
	assert.Nil(t, repo) // Returns nil when refreshing all
}

func TestMultiRepoClient_GetRepos(t *testing.T) {
	// Mock resource read - use ReadResourceResult that will be marshaled
	result := ReadResourceResult{
		Contents: []ResourceContent{{
			URI:      "relicta://repos",
			MIMEType: "application/json",
			Text: `{
				"primary": "repo-main",
				"repositories": {
					"repo-main": {"id": "repo-main", "name": "main", "path": "/repos/main"}
				},
				"count": 1
			}`,
		}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewMultiRepoClient(transport)
	resource, err := client.GetRepos(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "repo-main", resource.Primary)
	assert.Equal(t, 1, resource.Count)
}

func TestRepoContext_JSON(t *testing.T) {
	repo := &RepoContext{
		ID:        "repo-test",
		Path:      "/path/to/repo",
		Name:      "test",
		Branch:    "main",
		LatestTag: "v1.0.0",
		ReleaseID: "rel-123",
		State:     "approved",
		Version:   "1.1.0",
	}

	data, err := json.Marshal(repo)
	require.NoError(t, err)

	var decoded RepoContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, repo.ID, decoded.ID)
	assert.Equal(t, repo.Path, decoded.Path)
	assert.Equal(t, repo.Name, decoded.Name)
	assert.Equal(t, repo.Branch, decoded.Branch)
	assert.Equal(t, repo.LatestTag, decoded.LatestTag)
	assert.Equal(t, repo.ReleaseID, decoded.ReleaseID)
	assert.Equal(t, repo.State, decoded.State)
	assert.Equal(t, repo.Version, decoded.Version)
}

func TestMultiRepoResource_JSON(t *testing.T) {
	resource := &MultiRepoResource{
		Primary: "repo-main",
		Repositories: map[string]*RepoContext{
			"repo-main": {ID: "repo-main", Name: "main"},
			"repo-sub":  {ID: "repo-sub", Name: "sub"},
		},
		Count: 2,
	}

	data, err := json.Marshal(resource)
	require.NoError(t, err)

	var decoded MultiRepoResource
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "repo-main", decoded.Primary)
	assert.Equal(t, 2, decoded.Count)
	assert.Len(t, decoded.Repositories, 2)
}

func TestMultiRepoManager_ConcurrentAccess(t *testing.T) {
	manager := NewMultiRepoManager(nil)

	// Create multiple temp directories
	dirs := make([]string, 10)
	for i := 0; i < 10; i++ {
		dirs[i] = t.TempDir()
	}

	// Concurrent adds
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			_, _ = manager.AddRepository(dirs[idx])
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	repos := manager.ListRepositories()
	assert.Len(t, repos, 10)
}

func TestMultiRepoManager_AbsolutePath(t *testing.T) {
	// Test with relative path
	currentDir, _ := os.Getwd()
	manager := NewMultiRepoManager(nil)

	repo, err := manager.AddRepository(".")
	require.NoError(t, err)

	// Path should be absolute
	assert.True(t, filepath.IsAbs(repo.Path))
	assert.Equal(t, currentDir, repo.Path)
}
