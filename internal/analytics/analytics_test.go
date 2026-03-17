package analytics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Event Type Tests
// =============================================================================

func TestEventType_Valid(t *testing.T) {
	tests := []struct {
		eventType EventType
		valid     bool
	}{
		{EventRiskEvaluation, true},
		{EventPolicyDecision, true},
		{EventApprovalOutcome, true},
		{EventReleaseDuration, true},
		{EventBumpType, true},
		{EventType("unknown"), false},
		{EventType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.eventType.Valid())
		})
	}
}

func TestMarshalPayload(t *testing.T) {
	payload := RiskEvaluationPayload{
		RiskScore: 0.75,
		RiskLevel: "high",
		Factors:   []string{"breaking change", "large diff"},
	}

	raw, err := MarshalPayload(payload)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	var decoded RiskEvaluationPayload
	err = json.Unmarshal(raw, &decoded)
	require.NoError(t, err)
	assert.InDelta(t, 0.75, decoded.RiskScore, 0.001)
	assert.Equal(t, "high", decoded.RiskLevel)
	assert.Equal(t, []string{"breaking change", "large diff"}, decoded.Factors)
}

// =============================================================================
// FileStore Tests
// =============================================================================

func TestFileStore_NewFileStore(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "analytics")

	store, err := NewFileStore(storePath)
	require.NoError(t, err)
	require.NotNil(t, store)

	// Directory should exist
	info, err := os.Stat(storePath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileStore_AppendAndQuery(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	payload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.5, RiskLevel: "medium"})

	event := Event{
		ID:        "evt-1",
		Timestamp: now,
		Type:      EventRiskEvaluation,
		ReleaseID: "rel-1",
		Payload:   payload,
	}

	err := store.Append(ctx, event)
	require.NoError(t, err)

	// Query all events
	events, err := store.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "evt-1", events[0].ID)
	assert.Equal(t, EventRiskEvaluation, events[0].Type)
	assert.Equal(t, "rel-1", events[0].ReleaseID)
}

func TestFileStore_QueryByDateRange(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	day1 := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	payload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.5})

	for i, ts := range []time.Time{day1, day2, day3} {
		_ = store.Append(ctx, Event{
			ID:        "evt-" + string(rune('0'+i)),
			Timestamp: ts,
			Type:      EventRiskEvaluation,
			Payload:   payload,
		})
	}

	// Query with from/to
	from := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 14, 23, 59, 59, 0, time.UTC)
	events, err := store.Query(ctx, QueryFilter{From: &from, To: &to})
	require.NoError(t, err)
	assert.Len(t, events, 1) // only day2
}

func TestFileStore_QueryByEventType(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	riskPayload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.5})
	bumpPayload, _ := MarshalPayload(BumpTypePayload{BumpType: "minor"})

	_ = store.Append(ctx, Event{ID: "e1", Timestamp: now, Type: EventRiskEvaluation, Payload: riskPayload})
	_ = store.Append(ctx, Event{ID: "e2", Timestamp: now, Type: EventBumpType, Payload: bumpPayload})

	eventType := EventRiskEvaluation
	events, err := store.Query(ctx, QueryFilter{EventType: &eventType})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "e1", events[0].ID)
}

func TestFileStore_QueryByReleaseID(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	now := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	payload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.3})

	_ = store.Append(ctx, Event{ID: "e1", Timestamp: now, Type: EventRiskEvaluation, ReleaseID: "rel-1", Payload: payload})
	_ = store.Append(ctx, Event{ID: "e2", Timestamp: now, Type: EventRiskEvaluation, ReleaseID: "rel-2", Payload: payload})

	events, err := store.Query(ctx, QueryFilter{ReleaseID: "rel-1"})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "rel-1", events[0].ReleaseID)
}

