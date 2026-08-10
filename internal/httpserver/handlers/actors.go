package handlers

import (
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/dto"
)

// actorStats tracks aggregated statistics for an actor.
type actorStats struct {
	Kind             string
	Name             string
	ReleaseCount     int
	SuccessCount     int
	TotalRiskScore   float64
	LastSeen         time.Time
	ReliabilityScore float64
}

// ListActors returns actor metrics and performance data.
// Supports cursor-based pagination (?limit=N&cursor=<opaque>) and
// legacy offset pagination (?page=N&page_size=N).
// Sort: ?sort=name|-name|releases|-releases|risk|-risk|reliability|-reliability (default: -releases)
func ListActors(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.CursorPaginatedResponse[dto.ActorDTO]{
			Data:  []dto.ActorDTO{},
			Limit: defaultLimit,
		})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
		return
	}

	// List all runs to aggregate actor data
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to list releases", err.Error())
		return
	}

	// Aggregate actor statistics
	actorMap := make(map[string]*actorStats)
	for _, run := range loadRuns(r.Context(), ctx.ReleaseServices.Repository, repoRoot, runIDs) {
		actorID := run.ActorID()
		if actorID == "" {
			actorID = "unknown"
		}

		stats, ok := actorMap[actorID]
		if !ok {
			stats = &actorStats{
				Kind: string(run.ActorType()),
				Name: actorID,
			}
			actorMap[actorID] = stats
		}

		stats.ReleaseCount++
		stats.TotalRiskScore += run.RiskScore()

		// Track success (published or approved states)
		if run.State() == domain.StatePublished {
			stats.SuccessCount++
		}

		// Update last seen
		if run.UpdatedAt().After(stats.LastSeen) {
			stats.LastSeen = run.UpdatedAt()
		}
	}

	// Convert to DTOs
	actors := make([]dto.ActorDTO, 0, len(actorMap))
	for id, stats := range actorMap {
		successRate := 0.0
		avgRiskScore := 0.0
		if stats.ReleaseCount > 0 {
			successRate = float64(stats.SuccessCount) / float64(stats.ReleaseCount)
			avgRiskScore = stats.TotalRiskScore / float64(stats.ReleaseCount)
		}

		// Calculate reliability score based on success rate and risk scores
		// Higher success rate and lower avg risk = higher reliability
		reliabilityScore := (successRate * 0.6) + ((1 - avgRiskScore) * 0.4)

		trustLevel := "standard"
		switch {
		case reliabilityScore >= 0.8:
			trustLevel = "trusted"
		case reliabilityScore < 0.5:
			trustLevel = "probation"
		}

		actors = append(actors, dto.ActorDTO{
			ID:               id,
			Kind:             stats.Kind,
			Name:             stats.Name,
			ReleaseCount:     stats.ReleaseCount,
			SuccessRate:      successRate,
			AverageRiskScore: avgRiskScore,
			ReliabilityScore: reliabilityScore,
			LastSeen:         stats.LastSeen,
			TrustLevel:       trustLevel,
		})
	}

	// Apply sort parameter
	sortActors(actors, r.URL.Query().Get("sort"))

	params := ParsePagination(r)
	respondJSON(w, http.StatusOK, Paginate(actors, params, r, w))
}

// GetActor returns details for a specific actor.
func GetActor(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		writeError(w, r, http.StatusNotFound, ErrCodeNotFound, "actor not found", "services not initialized")
		return
	}

	actorID := chi.URLParam(r, "id")
	if actorID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "missing actor ID", nil)
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
		return
	}

	// List all runs to find actor data
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to list releases", err.Error())
		return
	}

	// Aggregate statistics for this specific actor
	var stats actorStats
	found := false
	for _, run := range loadRuns(r.Context(), ctx.ReleaseServices.Repository, repoRoot, runIDs) {
		runActorID := run.ActorID()
		if runActorID == "" {
			runActorID = "unknown"
		}

		if runActorID != actorID {
			continue
		}

		if !found {
			stats.Kind = string(run.ActorType())
			stats.Name = actorID
			found = true
		}

		stats.ReleaseCount++
		stats.TotalRiskScore += run.RiskScore()

		if run.State() == domain.StatePublished {
			stats.SuccessCount++
		}

		if run.UpdatedAt().After(stats.LastSeen) {
			stats.LastSeen = run.UpdatedAt()
		}
	}

	if !found {
		writeError(w, r, http.StatusNotFound, ErrCodeNotFound, "actor not found", nil)
		return
	}

	successRate := 0.0
	avgRiskScore := 0.0
	if stats.ReleaseCount > 0 {
		successRate = float64(stats.SuccessCount) / float64(stats.ReleaseCount)
		avgRiskScore = stats.TotalRiskScore / float64(stats.ReleaseCount)
	}

	reliabilityScore := (successRate * 0.6) + ((1 - avgRiskScore) * 0.4)

	trustLevel := "standard"
	switch {
	case reliabilityScore >= 0.8:
		trustLevel = "trusted"
	case reliabilityScore < 0.5:
		trustLevel = "probation"
	}

	respondJSON(w, http.StatusOK, dto.ActorDTO{
		ID:               actorID,
		Kind:             stats.Kind,
		Name:             stats.Name,
		ReleaseCount:     stats.ReleaseCount,
		SuccessRate:      successRate,
		AverageRiskScore: avgRiskScore,
		ReliabilityScore: reliabilityScore,
		LastSeen:         stats.LastSeen,
		TrustLevel:       trustLevel,
	})
}

// sortActors sorts actors by the given sort parameter.
// Supported values: name, -name, releases, -releases, risk, -risk, reliability, -reliability.
// A leading "-" indicates descending order. Default is "-releases" (descending by release count).
func sortActors(actors []dto.ActorDTO, sortParam string) {
	if sortParam == "" {
		sortParam = "-releases"
	}

	desc := false
	if sortParam[0] == '-' {
		desc = true
		sortParam = sortParam[1:]
	}

	sort.Slice(actors, func(i, j int) bool {
		var less bool
		switch sortParam {
		case "name":
			less = actors[i].Name < actors[j].Name
		case "releases":
			less = actors[i].ReleaseCount < actors[j].ReleaseCount
		case "risk":
			less = actors[i].AverageRiskScore < actors[j].AverageRiskScore
		case "reliability":
			less = actors[i].ReliabilityScore < actors[j].ReliabilityScore
		default:
			less = actors[i].ReleaseCount < actors[j].ReleaseCount
		}
		if desc {
			return !less
		}
		return less
	})
}
