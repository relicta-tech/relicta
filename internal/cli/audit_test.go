package cli

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// Governance activity lives in two records — the CGP protocol handshake and the release
// audit trail — and correctly so, since a proposal is not a release run. The cost fell on
// the reader, who had to run two commands and join them by eye. These tests cover the join.
//
// The version is the key: an ExecutionAuthorization names the version a proposal was
// authorized to release, and a release record names the version that was released.

func auditRelease(id, version string, outcome memory.ReleaseOutcome, at time.Time) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         id,
		Repository: "acme/widget",
		Version:    version,
		Actor:      cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:alice"},
		Outcome:    outcome,
		ReleasedAt: at,
	}
}

func TestAProposalIsJoinedToTheReleaseItAuthorized(t *testing.T) {
	now := time.Now().UTC()
	releases := []*memory.ReleaseRecord{auditRelease("run-1", "1.4.0", memory.OutcomeSuccess, now)}
	proposals := []auditProposal{{
		id: "prop_abc", actor: "agent:claude", state: "authorized",
		version: "1.4.0", decided: "approved", at: now.Add(-time.Hour),
	}}

	entries := joinGovernanceRecords(releases, proposals)

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1: the proposal and the release it authorized are one "+
			"line, which is the whole point of the join", len(entries))
	}
	if entries[0].Release == nil || entries[0].Propose == nil {
		t.Fatalf("entry has release=%v proposal=%v; both sides should be present",
			entries[0].Release != nil, entries[0].Propose != nil)
	}
	// Dated by the release, because "when did this ship" is what a reader scans for.
	if !entries[0].At.Equal(now) {
		t.Errorf("entry dated %s, want the release date %s", entries[0].At, now)
	}
}

// The bug this test exists for: `claimed` was keyed by version, so pairing one release
// suppressed every other release carrying the same version — and a repository routinely has
// several. A canceled 0.1.0 and the published 0.1.0 that followed it are two records of the
// same version, and the cancellation vanished from the timeline the moment a proposal claimed
// the release. Found by running the command, not by reading the loop.
func TestPairingOneReleaseDoesNotHideAnotherOfTheSameVersion(t *testing.T) {
	now := time.Now().UTC()
	releases := []*memory.ReleaseRecord{
		auditRelease("run-published", "0.1.0", memory.OutcomeSuccess, now),
		auditRelease("run-canceled", "0.1.0", memory.OutcomeCanceled, now.Add(-time.Minute)),
	}
	proposals := []auditProposal{{
		id: "prop_abc", actor: "agent:claude", state: "authorized", version: "0.1.0", at: now,
	}}

	entries := joinGovernanceRecords(releases, proposals)

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: the canceled run disappeared when the proposal "+
			"claimed the version it shares with the published one", len(entries))
	}

	var sawCanceled, sawPublished bool
	for _, e := range entries {
		if e.Release == nil {
			continue
		}
		switch e.Release.Outcome {
		case string(memory.OutcomeCanceled):
			sawCanceled = true
			if e.Propose != nil {
				t.Error("the authorization was paired with the canceled run; it authorized " +
					"the release that shipped, and reporting otherwise misstates what it led to")
			}
		case string(memory.OutcomeSuccess):
			sawPublished = true
			if e.Propose == nil {
				t.Error("the published release was not paired with the proposal that authorized it")
			}
		}
	}
	if !sawCanceled || !sawPublished {
		t.Errorf("timeline is missing a record: canceled=%v published=%v", sawCanceled, sawPublished)
	}
}

// A release nobody proposed is the ordinary case for a human-driven release, and a proposal
// that never shipped is governance activity too. Both must appear.
func TestBothHalvesAppearOnTheirOwn(t *testing.T) {
	now := time.Now().UTC()
	releases := []*memory.ReleaseRecord{auditRelease("run-1", "2.0.0", memory.OutcomeSuccess, now)}
	proposals := []auditProposal{{
		// Decided but never authorized, so it has no version to join on.
		id: "prop_unshipped", actor: "agent:claude", state: "decided",
		decided: "approval_required", at: now.Add(-2 * time.Hour),
	}}

	entries := joinGovernanceRecords(releases, proposals)

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (one release, one unshipped proposal)", len(entries))
	}
	for _, e := range entries {
		if e.Release != nil && e.Propose != nil {
			t.Error("an unauthorized proposal was joined to a release; it has no version to " +
				"join on, and pairing them would invent a link the records do not contain")
		}
	}
}

// Newest first, so the timeline reads the way every other listing in this tool does.
func TestTheTimelineIsNewestFirst(t *testing.T) {
	now := time.Now().UTC()
	releases := []*memory.ReleaseRecord{
		auditRelease("run-old", "1.0.0", memory.OutcomeSuccess, now.Add(-48*time.Hour)),
		auditRelease("run-new", "1.1.0", memory.OutcomeSuccess, now),
	}

	entries := joinGovernanceRecords(releases, nil)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Version != "1.1.0" {
		t.Errorf("first entry is %s, want the newest (1.1.0)", entries[0].Version)
	}
}

// A reader copying a version out of `git tag` gets it with the prefix.
func TestVersionFilterAcceptsATagName(t *testing.T) {
	entries := []auditEntry{{Version: "1.4.0"}, {Version: "1.5.0"}}

	for _, query := range []string{"1.4.0", "v1.4.0"} {
		got := filterByVersion(entries, query)
		if len(got) != 1 || got[0].Version != "1.4.0" {
			t.Errorf("filterByVersion(%q) returned %v, want the 1.4.0 entry", query, got)
		}
	}
}
