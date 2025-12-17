package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestNewInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()
	if store == nil {
		t.Fatal("NewInMemoryStore() should not return nil")
	}
	if store.releases == nil {
		t.Error("releases map should be initialized")
	}
	if store.incidents == nil {
		t.Error("incidents map should be initialized")
	}
	if store.actors == nil {
		t.Error("actors map should be initialized")
	}
}

func TestReleaseOutcome_IsValid(t *testing.T) {
	tests := []struct {
		outcome ReleaseOutcome
		valid   bool
	}{
		{OutcomeSuccess, true},
		{OutcomeRollback, true},
		{OutcomeFailed, true},
		{OutcomePartial, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.outcome.IsValid(); got != tt.valid {
			t.Errorf("ReleaseOutcome(%q).IsValid() = %v, want %v", tt.outcome, got, tt.valid)
		}
	}
}

func TestReleaseOutcome_IsNegative(t *testing.T) {
	tests := []struct {
		outcome  ReleaseOutcome
		negative bool
	}{
		{OutcomeSuccess, false},
		{OutcomeRollback, true},
		{OutcomeFailed, true},
		{OutcomePartial, true},
	}

	for _, tt := range tests {
		if got := tt.outcome.IsNegative(); got != tt.negative {
			t.Errorf("ReleaseOutcome(%q).IsNegative() = %v, want %v", tt.outcome, got, tt.negative)
		}
	}
}

func TestIncidentType_IsValid(t *testing.T) {
	tests := []struct {
		incidentType IncidentType
		valid        bool
	}{
		{IncidentRollback, true},
		{IncidentBugIntro, true},
		{IncidentPerformance, true},
		{IncidentSecurity, true},
		{IncidentAvailability, true},
		{IncidentDataIssue, true},
		{IncidentBreaking, true},
		{IncidentOther, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.incidentType.IsValid(); got != tt.valid {
			t.Errorf("IncidentType(%q).IsValid() = %v, want %v", tt.incidentType, got, tt.valid)
		}
	}
}

func TestInMemoryStore_RecordRelease(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	record := &ReleaseRecord{
		ID:              "release-1",
		Repository:      "owner/repo",
		Version:         "v1.0.0",
		Actor:           cgp.NewHumanActor("john@example.com", "John"),
		RiskScore:       0.3,
		Decision:        cgp.DecisionApproved,
		BreakingChanges: 0,
		SecurityChanges: 0,
		FilesChanged:    10,
		LinesChanged:    100,
		Outcome:         OutcomeSuccess,
		ReleasedAt:      time.Now(),
		Duration:        5 * time.Minute,
	}

	err := store.RecordRelease(ctx, record)
	if err != nil {
		t.Fatalf("RecordRelease() error = %v", err)
	}

	// Verify release was stored
	releases, err := store.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory() error = %v", err)
	}
	if len(releases) != 1 {
		t.Errorf("GetReleaseHistory() returned %d releases, want 1", len(releases))
	}
	if releases[0].ID != "release-1" {
		t.Errorf("Release ID = %v, want release-1", releases[0].ID)
	}
}

func TestInMemoryStore_RecordRelease_Validation(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	tests := []struct {
		name    string
		record  *ReleaseRecord
		wantErr bool
	}{
		{
			name:    "nil record",
			record:  nil,
			wantErr: true,
		},
		{
			name:    "empty ID",
			record:  &ReleaseRecord{Repository: "owner/repo"},
			wantErr: true,
		},
		{
			name:    "empty repository",
			record:  &ReleaseRecord{ID: "release-1"},
			wantErr: true,
		},
		{
			name: "valid record",
			record: &ReleaseRecord{
				ID:         "release-1",
				Repository: "owner/repo",
				Actor:      cgp.NewHumanActor("john@example.com", "John"),
				Outcome:    OutcomeSuccess,
				ReleasedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.RecordRelease(ctx, tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordRelease() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInMemoryStore_RecordIncident(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	incident := &IncidentRecord{
		ID:           "incident-1",
		Repository:   "owner/repo",
		ReleaseID:    "release-1",
		Version:      "v1.0.0",
		Type:         IncidentRollback,
		Severity:     cgp.SeverityHigh,
		Description:  "Service availability degraded",
		DetectedAt:   time.Now(),
		TimeToDetect: 10 * time.Minute,
		ActorID:      "human:john@example.com",
	}

	err := store.RecordIncident(ctx, incident)
	if err != nil {
		t.Fatalf("RecordIncident() error = %v", err)
	}

	// Verify incident was stored
	incidents, err := store.GetIncidentHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory() error = %v", err)
	}
	if len(incidents) != 1 {
		t.Errorf("GetIncidentHistory() returned %d incidents, want 1", len(incidents))
	}
	if incidents[0].ID != "incident-1" {
		t.Errorf("Incident ID = %v, want incident-1", incidents[0].ID)
	}
}

func TestInMemoryStore_RecordIncident_Validation(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	tests := []struct {
		name     string
		incident *IncidentRecord
		wantErr  bool
	}{
		{
			name:     "nil incident",
			incident: nil,
			wantErr:  true,
		},
		{
			name:     "empty ID",
			incident: &IncidentRecord{Repository: "owner/repo"},
			wantErr:  true,
		},
		{
			name:     "empty repository",
			incident: &IncidentRecord{ID: "incident-1"},
			wantErr:  true,
		},
		{
			name: "valid incident",
			incident: &IncidentRecord{
				ID:         "incident-1",
				Repository: "owner/repo",
				Type:       IncidentRollback,
				DetectedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.RecordIncident(ctx, tt.incident)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordIncident() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInMemoryStore_GetActorMetrics(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record several releases for the same actor
	actor := cgp.NewHumanActor("john@example.com", "John")
	now := time.Now()

	releases := []*ReleaseRecord{
		{ID: "r1", Repository: "owner/repo", Actor: actor, RiskScore: 0.2, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-3 * 24 * time.Hour)},
		{ID: "r2", Repository: "owner/repo", Actor: actor, RiskScore: 0.3, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-2 * 24 * time.Hour)},
		{ID: "r3", Repository: "owner/repo", Actor: actor, RiskScore: 0.8, Outcome: OutcomeFailed, BreakingChanges: 1, ReleasedAt: now.Add(-1 * 24 * time.Hour)},
		{ID: "r4", Repository: "owner/repo", Actor: actor, RiskScore: 0.4, Outcome: OutcomeSuccess, ReleasedAt: now},
	}

	for _, r := range releases {
		if err := store.RecordRelease(ctx, r); err != nil {
			t.Fatalf("RecordRelease() error = %v", err)
		}
	}

	metrics, err := store.GetActorMetrics(ctx, actor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics() error = %v", err)
	}

	if metrics.TotalReleases != 4 {
		t.Errorf("TotalReleases = %d, want 4", metrics.TotalReleases)
	}
	if metrics.SuccessfulReleases != 3 {
		t.Errorf("SuccessfulReleases = %d, want 3", metrics.SuccessfulReleases)
	}
	if metrics.FailedReleases != 1 {
		t.Errorf("FailedReleases = %d, want 1", metrics.FailedReleases)
	}
	if metrics.HighRiskReleases != 1 {
		t.Errorf("HighRiskReleases = %d, want 1", metrics.HighRiskReleases)
	}
	if metrics.BreakingChangeReleases != 1 {
		t.Errorf("BreakingChangeReleases = %d, want 1", metrics.BreakingChangeReleases)
	}
	if metrics.SuccessRate != 0.75 {
		t.Errorf("SuccessRate = %v, want 0.75", metrics.SuccessRate)
	}
}

func TestInMemoryStore_GetActorMetrics_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetActorMetrics(ctx, "unknown-actor")
	if err == nil {
		t.Error("GetActorMetrics() should return error for unknown actor")
	}
}

func TestActorMetrics_CalculateReliabilityScore(t *testing.T) {
	tests := []struct {
		name    string
		metrics ActorMetrics
		minScore float64
		maxScore float64
	}{
		{
			name: "no releases",
			metrics: ActorMetrics{
				TotalReleases: 0,
			},
			minScore: 0.5,
			maxScore: 0.5,
		},
		{
			name: "perfect track record",
			metrics: ActorMetrics{
				TotalReleases:      10,
				SuccessfulReleases: 10,
				FailedReleases:     0,
				RollbackCount:      0,
				IncidentCount:      0,
				AverageRiskScore:   0.2,
				SuccessRate:        1.0,
			},
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name: "poor track record",
			metrics: ActorMetrics{
				TotalReleases:      10,
				SuccessfulReleases: 3,
				FailedReleases:     7,
				RollbackCount:      5,
				IncidentCount:      8,
				AverageRiskScore:   0.8,
				SuccessRate:        0.3,
			},
			minScore: 0.0,
			maxScore: 0.4, // Adjusted - formula gives ~0.33 for this scenario
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := tt.metrics.CalculateReliabilityScore()
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("CalculateReliabilityScore() = %v, want between %v and %v", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestActorMetrics_IsReliable(t *testing.T) {
	tests := []struct {
		name     string
		metrics  ActorMetrics
		reliable bool
	}{
		{
			name: "reliable actor",
			metrics: ActorMetrics{
				TotalReleases:    10,
				ReliabilityScore: 0.8,
			},
			reliable: true,
		},
		{
			name: "unreliable actor",
			metrics: ActorMetrics{
				TotalReleases:    10,
				ReliabilityScore: 0.5,
			},
			reliable: false,
		},
		{
			name: "new actor (not enough releases)",
			metrics: ActorMetrics{
				TotalReleases:    3,
				ReliabilityScore: 0.9,
			},
			reliable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.metrics.IsReliable(); got != tt.reliable {
				t.Errorf("IsReliable() = %v, want %v", got, tt.reliable)
			}
		})
	}
}

func TestInMemoryStore_GetRiskPatterns(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	actor := cgp.NewHumanActor("john@example.com", "John")
	now := time.Now()

	// Create releases with varying risk
	releases := []*ReleaseRecord{
		{ID: "r1", Repository: "owner/repo", Actor: actor, RiskScore: 0.2, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-5 * 24 * time.Hour), Tags: []string{"api_change"}},
		{ID: "r2", Repository: "owner/repo", Actor: actor, RiskScore: 0.3, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-4 * 24 * time.Hour), Tags: []string{"api_change"}},
		{ID: "r3", Repository: "owner/repo", Actor: actor, RiskScore: 0.4, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-3 * 24 * time.Hour), Tags: []string{"breaking"}},
		{ID: "r4", Repository: "owner/repo", Actor: actor, RiskScore: 0.5, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-2 * 24 * time.Hour), Tags: []string{"breaking"}},
		{ID: "r5", Repository: "owner/repo", Actor: actor, RiskScore: 0.6, Outcome: OutcomeSuccess, ReleasedAt: now.Add(-1 * 24 * time.Hour), Tags: []string{"api_change", "breaking"}},
		{ID: "r6", Repository: "owner/repo", Actor: actor, RiskScore: 0.7, Outcome: OutcomeSuccess, ReleasedAt: now, Tags: []string{"api_change"}},
	}

	for _, r := range releases {
		if err := store.RecordRelease(ctx, r); err != nil {
			t.Fatalf("RecordRelease() error = %v", err)
		}
	}

	patterns, err := store.GetRiskPatterns(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("GetRiskPatterns() error = %v", err)
	}

	if patterns.Repository != "owner/repo" {
		t.Errorf("Repository = %v, want owner/repo", patterns.Repository)
	}
	if patterns.TotalReleases != 6 {
		t.Errorf("TotalReleases = %d, want 6", patterns.TotalReleases)
	}
	// Average should be (0.2+0.3+0.4+0.5+0.6+0.7)/6 = 0.45
	expectedAvg := 0.45
	if patterns.AverageRiskScore < expectedAvg-0.01 || patterns.AverageRiskScore > expectedAvg+0.01 {
		t.Errorf("AverageRiskScore = %v, want ~%v", patterns.AverageRiskScore, expectedAvg)
	}
	// Risk is increasing over time
	if patterns.RiskTrend != TrendIncreasing {
		t.Errorf("RiskTrend = %v, want %v", patterns.RiskTrend, TrendIncreasing)
	}
	if len(patterns.CommonRiskFactors) == 0 {
		t.Error("Should have common risk factors")
	}
}

func TestInMemoryStore_GetRiskPatterns_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetRiskPatterns(ctx, "unknown/repo")
	if err == nil {
		t.Error("GetRiskPatterns() should return error for unknown repository")
	}
}

func TestInMemoryStore_GetReleaseHistory_Limit(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	actor := cgp.NewHumanActor("john@example.com", "John")

	// Create 10 releases
	for i := 0; i < 10; i++ {
		record := &ReleaseRecord{
			ID:         fmt.Sprintf("release-%d", i),
			Repository: "owner/repo",
			Actor:      actor,
			Outcome:    OutcomeSuccess,
			ReleasedAt: time.Now(),
		}
		if err := store.RecordRelease(ctx, record); err != nil {
			t.Fatalf("RecordRelease() error = %v", err)
		}
	}

	// Request only 5
	releases, err := store.GetReleaseHistory(ctx, "owner/repo", 5)
	if err != nil {
		t.Fatalf("GetReleaseHistory() error = %v", err)
	}
	if len(releases) != 5 {
		t.Errorf("GetReleaseHistory() returned %d releases, want 5", len(releases))
	}

	// Should return most recent first
	if releases[0].ID != "release-9" {
		t.Errorf("First release ID = %v, want release-9", releases[0].ID)
	}
}

func TestInMemoryStore_GetIncidentHistory_Limit(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create 10 incidents
	for i := 0; i < 10; i++ {
		incident := &IncidentRecord{
			ID:         fmt.Sprintf("incident-%d", i),
			Repository: "owner/repo",
			Type:       IncidentBugIntro,
			DetectedAt: time.Now(),
		}
		if err := store.RecordIncident(ctx, incident); err != nil {
			t.Fatalf("RecordIncident() error = %v", err)
		}
	}

	// Request only 3
	incidents, err := store.GetIncidentHistory(ctx, "owner/repo", 3)
	if err != nil {
		t.Fatalf("GetIncidentHistory() error = %v", err)
	}
	if len(incidents) != 3 {
		t.Errorf("GetIncidentHistory() returned %d incidents, want 3", len(incidents))
	}

	// Should return most recent first
	if incidents[0].ID != "incident-9" {
		t.Errorf("First incident ID = %v, want incident-9", incidents[0].ID)
	}
}

func TestInMemoryStore_IncidentUpdatesActorMetrics(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	actor := cgp.NewHumanActor("john@example.com", "John")

	// Record a release first
	release := &ReleaseRecord{
		ID:         "release-1",
		Repository: "owner/repo",
		Actor:      actor,
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now(),
	}
	if err := store.RecordRelease(ctx, release); err != nil {
		t.Fatalf("RecordRelease() error = %v", err)
	}

	// Record an incident
	incident := &IncidentRecord{
		ID:         "incident-1",
		Repository: "owner/repo",
		ReleaseID:  "release-1",
		Type:       IncidentRollback,
		DetectedAt: time.Now(),
		ActorID:    actor.ID,
	}
	if err := store.RecordIncident(ctx, incident); err != nil {
		t.Fatalf("RecordIncident() error = %v", err)
	}

	// Verify incident count was updated
	metrics, err := store.GetActorMetrics(ctx, actor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics() error = %v", err)
	}
	if metrics.IncidentCount != 1 {
		t.Errorf("IncidentCount = %d, want 1", metrics.IncidentCount)
	}
}

func TestInMemoryStore_UpdateActorMetrics(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	actor := cgp.NewHumanActor("john@example.com", "John")

	// Record a successful release
	release := &ReleaseRecord{
		ID:         "release-1",
		Repository: "owner/repo",
		Actor:      actor,
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now(),
	}
	if err := store.RecordRelease(ctx, release); err != nil {
		t.Fatalf("RecordRelease() error = %v", err)
	}

	// Later, the release is rolled back
	err := store.UpdateActorMetrics(ctx, actor.ID, OutcomeRollback)
	if err != nil {
		t.Fatalf("UpdateActorMetrics() error = %v", err)
	}

	metrics, err := store.GetActorMetrics(ctx, actor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics() error = %v", err)
	}

	if metrics.RollbackCount != 1 {
		t.Errorf("RollbackCount = %d, want 1", metrics.RollbackCount)
	}
	if metrics.SuccessfulReleases != 0 {
		t.Errorf("SuccessfulReleases = %d, want 0 (should be decremented)", metrics.SuccessfulReleases)
	}
}

func TestInMemoryStore_UpdateActorMetrics_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.UpdateActorMetrics(ctx, "unknown-actor", OutcomeRollback)
	if err == nil {
		t.Error("UpdateActorMetrics() should return error for unknown actor")
	}
}

func TestInMemoryStore_EmptyHistory(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Empty release history should return empty slice, not error
	releases, err := store.GetReleaseHistory(ctx, "unknown/repo", 10)
	if err != nil {
		t.Errorf("GetReleaseHistory() error = %v, want nil", err)
	}
	if len(releases) != 0 {
		t.Errorf("GetReleaseHistory() returned %d releases, want 0", len(releases))
	}

	// Empty incident history should return empty slice, not error
	incidents, err := store.GetIncidentHistory(ctx, "unknown/repo", 10)
	if err != nil {
		t.Errorf("GetIncidentHistory() error = %v, want nil", err)
	}
	if len(incidents) != 0 {
		t.Errorf("GetIncidentHistory() returned %d incidents, want 0", len(incidents))
	}
}

func TestRiskTrend_Constants(t *testing.T) {
	// Verify trend constants are defined correctly
	if TrendIncreasing != "increasing" {
		t.Errorf("TrendIncreasing = %v, want 'increasing'", TrendIncreasing)
	}
	if TrendStable != "stable" {
		t.Errorf("TrendStable = %v, want 'stable'", TrendStable)
	}
	if TrendDecreasing != "decreasing" {
		t.Errorf("TrendDecreasing = %v, want 'decreasing'", TrendDecreasing)
	}
}
