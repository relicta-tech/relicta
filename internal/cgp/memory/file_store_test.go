package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestNewFileStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewFileStore() returned nil")
	}
	if store.basePath != tmpDir {
		t.Errorf("basePath = %v, want %v", store.basePath, tmpDir)
	}
	if !store.loaded {
		t.Error("store should be loaded after creation")
	}
}

func TestNewFileStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "memory", "data")

	store, err := NewFileStore(subDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewFileStore() returned nil")
	}

	// Verify directory was created
	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Path should be a directory")
	}
}

func TestFileStore_RecordAndRetrieve(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	// Record a release
	record := &ReleaseRecord{
		ID:              "release-1",
		Repository:      "owner/repo",
		Version:         "v1.0.0",
		Actor:           actor,
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

	err = store.RecordRelease(ctx, record)
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

	// Verify data file was created
	dataPath := filepath.Join(tmpDir, "memory.json")
	if _, err := os.Stat(dataPath); err != nil {
		t.Errorf("Data file not created: %v", err)
	}
}

func TestFileStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	// Create store and add data
	store1, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	record := &ReleaseRecord{
		ID:         "release-1",
		Repository: "owner/repo",
		Version:    "v1.0.0",
		Actor:      actor,
		RiskScore:  0.4,
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now(),
	}

	err = store1.RecordRelease(ctx, record)
	if err != nil {
		t.Fatalf("RecordRelease() error = %v", err)
	}

	// Create new store from same directory - should load existing data
	store2, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Verify data was persisted and loaded
	releases, err := store2.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory() error = %v", err)
	}
	if len(releases) != 1 {
		t.Errorf("After reload: got %d releases, want 1", len(releases))
	}
	if releases[0].ID != "release-1" {
		t.Errorf("After reload: Release ID = %v, want release-1", releases[0].ID)
	}

	// Verify actor metrics were persisted
	metrics, err := store2.GetActorMetrics(ctx, actor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics() error = %v", err)
	}
	if metrics.TotalReleases != 1 {
		t.Errorf("After reload: TotalReleases = %d, want 1", metrics.TotalReleases)
	}
}

func TestFileStore_RecordIncident(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()

	incident := &IncidentRecord{
		ID:           "incident-1",
		Repository:   "owner/repo",
		ReleaseID:    "release-1",
		Version:      "v1.0.0",
		Type:         IncidentRollback,
		Severity:     cgp.SeverityHigh,
		Description:  "Service outage",
		DetectedAt:   time.Now(),
		TimeToDetect: 10 * time.Minute,
	}

	err = store.RecordIncident(ctx, incident)
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
}

func TestFileStore_ActorMetricsPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	actor := cgp.NewAgentActor("cursor", "Cursor", "gpt-4")

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Record multiple releases
	for i := 0; i < 5; i++ {
		outcome := OutcomeSuccess
		if i == 2 {
			outcome = OutcomeFailed
		}
		record := &ReleaseRecord{
			ID:              fmt.Sprintf("release-%d", i),
			Repository:      "owner/repo",
			Actor:           actor,
			RiskScore:       float64(i) * 0.15,
			Outcome:         outcome,
			BreakingChanges: i % 2,
			ReleasedAt:      time.Now(),
		}
		if err := store.RecordRelease(ctx, record); err != nil {
			t.Fatalf("RecordRelease() error = %v", err)
		}
	}

	// Get metrics
	metrics, err := store.GetActorMetrics(ctx, actor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics() error = %v", err)
	}

	if metrics.TotalReleases != 5 {
		t.Errorf("TotalReleases = %d, want 5", metrics.TotalReleases)
	}
	if metrics.SuccessfulReleases != 4 {
		t.Errorf("SuccessfulReleases = %d, want 4", metrics.SuccessfulReleases)
	}
	if metrics.FailedReleases != 1 {
		t.Errorf("FailedReleases = %d, want 1", metrics.FailedReleases)
	}
	if metrics.BreakingChangeReleases != 2 {
		t.Errorf("BreakingChangeReleases = %d, want 2", metrics.BreakingChangeReleases)
	}

	// Reload and verify persistence
	store2, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	metrics2, err := store2.GetActorMetrics(ctx, actor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics() after reload error = %v", err)
	}

	if metrics2.TotalReleases != 5 {
		t.Errorf("After reload: TotalReleases = %d, want 5", metrics2.TotalReleases)
	}
}

func TestFileStore_GetRiskPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")
	now := time.Now()

	// Create releases with increasing risk
	for i := 0; i < 6; i++ {
		record := &ReleaseRecord{
			ID:         fmt.Sprintf("release-%d", i),
			Repository: "owner/repo",
			Actor:      actor,
			RiskScore:  0.2 + float64(i)*0.1,
			Outcome:    OutcomeSuccess,
			ReleasedAt: now.Add(time.Duration(-6+i) * 24 * time.Hour),
			Tags:       []string{"api_change"},
		}
		if err := store.RecordRelease(ctx, record); err != nil {
			t.Fatalf("RecordRelease() error = %v", err)
		}
	}

	patterns, err := store.GetRiskPatterns(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("GetRiskPatterns() error = %v", err)
	}

	if patterns.TotalReleases != 6 {
		t.Errorf("TotalReleases = %d, want 6", patterns.TotalReleases)
	}
	if patterns.RiskTrend != TrendIncreasing {
		t.Errorf("RiskTrend = %v, want %v", patterns.RiskTrend, TrendIncreasing)
	}
}

