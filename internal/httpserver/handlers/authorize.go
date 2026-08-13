package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// ADR-012: relicta already receives deployment evidence, which detects an ungoverned
// deployment after it happened. This answers the question before it happens — a
// deployer asks whether a version may reach an environment, and relicta answers from
// what it governed.
//
// Detection is what you build when you cannot prevent. Until a deployer could ask,
// relicta could only report an ungoverned production deployment to whoever read the
// reconcile output later.
//
// Deliberately generic, like the deployment endpoint. This is not "the Rollops gate":
// the schema is documented and any deployer can call it — a GitOps controller, a CI
// step with curl, a script. Naming a product here would be the coupling ADR-012
// exists to prevent.

// releaseHistoryLimit bounds how far back the gate looks for a matching release.
//
// A deployment is of something released recently; a version thousands of releases old
// arriving in production now is itself worth refusing. The bound also stops one
// request from loading an entire history.
const releaseHistoryLimit = 500

// authorizationRequest is the documented wire schema, matching the fields a deployer
// can be expected to know about its own deployment.
type authorizationRequest struct {
	// Action is "apply" for a real request, or "probe" for a readiness check that
	// decides nothing.
	Action string `json:"action"`

	// Environment is where the version is going. The gate only refuses deployments to
	// the declared production environment.
	Environment string `json:"environment,omitempty"`

	// Version is what is being deployed. A production request without it is refused:
	// relicta cannot check what it is not told.
	Version string `json:"version,omitempty"`

	// TargetRef identifies the destination in the deployer's own terms, recorded as
	// evidence rather than used to decide.
	TargetRef string `json:"target_ref,omitempty"`

	ActorID   string `json:"actor_id,omitempty"`
	ActorKind string `json:"actor_kind,omitempty"`
}

// Authorize answers whether a version may be deployed to an environment.
//
// Always 200 with a decision when the question could be answered, so a caller
// distinguishes "governance said no" (200, allowed false) from "governance is broken"
// (5xx). A caller that fails closed treats both as a block, but only the first is a
// decision it should show a human as a policy answer.
func Authorize(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, deploymentEventLimit))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "failed to read request body", nil)
		return
	}

	// The same secret and header as the deployment endpoint: a deployer already
	// signing its evidence should not need a second credential to ask a question.
	if !verifyDeploymentSignature(r, body) {
		writeError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized,
			"invalid or missing signature",
			"sign the request body with HMAC-SHA256 and send it as X-Relicta-Signature")
		return
	}

	var req authorizationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "invalid authorization request", err.Error())
		return
	}

	store, repository, err := deploymentStoreForRequest(r)
	if err != nil {
		// Deliberately an error rather than a permissive decision. A caller cannot tell
		// an allow-because-broken from an allow-because-governed, so answering "allowed"
		// when the store is unreadable would silently disable the gate.
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeInternal,
			"the governance store could not be read, so no decision can be made", err.Error())
		return
	}

	reader, ok := store.(memory.Store)
	if !ok {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeInternal,
			"the governance store cannot read release history", "store does not implement memory.Store")
		return
	}

	releases, err := reader.GetReleaseHistory(r.Context(), repository, releaseHistoryLimit)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeInternal,
			"release history could not be read, so no decision can be made", err.Error())
		return
	}

	decision := memory.Authorize(memory.AuthorizationRequest{
		Action:      strings.TrimSpace(req.Action),
		Environment: req.Environment,
		Version:     req.Version,
		TargetRef:   req.TargetRef,
		Actor:       authorizationActor(req),
	}, releases, productionEnvironmentForRequest())

	respondJSON(w, http.StatusOK, decision)
}

// productionEnvironmentForRequest reports which environment counts as production.
//
// Read from the repository's own config, the same declaration the CLI and the DORA
// report use, so all three agree on what "production" means. Two components deciding
// that separately would eventually disagree, and the disagreement would show up as a
// gate that refused what a report called delivered.
//
// An unreadable or silent config yields "", which leaves the gate inactive rather
// than guessing. Guessing would either refuse every environment or none, and both are
// wrong in a way an operator cannot see from the outside.
func productionEnvironmentForRequest() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	cfg, err := config.LoadFromDirectory(dir)
	if err != nil || cfg == nil {
		return ""
	}
	for _, env := range cfg.Environments {
		if env.Production {
			return env.Name
		}
	}
	return ""
}

// authorizationActor attributes the request, defaulting to a system actor because a
// deployment reaching this endpoint was performed by a machine unless it says
// otherwise.
func authorizationActor(req authorizationRequest) cgp.Actor {
	kind := cgp.ActorKind(strings.TrimSpace(req.ActorKind))
	if kind == "" {
		kind = cgp.ActorKindSystem
	}
	id := req.ActorID
	if id == "" {
		id = "system:deployer"
	}
	return cgp.Actor{Kind: kind, ID: id}
}
