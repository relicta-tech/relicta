package handlers

import (
	"net/http"
	"time"

	"github.com/relicta-tech/relicta/internal/analytics"
)

// analyticsAggregator is the global analytics aggregator used by analytics handlers.
// Set via SetAnalyticsAggregator during server initialization.
var analyticsAggregator *analytics.CachedAggregator

// SetAnalyticsAggregator sets the analytics aggregator for the handlers.
func SetAnalyticsAggregator(agg *analytics.CachedAggregator) {
	analyticsAggregator = agg
}

// GetAnalyticsAggregator returns the current analytics aggregator, or nil if not set.
func GetAnalyticsAggregator() *analytics.CachedAggregator {
	return analyticsAggregator
}

// parseTimeRange extracts from/to query parameters as RFC3339 timestamps.
func parseTimeRange(r *http.Request) (from, to *time.Time) {
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &t
		}
	}
	return
}

// GetAnalyticsRiskTrends returns time-series risk scores aggregated by the requested granularity.
// Query params: from, to (RFC3339), granularity (day/week/month).
func GetAnalyticsRiskTrends(w http.ResponseWriter, r *http.Request) {
	agg := GetAnalyticsAggregator()
	if agg == nil {
		respondJSON(w, http.StatusOK, map[string]any{"trends": []analytics.RiskTrendPoint{}})
		return
	}

	from, to := parseTimeRange(r)
	granularity := analytics.ParseGranularity(r.URL.Query().Get("granularity"))

	filter := analytics.QueryFilter{
		From: from,
		To:   to,
	}

	trends, err := agg.RiskTrends(r.Context(), filter, granularity)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to aggregate risk trends", err.Error())
		return
	}

	if trends == nil {
		trends = []analytics.RiskTrendPoint{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"trends": trends})
}

// GetAnalyticsDecisions returns policy decision distribution aggregated by time bucket.
// Query params: from, to (RFC3339), granularity (day/week/month).
func GetAnalyticsDecisions(w http.ResponseWriter, r *http.Request) {
	agg := GetAnalyticsAggregator()
	if agg == nil {
		respondJSON(w, http.StatusOK, map[string]any{"decisions": []analytics.DecisionDistribution{}})
		return
	}

	from, to := parseTimeRange(r)
	granularity := analytics.ParseGranularity(r.URL.Query().Get("granularity"))

	filter := analytics.QueryFilter{
		From: from,
		To:   to,
	}

	decisions, err := agg.Decisions(r.Context(), filter, granularity)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to aggregate decisions", err.Error())
		return
	}

	if decisions == nil {
		decisions = []analytics.DecisionDistribution{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"decisions": decisions})
}

// GetAnalyticsTeam returns per-actor team analytics (approvals, releases, velocity).
// Query params: from, to (RFC3339).
func GetAnalyticsTeam(w http.ResponseWriter, r *http.Request) {
	agg := GetAnalyticsAggregator()
	if agg == nil {
		respondJSON(w, http.StatusOK, map[string]any{"team": []analytics.TeamMetrics{}})
		return
	}

	from, to := parseTimeRange(r)

	filter := analytics.QueryFilter{
		From: from,
		To:   to,
	}

	team, err := agg.Team(r.Context(), filter)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to aggregate team metrics", err.Error())
		return
	}

	if team == nil {
		team = []analytics.TeamMetrics{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"team": team})
}
