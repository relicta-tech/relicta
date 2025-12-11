// Package ai provides AI-powered content generation for ReleasePilot.
package ai

import (
	"context"
	"regexp"
	"strings"
	"time"

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
	client     *anthropic.Client
	config     ServiceConfig
	prompts    promptTemplates
	resilience *Resilience
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

	// Configure resilience patterns with Fortify
	resilienceCfg := DefaultResilienceConfig()
	resilienceCfg.RateLimitRPM = cfg.RateLimitRPM
	resilienceCfg.RetryAttempts = cfg.RetryAttempts
	if cfg.Timeout > 0 {
		resilienceCfg.RetryMaxWait = cfg.Timeout
	}
	resilienceCfg.RetryInitialWait = 200 * time.Millisecond

	svc := &anthropicService{
		client:     client,
		config:     cfg,
		resilience: NewResilience(resilienceCfg),
		prompts:    prompts,
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

// complete sends a completion request to Anthropic using Fortify resilience patterns.
func (s *anthropicService) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := s.resilience.Execute(ctx, func(ctx context.Context) (string, error) {
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
			return "", err
		}

		if len(resp.Content) == 0 {
			return "", errors.AI("complete", "no response from Anthropic model")
		}

		return strings.TrimSpace(resp.GetFirstContentText()), nil
	})

	if err != nil {
		return "", errors.AIWrapSafe(err, "complete", "failed to generate content")
	}

	return result, nil
}

// toFloatPtr converts a float64 to a *float32 for the Anthropic API.
func toFloatPtr(f float64) *float32 {
	f32 := float32(f)
	return &f32
}
