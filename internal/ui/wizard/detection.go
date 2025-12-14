// Package wizard provides terminal user interface components for the ReleasePilot init wizard.
package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/felixgeelhaar/release-pilot/internal/cli/templates"
)

// DetectionModel is the Bubble Tea model for the detection screen.
type DetectionModel struct {
	styles    wizardStyles
	keymap    detectionKeyMap
	width     int
	height    int
	ready     bool
	detecting bool
	complete  bool
	next      bool
	spinner   spinner.Model
	detector  *templates.Detector
	detection *templates.Detection
	err       error
}

type detectionKeyMap struct {
	Continue key.Binding
	Skip     key.Binding
	Quit     key.Binding
}

func defaultDetectionKeyMap() detectionKeyMap {
	return detectionKeyMap{
		Continue: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "continue"),
		),
		Skip: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "skip detection"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c", "esc"),
			key.WithHelp("q/esc", "quit"),
		),
	}
}

// detectMsg is sent when detection starts.
type detectMsg struct{}

// detectionCompleteMsg is sent when detection completes.
type detectionCompleteMsg struct {
	detection *templates.Detection
	err       error
}

// NewDetectionModel creates a new detection screen model.
func NewDetectionModel(basePath string) DetectionModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = defaultWizardStyles().info

	return DetectionModel{
		styles:    defaultWizardStyles(),
		keymap:    defaultDetectionKeyMap(),
		ready:     false,
		detecting: false,
		complete:  false,
		next:      false,
		spinner:   s,
		detector:  templates.NewDetector(basePath),
	}
}

// Init implements tea.Model.
func (m DetectionModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return detectMsg{} },
	)
}

// Update implements tea.Model.
func (m DetectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case detectMsg:
		m.detecting = true
		return m, m.runDetection()

	case detectionCompleteMsg:
		m.detecting = false
		m.complete = true
		m.detection = msg.detection
		m.err = msg.err

	case tea.KeyMsg:
		if m.complete {
			switch {
			case key.Matches(msg, m.keymap.Continue):
				m.next = true
				return m, nil

			case key.Matches(msg, m.keymap.Quit):
				return m, tea.Quit
			}
		} else if !m.detecting {
			switch {
			case key.Matches(msg, m.keymap.Skip):
				// Skip detection and use defaults
				m.complete = true
				m.detection = &templates.Detection{
					Language:    templates.LanguageUnknown,
					Platform:    templates.PlatformNative,
					ProjectType: templates.ProjectTypeUnknown,
				}
				return m, nil

			case key.Matches(msg, m.keymap.Quit):
				return m, tea.Quit
			}
		}

	case spinner.TickMsg:
		if m.detecting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m DetectionModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	b.WriteString(m.styles.title.Render("Project Detection"))
	b.WriteString("\n")

	// Subtitle
	b.WriteString(m.styles.subtitle.Render("Analyzing your project to suggest the best configuration"))
	b.WriteString("\n\n")

	if m.detecting {
		// Show spinner while detecting
		b.WriteString(m.spinner.View())
		b.WriteString(" Scanning project files...")
		b.WriteString("\n\n")
	} else if m.complete {
		if m.err != nil {
			// Show error
			b.WriteString(m.styles.error.Render("⚠ Detection failed: " + m.err.Error()))
			b.WriteString("\n\n")
			b.WriteString(m.styles.subtle.Render("Continuing with manual configuration..."))
		} else {
			// Show detection results
			b.WriteString(m.renderDetectionResults())
		}
		b.WriteString("\n\n")
	}

	// Progress indicator
	b.WriteString(renderStepIndicator(StateDetecting, m.styles))
	b.WriteString("\n\n")

	// Help
	if m.complete {
		shortcuts := []string{"enter/space: continue", "q/esc: quit"}
		b.WriteString(renderHelp(shortcuts, m.styles))
	} else if !m.detecting {
		shortcuts := []string{"s: skip detection", "q/esc: quit"}
		b.WriteString(renderHelp(shortcuts, m.styles))
	}
	b.WriteString("\n\n")

	// Status bar
	if m.complete {
		b.WriteString(m.styles.statusBar.Render("Press Enter to continue"))
	} else if m.detecting {
		b.WriteString(m.styles.statusBar.Render("Detecting..."))
	}
	b.WriteString("\n")

	return b.String()
}

