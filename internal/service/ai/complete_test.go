// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// TestOpenAIComplete_ContextCancellation tests context cancellation in complete()
func TestOpenAIComplete_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "response"}},
			},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		BaseURL:       server.URL,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RateLimitRPM:  0, // No rate limiting
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	openaiSvc := svc.(*openAIService)

	// Create a context that we'll cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = openaiSvc.complete(ctx, "system", "user")
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
	if !strings.Contains(err.Error(), "context cancel") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// TestOllamaComplete_ContextCancellation tests context cancellation in ollama complete()
func TestOllamaComplete_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "response"}},
			},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "ollama",
		BaseURL:       server.URL,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RateLimitRPM:  0, // No rate limiting
	}

	svc, err := NewOllamaService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc := svc.(*ollamaService)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = ollamaSvc.complete(ctx, "system", "user")
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
	if !strings.Contains(err.Error(), "context cancel") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// TestAnthropicComplete_ContextCancellation tests context cancellation in anthropic complete()
func TestAnthropicComplete_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "response"},
			},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		BaseURL:       server.URL,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RateLimitRPM:  0, // No rate limiting
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	anthropicSvc := svc.(*anthropicService)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = anthropicSvc.complete(ctx, "system", "user")
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
	if !strings.Contains(err.Error(), "context cancel") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// TestOpenAIComplete_RateLimiterError tests rate limiter context cancellation
func TestOpenAIComplete_RateLimiterError(t *testing.T) {
	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RateLimitRPM:  1, // Very low rate limit
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	openaiSvc := svc.(*openAIService)

	// Create a context with immediate timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait a bit to ensure context expires
	time.Sleep(10 * time.Millisecond)

	_, err = openaiSvc.complete(ctx, "system", "user")
	if err == nil {
		t.Error("Expected error due to context timeout, got nil")
	}
}

// TestOllamaComplete_EmptyResponse tests handling of empty response
func TestOllamaComplete_EmptyResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		// Return response with no choices
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "ollama",
		BaseURL:       server.URL,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       1 * time.Second,
		RetryAttempts: 2,
		RateLimitRPM:  0,
	}

	svc, err := NewOllamaService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ollamaSvc := svc.(*ollamaService)
	_, err = ollamaSvc.complete(context.Background(), "system", "user")
	if err == nil {
		t.Error("Expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "no response") {
		t.Errorf("Expected 'no response' error, got: %v", err)
	}

	// Should retry
	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts due to retries, got %d", attempts)
	}
}

// TestOpenAIComplete_EmptyResponse tests handling of empty response
func TestOpenAIComplete_EmptyResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		BaseURL:       server.URL,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       1 * time.Second,
		RetryAttempts: 2,
		RateLimitRPM:  0,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	openaiSvc := svc.(*openAIService)
	_, err = openaiSvc.complete(context.Background(), "system", "user")
	if err == nil {
		t.Error("Expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "no response") {
		t.Errorf("Expected 'no response' error, got: %v", err)
	}
}

// TestAnthropicComplete_EmptyResponse tests handling of empty response
func TestAnthropicComplete_EmptyResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-api03-validkeyformat12345678901234567890",
		BaseURL:       server.URL,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       1 * time.Second,
		RetryAttempts: 2,
		RateLimitRPM:  0,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	anthropicSvc := svc.(*anthropicService)
	_, err = anthropicSvc.complete(context.Background(), "system", "user")
	if err == nil {
		t.Error("Expected error for empty response, got nil")
	}
	if !strings.Contains(err.Error(), "no response") {
		t.Errorf("Expected 'no response' error, got: %v", err)
	}
}

