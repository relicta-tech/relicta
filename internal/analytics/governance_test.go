package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
)

// makeRecord is a test helper that creates a ReleaseRecord with sensible defaults.
func makeRecord(id string, outcome memory.ReleaseOutcome, riskScore float64, releasedAt time.Time, tags []string) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         id,
		Repository: "test/repo",
		Version:    "1.0." + id,
		Actor:      cgp.Actor{ID: "actor-1", Kind: cgp.ActorKindHuman},
		RiskScore:  riskScore,
		Outcome:    outcome,
		ReleasedAt: releasedAt,
		Tags:       tags,
	}
}

// =============================================================================
// ComputeOutcomeMetrics Tests
// =============================================================================

func TestComputeOutcomeMetrics_KnownData(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, nil),
		makeRecord("2", memory.OutcomeSuccess, 0.3, base.Add(time.Hour), nil),
		makeRecord("3", memory.OutcomeSuccess, 0.1, base.Add(2*time.Hour), nil),
		makeRecord("4", memory.OutcomeSuccess, 0.15, base.Add(3*time.Hour), nil),
		makeRecord("5", memory.OutcomeSuccess, 0.25, base.Add(4*time.Hour), nil),
		makeRecord("6", memory.OutcomeRollback, 0.8, base.Add(5*time.Hour), nil),
		makeRecord("7", memory.OutcomeRollback, 0.7, base.Add(6*time.Hour), nil),
		makeRecord("8", memory.OutcomeFailed, 0.9, base.Add(7*time.Hour), []string{"incident"}),
	}

	metrics := ComputeOutcomeMetrics(records, Period{})

	assert.Equal(t, 8, metrics.TotalReleases)
	assert.InDelta(t, 5.0/8.0, metrics.SuccessRate, 0.001)
	assert.InDelta(t, 2.0/8.0, metrics.RollbackRate, 0.001)
	assert.InDelta(t, 1.0/8.0, metrics.IncidentRate, 0.001)
	assert.InDelta(t, 0.0, metrics.HotfixRate, 0.001)
}

func TestComputeOutcomeMetrics_WithHotfix(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, nil),
		makeRecord("2", memory.OutcomeSuccess, 0.3, base.Add(time.Hour), []string{"hotfix"}),
		makeRecord("3", memory.OutcomeSuccess, 0.1, base.Add(2*time.Hour), []string{"hotfix"}),
		makeRecord("4", memory.OutcomeRollback, 0.8, base.Add(3*time.Hour), nil),
	}

	metrics := ComputeOutcomeMetrics(records, Period{})

	assert.Equal(t, 4, metrics.TotalReleases)
	assert.InDelta(t, 0.5, metrics.HotfixRate, 0.001)
}

func TestComputeOutcomeMetrics_MonthlySeries(t *testing.T) {
	jan := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, jan, nil),
		makeRecord("2", memory.OutcomeRollback, 0.8, jan.Add(time.Hour), nil),
		makeRecord("3", memory.OutcomeSuccess, 0.1, feb, nil),
		makeRecord("4", memory.OutcomeSuccess, 0.3, mar, nil),
		makeRecord("5", memory.OutcomeFailed, 0.7, mar.Add(time.Hour), []string{"incident"}),
	}

	metrics := ComputeOutcomeMetrics(records, Period{})

	require.Len(t, metrics.ByMonth, 3)
	assert.Equal(t, "2026-01", metrics.ByMonth[0].Month)
	assert.Equal(t, 2, metrics.ByMonth[0].Total)
	assert.Equal(t, 1, metrics.ByMonth[0].Success)
	assert.Equal(t, 1, metrics.ByMonth[0].Rollback)

	assert.Equal(t, "2026-02", metrics.ByMonth[1].Month)
	assert.Equal(t, 1, metrics.ByMonth[1].Total)

	assert.Equal(t, "2026-03", metrics.ByMonth[2].Month)
	assert.Equal(t, 2, metrics.ByMonth[2].Total)
	assert.Equal(t, 1, metrics.ByMonth[2].Incident)
}

