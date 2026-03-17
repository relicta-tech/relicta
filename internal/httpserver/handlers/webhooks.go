package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/infrastructure/webhook/queue"
)

// WebhookDeliveryService defines the interface required by webhook handlers.
// This abstraction enables testing without the full queue infrastructure.
type WebhookDeliveryService interface {
	// GetDeliveries returns all deliveries for a webhook endpoint.
	GetDeliveries(endpointID string) ([]*queue.Delivery, error)
	// GetDelivery returns a specific delivery by ID for an endpoint.
	GetDelivery(endpointID, deliveryID string) (*queue.Delivery, error)
	// Redeliver re-enqueues a delivery for another attempt.
	Redeliver(endpointID, deliveryID string) (*queue.Delivery, error)
	// GetDeliveryLog returns delivery log entries for an endpoint.
	GetDeliveryLog(endpointID string) ([]queue.DeliveryLogEntry, error)
}

// webhookService is the service used by webhook handlers.
// Set via SetWebhookService during server initialization.
var webhookService WebhookDeliveryService

// SetWebhookService sets the webhook delivery service for handlers.
func SetWebhookService(svc WebhookDeliveryService) {
	webhookService = svc
}

// ListWebhookDeliveries handles GET /api/v1/webhooks/:id/deliveries.
// Returns delivery history for a specific webhook endpoint.
func ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	if webhookService == nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeServiceUnavailable,
			"webhook service not available", nil)
		return
	}

	endpointID := chi.URLParam(r, "id")
	if endpointID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField,
			"missing webhook endpoint ID", nil)
		return
	}

	deliveries, err := webhookService.GetDeliveries(endpointID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal,
			"failed to list deliveries", err.Error())
		return
	}

	logEntries, err := webhookService.GetDeliveryLog(endpointID)
	if err != nil {
		// Non-fatal: return deliveries without log entries
		logEntries = nil
	}

	type deliveryResponse struct {
		Deliveries []*queue.Delivery        `json:"deliveries"`
		LogEntries []queue.DeliveryLogEntry `json:"log_entries,omitempty"`
	}

	if deliveries == nil {
		deliveries = []*queue.Delivery{}
	}

	respondJSON(w, http.StatusOK, deliveryResponse{
		Deliveries: deliveries,
		LogEntries: logEntries,
	})
}

// RedeliverWebhook handles POST /api/v1/webhooks/:id/deliveries/:deliveryId/redeliver.
// Re-enqueues a specific delivery for another attempt.
func RedeliverWebhook(w http.ResponseWriter, r *http.Request) {
	if webhookService == nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeServiceUnavailable,
			"webhook service not available", nil)
		return
	}

	endpointID := chi.URLParam(r, "id")
	deliveryID := chi.URLParam(r, "deliveryId")

	if endpointID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField,
			"missing webhook endpoint ID", nil)
		return
	}
	if deliveryID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField,
			"missing delivery ID", nil)
		return
	}

	delivery, err := webhookService.Redeliver(endpointID, deliveryID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, ErrCodeNotFound,
			"delivery not found or cannot be redelivered", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message":  "delivery requeued",
		"delivery": delivery,
	})
}
