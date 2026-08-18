package audit

// event_recorder.go turns release lifecycle transitions into audit chain entries.
//
// It is a decorator on the release event publisher, the same shape and the same position
// as the governance memory outcome tracker, and it is there for one reason: `relicta` is
// several processes. plan, approve and publish are separate invocations, so anything
// holding lifecycle state in memory correlates nothing. The event stream is what survives
// the process boundary, the run ID is what ties the invocations together, and both are
// already in front of the publisher chain.
//
// What gets recorded, and why that list:
//
//   - release.created    → proposal.received. The change put forward for governance: a
//     repository and the commit it is proposed from. Without it the chain begins at a
//     decision about something it never names.
//   - release.versioned  → proposal.received's other half. `relicta plan` creates the
//     run before a version exists and `relicta bump` chooses one, so the version a
//     release proposes is a second transition rather than a field on the first, and
//     what version was proposed is what the decision was about.
//   - release.approved   → approval.granted. The actor, and whether a human or the
//     policy granted it. This is the entry an auditor is actually looking for.
//   - release.published  → execution.completed. What was authorized, and that it
//     happened.
//   - release.failed     → execution.failed. An authorization with no outcome is the
//     gap that makes a chain unreadable; a release that was allowed to proceed and did
//     not finish has to say so.
//
// Not release.planned, which reads like the obvious source for the proposal and is
// raised by nothing: it is declared, deserialized and handled in three places, and no
// production code has ever constructed one. A recorder hung off it would have appended
// nothing and looked correct.
//
// The risk evaluation and the decision are not here. They are not lifecycle transitions
// and never reach this stream — the evaluator runs inside `relicta approve` and `relicta
// publish` before any event is raised, and a rejected release raises no event at all, so
// a rejection recorded from here would not exist. They are appended where they happen,
// in the governance service.
//
// release.canceled is deliberately absent. Cancellation is already a governance record —
// the outcome tracker writes it as an OutcomeCanceled release — and the CGP event
// vocabulary (Section 9) has no term for it. Inventing one here is how the chain and
// `relicta audit` come to tell different stories about the same repository, which is the
// specific failure this work exists to remove.

import (
	"context"
	"log/slog"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

// EventRecorder appends governance evidence for the release transitions it sees, then
// forwards every event to the next publisher.
type EventRecorder struct {
	recorder *Recorder
	next     release.EventPublisher
	logger   *slog.Logger
}

// NewEventRecorder wraps next, appending to repository's chain in store.
//
// next may be nil, in which case events are recorded and not forwarded.
func NewEventRecorder(store Store, repository string, next release.EventPublisher) *EventRecorder {
	return &EventRecorder{
		recorder: NewRecorder(store, repository),
		next:     next,
		logger:   slog.Default().With("component", "audit_chain"),
	}
}

// Publish records the governance-relevant events and forwards all of them.
//
// A failed append is logged and does not stop the release, matching the outcome tracker
// beside it. That is a real trade and worth naming: the alternative — failing the publish
// — turns a full disk into a blocked release, and relicta's own evidence is not worth
// more than the release it is evidence of. What makes it acceptable is that the loss is
// visible rather than silent: a missing entry breaks nothing in the chain (the entries
// around it still link), but the attestation's entry count stops matching the release
// history, and `relicta audit` prints the chain's length beside the releases it has.
func (r *EventRecorder) Publish(ctx context.Context, events ...release.DomainEvent) error {
	for _, event := range events {
		entry := r.entryFor(event)
		if entry == nil {
			continue
		}
		if err := r.recorder.RecordIgnoringDuplicate(ctx, entry); err != nil {
			r.logger.Warn("failed to append a governance event to the audit chain",
				"event", event.EventName(),
				"release_id", string(event.AggregateID()),
				"entry", entry.ID,
				"error", err)
		}
	}

	if r.next != nil {
		return r.next.Publish(ctx, events...)
	}
	return nil
}

// entryFor builds the chain entry a transition deserves, or nil for one that is not
// governance evidence.
func (r *EventRecorder) entryFor(event release.DomainEvent) *Entry {
	switch e := event.(type) {
	case *release.RunCreatedEvent:
		return NewEntry(entryID(e.RunID, "proposal"), EventProposalReceived).
			WithProposal(string(e.RunID)).
			WithTimestamp(e.At).
			WithDetail("headSha", string(e.HeadSHA)).
			Build()

	case *release.RunVersionedEvent:
		return NewEntry(entryID(e.RunID, "version"), EventProposalReceived).
			WithProposal(string(e.RunID)).
			WithActor(e.Actor, actorKind(e.Actor)).
			WithTimestamp(e.At).
			WithDetail("versionNext", e.VersionNext.String()).
			WithDetail("bumpKind", string(e.BumpKind)).
			WithDetail("tag", e.TagName).
			Build()

	case *release.RunApprovedEvent:
		return NewEntry(entryID(e.RunID, "approval"), EventApprovalGranted).
			WithProposal(string(e.RunID)).
			WithActor(e.ApprovedBy, actorKind(e.ApprovedBy)).
			WithTimestamp(e.At).
			WithDetail("autoApproved", e.AutoApproved).
			WithDetail("planHash", e.PlanHash).
			Build()

	case *release.RunPublishedEvent:
		return NewEntry(entryID(e.RunID, "published"), EventExecutionCompleted).
			WithProposal(string(e.RunID)).
			WithTimestamp(e.At).
			WithDetail("version", e.Version.String()).
			Build()

	case *release.RunFailedEvent:
		return NewEntry(entryID(e.RunID, "failed"), EventExecutionFailed).
			WithProposal(string(e.RunID)).
			WithTimestamp(e.At).
			WithDetail("version", e.Version).
			WithDetail("reason", e.Reason).
			Build()

	default:
		return nil
	}
}

// entryID derives an entry's identity from the run and the transition.
//
// Derived rather than random so that a step which runs twice — a retried publish, a run
// approved again after a reset — names the entry it already wrote and is refused as a
// duplicate, instead of recording one transition as two. It also means the same release
// produces the same entry IDs on every backend, which is what lets the differential
// harness compare them.
func entryID(runID release.RunID, transition string) string {
	return string(runID) + ":" + transition
}

// actorKind reads the kind out of a qualified actor ID, defaulting to human.
//
// Release events carry the actor as a single string, which is sometimes the qualified
// "human:alice" form cgp.QualifiedActorID produces and sometimes a bare git identity.
// Defaulting to human is the honest reading of a bare name: it is what `relicta approve`
// records for a person, and claiming an unattributed approval came from automation would
// overstate what is known.
func actorKind(actorID string) cgp.ActorKind {
	if actorID == "" {
		return ""
	}
	for _, kind := range cgp.AllActorKinds() {
		if strings.HasPrefix(actorID, kind.String()+":") {
			return kind
		}
	}
	return cgp.ActorKindHuman
}
