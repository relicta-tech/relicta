// Package wizard provides terminal user interface components for the Relicta init wizard.
package wizard

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/relicta-tech/relicta/internal/cli/templates"
)

// Wizard orchestrates the multi-step wizard flow.
type Wizard struct {
	state      WizardState
	basePath   string
	configPath string

	// Shared data across screens
	registry  *templates.Registry
	detection *templates.Detection
	template  *templates.Template
	builder   *templates.Builder
	config    string

	// Screen models
	welcomeModel   WelcomeModel
	detectionModel DetectionModel
	templateModel  TemplateModel
	reviewModel    ReviewModel
	successModel   SuccessModel

	// Result
	result WizardResult
}

// NewWizard creates a new wizard instance.
func NewWizard(basePath string) (*Wizard, error) {
	if basePath == "" {
		basePath = "."
	}

	// Initialize template registry
	registry, err := templates.NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template registry: %w", err)
	}

	return &Wizard{
		state:      StateWelcome,
		basePath:   basePath,
		configPath: filepath.Join(basePath, ".relicta.yaml"),
		registry:   registry,
		result: WizardResult{
			State: StateWelcome,
		},
	}, nil
}

var newWizard = NewWizard

type programRunner interface {
	Run() (tea.Model, error)
}

var newProgram = func(model tea.Model) programRunner {
	return tea.NewProgram(model)
}

var runWizard = func(w *Wizard) (WizardResult, error) {
	return w.Run()
}

// Run executes the wizard flow and returns the result.
func (w *Wizard) Run() (WizardResult, error) {
	for {
		switch w.state {
		case StateWelcome:
			if err := w.runWelcome(); err != nil {
				return w.handleError(err)
			}

		case StateDetecting:
			if err := w.runDetection(); err != nil {
				return w.handleError(err)
			}

		case StateTemplate:
			if err := w.runTemplateSelection(); err != nil {
				return w.handleError(err)
			}

		case StateReview:
			if err := w.runReview(); err != nil {
				return w.handleError(err)
			}

		case StateSuccess:
			if err := w.runSuccess(); err != nil {
				return w.handleError(err)
			}
			// Success is the final state
			w.result.State = StateSuccess
			w.result.Config = w.config
			return w.result, nil

		case StateQuit:
			w.result.State = StateQuit
			return w.result, nil

		case StateError:
			w.result.State = StateError
			return w.result, w.result.Error

		default:
			return w.handleError(fmt.Errorf("unknown wizard state: %v", w.state))
		}
	}
}

// runWelcome runs the welcome screen.
func (w *Wizard) runWelcome() error {
	w.welcomeModel = NewWelcomeModel()
	p := newProgram(w.welcomeModel)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("welcome screen error: %w", err)
	}

	model, ok := finalModel.(WelcomeModel)
	if !ok {
		return fmt.Errorf("unexpected model type from welcome screen")
	}

	if model.ShouldContinue() {
		w.state = StateDetecting
	} else {
		w.state = StateQuit
	}

	return nil
}

// runDetection runs the detection screen.
func (w *Wizard) runDetection() error {
	w.detectionModel = NewDetectionModel(w.basePath)
	p := newProgram(w.detectionModel)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("detection screen error: %w", err)
	}

	model, ok := finalModel.(DetectionModel)
	if !ok {
		return fmt.Errorf("unexpected model type from detection screen")
	}

	if model.ShouldContinue() {
		w.detection = model.Detection()
		w.state = StateTemplate
	} else {
		w.state = StateQuit
	}

	return nil
}

// runTemplateSelection runs the template selection screen.
func (w *Wizard) runTemplateSelection() error {
	w.templateModel = NewTemplateModel(w.registry, w.detection)
	p := newProgram(w.templateModel)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("template selection error: %w", err)
	}

	model, ok := finalModel.(TemplateModel)
	if !ok {
		return fmt.Errorf("unexpected model type from template screen")
	}

	if model.ShouldContinue() {
		w.template = model.Selected()
		if err := w.buildConfiguration(); err != nil {
			return err
		}
		w.state = StateReview
	} else {
		w.state = StateQuit
	}

	return nil
}

// runReview runs the review and preview screen.
func (w *Wizard) runReview() error {
	w.reviewModel = NewReviewModel(w.config)
	p := newProgram(w.reviewModel)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("review screen error: %w", err)
	}

	model, ok := finalModel.(ReviewModel)
	if !ok {
		return fmt.Errorf("unexpected model type from review screen")
	}

	if model.ShouldGoBack() {
		w.state = StateTemplate
	} else if model.ShouldContinue() {
		if err := w.saveConfiguration(); err != nil {
			return err
		}
		w.state = StateSuccess
	} else {
		w.state = StateQuit
	}

	return nil
}

// runSuccess runs the success screen.
func (w *Wizard) runSuccess() error {
	w.successModel = NewSuccessModel(w.configPath)
	p := newProgram(w.successModel)

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("success screen error: %w", err)
	}

	return nil
}

// buildConfiguration builds the configuration from the selected template.
func (w *Wizard) buildConfiguration() error {
	w.builder = templates.NewBuilder(w.registry, w.detection)

	// Apply detection results
	w.builder.WithDetection()

	// Set defaults
	w.builder.SetGitSign(false) // Default to not signing

	// AI is disabled by default in the wizard. Users can enable AI by:
	// 1. Setting OPENAI_API_KEY, ANTHROPIC_API_KEY, or GEMINI_API_KEY environment variables
	// 2. Editing the generated config file to set ai.enabled: true
	// This approach keeps API keys out of config files for security.
	w.builder.SetAI(false, "", "")

	// Build configuration
	config, err := w.builder.Build(w.template.Name)
	if err != nil {
		return fmt.Errorf("failed to build configuration: %w", err)
	}

	w.config = config
	return nil
}

// saveConfiguration saves the configuration to disk.
func (w *Wizard) saveConfiguration() error {
	// Check if file already exists and log a warning
	if info, err := os.Stat(w.configPath); err == nil {
		slog.Warn("overwriting existing configuration file",
			"path", w.configPath,
			"previous_size", info.Size(),
			"previous_modified", info.ModTime())
	}

	// Write configuration file with restricted permissions (owner read/write only)
	// Use 0600 to prevent world-readable access to config files that may contain
	// sensitive information like webhook URLs or API key references
	if err := os.WriteFile(w.configPath, []byte(w.config), 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	slog.Info("configuration file saved",
		"path", w.configPath)

	return nil
}

// handleError handles an error and sets the wizard to error state.
func (w *Wizard) handleError(err error) (WizardResult, error) {
	w.state = StateError
	w.result.State = StateError
	w.result.Error = err
	return w.result, err
}

// RunWizard is a convenience function to create and run the wizard.
func RunWizard(basePath string) (WizardResult, error) {
	wizard, err := newWizard(basePath)
	if err != nil {
		return WizardResult{
			State: StateError,
			Error: err,
		}, err
	}

	return runWizard(wizard)
}
