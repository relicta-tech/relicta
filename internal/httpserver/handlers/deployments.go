package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// ADR-012: deployment evidence is reported by whatever performed the deployment,
// because only the deployer knows the difference between a deployment being
// requested and one succeeding. A GitOps controller reconciling in a cluster cannot
// run the CLI and has no checkout, so it reports here instead.
//
// Deliberately generic. This is not "the Rollops webhook" — the schema is
// documented and any deployer can post it: a CI step with curl, a hand-rolled
// script, a different controller. Naming a product here would be the coupling the
// ADR exists to prevent, whether or not any code was imported.

// deploymentEventLimit bounds the request body. Deployment events are small; a
// large body is either a mistake or an attempt to exhaust memory on an endpoint
// that, by design, accepts requests from outside the release pipeline.
const deploymentEventLimit = 1 << 20 // 1 MiB

// deploymentEvent is the documented wire schema.
//
// Field names are snake_case to match the rest of this API, and every field a
// reporter cannot know is optional. The reporter is expected to know what it
// deployed, where, and how it went — nothing else is required.
type deploymentEvent struct {
	Environment string `json:"environment"`
	Version     string `json:"version"`

	// Outcome is succeeded, failed or rolled_back. Empty means succeeded, because a
	// reporter that troubles itself to send an event usually did so on success and
	// omitting the field should not silently become "unknown".
	Outcome string `json:"outcome,omitempty"`

	// DeployedAt is when the deployment completed. Empty means now, which is
	// accurate for a controller reporting its own sync as it finishes.
	DeployedAt string `json:"deployed_at,omitempty"`

	// DurationMS is how long it took, when the reporter measured it.
	DurationMS int64 `json:"duration_ms,omitempty"`

	// Reference points back at the reporter's own record — a rollout ID, a CI run
	// URL — so a reader can follow the claim to its source.
	Reference string `json:"reference,omitempty"`

	// ReleaseID links to the relicta release run, when the reporter knows it. Empty
	// is meaningful rather than missing: a deployment with no release is a version
	// that reached an environment without passing through governance.
	ReleaseID string `json:"release_id,omitempty"`

	// ActorID and ActorKind attribute the deployment. A controller is a system
	// actor; a pipeline is CI.
	ActorID   string `json:"actor_id,omitempty"`
	ActorKind string `json:"actor_kind,omitempty"`

	// Provenance says what observed this. Defaults to "reported", which is what
	// posting to this endpoint claims: the reporter is asserting it did the work.
	// A reporter deducing a deployment from desired state should say "inferred", so
	// an auditor can weigh a controller's own report differently from a guess.
	Provenance string `json:"provenance,omitempty"`

	// Metadata carries reporter-specific detail.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DeploymentWebhook records that a version reached an environment.
//
// The repository is resolved server-side rather than taken from the payload. A
// reporter authenticated for this server must not be able to write governance
// records attributed to a different repository, and the server already knows which
// repository it serves — accepting the field would add a way to be wrong with no
// way to be more right.
func DeploymentWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, deploymentEventLimit))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "failed to read request body", nil)
		return
	}

	if !verifyDeploymentSignature(r, body) {
		writeError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized,
			"invalid or missing signature",
			"sign the request body with HMAC-SHA256 and send it as X-Relicta-Signature")
		return
	}

	var event deploymentEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "invalid deployment event", err.Error())
		return
	}

	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeInternal,
			"deployment recording is not available", "services not initialized")
		return
	}

	record, err := deploymentRecordFrom(event)
	if err != nil {
		// A rejected event is reported rather than stored with a guess. These arrive
		// from outside, and an unrecognized outcome silently accepted would be counted
		// as neither a success nor a failure and would bias every rate derived from it.
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "invalid deployment event", err.Error())
		return
	}

	store, repository, err := deploymentStoreForRequest(r)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeInternal,
			"deployment recording is not available", err.Error())
		return
	}
	record.Repository = repository

	if err := store.RecordDeployment(r.Context(), record); err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON,
			"the deployment could not be recorded", err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]any{
		"recorded":    true,
		"id":          record.ID,
		"repository":  record.Repository,
		"environment": record.Environment,
		"version":     record.Version,
		"outcome":     string(record.Outcome),
	})
}

