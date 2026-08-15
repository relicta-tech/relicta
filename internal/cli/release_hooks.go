package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// workflow.pre_release_hook and workflow.post_release_hook were configured commands that never
// ran. Both were declared, both were documented, both were even environment-expanded by the
// config loader — whose comment said in so many words that they are "stored but NOT executed".
// Verified against the shipped binary: with both hooks set to write a file, a full release
// tagged v0.1.0 and neither file existed afterwards.
//
// A hook that silently does nothing is worse than an unimplemented one. The operator who wrote
// `pre_release_hook: make test` believes their tests gate the release, and they do not.

// runPreReleaseHook runs workflow.pre_release_hook, refusing the release if it fails.
//
// Refusing is the entire point. A pre-release hook is where an operator puts the test suite,
// the build verification, the deploy-freeze check — commands whose job is to have a veto. A
// veto that is logged and then ignored is decoration, and it is worse than nothing because it
// reads like a safeguard. Refusal is also free here: this runs before relicta has written the
// changelog, made the release commit or created the tag, so nothing needs undoing.
func runPreReleaseHook(ctx context.Context, version, tag string) error {
	if cfg == nil || cfg.Workflow.PreReleaseHook == "" {
		return nil
	}

	if err := runReleaseHook(ctx, "pre-release", cfg.Workflow.PreReleaseHook, version, tag); err != nil {
		return fmt.Errorf("%w. Nothing has been tagged or committed; fix the hook or the "+
			"problem it found, then publish again", err)
	}
	return nil
}

// runPostReleaseHook runs workflow.post_release_hook, warning if it fails.
//
// Warning rather than failing, because by this point the release has shipped: the tag exists,
// and with git_push on it is on the remote where other people's tooling can already see it.
// Returning an error would make relicta's exit code say the release failed when it did not,
// and the standard reaction to a failed release step — in CI and in operators alike — is to
// run it again, which means a second publish against a version that is already tagged.
//
// So the exit code stays the release's own verdict and the failure is made loud instead: the
// command, its exit status and its output all reach the terminal. An operator who needs a
// failing command to fail the build should run it as a pre-release hook, where refusing still
// means something, or as the CI step after relicta.
func runPostReleaseHook(ctx context.Context, version, tag string) {
	if cfg == nil || cfg.Workflow.PostReleaseHook == "" {
		return
	}

	if err := runReleaseHook(ctx, "post-release", cfg.Workflow.PostReleaseHook, version, tag); err != nil {
		printWarning(fmt.Sprintf("%v.\n    The release itself succeeded and %s is tagged — "+
			"this is the hook, not the release. Run the command by hand once you have fixed "+
			"it; publishing again would re-run against a version that already exists.",
			err, tag))
	}
}

// runReleaseHook executes one configured hook command.
//
// Run through `sh -c`, not split into arguments. The loader's comment used to insist on the
// opposite, and it was wrong about the threat: this string comes from the operator's own
// .relicta.yaml, and a config file that can name a command to run is already arbitrary code
// execution — shell metacharacters buy an attacker who can write that file precisely nothing.
// What argument splitting does buy is broken hooks, because real ones are shell:
// `make test && ./scripts/verify.sh`, `npm run build | tee build.log`, `[ -z "$FREEZE" ]`.
// Splitting those on whitespace runs something the operator did not write.
//
// The same change is why the loader no longer expands ${VAR} into the hook string. Substituting
// a variable's value into a command *before* the shell parses it is the textbook injection: a
// variable holding `; rm -rf /` becomes a second command. `sh` expands `$VAR` itself, after
// parsing, where the value stays a value. Hooks therefore keep working exactly as written and
// stop being a way for whatever set an environment variable to append commands.
//
// Output streams straight through rather than being captured and replayed. Hooks are test
// suites and builds that take minutes, and a captured hook is indistinguishable from relicta
// hanging.
//
// No timeout beyond the caller's context: a hook that runs the test suite legitimately takes as
// long as the test suite takes, and a default deadline would kill correct releases on slow
// projects.
func runReleaseHook(ctx context.Context, name, command, version, tag string) error {
	printInfo(fmt.Sprintf("Running %s hook: %s", name, command))

	cmd := exec.CommandContext(ctx, "sh", "-c", command) // #nosec G204 -- the operator's own configured hook command
	cmd.Dir = hookWorkingDir(ctx)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil

	// The two facts a hook almost always needs, so the common cases — notifying a chat
	// channel, triggering a deploy, uploading an artifact — need no templating in the config
	// string. The version without the prefix and the tag with it are different strings and
	// hooks want both; deriving one from the other means every hook reimplements tag_prefix.
	cmd.Env = append(os.Environ(),
		"RELICTA_VERSION="+version,
		"RELICTA_TAG="+tag,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s hook failed: %s: %w", name, command, err)
	}

	printSuccess(fmt.Sprintf("%s hook succeeded", name))
	return nil
}

// hookWorkingDir returns the repository root, falling back to the process's own directory.
//
// The root rather than wherever relicta was invoked from: `relicta publish` works from any
// subdirectory, and a hook of `make test` that runs in internal/cli/ fails for a reason that
// has nothing to do with the release.
func hookWorkingDir(ctx context.Context) string {
	svc, err := gitservice.NewService()
	if err != nil {
		return ""
	}
	root, err := svc.GetRepositoryRoot(ctx)
	if err != nil {
		return ""
	}
	return root
}
