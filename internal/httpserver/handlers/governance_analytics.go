package handlers

import (
	"net/http"

	"github.com/relicta-tech/relicta/internal/analytics"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
)

// governanceMemoryStore is the release memory store used by governance analytics handlers.
// Set via SetGovernanceMemoryStore during server initialisation.
var governanceMemoryStore memory.Store

// governanceWeights holds the current risk weight configuration.
var governanceWeights risk.WeightConfig

// SetGovernanceMemoryStore sets the memory store for governance analytics handlers.
func SetGovernanceMemoryStore(store memory.Store) {
	governanceMemoryStore = store
}

// SetGovernanceWeights sets the risk weight configuration for calibration metrics.
func SetGovernanceWeights(w risk.WeightConfig) {
	governanceWeights = w
}

// defaultRepository is used when no repository query param is provided.
const defaultRepository = "default"

// parseRepository extracts the optional "repository" query param.
func parseRepository(r *http.Request) string {
	repo := r.URL.Query().Get("repository")
	if repo == "" {
		return defaultRepository
	}
	return repo
}

// GetAnalyticsOutcomes returns release outcome metrics for the requested period.
// Query params: from, to (RFC3339), repository.
func GetAnalyticsOutcomes(w http.ResponseWriter, r *http.Request) {
	if governanceMemoryStore == nil {
		respondJSON(w, http.StatusOK, map[string]any{"outcomes": analytics.OutcomeMetrics{}})
		return
	}

	repo := parseRepository(r)
	from, to := parseTimeRange(r)

	records, err := governanceMemoryStore.GetReleaseHistory(r.Context(), repo, 10000)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"outcomes": analytics.OutcomeMetrics{}})
		return
	}

	period := analytics.Period{From: from, To: to}
	metrics := analytics.ComputeOutcomeMetrics(records, period)

	respondJSON(w, http.StatusOK, map[string]any{"outcomes": metrics})
}

// GetAnalyticsRiskFactors returns risk factor attribution analysis.
// Query params: repository.
func GetAnalyticsRiskFactors(w http.ResponseWriter, r *http.Request) {
	if governanceMemoryStore == nil {
		respondJSON(w, http.StatusOK, map[string]any{"risk_factors": []analytics.RiskFactorAttribution{}})
		return
	}

	repo := parseRepository(r)

	records, err := governanceMemoryStore.GetReleaseHistory(r.Context(), repo, 10000)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"risk_factors": []analytics.RiskFactorAttribution{}})
		return
	}

	factors := analytics.ComputeRiskFactorAttribution(records)
	if factors == nil {
		factors = []analytics.RiskFactorAttribution{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"risk_factors": factors})
}

// GetAnalyticsCalibration returns calibration accuracy and weight change metrics.
// Query params: repository.
func GetAnalyticsCalibration(w http.ResponseWriter, r *http.Request) {
	if governanceMemoryStore == nil {
		respondJSON(w, http.StatusOK, map[string]any{"calibration": analytics.CalibrationMetrics{
			WeightChanges: make(map[string]analytics.WeightChange),
		}})
		return
	}

	repo := parseRepository(r)

	records, err := governanceMemoryStore.GetReleaseHistory(r.Context(), repo, 10000)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"calibration": analytics.CalibrationMetrics{
			WeightChanges: make(map[string]analytics.WeightChange),
		}})
		return
	}

	metrics := analytics.ComputeCalibrationMetrics(records, governanceWeights)

	respondJSON(w, http.StatusOK, map[string]any{"calibration": metrics})
}
