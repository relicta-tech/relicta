// Package wizard provides terminal user interface components for the Relicta init wizard.
package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/relicta-tech/relicta/internal/cli/templates"
)

func TestNewWizard(t *testing.T) {
	tests := []struct {
		name         string
		basePath     string
		expectedPath string
		expectError  bool
	}{
		{
			name:         "empty path defaults to current directory",
			basePath:     "",
			expectedPath: ".",
			expectError:  false,
		},
		{
			name:         "specific path is preserved",
			basePath:     "/tmp/test",
			expectedPath: "/tmp/test",
			expectError:  false,
		},
		{
			name:         "relative path is preserved",
			basePath:     "./testdata",
			expectedPath: "./testdata",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wizard, err := NewWizard(tt.basePath)

			if tt.expectError {
				if err == nil {
					t.Fatal("NewWizard() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewWizard() error = %v", err)
			}

			if wizard == nil {
				t.Fatal("NewWizard() returned nil wizard")
			}

			if wizard.basePath != tt.expectedPath {
				t.Errorf("basePath = %v, want %v", wizard.basePath, tt.expectedPath)
			}

			expectedConfigPath := filepath.Join(tt.expectedPath, "release.config.yaml")
			if wizard.configPath != expectedConfigPath {
				t.Errorf("configPath = %v, want %v", wizard.configPath, expectedConfigPath)
			}

			if wizard.state != StateWelcome {
				t.Errorf("initial state = %v, want %v", wizard.state, StateWelcome)
			}

			if wizard.registry == nil {
				t.Error("registry should not be nil")
			}

			if wizard.result.State != StateWelcome {
				t.Errorf("result.State = %v, want %v", wizard.result.State, StateWelcome)
			}
		})
	}
}

func TestWizard_buildConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal project structure
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module github.com/test/project\n"), 0600); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	wizard, err := NewWizard(tmpDir)
	if err != nil {
		t.Fatalf("NewWizard() error = %v", err)
	}

	// Create detection
	detector := templates.NewDetector(tmpDir)
	detection, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	wizard.detection = detection

	// Get a template from registry
	template, err := wizard.registry.Get("opensource-go")
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}
	wizard.template = template

	// Build configuration
	err = wizard.buildConfiguration()
	if err != nil {
		t.Fatalf("buildConfiguration() error = %v", err)
	}

	// Verify configuration was built
	if wizard.config == "" {
		t.Error("config should not be empty after buildConfiguration()")
	}

	if wizard.builder == nil {
		t.Error("builder should not be nil after buildConfiguration()")
	}

	// Verify config contains expected sections
	expectedSections := []string{
		"versioning:",
		"changelog:",
	}
	for _, section := range expectedSections {
		if !strings.Contains(wizard.config, section) {
			t.Errorf("config should contain %q section", section)
		}
	}
}

func TestWizard_saveConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	wizard, err := NewWizard(tmpDir)
	if err != nil {
		t.Fatalf("NewWizard() error = %v", err)
	}

	// Set test configuration
	testConfig := "versioning:\n  strategy: conventional\n"
	wizard.config = testConfig

	// Save configuration
	err = wizard.saveConfiguration()
	if err != nil {
		t.Fatalf("saveConfiguration() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(wizard.configPath); os.IsNotExist(err) {
		t.Errorf("configuration file was not created at %s", wizard.configPath)
	}

	// Verify file contents
	content, err := os.ReadFile(wizard.configPath)
	if err != nil {
		t.Fatalf("Failed to read configuration file: %v", err)
	}

	if string(content) != testConfig {
		t.Errorf("configuration content = %q, want %q", string(content), testConfig)
	}

	// Verify file permissions (should be 0600 - owner read/write only)
	info, err := os.Stat(wizard.configPath)
	if err != nil {
		t.Fatalf("Failed to stat configuration file: %v", err)
	}

	mode := info.Mode()
	expectedMode := os.FileMode(0600)
	if mode.Perm() != expectedMode {
		t.Errorf("file permissions = %v, want %v", mode.Perm(), expectedMode)
	}
}

