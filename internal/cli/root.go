// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/felixgeelhaar/release-pilot/internal/config"
)

var (
	// Version information set by main.
	versionInfo struct {
		Version string
		Commit  string
		Date    string
	}

	// Global flags
	cfgFile    string
	verbose    bool
	dryRun     bool
	outputJSON bool
	noColor    bool
	logLevel   string
	modelFlag  string // --model flag for AI provider/model selection
	ciMode     bool   // --ci flag for CI/CD pipeline mode (auto-approve, JSON output)

	// Global config
	cfg *config.Config

	// Logger
	logger *log.Logger

	// logFile holds the log file handle for cleanup
	logFile *os.File

	// Styles
	styles = struct {
		Title   lipgloss.Style
		Success lipgloss.Style
		Error   lipgloss.Style
		Warning lipgloss.Style
		Info    lipgloss.Style
		Subtle  lipgloss.Style
		Bold    lipgloss.Style
	}{
		Title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("33")),
		Subtle:  lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Bold:    lipgloss.NewStyle().Bold(true),
	}
)

// SetVersionInfo sets the version information from main.
func SetVersionInfo(version, commit, date string) {
	versionInfo.Version = version
	versionInfo.Commit = commit
	versionInfo.Date = date
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "release-pilot",
	Short: "AI-powered release management for modern software teams",
	Long: `ReleasePilot is a CLI tool that streamlines software release management.

It automates versioning, changelog generation, and release communication
using AI and a plugin-based integration system.

Key features:
  • Conventional commit parsing for automatic version detection
  • AI-powered changelog and release notes generation
  • Plugin ecosystem for GitHub, npm, Slack, and more
  • Interactive approval workflows
  • Dry-run support for safe operation

Get started with 'release-pilot init' to set up your project.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for init and version commands
		if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}
		return initConfig()
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ExecuteContext runs the root command with a context for graceful shutdown.
func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	// Initialize logger with default settings
	// JSON format and log level are configured in initConfig based on flags
	logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		ReportCaller:    false,
	})

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: release.config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output results as JSON")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-4, anthropic/claude-sonnet-4, local/mistral)")
	rootCmd.PersistentFlags().BoolVar(&ciMode, "ci", false, "CI/CD mode: auto-approve, JSON output, non-interactive")

	// Bind flags to viper
	viper.BindPFlag("output.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("output.color", rootCmd.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("output.log_level", rootCmd.PersistentFlags().Lookup("log-level"))

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(bumpCmd)
	rootCmd.AddCommand(notesCmd)
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(publishCmd)
}

// loadAndValidateConfig loads and validates the configuration.
func loadAndValidateConfig() error {
	loader := config.NewLoader()

	if cfgFile != "" {
		loader.WithConfigPath(cfgFile)
	}

	var err error
	cfg, err = loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return nil
}

// applyGlobalFlags applies global CLI flags to the configuration.
func applyGlobalFlags() {
	if verbose {
		cfg.Output.Verbose = true
	}

	if dryRun {
		cfg.Workflow.DryRunByDefault = true
	}

	if noColor {
		cfg.Output.Color = false
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// applyModelFlag applies the --model flag to the configuration.
func applyModelFlag() {
	if modelFlag == "" {
		return
	}

	provider, model := parseModelFlag(modelFlag)
	if provider != "" {
		cfg.AI.Provider = provider
		cfg.AI.Enabled = true
	}
	if model != "" {
		cfg.AI.Model = model
	}
}

// applyCIModeFlag applies the --ci flag settings.
func applyCIModeFlag() {
	if !ciMode {
		return
	}

	outputJSON = true                       // Force JSON output for machine parsing
	cfg.Workflow.RequireApproval = false    // Skip approval prompts
	noColor = true                          // Disable colors for cleaner logs
	lipgloss.SetColorProfile(termenv.Ascii) // Reset color profile
}

// configureLoggerFormat configures the logger format based on settings.
func configureLoggerFormat() {
	if outputJSON || cfg.Output.Format == "json" {
		logger.SetFormatter(log.JSONFormatter)
		logger.SetReportTimestamp(true)
		logger.SetReportCaller(true)
	} else if !cfg.Output.Color || noColor {
		logger.SetFormatter(log.TextFormatter)
	}
}

// configureLogLevel sets the logger level based on configuration.
func configureLogLevel() {
	switch cfg.Output.LogLevel {
	case "debug":
		logger.SetLevel(log.DebugLevel)
	case "warn":
		logger.SetLevel(log.WarnLevel)
	case "error":
		logger.SetLevel(log.ErrorLevel)
	default:
		logger.SetLevel(log.InfoLevel)
	}

	if cfg.Output.Verbose {
		logger.SetLevel(log.DebugLevel)
	}
}

// configureLogFile sets up log file output if specified.
func configureLogFile() error {
	if cfg.Output.LogFile == "" {
		return nil
	}

	var err error
	logFile, err = os.OpenFile(cfg.Output.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	logger.SetOutput(logFile)
	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	// Load and validate configuration
	if err := loadAndValidateConfig(); err != nil {
		return err
	}

	// Apply CLI flags to configuration
	applyGlobalFlags()
	applyModelFlag()
	applyCIModeFlag()

	// Configure logger
	configureLoggerFormat()
	configureLogLevel()

	// Configure log file
	return configureLogFile()
}

// Cleanup closes any open resources. Should be called before program exit.
func Cleanup() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// versionCmd prints version information.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("release-pilot %s\n", versionInfo.Version)
		if verbose {
			fmt.Printf("  commit: %s\n", versionInfo.Commit)
			fmt.Printf("  built:  %s\n", versionInfo.Date)
		}
	},
}

// Placeholder commands (to be implemented in separate files)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new release-pilot configuration",
	Long: `Initialize a new release-pilot configuration in the current directory.

This command creates a release.config.yaml file with sensible defaults
and guides you through the initial setup.`,
	RunE: runInit,
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Analyze changes and plan the next release",
	Long: `Analyze commits since the last release and suggest a version bump.

This command examines your commit history using conventional commits
to determine what type of release is needed (major, minor, or patch).`,
	RunE: runPlan,
}

var bumpCmd = &cobra.Command{
	Use:   "bump",
	Short: "Calculate and apply a version bump",
	Long: `Calculate the next version based on commits and apply the bump.

This command updates version tags and optionally version files.`,
	Aliases: []string{"version-bump"},
	RunE:    runVersion,
}

var notesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Generate changelog and release notes",
	Long: `Generate changelog entries and release notes for the current release.

This command creates human-readable release documentation from your
commit history, optionally using AI to enhance the content.`,
	RunE: runNotes,
}

var approveCmd = &cobra.Command{
	Use:   "approve",
	Short: "Review and approve the release",
	Long: `Review the prepared release and approve it for publishing.

This command presents the release summary and allows you to
review and edit the release notes before publishing.`,
	RunE: runApprove,
}

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Execute the release",
	Long: `Execute the release by creating tags, updating changelog, and
running configured plugins.

This command performs all the release actions including:
- Creating and pushing git tags
- Updating the changelog file
- Running plugins (GitHub release, npm publish, Slack notification)`,
	RunE: runPublish,
}

// Helper functions for output

func printSuccess(msg string) {
	fmt.Println(styles.Success.Render("✓ " + msg))
}

func printError(msg string) {
	fmt.Println(styles.Error.Render("✗ " + msg))
}

func printWarning(msg string) {
	fmt.Println(styles.Warning.Render("⚠ " + msg))
}

func printInfo(msg string) {
	fmt.Println(styles.Info.Render("ℹ " + msg))
}

func printTitle(msg string) {
	fmt.Println(styles.Title.Render(msg))
}

func printSubtle(msg string) {
	fmt.Println(styles.Subtle.Render(msg))
}

// IsCIMode returns true if running in CI/CD mode.
func IsCIMode() bool {
	return ciMode
}

// IsJSONOutput returns true if JSON output is enabled.
func IsJSONOutput() bool {
	return outputJSON
}

// parseModelFlag parses the --model flag in the format provider/model.
// Supported formats:
//   - "provider/model" (e.g., "ollama/llama3.2", "openai/gpt-4")
//   - "local/model" (alias for "ollama/model")
//   - "model" (uses default provider from config)
//
// Returns the provider and model name.
func parseModelFlag(flag string) (provider, model string) {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return "", ""
	}

	parts := strings.SplitN(flag, "/", 2)
	if len(parts) == 2 {
		provider = strings.ToLower(parts[0])
		model = parts[1]

		// "local" is an alias for "ollama"
		if provider == "local" {
			provider = "ollama"
		}
	} else {
		// No provider specified, just the model name
		model = flag
	}

	return provider, model
}
