// Package release provides application use cases for release management.
package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// GetLatestReleaseInput represents the input for the GetLatestRelease use case.
type GetLatestReleaseInput struct {
	RepositoryPath string
}

// Validate validates the GetLatestReleaseInput.
func (i *GetLatestReleaseInput) Validate() error {
	if i.RepositoryPath == "" {
		return fmt.Errorf("repository path is required")
	}

	// Check for path traversal attempts by looking for ".." in the original path
	if strings.Contains(i.RepositoryPath, "..") {
		return fmt.Errorf("repository path contains invalid traversal: %s", i.RepositoryPath)
	}

	return nil
}

// GetLatestReleaseOutput represents the output of the GetLatestRelease use case.
type GetLatestReleaseOutput struct {
	Release        *release.Release
	Version        *version.SemanticVersion
	State          release.ReleaseState
	RepositoryPath string
	Branch         string
	HasRelease     bool
}

// GetLatestReleaseUseCase implements the get latest release use case.
type GetLatestReleaseUseCase struct {
	releaseRepo release.Repository
}

// NewGetLatestReleaseUseCase creates a new GetLatestReleaseUseCase.
func NewGetLatestReleaseUseCase(releaseRepo release.Repository) *GetLatestReleaseUseCase {
	return &GetLatestReleaseUseCase{
		releaseRepo: releaseRepo,
	}
}

// Execute executes the get latest release use case.
func (uc *GetLatestReleaseUseCase) Execute(ctx context.Context, input GetLatestReleaseInput) (*GetLatestReleaseOutput, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Find the latest release for the repository
	rel, err := uc.releaseRepo.FindLatest(ctx, input.RepositoryPath)
	if err != nil {
		if errors.Is(err, release.ErrRunNotFound) {
			// No release found - return empty output with HasRelease=false
			return &GetLatestReleaseOutput{
				RepositoryPath: input.RepositoryPath,
				HasRelease:     false,
			}, nil
		}
		return nil, fmt.Errorf("failed to find latest release: %w", err)
	}

	return &GetLatestReleaseOutput{
		Release:        rel,
		Version:        rel.Version(),
		State:          rel.State(),
		RepositoryPath: rel.RepoRoot(),
		Branch:         rel.Branch(),
		HasRelease:     true,
	}, nil
}
