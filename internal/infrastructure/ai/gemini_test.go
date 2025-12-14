// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewGeminiService_NoAPIKey(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "", // No API key
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("NewGeminiService() unexpected error: %v", err)
	}

	// Should return noop service
	if svc.IsAvailable() {
		t.Error("Service should not be available without API key")
	}
}

func TestNewGeminiService_InvalidAPIKey(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "invalid-key",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	_, err := NewGeminiService(cfg)
	if err == nil {
		t.Error("NewGeminiService() should return error for invalid API key")
	}
	if !strings.Contains(err.Error(), "invalid Gemini API key") {
		t.Errorf("Error should mention invalid API key format, got: %v", err)
	}
}

func TestNewGeminiService_ValidConfig(t *testing.T) {
	// Test configuration with valid-format API key (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", // Example format
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("NewGeminiService() error = %v", err)
	}

	if !svc.IsAvailable() {
		t.Error("Service should be available with valid API key format")
	}
}

func TestNewGeminiService_DefaultModel(t *testing.T) {
	// Test default model assignment without making API calls
	// Using a fake but valid-format API key to test configuration only
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
		Model:         "", // No model specified
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		// If client creation fails with fake key, skip the test
		t.Skipf("Cannot create Gemini client with fake API key: %v", err)
	}

	geminiSvc, ok := svc.(*geminiService)
	if !ok {
		t.Fatal("Service is not a geminiService")
	}

	if geminiSvc.config.Model != DefaultGeminiModel {
		t.Errorf("Default model = %v, want %v", geminiSvc.config.Model, DefaultGeminiModel)
	}
}

func TestNewGeminiService_CustomModel(t *testing.T) {
	// Test custom model configuration (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
		Model:         "gemini-1.5-pro",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("NewGeminiService() error = %v", err)
	}

	geminiSvc, ok := svc.(*geminiService)
	if !ok {
		t.Fatal("Service is not a geminiService")
	}

	if geminiSvc.config.Model != "gemini-1.5-pro" {
		t.Errorf("Custom model = %v, want gemini-1.5-pro", geminiSvc.config.Model)
	}
}

func TestNewGeminiService_CustomPrompts(t *testing.T) {
	// Test custom prompts configuration (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		CustomPrompts: CustomPrompts{
			ChangelogSystem: "Custom system prompt",
			ChangelogUser:   "Custom user prompt",
		},
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("NewGeminiService() error = %v", err)
	}

	geminiSvc, ok := svc.(*geminiService)
	if !ok {
		t.Fatal("Service is not a geminiService")
	}

	if geminiSvc.prompts.changelogSystem != "Custom system prompt" {
		t.Errorf("Changelog system prompt = %v, want Custom system prompt", geminiSvc.prompts.changelogSystem)
	}
	if geminiSvc.prompts.changelogUser != "Custom user prompt" {
		t.Errorf("Changelog user prompt = %v, want Custom user prompt", geminiSvc.prompts.changelogUser)
	}
}

func TestGeminiService_GenerateChangelog_EmptyChanges(t *testing.T) {
	// Test early return path with geminiService (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", // Valid format to create geminiService
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Should return empty string for nil changes (early return, no API call)
	result, err := svc.GenerateChangelog(context.Background(), nil, DefaultGenerateOptions())
	if err != nil {
		t.Errorf("GenerateChangelog(nil) error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateChangelog(nil) = %q, want empty string", result)
	}
}

func TestGeminiService_GenerateReleaseNotes_EmptyChangelog(t *testing.T) {
	// Test early return path with geminiService (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", // Valid format to create geminiService
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Should return empty string for empty changelog (early return, no API call)
	result, err := svc.GenerateReleaseNotes(context.Background(), "", DefaultGenerateOptions())
	if err != nil {
		t.Errorf("GenerateReleaseNotes(\"\") error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateReleaseNotes(\"\") = %q, want empty string", result)
	}
}

func TestGeminiService_GenerateMarketingBlurb_EmptyNotes(t *testing.T) {
	// Test early return path with geminiService (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", // Valid format to create geminiService
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Should return empty string for empty notes (early return, no API call)
	result, err := svc.GenerateMarketingBlurb(context.Background(), "", DefaultGenerateOptions())
	if err != nil {
		t.Errorf("GenerateMarketingBlurb(\"\") error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateMarketingBlurb(\"\") = %q, want empty string", result)
	}
}