func TestWizard_saveConfiguration_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()

	wizard, err := NewWizard(tmpDir)
	if err != nil {
		t.Fatalf("NewWizard() error = %v", err)
	}

	// Create existing configuration file
	existingConfig := "old config"
	if err := os.WriteFile(wizard.configPath, []byte(existingConfig), 0600); err != nil {
		t.Fatalf("Failed to create existing config: %v", err)
	}

	// Save new configuration
	newConfig := "new config"
	wizard.config = newConfig
	err = wizard.saveConfiguration()
	if err != nil {
		t.Fatalf("saveConfiguration() error = %v", err)
	}

	// Verify file was overwritten
	content, err := os.ReadFile(wizard.configPath)
	if err != nil {
		t.Fatalf("Failed to read configuration file: %v", err)
	}

	if string(content) != newConfig {
		t.Errorf("configuration content = %q, want %q", string(content), newConfig)
	}
}

func TestWizard_handleError(t *testing.T) {
	wizard, err := NewWizard(".")
	if err != nil {
		t.Fatalf("NewWizard() error = %v", err)
	}

	testErr := os.ErrNotExist
	result, err := wizard.handleError(testErr)

	// Verify error was returned
	if err != testErr {
		t.Errorf("returned error = %v, want %v", err, testErr)
	}

	// Verify wizard state was updated
	if wizard.state != StateError {
		t.Errorf("wizard.state = %v, want %v", wizard.state, StateError)
	}

	// Verify result state
	if result.State != StateError {
		t.Errorf("result.State = %v, want %v", result.State, StateError)
	}

	// Verify result error
	if result.Error != testErr {
		t.Errorf("result.Error = %v, want %v", result.Error, testErr)
	}
}

func TestRunWizard_InvalidPath(t *testing.T) {
	// Test with a path that will cause registry initialization to fail
	// This exercises the error path in RunWizard
	result, err := RunWizard("")

	// Should return an error result
	if result.State != StateError {
		t.Errorf("result.State = %v, want %v", result.State, StateError)
	}

	// Registry initialization should succeed, so this test will only fail
	// if NewWizard itself fails, which it won't with empty path
	if err == nil {
		// This is expected - NewWizard("") succeeds and initializes registry
		t.Skip("NewWizard succeeds with empty path - cannot test error path")
	}
}

// Test model constructors

func TestNewDetectionModel(t *testing.T) {
	model := NewDetectionModel("/tmp/test")

	if model.detector == nil {
		t.Error("detector should not be nil")
	}

	if model.ready {
		t.Error("model should not be ready initially")
	}

	if model.detecting {
		t.Error("model should not be detecting initially")
	}

	if model.complete {
		t.Error("model should not be complete initially")
	}

	if model.next {
		t.Error("next should be false initially")
	}
}

func TestDetectionModel_ShouldContinue(t *testing.T) {
	model := NewDetectionModel("/tmp/test")

	// Initially false
	if model.ShouldContinue() {
		t.Error("ShouldContinue() should be false initially")
	}

	// Set next to true
	model.next = true
	if !model.ShouldContinue() {
		t.Error("ShouldContinue() should be true when next is true")
	}
}

func TestDetectionModel_Detection(t *testing.T) {
	model := NewDetectionModel("/tmp/test")

	// Initially nil
	if model.Detection() != nil {
		t.Error("Detection() should be nil initially")
	}

	// Set detection
	detection := &templates.Detection{
		Language:    templates.LanguageGo,
		ProjectType: templates.ProjectTypeCLI,
	}
	model.detection = detection

	if model.Detection() != detection {
		t.Error("Detection() should return the set detection")
	}
}

func TestNewWelcomeModel(t *testing.T) {
	model := NewWelcomeModel()

	if model.ready {
		t.Error("model should not be ready initially")
	}

	if model.next {
		t.Error("next should be false initially")
	}
}

func TestWelcomeModel_ShouldContinue(t *testing.T) {
	model := NewWelcomeModel()

	// Initially false
	if model.ShouldContinue() {
		t.Error("ShouldContinue() should be false initially")
	}

	// Set next to true
	model.next = true
	if !model.ShouldContinue() {
		t.Error("ShouldContinue() should be true when next is true")
	}
}

