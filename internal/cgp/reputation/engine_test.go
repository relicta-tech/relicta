package reputation

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

// helper to create a release record for a given actor.
func makeRelease(id, actorID string, outcome memory.ReleaseOutcome, riskScore float64, releasedAt time.Time) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         id,
		Repository: "org/repo",
		Version:    "1.0.0",
		Actor:      cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman},
		RiskScore:  riskScore,
		Outcome:    outcome,
		ReleasedAt: releasedAt,
	}
}

// helper to create an incident record.
func makeIncident(id, releaseID, actorID string, timeToResolve time.Duration) *memory.IncidentRecord {
	return &memory.IncidentRecord{
		ID:            id,
		Repository:    "org/repo",
		ReleaseID:     releaseID,
		ActorID:       actorID,
		Type:          memory.IncidentBugIntro,
		Severity:      cgp.SeverityMedium,
		DetectedAt:    time.Now(),
		TimeToResolve: timeToResolve,
	}
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	store, err := NewFileStore(filepath.Join(dir, "reputation"))
	if err != nil {
		t.Fatalf("creating file store: %v", err)
	}
	engine, err := NewEngine(store)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}
	return engine
}

func TestComputeScore_EmptyRecords(t *testing.T) {
	engine := newTestEngine(t)

	score := engine.ComputeScore(nil, nil, "actor-1")

	if score.Overall != 0.5 {
		t.Errorf("Overall = %v, want 0.5 for empty records", score.Overall)
	}
	if score.SampleSize != 0 {
		t.Errorf("SampleSize = %d, want 0", score.SampleSize)
	}
	if score.Trend != TrendStable {
		t.Errorf("Trend = %v, want %v", score.Trend, TrendStable)
	}
	if score.Level() != "probation" {
		t.Errorf("Level = %v, want probation for 0.5 score", score.Level())
	}
}

func TestComputeScore_PerfectActor(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now()

	records := make([]*memory.ReleaseRecord, 20)
	for i := range records {
		records[i] = makeRelease(
			fmt.Sprintf("r-%d", i),
			"actor-perfect",
			memory.OutcomeSuccess,
			0.2, // low risk, always succeeds
			now.Add(-time.Duration(i)*24*time.Hour),
		)
	}

	score := engine.ComputeScore(records, nil, "actor-perfect")

	if score.Overall < 0.85 {
		t.Errorf("Overall = %v, want >= 0.85 for perfect actor", score.Overall)
	}
	if score.ReleaseSuccess < 0.95 {
		t.Errorf("ReleaseSuccess = %v, want >= 0.95", score.ReleaseSuccess)
	}
	if score.IncidentRate < 0.95 {
		t.Errorf("IncidentRate = %v, want >= 0.95", score.IncidentRate)
	}
	if score.Level() != "trusted" {
		t.Errorf("Level = %v, want trusted", score.Level())
	}
	if score.SampleSize != 20 {
		t.Errorf("SampleSize = %d, want 20", score.SampleSize)
	}
}

func TestComputeScore_PoorActor(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now()

	var records []*memory.ReleaseRecord
	var incidents []*memory.IncidentRecord

	for i := 0; i < 20; i++ {
		outcome := memory.OutcomeFailed
		riskScore := 0.3 // low risk but still fails -> poor risk accuracy
		releaseID := fmt.Sprintf("r-%d", i)
		records = append(records, makeRelease(releaseID, "actor-poor", outcome, riskScore, now.Add(-time.Duration(i)*24*time.Hour)))

		// Half the releases have incidents.
		if i%2 == 0 {
			incidents = append(incidents, makeIncident(
				fmt.Sprintf("inc-%d", i),
				releaseID,
				"actor-poor",
				12*time.Hour,
			))
		}
	}

	score := engine.ComputeScore(records, incidents, "actor-poor")

	if score.Overall > 0.4 {
		t.Errorf("Overall = %v, want <= 0.4 for poor actor", score.Overall)
	}
	if score.ReleaseSuccess > 0.1 {
		t.Errorf("ReleaseSuccess = %v, want <= 0.1", score.ReleaseSuccess)
	}
	if score.Level() != "restricted" || score.Level() != "probation" {
		// Either restricted or probation is acceptable.
		if score.Overall >= ThresholdReliable {
			t.Errorf("Level = %v, want restricted or probation for poor actor", score.Level())
		}
	}
}

