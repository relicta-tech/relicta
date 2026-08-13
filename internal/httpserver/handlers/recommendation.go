package handlers

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// GetReleaseRecommendation serves the deterministic recommendation artifact
// (ADR-009) recorded for a release run.
//
// ADR-009 says every interface returns the same artifact — "CLI JSON output, MCP
// tool results, HTTP API" — and this API returned none, so a Hub reading over HTTP
// received a different shape than an agent reading MCP, for the same release.
//
// The artifact is read back from storage rather than rebuilt from the run. A
// reconstruction would be lossy in a way the shape hides: risk factors and
// required actions are not persisted on a run, and Assessment.Factors serializes
// as `[]` rather than being omitted, so "no factors were computed" would be
// served as "no factors exist". Reading the stored bytes also means the artifact's
// InputsDigest still describes the content being served.
func GetReleaseRecommendation(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil || ctx.ReleaseServices.Repository == nil {
		writeError(w, r, http.StatusNotFound, ErrCodeReleaseNotFound,
			"release not found", "services not initialized")
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "missing release ID", nil)
		return
	}

	store, ok := ctx.ReleaseServices.Repository.(ports.RecommendationStore)
	if !ok {
		// A repository without artifact storage is a configuration this build can
		// reach, so it gets an explanation rather than a bare 404.
		writeError(w, r, http.StatusNotImplemented, ErrCodeInternal,
			"recommendation artifacts are not available",
			"the configured release store does not persist recommendation artifacts")
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal,
			"failed to get working directory", nil)
		return
	}

	// Loaded first so a missing run and a run without an artifact are different
	// answers. Without this, an unknown ID and a release planned before artifacts
	// were stored would produce the same 404, and a Hub could not tell "no such
	// release" from "nothing recorded for this one".
	if _, err := loadRun(r.Context(), ctx.ReleaseServices.Repository, repoRoot, domain.RunID(runID)); err != nil {
		writeError(w, r, http.StatusNotFound, ErrCodeReleaseNotFound, "release not found", err.Error())
		return
	}

	artifact, found, err := store.LoadRecommendation(repoRoot, domain.RunID(runID))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal,
			"failed to read recommendation", err.Error())
		return
	}
	if !found {
		writeError(w, r, http.StatusNotFound, ErrCodeReleaseNotFound,
			"no recommendation recorded for this release",
			"the artifact is written when a release is planned; runs planned before "+
				"artifact storage existed, and runs created without it, have none")
		return
	}

	// Written straight through, not decoded and re-encoded. The artifact carries a
	// digest over its own content; re-marshaling could reorder or reformat it and
	// leave a consumer verifying a digest against different bytes.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(artifact); err != nil {
		// The status line is already sent, so this can only be logged.
		return
	}
}