func TestNewTemplateModel(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	detection := &templates.Detection{
		Language:          templates.LanguageGo,
		ProjectType:       templates.ProjectTypeCLI,
		SuggestedTemplate: "cli-tool",
	}

	model := NewTemplateModel(registry, detection)

	if model.registry == nil {
		t.Error("registry should not be nil")
	}

	if model.detection == nil {
		t.Error("detection should not be nil")
	}

	if model.ready {
		t.Error("model should not be ready initially")
	}

	if model.next {
		t.Error("next should be false initially")
	}
}

func TestTemplateModel_ShouldContinue(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	model := NewTemplateModel(registry, nil)

	// Initially false
	if model.ShouldContinue() {
		t.Error("ShouldContinue() should be false initially")
	}

	// Set next to true
	model.next = true
	if !model.ShouldContinue() {
		t.Error("ShouldContinue() should be true when next is true")
	}
}

func TestTemplateModel_Selected(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	model := NewTemplateModel(registry, nil)

	// Initially nil
	if model.Selected() != nil {
		t.Error("Selected() should be nil initially")
	}

	// Set selected template
	template, err := registry.Get("base")
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}
	model.selected = template

	if model.Selected() != template {
		t.Error("Selected() should return the selected template")
	}
}

func TestNewReviewModel(t *testing.T) {
	testConfig := "versioning:\n  strategy: conventional\n"
	model := NewReviewModel(testConfig)

	if model.config != testConfig {
		t.Errorf("config = %q, want %q", model.config, testConfig)
	}

	if model.ready {
		t.Error("model should not be ready initially")
	}

	if model.next {
		t.Error("next should be false initially")
	}

	if model.back {
		t.Error("back should be false initially")
	}
}

func TestReviewModel_ShouldContinue(t *testing.T) {
	model := NewReviewModel("test config")

	// Initially false
	if model.ShouldContinue() {
		t.Error("ShouldContinue() should be false initially")
	}

	// Set next to true
	model.next = true
	if !model.ShouldContinue() {
		t.Error("ShouldContinue() should be true when next is true")
	}
}

func TestReviewModel_ShouldGoBack(t *testing.T) {
	model := NewReviewModel("test config")

	// Initially false
	if model.ShouldGoBack() {
		t.Error("ShouldGoBack() should be false initially")
	}

	// Set back to true
	model.back = true
	if !model.ShouldGoBack() {
		t.Error("ShouldGoBack() should be true when back is true")
	}
}

func TestNewSuccessModel(t *testing.T) {
	configPath := "/tmp/test/release.config.yaml"
	model := NewSuccessModel(configPath)

	if model.configPath != configPath {
		t.Errorf("configPath = %q, want %q", model.configPath, configPath)
	}

	if model.ready {
		t.Error("model should not be ready initially")
	}
}

// Test keymap constructors

func TestDefaultDetectionKeyMap(t *testing.T) {
	keymap := defaultDetectionKeyMap()

	if len(keymap.Continue.Keys()) == 0 {
		t.Error("Continue key binding should have keys")
	}

	if len(keymap.Skip.Keys()) == 0 {
		t.Error("Skip key binding should have keys")
	}

	if len(keymap.Quit.Keys()) == 0 {
		t.Error("Quit key binding should have keys")
	}
}

func TestDefaultWelcomeKeyMap(t *testing.T) {
	keymap := defaultWelcomeKeyMap()

	if len(keymap.Continue.Keys()) == 0 {
		t.Error("Continue key binding should have keys")
	}

	if len(keymap.Quit.Keys()) == 0 {
		t.Error("Quit key binding should have keys")
	}
}

func TestDefaultTemplateKeyMap(t *testing.T) {
	keymap := defaultTemplateKeyMap()

	if len(keymap.Select.Keys()) == 0 {
		t.Error("Select key binding should have keys")
	}

	if len(keymap.Back.Keys()) == 0 {
		t.Error("Back key binding should have keys")
	}

	if len(keymap.Quit.Keys()) == 0 {
		t.Error("Quit key binding should have keys")
	}
}

