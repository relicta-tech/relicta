package risk

import (
	"math"
	"testing"
	"time"
)

func TestCalibrate_InsufficientSamples_ReturnsDefaults(t *testing.T) {
	calibrator := NewCalibrator() // default min 50

	records := make([]CalibrationRecord, 10)
	for i := range records {
		records[i] = CalibrationRecord{
			RiskScore:    0.3,
			FactorScores: map[string]float64{"api_change": 0.5},
			Outcome:      "success",
			ReleasedAt:   time.Now(),
		}
	}

	result := calibrator.Calibrate(records)

	if result.SampleSize != 10 {
		t.Errorf("SampleSize = %d, want 10", result.SampleSize)
	}
	if result.Accuracy != 0 {
		t.Errorf("Accuracy = %v, want 0 for insufficient samples", result.Accuracy)
	}

	defaults := DefaultWeights()
	if result.Weights != defaults {
		t.Errorf("Weights should equal defaults when insufficient samples")
	}
}

func TestCalibrate_InsufficientSamples_EmptyRecords(t *testing.T) {
	calibrator := NewCalibrator()

	result := calibrator.Calibrate(nil)

	if result.SampleSize != 0 {
		t.Errorf("SampleSize = %d, want 0", result.SampleSize)
	}
	if result.Accuracy != 0 {
		t.Errorf("Accuracy = %v, want 0", result.Accuracy)
	}
	if result.Weights != DefaultWeights() {
		t.Error("Weights should equal defaults for empty records")
	}
}

func TestCalibrate_ClearSignal_IncreasesCorrelatedWeight(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(10))

	// Create records where blast_radius strongly correlates with failure
	// and other factors do not.
	records := make([]CalibrationRecord, 100)
	for i := range records {
		isFailure := i%5 == 0 // 20% failure rate

		outcome := "success"
		blastScore := 0.1 + float64(i%3)*0.05 // low for successes
		if isFailure {
			outcome = "rollback"
			blastScore = 0.9 // high for failures
		}

		records[i] = CalibrationRecord{
			RiskScore: 0.5,
			FactorScores: map[string]float64{
				"api_change":        0.3,
				"dependency_impact": 0.3,
				"blast_radius":      blastScore,
				"code_complexity":   0.3,
				"test_coverage":     0.3,
				"actor_trust":       0.3,
				"historical_risk":   0.3,
				"security_impact":   0.3,
			},
			Outcome:    outcome,
			ReleasedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}

	result := calibrator.Calibrate(records)

	if result.SampleSize != 100 {
		t.Errorf("SampleSize = %d, want 100", result.SampleSize)
	}

	// blast_radius should have the highest weight because it correlates with failure.
	blastWeight := result.Weights.BlastRadius
	otherWeights := []float64{
		result.Weights.APIChanges,
		result.Weights.DependencyImpact,
		result.Weights.CodeComplexity,
		result.Weights.TestCoverage,
		result.Weights.ActorTrust,
		result.Weights.HistoricalRisk,
		result.Weights.SecurityImpact,
	}

	for _, w := range otherWeights {
		if blastWeight <= w {
			t.Errorf("blast_radius weight (%v) should be greater than other weights, found %v", blastWeight, w)
		}
	}

	// blast_radius should have positive factor impact.
	if result.FactorImpact["blast_radius"] <= 0 {
		t.Errorf("blast_radius factor impact = %v, want positive", result.FactorImpact["blast_radius"])
	}
}

func TestCalibrate_WeightsSumToOne(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(5))

	records := generateMixedRecords(60)
	result := calibrator.Calibrate(records)

	wc := result.Weights
	total := wc.APIChanges + wc.DependencyImpact + wc.BlastRadius +
		wc.CodeComplexity + wc.TestCoverage + wc.ActorTrust +
		wc.HistoricalRisk + wc.SecurityImpact

	if math.Abs(total-1.0) > 1e-10 {
		t.Errorf("Weights sum = %v, want 1.0", total)
	}
}

