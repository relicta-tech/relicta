package cli

// deploy.go: records that a released version reached an environment.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// Relicta recorded that a release was governed and published, and nothing about it
// reaching an environment, so the evidence chain ended at the tag. ADR-012 decides
// that deployment evidence is reported by whatever performs the deployment, because
// only the deployer knows the difference between a deployment being requested and a
// deployment succeeding.
//
// These commands are the local half of that: a pipeline step or a person records a
// deployment, and anyone can read what reached where. A GitOps controller that
// cannot run relicta reports over HTTP instead, which is the same record arriving
// through a different door.

var (
	deployEnv        string
	deployVersion    string
	deployOutcome    string
	deployRef        string
	deployReleaseID  string
	deployDuration   string
	deployProvenance string
	deployListEnv    string
	deployListLimit  int
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Record and inspect deployments of released versions",
	Long: `Record that a released version reached an environment, and read what did.

A release is a tag being published; a deployment is a change reaching an
environment. Relicta needs both to answer "what is running, and was it governed",
and DORA metrics need deployments rather than releases to mean anything.

Environments must be declared in configuration under 'environments', with exactly
one marked production.

Examples:
  # From a pipeline step, after the deploy succeeded
  relicta deploy record --env production --version 1.2.0

  # A failed deployment is worth recording — change failure rate is computed from these
  relicta deploy record --env production --version 1.2.0 --outcome failed

  # What reached where
  relicta deploy list
  relicta deploy list --env production`,
}

var deployRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a deployment of a version to an environment",
	RunE:  runDeployRecord,
}

var deployListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded deployments, newest first",
	RunE:  runDeployList,
}

func init() {
	deployCmd.AddCommand(deployRecordCmd)
	deployCmd.AddCommand(deployListCmd)
	deployCmd.AddCommand(deployAuditCmd)

	deployRecordCmd.Flags().StringVar(&deployEnv, "env", "", "environment the version reached (required)")
	deployRecordCmd.Flags().StringVar(&deployVersion, "version", "", "version that was deployed (required)")
	deployRecordCmd.Flags().StringVar(&deployOutcome, "outcome", "succeeded", "succeeded, failed or rolled_back")
	deployRecordCmd.Flags().StringVar(&deployRef, "ref", "", "reference back to the deploying system (CI run URL, rollout ID)")
	deployRecordCmd.Flags().StringVar(&deployReleaseID, "release-id", "", "relicta release run this deployment came from")
	deployRecordCmd.Flags().StringVar(&deployDuration, "duration", "", "how long the deployment took (e.g. 4m12s)")
	deployRecordCmd.Flags().StringVar(&deployProvenance, "provenance", "reported",
		"what observed this: reported, inferred or manual")

	deployListCmd.Flags().StringVar(&deployListEnv, "env", "", "only this environment")
	deployListCmd.Flags().IntVar(&deployListLimit, "limit", 20, "maximum records to show")

	deployAuditCmd.Flags().BoolVar(&deployAuditStrict, "strict", false,
		"also fail when a governed release never reached production")
}

// declaredEnvironment resolves a name against the configured environments.
//
// An unknown environment is refused rather than recorded. Free-form names let
// "prod", "production" and "Production" become three environments in one audit
// report, each holding part of the history, and nothing would report that as wrong —
// the records look fine individually.
func declaredEnvironment(name string) (config.EnvironmentConfig, error) {
	if cfg == nil || len(cfg.Environments) == 0 {
		// The hint carries the YAML, because `relicta init` does not write an
		// environments section and naming the key sends the reader to a file that
		// does not contain it.
		hintEnvironments.print()
		return config.EnvironmentConfig{}, errors.New("no environments are declared")
	}

	declared := make([]string, 0, len(cfg.Environments))
	for _, env := range cfg.Environments {
		if env.Name == name {
			return env, nil
		}
		declared = append(declared, env.Name)
	}

	return config.EnvironmentConfig{}, fmt.Errorf(
		"unknown environment %q; declared environments are %v", name, declared)
}

func runDeployRecord(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	if deployEnv == "" || deployVersion == "" {
		return fmt.Errorf("--env and --version are both required")
	}

	env, err := declaredEnvironment(deployEnv)
	if err != nil {
		return err
	}

	outcome := memory.DeploymentOutcome(deployOutcome)
	if !outcome.IsValid() {
		return fmt.Errorf("unknown outcome %q: use succeeded, failed or rolled_back", deployOutcome)
	}

	provenance := memory.DeploymentProvenance(deployProvenance)
	if !provenance.IsValid() {
		return fmt.Errorf("unknown provenance %q: use reported, inferred or manual", deployProvenance)
	}

	var duration time.Duration
	if deployDuration != "" {
		duration, err = time.ParseDuration(deployDuration)
		if err != nil {
			return fmt.Errorf("invalid --duration %q: %w", deployDuration, err)
		}
	}

	store, repository, err := deploymentStore(ctx)
	if err != nil {
		return err
	}

	record := &memory.DeploymentRecord{
		ID:          fmt.Sprintf("deploy-%s-%s-%d", env.Name, deployVersion, timeNowFunc().UnixNano()),
		Repository:  repository,
		Environment: env.Name,
		Version:     deployVersion,
		Actor:       createCGPActor(),
		Outcome:     outcome,
		DeployedAt:  timeNowFunc(),
		Duration:    duration,
		Provenance:  provenance,
		Reference:   deployRef,
		ReleaseID:   deployReleaseID,
	}

	if err := store.RecordDeployment(ctx, record); err != nil {
		return fmt.Errorf("failed to record the deployment: %w", err)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(record)
	}

	printSuccess(fmt.Sprintf("Recorded %s %s in %s", record.Version, record.Outcome, record.Environment))
	if env.Production {
		printInfo("This is the production environment, so it counts toward deployment frequency.")
	}
	return nil
}

