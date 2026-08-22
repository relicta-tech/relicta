package cli

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// Two commands in this CLI create a tag: `relicta publish` and `relicta release`. Everything a
// release has to clear before that tag exists therefore belongs somewhere neither of them owns,
// because the alternative is two copies that drift — and they had.
//
// `relicta release` called PublishRelease.Execute directly and applied none of it: not the
// per-actor autonomy budget, not workflow.require_clean_working_tree, not
// workflow.require_up_to_date, not the release commit that puts the changelog and version files
// inside the tag, and neither release hook. Verified against the shipped binary in four
// throwaway repositories: a modified tracked file, a branch one commit behind origin/main, a
// `pre_release_hook: exit 3` and an actor budget requiring a cosigner to publish each refused
// `relicta publish`, and each sailed through `relicta release --yes`, which tagged v0.1.0 and
// reported "Released 0.1.0 successfully!".
//
// Of the two, the unguarded one was the worse to lose: `relicta release` is the one-shot
// command, so it is the one CI runs.
//
// The order these run in is meaning, not arrangement:
//
//  1. the autonomy budget, because an actor who may not publish should not have relicta reading
//     the network or running their hooks on the way to being told so;
//  2. the clean-tree gate, before anything mutates — publishing writes the changelog, so a check
//     any later reports on relicta's own edits rather than the operator's;
//  3. the branch-freshness gate, after the clean-tree one because that is local and instant and
//     there is no reason to spend a network round trip proving a branch current only to refuse
//     the release for a dirty tree;
//  4. --dry-run returns — between the gates and the mutations, so a dry run is still told it
//     would be refused but nothing is written. That return belongs to each command, because
//     each has its own preview to print first;
//  5. the pre-release hook, where a veto is still free: nothing has been written yet;
//  6. the release commit, so the tag that follows contains the changelog and version files;
//  7. the publish itself;
//  8. the post-release hook, which warns rather than fails — the tag already exists.
//
// Steps 1–3 are enforcePrePublishGates and steps 5–6 are prepareReleaseForPublish; step 8 is
// runPostReleaseHook, which is already one function both paths call, so wrapping it again would
// add a name without removing a way to diverge.

// enforcePrePublishGates applies the gates that must pass before a release changes anything —
// steps 1 to 3 above.
//
// riskScore is the CGP risk of the change being published, which only the budget gate reads.
//
// Errors come back unwrapped. Each gate's message already names the setting that refused, what
// the release would otherwise have done, and how to proceed; prefixing them with a stage name
// would bury that under the caller's vocabulary.
func enforcePrePublishGates(ctx context.Context, riskScore float64) error {
	if err := enforceActorBudget("publish", riskScore); err != nil {
		return err
	}

	if err := enforceAllowedBranch(ctx); err != nil {
		return err
	}

	if err := enforceCleanWorkingTree(ctx); err != nil {
		return err
	}

	return enforceUpToDate(ctx)
}

// enforceAllowedBranch refuses a release from a branch workflow.allowed_branches does not list.
//
// The setting was validated, defaulted, and enforced by nothing: a repository restricting
// releases to main could release from anywhere. It hid from the unread-configuration sweep
// because the MCP server assigns to the field — a write counts as a use — and the only other
// implementation of the check lived in a package nothing imports.
//
// An empty list means no restriction, which is the default and what every repository has today.
// The gate is placed before the clean-tree check because being on the wrong branch is the
// cheaper thing to discover: nothing has to be read from the working tree to know it.
func enforceAllowedBranch(ctx context.Context) error {
	if cfg == nil || len(cfg.Workflow.AllowedBranches) == 0 {
		return nil
	}

	svc, err := gitservice.NewService()
	if err != nil {
		return fmt.Errorf("could not open the repository, and workflow.allowed_branches "+
			"requires knowing the branch: %w", err)
	}

	branch, err := svc.GetCurrentBranch(ctx)
	if err != nil {
		// Unknown is not permission: this gate exists to stop a release from the wrong place,
		// and "I could not tell" is not "it is fine" — the same rule the clean-tree gate uses.
		return fmt.Errorf("could not determine the current branch, and "+
			"workflow.allowed_branches requires it: %w", err)
	}

	if branchIsAllowed(cfg.Workflow.AllowedBranches, branch) {
		return nil
	}

	return errors.New(branchRefusal(cfg.Workflow.AllowedBranches, branch))
}

// branchIsAllowed reports whether a branch matches the configured list.
//
// Patterns are globs, so `release/*` covers `release/1.0` — and does not cover
// `release/1.0/hotfix`, because path.Match stops at a separator. That is the same rule git
// refspecs use, and the alternative would let one pattern quietly widen.
func branchIsAllowed(allowed []string, branch string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, pattern := range allowed {
		if matched, err := path.Match(pattern, branch); err == nil && matched {
			return true
		}
	}
	return false
}

// branchRefusal is what the operator reads: the setting, the branch they are on, and the two
// ways forward.
func branchRefusal(allowed []string, branch string) string {
	return fmt.Sprintf("releases are restricted to %s by workflow.allowed_branches, and this is "+
		"%q. Switch branches, or add this one to the list",
		strings.Join(allowed, ", "), branch)
}

// prepareReleaseForPublish runs the pre-release hook and then writes and commits the release
// artifacts — steps 5 and 6 above, the two things that happen between the last chance to abort
// and the tag.
//
// Both callers reach here only after their own --dry-run return, because a hook is somebody
// else's code and a commit is a commit.
//
// A release run that cannot be loaded is not fatal: commitReleaseArtifacts needs it for the
// notes it writes into the changelog, and a project with no readable run has no notes to write.
// The hook has already run by then and the publish that follows loads the run itself, so it
// will produce the real error if there genuinely is one.
func prepareReleaseForPublish(ctx context.Context, app cliApp, version, tagName string) error {
	if err := runPreReleaseHook(ctx, version, tagName); err != nil {
		return err
	}

	rel, err := getLatestRelease(ctx, app)
	if err != nil {
		return nil
	}
	return commitReleaseArtifacts(ctx, app, rel, version)
}