func TestCalibrate_WeightsSumToOne_DefaultsAlso(t *testing.T) {
	calibrator := NewCalibrator()

	// Insufficient samples should still return weights summing to 1.0.
	result := calibrator.Calibrate(nil)

	wc := result.Weights
	total := wc.APIChanges + wc.DependencyImpact + wc.BlastRadius +
		wc.CodeComplexity + wc.TestCoverage + wc.ActorTrust +
		wc.HistoricalRisk + wc.SecurityImpact

	if math.Abs(total-1.0) > 0.01 {
		t.Errorf("Default weights sum = %v, want ~1.0", total)
	}
}

func TestCalibrate_AccuracyComputation(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(2))

	// Create perfectly separable data:
	// - success records have all factor scores at 0.1
	// - failure records have all factor scores at 0.9
	records := make([]CalibrationRecord, 20)
	for i := range records {
		isFailure := i < 10
		outcome := "success"
		score := 0.1
		if isFailure {
			outcome = "incident"
			score = 0.9
		}

		factors := make(map[string]float64)
		for _, name := range factorNames {
			factors[name] = score
		}

		records[i] = CalibrationRecord{
			RiskScore:    score,
			FactorScores: factors,
			Outcome:      outcome,
			ReleasedAt:   time.Now(),
		}
	}

	result := calibrator.Calibrate(records)

	// With perfectly separable data, accuracy should be 1.0.
	if result.Accuracy != 1.0 {
		t.Errorf("Accuracy = %v, want 1.0 for perfectly separable data", result.Accuracy)
	}
}

func TestCalibrate_AllSuccesses(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(5))

	records := make([]CalibrationRecord, 20)
	for i := range records {
		factors := make(map[string]float64)
		for _, name := range factorNames {
			factors[name] = 0.3 + float64(i%3)*0.1
		}
		records[i] = CalibrationRecord{
			RiskScore:    0.3,
			FactorScores: factors,
			Outcome:      "success",
			ReleasedAt:   time.Now(),
		}
	}

	result := calibrator.Calibrate(records)

	// All correlations should be zero (no variance in outcome).
	for _, name := range factorNames {
		if result.FactorImpact[name] != 0 {
			t.Errorf("FactorImpact[%s] = %v, want 0 for all-success data", name, result.FactorImpact[name])
		}
	}

	// Weights should still sum to 1.0.
	wc := result.Weights
	total := wc.APIChanges + wc.DependencyImpact + wc.BlastRadius +
		wc.CodeComplexity + wc.TestCoverage + wc.ActorTrust +
		wc.HistoricalRisk + wc.SecurityImpact

	if math.Abs(total-1.0) > 1e-10 {
		t.Errorf("Weights sum = %v, want 1.0", total)
	}
}

func TestCalibrate_AllFailures(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(5))

	records := make([]CalibrationRecord, 20)
	for i := range records {
		factors := make(map[string]float64)
		for _, name := range factorNames {
			factors[name] = 0.5 + float64(i%3)*0.1
		}
		records[i] = CalibrationRecord{
			RiskScore:    0.7,
			FactorScores: factors,
			Outcome:      "rollback",
			ReleasedAt:   time.Now(),
		}
	}

	result := calibrator.Calibrate(records)

	// All correlations should be zero (no variance in outcome).
	for _, name := range factorNames {
		if result.FactorImpact[name] != 0 {
			t.Errorf("FactorImpact[%s] = %v, want 0 for all-failure data", name, result.FactorImpact[name])
		}
	}

	// Weights should still sum to 1.0.
	wc := result.Weights
	total := wc.APIChanges + wc.DependencyImpact + wc.BlastRadius +
		wc.CodeComplexity + wc.TestCoverage + wc.ActorTrust +
		wc.HistoricalRisk + wc.SecurityImpact

	if math.Abs(total-1.0) > 1e-10 {
		t.Errorf("Weights sum = %v, want 1.0", total)
	}
}

