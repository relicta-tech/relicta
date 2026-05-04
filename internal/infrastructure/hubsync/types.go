// Package hubsync ships governance events from the CLI to a Relicta Hub
// instance with retry, idempotency, schema versioning, and an offline queue
// so events captured while Hub is unreachable replay automatically on the
// next sync invocation.
//
// Wire format mirrors what Hub's POST /api/v1/sync expects today
// (CGP version 1.0). When that schema bumps, both Hub and this package
// update their SupportedVersion sets in lockstep.
package hubsync

import "time"

// CGPVersion is the wire-format version this client emits in the
// X-CGP-Version header. Hub rejects unknown versions with 412.
const CGPVersion = "1.0"

// SyncResponse is the parsed shape of Hub's sync handler response.
// Status 202 → all accepted; 207 → partial; 412 → version mismatch.
type SyncResponse struct {
	Accepted   int               `json:"accepted"`
	Received   int               `json:"received"`
	Results    []SyncEventStatus `json:"results"`
	CGPVersion string            `json:"cgp_version"`
}

// SyncEventStatus mirrors Hub's per-event outcome reporting.
type SyncEventStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"` // accepted | duplicate | rejected | failed
	Error  string `json:"error,omitempty"`
}

// IsTerminal reports whether the status means "stop retrying this event".
// Accepted, duplicate, and rejected are all terminal — only "failed" warrants
// re-queueing.
func (s SyncEventStatus) IsTerminal() bool {
	switch s.Status {
	case "accepted", "duplicate", "rejected":
		return true
	}
	return false
}

// QueueEntry is the on-disk record format for offline event queueing. One
// JSON object per line in the queue file. Reserved fields:
//
//	enqueued_at — wall-clock when this entry hit the queue (for TTL pruning)
//	attempts    — how many times we've tried to ship it
//	last_error  — most recent transport error (debugging aid)
//	payload     — the raw JSON the CLI built; opaque to the queue layer
type QueueEntry struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	EnqueuedAt time.Time `json:"enqueued_at"`
	Attempts   int       `json:"attempts"`
	LastError  string    `json:"last_error,omitempty"`
	Payload    []byte    `json:"payload"`
}
