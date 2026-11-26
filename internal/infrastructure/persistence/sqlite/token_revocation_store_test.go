package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/infrastructure/persistence/sqlite"
)

func TestTokenRevocationStorePersistsAndCleansRevocations(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "relicta.db")
	store, err := sqlite.OpenTokenRevocationStore(ctx, path)
	if err != nil {
		t.Fatalf("OpenTokenRevocationStore: %v", err)
	}

	activeExpiry := time.Now().Add(time.Hour)
	if err := store.Revoke("access-jti", activeExpiry); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked, err := store.IsRevoked("access-jti"); err != nil || !revoked {
		t.Fatalf("IsRevoked(active) = %v, %v; want true, nil", revoked, err)
	}
	if revoked, err := store.IsRevoked("unknown-jti"); err != nil || revoked {
		t.Fatalf("IsRevoked(unknown) = %v, %v; want false, nil", revoked, err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	store, err = sqlite.OpenTokenRevocationStore(ctx, path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store.Close()
	if revoked, err := store.IsRevoked("access-jti"); err != nil || !revoked {
		t.Fatalf("IsRevoked after reopen = %v, %v; want true, nil", revoked, err)
	}

	if err := store.Revoke("expired-jti", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("Revoke expired token: %v", err)
	}
	if err := store.CleanExpired(time.Now()); err != nil {
		t.Fatalf("CleanExpired: %v", err)
	}
	if revoked, err := store.IsRevoked("expired-jti"); err != nil || revoked {
		t.Fatalf("IsRevoked(expired) = %v, %v; want false, nil", revoked, err)
	}
	if revoked, err := store.IsRevoked("access-jti"); err != nil || !revoked {
		t.Fatalf("CleanExpired removed active token: %v, %v", revoked, err)
	}
}

func TestTokenRevocationStoreClaimsRefreshOnceAndRevokesPair(t *testing.T) {
	store, err := sqlite.OpenTokenRevocationStore(context.Background(), filepath.Join(t.TempDir(), "relicta.db"))
	if err != nil {
		t.Fatalf("OpenTokenRevocationStore: %v", err)
	}
	defer store.Close()

	expires := time.Now().Add(time.Hour)
	claimed, err := store.ClaimRefresh("refresh-jti", expires)
	if err != nil || !claimed {
		t.Fatalf("first ClaimRefresh = %v, %v; want true, nil", claimed, err)
	}
	claimed, err = store.ClaimRefresh("refresh-jti", expires)
	if err != nil || claimed {
		t.Fatalf("second ClaimRefresh = %v, %v; want false, nil", claimed, err)
	}

	if err := store.RevokePair("access-pair", expires, "refresh-pair", expires); err != nil {
		t.Fatalf("RevokePair: %v", err)
	}
	for _, jti := range []string{"access-pair", "refresh-pair"} {
		if revoked, err := store.IsRevoked(jti); err != nil || !revoked {
			t.Errorf("IsRevoked(%q) = %v, %v; want true, nil", jti, revoked, err)
		}
	}
}
