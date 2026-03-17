// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"strings"
	"testing"
	"time"
)

func TestToneConstants(t *testing.T) {
	tests := []struct {
		tone Tone
		want string
	}{
		{ToneTechnical, "technical"},
		{ToneFriendly, "friendly"},
		{ToneProfessional, "professional"},
		{ToneExcited, "excited"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.tone) != tt.want {
				t.Errorf("Tone constant = %v, want %v", tt.tone, tt.want)
			}
		})
	}
}

func TestAudienceConstants(t *testing.T) {
	tests := []struct {
		audience Audience
		want     string
	}{
		{AudienceDevelopers, "developers"},
		{AudienceUsers, "users"},
		{AudiencePublic, "public"},
		{AudienceMarketing, "marketing"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.audience) != tt.want {
				t.Errorf("Audience constant = %v, want %v", tt.audience, tt.want)
			}
		})
	}
}

func TestDefaultGenerateOptions(t *testing.T) {
	opts := DefaultGenerateOptions()

	if opts.Tone != ToneProfessional {
		t.Errorf("Default Tone = %v, want professional", opts.Tone)
	}
	if opts.Audience != AudienceDevelopers {
		t.Errorf("Default Audience = %v, want developers", opts.Audience)
	}
	if opts.MaxLength != 0 {
		t.Errorf("Default MaxLength = %v, want 0", opts.MaxLength)
	}
	if opts.IncludeEmoji {
		t.Error("Default IncludeEmoji should be false")
	}
	if opts.Language != "English" {
		t.Errorf("Default Language = %v, want English", opts.Language)
	}
}

func TestGenerateOptions_Fields(t *testing.T) {
	opts := GenerateOptions{
		ProductName:  "MyProduct",
		Tone:         ToneFriendly,
		Audience:     AudienceUsers,
		MaxLength:    500,
		IncludeEmoji: true,
		Context:      "Additional context",
		Language:     "Spanish",
	}

	if opts.ProductName != "MyProduct" {
		t.Errorf("ProductName = %v, want MyProduct", opts.ProductName)
	}
	if opts.Tone != ToneFriendly {
		t.Errorf("Tone = %v, want friendly", opts.Tone)
	}
	if opts.Audience != AudienceUsers {
		t.Errorf("Audience = %v, want users", opts.Audience)
	}
	if opts.MaxLength != 500 {
		t.Errorf("MaxLength = %v, want 500", opts.MaxLength)
	}
	if !opts.IncludeEmoji {
		t.Error("IncludeEmoji should be true")
	}
	if opts.Context != "Additional context" {
		t.Errorf("Context = %v, want Additional context", opts.Context)
	}
	if opts.Language != "Spanish" {
		t.Errorf("Language = %v, want Spanish", opts.Language)
	}
}

func TestDefaultServiceConfig(t *testing.T) {
	cfg := DefaultServiceConfig()

	if cfg.Provider != "openai" {
		t.Errorf("Default Provider = %v, want openai", cfg.Provider)
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Default Model = %v, want gpt-4", cfg.Model)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("Default MaxTokens = %v, want 2048", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Default Temperature = %v, want 0.7", cfg.Temperature)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Default Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("Default RetryAttempts = %v, want 3", cfg.RetryAttempts)
	}
	if cfg.RateLimitRPM != 60 {
		t.Errorf("Default RateLimitRPM = %v, want 60", cfg.RateLimitRPM)
	}
}