// deploymentRecordFrom converts a wire event into a record, applying defaults.
func deploymentRecordFrom(event deploymentEvent) (*memory.DeploymentRecord, error) {
	outcome := memory.DeploymentOutcome(strings.TrimSpace(event.Outcome))
	if outcome == "" {
		outcome = memory.DeploymentSucceeded
	}
	if !outcome.IsValid() {
		return nil, errUnknownField("outcome", event.Outcome, "succeeded, failed, rolled_back")
	}

	provenance := memory.DeploymentProvenance(strings.TrimSpace(event.Provenance))
	if provenance == "" {
		provenance = memory.ProvenanceReported
	}
	if !provenance.IsValid() {
		return nil, errUnknownField("provenance", event.Provenance, "reported, inferred, manual")
	}

	deployedAt := time.Now().UTC()
	if event.DeployedAt != "" {
		parsed, err := time.Parse(time.RFC3339, event.DeployedAt)
		if err != nil {
			return nil, errUnknownField("deployed_at", event.DeployedAt, "an RFC 3339 timestamp")
		}
		deployedAt = parsed.UTC()
	}

	actorKind := cgp.ActorKind(strings.TrimSpace(event.ActorKind))
	if actorKind == "" {
		// A deployment reported over this endpoint was performed by a machine unless
		// the reporter says otherwise.
		actorKind = cgp.ActorKindSystem
	}
	actorID := event.ActorID
	if actorID == "" {
		actorID = "system:deployer"
	}

	return &memory.DeploymentRecord{
		ID:          "deploy-" + event.Environment + "-" + event.Version + "-" + deployedAt.Format(time.RFC3339Nano),
		Environment: strings.TrimSpace(event.Environment),
		Version:     strings.TrimSpace(event.Version),
		Actor:       cgp.Actor{Kind: actorKind, ID: actorID},
		Outcome:     outcome,
		DeployedAt:  deployedAt,
		Duration:    time.Duration(event.DurationMS) * time.Millisecond,
		Provenance:  provenance,
		Reference:   event.Reference,
		ReleaseID:   event.ReleaseID,
		Metadata:    event.Metadata,
	}, nil
}

// verifyDeploymentSignature checks the HMAC signature when a secret is configured.
//
// When no secret is set the endpoint accepts unsigned requests, matching how the
// incident receiver behaves — but this is a write endpoint for governance evidence,
// so the absence of a secret is a deployment decision rather than a default to rely
// on. RELICTA_WEBHOOK_SECRET is read from the environment so the value never has to
// live in a config file that gets committed.
func verifyDeploymentSignature(r *http.Request, body []byte) bool {
	secret := os.Getenv("RELICTA_WEBHOOK_SECRET")
	if secret == "" {
		return true
	}

	signature := r.Header.Get("X-Relicta-Signature")
	if signature == "" {
		// Accepting a reporter's own header name costs nothing and saves every
		// integration a translation step.
		signature = r.Header.Get("X-Webhook-Signature")
	}
	if signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Compared with hmac.Equal rather than ==, so the comparison is constant time and
	// does not leak the expected value one byte at a time.
	for _, prefix := range []string{"sha256=", ""} {
		if hmac.Equal([]byte(expected), []byte(strings.TrimPrefix(signature, prefix))) {
			return true
		}
	}
	return false
}

// errUnknownField reports a rejected field value and names what is accepted, so a
// reporter can fix its payload without reading this source.
func errUnknownField(field, got, accepted string) error {
	return fmt.Errorf("%s %q is not recognized; accepted values are %s", field, got, accepted)
}

// deploymentStoreForRequest opens the governance store and resolves the repository
// this server is serving.
func deploymentStoreForRequest(r *http.Request) (memory.DeploymentStore, string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve the working directory: %w", err)
	}

	svc, err := gitservice.NewService(gitservice.WithRepoPath(repoRoot))
	if err != nil {
		return nil, "", fmt.Errorf("failed to open the repository: %w", err)
	}
	info, err := gitservice.NewAdapter(svc).GetInfo(r.Context())
	if err != nil {
		return nil, "", fmt.Errorf("failed to read repository info: %w", err)
	}

	repository := info.GovernanceID()
	if repository == "" {
		return nil, "", fmt.Errorf("could not determine the repository identity")
	}

	store, err := memory.NewFileStore(filepath.Dir(governance.MemoryStorePath("", info.Path)))
	if err != nil {
		return nil, "", fmt.Errorf("failed to open the governance store: %w", err)
	}
	return store, repository, nil
}