func runDeployList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	if deployListEnv != "" {
		if _, err := declaredEnvironment(deployListEnv); err != nil {
			return err
		}
	}

	store, repository, err := deploymentStore(ctx)
	if err != nil {
		return err
	}

	records, err := store.GetDeploymentHistory(ctx, repository, deployListEnv, deployListLimit)
	if err != nil {
		return fmt.Errorf("failed to read deployments: %w", err)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"repository":  repository,
			"deployments": records,
			"total":       len(records),
		})
	}

	if len(records) == 0 {
		fmt.Printf("No deployments recorded for %s.\n", repository)
		fmt.Println()
		fmt.Println("Record one with: relicta deploy record --env <name> --version <version>")
		fmt.Println("A GitOps controller can report them over HTTP instead — see docs/adr/012.")
		return nil
	}

	printTitle(fmt.Sprintf("Deployments for %s (%d)", repository, len(records)))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  WHEN\tENVIRONMENT\tVERSION\tOUTCOME\tEVIDENCE\tACTOR")
	for _, r := range records {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			r.DeployedAt.Format("2006-01-02 15:04"), r.Environment, r.Version,
			r.Outcome, r.Provenance, r.Actor.ID)
	}
	_ = w.Flush()
	fmt.Println()

	return nil
}

// deploymentStore opens the governance store and resolves the repository identity,
// so deployments are keyed exactly as releases are and the report path finds both.
func deploymentStore(ctx context.Context) (memory.DeploymentStore, string, error) {
	store, err := getMemoryStoreCtx(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open the governance store: %w", err)
	}

	deployments, ok := store.(memory.DeploymentStore)
	if !ok {
		// An honest refusal rather than a silent no-op: a configured store that
		// cannot hold deployments would otherwise accept the command and record
		// nothing.
		return nil, "", fmt.Errorf("the configured governance store does not record deployments")
	}

	repository := getRepositoryName(ctx)
	if repository == "" {
		return nil, "", fmt.Errorf("could not determine the repository identity")
	}
	return deployments, repository, nil
}

// timeNowFunc is swappable so tests can pin deployment timestamps.
var timeNowFunc = func() time.Time { return time.Now().UTC() }

// productionEnvironmentName returns the declared production environment, or "".
//
// Empty is meaningful: without it the report cannot tell which deployments reached
// users, so it falls back to counting releases and says so rather than counting
// every environment a version passes through and reading three times too high.
func productionEnvironmentName() string {
	if cfg == nil {
		return ""
	}
	for _, env := range cfg.Environments {
		if env.Production {
			return env.Name
		}
	}
	return ""
}

var deployAuditStrict bool

var deployAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Compare what is deployed against what was governed",
	Long: `Compare deployment records against release records.

Two discrepancies, and they are not equally serious:

  ungoverned deployment  a version is running with no relicta release behind it —
                         something reached an environment without passing through
                         governance
  undeployed release     a governed release never reached production

Exits non-zero when an ungoverned deployment is found, so a pipeline can gate on
it. An undeployed release is reported but does not fail the command: a release
awaiting rollout is a normal state, and failing on it would train people to ignore
the command — at which point it stops reporting the serious case too. Use --strict
to fail on both.`,
	RunE: runDeployAudit,
}

func runDeployAudit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	store, repository, err := deploymentStore(ctx)
	if err != nil {
		return err
	}

	deployments, err := store.GetDeploymentHistory(ctx, repository, "", 10000)
	if err != nil {
		return fmt.Errorf("failed to read deployments: %w", err)
	}

	full, ok := store.(memory.Store)
	if !ok {
		return fmt.Errorf("the configured store cannot read release history")
	}
	releases, err := full.GetReleaseHistory(ctx, repository, 10000)
	if err != nil {
		return fmt.Errorf("failed to read releases: %w", err)
	}

	discrepancies := memory.Reconcile(releases, deployments, productionEnvironmentName())

	severe := 0
	for _, d := range discrepancies {
		if d.Severe() {
			severe++
		}
	}

	if outputJSON {
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
			"repository":    repository,
			"discrepancies": discrepancies,
			"ungoverned":    severe,
			"total":         len(discrepancies),
		}); err != nil {
			return err
		}
	} else {
		reportDiscrepancies(repository, discrepancies, severe)
	}

	if severe > 0 || (deployAuditStrict && len(discrepancies) > 0) {
		return fmt.Errorf("%d discrepancy(ies) between what is deployed and what was governed", len(discrepancies))
	}
	return nil
}

func reportDiscrepancies(repository string, discrepancies []memory.Discrepancy, severe int) {
	if len(discrepancies) == 0 {
		printSuccess(fmt.Sprintf("Everything deployed for %s was governed, and every release reached production.", repository))
		return
	}

	printTitle(fmt.Sprintf("Deployment Audit: %s", repository))
	fmt.Println()

	for _, d := range discrepancies {
		marker := "•"
		if d.Severe() {
			marker = "✗"
		}
		fmt.Printf("  %s %s %s\n", marker, d.Version, d.Kind)
		fmt.Printf("      %s\n", d.Detail)
		if d.Actor != "" {
			fmt.Printf("      actor: %s\n", d.Actor)
		}
		if d.Reference != "" {
			fmt.Printf("      reported by: %s\n", d.Reference)
		}
		fmt.Println()
	}

	if severe > 0 {
		printWarning(fmt.Sprintf("%d version(s) are running without a governed release.", severe))
	}
}