func TestFileStore_QueryEmptyStore(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	events, err := store.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestFileStore_QuerySortedByTimestamp(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	ctx := context.Background()

	payload, _ := MarshalPayload(RiskEvaluationPayload{RiskScore: 0.5})

	t3 := time.Date(2026, 3, 15, 15, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	// Insert out of order
	_ = store.Append(ctx, Event{ID: "e3", Timestamp: t3, Type: EventRiskEvaluation, Payload: payload})
	_ = store.Append(ctx, Event{ID: "e1", Timestamp: t1, Type: EventRiskEvaluation, Payload: payload})
	_ = store.Append(ctx, Event{ID: "e2", Timestamp: t2, Type: EventRiskEvaluation, Payload: payload})

	events, err := store.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "e1", events[0].ID)
	assert.Equal(t, "e2", events[1].ID)
	assert.Equal(t, "e3", events[2].ID)
}

// =============================================================================
// Service Tests
// =============================================================================

func TestService_Capture(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	err := svc.Capture(ctx, EventRiskEvaluation, "rel-1", RiskEvaluationPayload{
		RiskScore: 0.6,
		RiskLevel: "medium",
		Factors:   []string{"many files changed"},
	})
	require.NoError(t, err)

	events, err := svc.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, EventRiskEvaluation, events[0].Type)
	assert.Equal(t, "rel-1", events[0].ReleaseID)
	assert.NotEmpty(t, events[0].ID)
}

func TestService_CaptureInvalidType(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	err := svc.Capture(ctx, EventType("bogus"), "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event type")
}

func TestService_CaptureAllEventTypes(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	tests := []struct {
		eventType EventType
		payload   any
	}{
		{EventRiskEvaluation, RiskEvaluationPayload{RiskScore: 0.3, RiskLevel: "low"}},
		{EventPolicyDecision, PolicyDecisionPayload{Decision: "approve", RiskScore: 0.3}},
		{EventApprovalOutcome, ApprovalOutcomePayload{Outcome: "approved", ActorID: "user-1", ActorKind: "human"}},
		{EventReleaseDuration, ReleaseDurationPayload{DurationMs: 5000, Success: true, Version: "1.2.0"}},
		{EventBumpType, BumpTypePayload{BumpType: "minor", CurrentVersion: "1.1.0", NextVersion: "1.2.0"}},
	}

	for _, tt := range tests {
		err := svc.Capture(ctx, tt.eventType, "rel-1", tt.payload)
		require.NoError(t, err, "failed to capture %s", tt.eventType)
	}

	events, err := svc.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	assert.Len(t, events, 5)
}

func TestService_WithClock(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := NewService(store).WithClock(func() time.Time { return fixedTime })
	ctx := context.Background()

	err := svc.Capture(ctx, EventBumpType, "", BumpTypePayload{BumpType: "patch"})
	require.NoError(t, err)

	events, err := svc.Query(ctx, QueryFilter{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, fixedTime, events[0].Timestamp)
}

// =============================================================================
// Aggregation Tests
// =============================================================================

func TestTimeBucketKey_Day(t *testing.T) {
	ts := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-03-15", TimeBucketKey(ts, GranularityDay))
}

func TestTimeBucketKey_Week(t *testing.T) {
	// 2026-03-15 is a Sunday, ISO week 11. Monday of that week is 2026-03-09.
	ts := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	key := TimeBucketKey(ts, GranularityWeek)
	assert.Equal(t, "2026-03-09", key)

	// Same week, different day (Wednesday)
	ts2 := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, key, TimeBucketKey(ts2, GranularityWeek))
}

func TestTimeBucketKey_Month(t *testing.T) {
	ts := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-03", TimeBucketKey(ts, GranularityMonth))
}

func TestParseGranularity(t *testing.T) {
	assert.Equal(t, GranularityDay, ParseGranularity("day"))
	assert.Equal(t, GranularityWeek, ParseGranularity("week"))
	assert.Equal(t, GranularityMonth, ParseGranularity("month"))
	assert.Equal(t, GranularityDay, ParseGranularity(""))
	assert.Equal(t, GranularityDay, ParseGranularity("invalid"))
}

func TestAggregateRiskTrends(t *testing.T) {
	events := buildRiskEvents(t, []riskInput{
		{time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC), 0.2},
		{time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC), 0.6},
		{time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC), 0.8},
	})

	points := AggregateRiskTrends(events, GranularityDay)
	require.Len(t, points, 2)

	// Day 1: avg 0.4, max 0.6, min 0.2
	assert.Equal(t, "2026-03-10", points[0].Bucket)
	assert.InDelta(t, 0.4, points[0].AvgRiskScore, 0.001)
	assert.InDelta(t, 0.6, points[0].MaxRiskScore, 0.001)
	assert.InDelta(t, 0.2, points[0].MinRiskScore, 0.001)
	assert.Equal(t, 2, points[0].Count)

	// Day 2: avg 0.8
	assert.Equal(t, "2026-03-11", points[1].Bucket)
	assert.InDelta(t, 0.8, points[1].AvgRiskScore, 0.001)
	assert.Equal(t, 1, points[1].Count)
}

