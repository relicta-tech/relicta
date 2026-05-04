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
		"run.created",
		"run.state_transitioned",
		"run.planned",
		"run.versioned",
		"run.notes_generated",
		"run.notes_updated",
		"run.approved",
		"run.publishing_started",
		"run.published",
		"run.failed",
		"run.canceled",
		"run.retried",
		"run.step_completed",
		"run.plugin_executed",
		"run.tag_push_mode_detected",
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
	evt, err := deserializeEvent("run.created", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("empty payload should decode without error; got %v", err)
	}
	if evt == nil {
		t.Error("expected non-nil event for valid empty JSON")
	}
}
