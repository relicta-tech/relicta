package domain

import "testing"

// Renaming the wire vocabulary from "run.*" to the documented "release.*" is only safe if
// the event stores can still read what earlier versions wrote. Both deserializers — the
// file event store and the Postgres one — switch on the stored name, so without
// canonicalization every event already persisted would fail to reconstruct: an audit trail
// unable to read its own history, which is worse than the naming inconsistency the rename
// fixed.
func TestHistoricalEventNamesStillResolve(t *testing.T) {
	for stored, want := range map[string]string{
		// The historical spelling, as found in stores written before the rename. These
		// literals must stay "run.*" — that is the whole point of the test.
		"run.created":            "release.created",
		"run.published":          "release.published",
		"run.failed":             "release.failed",
		"run.canceled":           "release.canceled",
		"run.state_transitioned": "release.state_transitioned",

		// Already current: unchanged.
		"release.published": "release.published",
		"release.canceled":  "release.canceled",

		// Not ours: passed through so the deserializer's own default case reports it
		// rather than this function inventing a name for it.
		"other.thing": "other.thing",
		"":            "",
	} {
		if got := CanonicalEventName(stored); got != want {
			t.Errorf("CanonicalEventName(%q) = %q, want %q", stored, got, want)
		}
	}
}

// Every event's name must carry the documented prefix. A new event added with the old
// prefix would silently stop matching users' "release.*" webhook filters.
func TestEveryEventNameUsesTheReleasePrefix(t *testing.T) {
	events := []DomainEvent{
		&RunCreatedEvent{}, &StateTransitionedEvent{}, &RunApprovedEvent{},
		&StepCompletedEvent{}, &RunPublishedEvent{}, &RunFailedEvent{},
		&RunCanceledEvent{}, &RunVersionedEvent{}, &RunRetriedEvent{},
		&RunPlannedEvent{}, &RunNotesGeneratedEvent{}, &RunNotesUpdatedEvent{},
		&RunPublishingStartedEvent{}, &PluginExecutedEvent{}, &TagPushModeDetectedEvent{},
	}

	for _, event := range events {
		name := event.EventName()
		if len(name) < len("release.") || name[:len("release.")] != "release." {
			t.Errorf("%T.EventName() = %q, which does not start with \"release.\" and so "+
				"matches none of the documented webhook filters", event, name)
		}
	}
}
