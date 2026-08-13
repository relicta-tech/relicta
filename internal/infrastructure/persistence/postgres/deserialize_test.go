package postgres

// Unit tests for deserializeEvent — exercises error branches that require
// crafting malformed payloads. testcontainer_test.go covers happy paths
// for every event type via real Postgres round-trips; this file fills the
// negative-space gap that needs no DB.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeserializeEvent_UnknownTypeReturnsError(t *testing.T) {
	_, err := deserializeEvent("run.alien", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown event type")
	}
	if !strings.Contains(err.Error(), "run.alien") {
		t.Errorf("error should reference offending event name; got %v", err)
	}
}

func TestDeserializeEvent_MalformedJSONPropagatesError(t *testing.T) {
	// One representative failure per event-type family. Unmarshal failure
	// branches are structurally identical, so covering a few proves the rule.
	cases := []string{
		"release.created",
		"release.state_transitioned",
		"release.planned",
		"release.versioned",
		"release.notes_generated",
		"release.notes_updated",
		"release.approved",
		"release.publishing_started",
		"release.published",
		"release.failed",
		"release.canceled",
		"release.retried",
		"release.step_completed",
		"release.plugin_executed",
		"release.tag_push_mode_detected",
	}
	garbage := json.RawMessage(`{"runId": 12345}`) // wrong type for RunID

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := deserializeEvent(name, garbage)
			if err == nil {
				t.Errorf("expected unmarshal error for %s with malformed payload", name)
			}
		})
	}
}

func TestDeserializeEvent_EmptyPayloadDecodesToZeroValues(t *testing.T) {
	// Empty `{}` is valid JSON — Unmarshal succeeds with zero-valued struct.
	// Verifies the happy-path branch; error path covered above.
	evt, err := deserializeEvent("release.created", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("empty payload should decode without error; got %v", err)
	}
	if evt == nil {
		t.Error("expected non-nil event for valid empty JSON")
	}
}
