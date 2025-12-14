// Package wizard provides terminal user interface components for the Relicta init wizard.
package wizard

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// SuccessModel is the Bubble Tea model for the success screen.
type SuccessModel struct {
	styles     wizardStyles
	keymap     successKeyMap
	width      int
	height     int
	ready      bool
	configPath string
}

type successKeyMap struct {
	Exit key.Binding
}

func defaultSuccessKeyMap() successKeyMap {
	return successKeyMap{
		Exit: key.NewBinding(
			key.WithKeys("enter", " ", "q", "esc", "ctrl+c"),
			key.WithHelp("enter/q", "exit"),
		),
	}
}

// NewSuccessModel creates a new success screen model.
func NewSuccessModel(configPath string) SuccessModel {
	return SuccessModel{
		styles:     defaultWizardStyles(),
		keymap:     defaultSuccessKeyMap(),
		ready:      false,
		configPath: configPath,
	}
}

// Init implements tea.Model.
func (m SuccessModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m SuccessModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		if key.Matches(msg, m.keymap.Exit) {
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m SuccessModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Success banner
	banner := `
   _____ _    _  _____ _____ ______  _____ _____
  / ____| |  | |/ ____/ ____|  ____|/ ____/ ____|
 | (___ | |  | | |   | |    | |__  | (___| (___
  \___ \| |  | | |   | |    |  __|  \___ \\___ \
  ____) | |__| | |___| |____| |____ ____) |___) |
 |_____/ \____/ \_____\_____|______|_____/_____/

`
	b.WriteString(m.styles.success.Render(banner))
	b.WriteString("\n")

	// Success message
	b.WriteString(m.styles.title.Render("✓ Configuration Complete!"))
	b.WriteString("\n\n")

	b.WriteString(m.styles.subtitle.Render("Relicta has been successfully initialized"))
	b.WriteString("\n\n")

	// File saved
	b.WriteString(m.styles.label.Render("Configuration saved to:"))
	b.WriteString("\n")
	b.WriteString("  " + m.styles.info.Render(m.configPath))
	b.WriteString("\n\n")

	// Next steps
	b.WriteString(m.styles.bold.Render("Next Steps:"))
	b.WriteString("\n\n")

	steps := []struct {
		icon string
		text string
	}{
		{"1️⃣", "Review and customize your configuration if needed"},
		{"2️⃣", "Set up any required environment variables (API keys, webhooks)"},
		{"3️⃣", "Test your configuration with: relicta plan --dry-run"},
		{"4️⃣", "Create your first release: relicta publish"},
	}

	for _, step := range steps {
		b.WriteString("  " + step.icon + "  " + step.text)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Documentation link
	b.WriteString(m.styles.subtle.Render("📚 Documentation: https://github.com/relicta-tech/relicta"))
	b.WriteString("\n\n")

	// Environment variables reminder
	b.WriteString(m.styles.warning.Render("⚠ Remember to set required environment variables:"))
	b.WriteString("\n")

	envVars := []string{
		"GITHUB_TOKEN or GITLAB_TOKEN (for version control)",
		"OPENAI_API_KEY or ANTHROPIC_API_KEY (if using AI features)",
		"SLACK_WEBHOOK_URL, DISCORD_WEBHOOK_URL, etc. (for notifications)",
	}

	for _, env := range envVars {
		b.WriteString("  • " + m.styles.subtle.Render(env))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Progress indicator (complete)
	b.WriteString(renderStepIndicator(StateSuccess, m.styles))
	b.WriteString("\n\n")

	// Help
	shortcuts := []string{"enter/q: exit"}
	b.WriteString(renderHelp(shortcuts, m.styles))
	b.WriteString("\n\n")

	// Status bar
	b.WriteString(m.styles.statusBar.Render("Press Enter to exit"))
	b.WriteString("\n")

	return b.String()
}
