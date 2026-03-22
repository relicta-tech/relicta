package analytics

import (
	"sort"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
)

// OutcomeMetrics captures release outcome statistics.
type OutcomeMetrics struct {
	TotalReleases int              `json:"total_releases"`
	SuccessRate   float64          `json:"success_rate"`
	RollbackRate  float64          `json:"rollback_rate"`
	IncidentRate  float64          `json:"incident_rate"`
	HotfixRate    float64          `json:"hotfix_rate"`
	ByMonth       []MonthlyOutcome `json:"by_month"`
}

// MonthlyOutcome holds outcome counts for a single calendar month.
type MonthlyOutcome struct {
	Month    string `json:"month"`
	Total    int    `json:"total"`
	Success  int    `json:"success"`
	Rollback int    `json:"rollback"`
	Incident int    `json:"incident"`
	Hotfix   int    `json:"hotfix"`
}

// RiskFactorAttribution shows which risk factors drove governance decisions.
type RiskFactorAttribution struct {
	Factor       string  `json:"factor"`
	AverageScore float64 `json:"average_score"`
	Correlation  float64 `json:"correlation"`
	Weight       float64 `json:"weight"`
	TriggerCount int     `json:"trigger_count"`
}

// CalibrationMetrics exposes prediction accuracy and weight drift.
type CalibrationMetrics struct {
	Accuracy         float64                 `json:"accuracy"`
	SampleSize       int                     `json:"sample_size"`
	HighRiskAccuracy float64                 `json:"high_risk_accuracy"`
	LowRiskAccuracy  float64                 `json:"low_risk_accuracy"`
	CalibratedAt     *time.Time              `json:"calibrated_at"`
	WeightChanges    map[string]WeightChange `json:"weight_changes"`
}

// WeightChange captures the delta between default and calibrated weights.
type WeightChange struct {
	Default    float64 `json:"default"`
	Calibrated float64 `json:"calibrated"`
	Delta      float64 `json:"delta"`
}

// Period defines a time window for outcome queries.
type Period struct {
	From *time.Time
	To   *time.Time
}

// outcomeCategory returns a normalised category string for a release record.
func outcomeCategory(r *memory.ReleaseRecord) string {
	switch r.Outcome {
	case memory.OutcomeSuccess:
		return "success"
	case memory.OutcomeRollback:
		return "rollback"
	default:
		return "other"
	}
}

// isHotfix returns true when the record's tags contain "hotfix".
func isHotfix(r *memory.ReleaseRecord) bool {
	for _, t := range r.Tags {
		if t == "hotfix" {
			return true
		}
	}
	return false
}

// isIncident returns true when the record's tags contain "incident".
func isIncident(r *memory.ReleaseRecord) bool {
	for _, t := range r.Tags {
		if t == "incident" {
			return true
		}
	}
	return false
}

