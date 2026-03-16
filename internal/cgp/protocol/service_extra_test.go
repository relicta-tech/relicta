package protocol

import (
	"log/slog"
	"testing"
)

func TestWithServiceLogger(t *testing.T) {
	logger := slog.Default()
	opt := WithServiceLogger(logger)

	svc := &Service{}
	opt(svc)

	if svc.logger != logger {
		t.Error("WithServiceLogger did not set logger")
	}
}

func TestWithStore(t *testing.T) {
	store := &inMemoryStore{}
	opt := WithStore(store)

	svc := &Service{}
	opt(svc)

	if svc.store != store {
		t.Error("WithStore did not set store")
	}
}

func TestNewService_WithOptions(t *testing.T) {
	// Test that NewService applies options correctly.
	logger := slog.Default()
	store := &inMemoryStore{}

	svc := NewService(nil, WithServiceLogger(logger), WithStore(store))

	if svc.logger != logger {
		t.Error("expected custom logger")
	}
	if svc.store != store {
		t.Error("expected custom store")
	}
}