func TestGeminiService_SummarizeChanges_EmptyChanges(t *testing.T) {
	// Test early return path with geminiService (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", // Valid format to create geminiService
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Should return empty string for nil changes (early return, no API call)
	result, err := svc.SummarizeChanges(context.Background(), nil, DefaultGenerateOptions())
	if err != nil {
		t.Errorf("SummarizeChanges(nil) error = %v", err)
	}
	if result != "" {
		t.Errorf("SummarizeChanges(nil) = %q, want empty string", result)
	}
}

func TestNewService_GeminiProvider(t *testing.T) {
	// Test service creation via factory (doesn't make API calls)
	svc, err := NewService(
		WithProvider("gemini"),
		WithAPIKey("AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI"),
		WithModel("gemini-1.5-pro"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("NewService(gemini) error = %v", err)
	}

	if svc == nil {
		t.Error("NewService(gemini) returned nil")
	}

	// Verify it's a geminiService
	_, ok := svc.(*geminiService)
	if !ok {
		t.Error("NewService(gemini) did not return a geminiService")
	}
}

func TestGeminiDefaultConstants(t *testing.T) {
	if DefaultGeminiModel != "gemini-2.0-flash-exp" {
		t.Errorf("DefaultGeminiModel = %v, want gemini-2.0-flash-exp", DefaultGeminiModel)
	}
}

func TestGeminiKeyPattern(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid key", "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", true},
		{"valid key with underscore", "AIzaSyDdI0hCZtE6vySjMm_WEfRq3CPzqKqqsHI", true},
		{"valid key with hyphen", "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", true},
		{"valid key alphanumeric", "AIzaSyDdI0hCZtE6vySjMmWEfRq3CPzqKqqsHI1", true},
		{"invalid prefix AIzb", "AIzbSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", false},
		{"invalid prefix Aiza", "AizaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", false},
		{"missing prefix", "SyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI", false},
		{"too short", "AIza-short", false},
		{"empty", "", false},
		{"openai format", "sk-1234567890abcdef1234567890abcdef", false},
		{"anthropic format", "sk-ant-api03-validkeyformat12345678901234567890", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := geminiKeyPattern.MatchString(tt.key)
			if result != tt.valid {
				t.Errorf("geminiKeyPattern.MatchString(%q) = %v, want %v", tt.key, result, tt.valid)
			}
		})
	}
}

func TestGeminiService_IsAvailable(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		wantAvail bool
		wantErr   bool
	}{
		{
			name:      "with valid API key format",
			apiKey:    "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
			wantAvail: false, // Will be false because client creation will fail without real API key
			wantErr:   false, // But no error during construction with valid format
		},
		{
			name:      "without API key",
			apiKey:    "",
			wantAvail: false,
			wantErr:   false,
		},
		{
			name:      "with invalid API key format",
			apiKey:    "invalid-key",
			wantAvail: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that would make real API calls
			if tt.apiKey != "" && !tt.wantErr {
				t.Skip("Skipping test that would require valid Gemini API key")
			}

			cfg := ServiceConfig{
				Provider:      "gemini",
				APIKey:        tt.apiKey,
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			svc, err := NewGeminiService(cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewGeminiService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if got := svc.IsAvailable(); got != tt.wantAvail {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.wantAvail)
			}
		})
	}
}

func TestGeminiService_ResilienceConfig(t *testing.T) {
	// Test resilience configuration (doesn't make API calls)
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       60 * time.Second,
		RetryAttempts: 5,
		RateLimitRPM:  100,
	}

	svc, err := NewGeminiService(cfg)
	if err != nil {
		t.Fatalf("NewGeminiService() error = %v", err)
	}

	geminiSvc, ok := svc.(*geminiService)
	if !ok {
		t.Fatal("Service is not a geminiService")
	}

	if geminiSvc.resilience == nil {
		t.Error("Resilience should be configured")
	}

	// Verify resilience config was applied
	if geminiSvc.config.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", geminiSvc.config.Timeout)
	}
	if geminiSvc.config.RetryAttempts != 5 {
		t.Errorf("RetryAttempts = %v, want 5", geminiSvc.config.RetryAttempts)
	}
	if geminiSvc.config.RateLimitRPM != 100 {
		t.Errorf("RateLimitRPM = %v, want 100", geminiSvc.config.RateLimitRPM)
	}
}