func TestDefaultReviewKeyMap(t *testing.T) {
	keymap := defaultReviewKeyMap()

	if len(keymap.Approve.Keys()) == 0 {
		t.Error("Approve key binding should have keys")
	}

	if len(keymap.Edit.Keys()) == 0 {
		t.Error("Edit key binding should have keys")
	}

	if len(keymap.Back.Keys()) == 0 {
		t.Error("Back key binding should have keys")
	}

	if len(keymap.Quit.Keys()) == 0 {
		t.Error("Quit key binding should have keys")
	}

	if len(keymap.Up.Keys()) == 0 {
		t.Error("Up key binding should have keys")
	}

	if len(keymap.Down.Keys()) == 0 {
		t.Error("Down key binding should have keys")
	}

	if len(keymap.PageUp.Keys()) == 0 {
		t.Error("PageUp key binding should have keys")
	}

	if len(keymap.PageDown.Keys()) == 0 {
		t.Error("PageDown key binding should have keys")
	}
}

func TestDefaultSuccessKeyMap(t *testing.T) {
	keymap := defaultSuccessKeyMap()

	if len(keymap.Exit.Keys()) == 0 {
		t.Error("Exit key binding should have keys")
	}
}

// Test DetectionModel renderDetectionResults

func TestDetectionModel_RenderDetectionResults(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	model.detection = &templates.Detection{
		Language:            templates.LanguageGo,
		LanguageConfidence:  90,
		SecondaryLanguages:  []templates.Language{templates.LanguageNode},
		Platform:            templates.PlatformDocker,
		PlatformConfidence:  85,
		ProjectType:         templates.ProjectTypeCLI,
		TypeConfidence:      95,
		GitRepository:       "https://github.com/user/repo",
		GitBranch:           "main",
		PackageManager:      "go mod",
		HasCI:               true,
		CIProvider:          "GitHub Actions",
		HasDockerfile:       true,
		HasKubernetesConfig: false,
		IsMonorepo:          false,
		SuggestedTemplate:   "cli-tool",
	}

	result := model.renderDetectionResults()

	// Verify result contains expected sections
	expectedPhrases := []string{
		"Detection complete",
		"Detected Configuration",
		"Language:",
		string(templates.LanguageGo),
		"90% confident",
		"Also detected:",
		string(templates.LanguageNode),
		"Platform:",
		string(templates.PlatformDocker),
		"85% confident",
		"Project Type:",
		string(templates.ProjectTypeCLI),
		"95% confident",
		"Repository:",
		"https://github.com/user/repo",
		"Branch:",
		"main",
		"Package Manager:",
		"go mod",
		"CI/CD:",
		"GitHub Actions",
		"Features:",
		"Docker",
		"Suggested Template:",
		"cli-tool",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(result, phrase) {
			t.Errorf("renderDetectionResults() should contain %q", phrase)
		}
	}
}

func TestDetectionModel_RenderDetectionResults_Minimal(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	model.detection = &templates.Detection{
		Language:    templates.LanguageUnknown,
		Platform:    templates.PlatformNative,
		ProjectType: templates.ProjectTypeUnknown,
	}

	result := model.renderDetectionResults()

	// Should still contain basic structure
	if !strings.Contains(result, "Detection complete") {
		t.Error("renderDetectionResults() should contain 'Detection complete'")
	}

	if !strings.Contains(result, "Detected Configuration") {
		t.Error("renderDetectionResults() should contain 'Detected Configuration'")
	}
}

// Test templateItem methods

func TestTemplateItem_FilterValue(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	template, err := registry.Get("base")
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}

	item := templateItem{
		template: template,
		styles:   defaultWizardStyles(),
	}

	if item.FilterValue() != template.DisplayName {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), template.DisplayName)
	}
}

func TestTemplateItem_Title(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	template, err := registry.Get("base")
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}

	item := templateItem{
		template: template,
		styles:   defaultWizardStyles(),
	}

	if item.Title() != template.DisplayName {
		t.Errorf("Title() = %q, want %q", item.Title(), template.DisplayName)
	}
}

