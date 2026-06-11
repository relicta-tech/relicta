package correlation

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/observability/receiver"
)

func setupStore(t *testing.T, releases ...*memory.ReleaseRecord) memory.Store {
	t.Helper()
	store := memory.NewInMemoryStore()
	for _, r := range releases {
		if err := store.RecordRelease(context.Background(), r); err != nil {
			t.Fatalf("failed to seed release: %v", err)
		}
	}
	return store
}

func TestEngine_Correlate_TimeProximity(t *testing.T) {
	now := time.Now()
	store := setupStore(t, &memory.ReleaseRecord{
		ID:         "rel-1",
		Repository: "my-service",
		Version:    "1.0.0",
		ReleasedAt: now.Add(-30 * time.Minute),
		Metadata:   map[string]string{},
	})

	engine := NewEngine(store, DefaultEngineConfig())

	incident := receiver.Incident{
		Name:      "HighErrorRate",
		Severity:  "critical",
		StartedAt: now.Add(-10 * time.Minute),
		Labels:    map[string]string{"repository": "my-service"},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(correlations) == 0 {
		t.Fatal("expected at least 1 correlation")
	}
	if correlations[0].ReleaseID != "rel-1" {
		t.Errorf("expected rel-1, got %s", correlations[0].ReleaseID)
	}
	if correlations[0].Confidence <= 0 {
		t.Error("expected positive confidence score")
	}
	if len(correlations[0].Reasons) == 0 {
		t.Error("expected reasons to be populated")
	}
}

func TestEngine_Correlate_IncidentBeforeRelease(t *testing.T) {
	now := time.Now()
	store := setupStore(t, &memory.ReleaseRecord{
		ID:         "rel-2",
		Repository: "svc",
		Version:    "2.0.0",
		ReleasedAt: now,
		Metadata:   map[string]string{},
	})

	engine := NewEngine(store, DefaultEngineConfig())

	incident := receiver.Incident{
		Name:      "OldIncident",
		StartedAt: now.Add(-1 * time.Hour), // Before the release.
		Labels:    map[string]string{"repository": "svc"},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(correlations) != 0 {
		t.Error("expected no correlations for incident before release")
	}
}

func TestEngine_Correlate_OutsideWindow(t *testing.T) {
	now := time.Now()
	store := setupStore(t, &memory.ReleaseRecord{
		ID:         "rel-3",
		Repository: "svc",
		Version:    "3.0.0",
		ReleasedAt: now.Add(-5 * time.Hour), // 5 hours ago.
		Metadata:   map[string]string{},
	})

	cfg := DefaultEngineConfig()
	cfg.TimeWindow = 2 * time.Hour // 2-hour window.
	engine := NewEngine(store, cfg)

	incident := receiver.Incident{
		Name:      "LateIncident",
		StartedAt: now,
		Labels:    map[string]string{"repository": "svc"},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(correlations) != 0 {
		t.Error("expected no correlations outside time window")
	}
}

func TestEngine_Correlate_ServiceNameMatch(t *testing.T) {
	now := time.Now()
	store := setupStore(t, &memory.ReleaseRecord{
		ID:         "rel-4",
		Repository: "payment-service",
		Version:    "1.2.0",
		ReleasedAt: now.Add(-15 * time.Minute),
		Metadata:   map[string]string{},
	})

	engine := NewEngine(store, DefaultEngineConfig())

	incident := receiver.Incident{
		Name:        "PaymentTimeout",
		ServiceName: "payment-service",
		StartedAt:   now.Add(-5 * time.Minute),
		Labels:      map[string]string{"repository": "payment-service"},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(correlations) == 0 {
		t.Fatal("expected correlation for matching service name")
	}
	// Service name match should boost confidence.
	if correlations[0].Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5 with service match, got %f", correlations[0].Confidence)
	}
}

func TestEngine_Correlate_VersionLabelMatch(t *testing.T) {
	now := time.Now()
	store := setupStore(t, &memory.ReleaseRecord{
		ID:         "rel-5",
		Repository: "api",
		Version:    "4.0.0",
		ReleasedAt: now.Add(-20 * time.Minute),
		Metadata:   map[string]string{},
	})

	engine := NewEngine(store, DefaultEngineConfig())

	incident := receiver.Incident{
		Name:      "APIError",
		StartedAt: now.Add(-5 * time.Minute),
		Labels: map[string]string{
			"repository": "api",
			"version":    "4.0.0",
		},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(correlations) == 0 {
		t.Fatal("expected correlation with version label match")
	}
	if correlations[0].Confidence < 0.6 {
		t.Errorf("expected high confidence with version match, got %f", correlations[0].Confidence)
	}
}

func TestEngine_Correlate_EmptyName(t *testing.T) {
	store := memory.NewInMemoryStore()
	engine := NewEngine(store, DefaultEngineConfig())

	_, err := engine.Correlate(context.Background(), receiver.Incident{})
	if err == nil {
		t.Error("expected error for incident without name")
	}
}

func TestEngine_Correlate_NoReleases(t *testing.T) {
	store := memory.NewInMemoryStore()
	engine := NewEngine(store, DefaultEngineConfig())

	incident := receiver.Incident{
		Name:      "SomeAlert",
		StartedAt: time.Now(),
		Labels:    map[string]string{"repository": "nonexistent"},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	if len(correlations) != 0 {
		t.Error("expected no correlations with no releases")
	}
}

func TestEngine_CorrelateForRelease(t *testing.T) {
	store := memory.NewInMemoryStore()
	engine := NewEngine(store, DefaultEngineConfig())

	incidents := []receiver.Incident{
		{Name: "Alert1", StartedAt: time.Now()},
		{Name: "Alert2", StartedAt: time.Now()},
	}

	correlations, err := engine.CorrelateForRelease(context.Background(), "rel-manual", incidents)
	if err != nil {
		t.Fatalf("CorrelateForRelease() error = %v", err)
	}
	if len(correlations) != 2 {
		t.Errorf("expected 2 correlations, got %d", len(correlations))
	}
	for _, c := range correlations {
		if c.ReleaseID != "rel-manual" {
			t.Errorf("expected rel-manual, got %s", c.ReleaseID)
		}
	}
}

func TestEngine_Correlate_MinConfidenceFilter(t *testing.T) {
	now := time.Now()
	store := setupStore(t, &memory.ReleaseRecord{
		ID:         "rel-low",
		Repository: "other-repo",
		Version:    "0.1.0",
		ReleasedAt: now.Add(-110 * time.Minute), // Near end of window.
		Metadata:   map[string]string{},
	})

	cfg := DefaultEngineConfig()
	cfg.MinConfidence = 0.8 // Very high threshold.
	engine := NewEngine(store, cfg)

	incident := receiver.Incident{
		Name:      "VagueAlert",
		StartedAt: now,
		Labels:    map[string]string{"repository": "other-repo"},
	}

	correlations, err := engine.Correlate(context.Background(), incident)
	if err != nil {
		t.Fatalf("Correlate() error = %v", err)
	}
	// Low time proximity score should be filtered out by high min confidence.
	if len(correlations) != 0 {
		t.Errorf("expected 0 correlations with high min confidence, got %d (confidence: %f)",
			len(correlations), correlations[0].Confidence)
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"PaymentService", "payment", true},
		{"api-gateway", "API", true},
		{"foo", "bar", false},
		{"", "", true},
	}
	for _, tt := range tests {
		if got := containsIgnoreCase(tt.s, tt.substr); got != tt.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestEngine_InferRepository(t *testing.T) {
	engine := NewEngine(nil, DefaultEngineConfig())

	tests := []struct {
		name     string
		incident receiver.Incident
		want     string
	}{
		{
			name:     "from repository label",
			incident: receiver.Incident{Labels: map[string]string{"repository": "my-repo"}},
			want:     "my-repo",
		},
		{
			name:     "from repo label",
			incident: receiver.Incident{Labels: map[string]string{"repo": "my-repo"}},
			want:     "my-repo",
		},
		{
			name:     "from service name",
			incident: receiver.Incident{ServiceName: "my-service"},
			want:     "my-service",
		},
		{
			name:     "empty",
			incident: receiver.Incident{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.inferRepository(tt.incident)
			if got != tt.want {
				t.Errorf("inferRepository() = %q, want %q", got, tt.want)
			}
		})
	}
}
