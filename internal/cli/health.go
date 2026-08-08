// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/config"
	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// HealthStatus represents the overall health status.
type HealthStatus string

const (
	// HealthStatusHealthy indicates all checks passed.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates some non-critical checks failed.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates critical checks failed.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	Name    string            `json:"name"`
	Status  HealthStatus      `json:"status"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
	Latency time.Duration     `json:"latency_ms,omitempty"`
}

// HealthReport contains the full health check results.
type HealthReport struct {
	Status      HealthStatus      `json:"status"`
	Version     string            `json:"version"`
	Timestamp   time.Time         `json:"timestamp"`
	Components  []ComponentHealth `json:"components"`
	Environment map[string]string `json:"environment,omitempty"`
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of relicta and its dependencies",
	Long: `Perform health checks on relicta and its dependencies.

This command verifies:
  - Git availability and repository status
  - Configuration validity
  - Plugin connectivity
  - AI service availability (if enabled)

Exit codes:
  0 - All checks passed (healthy)
  1 - Some non-critical checks failed (degraded)
  2 - Critical checks failed (unhealthy)`,
	RunE: runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func runHealth(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	report := &HealthReport{
		Status:      HealthStatusHealthy,
		Version:     versionInfo.Version,
		Timestamp:   time.Now().UTC(),
		Components:  make([]ComponentHealth, 0),
		Environment: make(map[string]string),
	}

	// Collect environment info
	report.Environment["go_version"] = runtime.Version()
	report.Environment["os"] = runtime.GOOS
	report.Environment["arch"] = runtime.GOARCH

	// Run health checks
	checks := []struct {
		name     string
		check    func(context.Context) ComponentHealth
		critical bool
	}{
		{"git", checkGit, true},
		{"repository", checkRepository, true},
		{"config", checkConfig, false},
		{"plugins_directory", checkPluginsDir, false},
	}

	for _, c := range checks {
		health := c.check(ctx)
		report.Components = append(report.Components, health)

		// Update overall status based on component health
		switch health.Status {
		case HealthStatusUnhealthy:
			if c.critical {
				report.Status = HealthStatusUnhealthy
			} else if report.Status == HealthStatusHealthy {
				report.Status = HealthStatusDegraded
			}
		case HealthStatusDegraded:
			if report.Status == HealthStatusHealthy {
				report.Status = HealthStatusDegraded
			}
		}
	}

	// Output results
	if outputJSON {
		return outputHealthJSON(report)
	}
	return outputHealthText(report)
}

func checkGit(ctx context.Context) ComponentHealth {
	start := time.Now()
	health := ComponentHealth{
		Name:    "git",
		Details: make(map[string]string),
	}

	// Check if git is available
	cmd := exec.CommandContext(ctx, "git", "--version")
	output, err := cmd.Output()
	health.Latency = time.Since(start)

	if err != nil {
		health.Status = HealthStatusUnhealthy
		health.Message = "git is not installed or not in PATH"
		return health
	}

	health.Details["version"] = strings.TrimSpace(string(output))
	health.Status = HealthStatusHealthy
	health.Message = "git is available"
	return health
}

// checkRepository reports repository health using the same git service every
// command uses, rather than its own `git rev-parse` subprocess.
//
// The two disagreed: `git rev-parse` walks up to the repository root, while the
// service did not, so from a subdirectory health reported the repository as OK
// while every real command failed to open it. #197 fixed the service; asking it
// directly is what stops the two drifting apart again (issue #199).
func checkRepository(ctx context.Context) ComponentHealth {
	start := time.Now()
	health := ComponentHealth{
		Name:    "repository",
		Details: make(map[string]string),
	}

	svc, err := gitservice.NewService()
	if err != nil {
		health.Latency = time.Since(start)
		health.Status = HealthStatusDegraded
		health.Message = "not in a git repository"
		return health
	}

	info, err := gitservice.NewAdapter(svc).GetInfo(ctx)
	health.Latency = time.Since(start)
	if err != nil {
		health.Status = HealthStatusDegraded
		health.Message = fmt.Sprintf("git repository could not be read: %v", err)
		return health
	}

	// Report the resolved root: a boolean hid the divergence that caused #193
	// and #199, whereas a path makes it visible at a glance.
	health.Details["root"] = info.Path
	if info.CurrentBranch != "" {
		health.Details["branch"] = info.CurrentBranch
	}
	if info.Owner != "" && info.Name != "" {
		health.Details["repository"] = info.Owner + "/" + info.Name
	}
	health.Details["uncommitted_changes"] = strconv.FormatBool(info.IsDirty)

	health.Status = HealthStatusHealthy
	health.Message = fmt.Sprintf("git repository detected at %s", info.Path)
	return health
}

func checkConfig(ctx context.Context) ComponentHealth {
	start := time.Now()
	health := ComponentHealth{
		Name:    "config",
		Details: make(map[string]string),
	}

	// Ask the loader which config applies, rather than stat-ing a hardcoded list
	// in the working directory. The old list also drifted from the loader's own
	// names, which are derived from ConfigFileNames x ConfigFileExtensions.
	configFile, err := config.ResolveConfigFile()
	health.Latency = time.Since(start)

	if err != nil {
		health.Status = HealthStatusDegraded
		health.Message = "no configuration file found (run 'relicta init' to create one); built-in defaults apply"
		return health
	}

	// Report the path rather than a boolean: from a subdirectory the applicable
	// config lives at the repository root, and "found" alone hid that.
	health.Details["config_file"] = configFile
	health.Status = HealthStatusHealthy
	health.Message = fmt.Sprintf("configuration file found: %s", configFile)
	return health
}

func checkPluginsDir(ctx context.Context) ComponentHealth {
	start := time.Now()
	health := ComponentHealth{
		Name:    "plugins_directory",
		Details: make(map[string]string),
	}

	// Check standard plugin directories
	pluginDirs := []string{
		".relicta/plugins",
		"/usr/local/lib/relicta/plugins",
	}

	// Also check home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		pluginDirs = append(pluginDirs, homeDir+"/.relicta/plugins")
	}

	var foundDirs []string
	for _, dir := range pluginDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			foundDirs = append(foundDirs, dir)
		}
	}

	health.Latency = time.Since(start)

	if len(foundDirs) == 0 {
		health.Status = HealthStatusHealthy
		health.Message = "no plugin directories found (plugins will use system PATH)"
		return health
	}

	health.Details["directories"] = strings.Join(foundDirs, ", ")
	health.Status = HealthStatusHealthy
	health.Message = fmt.Sprintf("found %d plugin director(y/ies)", len(foundDirs))
	return health
}

func outputHealthJSON(report *HealthReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}

	return exitWithHealthStatus(report.Status)
}

func outputHealthText(report *HealthReport) error {
	// Print header
	statusIcon := "?"
	switch report.Status {
	case HealthStatusHealthy:
		statusIcon = styles.Success.Render("healthy")
	case HealthStatusDegraded:
		statusIcon = styles.Warning.Render("degraded")
	case HealthStatusUnhealthy:
		statusIcon = styles.Error.Render("unhealthy")
	}

	fmt.Printf("Health Status: %s\n", statusIcon)
	fmt.Printf("Version: %s\n", report.Version)
	fmt.Printf("Timestamp: %s\n\n", report.Timestamp.Format(time.RFC3339))

	// Print components
	fmt.Println("Components:")
	for _, c := range report.Components {
		icon := "?"
		switch c.Status {
		case HealthStatusHealthy:
			icon = styles.Success.Render("[OK]")
		case HealthStatusDegraded:
			icon = styles.Warning.Render("[WARN]")
		case HealthStatusUnhealthy:
			icon = styles.Error.Render("[FAIL]")
		}

		latencyStr := ""
		if c.Latency > 0 {
			latencyStr = fmt.Sprintf(" (%dms)", c.Latency.Milliseconds())
		}

		fmt.Printf("  %s %s: %s%s\n", icon, c.Name, c.Message, latencyStr)

		// Print details if verbose
		if verbose && len(c.Details) > 0 {
			for k, v := range c.Details {
				fmt.Printf("      %s: %s\n", k, v)
			}
		}
	}

	// Print environment if verbose
	if verbose && len(report.Environment) > 0 {
		fmt.Println("\nEnvironment:")
		for k, v := range report.Environment {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return exitWithHealthStatus(report.Status)
}

// healthStatusExitCode maps a health status to its process exit code:
// 0 = healthy, 1 = degraded, 2 = unhealthy. Any unknown status maps to 0 so an
// unrecognized state never wedges the process on a non-zero exit. Pure so the
// mapping can be tested without forking a process.
func healthStatusExitCode(status HealthStatus) int {
	switch status {
	case HealthStatusDegraded:
		return 1
	case HealthStatusUnhealthy:
		return 2
	default:
		return 0
	}
}

// exitWithHealthStatusHook performs the process exit for a health status. It is
// a var so tests can substitute a capturing implementation in place of the real
// os.Exit side effect.
var exitWithHealthStatusHook = func(status HealthStatus) error {
	if code := healthStatusExitCode(status); code != 0 {
		os.Exit(code)
	}
	return nil
}

func exitWithHealthStatus(status HealthStatus) error {
	return exitWithHealthStatusHook(status)
}
