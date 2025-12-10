// Package version provides version management for ReleasePilot.
package version

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	rperrors "github.com/felixgeelhaar/release-pilot/internal/errors"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// mockGitService is a mock implementation of git.Service for testing.
type mockGitService struct {
	latestVersionTag *git.Tag
	latestTagErr     error
	createTagCalled  bool
	createTagName    string
	createTagErr     error
	pushTagCalled    bool
	pushTagName      string
	pushTagErr       error
}

func (m *mockGitService) GetRepositoryRoot(_ context.Context) (string, error) { return ".", nil }
func (m *mockGitService) GetRepositoryInfo(_ context.Context) (*git.RepositoryInfo, error) {
	return nil, nil
}
func (m *mockGitService) IsClean(_ context.Context) (bool, error) { return true, nil }
func (m *mockGitService) GetCommit(_ context.Context, _ string) (*git.Commit, error) {
	return nil, nil
}
func (m *mockGitService) GetCommitsSince(_ context.Context, _ string) ([]git.Commit, error) {
	return nil, nil
}
func (m *mockGitService) GetCommitsBetween(_ context.Context, _, _ string) ([]git.Commit, error) {
	return nil, nil
}
func (m *mockGitService) GetHeadCommit(_ context.Context) (*git.Commit, error) { return nil, nil }
func (m *mockGitService) GetBranchCommit(_ context.Context, _ string) (*git.Commit, error) {
	return nil, nil
}
func (m *mockGitService) GetLatestTag(_ context.Context) (*git.Tag, error) { return nil, nil }
func (m *mockGitService) GetLatestVersionTag(_ context.Context, _ string) (*git.Tag, error) {
	return m.latestVersionTag, m.latestTagErr
}
func (m *mockGitService) ListTags(_ context.Context) ([]git.Tag, error) { return nil, nil }
func (m *mockGitService) ListVersionTags(_ context.Context, _ string) ([]git.Tag, error) {
	return nil, nil
}
func (m *mockGitService) GetTag(_ context.Context, _ string) (*git.Tag, error) { return nil, nil }
func (m *mockGitService) CreateTag(_ context.Context, name, _ string, _ git.TagOptions) error {
	m.createTagCalled = true
	m.createTagName = name
	return m.createTagErr
}
func (m *mockGitService) DeleteTag(_ context.Context, _ string) error { return nil }
func (m *mockGitService) PushTag(_ context.Context, name string, _ git.PushOptions) error {
	m.pushTagCalled = true
	m.pushTagName = name
	return m.pushTagErr
}
func (m *mockGitService) GetCurrentBranch(_ context.Context) (string, error)   { return "main", nil }
func (m *mockGitService) GetDefaultBranch(_ context.Context) (string, error)   { return "main", nil }
func (m *mockGitService) ListBranches(_ context.Context) ([]git.Branch, error) { return nil, nil }
func (m *mockGitService) GetRemoteURL(_ context.Context, _ string) (string, error) {
	return "https://github.com/user/repo", nil
}
func (m *mockGitService) Push(_ context.Context, _ git.PushOptions) error   { return nil }
func (m *mockGitService) Pull(_ context.Context, _ git.PullOptions) error   { return nil }
func (m *mockGitService) Fetch(_ context.Context, _ git.FetchOptions) error { return nil }
func (m *mockGitService) GetDiffStats(_ context.Context, _, _ string) (*git.DiffStats, error) {
	return nil, nil
}
func (m *mockGitService) ParseConventionalCommit(_ string) (*git.ConventionalCommit, error) {
	return nil, nil
}
func (m *mockGitService) ParseConventionalCommits(_ []git.Commit, _ git.ParseOptions) ([]git.ConventionalCommit, error) {
	return nil, nil
}
func (m *mockGitService) DetectReleaseType(_ []git.ConventionalCommit) git.ReleaseType {
	return git.ReleaseTypePatch
}
func (m *mockGitService) CategorizeCommits(_ []git.ConventionalCommit) *git.CategorizedChanges {
	return nil
}
func (m *mockGitService) FilterCommits(commits []git.ConventionalCommit, _ git.CommitFilter) []git.ConventionalCommit {
	return commits
}

