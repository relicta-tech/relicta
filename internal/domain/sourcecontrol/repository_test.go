// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"testing"
)

func TestRepositoryInfo_Fields(t *testing.T) {
	info := RepositoryInfo{
		Path:          "/home/user/project",
		Name:          "myproject",
		Owner:         "myuser",
		RemoteURL:     "https://github.com/myuser/myproject",
		DefaultBranch: "main",
		CurrentBranch: "feature/new-feature",
		IsDirty:       true,
	}

	if info.Path != "/home/user/project" {
		t.Errorf("Path = %v, want /home/user/project", info.Path)
	}
	if info.Name != "myproject" {
		t.Errorf("Name = %v, want myproject", info.Name)
	}
	if info.Owner != "myuser" {
		t.Errorf("Owner = %v, want myuser", info.Owner)
	}
	if info.RemoteURL != "https://github.com/myuser/myproject" {
		t.Errorf("RemoteURL = %v, want https://github.com/myuser/myproject", info.RemoteURL)
	}
	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main", info.DefaultBranch)
	}
	if info.CurrentBranch != "feature/new-feature" {
		t.Errorf("CurrentBranch = %v, want feature/new-feature", info.CurrentBranch)
	}
	if !info.IsDirty {
		t.Error("IsDirty should be true")
	}
}

func TestRemoteInfo_Fields(t *testing.T) {
	remote := RemoteInfo{
		Name: "origin",
		URL:  "https://github.com/user/repo.git",
	}

	if remote.Name != "origin" {
		t.Errorf("Name = %v, want origin", remote.Name)
	}
	if remote.URL != "https://github.com/user/repo.git" {
		t.Errorf("URL = %v, want https://github.com/user/repo.git", remote.URL)
	}
}

func TestBranchInfo_Fields(t *testing.T) {
	branch := BranchInfo{
		Name:      "feature/awesome",
		IsRemote:  false,
		IsCurrent: true,
		Hash:      CommitHash("abc123def456"),
		Upstream:  "origin/feature/awesome",
	}

	if branch.Name != "feature/awesome" {
		t.Errorf("Name = %v, want feature/awesome", branch.Name)
	}
	if branch.IsRemote {
		t.Error("IsRemote should be false")
	}
	if !branch.IsCurrent {
		t.Error("IsCurrent should be true")
	}
	if branch.Hash != CommitHash("abc123def456") {
		t.Errorf("Hash = %v, want abc123def456", branch.Hash)
	}
	if branch.Upstream != "origin/feature/awesome" {
		t.Errorf("Upstream = %v, want origin/feature/awesome", branch.Upstream)
	}
}

func TestWorkingTreeStatus_Fields(t *testing.T) {
	status := WorkingTreeStatus{
		IsClean: false,
		Staged: []FileChange{
			{Path: "file1.go", Status: FileStatusAdded},
			{Path: "file2.go", Status: FileStatusModified},
		},
		Unstaged: []FileChange{
			{Path: "file3.go", Status: FileStatusDeleted},
		},
		Untracked: []string{"newfile.go", "temp.txt"},
	}

	if status.IsClean {
		t.Error("IsClean should be false")
	}
	if len(status.Staged) != 2 {
		t.Errorf("Staged length = %v, want 2", len(status.Staged))
	}
	if len(status.Unstaged) != 1 {
		t.Errorf("Unstaged length = %v, want 1", len(status.Unstaged))
	}
	if len(status.Untracked) != 2 {
		t.Errorf("Untracked length = %v, want 2", len(status.Untracked))
	}
}

func TestFileChange_Fields(t *testing.T) {
	change := FileChange{
		Path:   "src/main.go",
		Status: FileStatusModified,
	}

	if change.Path != "src/main.go" {
		t.Errorf("Path = %v, want src/main.go", change.Path)
	}
	if change.Status != FileStatusModified {
		t.Errorf("Status = %v, want modified", change.Status)
	}
}

func TestFileStatusConstants(t *testing.T) {
	tests := []struct {
		status FileStatus
		want   string
	}{
		{FileStatusAdded, "added"},
		{FileStatusModified, "modified"},
		{FileStatusDeleted, "deleted"},
		{FileStatusRenamed, "renamed"},
		{FileStatusCopied, "copied"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("FileStatus = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

func TestNewVersionDiscovery(t *testing.T) {
	vd := NewVersionDiscovery("v")

	if vd == nil {
		t.Fatal("NewVersionDiscovery returned nil")
	}
	if vd.tagPrefix != "v" {
		t.Errorf("tagPrefix = %v, want v", vd.tagPrefix)
	}
}

func TestNewVersionDiscovery_EmptyPrefix(t *testing.T) {
	vd := NewVersionDiscovery("")

	if vd.tagPrefix != "" {
		t.Errorf("tagPrefix = %v, want empty string", vd.tagPrefix)
	}
}
