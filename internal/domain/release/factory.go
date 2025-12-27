// Package release provides the release governance bounded context.
// This is the entry point for creating and using the DDD-based release services.
package release

import (
	"github.com/relicta-tech/relicta/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/internal/domain/release/app"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

// Services provides access to all release governance use cases.
type Services struct {
	PlanRelease    *app.PlanReleaseUseCase
	BumpVersion    *app.BumpVersionUseCase
	GenerateNotes  *app.GenerateNotesUseCase
	ApproveRelease *app.ApproveReleaseUseCase
	PublishRelease *app.PublishReleaseUseCase
	RetryPublish   *app.RetryPublishUseCase
	GetStatus      *app.GetStatusUseCase

	// Infrastructure
	Repository    ports.ReleaseRunRepository
	RepoInspector ports.RepoInspector
	LockManager   ports.LockManager
	StateMachine  *domain.StateMachineService
}

// Config contains configuration for creating services.
type Config struct {
	// RepoRoot is the root path of the repository.
	RepoRoot string

	// GitAdapter is the git repository interface.
	GitAdapter sourcecontrol.GitRepository

	// NotesGenerator generates release notes. Optional.
	NotesGenerator ports.NotesGenerator

	// Publisher executes publish steps. Optional.
	Publisher ports.Publisher

	// VersionWriter writes version files. Optional.
	VersionWriter ports.VersionWriter
}

// NewServices creates a new set of release governance services.
func NewServices(cfg Config) (*Services, error) {
	// Create state machine service
	stateMachine, err := domain.NewStateMachineService()
	if err != nil {
		return nil, err
	}

	// Create infrastructure adapters
	repoInspector := adapters.NewGitRepoInspector(cfg.GitAdapter)

	// Create file-based repository and lock manager
	repository := adapters.NewFileReleaseRunRepository()
	lockManager := adapters.NewFileLockManager()

	// Create use cases
	planRelease := app.NewPlanReleaseUseCase(
		repository,
		repoInspector,
		stateMachine,
	)

	bumpVersion := app.NewBumpVersionUseCase(
		repository,
		repoInspector,
		lockManager,
		cfg.VersionWriter,
		stateMachine,
	)

	generateNotes := app.NewGenerateNotesUseCase(
		repository,
		repoInspector,
		cfg.NotesGenerator,
		stateMachine,
	)

	approveRelease := app.NewApproveReleaseUseCase(
		repository,
		repoInspector,
		lockManager,
		stateMachine,
	)

	publishRelease := app.NewPublishReleaseUseCase(
		repository,
		repoInspector,
		lockManager,
		cfg.Publisher,
		stateMachine,
	)

	retryPublish := app.NewRetryPublishUseCase(
		repository,
		repoInspector,
		lockManager,
		cfg.Publisher,
		stateMachine,
	)

	getStatus := app.NewGetStatusUseCase(
		repository,
		repoInspector,
	)

	return &Services{
		PlanRelease:    planRelease,
		BumpVersion:    bumpVersion,
		GenerateNotes:  generateNotes,
		ApproveRelease: approveRelease,
		PublishRelease: publishRelease,
		RetryPublish:   retryPublish,
		GetStatus:      getStatus,
		Repository:     repository,
		RepoInspector:  repoInspector,
		LockManager:    lockManager,
		StateMachine:   stateMachine,
	}, nil
}

// ExportStateMachineJSON exports the state machine definition as XState-compatible JSON.
func (s *Services) ExportStateMachineJSON() ([]byte, error) {
	return s.StateMachine.ExportMachineJSON()
}
