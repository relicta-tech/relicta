// Package wizard provides terminal user interface components for the Relicta init wizard.
package wizard

import (
	"github.com/charmbracelet/lipgloss"
)

// wizardStyles defines the shared styles for the wizard UI.
type wizardStyles struct {
	title       lipgloss.Style
	subtitle    lipgloss.Style
	success     lipgloss.Style
	error       lipgloss.Style
	warning     lipgloss.Style
	info        lipgloss.Style
	subtle      lipgloss.Style
	bold        lipgloss.Style
	italic      lipgloss.Style
	border      lipgloss.Style
	focused     lipgloss.Style
	unfocused   lipgloss.Style
	help        lipgloss.Style
	statusBar   lipgloss.Style
	progressBar lipgloss.Style
	label       lipgloss.Style
	value       lipgloss.Style
	placeholder lipgloss.Style
	selected    lipgloss.Style
	unselected  lipgloss.Style
}

// defaultWizardStyles returns the default wizard styles matching the approval UI.
func defaultWizardStyles() wizardStyles {
	return wizardStyles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Padding(0, 1).
			MarginBottom(1),
		subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			MarginBottom(1),
		success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),
		error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")),
		warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),
		info: lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")),
		subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		bold: lipgloss.NewStyle().
			Bold(true),
		italic: lipgloss.NewStyle().
			Italic(true),
		border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(1, 2),
		focused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Padding(1, 2),
		unfocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(1, 2),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1),
		statusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1),
		progressBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")),
		label: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(20),
		value: lipgloss.NewStyle().
			Bold(true),
		placeholder: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true),
		selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true).
			PaddingLeft(2),
		unselected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(2),
	}
}

// WizardState represents the current state of the wizard.
type WizardState int

const (
	// StateWelcome is the welcome screen.
	StateWelcome WizardState = iota
	// StateDetecting is the project detection screen.
	StateDetecting
	// StateProjectType is the project type selection screen.
	StateProjectType
	// StateTemplate is the template selection screen.
	StateTemplate
	// StateQuestions is the question input screen.
	StateQuestions
	// StateAIConfig is the AI configuration screen.
	StateAIConfig
	// StateReview is the review and preview screen.
	StateReview
	// StateSuccess is the success screen.
	StateSuccess
	// StateError indicates an error occurred.
	StateError
	// StateQuit indicates the user quit the wizard.
	StateQuit
)

// WizardResult represents the result of the wizard.
type WizardResult struct {
	// State is the final state of the wizard.
	State WizardState
	// Config is the generated configuration YAML.
	Config string
	// Error is any error that occurred.
	Error error
}

// wizardBanner returns the ASCII art banner for the wizard.
func wizardBanner() string {
	return `
 ____      _                    ____  _ _       _
|  _ \ ___| | ___  __ _ ___  _|  _ \(_) | ___ | |_
| |_) / _ \ |/ _ \/ _' / __|/ /| |_) | | |/ _ \| __|
|  _ <  __/ |  __/ (_| \__ \  \|  __/| | | (_) | |_
|_| \_\___|_|\___|\__,_|___/\_\|_|   |_|_|\___/ \__|

`
}

// wizardWelcome returns the welcome message.
func wizardWelcome() string {
	return "Welcome to the Relicta initialization wizard!\n\n" +
		"This wizard will help you set up Relicta in less than 2 minutes.\n" +
		"We'll auto-detect your project type and generate a customized configuration."
}

// renderStepIndicator renders a step indicator showing which step the user is on.
func renderStepIndicator(current WizardState, styles wizardStyles) string {
	steps := []struct {
		state WizardState
		name  string
	}{
		{StateWelcome, "Welcome"},
		{StateDetecting, "Detecting"},
		{StateProjectType, "Project Type"},
		{StateTemplate, "Template"},
		{StateQuestions, "Configuration"},
		{StateAIConfig, "AI Setup"},
		{StateReview, "Review"},
		{StateSuccess, "Complete"},
	}

	result := ""
	for i, step := range steps {
		if i > 0 {
			result += styles.subtle.Render(" → ")
		}

		if step.state == current {
			result += styles.success.Render("●") + " " + styles.bold.Render(step.name)
		} else if step.state < current {
			result += styles.success.Render("✓") + " " + styles.subtle.Render(step.name)
		} else {
			result += styles.subtle.Render("○") + " " + styles.subtle.Render(step.name)
		}
	}

	return result
}

// renderHelp renders a help section with keyboard shortcuts.
func renderHelp(shortcuts []string, styles wizardStyles) string {
	result := styles.subtle.Render("Shortcuts: ")
	for i, shortcut := range shortcuts {
		if i > 0 {
			result += styles.subtle.Render(" • ")
		}
		result += styles.info.Render(shortcut)
	}
	return result
}
