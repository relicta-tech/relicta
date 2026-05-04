// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"context"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai/schemas"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

// Service defines the interface for AI operations.
type Service interface {
	// GenerateChangelog generates a changelog from commits using AI.
	GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error)

	// GenerateReleaseNotes generates release notes from a changelog using AI.
	GenerateReleaseNotes(ctx context.Context, changelog string, opts GenerateOptions) (string, error)

	// GenerateMarketingBlurb generates a marketing blurb from release notes using AI.
	GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts GenerateOptions) (string, error)

	// SummarizeChanges generates a summary of changes.
	SummarizeChanges(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error)

	// Complete generates a raw completion from prompts.
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)

	// IsAvailable returns true if the AI service is available.
	IsAvailable() bool
}

// StructuredOutputService is an opt-in interface implemented by AI services
// that support provider-native structured output (OpenAI response_format,
// Anthropic tool-use, Gemini responseJsonSchema).
//
// Callers type-assert: `if s, ok := svc.(StructuredOutputService); ok`. This
// keeps the core Service interface stable while letting structured-output
// call sites use the cross-provider primitive.
//
// Returns the raw JSON bytes — callers unmarshal into the matching Go struct.
type StructuredOutputService interface {
	CompleteStructured(ctx context.Context, systemPrompt, userPrompt string, schema schemas.Schema) ([]byte, error)
}

// Tone represents the tone of generated content.
type Tone string

const (
	// ToneTechnical produces technical, developer-focused content.
	ToneTechnical Tone = "technical"
	// ToneFriendly produces casual, approachable content.
	ToneFriendly Tone = "friendly"
	// ToneProfessional produces formal, business-like content.
	ToneProfessional Tone = "professional"
	// ToneExcited produces enthusiastic, marketing-style content.
	ToneExcited Tone = "excited"
)

// Audience represents the target audience for generated content.
type Audience string

const (
	// AudienceDevelopers targets technical developers.
	AudienceDevelopers Audience = "developers"
	// AudienceUsers targets end users.
	AudienceUsers Audience = "users"
	// AudiencePublic targets general public.
	AudiencePublic Audience = "public"
	// AudienceMarketing targets marketing teams.
	AudienceMarketing Audience = "marketing"
)

// GenerateOptions configures content generation.
type GenerateOptions struct {
	// Version is the release version.
	Version *version.SemanticVersion
	// ProductName is the product name to use in generated content.
	ProductName string
	// Tone is the tone of the generated content.
	Tone Tone
	// Audience is the target audience.
	Audience Audience
	// MaxLength is the maximum length of generated content (0 = no limit).
	MaxLength int
	// IncludeEmoji includes emojis in the output.
	IncludeEmoji bool
	// Context provides additional context for generation.
	Context string
	// Language is the output language (default: English).
	Language string
}

// DefaultGenerateOptions returns the default generation options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		Tone:         ToneProfessional,
		Audience:     AudienceDevelopers,
		MaxLength:    0,
		IncludeEmoji: false,
		Language:     "English",
	}
}

// ServiceConfig configures the AI service.
type ServiceConfig struct {
	// Provider is the AI provider (openai, anthropic, gemini, ollama, azure-openai).
	Provider string
	// APIKey is the API key for the provider.
	APIKey string
	// BaseURL is the base URL for the API (for custom endpoints).
	BaseURL string
	// APIVersion is the API version (required for Azure OpenAI).
	APIVersion string
	// Model is the model to use.
	Model string
	// MaxTokens is the maximum tokens for responses.
	MaxTokens int
	// Temperature controls randomness (0.0-2.0).
	Temperature float64
	// Timeout is the request timeout.
	Timeout time.Duration
	// RetryAttempts is the number of retry attempts.
	RetryAttempts int
	// RateLimitRPM is the rate limit in requests per minute (0 = no limit).
	RateLimitRPM int
	// CustomPrompts allows custom prompt templates.
	CustomPrompts CustomPrompts
}

// CustomPrompts allows customization of AI prompts.
type CustomPrompts struct {
	// ChangelogSystem is the system prompt for changelog generation.
	ChangelogSystem string
	// ChangelogUser is the user prompt template for changelog generation.
	ChangelogUser string
	// ReleaseNotesSystem is the system prompt for release notes generation.
	ReleaseNotesSystem string
	// ReleaseNotesUser is the user prompt template for release notes generation.
	ReleaseNotesUser string
	// MarketingSystem is the system prompt for marketing blurb generation.
	MarketingSystem string
	// MarketingUser is the user prompt template for marketing blurb generation.
	MarketingUser string
}

// DefaultOpenAIModel is the default OpenAI model. 2026 default: gpt-5.
const (
	DefaultOpenAIModel        = "gpt-5"
	OpenAIModelHighStakes     = "gpt-5"      // governance-sensitive prose
	OpenAIModelFast           = "gpt-5-mini" // fast/cheap for diff summaries
)

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		Provider:      "openai",
		Model:         DefaultOpenAIModel,
		MaxTokens:     2048,
		Temperature:   0.7,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RateLimitRPM:  60, // Default to 60 requests per minute
	}
}

// ServiceOption configures the AI service.
type ServiceOption func(*ServiceConfig)

// WithProvider sets the AI provider.
func WithProvider(provider string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.Provider = provider
	}
}

// WithAPIKey sets the API key.
func WithAPIKey(key string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.APIKey = key
	}
}

// WithBaseURL sets the base URL.
func WithBaseURL(url string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.BaseURL = url
	}
}

// WithAPIVersion sets the API version (required for Azure OpenAI).
func WithAPIVersion(version string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.APIVersion = version
	}
}

// WithModel sets the model.
func WithModel(model string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.Model = model
	}
}

// WithMaxTokens sets the maximum tokens.
func WithMaxTokens(tokens int) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.MaxTokens = tokens
	}
}

// WithTemperature sets the temperature.
func WithTemperature(temp float64) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.Temperature = temp
	}
}

// WithTimeout sets the timeout.
func WithTimeout(timeout time.Duration) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.Timeout = timeout
	}
}

// WithRetryAttempts sets the retry attempts.
func WithRetryAttempts(attempts int) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.RetryAttempts = attempts
	}
}

// WithCustomPrompts sets custom prompt templates.
func WithCustomPrompts(prompts CustomPrompts) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.CustomPrompts = prompts
	}
}

// WithRateLimit sets the rate limit in requests per minute.
func WithRateLimit(rpm int) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.RateLimitRPM = rpm
	}
}

// NewService creates a new AI service based on the configuration.
// Uses the provider registry to allow conditional compilation of AI providers
// via build tags (e.g., -tags=relicta_openai for OpenAI-only builds).
func NewService(opts ...ServiceOption) (Service, error) {
	cfg := DefaultServiceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Look up provider in registry (populated by init() functions when providers are compiled in)
	factory := GetProvider(cfg.Provider)
	if factory != nil {
		return factory(cfg)
	}

	// Fallback: try default provider (openai) if the requested one isn't available
	if cfg.Provider != "openai" {
		factory = GetProvider("openai")
		if factory != nil {
			return factory(cfg)
		}
	}

	// No providers available
	return nil, ProviderNotAvailableError{Provider: cfg.Provider}
}
