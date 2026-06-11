//go:build relicta_gemini || relicta_all_ai || !relicta_minimal

// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/relicta-tech/relicta/v4/internal/errors"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai/schemas"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

func init() {
	// Register Gemini provider
	RegisterProvider("gemini", NewGeminiService)
}

// Default Gemini configuration values.
const (
	// DefaultGeminiModel is the default model for Gemini.
	// 2.5 Flash is the 2026 production-stable default; 2.0 Flash Exp was a
	// preview tag retired by Q1 2026.
	DefaultGeminiModel = "gemini-2.5-flash"

	// GeminiModelHighStakes is recommended for governance-sensitive prose.
	GeminiModelHighStakes = "gemini-2.5-pro"
)

// Pre-compiled regex for API key validation.
// Gemini keys start with "AIza" followed by alphanumeric, hyphen, or underscore characters.
var geminiKeyPattern = regexp.MustCompile(`^AIza[a-zA-Z0-9_-]{35,}$`)

// geminiService implements the AI Service interface using Google Gemini.
type geminiService struct {
	client     *genai.Client
	config     ServiceConfig
	prompts    promptTemplates
	resilience *Resilience
}

// NewGeminiService creates a new Gemini-based AI service.
func NewGeminiService(cfg ServiceConfig) (Service, error) {
	if cfg.APIKey == "" {
		return &noopService{}, nil
	}

	// Validate API key format to fail fast and avoid leaking invalid keys in error messages
	if !geminiKeyPattern.MatchString(cfg.APIKey) {
		return nil, errors.AI("NewGeminiService", "invalid Gemini API key format (expected AIza...)")
	}

	// Set default model if not provided
	model := cfg.Model
	if model == "" {
		model = DefaultGeminiModel
	}
	cfg.Model = model

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		// Use AIWrapSafe to redact any API keys that might appear in SDK error messages
		return nil, errors.AIWrapSafe(err, "NewGeminiService", "failed to create Gemini client")
	}

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

	svc := &geminiService{
		client:     client,
		config:     cfg,
		resilience: NewResilience(resilienceCfg),
		prompts:    prompts,
	}

	return svc, nil
}