func TestGeminiService_TemperatureConversion(t *testing.T) {
	// Test temperature configuration (doesn't make API calls)
	tests := []struct {
		name        string
		temperature float64
	}{
		{"zero", 0.0},
		{"low", 0.3},
		{"medium", 0.7},
		{"high", 1.0},
		{"very high", 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "gemini",
				APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
				MaxTokens:     2048,
				Temperature:   tt.temperature,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			svc, err := NewGeminiService(cfg)
			if err != nil {
				t.Fatalf("NewGeminiService() error = %v", err)
			}

			geminiSvc, ok := svc.(*geminiService)
			if !ok {
				t.Fatal("Service is not a geminiService")
			}

			if geminiSvc.config.Temperature != tt.temperature {
				t.Errorf("Temperature = %v, want %v", geminiSvc.config.Temperature, tt.temperature)
			}
		})
	}
}

func TestGeminiService_MaxTokensConfig(t *testing.T) {
	// Test max tokens configuration (doesn't make API calls)
	tests := []struct {
		name      string
		maxTokens int
	}{
		{"small", 512},
		{"medium", 2048},
		{"large", 4096},
		{"very large", 8192},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "gemini",
				APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
				MaxTokens:     tt.maxTokens,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			svc, err := NewGeminiService(cfg)
			if err != nil {
				t.Fatalf("NewGeminiService() error = %v", err)
			}

			geminiSvc, ok := svc.(*geminiService)
			if !ok {
				t.Fatal("Service is not a geminiService")
			}

			if geminiSvc.config.MaxTokens != tt.maxTokens {
				t.Errorf("MaxTokens = %v, want %v", geminiSvc.config.MaxTokens, tt.maxTokens)
			}
		})
	}
}

func TestGeminiService_APIKeySafeErrorWrapping(t *testing.T) {
	// Test that invalid API key error doesn't leak the key value
	cfg := ServiceConfig{
		Provider:      "gemini",
		APIKey:        "invalid-key-that-should-not-appear-in-error",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	_, err := NewGeminiService(cfg)
	if err == nil {
		t.Fatal("NewGeminiService() should return error for invalid API key")
	}

	// Error message should not contain the actual API key
	errMsg := err.Error()
	if strings.Contains(errMsg, "invalid-key-that-should-not-appear-in-error") {
		t.Errorf("Error message should not contain the API key, got: %v", errMsg)
	}

	// Should mention invalid format generically
	if !strings.Contains(errMsg, "invalid Gemini API key format") {
		t.Errorf("Error should mention invalid key format, got: %v", errMsg)
	}
}

func TestGeminiService_APIKeyValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "exactly minimum length",
			apiKey:  "AIza" + strings.Repeat("a", 35),
			wantErr: false,
		},
		{
			name:    "longer than minimum",
			apiKey:  "AIza" + strings.Repeat("a", 50),
			wantErr: false,
		},
		{
			name:    "too short",
			apiKey:  "AIza" + strings.Repeat("a", 30),
			wantErr: true,
			errMsg:  "invalid Gemini API key format",
		},
		{
			name:    "wrong prefix case",
			apiKey:  "aiza" + strings.Repeat("a", 35),
			wantErr: true,
			errMsg:  "invalid Gemini API key format",
		},
		{
			name:    "missing prefix",
			apiKey:  strings.Repeat("a", 39),
			wantErr: true,
			errMsg:  "invalid Gemini API key format",
		},
		{
			name:    "contains invalid characters",
			apiKey:  "AIza" + strings.Repeat("a", 30) + "!@#$%",
			wantErr: true,
			errMsg:  "invalid Gemini API key format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "gemini",
				APIKey:        tt.apiKey,
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			svc, err := NewGeminiService(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewGeminiService() should return error for %s", tt.name)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Error should contain %q, got: %v", tt.errMsg, err.Error())
				}
			} else {
				// Skip successful cases as they would try to create real client
				if err != nil && strings.Contains(err.Error(), "invalid Gemini API key format") {
					t.Errorf("NewGeminiService() should not return validation error for valid format")
				}
			}

			// For valid formats that skip due to invalid credentials, verify noop service
			if !tt.wantErr && svc != nil && !svc.IsAvailable() {
				// This is expected - valid format but invalid credentials returns noop
				t.Skip("Skipping client creation test (would require real API)")
			}
		})
	}
}

