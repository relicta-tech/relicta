// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"regexp"
	"strings"

	"github.com/liushuangls/go-anthropic/v2"

	"github.com/felixgeelhaar/release-pilot/internal/errors"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// Default Anthropic configuration values.
const (
	// DefaultAnthropicModel is the default model for Anthropic.
	DefaultAnthropicModel = "claude-sonnet-4-20250514"
)

// Pre-compiled regex for API key validation.
// Anthropic keys start with "sk-ant-" followed by alphanumeric characters.
var anthropicKeyPattern = regexp.MustCompile(`^sk-ant-[a-zA-Z0-9_-]{20,}$`)

// anthropicService implements the AI Service interface using Anthropic Claude.
type anthropicService struct {
	client      *anthropic.Client
	config      ServiceConfig
	prompts     promptTemplates
	rateLimiter *RateLimiter
}

// NewAnthropicService creates a new Anthropic-based AI service.
func NewAnthropicService(cfg ServiceConfig) (Service, error) {
	if cfg.APIKey == "" {
		return &noopService{}, nil
	}

	// Validate API key format to fail fast and avoid leaking invalid keys in error messages
	if !anthropicKeyPattern.MatchString(cfg.APIKey) {
		return nil, errors.AI("NewAnthropicService", "invalid Anthropic API key format (expected sk-ant-...)")
	}

	// Set default model if not provided
	model := cfg.Model
	if model == "" {
		model = DefaultAnthropicModel
	}
	cfg.Model = model

	// Create Anthropic client
	var clientOptions []anthropic.ClientOption
	if cfg.BaseURL != "" {
		clientOptions = append(clientOptions, anthropic.WithBaseURL(cfg.BaseURL))
	}

	client := anthropic.NewClient(cfg.APIKey, clientOptions...)

	prompts := newDefaultPromptTemplates()
	prompts.applyCustomPrompts(cfg.CustomPrompts)

	svc := &anthropicService{
		client:      client,
		config:      cfg,
		rateLimiter: NewRateLimiter(cfg.RateLimitRPM),
		prompts:     prompts,
	}

	return svc, nil
}

// GenerateChangelog generates a changelog from commits using Anthropic.
func (s *anthropicService) GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.changelogUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.changelogSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateReleaseNotes generates release notes from a changelog using Anthropic.
func (s *anthropicService) GenerateReleaseNotes(ctx context.Context, changelog string, opts GenerateOptions) (string, error) {
	if changelog == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.releaseNotesUser, changelog, opts)
	systemPrompt := buildSystemPrompt(s.prompts.releaseNotesSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateMarketingBlurb generates a marketing blurb from release notes using Anthropic.
func (s *anthropicService) GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts GenerateOptions) (string, error) {
	if releaseNotes == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.marketingUser, releaseNotes, opts)
	systemPrompt := buildSystemPrompt(s.prompts.marketingSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// SummarizeChanges generates a summary of changes using Anthropic.
func (s *anthropicService) SummarizeChanges(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.summaryUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.summarySystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// IsAvailable returns true if the Anthropic service is available.
func (s *anthropicService) IsAvailable() bool {
	return s.client != nil && s.config.APIKey != ""
}

// complete sends a completion request to Anthropic.
func (s *anthropicService) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
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

		resp, err := s.client.CreateMessages(
			ctx,
			anthropic.MessagesRequest{
				Model:     anthropic.Model(s.config.Model),
				MaxTokens: s.config.MaxTokens,
				System:    systemPrompt,
				Messages: []anthropic.Message{
					anthropic.NewUserTextMessage(userPrompt),
				},
				Temperature: toFloatPtr(s.config.Temperature),
			},
		)
		if err != nil {
			lastErr = err
			continue
		}

		if len(resp.Content) == 0 {
			lastErr = errors.AI("complete", "no response from Anthropic model")
			continue
		}

		// Extract text from response using the helper method
		return strings.TrimSpace(resp.GetFirstContentText()), nil
	}

	return "", errors.AIWrapSafe(lastErr, "complete", "failed to generate content after retries")
}

// toFloatPtr converts a float64 to a *float32 for the Anthropic API.
func toFloatPtr(f float64) *float32 {
	f32 := float32(f)
	return &f32
}
