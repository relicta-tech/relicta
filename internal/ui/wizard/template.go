// Package wizard provides terminal user interface components for the ReleasePilot init wizard.
package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/felixgeelhaar/release-pilot/internal/cli/templates"
)

// templateItem implements list.Item for template selection.
type templateItem struct {
	template *templates.Template
	styles   wizardStyles
}

func (i templateItem) FilterValue() string {
	return i.template.DisplayName
}

func (i templateItem) Title() string {
	return i.template.DisplayName
}

func (i templateItem) Description() string {
	return i.template.Description
}

// TemplateModel is the Bubble Tea model for the template selection screen.
type TemplateModel struct {
	styles    wizardStyles
	keymap    templateKeyMap
	width     int
	height    int
	ready     bool
	next      bool
	list      list.Model
	registry  *templates.Registry
	detection *templates.Detection
	selected  *templates.Template
}

type templateKeyMap struct {
	Select key.Binding
	Back   key.Binding
	Quit   key.Binding
}

func defaultTemplateKeyMap() templateKeyMap {
	return templateKeyMap{
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// NewTemplateModel creates a new template selection screen model.
func NewTemplateModel(registry *templates.Registry, detection *templates.Detection) TemplateModel {
	styles := defaultWizardStyles()

	// Get templates filtered by detection results
	var items []list.Item
	var templateList []*templates.Template

	if detection != nil && detection.Language != templates.LanguageUnknown {
		// Filter by language if detected
		templateList = registry.List(templates.FilterByLanguage(detection.Language))
	} else {
		// Show all templates
		templateList = registry.All()
	}

	// Convert to list items
	for _, tmpl := range templateList {
		items = append(items, templateItem{
			template: tmpl,
			styles:   styles,
		})
	}

	// Create list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = styles.selected
	delegate.Styles.SelectedDesc = styles.subtle
	delegate.Styles.NormalTitle = styles.unselected
	delegate.Styles.NormalDesc = styles.subtle

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select a Configuration Template"
	l.Styles.Title = styles.title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	// Pre-select suggested template if available
	if detection != nil && detection.SuggestedTemplate != "" {
		for i, item := range items {
			if ti, ok := item.(templateItem); ok {
				if ti.template.Name == detection.SuggestedTemplate {
					l.Select(i)
					break
				}
			}
		}
	}

	return TemplateModel{
		styles:    styles,
		keymap:    defaultTemplateKeyMap(),
		ready:     false,
		next:      false,
		list:      l,
		registry:  registry,
		detection: detection,
	}
}

// Init implements tea.Model.
func (m TemplateModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m TemplateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update list size
		h := m.height - 10 // Leave space for header and footer
		m.list.SetSize(m.width-4, h)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Select):
			// Get selected template
			if item, ok := m.list.SelectedItem().(templateItem); ok {
				m.selected = item.template
				m.next = true
				return m, nil
			}

		case key.Matches(msg, m.keymap.Back):
			// Go back to previous screen
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit
		}
	}

	// Update list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m TemplateModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Subtitle
	subtitle := "Choose a template that best matches your project"
	if m.detection != nil && m.detection.SuggestedTemplate != "" {
		subtitle = fmt.Sprintf("Based on detection, we recommend: %s", m.styles.info.Render(m.detection.SuggestedTemplate))
	}
	b.WriteString(m.styles.subtitle.Render(subtitle))
	b.WriteString("\n\n")

	// List
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Progress indicator
	b.WriteString(renderStepIndicator(StateTemplate, m.styles))
	b.WriteString("\n\n")

	// Help
	shortcuts := []string{"↑↓: navigate", "enter/space: select", "esc: back", "q: quit"}
	b.WriteString(renderHelp(shortcuts, m.styles))
	b.WriteString("\n")

	return b.String()
}

// ShouldContinue returns true if the user wants to continue.
func (m TemplateModel) ShouldContinue() bool {
	return m.next
}

// Selected returns the selected template.
func (m TemplateModel) Selected() *templates.Template {
	return m.selected
}
