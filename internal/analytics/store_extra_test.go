package analytics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFileStore_CannotCreateDir verifies error when directory creation fails.
func TestNewFileStore_CannotCreateDir(t *testing.T) {
	// Try to create a store where the parent is a file, not a directory.
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	// Create a file at the path to prevent directory creation.
	err := os.WriteFile(tmpFile, []byte("block"), 0o644)
	require.NoError(t, err)

	_, err = NewFileStore(filepath.Join(tmpFile, "analytics"))
	assert.Error(t, err, "should fail when parent path is a file")
}

// TestMatchesFilter_AllFilters tests all filter combinations.
func TestMatchesFilter_AllFilters(t *testing.T) {
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	event := Event{
		ID:        "e1",
		Timestamp: now,
		Type:      EventRiskEvaluation,
		ReleaseID: "rel-1",
	}

	// Filter by From (event is after "from").
	from := now.Add(-1 * time.Hour)
	assert.True(t, matchesFilter(event, QueryFilter{From: &from}), "should match when after from")

	// Filter by From (event is before "from").
	fromFuture := now.Add(1 * time.Hour)
	assert.False(t, matchesFilter(event, QueryFilter{From: &fromFuture}), "should not match when before from")

	// Filter by To (event is before "to").
	to := now.Add(1 * time.Hour)
	assert.True(t, matchesFilter(event, QueryFilter{To: &to}), "should match when before to")

	// Filter by To (event is after "to").
	toPast := now.Add(-1 * time.Hour)
	assert.False(t, matchesFilter(event, QueryFilter{To: &toPast}), "should not match when after to")

	// Filter by EventType - matching.
	et := EventRiskEvaluation
	assert.True(t, matchesFilter(event, QueryFilter{EventType: &et}), "should match event type")

	// Filter by EventType - not matching.
	etOther := EventBumpType
	assert.False(t, matchesFilter(event, QueryFilter{EventType: &etOther}), "should not match different event type")

	// Filter by ReleaseID - matching.
	assert.True(t, matchesFilter(event, QueryFilter{ReleaseID: "rel-1"}), "should match release ID")

	// Filter by ReleaseID - not matching.
	assert.False(t, matchesFilter(event, QueryFilter{ReleaseID: "rel-99"}), "should not match different release ID")

	// No filters - always matches.
	assert.True(t, matchesFilter(event, QueryFilter{}), "empty filter should match everything")
}

// TestFileStore_Query_SkipsNonJSONL verifies non-.jsonl files are skipped.
func TestFileStore_Query_SkipsNonJSONL(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	require.NoError(t, err)
	ctx := context.Background()

	// Write a non-.jsonl file.
	err = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("some notes"), 0o600)
	require.NoError(t, err)

	// Write an invalid date file.
	err = os.WriteFile(filepath.Join(dir, "not-a-date.jsonl"), []byte("{}"), 0o600)
	require.NoError(t, err)

	// Valid event file.
	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	payload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.5})
	_ = store.Append(ctx, Event{ID: "e1", Timestamp: now, Type: EventRiskEvaluation, Payload: payload})

	events, err := store.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	// Only the valid event should be returned.
	assert.Len(t, events, 1)
}

// TestFileStore_Query_DateFiltering_BoundaryConditions tests date boundary behavior.
func TestFileStore_Query_DateFiltering_BoundaryConditions(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	// Event at noon on March 15.
	eventDay := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	payload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.5})
	_ = store.Append(ctx, Event{
		ID: "e1", Timestamp: eventDay, Type: EventRiskEvaluation, Payload: payload,
	})

	// From = March 16 (next day) — file date March 15 < March 16, so should be skipped.
	from := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	events, err := store.Query(ctx, QueryFilter{From: &from})
	require.NoError(t, err)
	assert.Len(t, events, 0, "events before from date should be excluded")

	// To = March 14 (previous day) — file date March 15 > March 14, so should be skipped.
	to := time.Date(2026, 3, 14, 23, 59, 59, 0, time.UTC)
	events, err = store.Query(ctx, QueryFilter{To: &to})
	require.NoError(t, err)
	assert.Len(t, events, 0, "events after to date should be excluded")
}

// TestDayFileName verifies the date format.
func TestDayFileName_Format(t *testing.T) {
	ts := time.Date(2026, 1, 5, 15, 30, 0, 0, time.UTC)
	got := dayFileName(ts)
	assert.Equal(t, "2026-01-05.jsonl", got)
}

// TestFileStore_Append_CorruptFile verifies query skips corrupt day files.
func TestFileStore_Append_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	require.NoError(t, err)
	ctx := context.Background()

	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	// Write a corrupt file directly.
	corruptData := []byte("this is not valid json\n{incomplete json\n")
	corruptFile := filepath.Join(dir, dayFileName(now))
	err = os.WriteFile(corruptFile, corruptData, 0o600)
	require.NoError(t, err)

	// Query should still succeed, skipping corrupt lines.
	events, err := store.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	// Should return no events since all lines are corrupt.
	assert.Len(t, events, 0)
}
