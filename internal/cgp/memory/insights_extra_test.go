package memory

import (
	"context"
	"testing"
	"time"
)

// TestCalculateRiskTrend exercises the calculateRiskTrend function directly.
func TestCalculateRiskTrend(t *testing.T) {
	now := time.Now()
	since := now.Add(-30 * 24 * time.Hour)

	tests := []struct {
		name      string
		releases  []*ReleaseRecord
		wantTrend RiskTrend
	}{
		{
			name:      "empty releases returns stable",
			releases:  nil,
			wantTrend: TrendStable,
		},
		{
			name: "less than 4 releases returns stable",
			releases: []*ReleaseRecord{
				{RiskScore: 0.2, ReleasedAt: now.Add(-3 * 24 * time.Hour)},
				{RiskScore: 0.8, ReleasedAt: now.Add(-2 * 24 * time.Hour)},
				{RiskScore: 0.5, ReleasedAt: now.Add(-1 * 24 * time.Hour)},
			},
			wantTrend: TrendStable,
		},
		{
			name: "increasing risk trend",
			releases: []*ReleaseRecord{
				{RiskScore: 0.1, ReleasedAt: now.Add(-8 * 24 * time.Hour)},
				{RiskScore: 0.2, ReleasedAt: now.Add(-6 * 24 * time.Hour)},
				{RiskScore: 0.6, ReleasedAt: now.Add(-4 * 24 * time.Hour)},
				{RiskScore: 0.8, ReleasedAt: now.Add(-2 * 24 * time.Hour)},
			},
			wantTrend: TrendIncreasing,
		},
		{
			name: "decreasing risk trend",
			releases: []*ReleaseRecord{
				{RiskScore: 0.8, ReleasedAt: now.Add(-8 * 24 * time.Hour)},
				{RiskScore: 0.7, ReleasedAt: now.Add(-6 * 24 * time.Hour)},
				{RiskScore: 0.2, ReleasedAt: now.Add(-4 * 24 * time.Hour)},
				{RiskScore: 0.1, ReleasedAt: now.Add(-2 * 24 * time.Hour)},
			},
			wantTrend: TrendDecreasing,
		},
		{
			name: "stable risk trend",
			releases: []*ReleaseRecord{
				{RiskScore: 0.4, ReleasedAt: now.Add(-8 * 24 * time.Hour)},
				{RiskScore: 0.5, ReleasedAt: now.Add(-6 * 24 * time.Hour)},
				{RiskScore: 0.4, ReleasedAt: now.Add(-4 * 24 * time.Hour)},
				{RiskScore: 0.5, ReleasedAt: now.Add(-2 * 24 * time.Hour)},
			},
			wantTrend: TrendStable,
		},
		{
			name: "releases outside window are filtered out",
			releases: []*ReleaseRecord{
				// These two are outside the 30-day window.
				{RiskScore: 0.1, ReleasedAt: now.Add(-60 * 24 * time.Hour)},
				{RiskScore: 0.2, ReleasedAt: now.Add(-45 * 24 * time.Hour)},
				// Only two inside — not enough for a trend.
				{RiskScore: 0.4, ReleasedAt: now.Add(-5 * 24 * time.Hour)},
				{RiskScore: 0.5, ReleasedAt: now.Add(-1 * 24 * time.Hour)},
			},
			wantTrend: TrendStable, // 2 filtered releases → < 4 → stable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateRiskTrend(tt.releases, since)
			if got != tt.wantTrend {
				t.Errorf("calculateRiskTrend() = %v, want %v", got, tt.wantTrend)
			}
		})
	}
}

// TestPatternSeverity covers all three severity branches.
func TestPatternSeverity(t *testing.T) {
	tests := []struct {
		name     string
		modifier float64
		expected string
	}{
		{"critical: >= 0.2", 0.2, "critical"},
		{"critical: high value", 0.5, "critical"},
		{"warning: >= 0.1 and < 0.2", 0.1, "warning"},
		{"warning: boundary", 0.15, "warning"},
		{"info: < 0.1", 0.05, "info"},
		{"info: zero", 0.0, "info"},
		{"info: negative", -0.1, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Pattern{RiskModifier: tt.modifier}
			got := patternSeverity(p)
			if got != tt.expected {
				t.Errorf("patternSeverity(%v) = %q, want %q", tt.modifier, got, tt.expected)
			}
		})
	}
}