func TestFileStore_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	actor1 := cgp.NewHumanActor("john@example.com", "John")
	actor2 := cgp.NewAgentActor("cursor", "Cursor", "gpt-4")

	// Add releases from different actors to different repos
	store.RecordRelease(ctx, &ReleaseRecord{
		ID: "r1", Repository: "owner/repo1", Actor: actor1,
		Outcome: OutcomeSuccess, ReleasedAt: time.Now(),
	})
	store.RecordRelease(ctx, &ReleaseRecord{
		ID: "r2", Repository: "owner/repo1", Actor: actor1,
		Outcome: OutcomeSuccess, ReleasedAt: time.Now(),
	})
	store.RecordRelease(ctx, &ReleaseRecord{
		ID: "r3", Repository: "owner/repo2", Actor: actor2,
		Outcome: OutcomeSuccess, ReleasedAt: time.Now(),
	})

	// Add an incident
	store.RecordIncident(ctx, &IncidentRecord{
		ID: "i1", Repository: "owner/repo1", Type: IncidentBugIntro,
		DetectedAt: time.Now(),
	})

	stats := store.Stats()

	if stats.Repositories != 2 {
		t.Errorf("Repositories = %d, want 2", stats.Repositories)
	}
	if stats.TotalReleases != 3 {
		t.Errorf("TotalReleases = %d, want 3", stats.TotalReleases)
	}
	if stats.TotalIncidents != 1 {
		t.Errorf("TotalIncidents = %d, want 1", stats.TotalIncidents)
	}
	if stats.TrackedActors != 2 {
		t.Errorf("TrackedActors = %d, want 2", stats.TrackedActors)
	}
}

func TestFileStore_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Flush should not error on empty store
	err = store.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Verify data file was created
	dataPath := filepath.Join(tmpDir, "memory.json")
	if _, err := os.Stat(dataPath); err != nil {
		t.Errorf("Data file not created after Flush: %v", err)
	}
}

func TestFileStore_UpdateActorMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()
	actor := cgp.NewHumanActor("john@example.com", "John")

	// First record a success
	record := &ReleaseRecord{
		ID:         "release-1",
		Repository: "owner/repo",
		Actor:      actor,
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now(),
	}
	store.RecordRelease(ctx, record)

	// Later update to rollback
	err = store.UpdateActorMetrics(ctx, actor.ID, OutcomeRollback)
	if err != nil {
		t.Fatalf("UpdateActorMetrics() error = %v", err)
	}

	metrics, _ := store.GetActorMetrics(ctx, actor.ID)
	if metrics.RollbackCount != 1 {
		t.Errorf("RollbackCount = %d, want 1", metrics.RollbackCount)
	}

	// Verify persistence
	store2, _ := NewFileStore(tmpDir)
	metrics2, _ := store2.GetActorMetrics(ctx, actor.ID)
	if metrics2.RollbackCount != 1 {
		t.Errorf("After reload: RollbackCount = %d, want 1", metrics2.RollbackCount)
	}
}

func TestFileStore_EmptyHistory(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	ctx := context.Background()

	// Empty release history should return empty slice
	releases, err := store.GetReleaseHistory(ctx, "unknown/repo", 10)
	if err != nil {
		t.Errorf("GetReleaseHistory() error = %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("GetReleaseHistory() returned %d, want 0", len(releases))
	}

	// Empty incident history should return empty slice
	incidents, err := store.GetIncidentHistory(ctx, "unknown/repo", 10)
	if err != nil {
		t.Errorf("GetIncidentHistory() error = %v", err)
	}
	if len(incidents) != 0 {
		t.Errorf("GetIncidentHistory() returned %d, want 0", len(incidents))
	}
}

func TestFileStore_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewFileStore(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name    string
		fn      func() error
		wantErr bool
	}{
		{
			name:    "nil release record",
			fn:      func() error { return store.RecordRelease(ctx, nil) },
			wantErr: true,
		},
		{
			name:    "empty release ID",
			fn:      func() error { return store.RecordRelease(ctx, &ReleaseRecord{Repository: "r"}) },
			wantErr: true,
		},
		{
			name:    "empty repository",
			fn:      func() error { return store.RecordRelease(ctx, &ReleaseRecord{ID: "1"}) },
			wantErr: true,
		},
		{
			name:    "nil incident record",
			fn:      func() error { return store.RecordIncident(ctx, nil) },
			wantErr: true,
		},
		{
			name:    "empty incident ID",
			fn:      func() error { return store.RecordIncident(ctx, &IncidentRecord{Repository: "r"}) },
			wantErr: true,
		},
		{
			name:    "empty incident repository",
			fn:      func() error { return store.RecordIncident(ctx, &IncidentRecord{ID: "1"}) },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
