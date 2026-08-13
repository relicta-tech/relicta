package adapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// The wire vocabulary was renamed from "run.*" to the documented "release.*". Event stores
// hold whatever name the version that wrote them used, so this is the case that decides
// whether the rename was safe: a store file written before it must still load.
//
// Asserted at the store rather than only on CanonicalEventName, because the unit test of
// the mapper passing tells you nothing about whether the deserializer calls it — which is
// the shape of bug this codebase keeps producing.
func TestAnEventFileWrittenWithHistoricalNamesStillLoads(t *testing.T) {
	repoRoot := t.TempDir()
	runID := domain.RunID("run-legacy")

	eventsDir := filepath.Join(repoRoot, ".relicta", "events")
	if err := os.MkdirAll(eventsDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Exactly what an earlier version wrote: JSON lines whose event_name is "run.*".
	legacy := `{"id":"e1","run_id":"run-legacy","event_name":"run.created","occurred_at":"2026-08-01T10:00:00Z","stored_at":"2026-08-01T10:00:00Z","sequence_num":1,"payload":{"RunID":"run-legacy","RepoID":"acme/widget","HeadSHA":"abc123","At":"2026-08-01T10:00:00Z"}}
{"id":"e2","run_id":"run-legacy","event_name":"run.published","occurred_at":"2026-08-01T11:00:00Z","stored_at":"2026-08-01T11:00:00Z","sequence_num":2,"payload":{"RunID":"run-legacy","At":"2026-08-01T11:00:00Z"}}
`
	path := filepath.Join(eventsDir, string(runID)+".events.jsonl")
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	store := NewFileEventStore()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	events, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents on a store written with historical names: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("loaded %d events, want 2: the rename made previously stored history "+
			"unreadable, which is worse than the inconsistency it fixed", len(events))
	}

	if _, ok := events[0].(*domain.RunCreatedEvent); !ok {
		t.Errorf("first event is %T, want *domain.RunCreatedEvent", events[0])
	}
	if _, ok := events[1].(*domain.RunPublishedEvent); !ok {
		t.Errorf("second event is %T, want *domain.RunPublishedEvent", events[1])
	}

	// And it reports the current name, so a reader sees one vocabulary regardless of
	// which version wrote the entry.
	if got := events[0].EventName(); got != "release.created" {
		t.Errorf("EventName() = %q, want release.created", got)
	}
}