func TestComputeOutcomeMetrics_PeriodFilter(t *testing.T) {
	jan := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, jan, nil),
		makeRecord("2", memory.OutcomeSuccess, 0.3, mar, nil),
	}

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	metrics := ComputeOutcomeMetrics(records, Period{From: &from})

	assert.Equal(t, 1, metrics.TotalReleases)
	assert.InDelta(t, 1.0, metrics.SuccessRate, 0.001)
}

func TestComputeOutcomeMetrics_Empty(t *testing.T) {
	metrics := ComputeOutcomeMetrics(nil, Period{})

	assert.Equal(t, 0, metrics.TotalReleases)
	assert.InDelta(t, 0.0, metrics.SuccessRate, 0.001)
	assert.Nil(t, metrics.ByMonth)
}

// =============================================================================
// ComputeRiskFactorAttribution Tests
// =============================================================================

func TestComputeRiskFactorAttribution_DominantFactor(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, []string{"api_change"}),
		makeRecord("2", memory.OutcomeSuccess, 0.3, base.Add(time.Hour), []string{"api_change"}),
		makeRecord("3", memory.OutcomeRollback, 0.8, base.Add(2*time.Hour), []string{"api_change", "security"}),
		makeRecord("4", memory.OutcomeSuccess, 0.1, base.Add(3*time.Hour), []string{"security"}),
		makeRecord("5", memory.OutcomeFailed, 0.9, base.Add(4*time.Hour), []string{"api_change"}),
	}

	factors := ComputeRiskFactorAttribution(records)

	require.NotEmpty(t, factors)

	// api_change should be the most triggered factor (4 times)
	assert.Equal(t, "api_change", factors[0].Factor)
	assert.Equal(t, 4, factors[0].TriggerCount)

	// security should appear second (2 times)
	assert.Equal(t, "security", factors[1].Factor)
	assert.Equal(t, 2, factors[1].TriggerCount)
}

func TestComputeRiskFactorAttribution_Empty(t *testing.T) {
	factors := ComputeRiskFactorAttribution(nil)
	assert.Nil(t, factors)
}

func TestComputeRiskFactorAttribution_NoTags(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, nil),
		makeRecord("2", memory.OutcomeRollback, 0.8, base.Add(time.Hour), nil),
	}

	factors := ComputeRiskFactorAttribution(records)
	assert.Empty(t, factors)
}

func TestComputeRiskFactorAttribution_CorrelationSign(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	// "dangerous" tag only appears on failures.
	// "safe" tag only appears on successes.
	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, []string{"safe"}),
		makeRecord("2", memory.OutcomeSuccess, 0.3, base.Add(time.Hour), []string{"safe"}),
		makeRecord("3", memory.OutcomeRollback, 0.8, base.Add(2*time.Hour), []string{"dangerous"}),
		makeRecord("4", memory.OutcomeFailed, 0.9, base.Add(3*time.Hour), []string{"dangerous"}),
	}

	factors := ComputeRiskFactorAttribution(records)
	require.Len(t, factors, 2)

	factorMap := make(map[string]RiskFactorAttribution)
	for _, f := range factors {
		factorMap[f.Factor] = f
	}

	// "dangerous" should have positive correlation with negative outcomes.
	assert.Greater(t, factorMap["dangerous"].Correlation, 0.0)

	// "safe" should have negative correlation with negative outcomes.
	assert.Less(t, factorMap["safe"].Correlation, 0.0)
}

// =============================================================================
// ComputeCalibrationMetrics Tests
// =============================================================================