// GenerateChangelog generates a changelog from commits using Gemini.
func (s *geminiService) GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.changelogUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.changelogSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateReleaseNotes generates release notes from a changelog using Gemini.
func (s *geminiService) GenerateReleaseNotes(ctx context.Context, changelog string, opts GenerateOptions) (string, error) {
	if changelog == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.releaseNotesUser, changelog, opts)
	systemPrompt := buildSystemPrompt(s.prompts.releaseNotesSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// GenerateMarketingBlurb generates a marketing blurb from release notes using Gemini.
func (s *geminiService) GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts GenerateOptions) (string, error) {
	if releaseNotes == "" {
		return "", nil
	}

	userPrompt := buildUserPrompt(s.prompts.marketingUser, releaseNotes, opts)
	systemPrompt := buildSystemPrompt(s.prompts.marketingSystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// SummarizeChanges generates a summary of changes using Gemini.
func (s *geminiService) SummarizeChanges(ctx context.Context, changes *git.CategorizedChanges, opts GenerateOptions) (string, error) {
	if changes == nil || changes.TotalCount() == 0 {
		return "", nil
	}

	changesText := formatChangesForPrompt(changes)
	userPrompt := buildUserPrompt(s.prompts.summaryUser, changesText, opts)
	systemPrompt := buildSystemPrompt(s.prompts.summarySystem, opts)

	return s.complete(ctx, systemPrompt, userPrompt)
}

// Complete generates a raw completion from prompts.
func (s *geminiService) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return s.complete(ctx, systemPrompt, userPrompt)
}

// CompleteStructured forces Gemini to return JSON conforming to the supplied
// schema using `responseMimeType=application/json` plus `responseJsonSchema`.
//
// Cross-provider parity with OpenAI's `response_format: json_schema` and
// Anthropic's tool-use approach. Returns the raw JSON bytes — callers
// unmarshal into the matching Go struct.
func (s *geminiService) CompleteStructured(ctx context.Context, systemPrompt, userPrompt string, schema schemas.Schema) ([]byte, error) {
	if schema == nil {
		return nil, errors.AI("CompleteStructured", "schema is required")
	}

	schemaJSON, err := schema.MarshalJSON()
	if err != nil {
		return nil, errors.AIWrapSafe(err, "CompleteStructured", "marshal schema")
	}

	// Decode into a generic any so the genai SDK can re-marshal as JSON
	// inside the API payload. responseJsonSchema accepts arbitrary JSON
	// Schema 2020-12; responseSchema is the older, OpenAPI-3.0-shaped variant.
	var schemaAny any
	if err := json.Unmarshal(schemaJSON, &schemaAny); err != nil {
		return nil, errors.AIWrapSafe(err, "CompleteStructured", "decode schema")
	}

	result, err := s.resilience.Execute(ctx, func(ctx context.Context) (string, error) {
		fullPrompt := systemPrompt + "\n\n" + userPrompt
		parts := []*genai.Part{{Text: fullPrompt}}
		temperature := float32(s.config.Temperature)

		resp, err := s.client.Models.GenerateContent(
			ctx,
			s.config.Model,
			[]*genai.Content{{Parts: parts}},
			&genai.GenerateContentConfig{
				Temperature:        &temperature,
				MaxOutputTokens:    int32(s.config.MaxTokens), // #nosec G115 -- bounded by config
				ResponseMIMEType:   "application/json",
				ResponseJsonSchema: schemaAny,
			},
		)
		if err != nil {
			return "", err
		}

		if len(resp.Candidates) == 0 {
			return "", errors.AI("CompleteStructured", "no response from Gemini model")
		}
		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return "", errors.AI("CompleteStructured", "empty response from Gemini model")
		}

		var b strings.Builder
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				b.WriteString(part.Text)
			}
		}
		out := strings.TrimSpace(b.String())
		if out == "" {
			return "", errors.AI("CompleteStructured", "no text in response from Gemini model")
		}
		return out, nil
	})

	if err != nil {
		return nil, errors.AIWrapSafe(err, "CompleteStructured", "failed to generate structured content")
	}
	return []byte(result), nil
}

// IsAvailable returns true if the Gemini service is available.
func (s *geminiService) IsAvailable() bool {
	return s.client != nil && s.config.APIKey != ""
}

// complete sends a completion request to Gemini using Fortify resilience patterns.
func (s *geminiService) complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := s.resilience.Execute(ctx, func(ctx context.Context) (string, error) {
		// Combine system and user prompts - Gemini uses a single prompt format
		fullPrompt := systemPrompt + "\n\n" + userPrompt

		// Create content parts
		parts := []*genai.Part{
			{Text: fullPrompt},
		}

		// Create generation config with pointer to temperature
		temperature := float32(s.config.Temperature)

		// Generate content
		resp, err := s.client.Models.GenerateContent(
			ctx,
			s.config.Model,
			[]*genai.Content{{Parts: parts}},
			&genai.GenerateContentConfig{
				Temperature:     &temperature,
				MaxOutputTokens: int32(s.config.MaxTokens), // #nosec G115 -- MaxTokens is bounded by config
			},
		)
		if err != nil {
			return "", err
		}

		// Extract text from response
		if len(resp.Candidates) == 0 {
			return "", errors.AI("complete", "no response from Gemini model")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return "", errors.AI("complete", "empty response from Gemini model")
		}

		// Get the text from all parts
		var resultText strings.Builder
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				resultText.WriteString(part.Text)
			}
		}

		if resultText.Len() == 0 {
			return "", errors.AI("complete", "no text in response from Gemini model")
		}

		return strings.TrimSpace(resultText.String()), nil
	})

	if err != nil {
		return "", errors.AIWrapSafe(err, "complete", "failed to generate content")
	}

	return result, nil
}
