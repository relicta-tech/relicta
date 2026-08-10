package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/dto"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/middleware"
)

// ListPendingApprovals returns releases waiting for approval.
// Supports cursor-based pagination (?limit=N&cursor=<opaque>) and
// legacy offset pagination (?page=N&page_size=N).
// Sort: ?sort=risk|-risk|submitted|-submitted (default: -risk)
func ListPendingApprovals(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.CursorPaginatedResponse[dto.ApprovalDTO]{
			Data:  []dto.ApprovalDTO{},
			Limit: defaultLimit,
		})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
		return
	}

	// Find runs in NotesReady state (awaiting approval)
	runs, err := ctx.ReleaseServices.Repository.FindByState(r.Context(), repoRoot, domain.StateNotesReady)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to find pending approvals", err.Error())
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

	// Apply sort parameter
	sortApprovals(approvals, r.URL.Query().Get("sort"))

	params := ParsePagination(r)
	respondJSON(w, http.StatusOK, Paginate(approvals, params, r, w))
}

// sortApprovals sorts approvals by the given sort parameter.
// Supported values: risk, -risk, submitted, -submitted.
// A leading "-" indicates descending order. Default is "-risk" (highest risk first).
func sortApprovals(approvals []dto.ApprovalDTO, sortParam string) {
	if sortParam == "" {
		sortParam = "-risk"
	}

	desc := false
	if sortParam[0] == '-' {
		desc = true
		sortParam = sortParam[1:]
	}

	sort.Slice(approvals, func(i, j int) bool {
		var less bool
		switch sortParam {
		case "risk":
			less = approvals[i].RiskScore < approvals[j].RiskScore
		case "submitted":
			less = approvals[i].SubmittedAt.Before(approvals[j].SubmittedAt)
		default:
			less = approvals[i].RiskScore < approvals[j].RiskScore
		}
		if desc {
			return !less
		}
		return less
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
		writeError(w, r, http.StatusForbidden, ErrCodeForbidden, "insufficient permissions to approve releases", nil)
		return
	}

	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil || ctx.ReleaseServices.ApproveRelease == nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeServiceUnavailable, "approval service not available", nil)
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "missing release ID", nil)
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
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
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
		writeError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "failed to approve release", err.Error())
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
		writeError(w, r, http.StatusForbidden, ErrCodeForbidden, "insufficient permissions to reject releases", nil)
		return
	}

	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeServiceUnavailable, "release service not available", nil)
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "missing release ID", nil)
		return
	}

	// Parse request body
	var req RejectRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "invalid request body", err.Error())
			return
		}
	}

	if req.Reason == "" {
		req.Reason = "Rejected via dashboard"
	}

	// Load and cancel the run
	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
		return
	}

	run, err := loadRun(r.Context(), ctx.ReleaseServices.Repository, repoRoot, domain.RunID(runID))
	if err != nil {
		writeError(w, r, http.StatusNotFound, ErrCodeReleaseNotFound, "release not found", err.Error())
		return
	}

	// Cancel the release
	if err := run.Cancel(req.Reason, user.Name); err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "failed to reject release", err.Error())
		return
	}

	// Save the updated run
	if err := ctx.ReleaseServices.Repository.Save(r.Context(), run); err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to save rejection", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"rejected":    true,
		"run_id":      runID,
		"reason":      req.Reason,
		"rejected_by": user.Name,
	})
}