func TestComputeScore_RecencyDecay(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now()

	// All old failures (300 days ago), all recent successes.
	var records []*memory.ReleaseRecord

	// 10 old failures.
	for i := 0; i < 10; i++ {
		records = append(records, makeRelease(
			fmt.Sprintf("old-%d", i),
			"actor-decay",
			memory.OutcomeFailed,
			0.8,
			now.Add(-300*24*time.Hour-time.Duration(i)*24*time.Hour),
		))
	}

	// 10 recent successes.
	for i := 0; i < 10; i++ {
		records = append(records, makeRelease(
			fmt.Sprintf("new-%d", i),
			"actor-decay",
			memory.OutcomeSuccess,
			0.2,
			now.Add(-time.Duration(i)*24*time.Hour),
		))
	}

	score := engine.ComputeScore(records, nil, "actor-decay")

	// Recent successes should dominate over old failures.
	if score.ReleaseSuccess < 0.8 {
		t.Errorf("ReleaseSuccess = %v, want >= 0.8 (old failures should decay)", score.ReleaseSuccess)
	}
}

func TestComputeScore_TrendDetection(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(now time.Time) []*memory.ReleaseRecord
		expected Trend
	}{
		{
			name: "improving: recent successes after prior failures",
			setup: func(now time.Time) []*memory.ReleaseRecord {
				var records []*memory.ReleaseRecord
				// Previous 30 days: 2/10 success.
				for i := 0; i < 10; i++ {
					outcome := memory.OutcomeFailed
					if i < 2 {
						outcome = memory.OutcomeSuccess
					}
					records = append(records, makeRelease(
						fmt.Sprintf("prev-%d", i), "actor-trend", outcome, 0.5,
						now.Add(-45*24*time.Hour+time.Duration(i)*24*time.Hour),
					))
				}
				// Recent 30 days: 9/10 success.
				for i := 0; i < 10; i++ {
					outcome := memory.OutcomeSuccess
					if i == 0 {
						outcome = memory.OutcomeFailed
					}
					records = append(records, makeRelease(
						fmt.Sprintf("recent-%d", i), "actor-trend", outcome, 0.5,
						now.Add(-25*24*time.Hour+time.Duration(i)*24*time.Hour),
					))
				}
				return records
			},
			expected: TrendImproving,
		},
		{
			name: "declining: recent failures after prior successes",
			setup: func(now time.Time) []*memory.ReleaseRecord {
				var records []*memory.ReleaseRecord
				// Previous 30 days: 9/10 success.
				for i := 0; i < 10; i++ {
					outcome := memory.OutcomeSuccess
					if i == 0 {
						outcome = memory.OutcomeFailed
					}
					records = append(records, makeRelease(
						fmt.Sprintf("prev-%d", i), "actor-trend", outcome, 0.5,
						now.Add(-45*24*time.Hour+time.Duration(i)*24*time.Hour),
					))
				}
				// Recent 30 days: 2/10 success.
				for i := 0; i < 10; i++ {
					outcome := memory.OutcomeFailed
					if i < 2 {
						outcome = memory.OutcomeSuccess
					}
					records = append(records, makeRelease(
						fmt.Sprintf("recent-%d", i), "actor-trend", outcome, 0.5,
						now.Add(-25*24*time.Hour+time.Duration(i)*24*time.Hour),
					))
				}
				return records
			},
			expected: TrendDeclining,
		},
		{
			name: "stable: consistent performance",
			setup: func(now time.Time) []*memory.ReleaseRecord {
				var records []*memory.ReleaseRecord
				// Both windows: 8/10 success.
				for i := 0; i < 10; i++ {
					outcome := memory.OutcomeSuccess
					if i < 2 {
						outcome = memory.OutcomeFailed
					}
					records = append(records, makeRelease(
						fmt.Sprintf("prev-%d", i), "actor-trend", outcome, 0.5,
						now.Add(-45*24*time.Hour+time.Duration(i)*24*time.Hour),
					))
				}
				for i := 0; i < 10; i++ {
					outcome := memory.OutcomeSuccess
					if i < 2 {
						outcome = memory.OutcomeFailed
					}
					records = append(records, makeRelease(
						fmt.Sprintf("recent-%d", i), "actor-trend", outcome, 0.5,
						now.Add(-25*24*time.Hour+time.Duration(i)*24*time.Hour),
					))
				}
				return records
			},
			expected: TrendStable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newTestEngine(t)
			now := time.Now()
			records := tt.setup(now)

			score := engine.ComputeScore(records, nil, "actor-trend")

			if score.Trend != tt.expected {
				t.Errorf("Trend = %v, want %v", score.Trend, tt.expected)
			}
		})
	}
}

