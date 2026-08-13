package queue

import (
	"testing"
	"time"
)

func TestNewDeliveryLog_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()

	log, err := NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog() error = %v", err)
	}
	if log == nil {
		t.Fatal("NewDeliveryLog() returned nil")
	}
}

func TestDeliveryLog_RecordAndList(t *testing.T) {
	dir := t.TempDir()
	log, err := NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	entries := []DeliveryLogEntry{
		{
			DeliveryID:     "d-001",
			Timestamp:      time.Now().UTC().Add(-2 * time.Minute),
			EndpointURL:    endpoint,
			StatusCode:     500,
			ResponseTimeMS: 150,
			PayloadHash:    "abc123",
			Success:        false,
			Error:          "server error",
			Attempt:        1,
			Event:          "release.published",
		},
		{
			DeliveryID:     "d-001",
			Timestamp:      time.Now().UTC().Add(-1 * time.Minute),
			EndpointURL:    endpoint,
			StatusCode:     200,
			ResponseTimeMS: 45,
			PayloadHash:    "abc123",
			Success:        true,
			Attempt:        2,
			Event:          "release.published",
		},
	}

	for _, entry := range entries {
		if err := log.Record(entry); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}

	// List all entries
	result, err := log.List(endpoint)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("List() returned %d entries, want 2", len(result))
	}

	// Should be sorted newest first
	if result[0].Attempt != 2 {
		t.Errorf("first entry should be newest (attempt 2), got attempt %d", result[0].Attempt)
	}
}

func TestDeliveryLog_ListByDeliveryID(t *testing.T) {
	dir := t.TempDir()
	log, err := NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog() error = %v", err)
	}

	endpoint := "https://example.com/webhook"

	// Record entries for multiple deliveries
	for _, id := range []string{"d-001", "d-002", "d-001"} {
		entry := DeliveryLogEntry{
			DeliveryID:  id,
			Timestamp:   time.Now().UTC(),
			EndpointURL: endpoint,
			StatusCode:  200,
			Success:     true,
			Attempt:     1,
		}
		if err := log.Record(entry); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}

	// Filter by delivery ID
	result, err := log.ListByDeliveryID(endpoint, "d-001")
	if err != nil {
		t.Fatalf("ListByDeliveryID() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("ListByDeliveryID('d-001') = %d, want 2", len(result))
	}
}

func TestDeliveryLog_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	log, err := NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog() error = %v", err)
	}

	result, err := log.List("https://nonexistent.com/webhook")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if result != nil {
		t.Errorf("List() for nonexistent endpoint should return nil, got %v", result)
	}
}

func TestDeliveryLog_MultipleEndpoints(t *testing.T) {
	dir := t.TempDir()
	log, err := NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog() error = %v", err)
	}

	ep1 := "https://a.com/webhook"
	ep2 := "https://b.com/webhook"

	for _, ep := range []string{ep1, ep1, ep2} {
		entry := DeliveryLogEntry{
			DeliveryID:  "d-test",
			Timestamp:   time.Now().UTC(),
			EndpointURL: ep,
			StatusCode:  200,
			Success:     true,
		}
		if err := log.Record(entry); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}

	r1, _ := log.List(ep1)
	r2, _ := log.List(ep2)

	if len(r1) != 2 {
		t.Errorf("ep1 entries = %d, want 2", len(r1))
	}
	if len(r2) != 1 {
		t.Errorf("ep2 entries = %d, want 1", len(r2))
	}
}