func TestTemplateItem_Description(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	template, err := registry.Get("base")
	if err != nil {
		t.Fatalf("registry.Get() error = %v", err)
	}

	item := templateItem{
		template: template,
		styles:   defaultWizardStyles(),
	}

	if item.Description() != template.Description {
		t.Errorf("Description() = %q, want %q", item.Description(), template.Description)
	}
}

// Test View() methods for basic functionality

func TestDetectionModel_View_NotReady(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	// Don't set ready flag

	view := model.View()
	if view != "Initializing..." {
		t.Errorf("View() should return 'Initializing...' when not ready, got %q", view)
	}
}

func TestWelcomeModel_View_NotReady(t *testing.T) {
	model := NewWelcomeModel()
	// Don't set ready flag

	view := model.View()
	if view != "Initializing..." {
		t.Errorf("View() should return 'Initializing...' when not ready, got %q", view)
	}
}

func TestWelcomeModel_View_Ready(t *testing.T) {
	model := NewWelcomeModel()
	model.ready = true
	model.width = 80
	model.height = 24

	view := model.View()

	// Verify contains expected sections
	expectedPhrases := []string{
		"What this wizard will do:",
		"Auto-detect your project type",
		"Press Enter to start",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(view, phrase) {
			t.Errorf("View() should contain %q", phrase)
		}
	}
}

func TestTemplateModel_View_NotReady(t *testing.T) {
	registry, err := templates.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	model := NewTemplateModel(registry, nil)
	// Don't set ready flag

	view := model.View()
	if view != "Initializing..." {
		t.Errorf("View() should return 'Initializing...' when not ready, got %q", view)
	}
}

func TestReviewModel_View_NotReady(t *testing.T) {
	model := NewReviewModel("test config")
	// Don't set ready flag

	view := model.View()
	if view != "Initializing..." {
		t.Errorf("View() should return 'Initializing...' when not ready, got %q", view)
	}
}

func TestSuccessModel_View_NotReady(t *testing.T) {
	model := NewSuccessModel("/tmp/test/release.config.yaml")
	// Don't set ready flag

	view := model.View()
	if view != "Initializing..." {
		t.Errorf("View() should return 'Initializing...' when not ready, got %q", view)
	}
}

func TestSuccessModel_View_Ready(t *testing.T) {
	configPath := "/tmp/test/release.config.yaml"
	model := NewSuccessModel(configPath)
	model.ready = true
	model.width = 80
	model.height = 40

	view := model.View()

	// Verify view is not empty and contains some expected text
	if len(view) == 0 {
		t.Error("View() should not be empty")
	}

	// Check for basic structure (without exact formatting due to styles)
	expectedPhrases := []string{
		"Complete",      // From "Configuration Complete!"
		configPath,      // Config path should always appear
		"Next Steps",    // Section header
		"relicta", // Command references
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(view, phrase) {
			t.Errorf("View() should contain %q, got:\n%s", phrase, view)
		}
	}
}

// Test Update methods for all models

func TestWelcomeModel_Init(t *testing.T) {
	model := NewWelcomeModel()
	cmd := model.Init()

	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestWelcomeModel_Update_WindowSize(t *testing.T) {
	model := NewWelcomeModel()

	if model.ready {
		t.Error("model should not be ready initially")
	}

	// Send WindowSizeMsg
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, cmd := model.Update(msg)

	if cmd != nil {
		t.Errorf("Update(WindowSizeMsg) cmd = %v, want nil", cmd)
	}

	welcome, ok := updatedModel.(WelcomeModel)
	if !ok {
		t.Fatal("Update should return WelcomeModel")
	}

	if !welcome.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if welcome.width != 80 {
		t.Errorf("width = %d, want 80", welcome.width)
	}
	if welcome.height != 24 {
		t.Errorf("height = %d, want 24", welcome.height)
	}
}

func TestWelcomeModel_Update_ContinueKey(t *testing.T) {
	model := NewWelcomeModel()

	// Send enter key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, cmd := model.Update(msg)

	if cmd != nil {
		t.Errorf("Update(KeyEnter) cmd = %v, want nil", cmd)
	}

	welcome, ok := updatedModel.(WelcomeModel)
	if !ok {
		t.Fatal("Update should return WelcomeModel")
	}

	if !welcome.next {
		t.Error("next should be true after pressing enter")
	}
	if !welcome.ShouldContinue() {
		t.Error("ShouldContinue() should return true after pressing enter")
	}
}

func TestWelcomeModel_Update_QuitKey(t *testing.T) {
	model := NewWelcomeModel()

	// Send quit key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Update(KeyQ) should return tea.Quit command")
	}

	welcome, ok := updatedModel.(WelcomeModel)
	if !ok {
		t.Fatal("Update should return WelcomeModel")
	}

	if welcome.next {
		t.Error("next should be false after pressing quit key")
	}
}