func TestComputeScore_RecoverySpeed(t *testing.T) {
	tests := []struct {
		name          string
		recoveryTime  time.Duration
		wantMin       float64
		wantMax       float64
	}{
		{
			name:         "fast recovery under 1h",
			recoveryTime: 30 * time.Minute,
			wantMin:      0.99,
			wantMax:      1.0,
		},
		{
			name:         "slow recovery over 24h",
			recoveryTime: 48 * time.Hour,
			wantMin:      0.0,
			wantMax:      0.01,
		},
		{
			name:         "medium recovery 12h",
			recoveryTime: 12 * time.Hour,
			wantMin:      0.4,
			wantMax:      0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newTestEngine(t)
			now := time.Now()

			records := []*memory.ReleaseRecord{
				makeRelease("r-1", "actor-recovery", memory.OutcomeSuccess, 0.3, now),
			}
			incidents := []*memory.IncidentRecord{
				makeIncident("inc-1", "r-1", "actor-recovery", tt.recoveryTime),
			}

			score := engine.ComputeScore(records, incidents, "actor-recovery")

			if score.RecoverySpeed < tt.wantMin || score.RecoverySpeed > tt.wantMax {
				t.Errorf("RecoverySpeed = %v, want [%v, %v]", score.RecoverySpeed, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestComputeScore_Consistency(t *testing.T) {
	tests := []struct {
		name       string
		riskScores []float64
		wantMin    float64
	}{
		{
			name:       "consistent low risk",
			riskScores: []float64{0.2, 0.2, 0.2, 0.2, 0.2},
			wantMin:    0.95,
		},
		{
			name:       "highly variable risk",
			riskScores: []float64{0.0, 1.0, 0.0, 1.0, 0.0},
			wantMin:    0.0, // stddev is high
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newTestEngine(t)
			now := time.Now()

			var records []*memory.ReleaseRecord
			for i, rs := range tt.riskScores {
				records = append(records, makeRelease(
					fmt.Sprintf("r-%d", i),
					"actor-consistency",
					memory.OutcomeSuccess,
					rs,
					now.Add(-time.Duration(i)*24*time.Hour),
				))
			}

			score := engine.ComputeScore(records, nil, "actor-consistency")

			if score.Consistency < tt.wantMin {
				t.Errorf("Consistency = %v, want >= %v", score.Consistency, tt.wantMin)
			}
		})
	}
}

func TestScore_Level(t *testing.T) {
	tests := []struct {
		overall  float64
		expected string
	}{
		{0.9, "trusted"},
		{0.8, "trusted"},
		{0.79, "reliable"},
		{0.6, "reliable"},
		{0.59, "probation"},
		{0.4, "probation"},
		{0.39, "restricted"},
		{0.1, "restricted"},
		{0.0, "restricted"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("overall_%.2f", tt.overall), func(t *testing.T) {
			score := Score{Overall: tt.overall}
			if got := score.Level(); got != tt.expected {
				t.Errorf("Level() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestComputeScore_NoMatchingActor(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now()

	// Records exist but for a different actor.
	records := []*memory.ReleaseRecord{
		makeRelease("r-1", "other-actor", memory.OutcomeSuccess, 0.2, now),
	}

	score := engine.ComputeScore(records, nil, "non-existent")

	if score.Overall != 0.5 {
		t.Errorf("Overall = %v, want 0.5 for non-matching actor", score.Overall)
	}
	if score.SampleSize != 0 {
		t.Errorf("SampleSize = %d, want 0", score.SampleSize)
	}
}

func TestComputeScore_OverallClamped(t *testing.T) {
	engine := newTestEngine(t)
	now := time.Now()

	records := make([]*memory.ReleaseRecord, 50)
	for i := range records {
		records[i] = makeRelease(
			fmt.Sprintf("r-%d", i),
			"actor-clamp",
			memory.OutcomeSuccess,
			0.1,
			now.Add(-time.Duration(i)*time.Hour),
		)
	}

	score := engine.ComputeScore(records, nil, "actor-clamp")

	if score.Overall < 0.0 || score.Overall > 1.0 {
		t.Errorf("Overall = %v, want in [0.0, 1.0]", score.Overall)
	}
}

func TestUpdateScore_PersistsToStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(filepath.Join(dir, "reputation"))
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	engine, err := NewEngine(store)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}

	now := time.Now()
	records := []*memory.ReleaseRecord{
		makeRelease("r-1", "actor-persist", memory.OutcomeSuccess, 0.2, now),
	}

	ctx := context.Background()
	saved, err := engine.UpdateScore(ctx, records, nil, "actor-persist")
	if err != nil {
		t.Fatalf("UpdateScore: %v", err)
	}

	retrieved, err := engine.GetScore(ctx, "actor-persist")
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}

	if math.Abs(saved.Overall-retrieved.Overall) > 0.001 {
		t.Errorf("retrieved Overall = %v, want %v", retrieved.Overall, saved.Overall)
	}
}

func TestFileStore_GetHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(filepath.Join(dir, "reputation"))
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	ctx := context.Background()

	// Save multiple scores.
	for i := 0; i < 5; i++ {
		score := &Score{
			Overall:     float64(i) * 0.2,
			LastUpdated: time.Now(),
			SampleSize:  i + 1,
		}
		if err := store.SaveScore(ctx, "actor-history", score); err != nil {
			t.Fatalf("SaveScore %d: %v", i, err)
		}
	}

	history, err := store.GetHistory(ctx, "actor-history", 3)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("len(history) = %d, want 3", len(history))
	}

	// Most recent first.
	if math.Abs(history[0].Overall-0.8) > 0.001 {
		t.Errorf("history[0].Overall = %v, want ~0.8", history[0].Overall)
	}
	if math.Abs(history[1].Overall-0.6) > 0.001 {
		t.Errorf("history[1].Overall = %v, want ~0.6", history[1].Overall)
	}
}

func TestFileStore_PersistenceAcrossInstances(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reputation")

	// First instance: save a score.
	store1, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating store1: %v", err)
	}

	ctx := context.Background()
	score := &Score{Overall: 0.75, LastUpdated: time.Now(), SampleSize: 10}
	if err := store1.SaveScore(ctx, "actor-persist", score); err != nil {
		t.Fatalf("SaveScore: %v", err)
	}

	// Second instance: read the score back.
	store2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating store2: %v", err)
	}

	retrieved, err := store2.GetScore(ctx, "actor-persist")
	if err != nil {
		t.Fatalf("GetScore: %v", err)
	}

	if math.Abs(retrieved.Overall-0.75) > 0.001 {
		t.Errorf("Overall = %v, want 0.75", retrieved.Overall)
	}
}

func TestFileStore_HistoryTrimming(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reputation")
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	ctx := context.Background()

	// Save more than maxHistoryPerActor entries.
	for i := 0; i < maxHistoryPerActor+20; i++ {
		score := &Score{
			Overall:     float64(i) / float64(maxHistoryPerActor+20),
			LastUpdated: time.Now(),
			SampleSize:  i,
		}
		if err := store.SaveScore(ctx, "actor-trim", score); err != nil {
			t.Fatalf("SaveScore %d: %v", i, err)
		}
	}

	history, err := store.GetHistory(ctx, "actor-trim", maxHistoryPerActor+50)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}

	if len(history) != maxHistoryPerActor {
		t.Errorf("len(history) = %d, want %d (should be trimmed)", len(history), maxHistoryPerActor)
	}
}

func TestNewEngine_NilStore(t *testing.T) {
	_, err := NewEngine(nil)
	if err == nil {
		t.Error("NewEngine(nil) should return an error")
	}
}

func TestNewFileStore_EmptyDir(t *testing.T) {
	_, err := NewFileStore("")
	if err == nil {
		t.Error("NewFileStore(\"\") should return an error")
	}
}

func TestFileStore_GetScore_NotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reputation")
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	_, err = store.GetScore(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetScore for nonexistent actor should return error")
	}
}

func TestFileStore_GetHistory_Empty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reputation")
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	history, err := store.GetHistory(context.Background(), "nonexistent", 10)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("len(history) = %d, want 0", len(history))
	}
}

func TestFileStore_SaveScore_NilScore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reputation")
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}

	err = store.SaveScore(context.Background(), "actor", nil)
	if err == nil {
		t.Error("SaveScore(nil) should return error")
	}
}

func TestFileStore_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested", "reputation")

	_, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("NewFileStore should create the directory")
	}
}

