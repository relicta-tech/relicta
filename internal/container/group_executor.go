package container

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v4/internal/config"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	inframultirepo "github.com/relicta-tech/relicta/v4/internal/infrastructure/multirepo"
)

// groupExecutor publishes a member repository's already-approved release.
//
// It lives in the container because it builds one: a member's release needs that
// repository's own configuration, its own git service and its own release services, and the
// container is what assembles those. It could not live in infrastructure without importing
// the composition root.
//
// THE RULE IT FOLLOWS: it publishes what a human already approved in that repository, and
// approves nothing itself. A group release that could approve on behalf of a member would let
// somebody bypass that member's policy by adding it to a group — the release would run under
// an approval nobody gave. Members that are not approved are reported by the readiness check
// before anything runs; this refuses again at execution time rather than trusting that, since
// the two are separated by however long the earlier members took to publish.
//
// THE RISK IT AVOIDS: every path in a container derives from its git service, which took no
// path and therefore opened the process working directory. Pointing release services at a
// member's root while the git adapter still pointed at the invoking repository would have
// published the invoking repository's tags — silently, and unrecoverably. NewForRepo scopes
// the whole container instead.
type groupExecutor struct {
	planner *inframultirepo.Planner
	logger  *slog.Logger
}

// NewGroupExecutor returns an executor that plans any member and publishes an approved one.
func NewGroupExecutor(tagPrefix string, logger *slog.Logger) appmultirepo.ReleaseExecutor {
	if logger == nil {
		logger = slog.Default()
	}
	return &groupExecutor{
		planner: inframultirepo.NewPlanner(tagPrefix),
		logger:  logger,
	}
}

// Plan reports what the member would release next, delegating to the planner.
func (e *groupExecutor) Plan(ctx context.Context, repoPath string) (*appmultirepo.RepoResult, error) {
	return e.planner.Plan(ctx, repoPath)
}

// Release publishes the member's approved run.
func (e *groupExecutor) Release(ctx context.Context, repoPath string) (*appmultirepo.RepoResult, error) {
	app, run, err := e.openApproved(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	// Closed on its own timeout rather than the caller's context, deliberately: if the
	// group release was interrupted, ctx is already canceled and passing it here would
	// abandon the member's container mid-shutdown — leaving a plugin process running and
	// the release's own events unflushed. Shutting down is exactly the work that must
	// still happen when the caller has given up.
	//nolint:contextcheck // see above: shutdown must outlive a canceled caller
	defer func() {
		if closeErr := app.CloseWithTimeout(10 * time.Second); closeErr != nil {
			e.logger.Warn("closing the member container", "repo", repoPath, "error", closeErr)
		}
	}()

	services := app.ReleaseServices()
	if services == nil || services.PublishRelease == nil {
		return nil, fmt.Errorf("release services are not available for %s", repoPath)
	}

	output, err := services.PublishRelease.Execute(ctx, releaseapp.PublishReleaseInput{
		RepoRoot: repoPath,
		RunID:    run.ID(),
		Actor: ports.ActorInfo{
			// Named for what it is. The audit trail should say a group release published
			// this, not that "cli" did — the run's own approval already records who
			// authorized it.
			Type: "group",
			ID:   "relicta group release",
		},
		// Not forced. `relicta publish` forces because the operator has just been shown
		// the plan and confirmed it; here nobody is watching this particular member, so a
		// repository whose HEAD moved since approval must stop rather than publish
		// something that was never reviewed.
		Force:  false,
		DryRun: false,
	})
	if err != nil {
		return nil, fmt.Errorf("publishing %s: %w", repoPath, err)
	}

	published := time.Now()
	return &appmultirepo.RepoResult{
		State:       appmultirepo.StateReleased,
		NextVersion: output.VersionNext,
		ReleasedAt:  &published,
	}, nil
}

// openApproved builds a container scoped to repoPath and returns its run, refusing unless
// that run is approved.
func (e *groupExecutor) openApproved(ctx context.Context, repoPath string) (*App, *domain.ReleaseRun, error) {
	// The member's own configuration. A group member is a repository in its own right: its
	// policies, plugins and version settings are its own, and running it under the calling
	// repository's configuration would apply governance it never agreed to. Defaults when
	// it has no config file, which is what relicta does for any unconfigured repository.
	cfg, err := config.LoadFromDirectory(repoPath)
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}

	app, err := NewForRepo(cfg, repoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("preparing %s: %w", repoPath, err)
	}
	if err := app.Initialize(ctx); err != nil {
		return nil, nil, fmt.Errorf("initializing %s: %w", repoPath, err)
	}
	if err := app.InitReleaseServices(ctx, repoPath); err != nil {
		return nil, nil, fmt.Errorf("preparing release services for %s: %w", repoPath, err)
	}

	services := app.ReleaseServices()
	if services == nil || services.Repository == nil {
		return nil, nil, fmt.Errorf("release services are not available for %s", repoPath)
	}

	run, err := services.Repository.LoadLatest(ctx, repoPath)
	if err != nil || run == nil {
		return nil, nil, fmt.Errorf("no planned release in %s: run 'relicta plan' there first", repoPath)
	}

	// Checked again here, not only by the readiness report. The two are separated by
	// however long the earlier members took to publish, and this is the check that
	// actually guards the publish.
	if run.State() != domain.StateApproved && run.State() != domain.StatePublishing {
		return nil, nil, fmt.Errorf(
			"%s is in state %q and not approved: a group release publishes what was already "+
				"approved and approves nothing itself — run 'relicta approve' there",
			repoPath, run.State())
	}

	return app, run, nil
}