func TestNewService(t *testing.T) {
	t.Run("success with git service", func(t *testing.T) {
		mock := &mockGitService{}
		svc, err := NewService(WithGitService(mock))
		if err != nil {
			t.Fatalf("NewService() error = %v, want nil", err)
		}
		if svc == nil {
			t.Fatal("NewService() returned nil")
		}
	})

	t.Run("error without git service", func(t *testing.T) {
		_, err := NewService()
		if err == nil {
			t.Fatal("NewService() error = nil, want error")
		}
		if !rperrors.IsKind(err, rperrors.KindVersion) {
			t.Errorf("NewService() error kind = %v, want KindVersion", rperrors.GetKind(err))
		}
	})

	t.Run("with options", func(t *testing.T) {
		mock := &mockGitService{}
		svc, err := NewService(
			WithGitService(mock),
			WithDefaultPrefix("release-"),
			WithVersionSource("file"),
			WithVersionFile("VERSION"),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v, want nil", err)
		}
		if svc.cfg.DefaultPrefix != "release-" {
			t.Errorf("DefaultPrefix = %q, want %q", svc.cfg.DefaultPrefix, "release-")
		}
		if svc.cfg.VersionSource != "file" {
			t.Errorf("VersionSource = %q, want %q", svc.cfg.VersionSource, "file")
		}
		if svc.cfg.VersionFile != "VERSION" {
			t.Errorf("VersionFile = %q, want %q", svc.cfg.VersionFile, "VERSION")
		}
	})
}

