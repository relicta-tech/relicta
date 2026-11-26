package persistence

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
	"github.com/relicta-tech/relicta/v5/internal/cgp"
	"github.com/relicta-tech/relicta/v5/internal/cgp/identity"
	"github.com/relicta-tech/relicta/v5/internal/config"
	cgpsdk "github.com/relicta-tech/relicta/v5/pkg/cgp"
)

func TestOpenAuxiliaryStoresSelectFileBackend(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	identityPath := filepath.Join(t.TempDir(), "identities")
	identities, err := OpenIdentityRegistryStore(ctx, config.PersistenceConfig{}, root, identityPath)
	if err != nil {
		t.Fatalf("OpenIdentityRegistryStore(file): %v", err)
	}
	if identities.Backend != config.BackendFile || identities.Location != identityPath {
		t.Fatalf("identity backend = %#v", identities)
	}
	actor := &identity.ActorIdentity{ID: "release-bot@platform", Kind: cgp.ActorKindAgent, Organization: "acme", Team: "platform", TrustScore: 0.7}
	if err := identities.Store.Save(ctx, actor); err != nil {
		t.Fatalf("save file identity: %v", err)
	}

	analyticsPath := filepath.Join(t.TempDir(), "analytics")
	analyticsBackend, err := OpenAnalyticsStore(ctx, config.PersistenceConfig{}, root, analyticsPath)
	if err != nil {
		t.Fatalf("OpenAnalyticsStore(file): %v", err)
	}
	if analyticsBackend.Backend != config.BackendFile || analyticsBackend.Location != analyticsPath {
		t.Fatalf("analytics backend = %#v", analyticsBackend)
	}
	event := analytics.Event{ID: "event-1", Timestamp: time.Now(), Type: analytics.EventReleaseDuration, Payload: json.RawMessage(`{"duration_ms":100}`)}
	if err := analyticsBackend.Store.Append(ctx, event); err != nil {
		t.Fatalf("append file analytics event: %v", err)
	}

	protocolBackend, err := OpenCGPProtocolStore(ctx, config.PersistenceConfig{}, root)
	if err != nil {
		t.Fatalf("OpenCGPProtocolStore(file): %v", err)
	}
	if protocolBackend.Backend != config.BackendFile || protocolBackend.Location != root {
		t.Fatalf("CGP protocol backend = %#v", protocolBackend)
	}
	if err := protocolBackend.Store.SaveProposal(ctx, &cgpsdk.ChangeProposal{ID: "proposal-1"}); err != nil {
		t.Fatalf("save file proposal: %v", err)
	}

	tokenBackend, err := OpenTokenRevocationStore(ctx, config.PersistenceConfig{}, root)
	if err != nil {
		t.Fatalf("OpenTokenRevocationStore(file): %v", err)
	}
	defer tokenBackend.Closer.Close()
	if err := tokenBackend.Store.Revoke("session-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("revoke file-backend token: %v", err)
	}
}

func TestOpenAuxiliaryStoresSelectSQLiteBackend(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := config.PersistenceConfig{Backend: config.BackendSQLite}

	identities, err := OpenIdentityRegistryStore(ctx, cfg, root, "")
	if err != nil {
		t.Fatalf("OpenIdentityRegistryStore(sqlite): %v", err)
	}
	if identities.Backend != config.BackendSQLite || identities.Closer == nil {
		t.Fatalf("identity backend = %#v", identities)
	}
	if err := identities.Store.Save(ctx, &identity.ActorIdentity{ID: "ci@platform", Kind: cgp.ActorKindCI, Organization: "acme", Team: "platform", TrustScore: 0.8}); err != nil {
		t.Fatalf("save sqlite identity: %v", err)
	}
	if err := identities.Closer.Close(); err != nil {
		t.Fatalf("close identity backend: %v", err)
	}

	analyticsBackend, err := OpenAnalyticsStore(ctx, cfg, root, "")
	if err != nil {
		t.Fatalf("OpenAnalyticsStore(sqlite): %v", err)
	}
	if analyticsBackend.Backend != config.BackendSQLite || analyticsBackend.Closer == nil {
		t.Fatalf("analytics backend = %#v", analyticsBackend)
	}
	if err := analyticsBackend.Store.Append(ctx, analytics.Event{ID: "event-2", Timestamp: time.Now(), Type: analytics.EventPolicyDecision, Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("append sqlite analytics event: %v", err)
	}
	if err := analyticsBackend.Closer.Close(); err != nil {
		t.Fatalf("close analytics backend: %v", err)
	}

	protocolBackend, err := OpenCGPProtocolStore(ctx, cfg, root)
	if err != nil {
		t.Fatalf("OpenCGPProtocolStore(sqlite): %v", err)
	}
	if protocolBackend.Backend != config.BackendSQLite || protocolBackend.Closer == nil {
		t.Fatalf("CGP protocol backend = %#v", protocolBackend)
	}
	if err := protocolBackend.Store.SaveProposal(ctx, &cgpsdk.ChangeProposal{ID: "proposal-2"}); err != nil {
		t.Fatalf("save sqlite proposal: %v", err)
	}
	if err := protocolBackend.Closer.Close(); err != nil {
		t.Fatalf("close CGP protocol backend: %v", err)
	}

	tokenBackend, err := OpenTokenRevocationStore(ctx, cfg, root)
	if err != nil {
		t.Fatalf("OpenTokenRevocationStore(sqlite): %v", err)
	}
	if err := tokenBackend.Store.Revoke("sqlite-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("revoke sqlite token: %v", err)
	}
	if err := tokenBackend.Closer.Close(); err != nil {
		t.Fatalf("close token backend: %v", err)
	}
}

func TestOpenAuxiliaryStoresRejectMissingRoots(t *testing.T) {
	ctx := context.Background()
	if _, err := OpenIdentityRegistryStore(ctx, config.PersistenceConfig{Backend: config.BackendSQLite}, "", ""); err == nil {
		t.Fatal("OpenIdentityRegistryStore accepted an empty root")
	}
	if _, err := OpenAnalyticsStore(ctx, config.PersistenceConfig{Backend: config.BackendSQLite}, "", ""); err == nil {
		t.Fatal("OpenAnalyticsStore accepted an empty root")
	}
	if _, err := OpenCGPProtocolStore(ctx, config.PersistenceConfig{Backend: config.BackendSQLite}, ""); err == nil {
		t.Fatal("OpenCGPProtocolStore accepted an empty root")
	}
	if _, err := OpenTokenRevocationStore(ctx, config.PersistenceConfig{}, ""); err == nil {
		t.Fatal("OpenTokenRevocationStore accepted an empty root")
	}
}
