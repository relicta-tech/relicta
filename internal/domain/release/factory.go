// Package release provides the release governance bounded context.
// This is the entry point for creating and using the domain release services.
package release

import (
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
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

	// AttestationEnabled adds an attestation step to the execution plan at approval.
	//
	// Without it the step is never planned, so `attestation: enabled: true` produced no
	// attestation however the publisher was configured — the feature was dead at two
	// levels, and the publish step that would have generated one reported success while
	// skipping. `relicta verify` exists to check an attestation and had nothing to check.
	AttestationEnabled bool

	// EventPublisher receives the aggregate's domain events after each successful
	// save. Optional; without it a release emits no events.
	//
	// This is how the outcome tracker and the webhook publisher are reached. Before it
	// existed, the only production caller of any EventPublisher was FileUnitOfWork,
	// which nothing constructed outside a test — so the container assembled the full
	// chain, logged it as initialized, and no release ever published an event. The
	// visible effects were that configured webhooks never fired for a release and that
	// no failed or canceled run was ever recorded, leaving change failure rate to be
	// computed from a history that held almost only successes.
	EventPublisher ports.EventPublisher
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
	var repository ports.ReleaseRunRepository = adapters.NewFileReleaseRunRepository()
	lockManager := adapters.NewFileLockManager()

	// Publication is a decorator on save rather than a call in each use case: every use
	// case already persists through this repository, and there are ten such calls across
	// plan, bump, notes, approve, publish and retry. One seam cannot be forgotten by the
	// eleventh.
	if cfg.EventPublisher != nil {
		repository = adapters.NewEventPublishingRepository(adapters.EventPublishingConfig{
			Repository: repository,
			Publisher:  cfg.EventPublisher,
		})
	}

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
	// Planned at approval, because that is when the execution plan is fixed. Nothing
	// called this, so the step was never in the plan and the attestation the publisher
	// would have generated was never asked for.
	approveRelease.SetAttestationEnabled(cfg.AttestationEnabled)

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
