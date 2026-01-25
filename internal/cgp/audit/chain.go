// Package audit provides cryptographic audit trail functionality for CGP.
//
// The audit chain maintains an immutable, verifiable record of all governance
// events. Each entry is linked to the previous via cryptographic hashing,
// enabling tamper detection and compliance verification.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// EventType defines the type of governance event.
type EventType string

// Event types as defined in CGP specification Section 9.
const (
	EventProposalReceived    EventType = "proposal.received"
	EventEvaluationCompleted EventType = "evaluation.completed"
	EventDecisionMade        EventType = "decision.made"
	EventApprovalRequested   EventType = "approval.requested"
	EventApprovalGranted     EventType = "approval.granted"
	EventApprovalDenied      EventType = "approval.denied"
	EventExecutionAuthorized EventType = "execution.authorized"
	EventExecutionStarted    EventType = "execution.started"
	EventExecutionCompleted  EventType = "execution.completed"
	EventExecutionFailed     EventType = "execution.failed"
	EventIncidentRecorded    EventType = "incident.recorded"
	EventRollbackInitiated   EventType = "rollback.initiated"
)

// Entry represents a single entry in the audit chain.
type Entry struct {
	// ID is a unique identifier for this entry.
	ID string `json:"id"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// EventType is the type of governance event.
	EventType EventType `json:"eventType"`

	// ProposalID links to the originating proposal.
	ProposalID string `json:"proposalId"`

	// ActorID identifies who triggered the event.
	ActorID string `json:"actorId"`

	// ActorKind is the type of actor (human, ci, agent, system).
	ActorKind cgp.ActorKind `json:"actorKind"`

	// Details contains event-specific data.
	Details map[string]any `json:"details,omitempty"`

	// PreviousHash is the hash of the previous entry (empty for genesis).
	PreviousHash string `json:"previousHash"`

	// Hash is the cryptographic hash of this entry.
	Hash string `json:"hash"`
}

// ComputeHash calculates the SHA-256 hash for this entry.
// The hash covers all fields except the Hash field itself.
func (e *Entry) ComputeHash() string {
	// Create a deterministic representation
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		e.ID,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		e.EventType,
		e.ProposalID,
		e.ActorID,
		e.ActorKind,
		e.PreviousHash,
	)

	// Include details if present
	if len(e.Details) > 0 {
		detailsJSON, _ := json.Marshal(e.Details)
		data += "|" + string(detailsJSON)
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Verify checks if the entry's hash is valid.
func (e *Entry) Verify() bool {
	return e.Hash == e.ComputeHash()
}

// Chain maintains an append-only, hash-linked audit trail.
type Chain struct {
	mu      sync.RWMutex
	entries []*Entry
	byID    map[string]*Entry
	// Index by proposalID for efficient querying
	byProposal map[string][]*Entry
}

// NewChain creates a new audit chain.
func NewChain() *Chain {
	return &Chain{
		entries:    make([]*Entry, 0),
		byID:       make(map[string]*Entry),
		byProposal: make(map[string][]*Entry),
	}
}

// ErrInvalidPreviousHash indicates the previous hash doesn't match.
var ErrInvalidPreviousHash = errors.New("previous hash does not match chain tail")

// ErrDuplicateEntry indicates an entry with this ID already exists.
var ErrDuplicateEntry = errors.New("entry with this ID already exists")

// ErrInvalidHash indicates the entry's hash is invalid.
var ErrInvalidHash = errors.New("entry hash is invalid")

// ErrChainCorrupted indicates the chain has been tampered with.
var ErrChainCorrupted = errors.New("audit chain integrity compromised")

// Append adds a new entry to the chain.
// It automatically sets the PreviousHash and computes the Hash.
func (c *Chain) Append(entry *Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for duplicate
	if _, exists := c.byID[entry.ID]; exists {
		return ErrDuplicateEntry
	}

	// Set previous hash from chain tail
	if len(c.entries) > 0 {
		entry.PreviousHash = c.entries[len(c.entries)-1].Hash
	} else {
		entry.PreviousHash = "" // Genesis entry
	}

	// Compute and set hash
	entry.Hash = entry.ComputeHash()

	// Add to chain
	c.entries = append(c.entries, entry)
	c.byID[entry.ID] = entry
	c.byProposal[entry.ProposalID] = append(c.byProposal[entry.ProposalID], entry)

	return nil
}

// Verify checks the integrity of the entire chain.
// Returns nil if valid, or an error describing the corruption.
func (c *Chain) Verify() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i, entry := range c.entries {
		// Verify hash
		if !entry.Verify() {
			return fmt.Errorf("%w: entry %d (%s) hash mismatch", ErrChainCorrupted, i, entry.ID)
		}

		// Verify chain linkage (skip genesis)
		if i > 0 {
			expectedPrevHash := c.entries[i-1].Hash
			if entry.PreviousHash != expectedPrevHash {
				return fmt.Errorf("%w: entry %d (%s) previous hash mismatch", ErrChainCorrupted, i, entry.ID)
			}
		}
	}

	return nil
}

// Get retrieves an entry by ID.
func (c *Chain) Get(id string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.byID[id]
	return entry, ok
}

// GetByProposal returns all entries for a proposal.
func (c *Chain) GetByProposal(proposalID string) []*Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entries := c.byProposal[proposalID]
	result := make([]*Entry, len(entries))
	copy(result, entries)
	return result
}

// List returns all entries in order.
func (c *Chain) List() []*Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Entry, len(c.entries))
	copy(result, c.entries)
	return result
}

// Len returns the number of entries.
func (c *Chain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// LastHash returns the hash of the last entry, or empty string if chain is empty.
func (c *Chain) LastHash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.entries) == 0 {
		return ""
	}
	return c.entries[len(c.entries)-1].Hash
}

// Query filters entries by criteria.
type Query struct {
	ProposalID string
	ActorID    string
	EventType  EventType
	From       time.Time
	To         time.Time
	Limit      int
	Offset     int
}

// Search returns entries matching the query.
func (c *Chain) Search(q Query) []*Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []*Entry

	for _, entry := range c.entries {
		// Apply filters
		if q.ProposalID != "" && entry.ProposalID != q.ProposalID {
			continue
		}
		if q.ActorID != "" && entry.ActorID != q.ActorID {
			continue
		}
		if q.EventType != "" && entry.EventType != q.EventType {
			continue
		}
		if !q.From.IsZero() && entry.Timestamp.Before(q.From) {
			continue
		}
		if !q.To.IsZero() && entry.Timestamp.After(q.To) {
			continue
		}

		results = append(results, entry)
	}

	// Apply offset
	if q.Offset > 0 {
		if q.Offset >= len(results) {
			return []*Entry{}
		}
		results = results[q.Offset:]
	}

	// Apply limit
	if q.Limit > 0 && len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return results
}

// EntryBuilder provides a fluent API for creating entries.
type EntryBuilder struct {
	entry *Entry
}

// NewEntry creates a new entry builder.
func NewEntry(id string, eventType EventType) *EntryBuilder {
	return &EntryBuilder{
		entry: &Entry{
			ID:        id,
			Timestamp: time.Now().UTC(),
			EventType: eventType,
			Details:   make(map[string]any),
		},
	}
}

// WithProposal sets the proposal ID.
func (b *EntryBuilder) WithProposal(proposalID string) *EntryBuilder {
	b.entry.ProposalID = proposalID
	return b
}

// WithActor sets the actor information.
func (b *EntryBuilder) WithActor(id string, kind cgp.ActorKind) *EntryBuilder {
	b.entry.ActorID = id
	b.entry.ActorKind = kind
	return b
}

// WithTimestamp overrides the timestamp.
func (b *EntryBuilder) WithTimestamp(t time.Time) *EntryBuilder {
	b.entry.Timestamp = t.UTC()
	return b
}

// WithDetail adds a detail key-value pair.
func (b *EntryBuilder) WithDetail(key string, value any) *EntryBuilder {
	b.entry.Details[key] = value
	return b
}

// WithDetails sets all details at once.
func (b *EntryBuilder) WithDetails(details map[string]any) *EntryBuilder {
	b.entry.Details = details
	return b
}

// Build returns the constructed entry.
func (b *EntryBuilder) Build() *Entry {
	return b.entry
}

// FromDecision creates an audit entry from a governance decision.
func FromDecision(decision *cgp.GovernanceDecision) *Entry {
	details := map[string]any{
		"decisionType": decision.Decision,
		"riskScore":    decision.RiskScore,
	}
	if len(decision.RiskFactors) > 0 {
		details["riskFactorCount"] = len(decision.RiskFactors)
	}
	if len(decision.Rationale) > 0 {
		details["rationale"] = decision.Rationale
	}

	return NewEntry(decision.ID+"_decision", EventDecisionMade).
		WithProposal(decision.ProposalID).
		WithTimestamp(decision.Timestamp).
		WithDetails(details).
		Build()
}

// FromAuthorization creates an audit entry from an execution authorization.
func FromAuthorization(auth *cgp.ExecutionAuthorization) *Entry {
	details := map[string]any{
		"version":      auth.Version,
		"allowedSteps": auth.AllowedSteps,
	}
	if !auth.ValidUntil.IsZero() {
		details["validUntil"] = auth.ValidUntil
	}

	return NewEntry(auth.ID+"_auth", EventExecutionAuthorized).
		WithProposal(auth.ProposalID).
		WithActor(auth.ApprovedBy.ID, auth.ApprovedBy.Kind).
		WithTimestamp(auth.ApprovedAt).
		WithDetails(details).
		Build()
}

// FromApproval creates an audit entry from an approval record.
func FromApproval(auth *cgp.ExecutionAuthorization, approval cgp.ApprovalRecord) *Entry {
	var eventType EventType
	switch approval.Action {
	case "approve":
		eventType = EventApprovalGranted
	case "reject":
		eventType = EventApprovalDenied
	default:
		eventType = EventApprovalRequested
	}

	details := map[string]any{
		"action": approval.Action,
	}
	if approval.Comment != "" {
		details["comment"] = approval.Comment
	}

	return NewEntry(auth.ID+"_approval_"+approval.Actor.ID, eventType).
		WithProposal(auth.ProposalID).
		WithActor(approval.Actor.ID, approval.Actor.Kind).
		WithTimestamp(approval.Timestamp).
		WithDetails(details).
		Build()
}
