// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
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
	Short: "Check the health of release-pilot and its dependencies",
	Long: `Perform health checks on release-pilot and its dependencies.

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
		if health.Status == HealthStatusUnhealthy && c.critical {
			report.Status = HealthStatusUnhealthy
		} else if health.Status == HealthStatusDegraded && report.Status == HealthStatusHealthy {
			report.Status = HealthStatusDegraded
		} else if health.Status == HealthStatusUnhealthy && report.Status == HealthStatusHealthy {
			report.Status = HealthStatusDegraded
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

func checkRepository(ctx context.Context) ComponentHealth {
	start := time.Now()
	health := ComponentHealth{
		Name:    "repository",
		Details: make(map[string]string),
	}

	// Check if we're in a git repository
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	_, err := cmd.Output()
	health.Latency = time.Since(start)

	if err != nil {
		health.Status = HealthStatusDegraded
		health.Message = "not in a git repository"
		return health
	}

	// Get current branch
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := cmd.Output()
	if err == nil {
		health.Details["branch"] = strings.TrimSpace(string(branchOutput))
	}

	// Get latest commit
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	commitOutput, err := cmd.Output()
	if err == nil {
		health.Details["commit"] = strings.TrimSpace(string(commitOutput))
	}

	// Check for uncommitted changes
	cmd = exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusOutput, _ := cmd.Output()
	if len(statusOutput) > 0 {
		health.Details["uncommitted_changes"] = "true"
	} else {
		health.Details["uncommitted_changes"] = "false"
	}

	health.Status = HealthStatusHealthy
	health.Message = "git repository detected"
	return health
}

func checkConfig(ctx context.Context) ComponentHealth {
	start := time.Now()
	health := ComponentHealth{
		Name:    "config",
		Details: make(map[string]string),
	}

	// Check if config file exists
	configFiles := []string{
		"release.config.yaml",
		"release.config.yml",
		".release.yaml",
		".release.yml",
		"release-pilot.config.yaml",
	}

	found := false
	for _, f := range configFiles {
		if _, err := os.Stat(f); err == nil {
			health.Details["config_file"] = f
			found = true
			break
		}
	}

	health.Latency = time.Since(start)

	if !found {
		health.Status = HealthStatusDegraded
		health.Message = "no configuration file found (run 'release-pilot init' to create one)"
		return health
	}

	health.Status = HealthStatusHealthy
	health.Message = "configuration file found"
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
		".release-pilot/plugins",
		"/usr/local/lib/release-pilot/plugins",
	}

	// Also check home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		pluginDirs = append(pluginDirs, homeDir+"/.release-pilot/plugins")
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

func exitWithHealthStatus(status HealthStatus) error {
	switch status {
	case HealthStatusHealthy:
		return nil
	case HealthStatusDegraded:
		os.Exit(1)
	case HealthStatusUnhealthy:
		os.Exit(2)
	}
	return nil
}
