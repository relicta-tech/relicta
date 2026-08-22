package config

import (
	"strings"
	"testing"
)

// Every signing problem used to surface during publish, from inside the attestation step:
// keyless fails with "keyless signing requires sigstore-go", local with no key fails with
// "key_path is required". By then the tag exists and the release is half done — and unless
// attestation.required is set the failure is only a warning, so the operator gets a release
// with no attestation and a line of log saying why.

func attestationConfig(apply func(*Config)) *Config {
	cfg := DefaultConfig()
	cfg.Attestation.Enabled = true
	apply(cfg)
	return cfg
}

func TestKeylessSigningIsRefusedBeforeTheReleaseStarts(t *testing.T) {
	err := NewValidator().Validate(attestationConfig(func(c *Config) {
		c.Attestation.SigningMode = "keyless"
	}))

	if err == nil {
		t.Fatal("keyless signing was accepted. It fails partway through publish, and with " +
			"attestation.required unset that failure is a warning — so the release ships " +
			"unattested by a policy that asked for signatures")
	}
	if !strings.Contains(err.Error(), "signing_mode") {
		t.Errorf("the error does not name the setting: %v", err)
	}
	// The message has to say what to do instead, or it is only half an answer.
	if !strings.Contains(err.Error(), "local") || !strings.Contains(err.Error(), "none") {
		t.Errorf("the error does not name the modes that do work: %v", err)
	}
}

func TestLocalSigningWithoutAKeyIsRefused(t *testing.T) {
	err := NewValidator().Validate(attestationConfig(func(c *Config) {
		c.Attestation.SigningMode = "local"
	}))

	if err == nil {
		t.Fatal("local signing with no key_path was accepted; signing fails after the tag exists")
	}
	if !strings.Contains(err.Error(), "key_path") {
		t.Errorf("the error does not name the missing setting: %v", err)
	}
}

func TestLocalSigningWithAKeyValidates(t *testing.T) {
	err := NewValidator().Validate(attestationConfig(func(c *Config) {
		c.Attestation.SigningMode = "local"
		c.Attestation.KeyPath = "/keys/release.pem"
	}))
	if err != nil {
		t.Errorf("a configured local signer was refused: %v", err)
	}
}

// An unsigned attestation is still a provenance record, and it is the default.
func TestTheDefaultAttestationConfigValidates(t *testing.T) {
	if err := NewValidator().Validate(attestationConfig(func(*Config) {})); err != nil {
		t.Errorf("the default attestation configuration was refused: %v", err)
	}
}

// A repository that has not asked for attestations is not told about signing modes.
func TestSigningIsNotCheckedWhenAttestationIsOff(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Attestation.Enabled = false
	cfg.Attestation.SigningMode = "keyless"

	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("a disabled attestation section was validated for signing: %v", err)
	}
}

// The Sigstore endpoints only mean anything to the mode that is not implemented.
func TestTheSigstoreEndpointsAreReportedAsUnread(t *testing.T) {
	warnings := warningsFor(t, func(c *Config) {
		c.Attestation.Enabled = true
		c.Attestation.RekorURL = "https://rekor.sigstore.dev"
	})

	var found bool
	for _, w := range warnings {
		if strings.Contains(w, "rekor_url") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning for rekor_url, which nothing reads; warnings were %v", warnings)
	}
}

// `relicta promote` builds its channel registry from name, stability, tag_pattern, promotes_to
// and prerelease — not from require_approval or auto_approve, which nothing reads.
// ChannelDefinitionConfig.NeedsApproval() exists to answer the question and has no caller.
//
// Warned rather than refused: promote has no approval step at all to attach them to, so this is
// a feature that has not been built rather than a setting wired to the wrong place.
func TestPerChannelApprovalSettingsAreReportedAsUnread(t *testing.T) {
	requireApproval := true

	warnings := warningsFor(t, func(c *Config) {
		c.Channels.Enabled = true
		c.Channels.Definitions = []ChannelDefinitionConfig{
			{Name: "beta", RequireApproval: &requireApproval},
		}
	})

	var found string
	for _, w := range warnings {
		if strings.Contains(w, "require_approval") {
			found = w
		}
	}
	if found == "" {
		t.Fatalf("no warning for a per-channel approval rule nothing enforces; warnings were %v",
			warnings)
	}
	if !strings.Contains(found, "beta") {
		t.Errorf("the warning does not name the channel: %q", found)
	}

	// A channel that configures neither says nothing.
	for _, w := range warningsFor(t, func(c *Config) {
		c.Channels.Enabled = true
		c.Channels.Definitions = []ChannelDefinitionConfig{{Name: "stable"}}
	}) {
		if strings.Contains(w, "require_approval") {
			t.Errorf("warned about a channel that configured no approval rule: %q", w)
		}
	}
}
