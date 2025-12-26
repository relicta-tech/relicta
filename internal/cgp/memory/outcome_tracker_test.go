package memory

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestOutcomeTracker_PublishedEvent(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	releaseID := release.ReleaseID("test-release-1")

	// Set up context
	tracker.SetReleaseContext(
		releaseID,
		"owner/repo",
		"1.2.0",
		cgp.NewHumanActor("dev-user", "Developer"),
		0.3,
		cgp.DecisionApproved,
	)
	tracker.SetChangeMetrics(releaseID, 1, 0, 10, 150)

	// Publish events
	ver := version.MustParse("1.2.0")
	publishedEvent := &release.RunPublishedEvent{RunID: releaseID, Version: ver, At: time.Now()}

	err := tracker.Publish(context.Background(), publishedEvent)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify release record was created
	history, err := store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 release record, got %d", len(history))
	}

	record := history[0]
	if record.ID != string(releaseID) {
		t.Errorf("expected ID %q, got %q", releaseID, record.ID)
	}
	if record.Repository != "owner/repo" {
		t.Errorf("expected repository %q, got %q", "owner/repo", record.Repository)
	}
	if record.Version != "1.2.0" {
		t.Errorf("expected version %q, got %q", "1.2.0", record.Version)
	}
	if record.Outcome != OutcomeSuccess {
		t.Errorf("expected outcome %q, got %q", OutcomeSuccess, record.Outcome)
	}
	if record.RiskScore != 0.3 {
		t.Errorf("expected risk score 0.3, got %f", record.RiskScore)
	}
	if record.BreakingChanges != 1 {
		t.Errorf("expected 1 breaking change, got %d", record.BreakingChanges)
	}
}

func TestOutcomeTracker_FailedEvent(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	releaseID := release.ReleaseID("test-release-2")

	// Set up context
	tracker.SetReleaseContext(
		releaseID,
		"owner/repo",
		"2.0.0",
		cgp.NewAgentActor("ci-agent", "CI System", ""),
		0.7,
		cgp.DecisionApproved,
	)

	// Publish failed event
	failedEvent := &release.RunFailedEvent{
		RunID:  releaseID,
		Reason: "plugin execution failed",
		At:     time.Now(),
	}

	err := tracker.Publish(context.Background(), failedEvent)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify release record was created
	history, err := store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 release record, got %d", len(history))
	}

	record := history[0]
	if record.Outcome != OutcomeFailed {
		t.Errorf("expected outcome %q, got %q", OutcomeFailed, record.Outcome)
	}
	if record.Metadata["failure_reason"] != "plugin execution failed" {
		t.Errorf("expected failure reason in metadata, got %v", record.Metadata)
	}
	if _, ok := record.Metadata["failure_reason"]; !ok {
		t.Errorf("expected failure_reason in metadata, got %v", record.Metadata)
	}
}

func TestOutcomeTracker_CanceledEvent(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	releaseID := release.ReleaseID("test-release-3")

	// Set up context
	tracker.SetReleaseContext(
		releaseID,
		"owner/repo",
		"3.0.0",
		cgp.NewHumanActor("admin", "Admin User"),
		0.5,
		cgp.DecisionApprovalRequired,
	)

	// Publish canceled event
	canceledEvent := &release.RunCancelledEvent{
		RunID:  releaseID,
		Reason: "user requested cancellation",
		By:     "admin",
		At:     time.Now(),
	}

	err := tracker.Publish(context.Background(), canceledEvent)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify release record was created
	history, err := store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 release record, got %d", len(history))
	}

	record := history[0]
	if record.Outcome != OutcomePartial {
		t.Errorf("expected outcome %q, got %q", OutcomePartial, record.Outcome)
	}
	if record.Metadata["canceled_by"] != "admin" {
		t.Errorf("expected canceled_by in metadata, got %v", record.Metadata)
	}
	if !containsTag(record.Tags, "canceled") {
		t.Errorf("expected 'canceled' tag, got %v", record.Tags)
	}
}

func TestOutcomeTracker_ForwardsToNextPublisher(t *testing.T) {
	store := NewInMemoryStore()
	nextPublisher := &mockEventPublisher{}
	tracker := NewOutcomeTracker(store, nextPublisher)

	releaseID := release.ReleaseID("test-release-4")
	event := &release.RunCreatedEvent{RunID: releaseID, RepoID: "/path/to/repo", At: time.Now()}

	err := tracker.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if len(nextPublisher.events) != 1 {
		t.Fatalf("expected 1 event forwarded, got %d", len(nextPublisher.events))
	}

	if nextPublisher.events[0].EventName() != "run.created" {
		t.Errorf("expected 'run.created' event, got %q", nextPublisher.events[0].EventName())
	}
}

