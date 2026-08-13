package container

import (
	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	inframultirepo "github.com/relicta-tech/relicta/v4/internal/infrastructure/multirepo"
)

// NewMultirepoCoordinator builds the coordinator for group commands, with a git adapter that
// can read each member repository.
//
// It lives here because the container is the composition root: the CLI must not import
// internal/infrastructure directly, which the hexagonal fitness function in
// internal/architecture enforces, and the application layer must not either. The CLI built
// this coordinator itself as NewCoordinator(nil, nil) — with a comment saying real adapters
// "would be injected through the container in a production setup" — and every `relicta group
// plan` panicked on the first repository as a result.
//
// Deliberately a plain constructor rather than an App method: group commands run without an
// initialized container, and requiring one for a command that only reads sibling checkouts
// would be a heavier change than the layering rule asks for.
//
// The release executor is nil. Running a full release inside another checkout — its config,
// its plugins, its approval state — is a separate feature, and Coordinator.Execute refuses
// clearly rather than dereferencing it.
func NewMultirepoCoordinator(tagPrefix string) *appmultirepo.Coordinator {
	return appmultirepo.NewCoordinator(inframultirepo.NewGitAdapter(tagPrefix), nil)
}