func TestCalibrate_SingleSample(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(1))

	records := []CalibrationRecord{
		{
			RiskScore: 0.5,
			FactorScores: map[string]float64{
				"api_change":        0.5,
				"dependency_impact": 0.5,
				"blast_radius":      0.5,
				"code_complexity":   0.5,
				"test_coverage":     0.5,
				"actor_trust":       0.5,
				"historical_risk":   0.5,
				"security_impact":   0.5,
			},
			Outcome:    "success",
			ReleasedAt: time.Now(),
		},
	}

	result := calibrator.Calibrate(records)

	// With a single sample, correlations should all be 0 (need at least 2).
	for _, name := range factorNames {
		if result.FactorImpact[name] != 0 {
			t.Errorf("FactorImpact[%s] = %v, want 0 for single sample", name, result.FactorImpact[name])
		}
	}

	// Weights should still sum to 1.0.
	wc := result.Weights
	total := wc.APIChanges + wc.DependencyImpact + wc.BlastRadius +
		wc.CodeComplexity + wc.TestCoverage + wc.ActorTrust +
		wc.HistoricalRisk + wc.SecurityImpact

	if math.Abs(total-1.0) > 1e-10 {
		t.Errorf("Weights sum = %v, want 1.0", total)
	}
}

func TestCalibrate_WithMinSamplesOption(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(5))

	// 4 records should be insufficient with minSamples=5.
	records := make([]CalibrationRecord, 4)
	for i := range records {
		records[i] = CalibrationRecord{
			RiskScore:    0.3,
			FactorScores: map[string]float64{"api_change": 0.5},
			Outcome:      "success",
			ReleasedAt:   time.Now(),
		}
	}

	result := calibrator.Calibrate(records)
	if result.Weights != DefaultWeights() {
		t.Error("Should return defaults when below custom minSamples threshold")
	}

	// 5 records should be sufficient.
	records = append(records, CalibrationRecord{
		RiskScore:    0.3,
		FactorScores: map[string]float64{"api_change": 0.5},
		Outcome:      "success",
		ReleasedAt:   time.Now(),
	})

	result = calibrator.Calibrate(records)
	if result.SampleSize != 5 {
		t.Errorf("SampleSize = %d, want 5", result.SampleSize)
	}
	// Should have non-default accuracy computation (even if 0 variance).
}

func TestCalibrate_MixedOutcomes(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(5))

	records := make([]CalibrationRecord, 60)
	for i := range records {
		outcome := "success"
		securityScore := 0.2
		switch i % 6 {
		case 0:
			outcome = "rollback"
			securityScore = 0.8
		case 1:
			outcome = "incident"
			securityScore = 0.9
		case 2:
			outcome = "hotfix"
			securityScore = 0.7
		}

		records[i] = CalibrationRecord{
			RiskScore: 0.5,
			FactorScores: map[string]float64{
				"api_change":        0.4,
				"dependency_impact": 0.4,
				"blast_radius":      0.4,
				"code_complexity":   0.4,
				"test_coverage":     0.4,
				"actor_trust":       0.4,
				"historical_risk":   0.4,
				"security_impact":   securityScore,
			},
			Outcome:    outcome,
			ReleasedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}

	result := calibrator.Calibrate(records)

	// security_impact should have the highest weight.
	secWeight := result.Weights.SecurityImpact
	if secWeight <= result.Weights.APIChanges {
		t.Errorf("security_impact weight (%v) should exceed api_change weight (%v)",
			secWeight, result.Weights.APIChanges)
	}

	// security_impact should have positive factor impact.
	if result.FactorImpact["security_impact"] <= 0 {
		t.Errorf("security_impact factor impact = %v, want positive",
			result.FactorImpact["security_impact"])
	}
}

func TestApplyCalibration(t *testing.T) {
	calc := NewCalculatorWithDefaults()

	customWeights := WeightConfig{
		APIChanges:       0.30,
		DependencyImpact: 0.15,
		BlastRadius:      0.20,
		CodeComplexity:   0.10,
		TestCoverage:     0.05,
		ActorTrust:       0.05,
		HistoricalRisk:   0.05,
		SecurityImpact:   0.10,
	}

	result := CalibrationResult{
		Weights:      customWeights,
		SampleSize:   100,
		Accuracy:     0.85,
		FactorImpact: map[string]float64{"api_change": 0.6},
		CalibratedAt: time.Now(),
	}

	calc.ApplyCalibration(result)

	if calc.weights != customWeights {
		t.Error("ApplyCalibration should update calculator weights")
	}
}

