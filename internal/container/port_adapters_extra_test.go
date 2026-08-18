package container

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

func TestWithAttestationConfig(t *testing.T) {
	cfg := &config.AttestationConfig{Enabled: true}
	opt := WithAttestationConfig(cfg)

	adapter := &PublisherAdapter{}
	opt(adapter)

	if adapter.attestationConfig != cfg {
		t.Error("WithAttestationConfig did not set config")
	}
	if !adapter.attestationConfig.Enabled {
		t.Error("attestation should be enabled")
	}
}

func TestWithAuditChain(t *testing.T) {
	store := cgpmemory.NewInMemoryStore()
	opt := WithAuditChain(store, "acme/widget")

	adapter := &PublisherAdapter{}
	opt(adapter)

	if adapter.auditChainStore != audit.Store(store) {
		t.Error("WithAuditChain did not set the store the attestation reads its chain from")
	}
	if adapter.auditChainRepo != "acme/widget" {
		t.Errorf("WithAuditChain set repository %q, want acme/widget: the attestation "+
			"would anchor to another repository's chain or to none",
			adapter.auditChainRepo)
	}
}

func TestWithAttestationConfig_Nil(t *testing.T) {
	opt := WithAttestationConfig(nil)

	adapter := &PublisherAdapter{}
	opt(adapter)

	if adapter.attestationConfig != nil {
		t.Error("expected nil attestation config")
	}
}

func TestExecuteAttestationStep_NilConfig(t *testing.T) {
	adapter := &PublisherAdapter{attestationConfig: nil}

	result, err := adapter.executeAttestationStep(t.Context(), nil)
	if err != nil {
		t.Fatalf("executeAttestationStep() error = %v", err)
	}
	if !result.Success {
		t.Error("expected success when config is nil")
	}
	if result.Output != "Attestation generation skipped (not enabled)" {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestExecuteAttestationStep_DisabledConfig(t *testing.T) {
	adapter := &PublisherAdapter{
		attestationConfig: &config.AttestationConfig{Enabled: false},
	}

	result, err := adapter.executeAttestationStep(t.Context(), nil)
	if err != nil {
		t.Fatalf("executeAttestationStep() error = %v", err)
	}
	if !result.Success {
		t.Error("expected success when config is disabled")
	}
	if result.Output != "Attestation generation skipped (not enabled)" {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestExecuteAttestationStep_EnabledNoAuditChain(t *testing.T) {
	// When attestation is enabled but no audit chain, Generator will work but
	// we still exercise the enabled path.
	adapter := &PublisherAdapter{
		attestationConfig: &config.AttestationConfig{
			Enabled:     true,
			SigningMode: "local",
		},
	}

	run := domain.NewReleaseRun("repo", t.TempDir(), "v1.0.0", "abc123", nil, "cfg", "plug")

	result, err := adapter.executeAttestationStep(t.Context(), run)
	if err != nil {
		t.Fatalf("executeAttestationStep() error = %v", err)
	}
	// Should succeed (non-blocking step)
	if !result.Success {
		t.Error("expected success for non-blocking attestation step")
	}
}

func TestNewPublisherAdapter_WithOptions(t *testing.T) {
	cfg := &config.AttestationConfig{Enabled: true}
	store := cgpmemory.NewInMemoryStore()

	adapter := NewPublisherAdapter(nil, nil, nil,
		WithPushTags(true),
		WithAttestationConfig(cfg),
		WithAuditChain(store, "acme/widget"),
	)

	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if !adapter.pushTags {
		t.Error("expected pushTags to be true")
	}
	if adapter.attestationConfig != cfg {
		t.Error("expected attestation config to be set")
	}
	if adapter.auditChainStore != audit.Store(store) || adapter.auditChainRepo != "acme/widget" {
		t.Error("expected the audit chain store and repository to be set")
	}
}