// TestGetAllOutcomes verifies the GetAllOutcomes method on InMemoryOutcomeStore.
func TestGetAllOutcomes(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	ctx := context.Background()
	now := time.Now()

	// Add outcomes for two different repos.
	outcomes := []*OutcomeRecord{
		{ID: "oc-1", ReleaseID: "rel-1", Repository: "owner/repo-a", OutcomeType: OutcomeTypeSuccessfulRelease, RecordedAt: now},
		{ID: "oc-2", ReleaseID: "rel-2", Repository: "owner/repo-a", OutcomeType: OutcomeTypeRollback, RecordedAt: now},
		{ID: "oc-3", ReleaseID: "rel-3", Repository: "owner/repo-b", OutcomeType: OutcomeTypeIncident, RecordedAt: now},
	}
	for _, o := range outcomes {
		if err := store.RecordOutcome(ctx, o); err != nil {
			t.Fatalf("RecordOutcome() error = %v", err)
		}
	}

	// GetAllOutcomes for repo-a.
	results, err := store.GetAllOutcomes(ctx, "owner/repo-a")
	if err != nil {
		t.Fatalf("GetAllOutcomes() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 outcomes for repo-a, got %d", len(results))
	}

	// GetAllOutcomes for repo-b.
	results, err = store.GetAllOutcomes(ctx, "owner/repo-b")
	if err != nil {
		t.Fatalf("GetAllOutcomes() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 outcome for repo-b, got %d", len(results))
	}

	// GetAllOutcomes for unknown repo returns nil/empty slice.
	results, err = store.GetAllOutcomes(ctx, "unknown/repo")
	if err != nil {
		t.Fatalf("GetAllOutcomes() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 outcomes for unknown repo, got %d", len(results))
	}
}

// TestInsightsService_GetInsights_WithPatterns tests that high-confidence patterns
// generate pattern insights when the release has outcomes.
func TestInsightsService_GetInsights_WithPatterns(t *testing.T) {
	outcomeStore := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	ctx := context.Background()
	now := time.Now()

	// Seed data to create high-confidence patterns.
	for i := 0; i < 10; i++ {
		outcomeType := OutcomeTypeIncident
		if i >= 8 {
			outcomeType = OutcomeTypeSuccessfulRelease
		}
		outcomeStore.RecordOutcome(ctx, &OutcomeRecord{
			ID:            idFor("oc", i),
			ReleaseID:     idFor("rel", i),
			Repository:    "owner/repo",
			OutcomeType:   outcomeType,
			FilesAffected: []string{"risky.go"},
			ChangeSize:    50,
			RecordedAt:    now.Add(-time.Duration(i) * time.Hour),
		})
	}

	// Record an outcome for the target release.
	outcomeStore.RecordOutcome(ctx, &OutcomeRecord{
		ID:          "oc-target",
		ReleaseID:   "target-release",
		Repository:  "owner/repo",
		OutcomeType: OutcomeTypeRollback,
		RecordedAt:  now,
	})

	detector := NewPatternDetector(outcomeStore, memStore, WithMinSampleSize(3), WithMinConfidence(0.5))
	svc := NewInsightsService(memStore, outcomeStore, detector, "owner/repo")

	insights, err := svc.GetInsights(ctx, "target-release")
	if err != nil {
		t.Fatalf("GetInsights() error = %v", err)
	}

	// Should have at least the historical outcome insight.
	if len(insights) == 0 {
		t.Error("expected at least 1 insight")
	}

	// Verify historical insight is present.
	var hasHistorical bool
	for _, ins := range insights {
		if ins.Category == InsightCategoryHistorical {
			hasHistorical = true
		}
	}
	if !hasHistorical {
		t.Error("expected a historical insight from negative outcome")
	}
}

// TestInsightsService_GetTrends_WithIncidents verifies incident count increments.
func TestInsightsService_GetTrends_WithIncidents(t *testing.T) {
	outcomeStore := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(outcomeStore, memStore)
	ctx := context.Background()
	now := time.Now()

	// Record incident outcomes.
	for i := 0; i < 3; i++ {
		outcomeStore.RecordOutcome(ctx, &OutcomeRecord{
			ID:          idFor("oc", i),
			ReleaseID:   idFor("rel", i),
			Repository:  "owner/repo",
			OutcomeType: OutcomeTypeIncident,
			RecordedAt:  now.Add(-time.Duration(i) * 24 * time.Hour),
		})
	}

	svc := NewInsightsService(memStore, outcomeStore, detector, "owner/repo")
	trends, err := svc.GetTrends(ctx, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("GetTrends() error = %v", err)
	}

	if trends.TotalReleases != 3 {
		t.Errorf("expected 3 total, got %d", trends.TotalReleases)
	}
	if trends.OutcomeDistribution[OutcomeTypeIncident] != 3 {
		t.Errorf("expected 3 incidents in distribution, got %d", trends.OutcomeDistribution[OutcomeTypeIncident])
	}
}