func TestAggregateRiskTrends_WeeklyGranularity(t *testing.T) {
	events := buildRiskEvents(t, []riskInput{
		{time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC), 0.3},  // Week 11
		{time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC), 0.5}, // Week 11
		{time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC), 0.7}, // Week 12
	})

	points := AggregateRiskTrends(events, GranularityWeek)
	require.Len(t, points, 2)
	assert.Equal(t, 2, points[0].Count)
	assert.Equal(t, 1, points[1].Count)
}

func TestAggregateRiskTrends_SkipsNonRiskEvents(t *testing.T) {
	payload, _ := MarshalPayload(BumpTypePayload{BumpType: "minor"})
	events := []Event{{
		ID:        "e1",
		Timestamp: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
		Type:      EventBumpType,
		Payload:   payload,
	}}

	points := AggregateRiskTrends(events, GranularityDay)
	assert.Empty(t, points)
}

func TestAggregateRiskTrends_Empty(t *testing.T) {
	points := AggregateRiskTrends(nil, GranularityDay)
	assert.Empty(t, points)
}

func TestAggregateDecisions(t *testing.T) {
	events := buildDecisionEvents(t, []decisionInput{
		{time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC), "approve"},
		{time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC), "deny"},
		{time.Date(2026, 3, 10, 16, 0, 0, 0, time.UTC), "approve"},
		{time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC), "require_review"},
	})

	dist := AggregateDecisions(events, GranularityDay)
	require.Len(t, dist, 2)

	assert.Equal(t, "2026-03-10", dist[0].Bucket)
	assert.Equal(t, 2, dist[0].Approve)
	assert.Equal(t, 1, dist[0].Deny)
	assert.Equal(t, 0, dist[0].RequireReview)
	assert.Equal(t, 3, dist[0].Total)

	assert.Equal(t, "2026-03-11", dist[1].Bucket)
	assert.Equal(t, 1, dist[1].RequireReview)
	assert.Equal(t, 1, dist[1].Total)
}

func TestAggregateDecisions_Empty(t *testing.T) {
	dist := AggregateDecisions(nil, GranularityDay)
	assert.Empty(t, dist)
}

func TestAggregateTeamMetrics(t *testing.T) {
	var events []Event

	// Approval events
	for _, input := range []struct {
		actorID string
		outcome string
	}{
		{"alice", "approved"},
		{"alice", "approved"},
		{"bob", "rejected"},
		{"alice", "rejected"},
	} {
		payload, _ := MarshalPayload(ApprovalOutcomePayload{
			Outcome:   input.outcome,
			ActorID:   input.actorID,
			ActorKind: "human",
		})
		events = append(events, Event{
			Timestamp: time.Now(),
			Type:      EventApprovalOutcome,
			Payload:   payload,
		})
	}

	// Release duration events
	for _, input := range []struct {
		durationMs int64
		success    bool
	}{
		{5000, true},
		{8000, false},
		{3000, true},
	} {
		payload, _ := MarshalPayload(ReleaseDurationPayload{
			DurationMs: input.durationMs,
			Success:    input.success,
		})
		events = append(events, Event{
			Timestamp: time.Now(),
			Type:      EventReleaseDuration,
			Payload:   payload,
		})
	}

	metrics := AggregateTeamMetrics(events)

	// Find alice
	var alice, bob, system *TeamMetrics
	for i := range metrics {
		switch metrics[i].ActorID {
		case "alice":
			alice = &metrics[i]
		case "bob":
			bob = &metrics[i]
		case "system":
			system = &metrics[i]
		}
	}

	require.NotNil(t, alice)
	assert.Equal(t, 2, alice.ApprovalCount)
	assert.Equal(t, 1, alice.RejectionCount)

	require.NotNil(t, bob)
	assert.Equal(t, 0, bob.ApprovalCount)
	assert.Equal(t, 1, bob.RejectionCount)

	require.NotNil(t, system)
	assert.Equal(t, 3, system.ReleaseCount)
	assert.InDelta(t, float64(16000)/3, system.AvgDurationMs, 1.0)
	assert.InDelta(t, float64(2)/3, system.SuccessRate, 0.01)
}

func TestAggregateTeamMetrics_Empty(t *testing.T) {
	metrics := AggregateTeamMetrics(nil)
	assert.Empty(t, metrics)
}