func TestServiceOptions(t *testing.T) {
	cfg := DefaultServiceConfig()

	// Apply options
	WithProvider("custom")(&cfg)
	WithAPIKey("test-key")(&cfg)
	WithBaseURL("https://custom.api.com")(&cfg)
	WithModel("gpt-3.5-turbo")(&cfg)
	WithMaxTokens(1024)(&cfg)
	WithTemperature(0.5)(&cfg)
	WithTimeout(60 * time.Second)(&cfg)
	WithRetryAttempts(5)(&cfg)
	WithRateLimit(30)(&cfg)
	WithCustomPrompts(CustomPrompts{
		ChangelogSystem: "Custom system prompt",
	})(&cfg)

	if cfg.Provider != "custom" {
		t.Errorf("Provider = %v, want custom", cfg.Provider)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %v, want test-key", cfg.APIKey)
	}
	if cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %v, want https://custom.api.com", cfg.BaseURL)
	}
	if cfg.Model != "gpt-3.5-turbo" {
		t.Errorf("Model = %v, want gpt-3.5-turbo", cfg.Model)
	}
	if cfg.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %v, want 1024", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", cfg.Temperature)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
	}
	if cfg.RetryAttempts != 5 {
		t.Errorf("RetryAttempts = %v, want 5", cfg.RetryAttempts)
	}
	if cfg.RateLimitRPM != 30 {
		t.Errorf("RateLimitRPM = %v, want 30", cfg.RateLimitRPM)
	}
	if cfg.CustomPrompts.ChangelogSystem != "Custom system prompt" {
		t.Errorf("CustomPrompts.ChangelogSystem = %v, want Custom system prompt", cfg.CustomPrompts.ChangelogSystem)
	}
}

func TestCustomPrompts_Fields(t *testing.T) {
	prompts := CustomPrompts{
		ChangelogSystem:    "Changelog system",
		ChangelogUser:      "Changelog user",
		ReleaseNotesSystem: "Release notes system",
		ReleaseNotesUser:   "Release notes user",
		MarketingSystem:    "Marketing system",
		MarketingUser:      "Marketing user",
	}

	if prompts.ChangelogSystem != "Changelog system" {
		t.Errorf("ChangelogSystem = %v, want Changelog system", prompts.ChangelogSystem)
	}
	if prompts.ChangelogUser != "Changelog user" {
		t.Errorf("ChangelogUser = %v, want Changelog user", prompts.ChangelogUser)
	}
	if prompts.ReleaseNotesSystem != "Release notes system" {
		t.Errorf("ReleaseNotesSystem = %v, want Release notes system", prompts.ReleaseNotesSystem)
	}
	if prompts.ReleaseNotesUser != "Release notes user" {
		t.Errorf("ReleaseNotesUser = %v, want Release notes user", prompts.ReleaseNotesUser)
	}
	if prompts.MarketingSystem != "Marketing system" {
		t.Errorf("MarketingSystem = %v, want Marketing system", prompts.MarketingSystem)
	}
	if prompts.MarketingUser != "Marketing user" {
		t.Errorf("MarketingUser = %v, want Marketing user", prompts.MarketingUser)
	}
}

func TestNewService_UnsupportedProvider(t *testing.T) {
	_, err := NewService(
		WithProvider("unsupported-provider"),
		WithAPIKey("test-key"),
	)
	if err == nil {
		t.Error("NewService() should return error for unsupported provider")
	}
}

func TestNewService_OllamaWithOptions(t *testing.T) {
	svc, err := NewService(
		WithProvider("ollama"),
		WithModel("llama3.2"),
		WithBaseURL("http://localhost:11434/v1"),
	)
	if err != nil {
		t.Fatalf("NewService(ollama) error = %v", err)
	}
	if svc == nil {
		t.Error("NewService(ollama) returned nil")
	}
}

func TestNewService_WithAllOptions(t *testing.T) {
	svc, err := NewService(
		WithProvider("openai"),
		WithAPIKey("sk-1234567890abcdef1234567890abcdef"),
		WithBaseURL("https://api.custom.com/v1"),
		WithModel("gpt-4-turbo"),
		WithMaxTokens(4096),
		WithTemperature(0.8),
		WithTimeout(60*time.Second),
		WithRetryAttempts(5),
		WithRateLimit(30),
		WithCustomPrompts(CustomPrompts{
			ChangelogSystem: "Test system",
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if svc == nil {
		t.Error("NewService() returned nil")
	}
}

func TestNewService_AzureOpenAI(t *testing.T) {
	key := strings.Repeat("a", 32)
	svc, err := NewService(
		WithProvider("azure-openai"),
		WithAPIKey(key),
		WithBaseURL("https://example.openai.azure.com/"),
		WithAPIVersion("2024-03-01-preview"),
	)
	if err != nil {
		t.Fatalf("NewService(azure-openai) error = %v", err)
	}
	if svc == nil {
		t.Fatal("azure-openai service should not be nil")
	}
	if !svc.IsAvailable() {
		t.Error("azure-openai service should report available")
	}
}
