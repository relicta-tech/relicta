// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/felixgeelhaar/release-pilot/internal/errors"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// Pre-compiled regex for API key validation.
// OpenAI keys start with "sk-" followed by alphanumeric characters.
// Also supports newer project-scoped keys (sk-proj-) and service account keys.
var openaiKeyPattern = regexp.MustCompile(`^sk-(?:proj-)?[a-zA-Z0-9_-]{20,}$`)

// openAIService implements the AI Service interface using OpenAI.
type openAIService struct {
	client     *openai.Client
	config     ServiceConfig
	prompts    promptTemplates
	resilience *Resilience
}

// NewOpenAIService creates a new OpenAI-based AI service.
func NewOpenAIService(cfg ServiceConfig) (Service, error) {
	if cfg.APIKey == "" {
		return &noopService{}, nil
	}

	// Validate API key format to fail fast and avoid leaking invalid keys in error messages
	if !openaiKeyPattern.MatchString(cfg.APIKey) {
		return nil, errors.AI("NewOpenAIService", "invalid OpenAI API key format")
	}

	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)

	prompts := newDefaultPromptTemplates()
	prompts.applyCustomPrompts(cfg.CustomPrompts)

	// Configure resilience patterns with Fortify
	resilienceCfg := DefaultResilienceConfig()
	resilienceCfg.RateLimitRPM = cfg.RateLimitRPM
	resilienceCfg.RetryAttempts = cfg.RetryAttempts
	if cfg.Timeout > 0 {
		resilienceCfg.RetryMaxWait = cfg.Timeout
	}
	// Use shorter initial wait for API calls (500ms is too long for fast APIs)
	resilienceCfg.RetryInitialWait = 200 * time.Millisecond

	svc := &openAIService{
		client:     client,
		config:     cfg,
		resilience: NewResilience(resilienceCfg),
		prompts:    prompts,
	}

	return svc, nil
}

// GenerateChangelog generates a changelog from commits using AI.
func (s *openAIService) GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.changelogUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.changelogSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateReleaseNotes generates release notes from a changelog using AI.
func (s *openAIService) GenerateReleaseNotes(ctx context.Context, changelog string, opts GenerateOptions) (string, error) {
	if changelog == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.releaseNotesUser, changelog, opts)
	systemPrompt := buildSystemPrompt(s.prompts.releaseNotesSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateMarketingBlurb generates a marketing blurb from release notes using AI.
func (s *openAIService) GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts GenerateOptions) (string, error) {
	if releaseNotes == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.marketingUser, releaseNotes, opts)
	systemPrompt := buildSystemPrompt(s.prompts.marketingSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// SummarizeChanges generates a summary of changes.
func (s *openAIService) SummarizeChanges(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.summaryUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.summarySystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// IsAvailable returns true if the AI service is available.
func (s *openAIService) IsAvailable() bool {
	return s.client != nil && s.config.APIKey != ""
}

// complete sends a completion request to OpenAI using Fortify resilience patterns.
func (s *openAIService) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := s.resilience.Execute(ctx, func(ctx context.Context) (string, error) {
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
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.AI("complete", "no response from AI model")
		}

		return strings.TrimSpace(resp.Choices[0].Message.Content), nil
	})

	if err != nil {
		return "", errors.AIWrapSafe(err, "complete", "failed to generate content")
	}

	return result, nil
}

// noopService is a no-op implementation used when AI is not configured.
type noopService struct{}

// GenerateChangelog returns empty string when AI is not available.
func (s *noopService) GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	return "", nil
}

// GenerateReleaseNotes returns empty string when AI is not available.
func (s *noopService) GenerateReleaseNotes(ctx context.Context, changelog string, opts GenerateOptions) (string, error) {
	return "", nil
}

// GenerateMarketingBlurb returns empty string when AI is not available.
func (s *noopService) GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts GenerateOptions) (string, error) {
	return "", nil
}

// SummarizeChanges returns empty string when AI is not available.
func (s *noopService) SummarizeChanges(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	return "", nil
}

// IsAvailable returns false for the noop service.
func (s *noopService) IsAvailable() bool {
	return false
}
