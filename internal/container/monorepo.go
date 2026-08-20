package container

import (
	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
	infraworkspace "github.com/relicta-tech/relicta/v4/internal/infrastructure/workspace"
)

// NewMonorepoBumper builds the per-package versioning service `relicta bump` uses when
// monorepo.enabled is set.
//
// It lives here for the same reason NewMultirepoCoordinator does: the composition root owns
// the infrastructure choice. Package discovery is a filesystem walk — internal/infrastructure/
// workspace — and the CLI may not import it, which the hexagonal fitness function in
// internal/architecture enforces.
//
// aiService may be nil. The analysis factory then yields an analyzer that classifies commits
// by their conventional-commit prefix alone, which is what an unconfigured repository already
// gets everywhere else; per-package versioning must not require an API key.
func NewMonorepoBumper(gitRepo sourcecontrol.GitRepository, aiService ai.Service) *appmonorepo.BumpService {
	analyzer := appmonorepo.NewMonorepoAnalyzer(
		gitRepo,
		&version.DefaultVersionCalculator{},
		analysisfactory.NewFactory(aiService),
		appmonorepo.NewCompositeVersionWriter(),
	)
	return appmonorepo.NewBumpService(infraworkspace.NewFileDetector(), analyzer, gitRepo)
}
