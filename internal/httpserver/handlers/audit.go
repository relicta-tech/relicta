package handlers

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

// ListAuditEvents returns the audit trail of release events.
func ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.AuditEventDTO]{
			Data:       []dto.AuditEventDTO{},
			Total:      0,
			Page:       1,
			PageSize:   100,
			TotalPages: 0,
		})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	// Time range filters
	var fromTime, toTime time.Time
	if fromStr := query.Get("from"); fromStr != "" {
		fromTime, _ = time.Parse(time.RFC3339, fromStr)
	}
	if toStr := query.Get("to"); toStr != "" {
		toTime, _ = time.Parse(time.RFC3339, toStr)
	}

	// Other filters
	releaseIDFilter := query.Get("release_id")
	eventTypeFilter := query.Get("event_type")
	actorFilter := query.Get("actor")

	// Pagination
	limit := 100
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}
	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// List all runs
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Collect all events from all runs
	var allEvents []dto.AuditEventDTO
	eventCounter := 0

	for _, runID := range runIDs {
		// Apply release ID filter if specified
		if releaseIDFilter != "" && string(runID) != releaseIDFilter {
			continue
		}

		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue
		}

		// Add events from transition history
		for _, tr := range run.History() {
			// Apply time filters
			if !fromTime.IsZero() && tr.At.Before(fromTime) {
				continue
			}
			if !toTime.IsZero() && tr.At.After(toTime) {
				continue
			}

			// Apply event type filter
			if eventTypeFilter != "" && tr.Event != eventTypeFilter {
				continue
			}

			// Apply actor filter
			if actorFilter != "" && tr.Actor != actorFilter {
				continue
			}

			eventCounter++
			allEvents = append(allEvents, dto.AuditEventDTO{
				ID:        string(run.ID()) + "-" + strconv.Itoa(eventCounter),
				Type:      tr.Event,
				ReleaseID: string(run.ID()),
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
	}

	// Sort events by timestamp descending (newest first)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.After(allEvents[j].Timestamp)
	})

	total := len(allEvents)
	totalPages := (total + limit - 1) / limit

	// Apply pagination
	start := offset
	end := offset + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	page := (offset / limit) + 1

	respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.AuditEventDTO]{
		Data:       allEvents[start:end],
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	})
}
