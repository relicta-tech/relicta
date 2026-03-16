package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

func TestGetMemoryInsights_WithService(t *testing.T) {
	origSvc := memoryInsightsService
	defer func() { memoryInsightsService = origSvc }()

	outcomeStore := memory.NewInMemoryOutcomeStore()
	memStore := memory.NewInMemoryStore()
	detector := memory.NewPatternDetector(outcomeStore, memStore)
	svc := memory.NewInsightsService(memStore, outcomeStore, detector, "owner/repo")
	SetMemoryInsightsService(svc)

	t.Run("missing release_id returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/insights", nil)
		w := httptest.NewRecorder()

		GetMemoryInsights(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("valid release_id returns insights", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/insights?release_id=rel-1", nil)
		w := httptest.NewRecorder()

		GetMemoryInsights(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if _, ok := resp["insights"]; !ok {
			t.Error("response should contain 'insights' key")
		}
	})
}

func TestGetMemoryTrends_WithService(t *testing.T) {
	origSvc := memoryInsightsService
	defer func() { memoryInsightsService = origSvc }()

	outcomeStore := memory.NewInMemoryOutcomeStore()
	memStore := memory.NewInMemoryStore()
	detector := memory.NewPatternDetector(outcomeStore, memStore)
	svc := memory.NewInsightsService(memStore, outcomeStore, detector, "owner/repo")
	SetMemoryInsightsService(svc)

	t.Run("default window", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/trends", nil)
		w := httptest.NewRecorder()

		GetMemoryTrends(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if _, ok := resp["trends"]; !ok {
			t.Error("response should contain 'trends' key")
		}
	})

	t.Run("custom window", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/trends?window=7d", nil)
		w := httptest.NewRecorder()

		GetMemoryTrends(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}
