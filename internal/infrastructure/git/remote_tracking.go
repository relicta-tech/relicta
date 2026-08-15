package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// RemoteFreshness describes how the current branch stands against its counterpart on the
// remote.
//
// The "nothing to compare against" cases are reported separately rather than collapsed into a
// zero count, because a caller gating a release has to treat them differently from being up to
// date. A repository with no remote genuinely cannot trail one; a branch that has never been
// pushed is unpublished rather than stale. Returning 0 for all of them would leave the caller
// able to say only "up to date", which is not what either case means — and a release gate that
// cannot tell "there is nothing behind me" from "I checked and found nothing" is the kind that
// passes a stale tree.
type RemoteFreshness struct {
	// Remote is the remote that was consulted. Empty when the repository has no remote.
	Remote string
	// Branch is the branch HEAD is on.
	Branch string
	// RemoteRef is the remote-tracking ref compared against, such as "origin/main". Empty
	// when the branch does not exist on the remote.
	RemoteRef string
	// Behind counts the commits reachable from RemoteRef but not from HEAD.
	Behind int
}

// HasRemote reports whether there was a remote to compare against at all.
func (f RemoteFreshness) HasRemote() bool { return f.Remote != "" }

// IsPublished reports whether the branch exists on the remote.
func (f RemoteFreshness) IsPublished() bool { return f.RemoteRef != "" }

// BehindRemote fetches from the remote and reports how far the current branch trails it.
//
// It fetches, deliberately. Comparing against a remote-tracking ref that was last refreshed
// whenever the operator happened to run `git fetch` answers "was this branch up to date at
// some point in the past" — and answers it with a pass, so the stale checkout sails through
// the check that exists to catch it. A stale comparison that passes is a worse failure than a
// slow one: the slow one is visible. The cost is one network round trip, on a publish where
// the operator switched this on and is paying for exactly that.
//
// The fetch shells out rather than using go-git's Fetch, for the reason CommitPaths shells out
// to commit: reaching a real remote needs the user's credential helpers, SSH config, insteadOf
// rewrites and proxy settings, and go-git implements none of them. A gate that cannot
// authenticate refuses every release on a private repository, which is how a safety gate gets
// switched off rather than obeyed.
//
// The ancestry count shells out too. "Behind" is defined by git's own merge-base walk, and a
// reimplementation that is subtly wrong either refuses correct releases or waves stale ones
// through — and having just asked git to write the refs, git is the reader that is certain to
// see what it wrote.
//
// announce, if non-nil, is called with the remote's name immediately before the fetch and only
// when a fetch will actually happen. A caller has to be able to say "contacting origin" ahead
// of a wait the user did not ask for, and it must not say it in the repository that has no
// remote to contact — a message about a network call that never occurs teaches the operator to
// distrust the messages.
func (s *ServiceImpl) BehindRemote(ctx context.Context, announce func(remote string)) (RemoteFreshness, error) {
	branch, err := s.GetCurrentBranch(ctx)
	if err != nil {
		return RemoteFreshness{}, err
	}

	remote, err := s.trackingRemote(branch)
	if err != nil {
		return RemoteFreshness{}, err
	}
	if remote == "" {
		// No remote configured. Determinate, and the answer is "nothing to trail".
		return RemoteFreshness{Branch: branch}, nil
	}

	repoRoot, err := s.GetRepositoryRoot(ctx)
	if err != nil {
		return RemoteFreshness{}, err
	}

	if announce != nil {
		announce(remote)
	}

	if err := runGit(ctx, repoRoot, "fetch", "--quiet", remote); err != nil {
		return RemoteFreshness{}, err
	}

	fresh := RemoteFreshness{Remote: remote, Branch: branch}

	ref, err := s.remoteTrackingRef(ctx, repoRoot, remote, branch)
	if err != nil {
		return RemoteFreshness{}, err
	}
	if ref == "" {
		// The branch is not on the remote. Also determinate: an unpushed branch has no
		// counterpart to have fallen behind.
		return fresh, nil
	}
	fresh.RemoteRef = ref

	// HEAD..ref is what the remote has and we do not — which is precisely "behind". The
	// reverse range would count our unpushed work, a different and harmless condition.
	out, err := outputGit(ctx, repoRoot, "rev-list", "--count", "HEAD.."+ref)
	if err != nil {
		return RemoteFreshness{}, err
	}
	behind, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		return RemoteFreshness{}, fmt.Errorf("could not read the commit count from %q: %w", out, convErr)
	}
	fresh.Behind = behind

	return fresh, nil
}

