package cli

// db_import.go is the operator-facing half of ADR-013's migration path.
//
// The ADR makes migration explicit: relicta does not move an audit trail because someone
// edited a config key, so switching persistence.backend leaves the existing history where it
// is until this command is run. It also does not delete the JSON afterwards — the tree stays
// as an export until the operator removes it — which is why this command reads and never
// writes on the source side.
//
// What the command owes an operator beyond doing the work is evidence that it happened: how
// many runs moved, which of them were already there, and which run the destination now calls
// the current release. "Imported successfully" is not checkable; the counts are.

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/releasehistory"
	"github.com/relicta-tech/relicta/v4/internal/container"
	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

var dbImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import the .relicta/releases history into the configured database backend",
	Long: `Import an existing file-based release history into the configured backend.

Switching persistence.backend does not move the history that is already on disk —
migration is explicit (ADR-013). This command reads every release run under
.relicta/releases, writes it into the backend persistence.backend selects, and
transfers the pointer to the current release.

The JSON tree is left exactly as it was. It stays as an export until you remove
it yourself. Nothing under .relicta/releases is deleted, moved or rewritten.

Re-running the import is safe: runs are matched by ID and replaced from the JSON,
so a second run converges on the same history instead of duplicating it. That
also makes it the fix for an import that was interrupted.

  # Show what would move, without writing
  relicta db import --dry-run

  # Move it
  relicta db import

For postgres under migration_mode: manual, run 'relicta db migrate' first —
relicta does not create tables in a database you provisioned.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runDBImport(cmd.Context())
	},
}

func runDBImport(ctx context.Context) error {
	persistenceCfg := newDBCommands().getConfig()

	repoRoot, err := currentRepoRoot(ctx)
	if err != nil {
		return err
	}

	if dryRun {
		printDryRunBanner()
	}

	report, into, err := container.ImportReleaseHistory(ctx, persistenceCfg, repoRoot,
		releasehistory.Options{DryRun: dryRun})
	if err != nil {
		// The report is printed before returning the error, not instead of it. A write that
		// failed partway leaves runs in the destination, and an operator who is told only
		// "it failed" has to go and find out whether anything moved.
		printPartialImport(report)
		return err
	}

	printImportReport(report, into)
	return nil
}

// currentRepoRoot resolves the repository the command is running in.
//
// The git adapter rather than the working directory: `relicta db import` run from a
// subdirectory must migrate the repository's history, and the file store's runs are keyed by
// the repository root. Guessing the working directory is how relicta once scattered stray
// .relicta trees through subdirectories.
func currentRepoRoot(ctx context.Context) (string, error) {
	svc, err := gitservice.NewService()
	if err != nil {
		return "", fmt.Errorf("relicta db import needs a git repository to find the release "+
			"history in: %w", err)
	}

	info, err := gitservice.NewAdapter(svc).GetInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to resolve the repository root: %w", err)
	}
	return info.Path, nil
}

func printImportReport(report releasehistory.Report, into container.ImportedInto) {
	if report.Runs == 0 {
		// Not a failure. A repository that has never planned a release has nothing to move,
		// and the operator's next step is unchanged either way.
		printInfo(fmt.Sprintf("No release history found under %s/.relicta/releases — nothing to import.",
			report.RepoRoot))
		return
	}

	destination := fmt.Sprintf("%s (%s)", into.Backend, into.Location)

	if report.DryRun {
		printInfo(fmt.Sprintf("Would import %s into %s.", runCount(report.Runs), destination))
		if report.Latest != "" {
			printInfo(fmt.Sprintf("Would point the current release at %s.", report.Latest))
		} else {
			printInfo("The history has no current release pointer to transfer.")
		}
		printForeignRoots(report)
		printInfo("Nothing was written.")
		return
	}

	printSuccess(fmt.Sprintf("Imported %s into %s (%d new, %d replaced).",
		runCount(report.Runs), destination, report.Created, report.Replaced))

	switch {
	case report.LatestTransferred:
		printSuccess(fmt.Sprintf("Current release pointer set to %s.", report.Latest))
	default:
		printInfo("The history has no current release pointer, so none was set.")
	}

	printForeignRoots(report)
	printInfo(fmt.Sprintf("%s/.relicta/releases was not modified. Remove it yourself once you "+
		"have verified the import.", report.RepoRoot))
}

// printPartialImport says what reached the destination before a failure.
func printPartialImport(report releasehistory.Report) {
	if report.Written() == 0 {
		return
	}
	printWarning(fmt.Sprintf(
		"%d of %d runs were written before the import stopped. The release history under "+
			"%s/.relicta/releases is unchanged; run `relicta db import` again to finish.",
		report.Written(), report.Runs, report.RepoRoot))
}

// printForeignRoots warns about runs the destination will file under another repository.
//
// Worth a line of its own because the symptom is otherwise baffling: the import reports twelve
// runs and `relicta history` shows nine. Both database adapters scope every query by the root
// the run carries, and a run copied from another checkout carries that checkout's path.
func printForeignRoots(report releasehistory.Report) {
	if len(report.ForeignRoots) == 0 {
		return
	}
	printWarning(fmt.Sprintf(
		"%d run(s) name a different repository root than %s and are stored under the root they "+
			"name, so this repository's history will not list them: %v",
		len(report.ForeignRoots), report.RepoRoot, report.ForeignRoots))
}

func runCount(n int) string {
	if n == 1 {
		return "1 release run"
	}
	return fmt.Sprintf("%d release runs", n)
}
