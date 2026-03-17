// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

func TestOpenAICompleteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "hello"}},
			},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "openai",
		APIKey:        "sk-1234567890abcdef1234567890abcdef",
		BaseURL:       server.URL + "/v1",
		Model:         "gpt-4",
		MaxTokens:     10,
		Temperature:   0.1,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RateLimitRPM:  0,
	}

	svc, err := NewOpenAIService(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIService error: %v", err)
	}

	out, err := svc.Complete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("unexpected completion: %q", out)
	}

	changes := &git.CategorizedChanges{
		Features: []git.ConventionalCommit{
			{Type: "feat", Description: "test"},
		},
	}
	if _, err := svc.GenerateChangelog(context.Background(), changes, DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateChangelog error: %v", err)
	}
	if _, err := svc.SummarizeChanges(context.Background(), changes, DefaultGenerateOptions()); err != nil {
		t.Fatalf("SummarizeChanges error: %v", err)
	}
	if _, err := svc.GenerateReleaseNotes(context.Background(), "changelog", DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateReleaseNotes error: %v", err)
	}
	if _, err := svc.GenerateMarketingBlurb(context.Background(), "notes", DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateMarketingBlurb error: %v", err)
	}
}

func TestAnthropicCompleteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "hello"},
			},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "anthropic",
		APIKey:        "sk-ant-abcdefghijklmnopqrstuvwxyz123456",
		BaseURL:       server.URL,
		Model:         "claude-3",
		MaxTokens:     10,
		Temperature:   0.1,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RateLimitRPM:  0,
	}

	svc, err := NewAnthropicService(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicService error: %v", err)
	}

	out, err := svc.Complete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("unexpected completion: %q", out)
	}

	changes := &git.CategorizedChanges{
		Features: []git.ConventionalCommit{
			{Type: "feat", Description: "test"},
		},
	}
	if _, err := svc.GenerateChangelog(context.Background(), changes, DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateChangelog error: %v", err)
	}
	if _, err := svc.SummarizeChanges(context.Background(), changes, DefaultGenerateOptions()); err != nil {
		t.Fatalf("SummarizeChanges error: %v", err)
	}
	if _, err := svc.GenerateReleaseNotes(context.Background(), "changelog", DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateReleaseNotes error: %v", err)
	}
	if _, err := svc.GenerateMarketingBlurb(context.Background(), "notes", DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateMarketingBlurb error: %v", err)
	}
}

func TestOllamaCompleteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "hello"}},
			},
		})
	}))
	defer server.Close()

	cfg := ServiceConfig{
		Provider:      "ollama",
		BaseURL:       server.URL + "/v1",
		Model:         "llama3.2",
		MaxTokens:     10,
		Temperature:   0.1,
		Timeout:       5 * time.Second,
		RetryAttempts: 1,
		RateLimitRPM:  0,
	}

	svc, err := NewOllamaService(cfg)
	if err != nil {
		t.Fatalf("NewOllamaService error: %v", err)
	}

	out, err := svc.Complete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("unexpected completion: %q", out)
	}

	changes := &git.CategorizedChanges{
		Features: []git.ConventionalCommit{
			{Type: "feat", Description: "test"},
		},
	}
	if _, err := svc.GenerateChangelog(context.Background(), changes, DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateChangelog error: %v", err)
	}
	if _, err := svc.SummarizeChanges(context.Background(), changes, DefaultGenerateOptions()); err != nil {
		t.Fatalf("SummarizeChanges error: %v", err)
	}
	if _, err := svc.GenerateReleaseNotes(context.Background(), "changelog", DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateReleaseNotes error: %v", err)
	}
	if _, err := svc.GenerateMarketingBlurb(context.Background(), "notes", DefaultGenerateOptions()); err != nil {
		t.Fatalf("GenerateMarketingBlurb error: %v", err)
	}
}
