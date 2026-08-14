package multirepo

import "context"

// Readiness is what a group needs to know before releasing: whether each member could be
// released right now, and what stands in the way when it could not.
//
// `relicta group release` refused on the first member with "no release executor is
// configured", which told the operator that something was unimplemented and nothing about
// their group. These types answer what they were asking — what is blocking this release, and
// in which repository.
//
// Declared here rather than in the adapter because the CLI reports them and must not import
// internal/infrastructure; the hexagonal fitness function in internal/architecture enforces
// that, and it caught this arrangement's first version.

// Member is the part of a group's configuration a readiness check needs.
type Member struct {
	Name string
	Path string
}

// MemberState is what one member's stored run says about its readiness.
type MemberState struct {
	// Name is the member's configured name.
	Name string `json:"name"`

	// Path is where its checkout lives.
	Path string `json:"path"`

	// State is the stored run's state, or empty when there is no run.
	State string `json:"state,omitempty"`

	// Version is the version the run would release.
	Version string `json:"version,omitempty"`

	// Ready reports whether a release could proceed for this member.
	Ready bool `json:"ready"`

	// Blocker says what stands in the way, in the operator's terms, and is empty when
	// Ready is true.
	Blocker string `json:"blocker,omitempty"`
}

// ReadinessChecker reports each member's readiness, in the order given.
type ReadinessChecker interface {
	Check(ctx context.Context, members []Member) []MemberState
}

// AllReady reports whether every member could be released, and returns those that could not.
func AllReady(states []MemberState) (bool, []MemberState) {
	blocked := make([]MemberState, 0, len(states))
	for _, s := range states {
		if !s.Ready {
			blocked = append(blocked, s)
		}
	}
	return len(blocked) == 0, blocked
}
