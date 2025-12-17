// Package ui provides terminal user interface components for Relicta.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ApprovalResult represents the result of an approval interaction.
type ApprovalResult int

const (
	// ApprovalPending means no decision has been made yet.
	ApprovalPending ApprovalResult = iota
	// ApprovalAccepted means the user approved the release.
	ApprovalAccepted
	// ApprovalRejected means the user rejected the release.
	ApprovalRejected
	// ApprovalEdit means the user wants to edit the release notes.
	ApprovalEdit
)

// ReleaseSummary contains the data to display in the approval UI.
type ReleaseSummary struct {
	ReleaseID      string
	CurrentVersion string
	NextVersion    string
	ReleaseType    string
	CommitCount    int
	Branch         string
	BreakingCount  int
	FeatureCount   int
	FixCount       int
	PerfCount      int
	OtherCount     int
	ReleaseNotes   string
	Plugins        []string
	// Governance fields (optional, nil if governance disabled)
	Governance *GovernanceSummary
}

// GovernanceSummary contains governance evaluation results for the TUI.
type GovernanceSummary struct {
	RiskScore       float64
	Severity        string
	Decision        string
	CanAutoApprove  bool
	RiskFactors     []RiskFactor
	RequiredActions []string
}

// RiskFactor represents a single risk factor in the governance evaluation.
type RiskFactor struct {
	Category    string
	Description string
	Score       float64
}

// ApprovalModel is the Bubble Tea model for the approval TUI.
type ApprovalModel struct {
	summary   ReleaseSummary
	viewport  viewport.Model
	result    ApprovalResult
	ready     bool
	width     int
	height    int
	focused   bool
	showHelp  bool
	showNotes bool
	keymap    approvalKeyMap
	styles    approvalStyles
}

type approvalKeyMap struct {
	Approve     key.Binding
	Reject      key.Binding
	Edit        key.Binding
	ToggleNotes key.Binding
	Help        key.Binding
	Quit        key.Binding
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
}

type approvalStyles struct {
	title     lipgloss.Style
	subtitle  lipgloss.Style
	success   lipgloss.Style
	error     lipgloss.Style
	warning   lipgloss.Style
	info      lipgloss.Style
	subtle    lipgloss.Style
	bold      lipgloss.Style
	border    lipgloss.Style
	focused   lipgloss.Style
	help      lipgloss.Style
	statusBar lipgloss.Style
	stat      lipgloss.Style
	statValue lipgloss.Style
}

func defaultKeyMap() approvalKeyMap {
	return approvalKeyMap{
		Approve: key.NewBinding(
			key.WithKeys("y", "enter"),
			key.WithHelp("y/enter", "approve"),
		),
		Reject: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "reject"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit notes"),
		),
		ToggleNotes: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle notes"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
	}
}

func defaultStyles() approvalStyles {
	return approvalStyles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Padding(0, 1),
		subtitle: lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true),
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		warning:  lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		info:     lipgloss.NewStyle().Foreground(lipgloss.Color("33")),
		subtle:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		bold:     lipgloss.NewStyle().Bold(true),
		border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(1, 2),
		focused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Padding(1, 2),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1),
		statusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1),
		stat: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(18),
		statValue: lipgloss.NewStyle().
			Bold(true),
	}
}

// NewApprovalModel creates a new approval model.
func NewApprovalModel(summary ReleaseSummary) ApprovalModel {
	return ApprovalModel{
		summary:   summary,
		result:    ApprovalPending,
		showNotes: false,
		keymap:    defaultKeyMap(),
		styles:    defaultStyles(),
	}
}

// Init implements tea.Model.
func (m ApprovalModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ApprovalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Initialize viewport for release notes
		headerHeight := 15 // Space for summary
		footerHeight := 4  // Space for help
		viewportHeight := m.height - headerHeight - footerHeight

		if !m.ready {
			m.viewport = viewport.New(m.width-4, viewportHeight)
			m.viewport.SetContent(m.renderNotesContent())
			m.ready = true
		} else {
			m.viewport.Width = m.width - 4
			m.viewport.Height = viewportHeight
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Approve):
			m.result = ApprovalAccepted
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Reject):
			m.result = ApprovalRejected
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Edit):
			m.result = ApprovalEdit
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Quit):
			m.result = ApprovalRejected
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keymap.ToggleNotes):
			m.showNotes = !m.showNotes
			m.focused = m.showNotes
			return m, nil

		case key.Matches(msg, m.keymap.Up):
			if m.showNotes {
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}

		case key.Matches(msg, m.keymap.Down):
			if m.showNotes {
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}

		case key.Matches(msg, m.keymap.PageUp):
			if m.showNotes {
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}

		case key.Matches(msg, m.keymap.PageDown):
			if m.showNotes {
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}
	}

	if m.showNotes {
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return m, cmd
}

