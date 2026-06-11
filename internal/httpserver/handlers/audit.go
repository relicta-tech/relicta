package handlers

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/httpserver/dto"
)

// ListAuditEvents returns a paginated, filterable audit trail of release events.
// Supports cursor-based pagination (?limit=N&cursor=<opaque>) and
// legacy offset pagination (?page=N&page_size=N).
// Filters: ?release_id=X&event_type=X&actor=X&from=RFC3339&to=RFC3339
func ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.CursorPaginatedResponse[dto.AuditEventDTO]{
			Data:  []dto.AuditEventDTO{},
			Limit: defaultLimit,
		})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to get working directory", nil)
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

	// List all runs
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "failed to list releases", err.Error())
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

	params := ParsePagination(r)
	respondJSON(w, http.StatusOK, Paginate(allEvents, params, r, w))
}
