package handlers

import (
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/httpserver/dto"
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
func ListActors(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.ActorDTO]{
			Data:       []dto.ActorDTO{},
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

	// List all runs to aggregate actor data
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Aggregate actor statistics
	actorMap := make(map[string]*actorStats)
	for _, runID := range runIDs {
		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue
		}

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

	// Sort by release count descending
	sort.Slice(actors, func(i, j int) bool {
		return actors[i].ReleaseCount > actors[j].ReleaseCount
	})

	respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.ActorDTO]{
		Data:       actors,
		Total:      len(actors),
		Page:       1,
		PageSize:   len(actors),
		TotalPages: 1,
	})
}

// GetActor returns details for a specific actor.
func GetActor(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondError(w, http.StatusNotFound, "actor not found", "services not initialized")
		return
	}

	actorID := chi.URLParam(r, "id")
	if actorID == "" {
		respondError(w, http.StatusBadRequest, "missing actor ID", "")
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// List all runs to find actor data
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Aggregate statistics for this specific actor
	var stats actorStats
	found := false
	for _, runID := range runIDs {
		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue
		}

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
		respondError(w, http.StatusNotFound, "actor not found", "")
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
