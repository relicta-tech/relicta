package hubclient

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The stored token is a bearer credential for a paying customer's governance data. These tests
// are about the two ways that goes wrong on disk: written where others can read it, or read
// after someone else has made it readable.

func TestSavedTokenIsNotReadableByOthers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := os.UserConfigDir(); err != nil {
		t.Skipf("no config dir on this platform: %v", err)
	}

	path, err := SaveToken(&StoredToken{
		Token: "jwt-value", HubURL: "https://hub.example",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("token written with mode %04o: any other account on the machine can read the "+
			"credential", perm)
	}
}

// Refused rather than read with a warning. A warning in output nobody reads is not a control,
// and the error names the one command that fixes it.
func TestLoadingRefusesAWorldReadableToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if _, err := os.UserConfigDir(); err != nil {
		t.Skipf("no config dir on this platform: %v", err)
	}

	path, err := SaveToken(&StoredToken{Token: "jwt-value", HubURL: "https://hub.example"})
	if err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := LoadToken(); err == nil {
		t.Fatal("a world-readable token file was loaded: a credential others can read must not " +
			"be used as though it were private")
	}
	_ = filepath.Dir(path)
}

func TestASavedTokenRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := os.UserConfigDir(); err != nil {
		t.Skipf("no config dir on this platform: %v", err)
	}

	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	if _, err := SaveToken(&StoredToken{
		Token: "jwt-value", HubURL: "https://hub.example",
		OrgID: "org-1", UserID: "user-1", ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	got, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if got.Token != "jwt-value" || got.HubURL != "https://hub.example" || got.OrgID != "org-1" {
		t.Errorf("read back %+v, want the stored values", got)
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, expires)
	}
}

// Expiry is judged with a margin, so a release does not start with a token that dies mid-run —
// a call that fails halfway through publishing is worse than one refused up front.
func TestExpiryLeavesAMargin(t *testing.T) {
	now := time.Now()

	fresh := &StoredToken{Token: "x", ExpiresAt: now.Add(time.Hour)}
	if fresh.Expired(now) {
		t.Error("a token valid for another hour was reported expired")
	}

	// Inside the margin: still technically valid, and refused anyway.
	soon := &StoredToken{Token: "x", ExpiresAt: now.Add(30 * time.Second)}
	if !soon.Expired(now) {
		t.Errorf("a token expiring in 30s was accepted: the margin exists so a long operation " +
			"does not begin with a credential that dies partway through")
	}

	empty := &StoredToken{}
	if !empty.Expired(now) {
		t.Error("a token with no value must count as expired rather than as usable")
	}
}
