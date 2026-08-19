package domain

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Every state transition on this aggregate raises an event except one: Plan raised nothing, so
// RunPlannedEvent was declared, deserialized, and constructed by no code in the tree.
//
// The cost was not confined to dead weight. `release.planned` is a documented webhook event
// name, so a subscriber configured for it received nothing and could not tell that from a
// repository nobody was releasing; the outcome tracker's handlePlanned was unreachable; and Hub
// synthesizes a release.planned from stored records because the real one never arrives — it is
// the branch carrying the risk score and commit count that a governance row is built from.
//
// Verified end to end after the fix: a webhook subscribed to release.planned received
// {"event":"release.planned",...,"data":{"bump_kind":"minor","commit_count":2,...}}.

func TestPlanningARunAnnouncesIt(t *testing.T) {
	run := newRunForPlanTest(t)

	if err := run.Plan("alice"); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var planned *RunPlannedEvent
	for _, e := range run.DomainEvents() {
		if p, ok := e.(*RunPlannedEvent); ok {
			planned = p
		}
	}
	if planned == nil {
		t.Fatal("planning a release raised no event, so a webhook subscribed to " +
			"release.planned receives nothing and cannot tell that from an idle repository")
	}
	if planned.Actor != "alice" {
		t.Errorf("Actor = %q, want the actor who planned it", planned.Actor)
	}
}

// The payload is the point: Hub builds a governance row from this event, and it is the only one
// carrying the risk score and commit count.
func TestThePlannedEventCarriesWhatAGovernanceRowNeeds(t *testing.T) {
	run := newRunForPlanTest(t)
	if err := run.SetVersionProposal(
		version.MustParse("1.0.0"), version.MustParse("1.1.0"), BumpMinor, 0.9,
	); err != nil {
		t.Fatalf("SetVersionProposal: %v", err)
	}
	if err := run.Plan("alice"); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var planned *RunPlannedEvent
	for _, e := range run.DomainEvents() {
		if p, ok := e.(*RunPlannedEvent); ok {
			planned = p
		}
	}
	if planned == nil {
		t.Fatal("no RunPlannedEvent")
	}

	if planned.VersionNext.String() != "1.1.0" {
		t.Errorf("VersionNext = %q, want the proposed version: the plan use case sets the "+
			"proposal before calling Plan, so an empty one means the event is raised too early",
			planned.VersionNext.String())
	}
	if planned.BumpKind != BumpMinor {
		t.Errorf("BumpKind = %q, want minor", planned.BumpKind)
	}
	if planned.CommitCount == 0 {
		t.Error("CommitCount is zero, so the row Hub builds from this event reports a release " +
			"of nothing")
	}
}

func newRunForPlanTest(t *testing.T) *ReleaseRun {
	t.Helper()

	return NewReleaseRun("repo", "/tmp/repo", "main", CommitSHA("abc123"),
		[]CommitSHA{CommitSHA("abc123"), CommitSHA("def456")}, "cfg", "plugins")
}