func TestServiceImpl_GetCurrentVersionFromTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success with existing tag", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{
				Name: "v1.2.3",
				Hash: "abc123",
			},
		}
		svc, _ := NewService(WithGitService(mock))
		v, err := svc.GetCurrentVersionFromTag(ctx, "v")
		if err != nil {
			t.Fatalf("GetCurrentVersionFromTag() error = %v, want nil", err)
		}
		if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
			t.Errorf("GetCurrentVersionFromTag() = %v.%v.%v, want 1.2.3", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("no tags returns initial version", func(t *testing.T) {
		mock := &mockGitService{
			latestTagErr: rperrors.NotFound("test", "no tags"),
		}
		svc, _ := NewService(WithGitService(mock))
		v, err := svc.GetCurrentVersionFromTag(ctx, "v")
		if err != nil {
			t.Fatalf("GetCurrentVersionFromTag() error = %v, want nil", err)
		}
		if v.Major != 0 || v.Minor != 0 || v.Patch != 0 {
			t.Errorf("GetCurrentVersionFromTag() = %v.%v.%v, want 0.0.0", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("error on non-notfound error", func(t *testing.T) {
		mock := &mockGitService{
			latestTagErr: rperrors.Version("test", "connection failed"),
		}
		svc, _ := NewService(WithGitService(mock))
		_, err := svc.GetCurrentVersionFromTag(ctx, "v")
		if err == nil {
			t.Fatal("GetCurrentVersionFromTag() error = nil, want error")
		}
	})
}

func TestServiceImpl_GetCurrentVersionFromFile(t *testing.T) {
	ctx := context.Background()
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	t.Run("plain text file", func(t *testing.T) {
		tmpDir := t.TempDir()
		versionFile := filepath.Join(tmpDir, "VERSION")
		if err := os.WriteFile(versionFile, []byte("2.0.0\n"), 0o644); err != nil {
			t.Fatalf("failed to create version file: %v", err)
		}

		v, err := svc.GetCurrentVersionFromFile(ctx, versionFile)
		if err != nil {
			t.Fatalf("GetCurrentVersionFromFile() error = %v, want nil", err)
		}
		if v.Major != 2 || v.Minor != 0 || v.Patch != 0 {
			t.Errorf("GetCurrentVersionFromFile() = %v.%v.%v, want 2.0.0", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("json file", func(t *testing.T) {
		tmpDir := t.TempDir()
		jsonFile := filepath.Join(tmpDir, "package.json")
		if err := os.WriteFile(jsonFile, []byte(`{"name": "test", "version": "3.1.4"}`), 0o644); err != nil {
			t.Fatalf("failed to create json file: %v", err)
		}

		v, err := svc.GetCurrentVersionFromFile(ctx, jsonFile)
		if err != nil {
			t.Fatalf("GetCurrentVersionFromFile() error = %v, want nil", err)
		}
		if v.Major != 3 || v.Minor != 1 || v.Patch != 4 {
			t.Errorf("GetCurrentVersionFromFile() = %v.%v.%v, want 3.1.4", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := svc.GetCurrentVersionFromFile(ctx, "/nonexistent/file")
		if err == nil {
			t.Fatal("GetCurrentVersionFromFile() error = nil, want error")
		}
		if !rperrors.IsKind(err, rperrors.KindNotFound) {
			t.Errorf("error kind = %v, want KindNotFound", rperrors.GetKind(err))
		}
	})
}

func TestServiceImpl_GetCurrentVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("from tag source", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v1.0.0"},
		}
		svc, _ := NewService(
			WithGitService(mock),
			WithVersionSource("tag"),
			WithDefaultPrefix("v"),
		)
		v, err := svc.GetCurrentVersion(ctx)
		if err != nil {
			t.Fatalf("GetCurrentVersion() error = %v, want nil", err)
		}
		if v.Major != 1 || v.Minor != 0 || v.Patch != 0 {
			t.Errorf("GetCurrentVersion() = %v.%v.%v, want 1.0.0", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("from file source", func(t *testing.T) {
		tmpDir := t.TempDir()
		versionFile := filepath.Join(tmpDir, "VERSION")
		if err := os.WriteFile(versionFile, []byte("2.5.0"), 0o644); err != nil {
			t.Fatalf("failed to create version file: %v", err)
		}

		mock := &mockGitService{}
		svc, _ := NewService(
			WithGitService(mock),
			WithVersionSource("file"),
			WithVersionFile(versionFile),
		)
		v, err := svc.GetCurrentVersion(ctx)
		if err != nil {
			t.Fatalf("GetCurrentVersion() error = %v, want nil", err)
		}
		if v.Major != 2 || v.Minor != 5 || v.Patch != 0 {
			t.Errorf("GetCurrentVersion() = %v.%v.%v, want 2.5.0", v.Major, v.Minor, v.Patch)
		}
	})

	t.Run("default to tag source", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v3.0.0"},
		}
		svc, _ := NewService(
			WithGitService(mock),
			WithVersionSource("unknown"),
		)
		v, err := svc.GetCurrentVersion(ctx)
		if err != nil {
			t.Fatalf("GetCurrentVersion() error = %v, want nil", err)
		}
		if v.Major != 3 || v.Minor != 0 || v.Patch != 0 {
			t.Errorf("GetCurrentVersion() = %v.%v.%v, want 3.0.0", v.Major, v.Minor, v.Patch)
		}
	})
}

func TestServiceImpl_CalculateNextVersion(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	tests := []struct {
		name        string
		current     *Version
		releaseType git.ReleaseType
		wantMajor   uint64
		wantMinor   uint64
		wantPatch   uint64
		wantErr     bool
	}{
		{
			name:        "major bump",
			current:     &Version{Major: 1, Minor: 2, Patch: 3},
			releaseType: git.ReleaseTypeMajor,
			wantMajor:   2,
			wantMinor:   0,
			wantPatch:   0,
		},
		{
			name:        "minor bump",
			current:     &Version{Major: 1, Minor: 2, Patch: 3},
			releaseType: git.ReleaseTypeMinor,
			wantMajor:   1,
			wantMinor:   3,
			wantPatch:   0,
		},
		{
			name:        "patch bump",
			current:     &Version{Major: 1, Minor: 2, Patch: 3},
			releaseType: git.ReleaseTypePatch,
			wantMajor:   1,
			wantMinor:   2,
			wantPatch:   4,
		},
		{
			name:        "no change",
			current:     &Version{Major: 1, Minor: 2, Patch: 3},
			releaseType: git.ReleaseTypeNone,
			wantMajor:   1,
			wantMinor:   2,
			wantPatch:   3,
		},
		{
			name:    "nil current version",
			current: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CalculateNextVersion(tt.current, tt.releaseType)
			if tt.wantErr {
				if err == nil {
					t.Fatal("CalculateNextVersion() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("CalculateNextVersion() error = %v, want nil", err)
			}
			if got.Major != tt.wantMajor || got.Minor != tt.wantMinor || got.Patch != tt.wantPatch {
				t.Errorf("CalculateNextVersion() = %v.%v.%v, want %v.%v.%v",
					got.Major, got.Minor, got.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestServiceImpl_BumpVersion(t *testing.T) {
	ctx := context.Background()

	t.Run("dry run", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v1.0.0"},
		}
		svc, _ := NewService(WithGitService(mock))

		v, err := svc.BumpVersion(ctx, BumpOptions{
			ReleaseType: git.ReleaseTypeMinor,
			DryRun:      true,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v, want nil", err)
		}
		if v.Major != 1 || v.Minor != 1 || v.Patch != 0 {
			t.Errorf("BumpVersion() = %v.%v.%v, want 1.1.0", v.Major, v.Minor, v.Patch)
		}
		if mock.createTagCalled {
			t.Error("BumpVersion() created tag in dry run")
		}
	})

	t.Run("with tag creation", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v1.0.0"},
		}
		svc, _ := NewService(WithGitService(mock))

		v, err := svc.BumpVersion(ctx, BumpOptions{
			ReleaseType: git.ReleaseTypePatch,
			Prefix:      "v",
			CreateTag:   true,
			DryRun:      false,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v, want nil", err)
		}
		if v.Major != 1 || v.Minor != 0 || v.Patch != 1 {
			t.Errorf("BumpVersion() = %v.%v.%v, want 1.0.1", v.Major, v.Minor, v.Patch)
		}
		if !mock.createTagCalled {
			t.Error("BumpVersion() did not create tag")
		}
		if mock.createTagName != "v1.0.1" {
			t.Errorf("CreateTag() name = %q, want %q", mock.createTagName, "v1.0.1")
		}
	})

	t.Run("with tag push", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v2.0.0"},
		}
		svc, _ := NewService(WithGitService(mock))

		_, err := svc.BumpVersion(ctx, BumpOptions{
			ReleaseType: git.ReleaseTypeMajor,
			Prefix:      "v",
			CreateTag:   true,
			PushTag:     true,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v, want nil", err)
		}
		if !mock.pushTagCalled {
			t.Error("BumpVersion() did not push tag")
		}
		if mock.pushTagName != "v3.0.0" {
			t.Errorf("PushTag() name = %q, want %q", mock.pushTagName, "v3.0.0")
		}
	})

	t.Run("with prerelease and metadata", func(t *testing.T) {
		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v1.0.0"},
		}
		svc, _ := NewService(WithGitService(mock))

		v, err := svc.BumpVersion(ctx, BumpOptions{
			ReleaseType: git.ReleaseTypeMinor,
			Prerelease:  "alpha.1",
			Metadata:    "build.123",
			DryRun:      true,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v, want nil", err)
		}
		if v.Prerelease != "alpha.1" {
			t.Errorf("Prerelease = %q, want %q", v.Prerelease, "alpha.1")
		}
		if v.Metadata != "build.123" {
			t.Errorf("Metadata = %q, want %q", v.Metadata, "build.123")
		}
	})

	t.Run("with version file update", func(t *testing.T) {
		tmpDir := t.TempDir()
		versionFile := filepath.Join(tmpDir, "VERSION")

		mock := &mockGitService{
			latestVersionTag: &git.Tag{Name: "v1.0.0"},
		}
		svc, _ := NewService(WithGitService(mock))

		_, err := svc.BumpVersion(ctx, BumpOptions{
			ReleaseType: git.ReleaseTypePatch,
			UpdateFile:  versionFile,
			CreateTag:   false,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v, want nil", err)
		}

		data, err := os.ReadFile(versionFile)
		if err != nil {
			t.Fatalf("failed to read version file: %v", err)
		}
		if string(data) != "1.0.1\n" {
			t.Errorf("version file content = %q, want %q", string(data), "1.0.1\n")
		}
	})
}

func TestServiceImpl_updateVersionFile(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))
	version := &Version{Major: 1, Minor: 2, Patch: 3}

	t.Run("plain text file", func(t *testing.T) {
		tmpDir := t.TempDir()
		versionFile := filepath.Join(tmpDir, "VERSION")

		err := svc.updateVersionFile(versionFile, version)
		if err != nil {
			t.Fatalf("updateVersionFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(versionFile)
		if err != nil {
			t.Fatalf("failed to read version file: %v", err)
		}
		if string(data) != "1.2.3\n" {
			t.Errorf("version file content = %q, want %q", string(data), "1.2.3\n")
		}
	})

	t.Run("json file - new", func(t *testing.T) {
		tmpDir := t.TempDir()
		jsonFile := filepath.Join(tmpDir, "package.json")

		err := svc.updateVersionFile(jsonFile, version)
		if err != nil {
			t.Fatalf("updateVersionFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(jsonFile)
		if err != nil {
			t.Fatalf("failed to read json file: %v", err)
		}
		expected := "{\n  \"version\": \"1.2.3\"\n}\n"
		if string(data) != expected {
			t.Errorf("json file content = %q, want %q", string(data), expected)
		}
	})

	t.Run("json file - existing", func(t *testing.T) {
		tmpDir := t.TempDir()
		jsonFile := filepath.Join(tmpDir, "package.json")
		if err := os.WriteFile(jsonFile, []byte(`{"name": "test", "version": "0.0.1"}`), 0o644); err != nil {
			t.Fatalf("failed to create json file: %v", err)
		}

		err := svc.updateVersionFile(jsonFile, version)
		if err != nil {
			t.Fatalf("updateVersionFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(jsonFile)
		if err != nil {
			t.Fatalf("failed to read json file: %v", err)
		}
		if !contains(string(data), `"version": "1.2.3"`) {
			t.Errorf("json file should contain updated version, got: %s", string(data))
		}
		if !contains(string(data), `"name": "test"`) {
			t.Errorf("json file should preserve name field, got: %s", string(data))
		}
	})
}

func TestServiceImpl_ParseVersion(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	tests := []struct {
		name       string
		input      string
		wantMajor  uint64
		wantMinor  uint64
		wantPatch  uint64
		wantPrerel string
		wantMeta   string
		wantErr    bool
	}{
		{
			name:      "basic version",
			input:     "1.2.3",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "with v prefix",
			input:     "v1.2.3",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:       "with prerelease",
			input:      "1.0.0-alpha.1",
			wantMajor:  1,
			wantPrerel: "alpha.1",
		},
		{
			name:      "with metadata",
			input:     "1.0.0+build.123",
			wantMajor: 1,
			wantMeta:  "build.123",
		},
		{
			name:    "invalid version",
			input:   "not-a-version",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseVersion() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion() error = %v, want nil", err)
			}
			if got.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", got.Major, tt.wantMajor)
			}
			if got.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", got.Minor, tt.wantMinor)
			}
			if got.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", got.Patch, tt.wantPatch)
			}
			if got.Prerelease != tt.wantPrerel {
				t.Errorf("Prerelease = %q, want %q", got.Prerelease, tt.wantPrerel)
			}
			if got.Metadata != tt.wantMeta {
				t.Errorf("Metadata = %q, want %q", got.Metadata, tt.wantMeta)
			}
		})
	}
}

func TestServiceImpl_FormatVersion(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	v := &Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc.1", Metadata: "build.456"}

	tests := []struct {
		name string
		opts FormatOptions
		want string
	}{
		{
			name: "basic",
			opts: FormatOptions{},
			want: "1.2.3-rc.1",
		},
		{
			name: "with prefix",
			opts: FormatOptions{IncludePrefix: true, Prefix: "v"},
			want: "v1.2.3-rc.1",
		},
		{
			name: "with metadata",
			opts: FormatOptions{IncludeMetadata: true},
			want: "1.2.3-rc.1+build.456",
		},
		{
			name: "full",
			opts: FormatOptions{IncludePrefix: true, Prefix: "v", IncludeMetadata: true},
			want: "v1.2.3-rc.1+build.456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.FormatVersion(v, tt.opts)
			if got != tt.want {
				t.Errorf("FormatVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceImpl_GenerateChangelog(t *testing.T) {
	ctx := context.Background()
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	commit := git.Commit{
		Hash:      "abc123def456",
		ShortHash: "abc123d",
		Message:   "feat: add new feature",
		Author:    git.Author{Name: "Test Author", Email: "test@example.com"},
		Date:      time.Now(),
	}

	t.Run("empty changes", func(t *testing.T) {
		_, err := svc.GenerateChangelog(ctx, nil, ChangelogOptions{})
		if err == nil {
			t.Fatal("GenerateChangelog() error = nil, want error")
		}
	})

	t.Run("with features", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Features: []git.ConventionalCommit{
				{
					Type:        git.CommitTypeFeat,
					Scope:       "api",
					Description: "add new endpoint",
					Commit:      commit,
				},
			},
		}
		opts := ChangelogOptions{
			Version: &Version{Major: 1, Minor: 0, Patch: 0},
			Date:    "2024-01-15",
			Format:  "keep-a-changelog",
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if !contains(changelog, "## [1.0.0]") {
			t.Errorf("changelog should contain version header, got: %s", changelog)
		}
		if !contains(changelog, "### Features") {
			t.Errorf("changelog should contain Features section, got: %s", changelog)
		}
		if !contains(changelog, "**api:**") {
			t.Errorf("changelog should contain scope, got: %s", changelog)
		}
	})

	t.Run("with breaking changes", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Breaking: []git.ConventionalCommit{
				{
					Type:        git.CommitTypeFeat,
					Description: "breaking change",
					Breaking:    true,
					Commit:      commit,
				},
			},
		}
		opts := ChangelogOptions{
			Format: "keep-a-changelog",
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if !contains(changelog, "### ⚠ BREAKING CHANGES") {
			t.Errorf("changelog should contain breaking changes section, got: %s", changelog)
		}
	})

	t.Run("conventional format", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Features: []git.ConventionalCommit{
				{
					Type:        git.CommitTypeFeat,
					Description: "new feature",
					Commit:      commit,
				},
			},
		}
		opts := ChangelogOptions{
			Version: &Version{Major: 1, Minor: 0, Patch: 0},
			Date:    "2024-01-15",
			Format:  "conventional",
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if !contains(changelog, "# 1.0.0") {
			t.Errorf("changelog should use conventional format header, got: %s", changelog)
		}
	})

	t.Run("with commit links", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Features: []git.ConventionalCommit{
				{
					Type:        git.CommitTypeFeat,
					Description: "feature with link",
					Commit:      commit,
				},
			},
		}
		opts := ChangelogOptions{
			RepositoryURL:     "https://github.com/user/repo",
			IncludeCommitHash: true,
			LinkCommits:       true,
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if !contains(changelog, "https://github.com/user/repo/commit/") {
			t.Errorf("changelog should contain commit link, got: %s", changelog)
		}
	})

	t.Run("with author", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Fixes: []git.ConventionalCommit{
				{
					Type:        git.CommitTypeFix,
					Description: "fix with author",
					Commit:      commit,
				},
			},
		}
		opts := ChangelogOptions{
			IncludeAuthor: true,
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if !contains(changelog, "Test Author") {
			t.Errorf("changelog should contain author, got: %s", changelog)
		}
	})

	t.Run("with issue references", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Features: []git.ConventionalCommit{
				{
					Type:        git.CommitTypeFeat,
					Description: "feature with issue",
					References: []git.Reference{
						{Type: "issue", ID: "123"},
					},
					Commit: commit,
				},
			},
		}
		opts := ChangelogOptions{
			RepositoryURL: "https://github.com/user/repo",
			LinkIssues:    true,
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if !contains(changelog, "#123") {
			t.Errorf("changelog should contain issue reference, got: %s", changelog)
		}
	})

	t.Run("exclude types", func(t *testing.T) {
		changes := &git.CategorizedChanges{
			Features: []git.ConventionalCommit{
				{Type: git.CommitTypeFeat, Description: "feature", Commit: commit},
			},
			Other: []git.ConventionalCommit{
				{Type: git.CommitTypeChore, Description: "chore", Commit: commit},
			},
		}
		opts := ChangelogOptions{
			Exclude: []string{"chore"},
		}

		changelog, err := svc.GenerateChangelog(ctx, changes, opts)
		if err != nil {
			t.Fatalf("GenerateChangelog() error = %v, want nil", err)
		}
		if contains(changelog, "chore") {
			t.Errorf("changelog should exclude chore commits, got: %s", changelog)
		}
	})
}

func TestServiceImpl_UpdateChangelogFile(t *testing.T) {
	ctx := context.Background()
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	t.Run("new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		changelogFile := filepath.Join(tmpDir, "CHANGELOG.md")

		content := "## [1.0.0] - 2024-01-15\n\n### Features\n\n* New feature\n"
		err := svc.UpdateChangelogFile(ctx, changelogFile, content, &Version{Major: 1, Minor: 0, Patch: 0})
		if err != nil {
			t.Fatalf("UpdateChangelogFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(changelogFile)
		if err != nil {
			t.Fatalf("failed to read changelog file: %v", err)
		}
		if !contains(string(data), "# Changelog") {
			t.Errorf("changelog should contain header, got: %s", string(data))
		}
		if !contains(string(data), "## [1.0.0]") {
			t.Errorf("changelog should contain version, got: %s", string(data))
		}
	})

	t.Run("existing file with versions", func(t *testing.T) {
		tmpDir := t.TempDir()
		changelogFile := filepath.Join(tmpDir, "CHANGELOG.md")
		existing := "# Changelog\n\n## [0.1.0] - 2024-01-01\n\n### Features\n\n* Initial release\n"
		if err := os.WriteFile(changelogFile, []byte(existing), 0o644); err != nil {
			t.Fatalf("failed to create existing changelog: %v", err)
		}

		content := "## [1.0.0] - 2024-01-15\n\n### Features\n\n* New feature\n\n"
		err := svc.UpdateChangelogFile(ctx, changelogFile, content, &Version{Major: 1, Minor: 0, Patch: 0})
		if err != nil {
			t.Fatalf("UpdateChangelogFile() error = %v, want nil", err)
		}

		data, err := os.ReadFile(changelogFile)
		if err != nil {
			t.Fatalf("failed to read changelog file: %v", err)
		}
		// Check that new version appears before old version
		newPos := indexOf(string(data), "## [1.0.0]")
		oldPos := indexOf(string(data), "## [0.1.0]")
		if newPos >= oldPos || newPos == -1 {
			t.Errorf("new version should appear before old version, got: %s", string(data))
		}
	})
}

func TestServiceImpl_ReadChangelogSection(t *testing.T) {
	ctx := context.Background()
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	t.Run("read existing version", func(t *testing.T) {
		tmpDir := t.TempDir()
		changelogFile := filepath.Join(tmpDir, "CHANGELOG.md")
		content := `# Changelog

## [1.0.0] - 2024-01-15

### Features

* New feature

## [0.1.0] - 2024-01-01

### Features

* Initial release
`
		if err := os.WriteFile(changelogFile, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create changelog: %v", err)
		}

		section, err := svc.ReadChangelogSection(ctx, changelogFile, "1.0.0")
		if err != nil {
			t.Fatalf("ReadChangelogSection() error = %v, want nil", err)
		}
		if !contains(section, "## [1.0.0]") {
			t.Errorf("section should contain version header, got: %s", section)
		}
		if !contains(section, "New feature") {
			t.Errorf("section should contain feature, got: %s", section)
		}
		if contains(section, "## [0.1.0]") {
			t.Errorf("section should not contain other version, got: %s", section)
		}
	})

	t.Run("version not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		changelogFile := filepath.Join(tmpDir, "CHANGELOG.md")
		content := "# Changelog\n\n## [1.0.0] - 2024-01-15\n"
		if err := os.WriteFile(changelogFile, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create changelog: %v", err)
		}

		_, err := svc.ReadChangelogSection(ctx, changelogFile, "2.0.0")
		if err == nil {
			t.Fatal("ReadChangelogSection() error = nil, want error")
		}
		if !rperrors.IsKind(err, rperrors.KindNotFound) {
			t.Errorf("error kind = %v, want KindNotFound", rperrors.GetKind(err))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := svc.ReadChangelogSection(ctx, "/nonexistent/CHANGELOG.md", "1.0.0")
		if err == nil {
			t.Fatal("ReadChangelogSection() error = nil, want error")
		}
	})
}

func TestMatchesVersionHeader(t *testing.T) {
	tests := []struct {
		line    string
		version string
		want    bool
	}{
		{"## [1.0.0] - 2024-01-15", "1.0.0", true},
		{"## 1.0.0 - 2024-01-15", "1.0.0", true},
		{"## [1.0.0]", "1.0.0", true},
		{"## 1.0.0", "1.0.0", true},
		{"## [2.0.0] - 2024-01-15", "1.0.0", false},
		{"# [1.0.0]", "1.0.0", false},
		{"### [1.0.0]", "1.0.0", false},
		{"## [1.0.0-alpha]", "1.0.0", false},
		{"", "1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.line+"_"+tt.version, func(t *testing.T) {
			if got := matchesVersionHeader(tt.line, tt.version); got != tt.want {
				t.Errorf("matchesVersionHeader(%q, %q) = %v, want %v", tt.line, tt.version, got, tt.want)
			}
		})
	}
}

func TestFindChangelogInsertPosition(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(pos int) bool
	}{
		{
			name:    "before first version header",
			content: "# Changelog\n\n## [1.0.0] - 2024-01-15\n",
			check:   func(pos int) bool { return pos == 13 }, // After "# Changelog\n\n"
		},
		{
			name:    "after unreleased section",
			content: "# Changelog\n\n## [Unreleased]\n\n## [1.0.0] - 2024-01-15\n",
			check:   func(pos int) bool { return pos == 30 }, // After "## [Unreleased]\n\n"
		},
		{
			name:    "empty content",
			content: "",
			check:   func(pos int) bool { return pos == 0 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := findChangelogInsertPosition(tt.content)
			if !tt.check(pos) {
				t.Errorf("findChangelogInsertPosition() = %d, check failed", pos)
			}
		})
	}
}

func TestServiceImpl_ValidateVersion(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	tests := []struct {
		version string
		wantErr bool
	}{
		{"1.0.0", false},
		{"v1.0.0", false},
		{"1.0.0-alpha", false},
		{"1.0.0+build", false},
		{"1.0", false}, // semver library accepts this as 1.0.0
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			err := svc.ValidateVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
		})
	}
}

func TestServiceImpl_CompareVersions(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	tests := []struct {
		name string
		v1   *Version
		v2   *Version
		want int
	}{
		{
			name: "equal versions",
			v1:   &Version{Major: 1, Minor: 0, Patch: 0},
			v2:   &Version{Major: 1, Minor: 0, Patch: 0},
			want: 0,
		},
		{
			name: "v1 major greater",
			v1:   &Version{Major: 2, Minor: 0, Patch: 0},
			v2:   &Version{Major: 1, Minor: 0, Patch: 0},
			want: 1,
		},
		{
			name: "v1 major less",
			v1:   &Version{Major: 1, Minor: 0, Patch: 0},
			v2:   &Version{Major: 2, Minor: 0, Patch: 0},
			want: -1,
		},
		{
			name: "v1 minor greater",
			v1:   &Version{Major: 1, Minor: 2, Patch: 0},
			v2:   &Version{Major: 1, Minor: 1, Patch: 0},
			want: 1,
		},
		{
			name: "v1 patch greater",
			v1:   &Version{Major: 1, Minor: 0, Patch: 2},
			v2:   &Version{Major: 1, Minor: 0, Patch: 1},
			want: 1,
		},
		{
			name: "no prerelease is greater",
			v1:   &Version{Major: 1, Minor: 0, Patch: 0},
			v2:   &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want: 1,
		},
		{
			name: "prerelease comparison",
			v1:   &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			v2:   &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "beta"},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.CompareVersions(tt.v1, tt.v2)
			if got != tt.want {
				t.Errorf("CompareVersions() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestServiceImpl_IsPrerelease(t *testing.T) {
	mock := &mockGitService{}
	svc, _ := NewService(WithGitService(mock))

	tests := []struct {
		name    string
		version *Version
		want    bool
	}{
		{
			name:    "not prerelease",
			version: &Version{Major: 1, Minor: 0, Patch: 0},
			want:    false,
		},
		{
			name:    "is prerelease",
			version: &Version{Major: 1, Minor: 0, Patch: 0, Prerelease: "alpha"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.IsPrerelease(tt.version)
			if got != tt.want {
				t.Errorf("IsPrerelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithGitService(t *testing.T) {
	mock := &mockGitService{}
	cfg := DefaultServiceConfig()
	WithGitService(mock)(&cfg)
	if cfg.GitService != mock {
		t.Error("WithGitService() did not set git service")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return indexOf(s, substr) != -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
