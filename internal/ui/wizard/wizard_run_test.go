package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/relicta-tech/relicta/internal/cli/templates"
)

type stubProgram struct {
	model tea.Model
	err   error
}

func (s stubProgram) Run() (tea.Model, error) {
	return s.model, s.err
}

func TestWizard_RunWelcome(t *testing.T) {
	w, err := NewWizard(t.TempDir())
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}

	origNewProgram := newProgram
	t.Cleanup(func() { newProgram = origNewProgram })
	newProgram = func(model tea.Model) programRunner {
		return stubProgram{model: WelcomeModel{next: true}}
	}

	if err := w.runWelcome(); err != nil {
		t.Fatalf("runWelcome error: %v", err)
	}
	if w.state != StateDetecting {
		t.Fatalf("state = %v, want %v", w.state, StateDetecting)
	}
}

func TestWizard_RunDetection(t *testing.T) {
	w, err := NewWizard(t.TempDir())
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}

	origNewProgram := newProgram
	t.Cleanup(func() { newProgram = origNewProgram })
	newProgram = func(model tea.Model) programRunner {
		return stubProgram{model: DetectionModel{
			next:      true,
			detection: &templates.Detection{Language: templates.LanguageGo},
		}}
	}

	if err := w.runDetection(); err != nil {
		t.Fatalf("runDetection error: %v", err)
	}
	if w.state != StateTemplate {
		t.Fatalf("state = %v, want %v", w.state, StateTemplate)
	}
	if w.detection == nil {
		t.Fatal("expected detection to be set")
	}
}

func TestWizard_RunTemplateSelection(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	w, err := NewWizard(tmpDir)
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}

	detector := templates.NewDetector(tmpDir)
	detection, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	w.detection = detection

	selected, err := w.registry.Get("opensource-go")
	if err != nil {
		t.Fatalf("registry.Get error: %v", err)
	}

	origNewProgram := newProgram
	t.Cleanup(func() { newProgram = origNewProgram })
	newProgram = func(model tea.Model) programRunner {
		return stubProgram{model: TemplateModel{next: true, selected: selected}}
	}

	if err := w.runTemplateSelection(); err != nil {
		t.Fatalf("runTemplateSelection error: %v", err)
	}
	if w.state != StateReview {
		t.Fatalf("state = %v, want %v", w.state, StateReview)
	}
	if w.config == "" {
		t.Fatal("expected config to be built")
	}
}

func TestWizard_RunReview(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewWizard(tmpDir)
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}
	w.config = "versioning:\n  strategy: conventional\n"

	origNewProgram := newProgram
	t.Cleanup(func() { newProgram = origNewProgram })
	newProgram = func(model tea.Model) programRunner {
		return stubProgram{model: ReviewModel{next: true}}
	}

	if err := w.runReview(); err != nil {
		t.Fatalf("runReview error: %v", err)
	}
	if w.state != StateSuccess {
		t.Fatalf("state = %v, want %v", w.state, StateSuccess)
	}
	if _, err := os.Stat(w.configPath); err != nil {
		t.Fatalf("config file not saved: %v", err)
	}
}

func TestWizard_RunSuccess(t *testing.T) {
	w, err := NewWizard(t.TempDir())
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}

	origNewProgram := newProgram
	t.Cleanup(func() { newProgram = origNewProgram })
	newProgram = func(model tea.Model) programRunner {
		return stubProgram{model: SuccessModel{}}
	}

	if err := w.runSuccess(); err != nil {
		t.Fatalf("runSuccess error: %v", err)
	}
}

func TestWizard_Run_States(t *testing.T) {
	w, err := NewWizard(t.TempDir())
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}

	w.state = StateQuit
	result, err := w.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.State != StateQuit {
		t.Fatalf("result.State = %v, want %v", result.State, StateQuit)
	}

	w.state = WizardState(999)
	if _, err := w.Run(); err == nil {
		t.Fatal("expected Run to fail for unknown state")
	}
}

func TestWizard_Run_FullFlow(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	w, err := NewWizard(tmpDir)
	if err != nil {
		t.Fatalf("NewWizard error: %v", err)
	}

	detector := templates.NewDetector(tmpDir)
	detection, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	selected, err := w.registry.Get("opensource-go")
	if err != nil {
		t.Fatalf("registry.Get error: %v", err)
	}

	steps := []tea.Model{
		WelcomeModel{next: true},
		DetectionModel{next: true, detection: detection},
		TemplateModel{next: true, selected: selected},
		ReviewModel{next: true},
		SuccessModel{},
	}

	origNewProgram := newProgram
	t.Cleanup(func() { newProgram = origNewProgram })
	newProgram = func(model tea.Model) programRunner {
		step := steps[0]
		steps = steps[1:]
		return stubProgram{model: step}
	}

	result, err := w.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.State != StateSuccess {
		t.Fatalf("result.State = %v, want %v", result.State, StateSuccess)
	}
	if result.Config == "" {
		t.Fatal("expected config to be set in result")
	}
}

func TestWizard_View_Rendering(t *testing.T) {
	detectModel := NewDetectionModel(t.TempDir())
	detectModel.ready = true
	detectModel.complete = true
	detectModel.detection = &templates.Detection{
		Language:           templates.LanguageGo,
		LanguageConfidence: 90,
		SuggestedTemplate:  "opensource-go",
	}
	detectView := detectModel.View()
	if !strings.Contains(detectView, "Detection complete") {
		t.Fatalf("Detection view missing completion text")
	}

	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry error: %v", err)
	}
	templateModel := NewTemplateModel(registry, &templates.Detection{SuggestedTemplate: "opensource-go"})
	templateModel.ready = true
	templateView := templateModel.View()
	if !strings.Contains(templateView, "Based on detection") && !strings.Contains(templateView, "Choose a template") {
		t.Fatalf("Template view missing content")
	}

	reviewModel := NewReviewModel("versioning:\n  strategy: conventional\n")
	reviewModel.ready = true
	reviewModel.viewport = viewport.New(20, 5)
	reviewModel.viewport.SetContent(reviewModel.config)
	reviewView := reviewModel.View()
	if !strings.Contains(reviewView, "Review Configuration") {
		t.Fatalf("Review view missing title")
	}
}
