package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SLSA attestation was dead at two levels, and each half hid the other.
//
//  1. ApproveReleaseUseCase.SetAttestationEnabled was never called, so the attestation step
//     was never added to the execution plan and the publisher was never asked to run one.
//  2. WithAttestationConfig was never called, so PublisherAdapter.attestationConfig was nil
//     and executeAttestationStep returned "Attestation generation skipped (not enabled)" —
//     with Success: true.
//
// So a user who set `attestation: enabled: true` got a release that reported success and
// produced nothing, and `relicta verify`, whose entire purpose is checking an attestation, had
// nothing to check. Fixing either half alone changes nothing observable, which is why this
// asserts both.
//
// Verified end to end before and after: a full publish with attestation enabled wrote no file
// and said nothing about attestation; it now writes attestation.intoto.jsonl and `relicta
// verify` reads the version, decision and approval count out of it.
//
// Read from the construction rather than by running a release: exercising it needs a
// repository, an approval and a publish, and what was broken is whether these two calls
// happen at all.

func TestAttestationIsWiredAtBothLevels(t *testing.T) {
	for _, w := range []struct {
		file string
		call string
		why  string
	}{
		{
			file: "container.go",
			call: "WithAttestationConfig(",
			why: "the publisher has no attestation config, so the step it runs reports " +
				"\"skipped (not enabled)\" and succeeds",
		},
		{
			file: "container.go",
			call: "AttestationEnabled:",
			why: "the release services never learn attestation is on, so approval does not " +
				"plan the step and the publisher is never asked to run one",
		},
		{
			file: "container.go",
			call: "WithAuditChain(",
			why: "the attestation generator takes the audit chain; without it the " +
				"attestation carries no record of the decisions behind the release",
		},
	} {
		t.Run(w.call, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Clean(w.file))
			if err != nil {
				t.Fatalf("read %s: %v", w.file, err)
			}
			if !strings.Contains(string(source), w.call) {
				t.Errorf("%s never calls %s — %s", w.file, w.call, w.why)
			}
		})
	}
}

// The step is planned at approval, which is when the execution plan is fixed. Planning it
// later would mean an approved run whose plan does not match what publish executes.
func TestApprovalIsWhatPlansTheAttestationStep(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("../domain/release/factory.go"))
	if err != nil {
		t.Fatalf("read factory.go: %v", err)
	}

	if !strings.Contains(string(source), "SetAttestationEnabled(cfg.AttestationEnabled)") {
		t.Error("the approve use case is never told whether attestation is enabled, so " +
			"ensureAttestationStep never runs and no release plans an attestation")
	}
}