// filterByPeriod returns the subset of records whose ReleasedAt falls
// within the given period boundaries (inclusive).
func filterByPeriod(records []*memory.ReleaseRecord, p Period) []*memory.ReleaseRecord {
	var out []*memory.ReleaseRecord
	for _, r := range records {
		if p.From != nil && r.ReleasedAt.Before(*p.From) {
			continue
		}
		if p.To != nil && r.ReleasedAt.After(*p.To) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ComputeOutcomeMetrics derives outcome statistics from a set of release records.
func ComputeOutcomeMetrics(records []*memory.ReleaseRecord, period Period) OutcomeMetrics {
	filtered := filterByPeriod(records, period)

	total := len(filtered)
	if total == 0 {
		return OutcomeMetrics{}
	}

	var successCount, rollbackCount, incidentCount, hotfixCount int
	monthly := make(map[string]*MonthlyOutcome)
	var monthOrder []string

	for _, r := range filtered {
		cat := outcomeCategory(r)
		hot := isHotfix(r)
		inc := isIncident(r)

		switch cat {
		case "success":
			successCount++
		case "rollback":
			rollbackCount++
		}
		if inc {
			incidentCount++
		}
		if hot {
			hotfixCount++
		}

		monthKey := r.ReleasedAt.UTC().Format("2006-01")
		m, exists := monthly[monthKey]
		if !exists {
			m = &MonthlyOutcome{Month: monthKey}
			monthly[monthKey] = m
			monthOrder = append(monthOrder, monthKey)
		}
		m.Total++
		switch cat {
		case "success":
			m.Success++
		case "rollback":
			m.Rollback++
		}
		if inc {
			m.Incident++
		}
		if hot {
			m.Hotfix++
		}
	}

	sort.Strings(monthOrder)
	byMonth := make([]MonthlyOutcome, 0, len(monthOrder))
	for _, k := range monthOrder {
		byMonth = append(byMonth, *monthly[k])
	}

	return OutcomeMetrics{
		TotalReleases: total,
		SuccessRate:   float64(successCount) / float64(total),
		RollbackRate:  float64(rollbackCount) / float64(total),
		IncidentRate:  float64(incidentCount) / float64(total),
		HotfixRate:    float64(hotfixCount) / float64(total),
		ByMonth:       byMonth,
	}
}

// ComputeRiskFactorAttribution analyses which risk factors appear most frequently
// and correlate with negative outcomes across the release history.
func ComputeRiskFactorAttribution(records []*memory.ReleaseRecord) []RiskFactorAttribution {
	if len(records) == 0 {
		return nil
	}

	// Collect per-tag statistics.
	type tagStats struct {
		totalScore float64
		count      int
		negCount   int
	}

	stats := make(map[string]*tagStats)

	for _, r := range records {
		negative := r.Outcome.IsNegative()
		for _, tag := range r.Tags {
			s, ok := stats[tag]
			if !ok {
				s = &tagStats{}
				stats[tag] = s
			}
			s.totalScore += r.RiskScore
			s.count++
			if negative {
				s.negCount++
			}
		}
	}

	totalRecords := float64(len(records))

	// Count overall negative outcomes for correlation baseline.
	var totalNeg int
	for _, r := range records {
		if r.Outcome.IsNegative() {
			totalNeg++
		}
	}
	baseNegRate := float64(totalNeg) / totalRecords

	result := make([]RiskFactorAttribution, 0, len(stats))
	for factor, s := range stats {
		avgScore := s.totalScore / float64(s.count)

		// Simple correlation: how much does this factor's negative rate
		// differ from the baseline? Positive values mean the factor
		// is associated with more negative outcomes.
		factorNegRate := float64(s.negCount) / float64(s.count)
		correlation := factorNegRate - baseNegRate

		result = append(result, RiskFactorAttribution{
			Factor:       factor,
			AverageScore: avgScore,
			Correlation:  correlation,
			Weight:       0, // filled by caller if weight config available
			TriggerCount: s.count,
		})
	}

	// Sort by trigger count descending for stable output.
	sort.Slice(result, func(i, j int) bool {
		if result[i].TriggerCount != result[j].TriggerCount {
			return result[i].TriggerCount > result[j].TriggerCount
		}
		return result[i].Factor < result[j].Factor
	})

	return result
}

// ComputeCalibrationMetrics evaluates prediction accuracy by replaying
// release records against the current weight configuration.
func ComputeCalibrationMetrics(records []*memory.ReleaseRecord, currentWeights risk.WeightConfig) CalibrationMetrics {
	if len(records) == 0 {
		return CalibrationMetrics{
			WeightChanges: make(map[string]WeightChange),
		}
	}

	// Convert records to CalibrationRecords for the calibrator.
	calRecords := make([]risk.CalibrationRecord, 0, len(records))
	for _, r := range records {
		outcome := string(r.Outcome)
		// Map memory outcomes to calibration outcomes.
		switch r.Outcome {
		case memory.OutcomeFailed, memory.OutcomePartial:
			outcome = "incident"
		}

		factorScores := make(map[string]float64)
		for _, tag := range r.Tags {
			factorScores[tag] = r.RiskScore
		}

		calRecords = append(calRecords, risk.CalibrationRecord{
			RiskScore:    r.RiskScore,
			FactorScores: factorScores,
			Outcome:      outcome,
			ReleasedAt:   r.ReleasedAt,
		})
	}

	// Compute accuracy by checking if the risk score correctly predicted
	// the outcome direction.
	var correct, highCorrect, lowCorrect int
	var highTotal, lowTotal int

	for _, r := range records {
		negative := r.Outcome.IsNegative()
		highRisk := r.RiskScore >= 0.5

		if highRisk {
			highTotal++
			if negative {
				highCorrect++
			}
		} else {
			lowTotal++
			if !negative {
				lowCorrect++
			}
		}

		if (highRisk && negative) || (!highRisk && !negative) {
			correct++
		}
	}

	accuracy := float64(correct) / float64(len(records))

	var highRiskAccuracy float64
	if highTotal > 0 {
		highRiskAccuracy = float64(highCorrect) / float64(highTotal)
	}

	var lowRiskAccuracy float64
	if lowTotal > 0 {
		lowRiskAccuracy = float64(lowCorrect) / float64(lowTotal)
	}

	// Compute weight changes between defaults and current.
	defaults := risk.DefaultWeights()
	weightChanges := computeWeightChanges(defaults, currentWeights)

	// Run the calibrator to get a calibration timestamp.
	calibrator := risk.NewCalibrator(risk.WithMinSamples(1))
	calResult := calibrator.Calibrate(calRecords)

	calibratedAt := calResult.CalibratedAt

	return CalibrationMetrics{
		Accuracy:         accuracy,
		SampleSize:       len(records),
		HighRiskAccuracy: highRiskAccuracy,
		LowRiskAccuracy:  lowRiskAccuracy,
		CalibratedAt:     &calibratedAt,
		WeightChanges:    weightChanges,
	}
}

// computeWeightChanges builds a map of weight deltas between two configurations.
func computeWeightChanges(defaults, current risk.WeightConfig) map[string]WeightChange {
	defaultMap := map[string]float64{
		"api_changes":       defaults.APIChanges,
		"dependency_impact": defaults.DependencyImpact,
		"blast_radius":      defaults.BlastRadius,
		"code_complexity":   defaults.CodeComplexity,
		"test_coverage":     defaults.TestCoverage,
		"actor_trust":       defaults.ActorTrust,
		"historical_risk":   defaults.HistoricalRisk,
		"security_impact":   defaults.SecurityImpact,
	}

	currentMap := map[string]float64{
		"api_changes":       current.APIChanges,
		"dependency_impact": current.DependencyImpact,
		"blast_radius":      current.BlastRadius,
		"code_complexity":   current.CodeComplexity,
		"test_coverage":     current.TestCoverage,
		"actor_trust":       current.ActorTrust,
		"historical_risk":   current.HistoricalRisk,
		"security_impact":   current.SecurityImpact,
	}

	changes := make(map[string]WeightChange, len(defaultMap))
	for name, def := range defaultMap {
		cal := currentMap[name]
		changes[name] = WeightChange{
			Default:    def,
			Calibrated: cal,
			Delta:      cal - def,
		}
	}

	return changes
}