// View implements tea.Model.
func (m ApprovalModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	title := m.styles.title.Render("Release Approval")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Release Summary
	b.WriteString(m.renderSummary())
	b.WriteString("\n")

	// Changes Overview
	b.WriteString(m.renderChangesOverview())
	b.WriteString("\n")

	// Governance (if enabled)
	if m.summary.Governance != nil {
		b.WriteString(m.renderGovernance())
		b.WriteString("\n")
	}

	// Plugins
	if len(m.summary.Plugins) > 0 {
		b.WriteString(m.renderPlugins())
		b.WriteString("\n")
	}

	// Release Notes (collapsible)
	if m.showNotes {
		b.WriteString(m.renderNotesSection())
		b.WriteString("\n")
	} else {
		b.WriteString(m.styles.subtle.Render("Press [tab] to view release notes"))
		b.WriteString("\n\n")
	}

	// Help or action prompt
	if m.showHelp {
		b.WriteString(m.renderHelp())
	} else {
		b.WriteString(m.renderPrompt())
	}

	return b.String()
}

func (m ApprovalModel) renderSummary() string {
	var b strings.Builder

	b.WriteString(m.styles.bold.Render("Release Summary"))
	b.WriteString("\n")

	// Version info
	row1 := fmt.Sprintf("%s %s  →  %s",
		m.styles.stat.Render("Version:"),
		m.styles.subtle.Render(m.summary.CurrentVersion),
		m.styles.statValue.Render(m.summary.NextVersion))
	b.WriteString(row1)
	b.WriteString("\n")

	// Release type with color
	typeStyle := m.styles.info
	switch strings.ToLower(m.summary.ReleaseType) {
	case "major":
		typeStyle = m.styles.error
	case "minor":
		typeStyle = m.styles.success
	case "patch":
		typeStyle = m.styles.info
	}
	row2 := fmt.Sprintf("%s %s",
		m.styles.stat.Render("Release Type:"),
		typeStyle.Render(m.summary.ReleaseType))
	b.WriteString(row2)
	b.WriteString("\n")

	// Commits and branch
	row3 := fmt.Sprintf("%s %s    %s %s",
		m.styles.stat.Render("Commits:"),
		m.styles.statValue.Render(fmt.Sprintf("%d", m.summary.CommitCount)),
		m.styles.stat.Render("Branch:"),
		m.styles.subtle.Render(m.summary.Branch))
	b.WriteString(row3)
	b.WriteString("\n")

	return b.String()
}

