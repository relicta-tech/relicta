// Package risk implements CGP risk assessment for change proposals.
package risk

import (
	"math"
	"time"
)

// factorNames lists the canonical factor categories in a stable order.
var factorNames = []string{
	"api_change",
	"dependency_impact",
	"blast_radius",
	"code_complexity",
	"test_coverage",
	"actor_trust",
	"historical_risk",
	"security_impact",
}

// CalibrationRecord holds a single historical release outcome for calibration.
type CalibrationRecord struct {
	// RiskScore is the overall risk score that was predicted.
	RiskScore float64

	// FactorScores holds per-factor scores from the original assessment.
	FactorScores map[string]float64

	// Outcome describes what happened after the release.
	// Valid values: "success", "rollback", "incident", "hotfix".
	Outcome string

	// ReleasedAt is when the release was deployed.
	ReleasedAt time.Time
}

// CalibrationResult holds the output of a calibration run.
type CalibrationResult struct {
	// Weights contains the calibrated weight configuration.
	Weights WeightConfig

	// SampleSize is the number of records used for calibration.
	SampleSize int

	// Accuracy is the prediction accuracy (0.0-1.0).
	// Measures the fraction of releases where the risk score correctly
	// predicted the outcome direction (high risk -> negative, low risk -> positive).
	Accuracy float64

	// FactorImpact maps each factor name to its correlation with negative outcomes.
	FactorImpact map[string]float64

	// CalibratedAt is when the calibration was performed.
	CalibratedAt time.Time
}

// CalibratorOption configures a Calibrator.
type CalibratorOption func(*Calibrator)

// Calibrator adjusts risk weights based on historical release outcomes.
type Calibrator struct {
	minSamples int
}

// WithMinSamples sets the minimum number of records required for calibration.
// If fewer records are provided, Calibrate returns default weights.
func WithMinSamples(n int) CalibratorOption {
	return func(c *Calibrator) {
		if n > 0 {
			c.minSamples = n
		}
	}
}

