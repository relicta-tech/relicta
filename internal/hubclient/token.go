// Package hubclient talks to a Relicta Hub from the CLI.
//
// This is the CLI's first Hub client. Hub has had a /api/v1/sync endpoint whose comment says
// "CLI pushes governance events" since before this existed, and nothing in this repository
// called it — the same shape as the governance hook with no caller and the RBAC guard on no
// route. The device authorization grant is the piece worth building first, because without it
// every other Hub call would need a token pasted in by hand, which is what the grant exists to
// avoid.
package hubclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StoredToken is what a successful login leaves on disk.
type StoredToken struct {
	Token string `json:"token"`

	// HubURL is recorded alongside the token because a token is only valid for the Hub that
	// issued it. Without it, pointing the CLI at a second Hub would silently send the first
	// one's credential to it.
	HubURL string `json:"hub_url"`

	OrgID     string    `json:"org_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Expired reports whether the token is past its expiry, with a small margin.
//
// The margin exists so the CLI does not start a release with a token that expires mid-run: a
// call that fails halfway through publishing is worse than one refused up front.
func (t *StoredToken) Expired(now time.Time) bool {
	if t == nil || t.Token == "" {
		return true
	}
	return !now.Add(expiryMargin).Before(t.ExpiresAt)
}

const expiryMargin = 2 * time.Minute

// tokenFileMode is the only permission a bearer token may be stored with.
const tokenFileMode os.FileMode = 0o600

// TokenPath returns where the Hub token is stored.
func TokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("hub: locating the config directory: %w", err)
	}
	return filepath.Join(dir, "relicta", "hub-token.json"), nil
}

// SaveToken writes the token, readable only by its owner.
//
// Written to a temporary file and renamed, so an interrupted write cannot leave a truncated
// token that then reads as a corrupt credential rather than an absent one. The temporary file
// is created with the final mode, because a token that is briefly world-readable is
// world-readable.
func SaveToken(t *StoredToken) (string, error) {
	path, err := TokenPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("hub: creating the config directory: %w", err)
	}

	body, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", fmt.Errorf("hub: encoding the token: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, tokenFileMode); err != nil {
		return "", fmt.Errorf("hub: writing the token: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("hub: installing the token: %w", err)
	}
	return path, nil
}

// LoadToken reads the stored token.
//
// Refuses a token file that others can read, rather than reading it anyway with a warning.
// This is a bearer credential for a paying customer's governance data; a warning in a log is
// not a control, and the fix is one chmod the error names.
func LoadToken() (*StoredToken, error) {
	path, err := TokenPath()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf(
			"hub: %s is readable by other users (mode %04o); run: chmod 600 %s",
			path, perm, path)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t StoredToken
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("hub: %s is not a valid token file: %w", path, err)
	}
	return &t, nil
}
