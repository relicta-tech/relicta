package multirepo

import (
	"context"
	"fmt"
	"io"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
)

// Readiness reports whether each member of a group could be released right now.
//
// `relicta group release` refused on the first member with "no release executor is
// configured", which told the operator that something was unimplemented and nothing about
// their group. This answers the question they were actually asking: what is blocking this
// release, and in which repository.
//
// It reads each member's stored run and nothing else — no container and no git service,
// deliberately, because both resolve against the process working directory somewhere, and a
// component that silently answered for the invoking repository instead of the member would be
// worse than the refusal it replaced.
//
// It does read each member's *own* configuration, which is not the same concession. This used
// to construct a file repository unconditionally, and once persistence.backend began selecting
// (ADR-013) that turned into a confident wrong answer: a member whose run was approved and
// stored in SQLite was reported as "no release has been planned — run 'relicta plan'", which
// the operator had already done. Verified against the shipped binary before the fix.
//
// Per member, not once for the group, because the backend is a property of the repository being
// released. A team may keep one service on the shared postgres store and another on files, and
// the invoking repository's configuration says nothing about either. The working-directory
// hazard above does not apply: the path comes from the group declaration, and the config is
// loaded from that path rather than found by searching upward from the process.
//
// It also never approves. Whether a group release may approve on behalf of a member whose
// policy requires a human is undecided (see docs/backlog.md), and reporting that a member
// needs approval is the honest thing to do while it stays that way.
type Readiness struct {
	// storeFor opens the run store for one member. Injected so the tests in this package can
	// supply a repository directly, and so that resolving configuration stays one function
	// rather than something every method has to remember to do.
	storeFor func(ctx context.Context, repoRoot string) (ports.ReleaseRunRepository, io.Closer, error)
}

// NewReadiness returns a readiness reporter over the release runs stored in each repository.
func NewReadiness() *Readiness {
	return &Readiness{storeFor: openConfiguredStore}
}

// openConfiguredStore opens a member's run store using that member's own .relicta.yaml.
//
// A member with no configuration file is not an error: config loading returns defaults, and the
// default backend is the file adapter, which is what such a repository has always used.
func openConfiguredStore(
	ctx context.Context, repoRoot string,
) (ports.ReleaseRunRepository, io.Closer, error) {
	cfg, err := config.LoadFromDirectory(repoRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("reading configuration in %s: %w", repoRoot, err)
	}

	store, err := persistence.OpenReleaseRunStore(ctx, cfg.Persistence, repoRoot)
	if err != nil {
		return nil, nil, err
	}
	return store.Repository, store.Closer, nil
}

var _ appmultirepo.ReadinessChecker = (*Readiness)(nil)

// Check reports each member's readiness, in the order given.
func (r *Readiness) Check(ctx context.Context, members []appmultirepo.Member) []appmultirepo.MemberState {
	states := make([]appmultirepo.MemberState, 0, len(members))
	for _, m := range members {
		states = append(states, r.checkOne(ctx, m))
	}
	return states
}

func (r *Readiness) checkOne(ctx context.Context, m appmultirepo.Member) appmultirepo.MemberState {
	state := appmultirepo.MemberState{Name: m.Name, Path: m.Path}

	repo, closer, err := r.storeFor(ctx, m.Path)
	if err != nil {
		// A store that cannot be opened is reported as its own blocker rather than as "no
		// release has been planned". The two send the operator to different places, and
		// telling someone whose database is unreachable to run 'relicta plan' would send
		// them to the wrong one.
		state.Blocker = fmt.Sprintf("its release store could not be opened: %v", err)
		return state
	}
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}

	run, err := repo.LoadLatest(ctx, m.Path)
	if err != nil || run == nil {
		// No stored run is the ordinary starting point, not a failure: nobody has planned
		// a release in that repository yet.
		state.Blocker = "no release has been planned — run 'relicta plan' in " + m.Path
		return state
	}

	state.State = string(run.State())
	state.Version = run.Summary().VersionNext
	state.Ready, state.Blocker = blockerFor(run.State(), m.Path)
	return state
}

// blockerFor maps a run's state onto whether a group release could proceed for it, and what
// the operator has to do when it could not.
//
// Separated from the store read so every state can be covered by a table. Driving an
// aggregate into each one through its state machine is neither possible for all of them —
// draft cannot transition straight to failed — nor the point: what matters is that each state
// sends the operator to the right command, and that a state nobody anticipated does not read
// as ready.
func blockerFor(state domain.RunState, path string) (ready bool, blocker string) {
	switch state {
	case domain.StateApproved, domain.StatePublishing:
		// Publishing means a previous attempt got partway, and resuming is what 'publish'
		// does for a single repository, so it counts as ready here too.
		return true, ""
	case domain.StatePublished:
		return false, "already published — run 'relicta plan' for the next release in " + path
	case domain.StateCanceled:
		return false, "the planned release was canceled — run 'relicta reset' then 'relicta plan' in " + path
	case domain.StateFailed:
		return false, "the last publish failed — run 'relicta retry' or 'relicta reset' in " + path
	default:
		// Draft, planned, versioned, noted: the run exists and has not been approved. The
		// default is not-ready on purpose — a state added later must block a group release
		// until somebody decides what it means, rather than silently permitting one.
		return false, fmt.Sprintf("in state %q and not approved — run 'relicta approve' in %s",
			state, path)
	}
}
