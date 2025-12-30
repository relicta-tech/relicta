package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/domain/release/app"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/httpserver/dto"
	"github.com/relicta-tech/relicta/internal/httpserver/middleware"
)

// ListPendingApprovals returns releases waiting for approval.
func ListPendingApprovals(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.ApprovalDTO]{
			Data:       []dto.ApprovalDTO{},
			Total:      0,
			Page:       1,
			PageSize:   20,
			TotalPages: 0,
		})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// Find runs in NotesReady state (awaiting approval)
	runs, err := ctx.ReleaseServices.Repository.FindByState(r.Context(), repoRoot, domain.StateNotesReady)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to find pending approvals", err.Error())
		return
	}

	approvals := make([]dto.ApprovalDTO, 0, len(runs))
	for _, run := range runs {
		approvalDTO := dto.ApprovalDTO{
			ReleaseID:      string(run.ID()),
			Version:        run.VersionNext().String(),
			RiskScore:      run.RiskScore(),
			RiskLevel:      getRiskLevel(run.RiskScore()),
			RequiresReview: run.RequiresApproval(),
			SubmittedAt:    run.CreatedAt(),
			SubmittedBy:    run.ActorID(),
			CommitCount:    len(run.Commits()),
			Changes:        run.Reasons(),
		}

		if run.RequiresApproval() {
			approvalDTO.ReviewReason = "Risk score exceeds auto-approve threshold"
		}

		approvals = append(approvals, approvalDTO)
	}

	respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.ApprovalDTO]{
		Data:       approvals,
		Total:      len(approvals),
		Page:       1,
		PageSize:   len(approvals),
		TotalPages: 1,
	})
}

// ApproveRequest represents the request body for approving a release.
type ApproveRequest struct {
	Justification string `json:"justification,omitempty"`
	Force         bool   `json:"force,omitempty"`
}

// ApproveRelease approves a pending release.
func ApproveRelease(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanApprove() {
		respondError(w, http.StatusForbidden, "insufficient permissions to approve releases", "")
		return
	}

	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil || ctx.ReleaseServices.ApproveRelease == nil {
		respondError(w, http.StatusServiceUnavailable, "approval service not available", "")
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		respondError(w, http.StatusBadRequest, "missing release ID", "")
		return
	}

	// Parse request body
	var req ApproveRequest
	if r.Body != nil {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&req) // Ignore errors, use defaults
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// Execute approval
	input := app.ApproveReleaseInput{
		RepoRoot: repoRoot,
		RunID:    domain.RunID(runID),
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   user.Name,
		},
		AutoApprove: false,
		Force:       req.Force,
	}

	output, err := ctx.ReleaseServices.ApproveRelease.Execute(r.Context(), input)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to approve release", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"approved":     output.Approved,
		"run_id":       string(output.RunID),
		"plan_hash":    output.PlanHash,
		"approved_by":  output.ApprovedBy,
		"version_next": output.VersionNext,
	})
}

// RejectRequest represents the request body for rejecting a release.
type RejectRequest struct {
	Reason string `json:"reason"`
}

// RejectRelease rejects a pending release.
func RejectRelease(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanApprove() {
		respondError(w, http.StatusForbidden, "insufficient permissions to reject releases", "")
		return
	}

	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondError(w, http.StatusServiceUnavailable, "release service not available", "")
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		respondError(w, http.StatusBadRequest, "missing release ID", "")
		return
	}

	// Parse request body
	var req RejectRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body", err.Error())
			return
		}
	}

	if req.Reason == "" {
		req.Reason = "Rejected via dashboard"
	}

	// Load and cancel the run
	run, err := ctx.ReleaseServices.Repository.Load(r.Context(), domain.RunID(runID))
	if err != nil {
		respondError(w, http.StatusNotFound, "release not found", err.Error())
		return
	}

	// Cancel the release
	if err := run.Cancel(req.Reason, user.Name); err != nil {
		respondError(w, http.StatusBadRequest, "failed to reject release", err.Error())
		return
	}

	// Save the updated run
	if err := ctx.ReleaseServices.Repository.Save(r.Context(), run); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save rejection", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"rejected":    true,
		"run_id":      runID,
		"reason":      req.Reason,
		"rejected_by": user.Name,
	})
}
