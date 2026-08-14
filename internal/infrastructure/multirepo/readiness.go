package multirepo

import (
	"context"
	"fmt"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// Readiness reports whether each member of a group could be released right now.
//
// `relicta group release` refused on the first member with "no release executor is
// configured", which told the operator that something was unimplemented and nothing about
// their group. This answers the question they were actually asking: what is blocking this
// release, and in which repository.
//
// It reads each member's stored run from disk and nothing else. No container, no git service,
// no configuration — deliberately, because every one of those resolves against the process
// working directory somewhere, and a component that silently answered for the invoking
// repository instead of the member would be worse than the refusal it replaced.
//
// It also never approves. Whether a group release may approve on behalf of a member whose
// policy requires a human is undecided (see docs/backlog.md), and reporting that a member
// needs approval is the honest thing to do while it stays that way.
type Readiness struct {
	repo *adapters.FileReleaseRunRepository
}

// NewReadiness returns a readiness reporter over the release runs stored in each repository.
func NewReadiness() *Readiness {
	return &Readiness{repo: adapters.NewFileReleaseRunRepository()}
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

	run, err := r.repo.LoadLatest(ctx, m.Path)
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
