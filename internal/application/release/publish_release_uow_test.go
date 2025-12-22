package release

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

type mockPublishUnitOfWork struct {
	repo           *mockReleaseRepository
	commitErr      error
	commitCalled   bool
	rollbackCalled bool
}

func (u *mockPublishUnitOfWork) Commit(ctx context.Context) error {
	u.commitCalled = true
	if u.commitErr != nil {
		return u.commitErr
	}
	return nil
}

func (u *mockPublishUnitOfWork) Rollback() error {
	u.rollbackCalled = true
	return nil
}

func (u *mockPublishUnitOfWork) ReleaseRepository() release.Repository {
	return u.repo
}

type mockPublishUnitOfWorkFactory struct {
	beginErr error
	uow      *mockPublishUnitOfWork
}

func (f *mockPublishUnitOfWorkFactory) Begin(ctx context.Context) (release.UnitOfWork, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	if f.uow == nil {
		f.uow = &mockPublishUnitOfWork{
			repo: newMockReleaseRepository(),
		}
	}
	return f.uow, nil
}

func TestNewPublishReleaseUseCaseWithUoW(t *testing.T) {
	factory := &mockPublishUnitOfWorkFactory{}
	uc := NewPublishReleaseUseCaseWithUoW(factory, &mockGitRepository{}, newMockPluginExecutor(), &mockEventPublisher{})
	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
	if uc.unitOfWorkFactory == nil {
		t.Fatal("expected unitOfWorkFactory to be set")
	}
}

func TestPublishReleaseUseCase_ExecuteWithUnitOfWork(t *testing.T) {
	ctx := context.Background()
	factory := &mockPublishUnitOfWorkFactory{
		uow: &mockPublishUnitOfWork{
			repo: newMockReleaseRepository(),
		},
	}
	releaseID := release.ReleaseID("release-123")
	r := createApprovedRelease(releaseID, "main", "/tmp/repo")
	factory.uow.repo.releases[releaseID] = r

	gitRepo := &mockGitRepository{
		latestCommit: createTestCommit("abc123", "latest"),
		tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
	}

	uc := NewPublishReleaseUseCaseWithUoW(factory, gitRepo, newMockPluginExecutor(), &mockEventPublisher{})

	input := PublishReleaseInput{
		ReleaseID: releaseID,
		CreateTag: true,
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == nil {
		t.Fatal("expected output")
	}
	if output.TagName != "v1.1.0" {
		t.Fatalf("TagName = %s, want v1.1.0", output.TagName)
	}

	if !factory.uow.commitCalled {
		t.Error("expected commit to be called")
	}
	if !factory.uow.repo.saveCalled {
		t.Error("expected release to be saved via UoW")
	}
}

func TestPublishReleaseUseCase_ExecuteWithUnitOfWork_BeginError(t *testing.T) {
	ctx := context.Background()
	factory := &mockPublishUnitOfWorkFactory{
		beginErr: errors.New("failure"),
	}
	uc := NewPublishReleaseUseCaseWithUoW(factory, &mockGitRepository{}, newMockPluginExecutor(), &mockEventPublisher{})

	_, err := uc.Execute(ctx, PublishReleaseInput{
		ReleaseID: "release-123",
		CreateTag: false,
	})
	if err == nil {
		t.Fatal("expected error when Begin fails")
	}
	if !errors.Is(err, factory.beginErr) {
		t.Fatalf("expected begin error, got %v", err)
	}
}
