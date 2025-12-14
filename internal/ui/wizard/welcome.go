// Package wizard provides terminal user interface components for the Relicta init wizard.
package wizard

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// WelcomeModel is the Bubble Tea model for the welcome screen.
type WelcomeModel struct {
	styles wizardStyles
	keymap welcomeKeyMap
	width  int
	height int
	ready  bool
	next   bool
}

type welcomeKeyMap struct {
	Continue key.Binding
	Quit     key.Binding
}

func defaultWelcomeKeyMap() welcomeKeyMap {
	return welcomeKeyMap{
		Continue: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "continue"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c", "esc"),
			key.WithHelp("q/esc", "quit"),
		),
	}
}

// NewWelcomeModel creates a new welcome screen model.
func NewWelcomeModel() WelcomeModel {
	return WelcomeModel{
		styles: defaultWizardStyles(),
		keymap: defaultWelcomeKeyMap(),
		ready:  false,
		next:   false,
	}
}

// Init implements tea.Model.
func (m WelcomeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m WelcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Continue):
			m.next = true
			return m, nil

		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m WelcomeModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Banner
	banner := m.styles.info.Render(wizardBanner())
	b.WriteString(banner)
	b.WriteString("\n")

	// Welcome message
	welcome := wizardWelcome()
	b.WriteString(welcome)
	b.WriteString("\n\n")

	// Features
	b.WriteString(m.styles.bold.Render("What this wizard will do:"))
	b.WriteString("\n\n")

	features := []string{
		"🔍 Auto-detect your project type and language",
		"📋 Suggest the best configuration template",
		"⚙️  Guide you through customization options",
		"🤖 Set up AI-powered release notes (optional)",
		"📝 Generate a complete release.config.yaml",
	}

	for _, feature := range features {
		b.WriteString("  " + feature)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Progress indicator (step 1 of 8)
	b.WriteString(renderStepIndicator(StateWelcome, m.styles))
	b.WriteString("\n\n")

	// Help
	shortcuts := []string{"enter/space: continue", "q/esc: quit"}
	b.WriteString(renderHelp(shortcuts, m.styles))
	b.WriteString("\n\n")

	// Action prompt
	prompt := m.styles.statusBar.Render("Press Enter to start")
	b.WriteString(prompt)
	b.WriteString("\n")

	return b.String()
}

// ShouldContinue returns true if the user wants to continue.
func (m WelcomeModel) ShouldContinue() bool {
	return m.next
}
