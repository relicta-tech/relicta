// Package wizard provides terminal user interface components for the ReleasePilot init wizard.
package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ReviewModel is the Bubble Tea model for the review and preview screen.
type ReviewModel struct {
	styles   wizardStyles
	keymap   reviewKeyMap
	width    int
	height   int
	ready    bool
	next     bool
	back     bool
	viewport viewport.Model
	config   string
}

type reviewKeyMap struct {
	Approve  key.Binding
	Edit     key.Binding
	Back     key.Binding
	Quit     key.Binding
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
}

func defaultReviewKeyMap() reviewKeyMap {
	return reviewKeyMap{
		Approve: key.NewBinding(
			key.WithKeys("enter", "y"),
			key.WithHelp("enter/y", "approve"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/↑", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/↓", "scroll down"),
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

// NewReviewModel creates a new review and preview screen model.
func NewReviewModel(config string) ReviewModel {
	return ReviewModel{
		styles: defaultWizardStyles(),
		keymap: defaultReviewKeyMap(),
		ready:  false,
		next:   false,
		back:   false,
		config: config,
	}
}

// Init implements tea.Model.
func (m ReviewModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Initialize viewport
		headerHeight := 8 // Space for title and instructions
		footerHeight := 6 // Space for help and status
		viewportHeight := m.height - headerHeight - footerHeight

		if !m.ready {
			m.viewport = viewport.New(m.width-4, viewportHeight)
			m.viewport.SetContent(m.config)
			m.ready = true
		} else {
			m.viewport.Width = m.width - 4
			m.viewport.Height = viewportHeight
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Approve):
			m.next = true
			return m, nil

		case key.Matches(msg, m.keymap.Edit):
			// User wants to edit manually - signal to open editor
			m.next = true
			return m, nil

		case key.Matches(msg, m.keymap.Back):
			m.back = true
			return m, nil

		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Up):
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case key.Matches(msg, m.keymap.Down):
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case key.Matches(msg, m.keymap.PageUp):
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case key.Matches(msg, m.keymap.PageDown):
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m ReviewModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	b.WriteString(m.styles.title.Render("Review Configuration"))
	b.WriteString("\n")

	// Subtitle
	b.WriteString(m.styles.subtitle.Render("Review the generated configuration file before saving"))
	b.WriteString("\n\n")

	// Instructions
	b.WriteString(m.styles.info.Render("📝 release.config.yaml"))
	b.WriteString("\n")
	b.WriteString(m.styles.subtle.Render("This configuration will be saved to your project root"))
	b.WriteString("\n\n")

	// Viewport with config
	b.WriteString(m.styles.focused.Render(m.viewport.View()))
	b.WriteString("\n\n")

	// Scroll percentage
	scrollInfo := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		scrollInfo = m.styles.subtle.Render(fmt.Sprintf("  %.0f%%", m.viewport.ScrollPercent()*100))
	}
	b.WriteString(scrollInfo)
	b.WriteString("\n")

	// Progress indicator
	b.WriteString(renderStepIndicator(StateReview, m.styles))
	b.WriteString("\n\n")

	// Help
	shortcuts := []string{"↑↓: scroll", "enter/y: approve", "e: edit manually", "esc: back", "q: quit"}
	b.WriteString(renderHelp(shortcuts, m.styles))
	b.WriteString("\n\n")

	// Status bar
	b.WriteString(m.styles.statusBar.Render("Do you approve this configuration?"))
	b.WriteString("\n")

	return b.String()
}

// ShouldContinue returns true if the user wants to continue.
func (m ReviewModel) ShouldContinue() bool {
	return m.next
}

// ShouldGoBack returns true if the user wants to go back.
func (m ReviewModel) ShouldGoBack() bool {
	return m.back
}
