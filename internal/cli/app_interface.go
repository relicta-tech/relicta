// Package cli defines interfaces for injecting the container into commands.
package cli

import (
	"context"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/container"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
)

type planReleaseUseCase interface {
	Execute(context.Context, apprelease.PlanReleaseInput) (*apprelease.PlanReleaseOutput, error)
	AnalyzeCommits(context.Context, apprelease.PlanReleaseInput) (*analysis.AnalysisResult, []analysis.CommitInfo, error)
}

type generateNotesUseCase interface {
	Execute(context.Context, apprelease.GenerateNotesInput) (*apprelease.GenerateNotesOutput, error)
}

type approveReleaseUseCase interface {
	Execute(context.Context, apprelease.ApproveReleaseInput) (*apprelease.ApproveReleaseOutput, error)
}

type publishReleaseUseCase interface {
	Execute(context.Context, apprelease.PublishReleaseInput) (*apprelease.PublishReleaseOutput, error)
}

type calculateVersionUseCase interface {
	Execute(context.Context, versioning.CalculateVersionInput) (*versioning.CalculateVersionOutput, error)
}

type setVersionUseCase interface {
	Execute(context.Context, versioning.SetVersionInput) (*versioning.SetVersionOutput, error)
}

type cliApp interface {
	Close() error
	GitAdapter() sourcecontrol.GitRepository
	ReleaseRepository() domainrelease.Repository
	PlanRelease() planReleaseUseCase
	GenerateNotes() generateNotesUseCase
	ApproveRelease() approveReleaseUseCase
	PublishRelease() publishReleaseUseCase
	CalculateVersion() calculateVersionUseCase
	SetVersion() setVersionUseCase
	HasAI() bool
	AI() ai.Service
	HasGovernance() bool
	GovernanceService() *governance.Service

	// Release workflow services
	InitReleaseServices(ctx context.Context, repoRoot string) error
	ReleaseServices() *domainrelease.Services
	HasReleaseServices() bool
}

var newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
	app, err := container.NewInitialized(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &containerAppWrapper{App: app}, nil
}

type containerAppWrapper struct {
	*container.App
}

func (w *containerAppWrapper) PlanRelease() planReleaseUseCase {
	return w.App.PlanRelease()
}

func (w *containerAppWrapper) GenerateNotes() generateNotesUseCase {
	return w.App.GenerateNotes()
}

func (w *containerAppWrapper) ApproveRelease() approveReleaseUseCase {
	return w.App.ApproveRelease()
}

func (w *containerAppWrapper) PublishRelease() publishReleaseUseCase {
	return w.App.PublishRelease()
}

func (w *containerAppWrapper) CalculateVersion() calculateVersionUseCase {
	return w.App.CalculateVersion()
}

func (w *containerAppWrapper) SetVersion() setVersionUseCase {
	return w.App.SetVersion()
}

func (w *containerAppWrapper) AI() ai.Service {
	return w.App.AI()
}

func (w *containerAppWrapper) InitReleaseServices(ctx context.Context, repoRoot string) error {
	return w.App.InitReleaseServices(ctx, repoRoot)
}

func (w *containerAppWrapper) ReleaseServices() *domainrelease.Services {
	return w.App.ReleaseServices()
}

func (w *containerAppWrapper) HasReleaseServices() bool {
	return w.App.HasReleaseServices()
}
