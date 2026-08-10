package sourcecontrol

import "testing"

// Governance records were written under an absolute path and read under
// "owner + directory name", so `relicta history` was empty in every repository and
// earned trust could never find the history it needs. These tests pin the single
// identity everything now uses.

func TestGovernanceIDFromRemote(t *testing.T) {
	cases := map[string]string{
		// The two clone forms must reduce to the same key, or one repository is
		// keyed twice depending on how it was cloned.
		"https://github.com/acme/widget.git": "acme/widget",
		"https://github.com/acme/widget":     "acme/widget",
		"git@github.com:acme/widget.git":     "acme/widget",
		"git@github.com:acme/widget":         "acme/widget",
		"ssh://git@github.com/acme/widget":   "acme/widget",

		// A port must not be mistaken for the scp-style separator.
		"ssh://git@git.example.com:2222/acme/widget.git": "acme/widget",

		// Nested groups keep the trailing pair, so a mirror at a shorter path keys
		// the same repository.
		"https://gitlab.example.com/group/subgroup/widget.git": "subgroup/widget",

		// Trailing slash is noise.
		"https://github.com/acme/widget/": "acme/widget",
	}

	for remote, want := range cases {
		t.Run(remote, func(t *testing.T) {
			info := &RepositoryInfo{RemoteURL: remote, Path: "/tmp/some-dir", Name: "some-dir"}
			if got := info.GovernanceID(); got != want {
				t.Errorf("GovernanceID() = %q, want %q", got, want)
			}
		})
	}
}

// Without a remote the identity is local and says so. The prefix matters: a bare
// directory name would look like a repository named after somebody's folder, and
// would collide with a real repository of that name.
func TestGovernanceIDWithoutARemote(t *testing.T) {
	info := &RepositoryInfo{Path: "/home/dev/widget", Name: "widget"}
	if got := info.GovernanceID(); got != "local:widget" {
		t.Errorf("GovernanceID() = %q, want local:widget", got)
	}

	// Name is derived from the path when absent, so the identity does not depend on
	// which populated it.
	fromPath := &RepositoryInfo{Path: "/home/dev/widget"}
	if got := fromPath.GovernanceID(); got != "local:widget" {
		t.Errorf("GovernanceID() from path = %q, want local:widget", got)
	}
}

// A remote that yields no owner/repo pair must fall back rather than key records
// under a fragment — a partial key is worse than a local one because it looks
// authoritative.
func TestGovernanceIDFallsBackOnAnUnusableRemote(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"https://github.com/",
		"https://github.com",
		"not-a-url",
	}

	for _, remote := range cases {
		t.Run(remote, func(t *testing.T) {
			info := &RepositoryInfo{RemoteURL: remote, Path: "/home/dev/widget", Name: "widget"}
			if got := info.GovernanceID(); got != "local:widget" {
				t.Errorf("GovernanceID() = %q, want the local fallback", got)
			}
		})
	}
}

// The identity must not depend on where the repository is checked out. Keying on
// the path is what made a second clone or a CI runner start with no history while
// the store looked healthy.
func TestGovernanceIDIsIndependentOfTheCheckoutPath(t *testing.T) {
	laptop := &RepositoryInfo{
		RemoteURL: "https://github.com/acme/widget.git",
		Path:      "/Users/dev/code/widget",
		Name:      "widget",
	}
	ci := &RepositoryInfo{
		RemoteURL: "git@github.com:acme/widget.git",
		Path:      "/home/runner/work/widget/widget",
		Name:      "widget",
	}
	// A different directory name entirely, which is what a temp checkout looks like.
	tmp := &RepositoryInfo{
		RemoteURL: "https://github.com/acme/widget",
		Path:      "/tmp/tmp.6fPqrJakiQ",
		Name:      "tmp.6fPqrJakiQ",
	}

	want := laptop.GovernanceID()
	if want != "acme/widget" {
		t.Fatalf("precondition: got %q", want)
	}
	if got := ci.GovernanceID(); got != want {
		t.Errorf("CI checkout keyed as %q, want %q", got, want)
	}
	if got := tmp.GovernanceID(); got != want {
		t.Errorf("temp checkout keyed as %q, want %q — this is the case that produced "+
			"acme/tmp.6fPqrJakiQ", got, want)
	}
}

func TestGovernanceIDOnNil(t *testing.T) {
	var info *RepositoryInfo
	if got := info.GovernanceID(); got != "" {
		t.Errorf("GovernanceID() on nil = %q, want empty", got)
	}
}