// TestIsRetryableError_EdgeCases tests additional error patterns
func TestIsRetryableError_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"connection refused lowercase", errors.New("connection refused"), true},
		{"no such host", errors.New("dial tcp: lookup example.com: no such host"), true},
		{"network unreachable", errors.New("network is unreachable"), true},
		{"connection reset by peer", errors.New("connection reset by peer"), true},
		{"connection reset", errors.New("read: connection reset"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"context deadline", context.DeadlineExceeded, false},
		{"context canceled", context.Canceled, false},
		{"rate limit 429", errors.New("429 Too Many Requests"), true},
		{"server error 500", errors.New("500 Internal Server Error"), true},
		{"bad gateway 502", errors.New("502 Bad Gateway"), true},
		{"service unavailable 503", errors.New("503 Service Unavailable"), true},
		{"gateway timeout 504", errors.New("504 Gateway Timeout"), true},
		{"bad request 400", errors.New("400 Bad Request"), false},
		{"unauthorized 401", errors.New("401 Unauthorized"), false},
		{"forbidden 403", errors.New("403 Forbidden"), false},
		{"not found 404", errors.New("404 Not Found"), false},
		{"EOF", errors.New("EOF"), true},               // Unknown error, default retry
		{"invalid response", errors.New("invalid response"), true}, // Unknown error, default retry
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestFormatChangesForPrompt_AllCategories tests formatting with all change categories
func TestFormatChangesForPrompt_AllCategories(t *testing.T) {
	changes := &git.CategorizedChanges{
		Breaking: []git.ConventionalCommit{
			{Type: "feat", Description: "breaking API change", Scope: "api", Breaking: true},
		},
		Features: []git.ConventionalCommit{
			{Type: "feat", Description: "new feature", Scope: "core"},
			{Type: "feat", Description: "breaking feature", Breaking: true}, // Should be filtered
		},
		Fixes: []git.ConventionalCommit{
			{Type: "fix", Description: "bug fix"},
		},
		Performance: []git.ConventionalCommit{
			{Type: "perf", Description: "performance improvement", Scope: "db"},
		},
		Documentation: []git.ConventionalCommit{
			{Type: "docs", Description: "update docs"},
		},
		Other: []git.ConventionalCommit{
			{Type: "chore", Description: "other change"},
		},
	}

	result := formatChangesForPrompt(changes)

	// Check all sections are present
	if !strings.Contains(result, "BREAKING CHANGES:") {
		t.Error("Missing BREAKING CHANGES section")
	}
	if !strings.Contains(result, "NEW FEATURES:") {
		t.Error("Missing NEW FEATURES section")
	}
	if !strings.Contains(result, "BUG FIXES:") {
		t.Error("Missing BUG FIXES section")
	}
	if !strings.Contains(result, "PERFORMANCE IMPROVEMENTS:") {
		t.Error("Missing PERFORMANCE IMPROVEMENTS section")
	}
	if !strings.Contains(result, "DOCUMENTATION:") {
		t.Error("Missing DOCUMENTATION section")
	}
	if !strings.Contains(result, "OTHER CHANGES:") {
		t.Error("Missing OTHER CHANGES section")
	}

	// Check scopes are included
	if !strings.Contains(result, "(api)") {
		t.Error("Missing scope (api)")
	}
	if !strings.Contains(result, "(core)") {
		t.Error("Missing scope (core)")
	}

	// Check breaking feature is not in NEW FEATURES (it's breaking)
	lines := strings.Split(result, "\n")
	inFeatures := false
	for _, line := range lines {
		if strings.Contains(line, "NEW FEATURES:") {
			inFeatures = true
		} else if strings.HasPrefix(line, "BUG FIXES:") {
			inFeatures = false
		}
		if inFeatures && strings.Contains(line, "breaking feature") {
			t.Error("Breaking feature should not appear in NEW FEATURES section")
		}
	}
}

// TestBuildSystemPrompt_AllOptions tests system prompt building with all options
func TestBuildSystemPrompt_AllOptions(t *testing.T) {
	template := "Base prompt"
	opts := GenerateOptions{
		Tone:         ToneTechnical,
		Audience:     AudienceDevelopers,
		Language:     "Spanish",
		IncludeEmoji: true,
		MaxLength:    500,
	}

	result := buildSystemPrompt(template, opts)

	if !strings.Contains(result, "Base prompt") {
		t.Error("Missing base template")
	}
	if !strings.Contains(result, "technical") {
		t.Error("Missing tone instruction")
	}
	if !strings.Contains(result, "software developers") {
		t.Error("Missing audience instruction")
	}
	if !strings.Contains(result, "Spanish") {
		t.Error("Missing language instruction")
	}
	if !strings.Contains(result, "Include relevant emojis") {
		t.Error("Missing emoji instruction")
	}
	if !strings.Contains(result, "500") {
		t.Error("Missing max length instruction")
	}
}

// TestBuildUserPrompt_WithContext tests user prompt building with context
func TestBuildUserPrompt_WithContext(t *testing.T) {
	template := "{{PRODUCT_NAME}}: {{CONTENT}}"
	opts := GenerateOptions{
		ProductName: "TestApp",
		Context:     "Additional context here",
	}

	result := buildUserPrompt(template, "test content", opts)

	if !strings.Contains(result, "TestApp") {
		t.Error("Missing product name")
	}
	if !strings.Contains(result, "test content") {
		t.Error("Missing content")
	}
	if !strings.Contains(result, "Additional context here") {
		t.Error("Missing additional context")
	}
}
