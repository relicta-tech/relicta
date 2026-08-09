// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/logging"
	"github.com/relicta-tech/relicta/v4/internal/security"
)

var (
	// Version information set by main.
	versionInfo struct {
		Version string
		Commit  string
		Date    string
	}

	// Global flags
	cfgFile               string
	verbose               bool
	dryRun                bool
	outputJSON            bool
	noColor               bool
	logLevel              string
	logLevelAlias         string
	logLevelExplicit      bool
	modelFlag             string // --model flag for AI provider/model selection
	ciMode                bool   // --ci flag for CI/CD pipeline mode (auto-approve, JSON output)
	redactSecrets         bool   // --redact flag to mask sensitive data in output
	versionProbeCognitive bool

	// allowUntrustedPlugins is the operator opt-in that bypasses the
	// plugin-sandbox trust gate. Required on best-effort sandbox platforms
	// (e.g. macOS) until plugin signature verification ships. See
	// `relicta plugin sandbox-status` for posture details.
	allowUntrustedPlugins bool

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

	// Setting rootCmd.Version is what makes `relicta --version` work. Without it
	// Cobra registers no such flag, so the near-universal first thing anyone
	// types at a new CLI failed with "unknown flag: --version" while only the
	// `version` subcommand worked — and `-v` is already --verbose, so the
	// shorthand guess failed too.
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("relicta {{.Version}}\n")
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "relicta",
	Short: "The governance layer for software change",

	// Present from declaration so Cobra always registers --version. Setting it
	// only inside SetVersionInfo would make the flag's existence depend on main
	// remembering to call that, and its absence is invisible until someone types
	// `relicta --version` and gets "unknown flag". SetVersionInfo overwrites this
	// placeholder with the real build version.
	Version: "dev",
	Long: `Relicta is the governance layer for software change.

As AI agents and CI systems generate more code, deciding what should ship
becomes the hardest problem. Relicta governs change — before it reaches
production.

The Change Governance Protocol (CGP):
  • Risk assessment — Analyze blast radius and impact of every change
  • Audit trails — Complete history of approvals and decisions
  • Approval workflows — Gate releases with configurable policies
  • AI-powered insights — Intelligent release notes and risk analysis

Today, it's a production-ready CLI for semantic versioning, changelogs,
and release automation. Tomorrow, it's the decision layer for risk-aware
releases in an AI-driven world.

Get started with 'relicta init' to set up your project.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Record whether the operator explicitly set a log level on this
		// invocation. configureSlog needs this to tell `--log-level info`
		// (an explicit opt-in that must raise the slog floor) apart from the
		// "info" default the flag carries when untouched.
		logLevelExplicit = cmd.Flags().Changed("log-level") || cmd.Flags().Changed("log")
		// Skip config loading for commands that don't need it
		if cmd.Name() == "init" || cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "verify" || cmd.Name() == "report" || cmd.Name() == "plugin" || cmd.Name() == "mcp" || cmd.Name() == "policy" || cmd.Parent() != nil && (cmd.Parent().Name() == "plugin" || cmd.Parent().Name() == "mcp" || cmd.Parent().Name() == "policy") {
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

// RootCommand returns the root command. Exposed for documentation generation
// (tools/gendocs walks the command tree to emit docs/cli/*.md). Do not use it
// to mutate command state at runtime.
func RootCommand() *cobra.Command {
	return rootCmd
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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: .relicta.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output results as JSON")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logLevelAlias, "log", "", "alias for --log-level")
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-4, anthropic/claude-sonnet-4, local/mistral)")
	rootCmd.PersistentFlags().BoolVar(&ciMode, "ci", false, "CI/CD mode: auto-approve, JSON output, non-interactive")
	rootCmd.PersistentFlags().BoolVar(&redactSecrets, "redact", false, "redact secrets and API keys from output (auto-enabled in CI mode)")
	rootCmd.PersistentFlags().BoolVar(&allowUntrustedPlugins, "allow-untrusted-plugins", false, "load plugins on best-effort sandbox platforms; review 'relicta plugin sandbox-status' first")
	versionCmd.Flags().BoolVar(&versionProbeCognitive, "cognitive", false, "probe Mnemos and Chronos health status")

	// Bind flags to viper (errors are non-fatal for flag binding)
	_ = viper.BindPFlag("output.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("output.color", rootCmd.PersistentFlags().Lookup("no-color"))
	_ = viper.BindPFlag("output.log_level", rootCmd.PersistentFlags().Lookup("log-level"))

	// Add subcommands
	// Cobra command groups — slot commands by user intent so `relicta --help`
	// reads as a structured menu rather than a 30-item flat list (Hick's Law).
	rootCmd.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Release Lifecycle:"},
		&cobra.Group{ID: "inspect", Title: "Inspect & Report:"},
		&cobra.Group{ID: "governance", Title: "Governance:"},
		&cobra.Group{ID: "extend", Title: "Extend (MCP, Plugins, Server):"},
		&cobra.Group{ID: "ops", Title: "Operations:"},
		&cobra.Group{ID: "integrations", Title: "Integrations (Vanta, Drata):"},
	)

	// Lifecycle: the canonical release flow.
	initCmd.GroupID = "lifecycle"
	planCmd.GroupID = "lifecycle"
	bumpCmd.GroupID = "lifecycle"
	notesCmd.GroupID = "lifecycle"
	approveCmd.GroupID = "lifecycle"
	publishCmd.GroupID = "lifecycle"
	releaseCmd.GroupID = "lifecycle"
	promoteCmd.GroupID = "lifecycle"
	cancelCmd.GroupID = "lifecycle"
	resetCmd.GroupID = "lifecycle"

	// Every command needs a group, or Cobra files it under "Additional Commands".
	// Only 17 of 32 were assigned, so that bucket had grown to hold `status`,
	// `evaluate`, `health`, `history`, `verify` and `rollback` — the commands
	// people reach for most — alongside `completion` and `demo`, while
	// "Governance:" listed `policy` alone. The grouping was doing the opposite of
	// its stated job.
	rollbackCmd.GroupID = "lifecycle"

	// Inspect & report: read-only views and compliance bundles.
	statusCmd.GroupID = "inspect"
	historyCmd.GroupID = "inspect"
	blastCmd.GroupID = "inspect"
	analyticsCmd.GroupID = "inspect"
	reportCmd.GroupID = "inspect"
	communicateCmd.GroupID = "inspect"
	evalCmd.GroupID = "inspect"

	// Governance: policy authoring, evaluation, audit.
	policyCmd.GroupID = "governance"
	evaluateCmd.GroupID = "governance"
	verifyCmd.GroupID = "governance"
	groupCmd.GroupID = "governance"

	// Extend: agent integrations and headless servers.
	mcpCmd.GroupID = "extend"
	pluginCmd.GroupID = "extend"
	serverCmd.GroupID = "extend"
	metricsCmd.GroupID = "extend"

	// Integrations: third-party GRC + evidence push.
	integrationsCmd.GroupID = "integrations"

	// Ops: meta and housekeeping.
	versionCmd.GroupID = "ops"
	healthCmd.GroupID = "ops"
	cleanCmd.GroupID = "ops"
	demoCmd.GroupID = "ops"

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(bumpCmd)
	rootCmd.AddCommand(notesCmd)
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(releaseCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(policyCmd)
	rootCmd.AddCommand(promoteCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(integrationsCmd)
	rootCmd.AddCommand(evalCmd)
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

	// The --log-level flag is BindPFlag'd to the global viper, but cfg is built
	// by config.NewLoader()'s own viper instance — so the binding never reaches
	// cfg. Copy it explicitly when the operator set it (mirrors --verbose).
	if logLevelExplicit {
		cfg.Output.LogLevel = logLevel
	}

	if dryRun {
		cfg.Workflow.DryRunByDefault = true
	}

	if noColor {
		cfg.Output.Color = false
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// applyLogAlias maps --log to --log-level when set.
func applyLogAlias() {
	if logLevelAlias != "" {
		logLevel = logLevelAlias
		cfg.Output.LogLevel = logLevelAlias
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
	security.Enable()                       // Auto-enable secret masking in CI
}

// applyRedactSecretsFlag enables secret masking if --redact flag is set.
func applyRedactSecretsFlag() {
	if redactSecrets {
		security.Enable()
	}
	// Also check for CI environment variables even if --redact not explicitly set
	security.EnableInCI()
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

// configureSlog routes operational slog logging through Bolt to stderr. The
// default level is WARN so routine INFO chatter (e.g. "release services
// initialized") never pollutes normal command output; --verbose or an
// explicit --log-level raises it. This is the single hygiene swap point —
// all slog.Default() call sites across the codebase inherit it.
func configureSlog() {
	// The operational slog firehose (release services, plugin loads, CGP
	// evaluation traces) is internal plumbing, not command output. It stays at
	// WARN by default so routine INFO chatter never pollutes stdout consumers,
	// and rises only when the operator explicitly opts in via --log-level /
	// --log, an output.log_level entry in the config file, or --verbose.
	level := slog.LevelWarn
	// Detect explicit opt-in without referencing rootCmd (avoids a package
	// init cycle): the --log alias is non-empty, --log-level was moved off its
	// "info" default, or the config file carries an output.log_level entry.
	explicit := logLevelExplicit || logLevelAlias != "" || viper.InConfig("output.log_level")
	if explicit {
		level = logging.LevelFromString(cfg.Output.LogLevel)
	}
	if cfg.Output.Verbose {
		level = slog.LevelDebug
	}

	out := io.Writer(os.Stderr)
	if logFile != nil {
		out = logFile
	}
	logging.Configure(level, out)
}

// configureLogFile sets up log file output if specified.
func configureLogFile() error {
	if cfg.Output.LogFile == "" {
		return nil
	}

	var err error
	logFile, err = os.OpenFile(cfg.Output.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermPrivate)
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
	applyLogAlias()
	applyModelFlag()
	applyCIModeFlag()
	applyRedactSecretsFlag()

	// Configure logger
	configureLoggerFormat()
	configureLogLevel()

	// Configure log file (may redirect output)
	if err := configureLogFile(); err != nil {
		return err
	}

	// Route operational slog logging through Bolt (stderr or log file).
	// Done last so it picks up any --log-file redirect.
	configureSlog()
	return nil
}

// Cleanup closes any open resources. Should be called before program exit.
func Cleanup() {
	if logFile != nil {
		_ = logFile.Close() // Error on cleanup is logged but not propagated
		logFile = nil
	}
}

// versionCmd prints version information.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("relicta %s\n", versionInfo.Version)
		if verbose {
			fmt.Printf("  commit: %s\n", versionInfo.Commit)
			fmt.Printf("  built:  %s\n", versionInfo.Date)
		}
		if versionProbeCognitive {
			mnemosBase := os.Getenv("RELICTA_MNEMOS_ENDPOINT")
			if mnemosBase == "" {
				mnemosBase = "http://localhost:7777"
			}
			chronosBase := os.Getenv("RELICTA_CHRONOS_ENDPOINT")
			if chronosBase == "" {
				chronosBase = "http://localhost:7778"
			}

			fmt.Printf("  mnemos:  %s (%s)\n", probeBackendHealth(mnemosBase), mnemosBase)
			fmt.Printf("  chronos: %s (%s)\n", probeBackendHealth(chronosBase), chronosBase)
		}
	},
}

// Placeholder commands (to be implemented in separate files)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new relicta configuration",
	Long: `Initialize a new relicta configuration in the current directory.

By default this is zero-config: relicta detects your project from its git
remote and manifests, writes a .relicta.yaml with sensible defaults, and
prints next steps — no prompts. Pass --guided for the 8-step interactive
setup wizard, or --force to overwrite an existing config.`,
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
// All print functions mask sensitive data when masking is enabled.

// humanOutputSuppressed reports whether prose belongs on stdout right now.
//
// With --json, stdout is a JSON document and nothing else may share it. That was
// not enforced, so `relicta plan --json` emitted a "Release Plan" heading before
// its object and produced output no parser accepts:
//
//	$ relicta plan --json | jq .
//	parse error: Invalid numeric literal at line 1, column 8
//
// Guarding each call site was the other option and it does not hold: the next
// printTitle someone adds silently breaks the contract again, in a command whose
// tests probably never parse its output. Suppressing at the helper makes the rule
// structural — decorative output cannot reach stdout in JSON mode, wherever it is
// called from.
//
// Errors are exempt. printError already writes to stderr, which is the right
// place for diagnostics whether or not stdout is machine-readable.
func humanOutputSuppressed() bool {
	return outputJSON
}

func printSuccess(msg string) {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Success.Render("✓ " + security.Mask(msg)))
}

// printError writes a masked error to stderr. Errors are diagnostics, not
// command output — keeping them off stdout means machine consumers parsing a
// command's stdout (e.g. --json) never see error chrome mixed into the data.
func printError(msg string) {
	fmt.Fprintln(os.Stderr, styles.Error.Render("✗ "+security.Mask(msg)))
}

// printErrorResult renders an error-styled line to stdout. Use it for failure
// rows inside a results table (alongside printSuccess/printInfo), where the
// line is part of the command's output — not for top-level diagnostics, which
// belong on stderr via printError.
func printErrorResult(msg string) {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Error.Render("✗ " + security.Mask(msg)))
}

// ReportError prints a top-level command error to stderr. cobra runs with
// SilenceErrors, so without this a non-zero exit from Execute would be silent
// (issue: `relicta plan` exited 1 with no message). Call once from main.
func ReportError(err error) {
	if err == nil {
		return
	}
	printError(err.Error())
}

func printWarning(msg string) {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Warning.Render("⚠ " + security.Mask(msg)))
}

func printInfo(msg string) {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Info.Render("ℹ " + security.Mask(msg)))
}

func printDryRunBanner() {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Warning.Render("⚠ DRY RUN MODE - no changes will be made"))
	fmt.Println()
}

func printTitle(msg string) {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Title.Render(security.Mask(msg)))
}

func printSubtle(msg string) {
	if humanOutputSuppressed() {
		return
	}
	fmt.Println(styles.Subtle.Render(security.Mask(msg)))
}

func probeBackendHealth(baseURL string) string {
	client := &http.Client{Timeout: 2 * time.Second}
	base := strings.TrimRight(baseURL, "/")
	candidates := []string{base + "/health", base + "/healthz"}

	for _, u := range candidates {
		resp, err := client.Get(u) // #nosec G107 G704 -- URL is operator-configured local endpoint
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return "healthy"
		}
	}

	return "unreachable"
}

// Spinner provides a simple CLI loading indicator. It animates on stderr only
// when stderr is an interactive terminal — never in CI, when --json is set, or
// when output is piped. This keeps cursor-control escape sequences (\r, \033[K)
// out of redirected output, where they previously leaked as "[K" artifacts.
type Spinner struct {
	message string
	stop    chan struct{}
	done    chan struct{}
	active  bool
}

// spinnerEnabled reports whether an animated spinner is appropriate: stderr is
// a TTY and the operator hasn't requested machine-readable / CI output.
func spinnerEnabled() bool {
	if outputJSON || ciMode {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// NewSpinner creates a new spinner with a message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		active:  spinnerEnabled(),
	}
}

// Start begins the spinner animation. No-op on a non-interactive stderr.
func (s *Spinner) Start() {
	if !s.active {
		return
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stop:
				// Clear the spinner line.
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s %s", styles.Info.Render(frames[i%len(frames)]), s.message)
				i++
			}
		}
	}()
}

// Stop stops the spinner. No-op (no stray newline) when it never animated.
func (s *Spinner) Stop() {
	if !s.active {
		return
	}
	close(s.stop)
	<-s.done
}

// StopWithSuccess stops the spinner and shows a success message.
func (s *Spinner) StopWithSuccess(msg string) {
	s.Stop()
	printSuccess(msg)
}

// StopWithError stops the spinner and shows an error message.
func (s *Spinner) StopWithError(msg string) {
	s.Stop()
	printError(msg)
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
