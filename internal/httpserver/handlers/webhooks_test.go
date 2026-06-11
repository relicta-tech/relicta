package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v4/internal/infrastructure/webhook/queue"
)

// mockWebhookService implements WebhookDeliveryService for testing.
type mockWebhookService struct {
	deliveries   []*queue.Delivery
	logEntries   []queue.DeliveryLogEntry
	redelivered  *queue.Delivery
	getErr       error
	logErr       error
	redeliverErr error
}

func (m *mockWebhookService) GetDeliveries(_ string) ([]*queue.Delivery, error) {
	return m.deliveries, m.getErr
}

func (m *mockWebhookService) GetDelivery(_, _ string) (*queue.Delivery, error) {
	if len(m.deliveries) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return m.deliveries[0], m.getErr
}

func (m *mockWebhookService) Redeliver(_, _ string) (*queue.Delivery, error) {
	return m.redelivered, m.redeliverErr
}

func (m *mockWebhookService) GetDeliveryLog(_ string) ([]queue.DeliveryLogEntry, error) {
	return m.logEntries, m.logErr
}

// makeWebhookRequest creates a request with chi URL params set via context.
func makeWebhookRequest(method, path string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestSetWebhookService(t *testing.T) {
	// Save original and restore after test.
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{}
	SetWebhookService(svc)

	if webhookService != svc {
		t.Error("SetWebhookService did not set the service")
	}
}

func TestListWebhookDeliveries_ServiceUnavailable(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()
	webhookService = nil

	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks/ep-1/deliveries", map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestListWebhookDeliveries_Success(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	now := time.Now().UTC()
	svc := &mockWebhookService{
		deliveries: []*queue.Delivery{
			{
				ID:          "del-1",
				EndpointURL: "https://example.com/hook",
				Status:      queue.StatusDelivered,
				CreatedAt:   now,
			},
		},
		logEntries: []queue.DeliveryLogEntry{
			{DeliveryID: "del-1", Timestamp: now, Success: true},
		},
	}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks/ep-1/deliveries", map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	deliveries, ok := resp["deliveries"]
	if !ok {
		t.Error("response should contain 'deliveries' key")
	}
	arr, ok := deliveries.([]any)
	if !ok || len(arr) != 1 {
		t.Errorf("expected 1 delivery, got %v", deliveries)
	}
}

func TestListWebhookDeliveries_EmptyDeliveries(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{deliveries: nil}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks/ep-1/deliveries", map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Nil deliveries should be replaced with empty slice.
	deliveries := resp["deliveries"]
	arr, ok := deliveries.([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("expected empty deliveries array, got %v", deliveries)
	}
}

func TestListWebhookDeliveries_ServiceError(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{getErr: fmt.Errorf("db connection failed")}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks/ep-1/deliveries", map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestListWebhookDeliveries_LogError_NonFatal(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{
		deliveries: []*queue.Delivery{{ID: "del-1", Status: queue.StatusDelivered}},
		logErr:     fmt.Errorf("log read error"),
	}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks/ep-1/deliveries", map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	// Log errors are non-fatal — should still return 200.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even with log error, got %d", w.Code)
	}
}

func TestListWebhookDeliveries_MissingEndpointID(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{}
	SetWebhookService(svc)

	// No "id" param set — chi.URLParam returns "".
	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks//deliveries", map[string]string{})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing endpointId, got %d", w.Code)
	}
}

func TestRedeliverWebhook_ServiceUnavailable(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()
	webhookService = nil

	req := makeWebhookRequest(http.MethodPost, "/api/v1/webhooks/ep-1/deliveries/del-1/redeliver",
		map[string]string{"id": "ep-1", "deliveryId": "del-1"})
	w := httptest.NewRecorder()

	RedeliverWebhook(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestRedeliverWebhook_Success(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{
		redelivered: &queue.Delivery{
			ID:     "del-1",
			Status: queue.StatusPending,
		},
	}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodPost, "/api/v1/webhooks/ep-1/deliveries/del-1/redeliver",
		map[string]string{"id": "ep-1", "deliveryId": "del-1"})
	w := httptest.NewRecorder()

	RedeliverWebhook(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "delivery requeued" {
		t.Errorf("expected 'delivery requeued', got %v", resp["message"])
	}
}

func TestRedeliverWebhook_NotFound(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{redeliverErr: fmt.Errorf("delivery not found")}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodPost, "/api/v1/webhooks/ep-1/deliveries/del-999/redeliver",
		map[string]string{"id": "ep-1", "deliveryId": "del-999"})
	w := httptest.NewRecorder()

	RedeliverWebhook(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestRedeliverWebhook_MissingDeliveryID(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{}
	SetWebhookService(svc)

	// id is set but deliveryId is missing.
	req := makeWebhookRequest(http.MethodPost, "/api/v1/webhooks/ep-1/deliveries//redeliver",
		map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	RedeliverWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing deliveryId, got %d", w.Code)
	}
}

func TestRedeliverWebhook_MissingEndpointID(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{}
	SetWebhookService(svc)

	// No id param — chi.URLParam returns "".
	req := makeWebhookRequest(http.MethodPost, "/api/v1/webhooks//deliveries/del-1/redeliver",
		map[string]string{"deliveryId": "del-1"})
	w := httptest.NewRecorder()

	RedeliverWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing endpointId, got %d", w.Code)
	}
}