func (m ApprovalModel) renderChangesOverview() string {
	var b strings.Builder

	b.WriteString(m.styles.bold.Render("Changes"))
	b.WriteString("\n")

	// Breaking changes (highlight if present)
	if m.summary.BreakingCount > 0 {
		b.WriteString(m.styles.error.Render(fmt.Sprintf("  ! Breaking: %d", m.summary.BreakingCount)))
		b.WriteString("\n")
	}

	// Features
	if m.summary.FeatureCount > 0 {
		b.WriteString(m.styles.success.Render(fmt.Sprintf("  + Features: %d", m.summary.FeatureCount)))
		b.WriteString("\n")
	}

	// Fixes
	if m.summary.FixCount > 0 {
		b.WriteString(m.styles.info.Render(fmt.Sprintf("  * Fixes: %d", m.summary.FixCount)))
		b.WriteString("\n")
	}

	// Performance
	if m.summary.PerfCount > 0 {
		b.WriteString(m.styles.warning.Render(fmt.Sprintf("  ~ Perf: %d", m.summary.PerfCount)))
		b.WriteString("\n")
	}

	// Other
	if m.summary.OtherCount > 0 {
		b.WriteString(m.styles.subtle.Render(fmt.Sprintf("    Other: %d", m.summary.OtherCount)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m ApprovalModel) renderGovernance() string {
	var b strings.Builder
	gov := m.summary.Governance

	b.WriteString(m.styles.bold.Render("Governance"))
	b.WriteString("\n")

	// Risk score with severity-based coloring
	riskPercent := fmt.Sprintf("%.1f%%", gov.RiskScore*100)
	var riskStyle lipgloss.Style
	switch gov.Severity {
	case "critical", "high":
		riskStyle = m.styles.error
	case "medium":
		riskStyle = m.styles.warning
	default:
		riskStyle = m.styles.success
	}
	b.WriteString(fmt.Sprintf("  %s %s",
		m.styles.stat.Render("Risk:"),
		riskStyle.Render(fmt.Sprintf("%s (%s)", riskPercent, gov.Severity))))
	b.WriteString("\n")

	// Decision
	var decisionStyle lipgloss.Style
	decisionText := gov.Decision
	switch gov.Decision {
	case "approved":
		decisionStyle = m.styles.success
	case "approval_required":
		decisionStyle = m.styles.warning
		decisionText = "requires approval"
	case "rejected":
		decisionStyle = m.styles.error
	default:
		decisionStyle = m.styles.subtle
	}
	b.WriteString(fmt.Sprintf("  %s %s",
		m.styles.stat.Render("Decision:"),
		decisionStyle.Render(decisionText)))
	b.WriteString("\n")

	// Auto-approve status
	autoApproveText := "no"
	autoApproveStyle := m.styles.warning
	if gov.CanAutoApprove {
		autoApproveText = "yes"
		autoApproveStyle = m.styles.success
	}
	b.WriteString(fmt.Sprintf("  %s %s",
		m.styles.stat.Render("Auto-Approve:"),
		autoApproveStyle.Render(autoApproveText)))
	b.WriteString("\n")

	// Risk factors (if any)
	if len(gov.RiskFactors) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.subtle.Render("  Risk Factors:"))
		b.WriteString("\n")
		for _, factor := range gov.RiskFactors {
			factorText := fmt.Sprintf("    [%s] %s (%.0f%%)",
				factor.Category, factor.Description, factor.Score*100)
			b.WriteString(m.styles.subtle.Render(factorText))
			b.WriteString("\n")
		}
	}

	// Required actions (if any)
	if len(gov.RequiredActions) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.warning.Render("  Required Actions:"))
		b.WriteString("\n")
		for _, action := range gov.RequiredActions {
			b.WriteString(m.styles.warning.Render(fmt.Sprintf("    ! %s", action)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m ApprovalModel) renderPlugins() string {
	var b strings.Builder

	b.WriteString(m.styles.bold.Render("Plugins to Execute"))
	b.WriteString("\n")

	for _, plugin := range m.summary.Plugins {
		b.WriteString(m.styles.subtle.Render(fmt.Sprintf("  - %s", plugin)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m ApprovalModel) renderNotesSection() string {
	var b strings.Builder

	header := m.styles.bold.Render("Release Notes")
	if m.focused {
		header = m.styles.success.Render("Release Notes (scroll with j/k)")
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Use viewport for scrollable content
	viewportStyle := m.styles.border
	if m.focused {
		viewportStyle = m.styles.focused
	}
	b.WriteString(viewportStyle.Render(m.viewport.View()))
	b.WriteString("\n")

	// Scroll percentage
	scrollInfo := fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100)
	b.WriteString(m.styles.subtle.Render(scrollInfo))
	b.WriteString("\n")

	return b.String()
}

func (m ApprovalModel) renderNotesContent() string {
	if m.summary.ReleaseNotes == "" {
		return m.styles.subtle.Render("No release notes available")
	}
	return m.summary.ReleaseNotes
}

func (m ApprovalModel) renderPrompt() string {
	var b strings.Builder

	b.WriteString(m.styles.statusBar.Render("Do you approve this release?"))
	b.WriteString("\n\n")

	// Action hints
	approve := m.styles.success.Render("[y]es")
	reject := m.styles.error.Render("[n]o")
	edit := m.styles.info.Render("[e]dit")
	help := m.styles.subtle.Render("[?]help")

	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s", approve, reject, edit, help))
	b.WriteString("\n")

	return b.String()
}

func (m ApprovalModel) renderHelp() string {
	var b strings.Builder

	b.WriteString(m.styles.bold.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"y / Enter", "Approve the release"},
		{"n / Esc", "Reject the release"},
		{"e", "Edit release notes"},
		{"Tab", "Toggle release notes view"},
		{"j / k", "Scroll notes up/down"},
		{"?", "Toggle help"},
		{"q / Ctrl+C", "Quit"},
	}

	for _, s := range shortcuts {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			m.styles.success.Render(fmt.Sprintf("%-12s", s.key)),
			m.styles.subtle.Render(s.desc)))
	}

	b.WriteString("\n")
	b.WriteString(m.styles.subtle.Render("Press ? to close help"))
	b.WriteString("\n")

	return b.String()
}

// Result returns the approval result.
func (m ApprovalModel) Result() ApprovalResult {
	return m.result
}

// RunApprovalTUI runs the approval TUI and returns the result.
func RunApprovalTUI(summary ReleaseSummary) (ApprovalResult, error) {
	model := NewApprovalModel(summary)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return ApprovalRejected, fmt.Errorf("TUI error: %w", err)
	}

	// Safe type assertion with error handling
	approvalModel, ok := finalModel.(ApprovalModel)
	if !ok {
		return ApprovalRejected, fmt.Errorf("unexpected model type returned from TUI")
	}

	return approvalModel.Result(), nil
}
