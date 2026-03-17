// Package wizard provides terminal user interface components for the Relicta init wizard.
package wizard

import (
	"strings"
	"testing"
)

func TestWizardBanner(t *testing.T) {
	banner := wizardBanner()

	if banner == "" {
		t.Error("wizardBanner() should not be empty")
	}

	// Verify banner contains ASCII art characters
	// Banner is ASCII art, not literal text
	if len(banner) < 100 {
		t.Error("banner should be substantial ASCII art (> 100 chars)")
	}

	// Should contain common ASCII art box-drawing characters
	if !strings.Contains(banner, "|") && !strings.Contains(banner, "_") {
		t.Error("banner should contain ASCII art characters like | or _")
	}
}

func TestWizardWelcome(t *testing.T) {
	welcome := wizardWelcome()

	if welcome == "" {
		t.Error("wizardWelcome() should not be empty")
	}

	// Verify welcome message contains expected phrases
	expectedPhrases := []string{
		"Welcome",
		"Relicta",
		"wizard",
		"2 minutes",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(welcome, phrase) {
			t.Errorf("welcome message should contain %q", phrase)
		}
	}
}

func TestRenderStepIndicator(t *testing.T) {
	styles := defaultWizardStyles()

	tests := []struct {
		name           string
		currentState   WizardState
		shouldContain  []string
		shouldNotEmpty bool
	}{
		{
			name:         "Welcome state",
			currentState: StateWelcome,
			shouldContain: []string{
				"Welcome",
				"Detecting",
			},
			shouldNotEmpty: true,
		},
		{
			name:         "Detecting state",
			currentState: StateDetecting,
			shouldContain: []string{
				"Welcome",
				"Detecting",
				"Project Type",
			},
			shouldNotEmpty: true,
		},
		{
			name:         "Template state",
			currentState: StateTemplate,
			shouldContain: []string{
				"Template",
				"Configuration",
			},
			shouldNotEmpty: true,
		},
		{
			name:         "Success state",
			currentState: StateSuccess,
			shouldContain: []string{
				"Complete",
			},
			shouldNotEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderStepIndicator(tt.currentState, styles)

			if tt.shouldNotEmpty && result == "" {
				t.Error("renderStepIndicator() should not be empty")
			}

			for _, phrase := range tt.shouldContain {
				if !strings.Contains(result, phrase) {
					t.Errorf("step indicator should contain %q, got: %s", phrase, result)
				}
			}
		})
	}
}

func TestRenderHelp(t *testing.T) {
	styles := defaultWizardStyles()

	tests := []struct {
		name      string
		shortcuts []string
		wantEmpty bool
	}{
		{
			name:      "Single shortcut",
			shortcuts: []string{"Enter: Continue"},
			wantEmpty: false,
		},
		{
			name:      "Multiple shortcuts",
			shortcuts: []string{"Enter: Continue", "Esc: Quit", "Tab: Next"},
			wantEmpty: false,
		},
		{
			name:      "No shortcuts",
			shortcuts: []string{},
			wantEmpty: false, // Still renders "Shortcuts: " prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderHelp(tt.shortcuts, styles)

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("renderHelp() should be empty, got %q", result)
				}
				return
			}

			// Should always contain "Shortcuts"
			if !strings.Contains(result, "Shortcuts") {
				t.Errorf("help should contain 'Shortcuts', got: %s", result)
			}

			// Should contain each shortcut
			for _, shortcut := range tt.shortcuts {
				if !strings.Contains(result, shortcut) {
					t.Errorf("help should contain shortcut %q, got: %s", shortcut, result)
				}
			}
		})
	}
}

func TestDefaultWizardStyles(t *testing.T) {
	styles := defaultWizardStyles()

	// Verify all styles are initialized (non-nil)
	// We can't directly check if they're "nil" since they're structs,
	// but we can verify they have some properties set

	// Test a few key styles to ensure they're configured
	if styles.title.GetBold() != true {
		t.Error("title style should be bold")
	}

	if styles.italic.GetItalic() != true {
		t.Error("italic style should be italic")
	}

	if styles.bold.GetBold() != true {
		t.Error("bold style should be bold")
	}

	// Verify subtitle has italic
	if styles.subtitle.GetItalic() != true {
		t.Error("subtitle style should be italic")
	}
}

func TestWizardResult(t *testing.T) {
	tests := []struct {
		name   string
		result WizardResult
	}{
		{
			name: "Success result",
			result: WizardResult{
				State:  StateSuccess,
				Config: "test config",
				Error:  nil,
			},
		},
		{
			name: "Error result",
			result: WizardResult{
				State:  StateError,
				Config: "",
				Error:  ErrTest,
			},
		},
		{
			name: "Quit result",
			result: WizardResult{
				State:  StateQuit,
				Config: "",
				Error:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify expected values match what was set
			switch tt.name {
			case "Success result":
				if tt.result.State != StateSuccess {
					t.Errorf("State = %v, want StateSuccess", tt.result.State)
				}
				if tt.result.Config != "test config" {
					t.Errorf("Config = %v, want 'test config'", tt.result.Config)
				}
				if tt.result.Error != nil {
					t.Errorf("Error = %v, want nil", tt.result.Error)
				}
			case "Error result":
				if tt.result.State != StateError {
					t.Errorf("State = %v, want StateError", tt.result.State)
				}
				if tt.result.Error == nil {
					t.Error("Error should not be nil for error result")
				}
			case "Quit result":
				if tt.result.State != StateQuit {
					t.Errorf("State = %v, want StateQuit", tt.result.State)
				}
			}
		})
	}
}

// ErrTest is a test error for testing error results.
var ErrTest = &testError{msg: "test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
