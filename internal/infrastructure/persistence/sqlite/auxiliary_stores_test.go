package sqlite_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
	"github.com/relicta-tech/relicta/v5/internal/cgp"
	"github.com/relicta-tech/relicta/v5/internal/cgp/identity"
	cgpprotocol "github.com/relicta-tech/relicta/v5/internal/cgp/protocol"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/sqlite"
	cgpsdk "github.com/relicta-tech/relicta/v5/pkg/cgp"
)

func TestSQLiteIdentityStoreRoundTripAndValidation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "relicta.db")
	store, err := sqlite.OpenIdentityStore(ctx, path)
	if err != nil {
		t.Fatalf("OpenIdentityStore: %v", err)
	}
	defer store.Close()

	if actors, err := store.LoadAll(ctx); err != nil || len(actors) != 0 {
		t.Fatalf("empty LoadAll = %v, %v; want no actors, nil", actors, err)
	}
	actor := &identity.ActorIdentity{
		ID: "alice@platform", Kind: cgp.ActorKindHuman, Organization: "acme", Team: "platform",
		TrustScore: 0.8, Capabilities: []identity.Capability{{Action: "approve", Scope: "minor"}},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := store.Save(ctx, actor); err != nil {
		t.Fatalf("Save: %v", err)
	}
	updated := *actor
	updated.TrustScore = 0.9
	if err := store.Save(ctx, &updated); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	actors, err := store.LoadAll(ctx)
	if err != nil || len(actors) != 1 || actors[0].TrustScore != 0.9 {
		t.Fatalf("LoadAll after upsert = %#v, %v", actors, err)
	}
	if err := store.Save(ctx, nil); err == nil {
		t.Fatal("Save(nil) succeeded")
	}
	if err := store.Save(ctx, &identity.ActorIdentity{ID: "bad"}); err == nil {
		t.Fatal("Save(invalid identity) succeeded")
	}
	if err := store.Delete(ctx, "missing@platform"); err == nil {
		t.Fatal("Delete(missing) succeeded")
	}
	if err := store.Delete(ctx, actor.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if actors, err := store.LoadAll(ctx); err != nil || len(actors) != 0 {
		t.Fatalf("LoadAll after delete = %v, %v", actors, err)
	}
}

func TestSQLiteAnalyticsStoreValidatesAndFilters(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenAnalyticsStore(ctx, filepath.Join(t.TempDir(), "relicta.db"))
	if err != nil {
		t.Fatalf("OpenAnalyticsStore: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Microsecond)
	first := analytics.Event{ID: "evt-1", Timestamp: now, Type: analytics.EventApprovalOutcome, ReleaseID: "release-1", Payload: json.RawMessage(`{"approved":true}`)}
	second := analytics.Event{ID: "evt-2", Timestamp: now.Add(time.Minute), Type: analytics.EventRiskEvaluation, ReleaseID: "release-2", Payload: json.RawMessage(`{"score":0.2}`)}
	for _, event := range []analytics.Event{first, second} {
		if err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append(%s): %v", event.ID, err)
		}
	}
	for _, invalid := range []analytics.Event{
		{},
		{ID: "bad-type", Timestamp: now, Type: analytics.EventType("other"), Payload: json.RawMessage(`{}`)},
		{ID: "bad-payload", Timestamp: now, Type: analytics.EventRiskEvaluation, Payload: json.RawMessage(`{`)},
	} {
		if err := store.Append(ctx, invalid); err == nil {
			t.Errorf("Append(%#v) succeeded", invalid)
		}
	}

	eventType := analytics.EventApprovalOutcome
	from, to := now.Add(-time.Second), now.Add(time.Second)
	got, err := store.Query(ctx, analytics.QueryFilter{From: &from, To: &to, EventType: &eventType, ReleaseID: "release-1"})
	if err != nil || len(got) != 1 || got[0].ID != first.ID || !got[0].Timestamp.Equal(now) {
		t.Fatalf("filtered Query = %#v, %v", got, err)
	}
	got, err = store.Query(ctx, analytics.QueryFilter{ReleaseID: "missing"})
	if err != nil || len(got) != 0 {
		t.Fatalf("empty Query = %#v, %v", got, err)
	}
}

func TestSQLiteCGPProtocolStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.OpenCGPProtocolStore(ctx, filepath.Join(t.TempDir(), "relicta.db"))
	if err != nil {
		t.Fatalf("OpenCGPProtocolStore: %v", err)
	}
	defer store.Close()

	proposal := &cgpsdk.ChangeProposal{ID: "proposal-1", Intent: cgpsdk.Intent{Summary: "Ship safer releases"}}
	if err := store.SaveProposal(ctx, proposal); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}
	loaded, err := store.GetProposal(ctx, proposal.ID)
	if err != nil || loaded.Intent.Summary != proposal.Intent.Summary {
		t.Fatalf("GetProposal = %#v, %v", loaded, err)
	}
	if _, err := store.GetProposal(ctx, "missing"); !errors.Is(err, cgpprotocol.ErrRecordNotFound) {
		t.Fatalf("GetProposal(missing) error = %v, want ErrRecordNotFound", err)
	}
	if err := store.SaveProposal(ctx, nil); !errors.Is(err, cgpprotocol.ErrInvalidID) {
		t.Fatalf("SaveProposal(nil) error = %v, want ErrInvalidID", err)
	}
	if err := store.SaveProposal(ctx, &cgpsdk.ChangeProposal{ID: "../escape"}); !errors.Is(err, cgpprotocol.ErrInvalidID) {
		t.Fatalf("SaveProposal(unsafe ID) error = %v, want ErrInvalidID", err)
	}

	decision := &cgpsdk.GovernanceDecision{ProposalID: proposal.ID, Decision: "approved", Rationale: []string{"reviewed"}}
	if err := store.SaveDecision(ctx, decision); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	if loaded, err := store.GetDecision(ctx, proposal.ID); err != nil || loaded.Decision != decision.Decision {
		t.Fatalf("GetDecision = %#v, %v", loaded, err)
	}
	authorization := &cgpsdk.ExecutionAuthorization{ProposalID: proposal.ID, DecisionID: "decision-1", Version: "1.2.3"}
	if err := store.SaveAuthorization(ctx, authorization); err != nil {
		t.Fatalf("SaveAuthorization: %v", err)
	}
	if loaded, err := store.GetAuthorization(ctx, proposal.ID); err != nil || loaded.Version != authorization.Version {
		t.Fatalf("GetAuthorization = %#v, %v", loaded, err)
	}
	ids, err := store.ListProposals(ctx)
	if err != nil || len(ids) != 1 || ids[0] != proposal.ID {
		t.Fatalf("ListProposals = %v, %v", ids, err)
	}
}
