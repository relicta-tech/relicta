package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// A GitOps controller cannot run the CLI and has no checkout, so it reports
// deployments here (ADR-012). These records arrive from outside the release
// pipeline, which is what most of these tests are about: what the endpoint accepts,
// what it refuses, and what it refuses to guess.

func TestDeploymentEventDefaults(t *testing.T) {
	before := time.Now().UTC()

	record, err := deploymentRecordFrom(deploymentEvent{
		Environment: "production",
		Version:     "1.2.0",
	})
	if err != nil {
		t.Fatalf("a minimal event must be accepted: %v", err)
	}

	// A reporter that troubles itself to send an event usually did so on success,
	// and omitting the field must not silently become "unknown".
	if record.Outcome != memory.DeploymentSucceeded {
		t.Errorf("Outcome = %q, want succeeded", record.Outcome)
	}
	// Posting here is itself the claim "I did this", so the default provenance says
	// so — and an inferring reporter has to say "inferred" to be weighed differently.
	if record.Provenance != memory.ProvenanceReported {
		t.Errorf("Provenance = %q, want reported", record.Provenance)
	}
	if record.DeployedAt.Before(before) {
		t.Errorf("DeployedAt = %v, want roughly now", record.DeployedAt)
	}
	if record.Actor.Kind != cgp.ActorKindSystem {
		t.Errorf("Actor.Kind = %q, want system: a deployment reported over HTTP was "+
			"performed by a machine unless the reporter says otherwise", record.Actor.Kind)
	}
}

// Refused rather than stored with a guess. An unrecognized outcome accepted
// silently would be counted as neither a success nor a failure and would bias every
// rate derived from it.
func TestDeploymentEventRejectsUnknownValues(t *testing.T) {
	cases := map[string]deploymentEvent{
		"outcome":     {Environment: "production", Version: "1.0.0", Outcome: "probably-fine"},
		"provenance":  {Environment: "production", Version: "1.0.0", Provenance: "vibes"},
		"deployed_at": {Environment: "production", Version: "1.0.0", DeployedAt: "yesterday"},
	}

	for field, event := range cases {
		t.Run(field, func(t *testing.T) {
			_, err := deploymentRecordFrom(event)
			if err == nil {
				t.Fatalf("an unrecognized %s must be refused", field)
			}
			// The message has to name what is accepted, or a reporter has to read this
			// source to fix its payload.
			if !strings.Contains(err.Error(), field) {
				t.Errorf("the error should name the field; got %v", err)
			}
		})
	}
}

func TestDeploymentEventCarriesReportedDetail(t *testing.T) {
	record, err := deploymentRecordFrom(deploymentEvent{
		Environment: "production",
		Version:     "1.2.0",
		Outcome:     "rolled_back",
		DeployedAt:  "2026-08-11T09:15:00Z",
		DurationMS:  252000,
		Reference:   "rollout-abc",
		ReleaseID:   "run-123",
		ActorID:     "system:controller",
		Provenance:  "inferred",
	})
	if err != nil {
		t.Fatalf("deploymentRecordFrom: %v", err)
	}

	if record.Outcome != memory.DeploymentRolledBack {
		t.Errorf("Outcome = %q", record.Outcome)
	}
	if record.Provenance != memory.ProvenanceInferred {
		t.Errorf("Provenance = %q: an inferred deployment must not be recorded as "+
			"reported, since an auditor weighs them differently", record.Provenance)
	}
	if record.Duration != 252*time.Second {
		t.Errorf("Duration = %v, want 4m12s", record.Duration)
	}
	if record.Reference != "rollout-abc" || record.ReleaseID != "run-123" {
		t.Errorf("the reporter's own references were lost: %+v", record)
	}
	if !record.DeployedAt.Equal(time.Date(2026, 8, 11, 9, 15, 0, 0, time.UTC)) {
		t.Errorf("DeployedAt = %v, want the reported time rather than now", record.DeployedAt)
	}
}

// The repository is resolved server-side. A reporter must not be able to write
// governance records attributed to a different repository, so the field is not part
// of the schema at all — there is nothing to spoof.
func TestDeploymentEventCannotNameARepository(t *testing.T) {
	record, err := deploymentRecordFrom(deploymentEvent{
		Environment: "production",
		Version:     "1.0.0",
		Metadata:    map[string]string{"repository": "someone-else/repo"},
	})
	if err != nil {
		t.Fatalf("deploymentRecordFrom: %v", err)
	}
	if record.Repository != "" {
		t.Errorf("Repository = %q; it must be set by the server, not the payload",
			record.Repository)
	}
}

func TestDeploymentSignatureVerification(t *testing.T) {
	const secret = "s3cret"
	body := []byte(`{"environment":"production","version":"1.0.0"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	valid := hex.EncodeToString(mac.Sum(nil))

	cases := []struct {
		name      string
		secret    string
		signature string
		want      bool
	}{
		{"no secret configured accepts unsigned", "", "", true},
		{"valid signature", secret, "sha256=" + valid, true},
		{"valid signature without prefix", secret, valid, true},
		{"wrong signature", secret, "sha256=deadbeef", false},
		// The case that matters most: a configured secret must not be satisfied by
		// omitting the header, or the protection is opt-out by the caller.
		{"missing signature with a secret configured", secret, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RELICTA_WEBHOOK_SECRET", tc.secret)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/deployments", nil)
			if tc.signature != "" {
				req.Header.Set("X-Relicta-Signature", tc.signature)
			}

			if got := verifyDeploymentSignature(req, body); got != tc.want {
				t.Errorf("verifyDeploymentSignature = %v, want %v", got, tc.want)
			}
		})
	}
}
