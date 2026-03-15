package handlers

import (
	"net/http"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

// memoryInsightsService is the global insights service used by memory handlers.
// Set via SetMemoryInsightsService during server initialization.
var memoryInsightsService *memory.InsightsService

// SetMemoryInsightsService sets the insights service for the handlers.
func SetMemoryInsightsService(svc *memory.InsightsService) {
	memoryInsightsService = svc
}

// GetMemoryInsightsService returns the current insights service, or nil if not set.
func GetMemoryInsightsService() *memory.InsightsService {
	return memoryInsightsService
}

// GetMemoryInsights returns historical insights for a specific release.
// GET /api/v1/memory/insights?release_id=X
func GetMemoryInsights(w http.ResponseWriter, r *http.Request) {
	svc := GetMemoryInsightsService()
	if svc == nil {
		respondJSON(w, http.StatusOK, map[string]any{"insights": []memory.Insight{}})
		return
	}

	releaseID := r.URL.Query().Get("release_id")
	if releaseID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "release_id query parameter is required", nil)
		return
	}

	insights, err := svc.GetInsights(r.Context(), releaseID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get insights", err.Error())
		return
	}

	if insights == nil {
		insights = []memory.Insight{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"insights": insights})
}

// GetMemoryTrends returns trend data for the repository.
// GET /api/v1/memory/trends?window=30d
func GetMemoryTrends(w http.ResponseWriter, r *http.Request) {
	svc := GetMemoryInsightsService()
	if svc == nil {
		respondJSON(w, http.StatusOK, map[string]any{"trends": nil})
		return
	}

	window := parseWindow(r.URL.Query().Get("window"))

	trends, err := svc.GetTrends(r.Context(), window)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get trends", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"trends": trends})
}

// parseWindow parses a window parameter like "30d", "7d", "90d" into a duration.
// Defaults to 30 days if unparseable.
func parseWindow(s string) time.Duration {
	if s == "" {
		return 30 * 24 * time.Hour
	}

	// Try parsing as Go duration first.
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	// Try parsing Nd format (days).
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var days int
		for _, c := range s[:len(s)-1] {
			if c >= '0' && c <= '9' {
				days = days*10 + int(c-'0')
			} else {
				return 30 * 24 * time.Hour
			}
		}
		if days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}

	return 30 * 24 * time.Hour
}
