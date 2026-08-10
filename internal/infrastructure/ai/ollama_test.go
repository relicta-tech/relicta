// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOllamaService(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ServiceConfig
		wantErr bool
	}{
		{
			name: "default config",
			cfg: ServiceConfig{
				Provider:      "ollama",
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
			},
			wantErr: false,
		},
		{
			name: "with custom base URL",
			cfg: ServiceConfig{
				Provider:      "ollama",
				BaseURL:       "http://custom-ollama:11434/v1",
				Model:         "mistral",
				MaxTokens:     1024,
				Temperature:   0.5,
				Timeout:       60 * time.Second,
				RetryAttempts: 5,
			},
			wantErr: false,
		},
		{
			name: "with custom prompts",
			cfg: ServiceConfig{
				Provider:      "ollama",
				Model:         "llama3.2",
				MaxTokens:     2048,
				Temperature:   0.7,
				Timeout:       30 * time.Second,
				RetryAttempts: 3,
				CustomPrompts: CustomPrompts{
					ChangelogSystem:    "Custom changelog system prompt",
					ChangelogUser:      "Custom changelog user prompt",
					ReleaseNotesSystem: "Custom release notes system prompt",
					ReleaseNotesUser:   "Custom release notes user prompt",
					MarketingSystem:    "Custom marketing system prompt",
					MarketingUser:      "Custom marketing user prompt",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewOllamaService(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOllamaService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && svc == nil {
				t.Error("NewOllamaService() returned nil service")
			}
		})
	}
}

func TestOllamaService_IsAvailable(t *testing.T) {
	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if !svc.IsAvailable() {
		t.Error("Ollama service should be available after creation")
	}
}

func TestOllamaService_DefaultValues(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "ollama",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOllamaService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc, ok := svc.(*ollamaService)
	if !ok {
		t.Fatal("Service is not an ollamaService")
	}

	// Check that default model is set
	if ollamaSvc.config.Model != DefaultOllamaModel {
		t.Errorf("Default model = %v, want %v", ollamaSvc.config.Model, DefaultOllamaModel)
	}
}

func TestOllamaService_CustomModel(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "ollama",
		Model:         "codellama:13b",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}

	svc, err := NewOllamaService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc, ok := svc.(*ollamaService)
	if !ok {
		t.Fatal("Service is not an ollamaService")
	}

	// Check that custom model is preserved
	if ollamaSvc.config.Model != "codellama:13b" {
		t.Errorf("Custom model = %v, want codellama:13b", ollamaSvc.config.Model)
	}
}

func TestOllamaService_CustomPrompts(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "ollama",
		Model:         "llama3.2",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		CustomPrompts: CustomPrompts{
			ChangelogSystem: "Custom system prompt",
			ChangelogUser:   "Custom user prompt",
		},
	}

	svc, err := NewOllamaService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc, ok := svc.(*ollamaService)
	if !ok {
		t.Fatal("Service is not an ollamaService")
	}

	if ollamaSvc.prompts.changelogSystem != "Custom system prompt" {
		t.Errorf("Changelog system prompt = %v, want Custom system prompt", ollamaSvc.prompts.changelogSystem)
	}
	if ollamaSvc.prompts.changelogUser != "Custom user prompt" {
		t.Errorf("Changelog user prompt = %v, want Custom user prompt", ollamaSvc.prompts.changelogUser)
	}
}

func TestOllamaService_GenerateChangelog_EmptyChanges(t *testing.T) {
	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
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

func TestOllamaService_GenerateReleaseNotes_EmptyChangelog(t *testing.T) {
	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
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

func TestOllamaService_GenerateMarketingBlurb_EmptyNotes(t *testing.T) {
	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
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

func TestOllamaService_SummarizeChanges_EmptyChanges(t *testing.T) {
	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
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

func TestBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		template string
		opts     GenerateOptions
		contains []string
	}{
		{
			name:     "technical tone",
			template: "Base prompt",
			opts:     GenerateOptions{Tone: ToneTechnical},
			contains: []string{"Base prompt", "technical", "developer-focused"},
		},
		{
			name:     "friendly tone",
			template: "Base prompt",
			opts:     GenerateOptions{Tone: ToneFriendly},
			contains: []string{"Base prompt", "friendly", "casual"},
		},
		{
			name:     "professional tone",
			template: "Base prompt",
			opts:     GenerateOptions{Tone: ToneProfessional},
			contains: []string{"Base prompt", "professional", "formal"},
		},
		{
			name:     "excited tone",
			template: "Base prompt",
			opts:     GenerateOptions{Tone: ToneExcited},
			contains: []string{"Base prompt", "enthusiastic", "excited"},
		},
		{
			name:     "developers audience",
			template: "Base prompt",
			opts:     GenerateOptions{Audience: AudienceDevelopers},
			contains: []string{"Base prompt", "software developers"},
		},
		{
			name:     "users audience",
			template: "Base prompt",
			opts:     GenerateOptions{Audience: AudienceUsers},
			contains: []string{"Base prompt", "end users"},
		},
		{
			name:     "with emoji",
			template: "Base prompt",
			opts:     GenerateOptions{IncludeEmoji: true},
			contains: []string{"Base prompt", "Include relevant emojis"},
		},
		{
			name:     "without emoji",
			template: "Base prompt",
			opts:     GenerateOptions{IncludeEmoji: false},
			contains: []string{"Base prompt", "Do not include emojis"},
		},
		{
			name:     "custom language",
			template: "Base prompt",
			opts:     GenerateOptions{Language: "Spanish"},
			contains: []string{"Base prompt", "Spanish"},
		},
		{
			name:     "max length",
			template: "Base prompt",
			opts:     GenerateOptions{MaxLength: 500},
			contains: []string{"Base prompt", "500", "characters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSystemPrompt(tt.template, tt.opts)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("buildSystemPrompt() missing substring %q in result: %s", substr, result)
				}
			}
		})
	}
}