func TestComputeCalibrationMetrics_AccuracyCalculation(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	// 4 records: high risk + negative = correct, low risk + positive = correct
	records := []*memory.ReleaseRecord{
		// Correct predictions
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, nil),                   // low risk, success -> correct
		makeRecord("2", memory.OutcomeSuccess, 0.3, base.Add(time.Hour), nil),    // low risk, success -> correct
		makeRecord("3", memory.OutcomeRollback, 0.8, base.Add(2*time.Hour), nil), // high risk, rollback -> correct
		// Incorrect prediction
		makeRecord("4", memory.OutcomeSuccess, 0.7, base.Add(3*time.Hour), nil), // high risk, success -> incorrect
	}

	weights := risk.DefaultWeights()
	metrics := ComputeCalibrationMetrics(records, weights)

	assert.Equal(t, 4, metrics.SampleSize)
	assert.InDelta(t, 0.75, metrics.Accuracy, 0.001) // 3 out of 4 correct
	assert.NotNil(t, metrics.CalibratedAt)
	assert.NotEmpty(t, metrics.WeightChanges)
}

func TestComputeCalibrationMetrics_HighLowRiskSplit(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		// Low risk records: 2 correct, 1 incorrect
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, nil),                   // correct
		makeRecord("2", memory.OutcomeSuccess, 0.3, base.Add(time.Hour), nil),    // correct
		makeRecord("3", memory.OutcomeRollback, 0.4, base.Add(2*time.Hour), nil), // incorrect (low risk but negative)
		// High risk records: 1 correct, 1 incorrect
		makeRecord("4", memory.OutcomeRollback, 0.8, base.Add(3*time.Hour), nil), // correct
		makeRecord("5", memory.OutcomeSuccess, 0.6, base.Add(4*time.Hour), nil),  // incorrect (high risk but positive)
	}

	metrics := ComputeCalibrationMetrics(records, risk.DefaultWeights())

	// Low risk accuracy: 2/3 correct
	assert.InDelta(t, 2.0/3.0, metrics.LowRiskAccuracy, 0.001)
	// High risk accuracy: 1/2 correct
	assert.InDelta(t, 0.5, metrics.HighRiskAccuracy, 0.001)
}

func TestComputeCalibrationMetrics_WeightChanges(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.2, base, nil),
	}

	// Use non-default weights to verify delta computation.
	customWeights := risk.WeightConfig{
		APIChanges:       0.30,
		DependencyImpact: 0.20,
		BlastRadius:      0.15,
		CodeComplexity:   0.10,
		TestCoverage:     0.10,
		ActorTrust:       0.05,
		HistoricalRisk:   0.05,
		SecurityImpact:   0.05,
	}

	metrics := ComputeCalibrationMetrics(records, customWeights)

	apiChange, ok := metrics.WeightChanges["api_changes"]
	require.True(t, ok)
	assert.InDelta(t, 0.25, apiChange.Default, 0.001)    // DefaultWeights().APIChanges
	assert.InDelta(t, 0.30, apiChange.Calibrated, 0.001) // customWeights.APIChanges
	assert.InDelta(t, 0.05, apiChange.Delta, 0.001)      // 0.30 - 0.25
}

func TestComputeCalibrationMetrics_Empty(t *testing.T) {
	metrics := ComputeCalibrationMetrics(nil, risk.DefaultWeights())

	assert.Equal(t, 0, metrics.SampleSize)
	assert.InDelta(t, 0.0, metrics.Accuracy, 0.001)
	assert.NotNil(t, metrics.WeightChanges)
}

func TestComputeCalibrationMetrics_AllCorrect(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	records := []*memory.ReleaseRecord{
		makeRecord("1", memory.OutcomeSuccess, 0.1, base, nil),
		makeRecord("2", memory.OutcomeSuccess, 0.2, base.Add(time.Hour), nil),
		makeRecord("3", memory.OutcomeRollback, 0.9, base.Add(2*time.Hour), nil),
	}

	metrics := ComputeCalibrationMetrics(records, risk.DefaultWeights())

	assert.InDelta(t, 1.0, metrics.Accuracy, 0.001)
}
