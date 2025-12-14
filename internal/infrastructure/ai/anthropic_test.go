// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewAnthropicService_NoAPIKey(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "", // No API key
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService() unexpected error: %v", err)
	}

	// Should return noop service
	if svc.IsAvailable() {
		t.Error("Service should not be available without API key")
	}
}

func TestNewAnthropicService_InvalidAPIKey(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "invalid-key",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	_, err := NewAnthropicService(cfg)
	if err == nil {
		t.Error("NewAnthropicService() should return error for invalid API key")
	}
	if !strings.Contains(err.Error(), "invalid Anthropic API key") {
		t.Errorf("Error should mention invalid API key format, got: %v", err)
	}
}

func TestNewAnthropicService_ValidConfig(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService() error = %v", err)
	}

	if !svc.IsAvailable() {
		t.Error("Service should be available with valid API key")
	}
}

func TestNewAnthropicService_DefaultModel(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		Model:         "", // No model specified
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService() error = %v", err)
	}

	anthropicSvc, ok := svc.(*anthropicService)
	if !ok {
		t.Fatal("Service is not an anthropicService")
	}

	if anthropicSvc.config.Model != DefaultAnthropicModel {
		t.Errorf("Default model = %v, want %v", anthropicSvc.config.Model, DefaultAnthropicModel)
	}
}

func TestNewAnthropicService_CustomModel(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		Model:         "claude-3-opus-20240229",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService() error = %v", err)
	}

	anthropicSvc, ok := svc.(*anthropicService)
	if !ok {
		t.Fatal("Service is not an anthropicService")
	}

	if anthropicSvc.config.Model != "claude-3-opus-20240229" {
		t.Errorf("Custom model = %v, want claude-3-opus-20240229", anthropicSvc.config.Model)
	}
}

func TestNewAnthropicService_CustomPrompts(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		CustomPrompts: CustomPrompts{
			ChangelogSystem: "Custom system prompt",
			ChangelogUser:   "Custom user prompt",
		},
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService() error = %v", err)
	}

	anthropicSvc, ok := svc.(*anthropicService)
	if !ok {
		t.Fatal("Service is not an anthropicService")
	}

	if anthropicSvc.prompts.changelogSystem != "Custom system prompt" {
		t.Errorf("Changelog system prompt = %v, want Custom system prompt", anthropicSvc.prompts.changelogSystem)
	}
	if anthropicSvc.prompts.changelogUser != "Custom user prompt" {
		t.Errorf("Changelog user prompt = %v, want Custom user prompt", anthropicSvc.prompts.changelogUser)
	}
}

func TestAnthropicService_GenerateChangelog_EmptyChanges(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
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

func TestAnthropicService_GenerateReleaseNotes_EmptyChangelog(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
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

func TestAnthropicService_GenerateMarketingBlurb_EmptyNotes(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
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

func TestAnthropicService_SummarizeChanges_EmptyChanges(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
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

func TestNewService_AnthropicProvider(t *testing.T) {
	svc, err := NewService(
		WithProvider("anthropic"),
		WithAPIKey("sk-ant-api03-validkeyformat12345678901234567890"),
		WithModel("claude-3-opus-20240229"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("NewService(anthropic) error = %v", err)
	}

	if svc == nil {
		t.Error("NewService(anthropic) returned nil")
	}

	// Verify it's an anthropicService
	_, ok := svc.(*anthropicService)
	if !ok {
		t.Error("NewService(anthropic) did not return an anthropicService")
	}
}

func TestNewService_ClaudeAlias(t *testing.T) {
	svc, err := NewService(
		WithProvider("claude"),
		WithAPIKey("sk-ant-api03-validkeyformat12345678901234567890"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("NewService(claude) error = %v", err)
	}

	// Verify it's an anthropicService
	_, ok := svc.(*anthropicService)
	if !ok {
		t.Error("NewService(claude) did not return an anthropicService")
	}
}

func TestAnthropicDefaultConstants(t *testing.T) {
	if DefaultAnthropicModel != "claude-sonnet-4-20250514" {
		t.Errorf("DefaultAnthropicModel = %v, want claude-sonnet-4-20250514", DefaultAnthropicModel)
	}
}

func TestToFloatPtr(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float32
	}{
		{"zero", 0.0, 0.0},
		{"positive", 0.7, 0.7},
		{"one", 1.0, 1.0},
		{"large", 2.0, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toFloatPtr(tt.input)
			if result == nil {
				t.Fatal("toFloatPtr returned nil")
			}
			if *result != tt.want {
				t.Errorf("toFloatPtr(%v) = %v, want %v", tt.input, *result, tt.want)
			}
		})
	}
}

func TestAnthropicKeyPattern(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid key", "sk-ant-api03-validkeyformat12345678901234567890", true},
		{"valid key short", "sk-ant-12345678901234567890", true},
		{"invalid prefix", "sk-invalid-12345678901234567890", false},
		{"missing prefix", "12345678901234567890", false},
		{"too short", "sk-ant-short", false},
		{"empty", "", false},
		{"openai format", "sk-1234567890abcdef1234567890abcdef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anthropicKeyPattern.MatchString(tt.key)
			if result != tt.valid {
				t.Errorf("anthropicKeyPattern.MatchString(%q) = %v, want %v", tt.key, result, tt.valid)
			}
		})
	}
}

func TestNewAnthropicService_BaseURL(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		BaseURL:       "https://custom-anthropic.example.com",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService() error = %v", err)
	}

	anthropicSvc, ok := svc.(*anthropicService)
	if !ok {
		t.Fatal("Service is not an anthropicService")
	}

	if anthropicSvc.config.BaseURL != "https://custom-anthropic.example.com" {
		t.Errorf("BaseURL = %v, want https://custom-anthropic.example.com", anthropicSvc.config.BaseURL)
	}
}
