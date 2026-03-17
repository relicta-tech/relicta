package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

type stubReleaseRepo struct {
	findLatestErr error
}

func (s stubReleaseRepo) Save(ctx context.Context, release *release.ReleaseRun) error {
	return nil
}

func (s stubReleaseRepo) FindByID(ctx context.Context, id release.RunID) (*release.ReleaseRun, error) {
	return nil, nil
}

func (s stubReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*release.ReleaseRun, error) {
	return nil, s.findLatestErr
}

func (s stubReleaseRepo) FindByState(ctx context.Context, state release.RunState) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (s stubReleaseRepo) FindActive(ctx context.Context) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (s stubReleaseRepo) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (s stubReleaseRepo) Delete(ctx context.Context, id release.RunID) error {
	return nil
}

func (s stubReleaseRepo) List(ctx context.Context, repoPath string) ([]release.RunID, error) {
	return nil, nil
}

func TestHandleForcedVersion_Succeeds(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = &config.Config{
		Versioning: config.VersioningConfig{TagPrefix: "v"},
	}

	app := testCLIApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: stubReleaseRepo{findLatestErr: release.ErrRunNotFound},
	}

	// handleForcedVersion no longer calls SetVersion - it just updates release state
	// and prints the version info. Tag creation happens during publish.
	if err := handleForcedVersion(context.Background(), app, "1.2.3"); err != nil {
		t.Fatalf("handleForcedVersion error: %v", err)
	}
}

func TestHandleForcedVersion_InvalidVersion(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	app := testCLIApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: stubReleaseRepo{},
	}
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	if err := handleForcedVersion(context.Background(), app, "not-a-version"); err == nil {
		t.Fatal("expected error for invalid version")
	}
}