func TestAggregateTeamMetrics_EmptyActorID(t *testing.T) {
	payload, _ := MarshalPayload(ApprovalOutcomePayload{
		Outcome:   "approved",
		ActorID:   "",
		ActorKind: "ci",
	})
	events := []Event{{
		Timestamp: time.Now(),
		Type:      EventApprovalOutcome,
		Payload:   payload,
	}}

	metrics := AggregateTeamMetrics(events)
	require.Len(t, metrics, 1)
	assert.Equal(t, "unknown", metrics[0].ActorID)
}

// =============================================================================
// CachedAggregator Tests
// =============================================================================

func TestCachedAggregator_RiskTrends(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	// Seed data
	_ = svc.Capture(ctx, EventRiskEvaluation, "rel-1", RiskEvaluationPayload{RiskScore: 0.5, RiskLevel: "medium"})

	agg := NewCachedAggregator(svc, 5*time.Minute)

	trends, err := agg.RiskTrends(ctx, QueryFilter{}, GranularityDay)
	require.NoError(t, err)
	assert.Len(t, trends, 1)

	// Second call should use cache
	trends2, err := agg.RiskTrends(ctx, QueryFilter{}, GranularityDay)
	require.NoError(t, err)
	assert.Equal(t, trends, trends2)
}

func TestCachedAggregator_Decisions(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	_ = svc.Capture(ctx, EventPolicyDecision, "rel-1", PolicyDecisionPayload{Decision: "approve", RiskScore: 0.3})

	agg := NewCachedAggregator(svc, 5*time.Minute)

	decisions, err := agg.Decisions(ctx, QueryFilter{}, GranularityDay)
	require.NoError(t, err)
	assert.Len(t, decisions, 1)
	assert.Equal(t, 1, decisions[0].Approve)
}

func TestCachedAggregator_Team(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	_ = svc.Capture(ctx, EventApprovalOutcome, "", ApprovalOutcomePayload{
		Outcome: "approved", ActorID: "alice", ActorKind: "human",
	})

	agg := NewCachedAggregator(svc, 5*time.Minute)

	team, err := agg.Team(ctx, QueryFilter{})
	require.NoError(t, err)
	require.Len(t, team, 1)
	assert.Equal(t, "alice", team[0].ActorID)
	assert.Equal(t, 1, team[0].ApprovalCount)
}

func TestCachedAggregator_CacheExpiry(t *testing.T) {
	store, _ := NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	svc := NewService(store)
	ctx := context.Background()

	_ = svc.Capture(ctx, EventRiskEvaluation, "", RiskEvaluationPayload{RiskScore: 0.3})

	agg := NewCachedAggregator(svc, 1*time.Millisecond)

	trends1, _ := agg.RiskTrends(ctx, QueryFilter{}, GranularityDay)
	assert.Len(t, trends1, 1)

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Add another event
	_ = svc.Capture(ctx, EventRiskEvaluation, "", RiskEvaluationPayload{RiskScore: 0.7})

	// Should get fresh data now
	trends2, _ := agg.RiskTrends(ctx, QueryFilter{}, GranularityDay)
	assert.Len(t, trends2, 1) // Same day, so still 1 bucket but count should be 2
	assert.Equal(t, 2, trends2[0].Count)
}

// =============================================================================
// Test Helpers
// =============================================================================

type riskInput struct {
	ts    time.Time
	score float64
}

func buildRiskEvents(t *testing.T, inputs []riskInput) []Event {
	t.Helper()
	var events []Event
	for i, input := range inputs {
		payload, err := MarshalPayload(RiskEvaluationPayload{RiskScore: input.score, RiskLevel: "test"})
		require.NoError(t, err)
		events = append(events, Event{
			ID:        "e" + string(rune('0'+i)),
			Timestamp: input.ts,
			Type:      EventRiskEvaluation,
			Payload:   payload,
		})
	}
	return events
}

type decisionInput struct {
	ts       time.Time
	decision string
}

func buildDecisionEvents(t *testing.T, inputs []decisionInput) []Event {
	t.Helper()
	var events []Event
	for i, input := range inputs {
		payload, err := MarshalPayload(PolicyDecisionPayload{Decision: input.decision, RiskScore: 0.5})
		require.NoError(t, err)
		events = append(events, Event{
			ID:        "e" + string(rune('0'+i)),
			Timestamp: input.ts,
			Type:      EventPolicyDecision,
			Payload:   payload,
		})
	}
	return events
}
