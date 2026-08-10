package protocol

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cgpsdk "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

// The service defaulted to inMemoryStore and WithStore was called from nowhere
// outside tests, so every server process began empty and forgot the handshake when
// it exited. `cgp_status` answered "proposal not found" for a decision made in an
// earlier session — indistinguishable from an ID that never existed — and a
// governance decision made over the protocol left no durable evidence, which is
// the opposite of the property the audit trail exists for.

func newStore(t *testing.T) *FileProposalStore {
	t.Helper()
	return NewFileProposalStore(t.TempDir())
}

func sampleProposal(id string) *cgpsdk.ChangeProposal {
	return &cgpsdk.ChangeProposal{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeChangeProposal,
		ID:         id,
		Timestamp:  time.Unix(1_700_000_000, 0).UTC(),
		Actor:      cgpsdk.Actor{Kind: "human", ID: "human:dev"},
		Scope:      cgpsdk.Scope{Repository: "owner/repo", CommitRange: "HEAD~1..HEAD"},
		Intent:     cgpsdk.Intent{Summary: "a change"},
	}
}

// The property the whole change exists for: a record written by one process is
// readable by another. A store held in memory cannot satisfy this, and no unit
// test of the in-memory store would have noticed.
func TestRecordsSurviveANewStoreInstance(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	written := NewFileProposalStore(root)
	if err := written.SaveProposal(ctx, sampleProposal("prop_abc123")); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}

	// A separate instance over the same repository stands in for the next server
	// process.
	reread := NewFileProposalStore(root)
	got, err := reread.GetProposal(ctx, "prop_abc123")
	if err != nil {
		t.Fatalf("a proposal saved by an earlier process must be readable: %v", err)
	}
	if got.Intent.Summary != "a change" || got.Actor.ID != "human:dev" {
		t.Errorf("the proposal came back changed: %+v", got)
	}
}

func TestDecisionAndAuthorizationRoundTrip(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	decision := &cgpsdk.GovernanceDecision{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeGovernanceDecision,
		ID:         "dec_abc",
		ProposalID: "prop_abc123",
		Decision:   "approved",
		RiskScore:  0.25,
	}
	if err := store.SaveDecision(ctx, decision); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	gotDecision, err := store.GetDecision(ctx, "prop_abc123")
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if gotDecision.RiskScore != 0.25 || gotDecision.Decision != "approved" {
		t.Errorf("decision came back changed: %+v", gotDecision)
	}

	auth := &cgpsdk.ExecutionAuthorization{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeExecutionAuthorization,
		ID:         "auth_abc",
		ProposalID: "prop_abc123",
		DecisionID: "dec_abc",
	}
	if err := store.SaveAuthorization(ctx, auth); err != nil {
		t.Fatalf("SaveAuthorization: %v", err)
	}
	gotAuth, err := store.GetAuthorization(ctx, "prop_abc123")
	if err != nil {
		t.Fatalf("GetAuthorization: %v", err)
	}
	if gotAuth.DecisionID != "dec_abc" {
		t.Errorf("authorization came back changed: %+v", gotAuth)
	}
}

// A decision is keyed by the proposal it decides, and a proposal by its own ID.
// They share an ID space, so they must not share a file — the three kinds live in
// separate directories for exactly this reason.
func TestRecordKindsDoNotCollide(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	const id = "prop_shared"

	if err := store.SaveProposal(ctx, sampleProposal(id)); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}
	if err := store.SaveDecision(ctx, &cgpsdk.GovernanceDecision{
		ID: "dec_1", ProposalID: id, Decision: "approved", RiskScore: 0.9,
	}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	proposal, err := store.GetProposal(ctx, id)
	if err != nil {
		t.Fatalf("GetProposal after saving a decision under the same ID: %v", err)
	}
	if proposal.Intent.Summary != "a change" {
		t.Errorf("the decision overwrote the proposal: %+v", proposal)
	}
}

// Absence must be reported as not-found, which is what the service's callers
// already distinguish from a read failure.
func TestMissingRecordsReportNotFound(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	if _, err := store.GetProposal(ctx, "prop_missing"); err == nil {
		t.Error("expected an error for a proposal that was never saved")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("the error should say the record was not found; got %v", err)
	}

	if _, err := store.GetDecision(ctx, "prop_missing"); err == nil {
		t.Error("expected an error for a missing decision")
	}
	if _, err := store.GetAuthorization(ctx, "prop_missing"); err == nil {
		t.Error("expected an error for a missing authorization")
	}
}

// IDs arrive from MCP tool input, so they are caller-controlled. They are refused
// rather than sanitized: quietly rewriting "../../etc/passwd" into a plausible
// filename would store the record under a key the caller did not ask for, and the
// next read would miss it — a traversal attempt becoming silent data loss.
func TestHostileIdentifiersAreRefused(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	hostile := []string{
		"../../../etc/passwd",
		"..",
		"a/b",
		`a\b`,
		"",
		strings.Repeat("x", 200),
	}

	for _, id := range hostile {
		t.Run(id, func(t *testing.T) {
			err := store.SaveProposal(ctx, sampleProposal(id))
			if err == nil {
				t.Fatalf("saving with id %q must be refused", id)
			}
			if !errors.Is(err, ErrInvalidID) {
				t.Errorf("expected ErrInvalidID, got %v", err)
			}

			if _, err := store.GetProposal(ctx, id); !errors.Is(err, ErrInvalidID) {
				t.Errorf("reading id %q should also be refused, got %v", id, err)
			}
		})
	}
}

// Nothing may be written outside the store's own directory, whatever the ID.
func TestNothingIsWrittenOutsideTheStoreRoot(t *testing.T) {
	root := t.TempDir()
	store := NewFileProposalStore(root)

	// A sentinel above the store root: if a traversal succeeded it would land here.
	outside := filepath.Join(root, "..", "escaped.json")
	_ = os.Remove(outside)

	_ = store.SaveProposal(context.Background(), sampleProposal("../escaped"))

	if _, err := os.Stat(outside); err == nil {
		t.Error("a record was written above the store root")
	}
}

// A stored record that cannot be parsed is a distinct failure from absence, and
// must not be reported as "not found" — that would present corruption as an
// ordinary empty state.
func TestCorruptRecordIsNotReportedAsMissing(t *testing.T) {
	root := t.TempDir()
	store := NewFileProposalStore(root)
	ctx := context.Background()

	if err := store.SaveProposal(ctx, sampleProposal("prop_abc123")); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}

	path := filepath.Join(root, ".relicta", cgpDirName, proposalsDir, "prop_abc123.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("corrupt the record: %v", err)
	}

	_, err := store.GetProposal(ctx, "prop_abc123")
	if err == nil {
		t.Fatal("expected an error for an unparseable record")
	}
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("corruption must not read as absence; got %v", err)
	}
}

// The store satisfies the interface the service takes, which is how it gets used
// at all. Without this the default in-memory store stays in place and nothing
// says so.
func TestFileStoreSatisfiesProposalStore(t *testing.T) {
	var _ ProposalStore = NewFileProposalStore(t.TempDir())
}