// renderDetectionResults renders the detection results.
func (m DetectionModel) renderDetectionResults() string {
	var b strings.Builder

	b.WriteString(m.styles.success.Render("✓ Detection complete!"))
	b.WriteString("\n\n")

	// Language
	b.WriteString(m.styles.bold.Render("Detected Configuration:"))
	b.WriteString("\n\n")

	b.WriteString(m.styles.label.Render("Language:"))
	b.WriteString(m.styles.value.Render(string(m.detection.Language)))
	if m.detection.LanguageConfidence > 0 {
		b.WriteString(m.styles.subtle.Render(fmt.Sprintf(" (%d%% confident)", m.detection.LanguageConfidence)))
	}
	b.WriteString("\n")

	// Secondary languages
	if len(m.detection.SecondaryLanguages) > 0 {
		langs := make([]string, len(m.detection.SecondaryLanguages))
		for i, lang := range m.detection.SecondaryLanguages {
			langs[i] = string(lang)
		}
		b.WriteString(m.styles.label.Render("Also detected:"))
		b.WriteString(m.styles.subtle.Render(strings.Join(langs, ", ")))
		b.WriteString("\n")
	}

	// Platform
	if m.detection.Platform != templates.PlatformNative {
		b.WriteString(m.styles.label.Render("Platform:"))
		b.WriteString(m.styles.value.Render(string(m.detection.Platform)))
		if m.detection.PlatformConfidence > 0 {
			b.WriteString(m.styles.subtle.Render(fmt.Sprintf(" (%d%% confident)", m.detection.PlatformConfidence)))
		}
		b.WriteString("\n")
	}

	// Project type
	b.WriteString(m.styles.label.Render("Project Type:"))
	b.WriteString(m.styles.value.Render(string(m.detection.ProjectType)))
	if m.detection.TypeConfidence > 0 {
		b.WriteString(m.styles.subtle.Render(fmt.Sprintf(" (%d%% confident)", m.detection.TypeConfidence)))
	}
	b.WriteString("\n")

	// Git info
	if m.detection.GitRepository != "" {
		b.WriteString(m.styles.label.Render("Repository:"))
		b.WriteString(m.styles.value.Render(m.detection.GitRepository))
		b.WriteString("\n")
	}

	if m.detection.GitBranch != "" {
		b.WriteString(m.styles.label.Render("Branch:"))
		b.WriteString(m.styles.value.Render(m.detection.GitBranch))
		b.WriteString("\n")
	}

	// Package manager
	if m.detection.PackageManager != "" {
		b.WriteString(m.styles.label.Render("Package Manager:"))
		b.WriteString(m.styles.value.Render(m.detection.PackageManager))
		b.WriteString("\n")
	}

	// CI/CD
	if m.detection.HasCI {
		b.WriteString(m.styles.label.Render("CI/CD:"))
		b.WriteString(m.styles.value.Render(m.detection.CIProvider))
		b.WriteString("\n")
	}

	// Features
	var features []string
	if m.detection.HasDockerfile {
		features = append(features, "Docker")
	}
	if m.detection.HasKubernetesConfig {
		features = append(features, "Kubernetes")
	}
	if m.detection.IsMonorepo {
		features = append(features, "Monorepo")
	}

	if len(features) > 0 {
		b.WriteString(m.styles.label.Render("Features:"))
		b.WriteString(m.styles.value.Render(strings.Join(features, ", ")))
		b.WriteString("\n")
	}

	// Suggested template
	if m.detection.SuggestedTemplate != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.label.Render("Suggested Template:"))
		b.WriteString(m.styles.info.Render(m.detection.SuggestedTemplate))
		b.WriteString("\n")
	}

	return b.String()
}

// runDetection executes the project detection.
func (m DetectionModel) runDetection() tea.Cmd {
	return func() tea.Msg {
		detection, err := m.detector.Detect()
		return detectionCompleteMsg{
			detection: detection,
			err:       err,
		}
	}
}

// ShouldContinue returns true if the user wants to continue.
func (m DetectionModel) ShouldContinue() bool {
	return m.next
}

// Detection returns the detection results.
func (m DetectionModel) Detection() *templates.Detection {
	return m.detection
}
