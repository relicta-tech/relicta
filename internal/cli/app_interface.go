// Package cli defines interfaces for injecting the container into commands.
package cli

import (
	"context"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/container"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

type calculateVersionUseCase interface {
	Execute(context.Context, versioning.CalculateVersionInput) (*versioning.CalculateVersionOutput, error)
}

type cliApp interface {
	Close() error
	GitAdapter() sourcecontrol.GitRepository
	ReleaseRepository() domainrelease.Repository
	ReleaseAnalyzer() *servicerelease.Analyzer
	CalculateVersion() calculateVersionUseCase
	HasAI() bool
	AI() ai.Service
	HasGovernance() bool
	GovernanceService() *governance.Service

	// Release workflow services (DDD layer)
	InitReleaseServices(ctx context.Context, repoRoot string) error
	ReleaseServices() *domainrelease.Services
	HasReleaseServices() bool
}

var newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
	c, err := container.New(cfg)
	if err != nil {
		return nil, err
	}
	// Forward the operator's --allow-untrusted-plugins opt-in BEFORE Initialize
	// so the plugin manager picks it up at construction time (the trust gate
	// is read inside initPluginSystem).
	c.SetAllowUntrustedPlugins(allowUntrustedPlugins)

	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	return &containerAppWrapper{App: c}, nil
}

type containerAppWrapper struct {
	*container.App
}

func (w *containerAppWrapper) ReleaseAnalyzer() *servicerelease.Analyzer {
	return w.App.ReleaseAnalyzer()
}

func (w *containerAppWrapper) CalculateVersion() calculateVersionUseCase {
	return w.App.CalculateVersion()
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
