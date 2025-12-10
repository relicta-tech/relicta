// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

func TestNewOpenAIService_NoAPIKey(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "", // No API key
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIService() unexpected error: %v", err)
	}

	// Should return noop service
	if svc.IsAvailable() {
		t.Error("Service should not be available without API key")
	}
}

func TestNewOpenAIService_InvalidAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{"no prefix", "invalid-key-without-sk"},
		{"wrong prefix", "api-1234567890abcdef"},
		{"too short", "sk-short"},
		{"empty", ""},
		{"special chars", "sk-!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "openai",
				APIKey:        tt.apiKey,
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			_, err := NewOpenAIService(cfg)
			if tt.apiKey == "" {
				// Empty key should return noop service, not error
				if err != nil {
					t.Errorf("NewOpenAIService() with empty key should not error, got: %v", err)
				}
			} else if err == nil {
				t.Error("NewOpenAIService() should return error for invalid API key")
			} else if !strings.Contains(err.Error(), "invalid OpenAI API key") {
				t.Errorf("Error should mention invalid API key format, got: %v", err)
			}
		})
	}
}

func TestNewOpenAIService_ValidConfig(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{"standard key", "sk-1234567890abcdef1234567890abcdef"},
		{"project key", "sk-proj-1234567890abcdef1234567890abcdef"},
		{"long key", "sk-1234567890abcdef1234567890abcdef1234567890abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ServiceConfig{
				Provider:      "openai",
				APIKey:        tt.apiKey,
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			}

			svc, err := NewOpenAIService(cfg)
			if err != nil {
				t.Fatalf("NewOpenAIService() error = %v", err)
			}

			if !svc.IsAvailable() {
				t.Error("Service should be available with valid API key")
			}
		})
	}
}

func TestNewOpenAIService_CustomBaseURL(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		BaseURL:       "https://custom.openai.com/v1",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIService() error = %v", err)
	}

	openaiSvc, ok := svc.(*openAIService)
	if !ok {
		t.Fatal("Service is not an openAIService")
	}

	if openaiSvc.config.BaseURL != "https://custom.openai.com/v1" {
		t.Errorf("BaseURL = %v, want https://custom.openai.com/v1", openaiSvc.config.BaseURL)
	}
}

func TestNewOpenAIService_CustomPrompts(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		CustomPrompts: CustomPrompts{
			ChangelogSystem: "Custom OpenAI system prompt",
			ChangelogUser:   "Custom OpenAI user prompt",
		},
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIService() error = %v", err)
	}

	openaiSvc, ok := svc.(*openAIService)
	if !ok {
		t.Fatal("Service is not an openAIService")
	}

	if openaiSvc.prompts.changelogSystem != "Custom OpenAI system prompt" {
		t.Errorf("Changelog system prompt = %v, want Custom OpenAI system prompt", openaiSvc.prompts.changelogSystem)
	}
	if openaiSvc.prompts.changelogUser != "Custom OpenAI user prompt" {
		t.Errorf("Changelog user prompt = %v, want Custom OpenAI user prompt", openaiSvc.prompts.changelogUser)
	}
}

func TestOpenAIService_GenerateChangelog_EmptyChanges(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	result, err := svc.GenerateChangelog(context.Background(), nil, DefaultGenerateOptions())
	if err != nil {
		t.Errorf("GenerateChangelog() error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateChangelog() = %v, want empty string", result)
	}
}

func TestOpenAIService_GenerateReleaseNotes_EmptyChangelog(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	result, err := svc.GenerateReleaseNotes(context.Background(), "", DefaultGenerateOptions())
	if err != nil {
		t.Errorf("GenerateReleaseNotes() error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateReleaseNotes() = %v, want empty string", result)
	}
}

func TestOpenAIService_GenerateMarketingBlurb_EmptyNotes(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	result, err := svc.GenerateMarketingBlurb(context.Background(), "", DefaultGenerateOptions())
	if err != nil {
		t.Errorf("GenerateMarketingBlurb() error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateMarketingBlurb() = %v, want empty string", result)
	}
}

