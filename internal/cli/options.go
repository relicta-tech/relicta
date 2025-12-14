// Package cli provides the command-line interface for Relicta.
package cli

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"github.com/relicta-tech/relicta/internal/config"
)

// Options holds the CLI runtime options and dependencies.
// This struct encapsulates the global state for better testability
// and dependency injection.
type Options struct {
	// Version information
	Version VersionInfo

	// Global flags
	ConfigFile string
	Verbose    bool
	DryRun     bool
	JSONOutput bool
	NoColor    bool
	LogLevel   string
	Model      string
	CIMode     bool

	// Runtime state
	Config  *config.Config
	Logger  *log.Logger
	LogFile *os.File
	Styles  Styles

	// I/O streams (for testing)
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// VersionInfo holds version metadata.
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

// Styles holds the CLI styling configuration.
type Styles struct {
	Title   lipgloss.Style
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
	Subtle  lipgloss.Style
	Bold    lipgloss.Style
}

// DefaultStyles returns the default CLI styles.
func DefaultStyles() Styles {
	return Styles{
		Title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("33")),
		Subtle:  lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Bold:    lipgloss.NewStyle().Bold(true),
	}
}

// NewOptions creates a new Options instance with default values.
func NewOptions() *Options {
	return &Options{
		Styles: DefaultStyles(),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		Logger: log.NewWithOptions(os.Stderr, log.Options{
			ReportTimestamp: true,
			ReportCaller:    false,
		}),
	}
}

// SetVersion sets the version information.
func (o *Options) SetVersion(version, commit, date string) {
	o.Version.Version = version
	o.Version.Commit = commit
	o.Version.Date = date
}

// IsCI returns true if running in CI/CD mode.
func (o *Options) IsCI() bool {
	return o.CIMode
}

// IsJSON returns true if JSON output is enabled.
func (o *Options) IsJSON() bool {
	return o.JSONOutput
}

// IsDryRun returns true if dry-run mode is enabled.
func (o *Options) IsDryRun() bool {
	return o.DryRun || (o.Config != nil && o.Config.Workflow.DryRunByDefault)
}

// IsVerbose returns true if verbose output is enabled.
func (o *Options) IsVerbose() bool {
	return o.Verbose || (o.Config != nil && o.Config.Output.Verbose)
}

// Cleanup closes any open resources.
func (o *Options) Cleanup() {
	if o.LogFile != nil {
		o.LogFile.Close()
		o.LogFile = nil
	}
}

// PrintSuccess prints a success message.
func (o *Options) PrintSuccess(msg string) {
	o.println(o.Styles.Success.Render("✓ " + msg))
}

// PrintError prints an error message.
func (o *Options) PrintError(msg string) {
	o.println(o.Styles.Error.Render("✗ " + msg))
}

// PrintWarning prints a warning message.
func (o *Options) PrintWarning(msg string) {
	o.println(o.Styles.Warning.Render("⚠ " + msg))
}

// PrintInfo prints an info message.
func (o *Options) PrintInfo(msg string) {
	o.println(o.Styles.Info.Render("ℹ " + msg))
}

// PrintTitle prints a title.
func (o *Options) PrintTitle(msg string) {
	o.println(o.Styles.Title.Render(msg))
}

// PrintSubtle prints subtle/muted text.
func (o *Options) PrintSubtle(msg string) {
	o.println(o.Styles.Subtle.Render(msg))
}

func (o *Options) println(s string) {
	if o.Stdout != nil {
		o.Stdout.Write([]byte(s + "\n"))
	}
}

// CommandOptions holds options for a specific command.
// Embed this in command-specific option structs.
type CommandOptions struct {
	*Options
}

// PlanOptions holds options for the plan command.
type PlanOptions struct {
	CommandOptions
	FromRef string
	ToRef   string
	ShowAll bool
	Minimal bool
}

// BumpOptions holds options for the bump command.
type BumpOptions struct {
	CommandOptions
	ReleaseType     string
	Prerelease      string
	BuildMetadata   string
	SkipTag         bool
	SkipPush        bool
	ForceVersion    string
	GenerateNotes   bool
	UpdateChangelog bool
	NoGitTag        bool
	NoCommit        bool
}

// NotesOptions holds options for the notes command.
type NotesOptions struct {
	CommandOptions
	NoAI        bool
	OutputFile  string
	Template    string
	Format      string
	FromVersion string
}

// ApproveOptions holds options for the approve command.
type ApproveOptions struct {
	CommandOptions
	Edit bool
}

// PublishOptions holds options for the publish command.
type PublishOptions struct {
	CommandOptions
	SkipPlugins []string
	OnlyPlugins []string
	Force       bool
}

// HealthOptions holds options for the health command.
type HealthOptions struct {
	CommandOptions
}

// MetricsOptions holds options for the metrics command.
type MetricsOptions struct {
	CommandOptions
}

// BlastOptions holds options for the blast command.
type BlastOptions struct {
	CommandOptions
	Paths         []string
	IncludeShared bool
	OutputFormat  string
}
