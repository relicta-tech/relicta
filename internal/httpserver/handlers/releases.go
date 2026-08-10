package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/dto"
)

// ListReleases returns a paginated list of releases.
// Supports cursor-based pagination (?limit=N&cursor=<opaque>) and
// legacy offset pagination (?page=N&page_size=N).
// Optional filters: ?state=<state>
// Sort: ?sort=created|-created|risk|-risk|version|-version (default: -created)
func ListReleases(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.CursorPaginatedResponse[dto.ReleaseDTO]{
			Data:  []dto.ReleaseDTO{},
			Limit: defaultLimit,
		})
		return
	}

	// Get repository root from current working directory
	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
		return
	}

	// List all run IDs
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to list releases", err.Error())
		return
	}

	// Load all runs and convert to DTOs (applying optional state filter)
	stateFilter := r.URL.Query().Get("state")
	releases := make([]dto.ReleaseDTO, 0, len(runIDs))
	for _, run := range loadRuns(r.Context(), ctx.ReleaseServices.Repository, repoRoot, runIDs) {
		if stateFilter != "" && string(run.State()) != stateFilter {
			continue
		}
		releases = append(releases, mapReleaseToDTO(run))
	}

	// Apply sort parameter
	sortReleases(releases, r.URL.Query().Get("sort"))

	params := ParsePagination(r)
	respondJSON(w, http.StatusOK, Paginate(releases, params, r, w))
}

// sortReleases sorts releases by the given sort parameter.
// Supported values: created, -created, risk, -risk, version, -version.
// A leading "-" indicates descending order. Default is "-created" (newest first).
func sortReleases(releases []dto.ReleaseDTO, sortParam string) {
	if sortParam == "" {
		sortParam = "-created"
	}

	desc := false
	if sortParam[0] == '-' {
		desc = true
		sortParam = sortParam[1:]
	}

	sort.Slice(releases, func(i, j int) bool {
		var less bool
		switch sortParam {
		case "created":
			less = releases[i].CreatedAt.Before(releases[j].CreatedAt)
		case "risk":
			less = releases[i].RiskScore < releases[j].RiskScore
		case "version":
			less = releases[i].NextVersion < releases[j].NextVersion
		default:
			less = releases[i].CreatedAt.Before(releases[j].CreatedAt)
		}
		if desc {
			return !less
		}
		return less
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
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
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
		writeError(w, r, http.StatusNotFound, ErrCodeReleaseNotFound, "release not found", "services not initialized")
		return
	}

	runID := chi.URLParam(r, "id")
	if runID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "missing release ID", nil)
		return
	}

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
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "missing release ID", nil)
		return
	}

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