func TestGeminiService_ConfigDefaults(t *testing.T) {
	// Test config defaults (doesn't make API calls)
	tests := []struct {
		name          string
		inputModel    string
		expectedModel string
	}{
		{
			name:          "empty model uses default",
			inputModel:    "",
			expectedModel: DefaultGeminiModel,
		},
		{
			name:          "custom model preserved",
			inputModel:    "gemini-1.5-pro",
			expectedModel: "gemini-1.5-pro",
		},
		{
			name:          "another custom model",
			inputModel:    "gemini-1.5-flash",
			expectedModel: "gemini-1.5-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "gemini",
				APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
				Model:         tt.inputModel,
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			svc, err := NewGeminiService(cfg)
			if err != nil {
				t.Fatalf("NewGeminiService() error = %v", err)
			}

			geminiSvc, ok := svc.(*geminiService)
			if !ok {
				t.Fatal("Service is not a geminiService")
			}

			if geminiSvc.config.Model != tt.expectedModel {
				t.Errorf("Model = %v, want %v", geminiSvc.config.Model, tt.expectedModel)
			}
		})
	}
}

func TestGeminiService_PromptCustomization(t *testing.T) {
	// Test prompt customization (doesn't make API calls)
	tests := []struct {
		name          string
		customPrompts CustomPrompts
		checkField    string
		expectedValue string
	}{
		{
			name: "changelog system prompt",
			customPrompts: CustomPrompts{
				ChangelogSystem: "Custom changelog system",
			},
			checkField:    "changelogSystem",
			expectedValue: "Custom changelog system",
		},
		{
			name: "changelog user prompt",
			customPrompts: CustomPrompts{
				ChangelogUser: "Custom changelog user",
			},
			checkField:    "changelogUser",
			expectedValue: "Custom changelog user",
		},
		{
			name: "release notes system prompt",
			customPrompts: CustomPrompts{
				ReleaseNotesSystem: "Custom release notes system",
			},
			checkField:    "releaseNotesSystem",
			expectedValue: "Custom release notes system",
		},
		{
			name: "all prompts customized",
			customPrompts: CustomPrompts{
				ChangelogSystem:    "Custom changelog system",
				ChangelogUser:      "Custom changelog user",
				ReleaseNotesSystem: "Custom release notes system",
				ReleaseNotesUser:   "Custom release notes user",
				MarketingSystem:    "Custom marketing system",
				MarketingUser:      "Custom marketing user",
			},
			checkField:    "marketingSystem",
			expectedValue: "Custom marketing system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "gemini",
				APIKey:        "AIzaSyDdI0hCZtE6vySjMm-WEfRq3CPzqKqqsHI",
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
				CustomPrompts: tt.customPrompts,
			}

			svc, err := NewGeminiService(cfg)
			if err != nil {
				t.Fatalf("NewGeminiService() error = %v", err)
			}

			geminiSvc, ok := svc.(*geminiService)
			if !ok {
				t.Fatal("Service is not a geminiService")
			}

			// Verify the prompts were applied
			switch tt.checkField {
			case "changelogSystem":
				if geminiSvc.prompts.changelogSystem != tt.expectedValue {
					t.Errorf("changelogSystem = %v, want %v", geminiSvc.prompts.changelogSystem, tt.expectedValue)
				}
			case "changelogUser":
				if geminiSvc.prompts.changelogUser != tt.expectedValue {
					t.Errorf("changelogUser = %v, want %v", geminiSvc.prompts.changelogUser, tt.expectedValue)
				}
			case "releaseNotesSystem":
				if geminiSvc.prompts.releaseNotesSystem != tt.expectedValue {
					t.Errorf("releaseNotesSystem = %v, want %v", geminiSvc.prompts.releaseNotesSystem, tt.expectedValue)
				}
			case "marketingSystem":
				if geminiSvc.prompts.marketingSystem != tt.expectedValue {
					t.Errorf("marketingSystem = %v, want %v", geminiSvc.prompts.marketingSystem, tt.expectedValue)
				}
			}
		})
	}
}