// trackingRemote picks the remote this branch is measured against, or "" when the repository
// has none.
//
// The branch's own configured remote wins, because a branch tracking an upstream other than
// origin — a fork workflow, a release branch pushed to a mirror — is measured against the
// place it is actually published to. Falling straight through to "origin" would compare
// against a remote the branch has nothing to do with.
func (s *ServiceImpl) trackingRemote(branch string) (string, error) {
	remotes, err := s.repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("could not list the repository's remotes: %w", err)
	}
	if len(remotes) == 0 {
		return "", nil
	}

	names := make(map[string]bool, len(remotes))
	for _, r := range remotes {
		if cfg := r.Config(); cfg != nil {
			names[cfg.Name] = true
		}
	}

	if cfg, cfgErr := s.repo.Config(); cfgErr == nil && cfg != nil {
		if b, ok := cfg.Branches[branch]; ok && b != nil && names[b.Remote] {
			return b.Remote, nil
		}
	}

	if names[s.cfg.DefaultRemote] {
		return s.cfg.DefaultRemote, nil
	}

	// A single remote under some other name is unambiguous; guessing among several is not,
	// so the configured default stands and the ref lookup will report it as unpublished.
	if len(remotes) == 1 {
		return remotes[0].Config().Name, nil
	}
	return s.cfg.DefaultRemote, nil
}

// remoteTrackingRef returns the ref to compare HEAD against, or "" when the branch is not on
// the remote.
func (s *ServiceImpl) remoteTrackingRef(ctx context.Context, repoRoot, remote, branch string) (string, error) {
	// The configured upstream first: it survives a branch published under a different name,
	// which `<remote>/<branch>` guesses wrong.
	if out, err := outputGit(ctx, repoRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		if upstream := strings.TrimSpace(out); upstream != "" && refExists(ctx, repoRoot, upstream) {
			return upstream, nil
		}
	}

	candidate := remote + "/" + branch
	if !refExists(ctx, repoRoot, candidate) {
		return "", nil
	}
	return candidate, nil
}

// refExists reports whether a ref names a commit in this repository.
//
// Checked rather than assumed: an upstream can stay configured after the branch it points at
// is deleted on the remote, and comparing against a ref that is not there fails the release
// with a git error instead of the honest answer, which is that there is nothing to trail.
func refExists(ctx context.Context, repoRoot, ref string) bool {
	err := runGit(ctx, repoRoot, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// runGit runs a git command in the repository, discarding its output.
func runGit(ctx context.Context, repoRoot string, args ...string) error {
	_, err := outputGit(ctx, repoRoot, args...)
	return err
}

// outputGit runs a git command in the repository and returns its standard output.
//
// stderr is kept out of the return value and folded into the error instead: `git fetch` writes
// its progress there even when it succeeds, and a caller parsing a commit count would be
// parsing that progress. A failure carries git's own words, because a gate that refuses a
// release has to distinguish a mistyped remote from an expired credential for the operator.
func outputGit(ctx context.Context, repoRoot string, args ...string) (string, error) {
	var stderr strings.Builder

	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- fixed git subcommands with repository-derived refs
	cmd.Dir = repoRoot
	cmd.Env = filteredGitEnv()
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
