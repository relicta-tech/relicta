package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	appmultirepo "github.com/relicta-tech/relicta/internal/application/multirepo"
	"github.com/relicta-tech/relicta/internal/domain/multirepo"
)

var (
	groupName   string
	targetRepos []string
)

// groupCmd is the parent command for multi-repo operations.
var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Multi-repository governance commands",
	Long: `Manage coordinated releases across multiple repositories.

Repository groups are defined in .relicta.yaml under 'repository_groups'.
Each group contains repositories with optional dependency relationships.

Supported strategies:
  - independent: Each repo is released separately
  - coordinated: Repos are released in dependency order`,
}

// groupPlanCmd shows the release plan across a repo group.
var groupPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show release plan across repository group",
	Long: `Analyze all repositories in a group and show which ones
have unreleased changes and what versions they would receive.

For coordinated groups, the plan shows dependency-ordered release sequence.`,
	RunE: runGroupPlan,
}

// groupReleaseCmd performs a coordinated release across a group.
var groupReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Coordinated release across repository group",
	Long: `Execute releases across all repositories in a group.

For coordinated strategy, repositories are released in dependency order.
If a repository fails, coordinated releases stop to prevent inconsistency.
For independent strategy, failures in one repo do not affect others.`,
	RunE: runGroupRelease,
}

// groupStatusCmd shows the release state of all repos in a group.
var groupStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show release state of all repositories in group",
	Long:  `Display the current release state, version, and change status for each repository in the group.`,
	RunE:  runGroupStatus,
}

func init() {
	// Group-level flags
	groupCmd.PersistentFlags().StringVar(&groupName, "group", "", "repository group name (required)")
	groupCmd.PersistentFlags().StringSliceVar(&targetRepos, "repo", nil, "target specific repos within the group")

	// Add subcommands
	groupCmd.AddCommand(groupPlanCmd)
	groupCmd.AddCommand(groupReleaseCmd)
	groupCmd.AddCommand(groupStatusCmd)

	rootCmd.AddCommand(groupCmd)
}

// runGroupPlan implements the group plan command.
func runGroupPlan(cmd *cobra.Command, _ []string) error {
	group, coordinator, err := setupGroupCommand()
	if err != nil {
		return err
	}

	plan, err := coordinator.Plan(cmd.Context(), group, targetRepos...)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(plan)
	}

	printGroupPlan(plan)
	return nil
}

// runGroupRelease implements the group release command.
func runGroupRelease(cmd *cobra.Command, _ []string) error {
	group, coordinator, err := setupGroupCommand()
	if err != nil {
		return err
	}

	// Plan first
	plan, err := coordinator.Plan(cmd.Context(), group, targetRepos...)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	if plan.ReposWithChanges == 0 {
		fmt.Println(styles.Info.Render("No repositories have unreleased changes."))
		return nil
	}

	if dryRun {
		fmt.Println(styles.Warning.Render("Dry run - no changes will be made."))
		printGroupPlan(plan)
		return nil
	}

	// Execute
	plan, err = coordinator.Execute(cmd.Context(), group, plan)
	if err != nil {
		return fmt.Errorf("release failed: %w", err)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(plan)
	}

	printGroupReleaseResults(plan)
	return nil
}

// runGroupStatus implements the group status command.
func runGroupStatus(cmd *cobra.Command, _ []string) error {
	group, coordinator, err := setupGroupCommand()
	if err != nil {
		return err
	}

	plan, err := coordinator.Plan(cmd.Context(), group)
	if err != nil {
		return fmt.Errorf("status check failed: %w", err)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(plan)
	}

	printGroupStatus(group, plan)
	return nil
}

// setupGroupCommand validates flags and creates the coordinator.
func setupGroupCommand() (*multirepo.RepositoryGroup, *appmultirepo.Coordinator, error) {
	if groupName == "" {
		return nil, nil, fmt.Errorf("--group flag is required")
	}

	group := findGroup(groupName)
	if group == nil {
		return nil, nil, fmt.Errorf("repository group %q not found in configuration", groupName)
	}

	if err := group.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid group configuration: %w", err)
	}

	// Create a no-op coordinator for CLI. Real adapters would be injected
	// through the container in a production setup.
	coordinator := appmultirepo.NewCoordinator(nil, nil)

	return group, coordinator, nil
}

// findGroup looks up a repository group by name from the config.
func findGroup(name string) *multirepo.RepositoryGroup {
	if cfg == nil {
		return nil
	}
	for i := range cfg.RepositoryGroups {
		if cfg.RepositoryGroups[i].Name == name {
			return &cfg.RepositoryGroups[i]
		}
	}
	return nil
}

// printGroupPlan renders the release plan to stdout.
func printGroupPlan(plan *appmultirepo.MultiRepoPlan) {
	fmt.Println(styles.Title.Render(fmt.Sprintf("Release Plan: %s", plan.GroupName)))
	fmt.Printf("Strategy: %s\n", plan.Strategy)
	fmt.Printf("Repos with changes: %d/%d\n", plan.ReposWithChanges, len(plan.Results))
	fmt.Printf("Total changes: %d\n\n", plan.TotalChanges)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tSTATE\tCURRENT\tNEXT\tCHANGES")

	for _, repoName := range plan.ReleaseOrder {
		result := plan.Results[repoName]
		if result == nil {
			continue
		}
		state := stateIcon(result.State)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			result.Name, state, result.CurrentVersion, result.NextVersion, result.ChangeCount)
	}
	w.Flush()
}

// printGroupReleaseResults renders release results to stdout.
func printGroupReleaseResults(plan *appmultirepo.MultiRepoPlan) {
	fmt.Println(styles.Title.Render(fmt.Sprintf("Release Results: %s", plan.GroupName)))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tSTATE\tVERSION\tERROR")

	for _, repoName := range plan.ReleaseOrder {
		result := plan.Results[repoName]
		if result == nil {
			continue
		}
		state := stateIcon(result.State)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			result.Name, state, result.NextVersion, result.Error)
	}
	w.Flush()
}

// printGroupStatus renders group status to stdout.
func printGroupStatus(group *multirepo.RepositoryGroup, plan *appmultirepo.MultiRepoPlan) {
	fmt.Println(styles.Title.Render(fmt.Sprintf("Group Status: %s", group.Name)))
	fmt.Printf("Strategy: %s\n", group.Strategy)
	fmt.Printf("Repositories: %d\n\n", len(group.Repositories))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tVERSION\tCHANGES\tDEPENDENCIES")

	for _, repo := range group.Repositories {
		result := plan.Results[repo.Name]
		version := "-"
		changes := "0"
		if result != nil {
			if result.CurrentVersion != "" {
				version = result.CurrentVersion
			}
			changes = fmt.Sprintf("%d", result.ChangeCount)
		}
		deps := "-"
		if len(repo.Dependencies) > 0 {
			deps = fmt.Sprintf("%v", repo.Dependencies)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", repo.Name, version, changes, deps)
	}
	w.Flush()
}

// stateIcon returns a display string for a repo release state.
func stateIcon(state appmultirepo.RepoReleaseState) string {
	switch state {
	case appmultirepo.StateReleased:
		return styles.Success.Render("released")
	case appmultirepo.StateFailed:
		return styles.Error.Render("failed")
	case appmultirepo.StateSkipped:
		return styles.Subtle.Render("skipped")
	case appmultirepo.StateReleasing:
		return styles.Warning.Render("releasing")
	case appmultirepo.StatePlanning:
		return styles.Info.Render("planning")
	default:
		return string(state)
	}
}
