// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/felixgeelhaar/release-pilot/internal/errors"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// Default Ollama configuration values.
const (
	// DefaultOllamaBaseURL is the default Ollama API endpoint.
	DefaultOllamaBaseURL = "http://localhost:11434/v1"
	// DefaultOllamaModel is the default model for Ollama.
	DefaultOllamaModel = "llama3.2"
)

// ollamaService implements the AI Service interface using Ollama's OpenAI-compatible API.
type ollamaService struct {
	client      *openai.Client
	config      ServiceConfig
	prompts     promptTemplates
	rateLimiter *RateLimiter
}

// NewOllamaService creates a new Ollama-based AI service.
// Ollama uses an OpenAI-compatible API, so we can reuse the openai-go client.
func NewOllamaService(cfg ServiceConfig) (Service, error) {
	// Set default base URL if not provided
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultOllamaBaseURL
	}

	// Set default model if not provided
	model := cfg.Model
	if model == "" {
		model = DefaultOllamaModel
	}
	cfg.Model = model

	// Create OpenAI client with Ollama configuration
	// Ollama doesn't require an API key, but the client expects one
	clientConfig := openai.DefaultConfig("ollama")
	clientConfig.BaseURL = baseURL

	// Use custom HTTP client with timeout for local service
	clientConfig.HTTPClient = &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	client := openai.NewClientWithConfig(clientConfig)

	prompts := newDefaultPromptTemplates()
	prompts.applyCustomPrompts(cfg.CustomPrompts)

	svc := &ollamaService{
		client:      client,
		config:      cfg,
		rateLimiter: NewRateLimiter(cfg.RateLimitRPM),
		prompts:     prompts,
	}

	return svc, nil
}

// GenerateChangelog generates a changelog from commits using Ollama.
func (s *ollamaService) GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.changelogUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.changelogSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateReleaseNotes generates release notes from a changelog using Ollama.
func (s *ollamaService) GenerateReleaseNotes(ctx context.Context, changelog string, opts GenerateOptions) (string, error) {
	if changelog == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.releaseNotesUser, changelog, opts)
	systemPrompt := buildSystemPrompt(s.prompts.releaseNotesSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateMarketingBlurb generates a marketing blurb from release notes using Ollama.
func (s *ollamaService) GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts GenerateOptions) (string, error) {
	if releaseNotes == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.marketingUser, releaseNotes, opts)
	systemPrompt := buildSystemPrompt(s.prompts.marketingSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// SummarizeChanges generates a summary of changes using Ollama.
func (s *ollamaService) SummarizeChanges(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.summaryUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.summarySystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// IsAvailable returns true if the Ollama service is available.
func (s *ollamaService) IsAvailable() bool {
	return s.client != nil
}

// CheckConnection verifies that Ollama is running and accessible.
func (s *ollamaService) CheckConnection(ctx context.Context) error {
	// Try to list models as a health check
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURL = DefaultOllamaBaseURL
	}

	// Remove /v1 suffix for the health check endpoint
	healthURL := strings.TrimSuffix(baseURL, "/v1")
	healthURL = strings.TrimSuffix(healthURL, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return errors.AIWrap(err, "CheckConnection", "failed to create health check request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.AI("CheckConnection", fmt.Sprintf("Ollama is not running at %s: %v", healthURL, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.AI("CheckConnection", fmt.Sprintf("Ollama returned status %d", resp.StatusCode))
	}

	return nil
}

// complete sends a completion request to Ollama.
func (s *ollamaService) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= s.config.RetryAttempts; attempt++ {
		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			return "", errors.AIWrap(ctx.Err(), "complete", "context canceled")
		default:
		}

		// Wait for rate limit token before making request
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return "", errors.AIWrap(err, "complete", "rate limiter wait interrupted")
		}

		resp, err := s.client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model: s.config.Model,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: systemPrompt,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: userPrompt,
					},
				},
				MaxTokens:   s.config.MaxTokens,
				Temperature: float32(s.config.Temperature),
			},
		)
		if err != nil {
			lastErr = err
			// For Ollama, check if it's a connection error that might be transient
			if isConnectionError(err) && attempt < s.config.RetryAttempts {
				// Wait before retry with exponential backoff
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				select {
				case <-ctx.Done():
					return "", errors.AIWrap(ctx.Err(), "complete", "context canceled during retry backoff")
				case <-time.After(backoff):
				}
			}
			continue
		}

		if len(resp.Choices) == 0 {
			lastErr = errors.AI("complete", "no response from Ollama model")
			continue
		}

		return strings.TrimSpace(resp.Choices[0].Message.Content), nil
	}

	return "", errors.AIWrapSafe(lastErr, "complete", "failed to generate content after retries (is Ollama running?)")
}

// OllamaConnectionChecker provides methods to check Ollama availability.
type OllamaConnectionChecker interface {
	CheckConnection(ctx context.Context) error
}
