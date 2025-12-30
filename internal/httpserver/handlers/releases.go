package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

// ListReleases returns a list of releases.
func ListReleases(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.ReleaseDTO]{
			Data:       []dto.ReleaseDTO{},
			Total:      0,
			Page:       1,
			PageSize:   20,
			TotalPages: 0,
		})
		return
	}

	// Get repository root from current working directory
	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// List all run IDs
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	total := len(runIDs)
	totalPages := (total + pageSize - 1) / pageSize

	// Apply pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// Load each run and convert to DTO
	releases := make([]dto.ReleaseDTO, 0, end-start)
	for _, runID := range runIDs[start:end] {
		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue // Skip runs that can't be loaded
		}
		releases = append(releases, mapReleaseToDTO(run))
	}

	respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.ReleaseDTO]{
		Data:       releases,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// GetActiveRelease returns the currently active release.
func GetActiveRelease(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, map[string]any{"release": nil})
		return
	}

	// Get repository root from current working directory
	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// Load the latest release
	run, err := ctx.ReleaseServices.Repository.LoadLatest(r.Context(), repoRoot)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{"release": nil})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"release": mapReleaseToDTO(run),
	})
}

// GetRelease returns a specific release by ID.
func GetRelease(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondError(w, http.StatusNotFound, "release not found", "services not initialized")
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		respondError(w, http.StatusBadRequest, "missing release ID", "")
		return
	}

	run, err := ctx.ReleaseServices.Repository.Load(r.Context(), domain.RunID(runID))
	if err != nil {
		respondError(w, http.StatusNotFound, "release not found", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, mapReleaseToDTO(run))
}

// GetReleaseEvents returns events for a specific release.
func GetReleaseEvents(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, map[string]any{"events": []any{}})
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		respondError(w, http.StatusBadRequest, "missing release ID", "")
		return
	}

	run, err := ctx.ReleaseServices.Repository.Load(r.Context(), domain.RunID(runID))
	if err != nil {
		respondError(w, http.StatusNotFound, "release not found", err.Error())
		return
	}

	// Map transition history to audit events
	events := make([]dto.AuditEventDTO, 0, len(run.History()))
	for i, tr := range run.History() {
		events = append(events, dto.AuditEventDTO{
			ID:        runID + "-" + strconv.Itoa(i),
			Type:      tr.Event,
			ReleaseID: runID,
			ActorID:   tr.Actor,
			Timestamp: tr.At,
			Data: map[string]any{
				"from":     string(tr.From),
				"to":       string(tr.To),
				"reason":   tr.Reason,
				"metadata": tr.Metadata,
			},
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"events": events,
	})
}

// mapReleaseToDTO converts a domain ReleaseRun to a ReleaseDTO.
func mapReleaseToDTO(run *domain.ReleaseRun) dto.ReleaseDTO {
	d := dto.ReleaseDTO{
		ID:          string(run.ID()),
		State:       string(run.State()),
		BaseRef:     run.BaseRef(),
		HeadRef:     string(run.HeadSHA()),
		RiskScore:   run.RiskScore(),
		CreatedAt:   run.CreatedAt(),
		UpdatedAt:   run.UpdatedAt(),
		CommitCount: len(run.Commits()),
	}

	// Set version information if available
	if run.VersionCurrent().String() != "" && run.VersionCurrent().String() != "0.0.0" {
		d.Version = run.VersionCurrent().String()
	}
	if run.VersionNext().String() != "" && run.VersionNext().String() != "0.0.0" {
		d.NextVersion = run.VersionNext().String()
	}
	if run.BumpKind() != "" {
		d.BumpType = string(run.BumpKind())
	}

	// Set risk level based on score
	d.RiskLevel = getRiskLevel(run.RiskScore())

	// Set approval information
	if approval := run.Approval(); approval != nil {
		d.ApprovedAt = &approval.ApprovedAt
		d.ApprovedBy = approval.ApprovedBy
	}

	// Set published time
	d.PublishedAt = run.PublishedAt()

	// Set release notes
	if notes := run.Notes(); notes != nil {
		d.ReleaseNotes = notes.Text
	}

	// Get change types from reasons
	d.ChangeTypes = run.Reasons()

	return d
}

// getRiskLevel returns a human-readable risk level from a score.
func getRiskLevel(score float64) string {
	switch {
	case score >= 0.7:
		return "high"
	case score >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

// respondJSON writes a JSON response.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// respondError writes an error response.
func respondError(w http.ResponseWriter, status int, message, details string) {
	resp := dto.ErrorResponse{
		Error:   message,
		Details: details,
	}
	if details != "" {
		resp.Details = details
	}
	respondJSON(w, status, resp)
}