func TestOutcomeTracker_ActorMetricsUpdated(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	actorID := "human:dev-user"
	actor := cgp.NewHumanActor("dev-user", "Developer")

	// Create successful release
	releaseID1 := release.ReleaseID("release-1")
	tracker.SetReleaseContext(releaseID1, "owner/repo", "1.0.0", actor, 0.2, cgp.DecisionApproved)
	ver1 := version.MustParse("1.0.0")
	err := tracker.Publish(context.Background(), &release.RunPublishedEvent{RunID: releaseID1, Version: ver1, At: time.Now()})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Create failed release
	releaseID2 := release.ReleaseID("release-2")
	tracker.SetReleaseContext(releaseID2, "owner/repo", "1.1.0", actor, 0.8, cgp.DecisionApproved)
	err = tracker.Publish(context.Background(), &release.RunFailedEvent{RunID: releaseID2, Reason: "test failure", At: time.Now()})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Check actor metrics
	metrics, err := store.GetActorMetrics(context.Background(), actorID)
	if err != nil {
		t.Fatalf("GetActorMetrics failed: %v", err)
	}

	if metrics.TotalReleases != 2 {
		t.Errorf("expected 2 total releases, got %d", metrics.TotalReleases)
	}
	if metrics.SuccessfulReleases != 1 {
		t.Errorf("expected 1 successful release, got %d", metrics.SuccessfulReleases)
	}
	if metrics.FailedReleases != 1 {
		t.Errorf("expected 1 failed release, got %d", metrics.FailedReleases)
	}
	if metrics.SuccessRate != 0.5 {
		t.Errorf("expected 50%% success rate, got %f", metrics.SuccessRate)
	}
}

func TestOutcomeTracker_DurationTracking(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	releaseID := release.ReleaseID("timed-release")

	// Simulate release start
	startTime := time.Now().Add(-5 * time.Minute)
	ctx := tracker.getOrCreateContext(releaseID)
	ctx.Repository = "owner/repo"
	ctx.Version = "1.0.0"
	ctx.StartedAt = startTime

	// Publish completed event
	ver := version.MustParse("1.0.0")
	err := tracker.Publish(context.Background(), &release.RunPublishedEvent{RunID: releaseID, Version: ver, At: time.Now()})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Check duration is approximately 5 minutes
	history, err := store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 release record, got %d", len(history))
	}

	record := history[0]
	// Allow some tolerance for test execution time
	if record.Duration < 4*time.Minute || record.Duration > 6*time.Minute {
		t.Errorf("expected duration around 5 minutes, got %v", record.Duration)
	}
}

func TestOutcomeTracker_AddTags(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	releaseID := release.ReleaseID("tagged-release")
	tracker.SetReleaseContext(releaseID, "owner/repo", "1.0.0", cgp.NewHumanActor("dev", "Dev"), 0.1, cgp.DecisionApproved)
	tracker.AddTags(releaseID, "hotfix", "production")

	ver := version.MustParse("1.0.0")
	err := tracker.Publish(context.Background(), &release.RunPublishedEvent{RunID: releaseID, Version: ver, At: time.Now()})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	history, err := store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	record := history[0]
	if !containsTag(record.Tags, "hotfix") {
		t.Errorf("expected 'hotfix' tag, got %v", record.Tags)
	}
	if !containsTag(record.Tags, "production") {
		t.Errorf("expected 'production' tag, got %v", record.Tags)
	}
}

func TestOutcomeTracker_EventChaining(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil)

	releaseID := release.ReleaseID("chained-release")

	// Simulate the full event lifecycle
	events := []release.DomainEvent{
		&release.RunCreatedEvent{RunID: releaseID, RepoID: "owner/repo", At: time.Now()},
		&release.RunPlannedEvent{RunID: releaseID, VersionCurrent: version.MustParse("1.0.0"), VersionNext: version.MustParse("1.1.0"), BumpKind: release.BumpMinor, CommitCount: 5, At: time.Now()},
		&release.RunApprovedEvent{RunID: releaseID, ApprovedBy: "approver", At: time.Now()},
		&release.RunPublishedEvent{RunID: releaseID, Version: version.MustParse("1.1.0"), At: time.Now()},
	}

	err := tracker.Publish(context.Background(), events...)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	history, err := store.GetReleaseHistory(context.Background(), "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 release record, got %d", len(history))
	}

	record := history[0]
	if record.Version != "1.1.0" {
		t.Errorf("expected version '1.1.0', got %q", record.Version)
	}
	if record.Metadata["approved_by"] != "approver" {
		t.Errorf("expected approved_by metadata, got %v", record.Metadata)
	}
}

// mockEventPublisher is a test double for EventPublisher.
type mockEventPublisher struct {
	events []release.DomainEvent
}

func (m *mockEventPublisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	m.events = append(m.events, events...)
	return nil
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
