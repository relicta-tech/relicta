package sourcecontrol

import (
	"path"
	"strings"
)

// GovernanceID returns the stable identity used to key governance records for
// this repository.
//
// The CGP memory store — which feeds `relicta history`, risk trends, calibration,
// reputation and earned trust — was written under one identity and read under
// others, so records accumulated and were never found. On a repository whose
// remote is https://github.com/acme/widget.git, taken through a complete release:
//
//	stored by publish:   /private/var/folders/.../T/tmp.6fPqrJakiQ   (absolute path)
//	queried by history:  acme/tmp.6fPqrJakiQ                         (owner + directory)
//	correct identity:    acme/widget
//
// Three identifiers for one repository, and the third malformed: RepositoryInfo.Owner
// comes from the remote while RepositoryInfo.Name is the directory's last path
// segment, so composing them mixes two sources. `relicta history` was empty in
// every repository, and earned trust could never escalate an actor because it
// reads history that no write had ever been keyed to.
//
// Remote-derived rather than path-derived, because the identity has to survive a
// second clone, a different CI workspace and a moved directory. Keying on the path
// meant calibration and reputation silently reset to zero whenever any of those
// changed — the store would look fine and simply contain nothing about this
// repository.
//
// The "local:" prefix on the fallback is deliberate: it cannot be mistaken for an
// owner/repo pair, so a record keyed locally is visibly local rather than looking
// like a repository named after somebody's directory.
func (r *RepositoryInfo) GovernanceID() string {
	if r == nil {
		return ""
	}

	if id := governanceIDFromRemote(r.RemoteURL); id != "" {
		return id
	}

	name := r.Name
	if name == "" {
		name = path.Base(r.Path)
	}
	if name == "" || name == "." || name == "/" {
		return ""
	}
	return "local:" + name
}

// GovernanceIDFromRemote normalizes a git remote URL to the governance identity.
//
// Exported because records are written from more than one place. The outcome tracker
// sees only domain events, which carry a raw remote URL, while every reader queries
// by governance ID — so without a shared normalizer the two ends key the same
// repository differently and the records are never found. That is the bug this
// identity exists to prevent, reintroduced one caller at a time.
//
// Returns "" when the URL yields no owner/repo pair, so a caller can fall back rather
// than keying records under a fragment.
func GovernanceIDFromRemote(remoteURL string) string {
	return governanceIDFromRemote(remoteURL)
}

// governanceIDFromRemote normalizes a git remote URL to "owner/repo".
//
// Both forms have to reduce to the same string, or the same repository would be
// keyed twice depending on how it happened to be cloned:
//
//	https://github.com/acme/widget.git  -> acme/widget
//	git@github.com:acme/widget.git      -> acme/widget
//	ssh://git@host:2222/acme/widget     -> acme/widget
//
// Returns "" when the URL yields no owner/repo pair, so the caller falls back
// rather than keying records under a fragment.
func governanceIDFromRemote(remoteURL string) string {
	url := strings.TrimSpace(remoteURL)
	if url == "" {
		return ""
	}

	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")

	// scp-style: git@host:owner/repo. Split on the last colon so a port in an
	// ssh:// URL is not mistaken for the scp separator.
	if !strings.Contains(url, "://") {
		if idx := strings.LastIndex(url, ":"); idx >= 0 {
			url = url[idx+1:]
		}
	} else {
		// scheme://[user@]host[:port]/owner/repo
		if idx := strings.Index(url, "://"); idx >= 0 {
			url = url[idx+3:]
		}
		if idx := strings.Index(url, "/"); idx >= 0 {
			url = url[idx+1:]
		} else {
			return ""
		}
	}

	url = strings.Trim(url, "/")
	if url == "" {
		return ""
	}

	// Keep the last two segments. Self-hosted forges nest groups
	// (gitlab.example.com/group/subgroup/widget); the trailing pair is the part
	// that identifies the repository, and taking the whole path would key the same
	// repository differently from a shorter mirror of it.
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		owner, repo := parts[len(parts)-2], parts[len(parts)-1]
		if owner == "" || repo == "" {
			return ""
		}
		return owner + "/" + repo
	}
	return ""
}
