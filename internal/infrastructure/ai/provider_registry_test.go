package ai

import (
	"strings"
	"testing"
)

func TestIsProviderAvailable(t *testing.T) {
	// Registered providers should be available
	providers := ListProviders()
	if len(providers) == 0 {
		t.Skip("no providers registered")
	}
	for _, p := range providers {
		if !IsProviderAvailable(p) {
			t.Errorf("IsProviderAvailable(%s) = false, want true", p)
		}
	}

	// Unknown provider should not be available
	if IsProviderAvailable("nonexistent-provider-xyz") {
		t.Error("nonexistent provider should not be available")
	}
}

func TestListProviders(t *testing.T) {
	providers := ListProviders()
	// At minimum, some providers should be registered via init()
	if len(providers) == 0 {
		t.Skip("no providers registered in this build")
	}
}

func TestProviderNotAvailableError(t *testing.T) {
	err := ProviderNotAvailableError{Provider: "test-provider"}
	msg := err.Error()
	if !strings.Contains(msg, "test-provider") {
		t.Errorf("error message should contain provider name, got: %s", msg)
	}
	if !strings.Contains(msg, "not available") {
		t.Errorf("error message should contain 'not available', got: %s", msg)
	}
}
