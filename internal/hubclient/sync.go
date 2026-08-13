package hubclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	pubcgp "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

// Pushing the local governance record to Hub.
//
// Hub has held a /api/v1/sync endpoint since before this existed, commented "CLI pushes
// governance events", with nothing calling it. So Hub could authenticate a CLI, store events,
// aggregate releases, compute reputation and render a dashboard — over a database that was
// always empty. Every reader worked; nothing wrote.

// cgpWireVersion is the schema version Hub negotiates on. An unknown non-empty value is
// answered with 412 and the versions it does support, so a mismatch is a clear error rather
// than a silently mis-encoded event.
const cgpWireVersion = "1.0"

// Event is one governance event as Hub receives it.
//
// This mirrors Hub's domain.GovernanceEvent rather than sharing a type with it. The two
// products are separately released and depend on each other only through published module
// versions, so a shared struct would have to live in pkg/cgp and be adopted by Hub in the same
// breath — worth doing, and not worth doing halfway, since a third definition that nothing uses
// is worse than two that are honest about spanning a network.
//
// Drift shows up as a rejected event rather than a corrupted one: Hub requires id and org_id,
// and answers 207 with a per-event reason for anything it will not store.
type Event struct {
	ID        string         `json:"id"`
	OrgID     string         `json:"org_id"`
	RepoID    string         `json:"repo_id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Actor     pubcgp.Actor   `json:"actor"`
	Data      map[string]any `json:"data,omitempty"`
	AuditHash string         `json:"audit_hash,omitempty"`
}

// SyncResult is Hub's answer to a push.
type SyncResult struct {
	Accepted int `json:"accepted"`
	Received int `json:"received"`
	Results  []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	} `json:"results"`
	CGPVersion string `json:"cgp_version"`
}

// Rejected returns the events Hub declined, with its reasons.
//
// Surfaced rather than folded into a count, because "3 of 12 accepted" tells an operator
// nothing they can act on and Hub already explains each one.
func (r *SyncResult) Rejected() []string {
	var out []string
	for _, e := range r.Results {
		if e.Status != "accepted" {
			out = append(out, fmt.Sprintf("%s: %s (%s)", e.ID, e.Status, e.Error))
		}
	}
	return out
}

// SyncEvents pushes events to Hub with the stored token.
//
// Hub treats a repeated event id as accepted rather than as a conflict — SaveEvent is
// ON CONFLICT DO NOTHING — which is what makes this safe to run repeatedly. That property is
// the reason the ids below are derived from the record rather than generated per push: a
// generated id would insert a duplicate release on every sync.
func (c *Client) SyncEvents(ctx context.Context, token string, events []Event) (*SyncResult, error) {
	if len(events) == 0 {
		return &SyncResult{CGPVersion: cgpWireVersion}, nil
	}

	body, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("hub: encoding events: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/sync", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("hub: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CGP-Version", cgpWireVersion)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hub: %s is unreachable: %w", c.BaseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := readLimited(resp)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusMultiStatus:
		var result SyncResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("hub: unreadable sync response: %w", err)
		}
		return &result, nil
	case http.StatusPreconditionFailed:
		// Version negotiation failed. Reported as itself because the fix is upgrading one side,
		// not retrying.
		return nil, fmt.Errorf("hub: %s does not support CGP version %s: %s",
			c.BaseURL, cgpWireVersion, strings.TrimSpace(string(raw)))
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("hub: %s refused the token — run `relicta hub login`: %s",
			c.BaseURL, strings.TrimSpace(string(raw)))
	case http.StatusServiceUnavailable:
		// Hub returns this while draining, with Retry-After. Named so a caller can distinguish
		// "try again shortly" from "this will never work".
		return nil, fmt.Errorf("hub: %s is draining, retry shortly", c.BaseURL)
	default:
		return nil, fmt.Errorf("hub: %s answered %s: %s", c.BaseURL, resp.Status, strings.TrimSpace(string(raw)))
	}
}

// EventsFromReleases converts local release records into Hub events.
//
// Ids are derived from the record id and the event type, never generated. Hub is idempotent on
// id, so a derived id makes repeated syncs converge on the same rows while a generated one would
// add a duplicate release to somebody's governance record on every run — which is worse than
// not syncing, because the report built on top would be quietly wrong.
func EventsFromReleases(orgID string, records []*memory.ReleaseRecord) []Event {
	// Two events per record. Hub builds its release row from release.planned — that is
	// the only branch that reads the risk score, the commit count and the breaking
	// flag — and then completes it with the outcome. Sending the outcome alone leaves a
	// row with no risk data, which is most of what a governance dashboard is for.
	events := make([]Event, 0, 2*len(records))
	for _, rec := range records {
		if rec == nil || rec.ID == "" {
			continue
		}

		actor := pubcgp.Actor{Kind: string(rec.Actor.Kind), ID: rec.Actor.ID}

		// Hub keys the release row on release_id, so an event without one materializes a
		// row with an empty ID that no later event can find again.
		planData := map[string]any{
			"release_id":       rec.ID,
			"next_version":     rec.Version,
			"risk_score":       rec.RiskScore,
			"commit_count":     rec.FilesChanged,
			"has_breaking":     rec.BreakingChanges > 0,
			"breaking_changes": rec.BreakingChanges,
			"security_changes": rec.SecurityChanges,
			"files_changed":    rec.FilesChanged,
			"lines_changed":    rec.LinesChanged,
			"decision":         string(rec.Decision),
		}

		// Hub's lead time is PublishedAt - PlannedAt, and PlannedAt comes from this
		// event's timestamp. Dating it from the oldest commit in the release rather than
		// from when the release command ran is what makes that subtraction answer the
		// DORA question — how long a change waited to reach users — instead of timing
		// relicta's own execution. It is the same measurement relicta's local report
		// makes, so the two do not disagree about the same release.
		plannedAt := rec.FirstCommitAt
		if plannedAt.IsZero() {
			plannedAt = rec.ReleasedAt
		}
		planData["first_commit_at"] = rec.FirstCommitAt

		events = append(events, Event{
			ID:    rec.ID + ":release.planned",
			OrgID: orgID,
			// The governance identity, not the checkout path — the same key relicta's own
			// readers use, so a repository looks like one repository on both sides.
			RepoID:    rec.Repository,
			Type:      "release.planned",
			Timestamp: plannedAt,
			Actor:     actor,
			Data:      planData,
		})

		eventType := eventTypeFor(rec.Outcome)
		events = append(events, Event{
			ID:        rec.ID + ":" + eventType,
			OrgID:     orgID,
			RepoID:    rec.Repository,
			Type:      eventType,
			Timestamp: rec.ReleasedAt,
			Actor:     actor,
			Data: map[string]any{
				"release_id":       rec.ID,
				"version":          rec.Version,
				"outcome":          string(rec.Outcome),
				"decision":         string(rec.Decision),
				"risk_score":       rec.RiskScore,
				"breaking_changes": rec.BreakingChanges,
				"security_changes": rec.SecurityChanges,
				"files_changed":    rec.FilesChanged,
				"lines_changed":    rec.LinesChanged,
				"duration_seconds": rec.Duration.Seconds(),
			},
		})
	}
	return events
}

// eventTypeFor maps a local outcome to the event type Hub aggregates on.
//
// Hub materializes releases from these, so the mapping decides what its dashboard shows. A
// rollback reports as failed rather than published: the change reached users and was withdrawn,
// and calling it published would count it as a success in every rate derived from it.
func eventTypeFor(outcome memory.ReleaseOutcome) string {
	switch outcome {
	case memory.OutcomeSuccess:
		return "release.published"
	case memory.OutcomeCanceled:
		// Hub records this and excludes it from every rate, mirroring
		// ReleaseOutcome.CountsAsRelease here. Cancellations were skipped entirely while
		// Hub had no term for them: the default below is release.published, so sending one
		// would have reported a run nobody shipped as a successful release.
		return "release.canceled"
	case memory.OutcomeFailed, memory.OutcomeRollback:
		return "release.failed"
	case memory.OutcomePartial:
		// Partial reached users incompletely. Reported as failed rather than published for the
		// same reason as a rollback — the alternative flatters the number.
		return "release.failed"
	default:
		return "release.published"
	}
}
