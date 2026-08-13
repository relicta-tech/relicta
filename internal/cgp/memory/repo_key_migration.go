package memory

import (
	"context"
	"path/filepath"
	"strings"
)

// Governance records were keyed by the repository's absolute checkout path until
// the identity was made canonical (remote-derived "owner/repo", or "local:<name>"
// without a remote). Records written before that are keyed by a path and are
// invisible to every read under the new identity.
//
// Dropping them silently is the wrong behavior for an audit trail: the store would
// look healthy and simply contain nothing about this repository's past, which is
// the same failure the canonical identity was introduced to fix. So a read that
// finds nothing under the canonical key looks once for a legacy path-keyed entry
// and adopts it.
//
// Adoption rather than a copy: the records move to the canonical key, so the
// migration happens once and the legacy key stops shadowing anything.

// legacyPathKey reports whether a stored key looks like a filesystem path rather
// than a governance identity.
//
// Canonical identities are "owner/repo" or "local:name". A legacy key is an
// absolute path, which is distinguishable by its leading separator — and must be,
// because "owner/repo" also contains a slash. Windows-style volume prefixes are
// included since a store written on one platform can be read on another.
func legacyPathKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "local:") {
		return false
	}
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, `\\`) {
		return true
	}
	// C:\path or C:/path
	if len(key) >= 3 && key[1] == ':' && (key[2] == '/' || key[2] == '\\') {
		return true
	}
	return false
}

// AdoptLegacyRepositoryKey moves records stored under a filesystem path to the
// canonical identity, when the path's last segment matches the repository the
// caller is asking about.
//
// Matching on the directory name rather than adopting any path is deliberate: a
// store may hold records for several repositories, and adopting the wrong one would
// attribute another repository's release history — and therefore its risk
// calibration and actor reputation — to this one. A wrong history is worse than a
// missing one, because nothing downstream can tell it is wrong.
//
// Returns the number of records adopted.
func (s *FileStore) AdoptLegacyRepositoryKey(_ context.Context, canonicalID, checkoutPath string) (int, error) {
	if canonicalID == "" || checkoutPath == "" {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.releases[canonicalID]) > 0 {
		// Already migrated, or genuinely has history. Either way there is nothing to
		// adopt, and overwriting would be destructive.
		return 0, nil
	}

	want := filepath.Base(checkoutPath)
	adopted := 0
	for key, records := range s.releases {
		if !legacyPathKey(key) || filepath.Base(key) != want || len(records) == 0 {
			continue
		}
		s.releases[canonicalID] = append(s.releases[canonicalID], records...)
		delete(s.releases, key)
		adopted += len(records)
	}

	if adopted == 0 {
		return 0, nil
	}

	// Persisted while the lock is held, matching how every other mutation in this
	// store saves. An adoption held only in memory would be redone by every
	// process, and a crash between adopting and saving would lose the records it
	// had just taken ownership of.
	if err := s.save(); err != nil {
		return adopted, err
	}
	return adopted, nil
}
