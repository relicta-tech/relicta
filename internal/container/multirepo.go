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
// The executor plans any member and publishes an approved one. It approves nothing itself:
// a group release that could approve on behalf of a member would let somebody bypass that
// member's policy by adding it to a group.
func NewMultirepoCoordinator(tagPrefix string) *appmultirepo.Coordinator {
	return appmultirepo.NewCoordinator(
		inframultirepo.NewGitAdapter(tagPrefix),
		NewGroupExecutor(tagPrefix, nil),
	)
}

// NewMultirepoReadiness returns the readiness reporter for group commands.
//
// Here for the same reason the coordinator is: the CLI reports readiness and must not import
// internal/infrastructure. The types it renders live in internal/application/multirepo, which
// it may.
func NewMultirepoReadiness() appmultirepo.ReadinessChecker {
	return inframultirepo.NewReadiness()
}
