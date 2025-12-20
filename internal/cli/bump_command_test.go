package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

type fakeSetVersionUseCase struct {
	output        *versioning.SetVersionOutput
	executeCalled bool
	input         versioning.SetVersionInput
}

func (f *fakeSetVersionUseCase) Execute(ctx context.Context, input versioning.SetVersionInput) (*versioning.SetVersionOutput, error) {
	f.executeCalled = true
	f.input = input
	if f.output != nil {
		return f.output, nil
	}
	return &versioning.SetVersionOutput{Version: input.Version, TagName: cfg.Versioning.TagPrefix + input.Version.String()}, nil
}

type stubReleaseRepo struct {
	findLatestErr error
}

func (s stubReleaseRepo) Save(ctx context.Context, release *release.Release) error {
	return nil
}

func (s stubReleaseRepo) FindByID(ctx context.Context, id release.ReleaseID) (*release.Release, error) {
	return nil, nil
}

func (s stubReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*release.Release, error) {
	return nil, s.findLatestErr
}

func (s stubReleaseRepo) FindByState(ctx context.Context, state release.ReleaseState) ([]*release.Release, error) {
	return nil, nil
}

func (s stubReleaseRepo) FindActive(ctx context.Context) ([]*release.Release, error) {
	return nil, nil
}

func (s stubReleaseRepo) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.Release, error) {
	return nil, nil
}

func (s stubReleaseRepo) Delete(ctx context.Context, id release.ReleaseID) error {
	return nil
}

func TestHandleForcedVersion_Succeeds(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = &config.Config{
		Versioning: config.VersioningConfig{TagPrefix: "v"},
	}

	fake := &fakeSetVersionUseCase{}
	app := testCLIApp{
		gitRepo:      stubGitRepo{},
		setVersionUC: fake,
		releaseRepo:  stubReleaseRepo{findLatestErr: errors.New("not found")},
	}

	if err := handleForcedVersion(context.Background(), app, "1.2.3"); err != nil {
		t.Fatalf("handleForcedVersion error: %v", err)
	}
	if !fake.executeCalled {
		t.Fatal("expected SetVersion execute called")
	}
	if fake.input.Version.String() != "1.2.3" {
		t.Fatalf("unexpected version %q", fake.input.Version.String())
	}
}

func TestHandleForcedVersion_InvalidVersion(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()
	fake := &fakeSetVersionUseCase{}
	app := testCLIApp{
		gitRepo:      stubGitRepo{},
		setVersionUC: fake,
		releaseRepo:  stubReleaseRepo{},
	}
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	if err := handleForcedVersion(context.Background(), app, "not-a-version"); err == nil {
		t.Fatal("expected error for invalid version")
	}
	if fake.executeCalled {
		t.Fatal("SetVersion should not be executed on invalid input")
	}
}