func TestOpenAIService_SummarizeChanges_EmptyChanges(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	result, err := svc.SummarizeChanges(context.Background(), nil, DefaultGenerateOptions())
	if err != nil {
		t.Errorf("SummarizeChanges() error = %v", err)
	}
	if result != "" {
		t.Errorf("SummarizeChanges() = %v, want empty string", result)
	}
}

func TestOpenAIService_ServiceMethods(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RateLimitRPM:  60,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	openaiSvc, ok := svc.(*openAIService)
	if !ok {
		t.Fatal("Service is not an openAIService")
	}

	// Test service fields
	if openaiSvc.client == nil {
		t.Error("Client should not be nil")
	}
	if openaiSvc.config.APIKey != cfg.APIKey {
		t.Errorf("API key = %v, want %v", openaiSvc.config.APIKey, cfg.APIKey)
	}
}

func TestNewService_OpenAIProvider(t *testing.T) {
	svc, err := NewService(
		WithProvider("openai"),
		WithAPIKey("sk-1234567890abcdef1234567890abcdef"),
		WithModel("gpt-4"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("NewService(openai) error = %v", err)
	}

	if svc == nil {
		t.Error("NewService(openai) returned nil")
	}

	// Verify it's an openAIService
	_, ok := svc.(*openAIService)
	if !ok {
		t.Error("NewService(openai) did not return an openAIService")
	}
}

func TestNoopService_Methods(t *testing.T) {
	svc := &noopService{}

	ctx := context.Background()
	opts := DefaultGenerateOptions()

	changes := &git.CategorizedChanges{
		Features: []git.ConventionalCommit{
			{Type: "feat", Description: "test"},
		},
	}

	// Test GenerateChangelog
	result, err := svc.GenerateChangelog(ctx, changes, opts)
	if err != nil {
		t.Errorf("GenerateChangelog() error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateChangelog() = %v, want empty", result)
	}

	// Test GenerateReleaseNotes
	result, err = svc.GenerateReleaseNotes(ctx, "test changelog", opts)
	if err != nil {
		t.Errorf("GenerateReleaseNotes() error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateReleaseNotes() = %v, want empty", result)
	}

	// Test GenerateMarketingBlurb
	result, err = svc.GenerateMarketingBlurb(ctx, "test notes", opts)
	if err != nil {
		t.Errorf("GenerateMarketingBlurb() error = %v", err)
	}
	if result != "" {
		t.Errorf("GenerateMarketingBlurb() = %v, want empty", result)
	}

	// Test SummarizeChanges
	result, err = svc.SummarizeChanges(ctx, changes, opts)
	if err != nil {
		t.Errorf("SummarizeChanges() error = %v", err)
	}
	if result != "" {
		t.Errorf("SummarizeChanges() = %v, want empty", result)
	}

	// Test IsAvailable
	if svc.IsAvailable() {
		t.Error("IsAvailable() should return false for noop service")
	}
}

func TestOpenAIKeyPattern(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid standard key", "sk-1234567890abcdef1234567890abcdef", true},
		{"valid project key", "sk-proj-1234567890abcdef1234567890abcdef", true},
		{"valid long key", "sk-abcdefghij1234567890ABCDEFGHIJ1234567890", true},
		{"valid with dashes", "sk-12345678901234567890-abcdef", true},
		{"valid with underscores", "sk-12345678901234567890_abcdef", true},
		{"invalid no prefix", "1234567890abcdef1234567890abcdef", false},
		{"invalid wrong prefix", "api-1234567890abcdef1234567890abcdef", false},
		{"invalid too short", "sk-short", false},
		{"invalid empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openaiKeyPattern.MatchString(tt.key)
			if result != tt.valid {
				t.Errorf("openaiKeyPattern.MatchString(%q) = %v, want %v", tt.key, result, tt.valid)
			}
		})
	}
}