// NewCalibrator creates a Calibrator with the given options.
// The default minimum sample size is 50.
func NewCalibrator(opts ...CalibratorOption) *Calibrator {
	c := &Calibrator{
		minSamples: 50,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Calibrate analyzes historical release records and computes optimized weights.
// If the number of records is below the minimum sample threshold, the current
// default weights are returned with an accuracy of 0.
func (c *Calibrator) Calibrate(records []CalibrationRecord) CalibrationResult {
	if len(records) < c.minSamples {
		return CalibrationResult{
			Weights:      DefaultWeights(),
			SampleSize:   len(records),
			Accuracy:     0,
			FactorImpact: make(map[string]float64),
			CalibratedAt: time.Now(),
		}
	}

	// Compute per-factor correlation with negative outcomes.
	factorImpact := make(map[string]float64, len(factorNames))
	for _, name := range factorNames {
		factorImpact[name] = correlationWithNegativeOutcome(records, name)
	}

	// Convert correlations to weights. Factors with stronger positive
	// correlation to negative outcomes receive higher weights.
	// Use absolute value of correlation clipped to [0, 1], then normalize.
	rawWeights := make(map[string]float64, len(factorNames))
	for _, name := range factorNames {
		// Use max(correlation, 0) so that factors negatively correlated with
		// failure (i.e., protective factors) still receive a small baseline weight.
		w := math.Max(factorImpact[name], 0.0)
		// Apply a minimum floor so no factor is completely zeroed out.
		w = math.Max(w, 0.01)
		rawWeights[name] = w
	}

	// Normalize weights to sum to 1.0.
	weights := normalizeWeights(rawWeights)

	// Compute prediction accuracy using the calibrated weights.
	accuracy := computeAccuracy(records, weights)

	return CalibrationResult{
		Weights:      mapToWeightConfig(weights),
		SampleSize:   len(records),
		Accuracy:     accuracy,
		FactorImpact: factorImpact,
		CalibratedAt: time.Now(),
	}
}

// ApplyCalibration updates the calculator's weights from a calibration result.
func (c *Calculator) ApplyCalibration(result CalibrationResult) {
	c.weights = result.Weights
}

// isNegativeOutcome returns true for outcomes that indicate a release problem.
func isNegativeOutcome(outcome string) bool {
	return outcome == "rollback" || outcome == "incident" || outcome == "hotfix"
}

// correlationWithNegativeOutcome computes the Pearson correlation coefficient
// between a factor's scores and a binary negative-outcome indicator (1 for
// negative, 0 for positive) across all records.
func correlationWithNegativeOutcome(records []CalibrationRecord, factorName string) float64 {
	n := float64(len(records))
	if n < 2 {
		return 0
	}

	// Collect paired values: factor score (x) and outcome indicator (y).
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	var count float64

	for _, r := range records {
		score, ok := r.FactorScores[factorName]
		if !ok {
			continue
		}
		count++

		y := 0.0
		if isNegativeOutcome(r.Outcome) {
			y = 1.0
		}

		sumX += score
		sumY += y
		sumXY += score * y
		sumX2 += score * score
		sumY2 += y * y
	}

	if count < 2 {
		return 0
	}

	// Pearson correlation: r = (n*sumXY - sumX*sumY) / sqrt((n*sumX2 - sumX^2) * (n*sumY2 - sumY^2))
	numerator := count*sumXY - sumX*sumY
	denomX := count*sumX2 - sumX*sumX
	denomY := count*sumY2 - sumY*sumY

	if denomX <= 0 || denomY <= 0 {
		return 0
	}

	denom := math.Sqrt(denomX * denomY)
	if denom == 0 {
		return 0
	}

	r := numerator / denom
	// Clamp to [-1, 1] to guard against floating point drift.
	return clamp(r, -1.0, 1.0)
}

// normalizeWeights scales a map of weights so they sum to 1.0.
func normalizeWeights(raw map[string]float64) map[string]float64 {
	total := 0.0
	for _, w := range raw {
		total += w
	}

	normalized := make(map[string]float64, len(raw))
	if total == 0 {
		// Equal distribution as fallback.
		equal := 1.0 / float64(len(raw))
		for k := range raw {
			normalized[k] = equal
		}
		return normalized
	}

	for k, w := range raw {
		normalized[k] = w / total
	}
	return normalized
}

// computeAccuracy calculates what fraction of releases the calibrated weights
// would have correctly classified. A release is "correctly classified" if:
//   - risk >= 0.5 and the outcome was negative, OR
//   - risk < 0.5 and the outcome was positive.
func computeAccuracy(records []CalibrationRecord, weights map[string]float64) float64 {
	if len(records) == 0 {
		return 0
	}

	correct := 0
	for _, r := range records {
		// Recompute weighted risk score using the calibrated weights.
		score := weightedScore(r.FactorScores, weights)
		negative := isNegativeOutcome(r.Outcome)

		if (score >= 0.5 && negative) || (score < 0.5 && !negative) {
			correct++
		}
	}

	return float64(correct) / float64(len(records))
}

// weightedScore computes a weighted average of factor scores.
func weightedScore(factorScores map[string]float64, weights map[string]float64) float64 {
	totalWeight := 0.0
	totalScore := 0.0

	for name, w := range weights {
		score, ok := factorScores[name]
		if !ok {
			continue
		}
		totalScore += score * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0
	}
	return totalScore / totalWeight
}

// mapToWeightConfig converts a factor-name-keyed map to a WeightConfig struct.
func mapToWeightConfig(m map[string]float64) WeightConfig {
	return WeightConfig{
		APIChanges:       m["api_change"],
		DependencyImpact: m["dependency_impact"],
		BlastRadius:      m["blast_radius"],
		CodeComplexity:   m["code_complexity"],
		TestCoverage:     m["test_coverage"],
		ActorTrust:       m["actor_trust"],
		HistoricalRisk:   m["historical_risk"],
		SecurityImpact:   m["security_impact"],
	}
}

// weightConfigToMap converts a WeightConfig to a factor-name-keyed map.
func weightConfigToMap(wc WeightConfig) map[string]float64 {
	return map[string]float64{
		"api_change":        wc.APIChanges,
		"dependency_impact": wc.DependencyImpact,
		"blast_radius":      wc.BlastRadius,
		"code_complexity":   wc.CodeComplexity,
		"test_coverage":     wc.TestCoverage,
		"actor_trust":       wc.ActorTrust,
		"historical_risk":   wc.HistoricalRisk,
		"security_impact":   wc.SecurityImpact,
	}
}
