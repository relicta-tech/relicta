package audit_test

// event_recorder_test.go pins the list of release transitions that become evidence.
//
// The list is the decision this file exists to protect. An event added to the domain and
// not recorded here is a governance transition with no evidence, and an event recorded
// twice — or recorded from a type nothing raises — is a chain that tells a different story
// from `relicta audit`. Both are silent, so both need a test that names them.

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

func mustVersion(t *testing.T, s string) version.SemanticVersion {
	t.Helper()
	v, err := version.Parse(s)
	if err != nil {
		t.Fatalf("parsing %s: %v", s, err)
	}
	return v
}

// releaseEvents is one release's transitions in the order the domain raises them.
func releaseEvents(t *testing.T, at time.Time) []release.DomainEvent {
	t.Helper()
	return []release.DomainEvent{
		&release.RunCreatedEvent{RunID: "run-1", RepoID: testRepo, HeadSHA: "abc123", At: at},
		&release.RunVersionedEvent{
			RunID: "run-1", VersionNext: mustVersion(t, "1.2.0"),
			BumpKind: release.BumpMinor, TagName: "v1.2.0", Actor: "cli", At: at,
		},
		&release.RunApprovedEvent{
			RunID: "run-1", PlanHash: "planhash", ApprovedBy: "human:alice", At: at,
		},
		&release.RunPublishedEvent{RunID: "run-1", Version: mustVersion(t, "1.2.0"), At: at},
	}
}

func TestTheReleaseLifecycleBecomesAVerifiableChain(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	recorder := audit.NewEventRecorder(store, testRepo, nil)

	if err := recorder.Publish(ctx, releaseEvents(t, time.Now())...); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	chain, err := audit.LoadChain(ctx, store, testRepo)
	if err != nil {
		t.Fatalf("the chain recorded from one release does not verify: %v", err)
	}

	want := []audit.EventType{
		audit.EventProposalReceived,
		audit.EventProposalReceived,
		audit.EventApprovalGranted,
		audit.EventExecutionCompleted,
	}
	got := chain.List()
	if len(got) != len(want) {
		t.Fatalf("one release produced %d chain entries, want %d: %v",
			len(got), len(want), entryTypes(got))
	}
	for i, eventType := range want {
		if got[i].EventType != eventType {
			t.Errorf("entry %d is %s, want %s: the chain tells a different story from "+
				"the release it records", i, got[i].EventType, eventType)
		}
	}
}

// The approval is the entry an auditor is looking for, and it is worthless without the
// actor. Recording it anonymously would leave the chain unable to say who authorized a
// release.
func TestTheApprovalEntryNamesItsActor(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	recorder := audit.NewEventRecorder(store, testRepo, nil)

	if err := recorder.Publish(ctx, &release.RunApprovedEvent{
		RunID: "run-1", ApprovedBy: "human:alice", AutoApproved: false, At: time.Now(),
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	chain, err := audit.LoadChain(ctx, store, testRepo)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	entry := chain.List()[0]
	if entry.ActorID != "human:alice" {
		t.Errorf("the approval entry names actor %q, want human:alice: the chain records "+
			"that a release was approved and not by whom", entry.ActorID)
	}
	if entry.Details["autoApproved"] != false {
		t.Errorf("the approval entry says autoApproved=%v, want false: a human approval "+
			"and a policy auto-approval are not the same evidence",
			entry.Details["autoApproved"])
	}
}

// A retried publish re-raises the transition it already recorded. Appending it twice would
// report one release as having been published twice.
func TestAReplayedTransitionIsNotRecordedTwice(t *testing.T) {
	ctx := context.Background()
	store := memory.NewInMemoryStore()
	recorder := audit.NewEventRecorder(store, testRepo, nil)

	published := &release.RunPublishedEvent{
		RunID: "run-1", Version: mustVersion(t, "1.2.0"), At: time.Now(),
	}
	if err := recorder.Publish(ctx, published); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := recorder.Publish(ctx, published); err != nil {
		t.Fatalf("republishing the same transition: %v", err)
	}

	chain, err := audit.LoadChain(ctx, store, testRepo)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Len() != 1 {
		t.Errorf("one transition raised twice produced %d entries, want 1", chain.Len())
	}
}

// Recording is best effort, forwarding is not: an event that failed to become evidence
// must still reach the webhook subscribers and the outcome tracker behind this decorator.
func TestEveryEventIsForwardedEvenWhenItIsNotRecorded(t *testing.T) {
	forwarded := &countingPublisher{}
	recorder := audit.NewEventRecorder(memory.NewInMemoryStore(), testRepo, forwarded)

	events := append(releaseEvents(t, time.Now()),
		&release.RunNotesGeneratedEvent{RunID: "run-1", At: time.Now()})

	if err := recorder.Publish(context.Background(), events...); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if forwarded.count != len(events) {
		t.Errorf("%d of %d events reached the next publisher: the audit chain swallowed "+
			"events the webhooks and the outcome tracker need", forwarded.count, len(events))
	}
}

type countingPublisher struct{ count int }

func (p *countingPublisher) Publish(_ context.Context, events ...release.DomainEvent) error {
	p.count += len(events)
	return nil
}

func entryTypes(entries []*audit.Entry) []audit.EventType {
	types := make([]audit.EventType, 0, len(entries))
	for _, e := range entries {
		types = append(types, e.EventType)
	}
	return types
}
