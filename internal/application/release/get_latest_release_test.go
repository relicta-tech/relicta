// Package release provides application use cases for release management.
package release

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/release"
)

// mockReleaseRepo is a mock implementation of release.Repository for testing.
type mockReleaseRepo struct {
	findLatestFn func(ctx context.Context, repoPath string) (*release.ReleaseRun, error)
}

func (m *mockReleaseRepo) Save(_ context.Context, _ *release.ReleaseRun) error {
	return nil
}

func (m *mockReleaseRepo) FindByID(_ context.Context, _ release.RunID) (*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*release.ReleaseRun, error) {
	if m.findLatestFn != nil {
		return m.findLatestFn(ctx, repoPath)
	}
	return nil, release.ErrRunNotFound
}

func (m *mockReleaseRepo) FindByState(_ context.Context, _ release.RunState) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepo) FindActive(_ context.Context) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepo) FindBySpecification(_ context.Context, _ release.Specification) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepo) Delete(_ context.Context, _ release.RunID) error {
	return nil
}

func TestGetLatestReleaseInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   GetLatestReleaseInput
		wantErr bool
	}{
		{
			name:    "empty repository path",
			input:   GetLatestReleaseInput{RepositoryPath: ""},
			wantErr: true,
		},
		{
			name:    "path traversal attempt",
			input:   GetLatestReleaseInput{RepositoryPath: "/path/../../../etc/passwd"},
			wantErr: true,
		},
		{
			name:    "valid path",
			input:   GetLatestReleaseInput{RepositoryPath: "/path/to/repo"},
			wantErr: false,
		},
		{
			name:    "relative path",
			input:   GetLatestReleaseInput{RepositoryPath: "./repo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetLatestReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("returns latest release when found", func(t *testing.T) {
		expectedRelease := release.NewRelease("test-release-1", "main", "/path/to/repo")

		repo := &mockReleaseRepo{
			findLatestFn: func(_ context.Context, repoPath string) (*release.ReleaseRun, error) {
				if repoPath == "/path/to/repo" {
					return expectedRelease, nil
				}
				return nil, release.ErrRunNotFound
			},
		}

		uc := NewGetLatestReleaseUseCase(repo)
		output, err := uc.Execute(ctx, GetLatestReleaseInput{RepositoryPath: "/path/to/repo"})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if !output.HasRelease {
			t.Error("Execute() HasRelease = false, want true")
		}
		if output.Release.ID() != expectedRelease.ID() {
			t.Errorf("Execute() Release.ID() = %v, want %v", output.Release.ID(), expectedRelease.ID())
		}
		if output.Branch != "main" {
			t.Errorf("Execute() Branch = %v, want main", output.Branch)
		}
	})

	t.Run("returns HasRelease=false when no release found", func(t *testing.T) {
		repo := &mockReleaseRepo{
			findLatestFn: func(_ context.Context, _ string) (*release.ReleaseRun, error) {
				return nil, release.ErrRunNotFound
			},
		}

		uc := NewGetLatestReleaseUseCase(repo)
		output, err := uc.Execute(ctx, GetLatestReleaseInput{RepositoryPath: "/path/to/repo"})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if output.HasRelease {
			t.Error("Execute() HasRelease = true, want false")
		}
		if output.RepositoryPath != "/path/to/repo" {
			t.Errorf("Execute() RepositoryPath = %v, want /path/to/repo", output.RepositoryPath)
		}
	})

	t.Run("returns error for invalid input", func(t *testing.T) {
		repo := &mockReleaseRepo{}
		uc := NewGetLatestReleaseUseCase(repo)

		_, err := uc.Execute(ctx, GetLatestReleaseInput{RepositoryPath: ""})

		if err == nil {
			t.Error("Execute() expected error for invalid input")
		}
	})

	t.Run("returns error for repository error", func(t *testing.T) {
		repo := &mockReleaseRepo{
			findLatestFn: func(_ context.Context, _ string) (*release.ReleaseRun, error) {
				return nil, context.DeadlineExceeded
			},
		}

		uc := NewGetLatestReleaseUseCase(repo)
		_, err := uc.Execute(ctx, GetLatestReleaseInput{RepositoryPath: "/path/to/repo"})

		if err == nil {
			t.Error("Execute() expected error for repository error")
		}
	})
}
