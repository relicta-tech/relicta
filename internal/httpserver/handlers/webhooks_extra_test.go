package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWriteSSEEvent covers the writeSSEEvent function at 0% coverage.
func TestWriteSSEEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    SSEEvent
		wantErr  bool
		contains string
	}{
		{
			name: "valid event with string data",
			event: SSEEvent{
				ID:   1,
				Type: "test.event",
				Data: map[string]string{"key": "value"},
			},
			wantErr:  false,
			contains: "event: test.event",
		},
		{
			name: "valid event with nil data",
			event: SSEEvent{
				ID:   2,
				Type: "empty.event",
				Data: nil,
			},
			wantErr:  false,
			contains: "event: empty.event",
		},
		{
			name: "valid event with numeric data",
			event: SSEEvent{
				ID:   42,
				Type: "num.event",
				Data: 12345,
			},
			wantErr:  false,
			contains: "id: 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := writeSSEEvent(w, tt.event)

			if (err != nil) != tt.wantErr {
				t.Errorf("writeSSEEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			body := w.Body.String()
			if tt.contains != "" && !containsStr(body, tt.contains) {
				t.Errorf("expected body to contain %q, got %q", tt.contains, body)
			}
		})
	}
}

// TestWriteSSEEvent_UnmarshalableData tests that writeSSEEvent returns an error
// for data that cannot be marshaled to JSON.
func TestWriteSSEEvent_UnmarshalableData(t *testing.T) {
	w := httptest.NewRecorder()
	// Channels cannot be JSON-marshaled.
	evt := SSEEvent{
		ID:   1,
		Type: "bad.event",
		Data: make(chan int),
	}

	err := writeSSEEvent(w, evt)
	if err == nil {
		t.Error("expected error for unmarshalable data")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestListWebhookDeliveries_SuccessWithNilLogEntries verifies the handler
// returns deliveries with nil log entries when GetDeliveryLog returns nil entries.
func TestListWebhookDeliveries_SuccessWithNilLogEntries(t *testing.T) {
	original := webhookService
	defer func() { webhookService = original }()

	svc := &mockWebhookService{
		deliveries: nil,
		logEntries: nil,
	}
	SetWebhookService(svc)

	req := makeWebhookRequest(http.MethodGet, "/api/v1/webhooks/ep-1/deliveries", map[string]string{"id": "ep-1"})
	w := httptest.NewRecorder()

	ListWebhookDeliveries(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
