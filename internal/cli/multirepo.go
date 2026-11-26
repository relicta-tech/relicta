package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	appmultirepo "github.com/relicta-tech/relicta/v5/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v5/internal/container"
	"github.com/relicta-tech/relicta/v5/internal/domain/multirepo"
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

Repository groups are declared in .relicta.yaml, which relicta init does not
write — add the section yourself:

  repository_groups:
    - name: platform
      strategy: coordinated
      repositories:
        - name: service-a
          path: ../service-a
        - name: service-b
          path: ../service-b
          dependencies: [service-a]

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

For coordinated groups, the plan shows dependency-ordered release sequence. With --repo,
coordinated groups include every transitive dependency of the selected repositories; independent
groups include only the selected repositories. Unknown repository names are errors.`,
	RunE: runGroupPlan,
}

// groupReleaseCmd performs a coordinated release across a group.
var groupReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Coordinated release across repository group",
	Long: `Publish already-approved release runs across repositories in a group.

First prepare and approve a release in each member repository with 'relicta plan',
'relicta bump', 'relicta notes', and 'relicta approve'. 'relicta group plan' is
an estimate only; 'relicta group release' does not create or approve member runs.

For coordinated strategy, repositories are released in dependency order.
If a repository fails, coordinated releases stop to prevent inconsistency.
For independent strategy, failures in one repo do not affect others. With --repo, coordinated
groups include every transitive dependency of the selected repositories; independent groups
include only the selected repositories. Unknown repository names are errors.`,
	RunE: runGroupRelease,
}

// groupStatusCmd shows the release state of all repos in a group.
var groupStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show release state of repositories in group",
	Long:  `Display the current release state, version, and change status for each selected repository in the group.`,
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

	planningFailures := countGroupResults(plan, func(result *appmultirepo.RepoResult) bool {
		return result.State == appmultirepo.StateFailed
	})
	if outputJSON {
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			return err
		}
	} else {
		printGroupPlan(plan)
	}
	if planningFailures > 0 {
		return fmt.Errorf("could not plan %d repositories", planningFailures)
	}
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
	planningFailures := countGroupResults(plan, func(result *appmultirepo.RepoResult) bool {
		return result.State == appmultirepo.StateFailed
	})
	if planningFailures > 0 && group.Strategy == multirepo.StrategyCoordinated {
		if outputJSON {
			if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
				return err
			}
		} else {
			printGroupPlan(plan)
		}
		return fmt.Errorf("could not plan %d repositories; coordinated release was not started", planningFailures)
	}

	if plan.ReposWithChanges == 0 {
		if planningFailures > 0 {
			if outputJSON {
				if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
					return err
				}
			} else {
				printGroupPlan(plan)
			}
			return fmt.Errorf("could not determine release changes for %d repositories", planningFailures)
		}
		if outputJSON {
			return json.NewEncoder(os.Stdout).Encode(plan)
		}
		fmt.Println(styles.Info.Render("No repositories have unreleased changes."))
		return nil
	}

	if dryRun {
		if outputJSON {
			if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
				return err
			}
		} else {
			fmt.Println(styles.Warning.Render("Dry run - no changes will be made."))
			printGroupPlan(plan)
		}
		if planningFailures > 0 {
			return fmt.Errorf("could not plan %d repositories", planningFailures)
		}
		return nil
	}

	// Readiness before execution, and reported for every member at once.
	//
	// Execute refused on the first member with "no release executor is configured", which
	// told the operator that something was unimplemented and nothing about their group.
	// This answers what they were asking: what is blocking this release, and where.
	//
	// Checked before anything runs, because a coordinated release that publishes two of
	// four repositories and then stops leaves the group in a state nobody chose.
	if err := reportGroupReadiness(cmd.Context(), group, plan); err != nil {
		return err
	}

	// Execute
	plan, err = coordinator.Execute(cmd.Context(), group, plan)
	if err != nil {
		return fmt.Errorf("release failed: %w", err)
	}

	if outputJSON {
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			return err
		}
	} else {
		printGroupReleaseResults(plan)
	}

	failed := countGroupResults(plan, func(result *appmultirepo.RepoResult) bool {
		return result.State == appmultirepo.StateFailed
	})
	skipped := countGroupResults(plan, func(result *appmultirepo.RepoResult) bool {
		return result.State == appmultirepo.StateSkipped && result.Error != ""
	})
	if failed > 0 || skipped > 0 {
		return fmt.Errorf("group release finished with %d failed and %d skipped repositories", failed, skipped)
	}
	return nil
}

func countGroupResults(plan *appmultirepo.MultiRepoPlan, match func(*appmultirepo.RepoResult) bool) int {
	count := 0
	for _, result := range plan.Results {
		if result != nil && match(result) {
			count++
		}
	}
	return count
}

// runGroupStatus implements the group status command.
func runGroupStatus(cmd *cobra.Command, _ []string) error {
	group, coordinator, err := setupGroupCommand()
	if err != nil {
		return err
	}

	plan, err := coordinator.Plan(cmd.Context(), group, targetRepos...)
	if err != nil {
		return fmt.Errorf("status check failed: %w", err)
	}

	planningFailures := countGroupResults(plan, func(result *appmultirepo.RepoResult) bool {
		return result.State == appmultirepo.StateFailed
	})
	if outputJSON {
		if err := json.NewEncoder(os.Stdout).Encode(plan); err != nil {
			return err
		}
	} else {
		printGroupStatus(group, plan, groupTargetSelection(group, targetRepos))
	}
	if planningFailures > 0 {
		return fmt.Errorf("could not inspect %d repositories", planningFailures)
	}
	return nil
}

// setupGroupCommand validates flags and creates the coordinator.
func setupGroupCommand() (*multirepo.RepositoryGroup, *appmultirepo.Coordinator, error) {
	if groupName == "" {
		return nil, nil, fmt.Errorf("--group flag is required")
	}

	group := findGroup(groupName)
	if group == nil && (cfg == nil || len(cfg.RepositoryGroups) == 0) {
		// Nothing is declared at all, which is a different situation from a
		// misspelled group name: `relicta init` writes no repository_groups section,
		// so the reader needs the shape, not the key's name.
		hintRepositoryGroups.print()
		return nil, nil, errors.New("no repository groups are declared")
	}
	if group == nil {
		return nil, nil, fmt.Errorf("repository group %q not found in configuration", groupName)
	}

	if err := group.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid group configuration: %w", err)
	}

	// Built by the container, which is the composition root. The CLI must not import
	// internal/infrastructure directly — the hexagonal fitness function in
	// internal/architecture enforces that — and it previously built this coordinator as
	// NewCoordinator(nil, nil), so every `relicta group plan` panicked on the first
	// repository in the group.
	coordinator := container.NewMultirepoCoordinator(configuredTagPrefix())

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
func printGroupStatus(group *multirepo.RepositoryGroup, plan *appmultirepo.MultiRepoPlan, selected map[string]bool) {
	fmt.Println(styles.Title.Render(fmt.Sprintf("Group Status: %s", group.Name)))
	fmt.Printf("Strategy: %s\n", group.Strategy)
	repositoryCount := len(group.Repositories)
	if selected != nil {
		repositoryCount = len(selected)
	}
	fmt.Printf("Repositories: %d\n\n", repositoryCount)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tVERSION\tCHANGES\tDEPENDENCIES")

	for _, repo := range group.Repositories {
		if selected != nil && !selected[repo.Name] {
			continue
		}
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

// groupTargetSelection resolves the same dependency closure used by the group planner so
// status output matches --repo behavior in plan and release commands.
func groupTargetSelection(group *multirepo.RepositoryGroup, targetRepos []string) map[string]bool {
	if len(targetRepos) == 0 {
		return nil
	}
	selected := make(map[string]bool, len(targetRepos))
	for _, name := range targetRepos {
		selected[name] = true
	}
	if group.Strategy != multirepo.StrategyCoordinated {
		return selected
	}
	graph, err := multirepo.NewDependencyGraph(group)
	if err != nil {
		return selected
	}
	queue := append([]string(nil), targetRepos...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, dep := range graph.DirectDependencies(name) {
			if !selected[dep] {
				selected[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	return selected
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

// reportGroupReadiness checks only members that the plan will release. Coordinated groups
// remain all-or-nothing; independent groups skip blocked members and continue with ready ones.
//
// It never approves on behalf of a member: group membership must not bypass that repository's
// own policy. Naming the member that needs approval tells the operator how to unblock it.
func reportGroupReadiness(ctx context.Context, group *multirepo.RepositoryGroup, plan *appmultirepo.MultiRepoPlan) error {
	members := make([]appmultirepo.Member, 0, len(group.Repositories))
	for _, repo := range group.Repositories {
		result := plan.Results[repo.Name]
		if result == nil || result.State == appmultirepo.StateSkipped || !result.HasChanges {
			continue
		}
		members = append(members, appmultirepo.Member{Name: repo.Name, Path: repo.Path})
	}

	states := container.NewMultirepoReadiness().Check(ctx, members)
	ready, blocked := appmultirepo.AllReady(states)
	if ready {
		return nil
	}

	if group.Strategy == multirepo.StrategyIndependent {
		for _, state := range blocked {
			if result := plan.Results[state.Name]; result != nil {
				result.State = appmultirepo.StateSkipped
				result.Error = state.Blocker
			}
		}
		if len(blocked) < len(states) {
			if !outputJSON {
				fmt.Printf("%d independent repositories will be skipped because they are not ready:\n", len(blocked))
				printGroupReadiness(blocked)
			}
			return nil
		}
		if outputJSON {
			if encErr := json.NewEncoder(os.Stdout).Encode(map[string]any{
				"released": false,
				"reason":   "no independent repository is ready to release",
				"plan":     plan,
			}); encErr != nil {
				return encErr
			}
		}
		return fmt.Errorf("none of the %d independent repositories with changes are ready to release", len(states))
	}

	if outputJSON {
		// Refusing is still an answer, and a caller parsing output needs it in the shape
		// they asked for rather than as prose on stderr.
		if encErr := json.NewEncoder(os.Stdout).Encode(map[string]any{
			"released": false,
			"reason":   "not every member is ready to release",
			"members":  states,
		}); encErr != nil {
			return encErr
		}
		return fmt.Errorf("%d of %d repositories are not ready to release", len(blocked), len(states))
	}

	fmt.Println(styles.Warning.Render("This group is not ready to release."))
	printGroupReadiness(states)
	fmt.Println()
	fmt.Println("Nothing was released. A coordinated release that publishes some members and")
	fmt.Println("stops leaves the group in a state nobody chose, so it is refused as a whole.")

	return fmt.Errorf("%d of %d repositories are not ready to release", len(blocked), len(states))
}

func printGroupReadiness(states []appmultirepo.MemberState) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  REPOSITORY\tSTATE\tVERSION\tWHAT IS NEEDED")
	for _, s := range states {
		state, version, needed := s.State, s.Version, s.Blocker
		if state == "" {
			state = "-"
		}
		if version == "" {
			version = "-"
		}
		if s.Ready {
			needed = "ready"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", s.Name, state, version, needed)
	}
	_ = w.Flush()
}