func TestDetectionModel_Init(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	cmd := model.Init()

	if cmd == nil {
		t.Error("Init() should return a command (spinner tick + detect)")
	}
}

func TestDetectionModel_Update_WindowSize(t *testing.T) {
	model := NewDetectionModel("/tmp/test")

	// Send WindowSizeMsg
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := model.Update(msg)

	detection, ok := updatedModel.(DetectionModel)
	if !ok {
		t.Fatal("Update should return DetectionModel")
	}

	if !detection.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if detection.width != 80 {
		t.Errorf("width = %d, want 80", detection.width)
	}
	if detection.height != 24 {
		t.Errorf("height = %d, want 24", detection.height)
	}
}

func TestDetectionModel_Update_DetectMsg(t *testing.T) {
	model := NewDetectionModel("/tmp/test")

	// Send detectMsg
	msg := detectMsg{}
	updatedModel, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Update(detectMsg) should return runDetection command")
	}

	detection, ok := updatedModel.(DetectionModel)
	if !ok {
		t.Fatal("Update should return DetectionModel")
	}

	if !detection.detecting {
		t.Error("detecting should be true after detectMsg")
	}
}

func TestDetectionModel_Update_DetectionCompleteMsg(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	model.detecting = true

	// Create a test detection result
	testDetection := &templates.Detection{
		Language:    templates.LanguageGo,
		Platform:    templates.PlatformNative,
		ProjectType: templates.ProjectTypeCLI,
	}

	// Send detectionCompleteMsg
	msg := detectionCompleteMsg{
		detection: testDetection,
		err:       nil,
	}
	updatedModel, _ := model.Update(msg)

	detection, ok := updatedModel.(DetectionModel)
	if !ok {
		t.Fatal("Update should return DetectionModel")
	}

	if detection.detecting {
		t.Error("detecting should be false after detectionCompleteMsg")
	}
	if !detection.complete {
		t.Error("complete should be true after detectionCompleteMsg")
	}
	if detection.detection == nil {
		t.Error("detection should not be nil")
	}
	if detection.detection.Language != templates.LanguageGo {
		t.Errorf("Language = %v, want %v", detection.detection.Language, templates.LanguageGo)
	}
}

func TestDetectionModel_Update_ContinueKey(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	model.complete = true

	// Send enter key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(msg)

	detection, ok := updatedModel.(DetectionModel)
	if !ok {
		t.Fatal("Update should return DetectionModel")
	}

	if !detection.next {
		t.Error("next should be true after pressing enter")
	}
	if !detection.ShouldContinue() {
		t.Error("ShouldContinue() should return true")
	}
}

func TestDetectionModel_Update_SkipKey(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	model.detecting = false
	model.complete = false

	// Send 's' key (skip)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	updatedModel, _ := model.Update(msg)

	detection, ok := updatedModel.(DetectionModel)
	if !ok {
		t.Fatal("Update should return DetectionModel")
	}

	if !detection.complete {
		t.Error("complete should be true after skip")
	}
	if detection.detection == nil {
		t.Error("detection should not be nil after skip")
	}
	if detection.detection.Language != templates.LanguageUnknown {
		t.Errorf("Language should be Unknown after skip, got %v", detection.detection.Language)
	}
}

