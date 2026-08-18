package cli

// db_import.go is the operator-facing half of ADR-013's migration path.
//
// The ADR makes migration explicit: relicta does not move an audit trail because someone
// edited a config key, so switching persistence.backend leaves the existing history where it
// is until this command is run. It also does not delete the JSON afterwards — the tree stays
// as an export until the operator removes it — which is why this command reads and never
// writes on the source side.
//
// It covers the whole `.relicta/` system of record, not only the runs. That is the difference
// between a migration and a trap: once persistence.backend selects the governance store too,
// an importer that moved runs alone would leave an operator with a healthy-looking `relicta
// status` and an empty `relicta history`, empty DORA and SOC 2 reports, and a deployment gate
// authorizing against a record with no releases in it.
//
// What the command owes an operator beyond doing the work is evidence that it happened: how
// many runs and governance records moved, which of them were already there, which run the
// destination now calls the current release, and what could not be moved at all.
// "Imported successfully" is not checkable; the counts are.

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/releasehistory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/container"
	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

var dbImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import the .relicta history into the configured database backend",
	Long: `Import an existing file-based history into the configured backend.

Switching persistence.backend does not move the history that is already on disk —
migration is explicit (ADR-013). This command reads what is under .relicta and
writes it into the backend persistence.backend selects:

  * every release run under .relicta/releases, and the pointer to the current one
  * the governance record in .relicta/governance/memory.json: release records,
    incidents, decisions and execution authorizations

Deployment records cannot move. Neither database backend has a deployments table,
so they stay in memory.json and the command says how many were left behind.

The JSON is left exactly as it was. It stays as an export until you remove it
yourself. Nothing under .relicta is deleted, moved or rewritten.

Re-running the import is safe: records are matched by ID and replaced from the
JSON, so a second run converges on the same history instead of duplicating it.
That also makes it the fix for an import that was interrupted.

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
	// The whole configuration, not only its persistence section: the governance half of the
	// import reads governance.memory_path to find the file store it exports from, and the two
	// have to come from one Config or the import would read the right backend at the wrong
	// path.
	importCfg := cfg
	if importCfg == nil {
		importCfg = config.DefaultConfig()
	}

	repoRoot, err := currentRepoRoot(ctx)
	if err != nil {
		return err
	}

	if dryRun {
		printDryRunBanner()
	}

	result, err := container.ImportHistory(ctx, importCfg, repoRoot,
		releasehistory.Options{DryRun: dryRun})
	if err != nil {
		// The reports are printed before returning the error, not instead of it. A write
		// that failed partway leaves records in the destination, and an operator told only
		// "it failed" has to go and find out whether anything moved.
		printPartialImport(result.Runs)
		printPartialGovernanceImport(result.Governance)
		return err
	}

	printImportReport(result.Runs, result.Into)
	printGovernanceImportReport(result.Governance, result.Into)
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

// printGovernanceImportReport says what happened to the governance record.
//
// A section of its own rather than folded into the run counts, because the two answer
// different questions and an operator checking a migration has to see both. A destination
// holding runs and no governance record has no risk scores, no decisions and no actor
// metrics: it looks migrated and reports nothing.
func printGovernanceImportReport(report releasehistory.GovernanceReport, into container.ImportedInto) {
	if report.Records() == 0 {
		printInfo("No governance memory found under .relicta/governance — nothing to import.")
		return
	}

	destination := fmt.Sprintf("%s (%s)", into.Backend, into.GovernanceLocation)
	counts := governanceCounts(report)

	if report.DryRun {
		printInfo(fmt.Sprintf("Would import governance memory into %s: %s.", destination, counts))
	} else {
		printSuccess(fmt.Sprintf("Imported governance memory into %s: %s.", destination, counts))
	}

	if len(report.Repositories) > 1 {
		// More than one key in one checkout's memory.json almost always means records
		// written before the governance identity was made canonical sitting beside records
		// written after. `relicta history` reads one key, so the operator would otherwise
		// see part of their history and no reason for it.
		printWarning(fmt.Sprintf(
			"The governance memory holds records under %d repository identities: %v. "+
				"`relicta history` reads one of them, so records under the others will not "+
				"be listed.", len(report.Repositories), report.Repositories))
	}

	if report.Deployments > 0 {
		// Named rather than silently dropped. This is the one part of the audit trail the
		// import cannot move, and an operator who is not told is an operator whose
		// deployment frequency quietly changes.
		printWarning(fmt.Sprintf(
			"%d deployment record(s) were not imported: the %s backend has no deployments "+
				"table, so `relicta deploy list` and the deployment half of the DORA report "+
				"will be empty under it. They remain in .relicta/governance/memory.json.",
			report.Deployments, into.Backend))
	}

	if !report.DryRun {
		printInfo(".relicta/governance/memory.json was not modified. Remove it yourself once " +
			"you have verified the import.")
	}
}

// printPartialGovernanceImport says what reached the destination before a failure.
func printPartialGovernanceImport(report releasehistory.GovernanceReport) {
	if report.Written() == 0 {
		return
	}
	printWarning(fmt.Sprintf(
		"%s were written before the governance import stopped. The governance memory under "+
			".relicta/governance is unchanged; run `relicta db import` again to finish.",
		governanceCounts(report)))
}

// governanceCounts renders the four record types in one line.
func governanceCounts(report releasehistory.GovernanceReport) string {
	return fmt.Sprintf("%d release record(s), %d incident(s), %d decision(s), %d authorization(s)",
		report.Releases, report.Incidents, report.Decisions, report.Authorizations)
}