func TestWithMinSamples_InvalidValue(t *testing.T) {
	// Zero or negative should not change the default.
	calibrator := NewCalibrator(WithMinSamples(0))
	// Default should remain 50.
	records := make([]CalibrationRecord, 49)
	for i := range records {
		records[i] = CalibrationRecord{
			RiskScore:    0.3,
			FactorScores: map[string]float64{"api_change": 0.5},
			Outcome:      "success",
			ReleasedAt:   time.Now(),
		}
	}
	result := calibrator.Calibrate(records)
	if result.Weights != DefaultWeights() {
		t.Error("With invalid minSamples, default (50) should be used; 49 records should return defaults")
	}
}

func TestCalibrate_MissingFactorScores(t *testing.T) {
	calibrator := NewCalibrator(WithMinSamples(2))

	// Records with only some factors present.
	records := make([]CalibrationRecord, 10)
	for i := range records {
		outcome := "success"
		if i%3 == 0 {
			outcome = "rollback"
		}
		records[i] = CalibrationRecord{
			RiskScore: 0.5,
			FactorScores: map[string]float64{
				"api_change":   0.5,
				"blast_radius": float64(i) / 10.0,
			},
			Outcome:    outcome,
			ReleasedAt: time.Now(),
		}
	}

	result := calibrator.Calibrate(records)

	// Should not panic and weights should sum to 1.0.
	wc := result.Weights
	total := wc.APIChanges + wc.DependencyImpact + wc.BlastRadius +
		wc.CodeComplexity + wc.TestCoverage + wc.ActorTrust +
		wc.HistoricalRisk + wc.SecurityImpact

	if math.Abs(total-1.0) > 1e-10 {
		t.Errorf("Weights sum = %v, want 1.0", total)
	}
}

func TestCalibrate_HotfixAndIncidentOutcomes(t *testing.T) {
	// Verify that hotfix and incident are treated as negative outcomes.
	if !isNegativeOutcome("rollback") {
		t.Error("rollback should be negative")
	}
	if !isNegativeOutcome("incident") {
		t.Error("incident should be negative")
	}
	if !isNegativeOutcome("hotfix") {
		t.Error("hotfix should be negative")
	}
	if isNegativeOutcome("success") {
		t.Error("success should not be negative")
	}
	if isNegativeOutcome("") {
		t.Error("empty string should not be negative")
	}
}

func TestWeightConfigRoundTrip(t *testing.T) {
	original := DefaultWeights()
	m := weightConfigToMap(original)
	restored := mapToWeightConfig(m)

	if original != restored {
		t.Errorf("WeightConfig round-trip failed: %v != %v", original, restored)
	}
}

func TestNormalizeWeights_AllZeros(t *testing.T) {
	raw := map[string]float64{
		"a": 0,
		"b": 0,
		"c": 0,
	}
	result := normalizeWeights(raw)

	total := 0.0
	for _, v := range result {
		total += v
	}
	if math.Abs(total-1.0) > 1e-10 {
		t.Errorf("normalizeWeights with all zeros should sum to 1.0, got %v", total)
	}

	// Each weight should be equal.
	expected := 1.0 / 3.0
	for k, v := range result {
		if math.Abs(v-expected) > 1e-10 {
			t.Errorf("normalizeWeights[%s] = %v, want %v", k, v, expected)
		}
	}
}

// generateMixedRecords creates a set of records with varied outcomes and scores.
func generateMixedRecords(n int) []CalibrationRecord {
	records := make([]CalibrationRecord, n)
	for i := range records {
		outcome := "success"
		if i%4 == 0 {
			outcome = "rollback"
		} else if i%7 == 0 {
			outcome = "hotfix"
		}

		factors := make(map[string]float64)
		for j, name := range factorNames {
			factors[name] = math.Mod(float64(i+j)*0.13, 1.0)
		}

		records[i] = CalibrationRecord{
			RiskScore:    float64(i%10) / 10.0,
			FactorScores: factors,
			Outcome:      outcome,
			ReleasedAt:   time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return records
}