func TestDetectionModel_Update_QuitKey(t *testing.T) {
	model := NewDetectionModel("/tmp/test")
	model.complete = true

	// Send quit key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Update(KeyQ) should return tea.Quit command")
	}

	detection, ok := updatedModel.(DetectionModel)
	if !ok {
		t.Fatal("Update should return DetectionModel")
	}

	if detection.next {
		t.Error("next should be false after quit")
	}
}

// TemplateModel tests

func TestTemplateModel_Init(t *testing.T) {
	registry, _ := templates.NewRegistry()
	model := NewTemplateModel(registry, nil)
	cmd := model.Init()

	if cmd != nil {
		t.Error("Init() should return nil for TemplateModel")
	}
}

func TestTemplateModel_Update_WindowSize(t *testing.T) {
	registry, _ := templates.NewRegistry()
	model := NewTemplateModel(registry, nil)

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := model.Update(msg)

	template, ok := updatedModel.(TemplateModel)
	if !ok {
		t.Fatal("Update should return TemplateModel")
	}

	if !template.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if template.width != 80 {
		t.Errorf("width = %d, want 80", template.width)
	}
}

func TestTemplateModel_Update_QuitKey(t *testing.T) {
	registry, _ := templates.NewRegistry()
	model := NewTemplateModel(registry, nil)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Update(KeyQ) should return tea.Quit command")
	}
}

// ReviewModel tests

func TestReviewModel_Init(t *testing.T) {
	model := NewReviewModel("test: config")
	cmd := model.Init()

	if cmd != nil {
		t.Error("Init() should return nil for ReviewModel")
	}
}

func TestReviewModel_Update_WindowSize(t *testing.T) {
	model := NewReviewModel("test: config")

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := model.Update(msg)

	review, ok := updatedModel.(ReviewModel)
	if !ok {
		t.Fatal("Update should return ReviewModel")
	}

	if !review.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if review.width != 80 {
		t.Errorf("width = %d, want 80", review.width)
	}
}

func TestReviewModel_Update_ApproveKey(t *testing.T) {
	model := NewReviewModel("test: config")
	model.ready = true

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(msg)

	review, ok := updatedModel.(ReviewModel)
	if !ok {
		t.Fatal("Update should return ReviewModel")
	}

	if !review.next {
		t.Error("next should be true after approve")
	}
	if !review.ShouldContinue() {
		t.Error("ShouldContinue() should return true")
	}
}

func TestReviewModel_Update_BackKey(t *testing.T) {
	model := NewReviewModel("test: config")
	model.ready = true

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updatedModel, _ := model.Update(msg)

	review, ok := updatedModel.(ReviewModel)
	if !ok {
		t.Fatal("Update should return ReviewModel")
	}

	if !review.back {
		t.Error("back should be true after back key")
	}
	if !review.ShouldGoBack() {
		t.Error("ShouldGoBack() should return true")
	}
}

func TestReviewModel_Update_QuitKey(t *testing.T) {
	model := NewReviewModel("test: config")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Update(KeyQ) should return tea.Quit command")
	}
}

// SuccessModel tests

func TestSuccessModel_Init(t *testing.T) {
	model := NewSuccessModel("/tmp/test.yaml")
	cmd := model.Init()

	if cmd != nil {
		t.Error("Init() should return nil for SuccessModel")
	}
}

func TestSuccessModel_Update_WindowSize(t *testing.T) {
	model := NewSuccessModel("/tmp/test.yaml")

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedModel, _ := model.Update(msg)

	success, ok := updatedModel.(SuccessModel)
	if !ok {
		t.Fatal("Update should return SuccessModel")
	}

	if !success.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if success.width != 80 {
		t.Errorf("width = %d, want 80", success.width)
	}
}

func TestSuccessModel_Update_ExitKey(t *testing.T) {
	model := NewSuccessModel("/tmp/test.yaml")
	model.ready = true

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := model.Update(msg)

	if cmd == nil {
		t.Error("Update(KeyEnter) should return tea.Quit command")
	}
}
