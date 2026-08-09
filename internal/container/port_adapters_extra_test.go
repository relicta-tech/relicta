package container

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
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
	chain := audit.NewChain()
	opt := WithAuditChain(chain)

	adapter := &PublisherAdapter{}
	opt(adapter)

	if adapter.auditChain != chain {
		t.Error("WithAuditChain did not set chain")
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
	chain := audit.NewChain()

	adapter := NewPublisherAdapter(nil, nil, nil,
		WithPushTags(true),
		WithAttestationConfig(cfg),
		WithAuditChain(chain),
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
	if adapter.auditChain != chain {
		t.Error("expected audit chain to be set")
	}
}
