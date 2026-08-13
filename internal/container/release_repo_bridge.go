package container

import (
	"context"
	"fmt"

	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// releaseRepoBridge exposes the release services' repository through the
// domainrelease.Repository interface that the CLI commands expect.
//
// There were two independent file-based implementations of the same aggregate.
// The release services (plan, bump, notes, approve, publish) used
// adapters.FileReleaseRunRepository; app.ReleaseRepository() returned
// persistence.FileReleaseRepository, whose DTO has a different shape. A run
// written by one and read by the other came back missing its changeset, HEAD SHA
// and commits — which broke governance three separate ways: `relicta evaluate`
// failed on every release, `relicta approve` skipped its gate because evaluation
// errored, and relicta_evaluate returned "internal error" over MCP. Each was
// fixed by pointing that one caller at the services repository, which left three
// redirects and six callers still on the old one.
//
// This is the consolidation those redirects were standing in for. Rather than
// migrate six call sites and change their interface, the bridge implements the
// interface they already use on top of the repository that has the data, so
// `cancel`, `clean`, `rollback`, `bump` and `approve` all read correct runs
// without touching their code.
//
// Only the five methods those callers use are meaningfully implemented.
// FindBySpecification and Publish are part of the interface and called from
// nowhere; they are honest errors rather than plausible-looking no-ops, so a
// future caller finds out immediately instead of getting silently empty results.
type releaseRepoBridge struct {
	inner ports.ReleaseRunRepository

	// repoRoot supplies the argument the ports interface takes and this one does
	// not. FindLatest and List receive a path from their caller; FindByID,
	// FindActive and FindByState do not, and the underlying repository needs one.
	repoRoot string
}

// newReleaseRepoBridge builds the bridge over a fresh services-format repository.
//
// It constructs its own rather than borrowing App.releaseServices, because those
// are initialized lazily by commands that need them and this must work before
// then — several commands call ReleaseRepository() without ever calling
// InitReleaseServices. Both point at the same files, so there is one store either
// way.
// The publisher is the same composed chain the release services receive, so a run saved
// through this bridge emits its events exactly as one saved by plan or publish does.
// Without it, the commands reaching the aggregate through here — cancel, clean, rollback,
// bump, approve — were the half of the workflow that recorded nothing.
func newReleaseRepoBridge(repoRoot string, publisher ports.EventPublisher) *releaseRepoBridge {
	var inner ports.ReleaseRunRepository = adapters.NewFileReleaseRunRepository()
	if publisher != nil {
		inner = adapters.NewEventPublishingRepository(adapters.EventPublishingConfig{
			Repository: inner,
			Publisher:  publisher,
		})
	}
	return &releaseRepoBridge{
		inner:    inner,
		repoRoot: repoRoot,
	}
}

func (b *releaseRepoBridge) Save(ctx context.Context, run *domain.ReleaseRun) error {
	return b.inner.Save(ctx, run)
}

// FindByID resolves through LoadBatch, not Load.
//
// ports.Load finds a run by scanning repo roots it has already seen in this
// process, and a fresh CLI invocation has seen none — it returns "release run not
// found" for a file that is right there. LoadBatch takes the root explicitly.
func (b *releaseRepoBridge) FindByID(ctx context.Context, id domain.RunID) (*domain.ReleaseRun, error) {
	runs, err := b.inner.LoadBatch(ctx, b.repoRoot, []domain.RunID{id})
	if err != nil {
		return nil, err
	}
	run := runs[id]
	if run == nil {
		return nil, fmt.Errorf("release run %s not found", id)
	}
	return run, nil
}

func (b *releaseRepoBridge) FindLatest(ctx context.Context, repoPath string) (*domain.ReleaseRun, error) {
	return b.inner.LoadLatest(ctx, b.rootOr(repoPath))
}

func (b *releaseRepoBridge) FindByState(ctx context.Context, state domain.RunState) ([]*domain.ReleaseRun, error) {
	return b.inner.FindByState(ctx, b.repoRoot, state)
}

func (b *releaseRepoBridge) FindActive(ctx context.Context) ([]*domain.ReleaseRun, error) {
	return b.inner.FindActive(ctx, b.repoRoot)
}

// FindBySpecification is unimplemented deliberately. Nothing calls it, and a
// filter-everything-in-memory stand-in would be a silent performance trap on a
// repository with a long release history.
func (b *releaseRepoBridge) FindBySpecification(_ context.Context, _ domainrelease.Specification) ([]*domain.ReleaseRun, error) {
	return nil, fmt.Errorf("FindBySpecification is not implemented by the release store bridge")
}

func (b *releaseRepoBridge) List(ctx context.Context, repoPath string) ([]domain.RunID, error) {
	return b.inner.List(ctx, b.rootOr(repoPath))
}

func (b *releaseRepoBridge) Delete(ctx context.Context, id domain.RunID) error {
	return b.inner.Delete(ctx, id)
}

// Publish is unimplemented deliberately: event publishing belongs to the event
// store, and nothing routes domain events through this interface.
func (b *releaseRepoBridge) Publish(_ context.Context, _ ...domainrelease.DomainEvent) error {
	return fmt.Errorf("Publish is not implemented by the release store bridge")
}

// rootOr prefers the caller's path and falls back to the resolved root.
//
// Callers pass a repository path they resolved themselves, which is the right
// value; the fallback covers a caller that passes nothing rather than silently
// reading the wrong directory — the bug that made `relicta cancel` look in the
// process working directory.
func (b *releaseRepoBridge) rootOr(repoPath string) string {
	if repoPath != "" {
		return repoPath
	}
	return b.repoRoot
}