func TestBuildUserPrompt(t *testing.T) {
	tests := []struct {
		name     string
		template string
		content  string
		opts     GenerateOptions
		contains []string
	}{
		{
			name:     "with content",
			template: "Generate for {{CONTENT}}",
			content:  "test content",
			opts:     GenerateOptions{},
			contains: []string{"test content"},
		},
		{
			name:     "with product name",
			template: "{{PRODUCT_NAME}} release",
			content:  "",
			opts:     GenerateOptions{ProductName: "MyApp"},
			contains: []string{"MyApp"},
		},
		{
			name:     "default product name",
			template: "{{PRODUCT_NAME}} release",
			content:  "",
			opts:     GenerateOptions{},
			contains: []string{"the project"},
		},
		{
			name:     "with context",
			template: "Base prompt",
			content:  "",
			opts:     GenerateOptions{Context: "Additional info"},
			contains: []string{"Base prompt", "Additional context", "Additional info"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildUserPrompt(tt.template, tt.content, tt.opts)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("buildUserPrompt() missing substring %q in result: %s", substr, result)
				}
			}
		})
	}
}

func TestOllamaService_CheckConnection(t *testing.T) {
	// Create a mock server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ollama is running"))
	}))
	defer server.Close()

	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		BaseURL:       server.URL + "/v1",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc := svc.(*ollamaService)

	ctx := context.Background()
	err = ollamaSvc.CheckConnection(ctx)
	if err != nil {
		t.Errorf("CheckConnection() error = %v, want nil", err)
	}
}

func TestOllamaService_CheckConnection_Failure(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	svc, err := NewOllamaService(ServiceConfig{
		Provider:      "ollama",
		BaseURL:       server.URL + "/v1",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc := svc.(*ollamaService)

	ctx := context.Background()
	err = ollamaSvc.CheckConnection(ctx)
	if err == nil {
		t.Error("CheckConnection() should return error for failing server")
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"connection refused", mockError("connection refused"), true},
		{"no such host", mockError("no such host"), true},
		{"network unreachable", mockError("network is unreachable"), true},
		{"connection reset", mockError("connection reset"), true},
		{"i/o timeout", mockError("i/o timeout"), true},
		{"rate limit exceeded", mockError("rate limit exceeded"), true},
		{"too many requests", mockError("too many requests"), true},
		{"server error 500", mockError("500 internal server error"), true},
		{"bad gateway 502", mockError("502 bad gateway"), true},
		{"unauthorized 401", mockError("401 unauthorized"), false},
		{"bad request 400", mockError("400 bad request"), false},
		{"other error", mockError("some other error"), true}, // Unknown errors are retried by default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Errorf("isRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockError is a simple error type for testing
type mockError string

func (e mockError) Error() string {
	return string(e)
}

func TestNewService_OllamaProvider(t *testing.T) {
	svc, err := NewService(
		WithProvider("ollama"),
		WithModel("llama3.2"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("NewService(ollama) error = %v", err)
	}

	if svc == nil {
		t.Error("NewService(ollama) returned nil")
	}

	// Verify it's an ollamaService
	_, ok := svc.(*ollamaService)
	if !ok {
		t.Error("NewService(ollama) did not return an ollamaService")
	}
}

func TestOllamaDefaultConstants(t *testing.T) {
	if DefaultOllamaBaseURL != "http://localhost:11434/v1" {
		t.Errorf("DefaultOllamaBaseURL = %v, want http://localhost:11434/v1", DefaultOllamaBaseURL)
	}
	if DefaultOllamaModel != "llama3.2" {
		t.Errorf("DefaultOllamaModel = %v, want llama3.2", DefaultOllamaModel)
	}
}
