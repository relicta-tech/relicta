package ai

import (
	"fmt"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

// ============================================================================
// Service Configuration Benchmarks
// ============================================================================

// BenchmarkServiceConfig_Creation measures service configuration creation overhead.
func BenchmarkServiceConfig_Creation(b *testing.B) {
	b.ReportAllocs()

	b.Run("default_config", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DefaultServiceConfig()
		}
	})

	b.Run("with_options", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cfg := DefaultServiceConfig()
			WithProvider("anthropic")(&cfg)
			WithAPIKey("sk-test-key")(&cfg)
			WithModel("claude-3-opus")(&cfg)
			WithMaxTokens(4096)(&cfg)
			WithTemperature(0.5)(&cfg)
			WithTimeout(60 * time.Second)(&cfg)
			WithRetryAttempts(5)(&cfg)
			WithRateLimit(120)(&cfg)
		}
	})

	b.Run("with_custom_prompts", func(b *testing.B) {
		customPrompts := CustomPrompts{
			ChangelogSystem:    "You are a changelog generator.",
			ChangelogUser:      "Generate changelog for: {{CONTENT}}",
			ReleaseNotesSystem: "You are a release notes writer.",
			ReleaseNotesUser:   "Write notes for: {{CONTENT}}",
		}
		for i := 0; i < b.N; i++ {
			cfg := DefaultServiceConfig()
			WithCustomPrompts(customPrompts)(&cfg)
		}
	})
}

// BenchmarkGenerateOptions_Creation measures generation options creation overhead.
func BenchmarkGenerateOptions_Creation(b *testing.B) {
	b.ReportAllocs()

	b.Run("default_options", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DefaultGenerateOptions()
		}
	})

	b.Run("full_options", func(b *testing.B) {
		ver, _ := version.Parse("1.2.3")
		for i := 0; i < b.N; i++ {
			opts := GenerateOptions{
				Version:      &ver,
				ProductName:  "Relicta",
				Tone:         ToneExcited,
				Audience:     AudienceMarketing,
				MaxLength:    500,
				IncludeEmoji: true,
				Context:      "This is a major release with breaking changes.",
				Language:     "English",
			}
			_ = opts
		}
	})
}

// ============================================================================
// Provider Registry Benchmarks
// ============================================================================

// BenchmarkProviderRegistry_Lookup measures provider lookup overhead.
func BenchmarkProviderRegistry_Lookup(b *testing.B) {
	b.ReportAllocs()

	b.Run("existing_provider", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetProvider("openai")
		}
	})

	b.Run("non_existing_provider", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetProvider("nonexistent")
		}
	})

	b.Run("is_available", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = IsProviderAvailable("openai")
		}
	})

	b.Run("list_providers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ListProviders()
		}
	})
}

// BenchmarkProviderRegistry_ConcurrentLookup measures concurrent provider lookups.
func BenchmarkProviderRegistry_ConcurrentLookup(b *testing.B) {
	b.ReportAllocs()

	providers := []string{"openai", "anthropic", "gemini", "ollama"}

	b.Run("parallel_lookup", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_ = GetProvider(providers[i%len(providers)])
				i++
			}
		})
	})
}

// ============================================================================
// Resilience Configuration Benchmarks
// ============================================================================

// BenchmarkResilience_Creation measures resilience wrapper creation overhead.
func BenchmarkResilience_Creation(b *testing.B) {
	b.ReportAllocs()

	b.Run("default_config", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cfg := DefaultResilienceConfig()
			r := NewResilience(cfg)
			_ = r.Close()
		}
	})

	b.Run("rate_limit_only", func(b *testing.B) {
		cfg := ResilienceConfig{
			RateLimitRPM:          120,
			CircuitBreakerEnabled: false,
		}
		for i := 0; i < b.N; i++ {
			r := NewResilience(cfg)
			_ = r.Close()
		}
	})

	b.Run("retry_only", func(b *testing.B) {
		cfg := ResilienceConfig{
			RetryAttempts:         5,
			RetryInitialWait:      100 * time.Millisecond,
			RetryMaxWait:          5 * time.Second,
			CircuitBreakerEnabled: false,
		}
		for i := 0; i < b.N; i++ {
			r := NewResilience(cfg)
			_ = r.Close()
		}
	})

	b.Run("circuit_breaker_only", func(b *testing.B) {
		cfg := ResilienceConfig{
			CircuitBreakerEnabled:     true,
			CircuitBreakerThreshold:   5,
			CircuitBreakerTimeout:     30 * time.Second,
			CircuitBreakerMaxRequests: 3,
		}
		for i := 0; i < b.N; i++ {
			r := NewResilience(cfg)
			_ = r.Close()
		}
	})

	b.Run("full_resilience", func(b *testing.B) {
		cfg := ResilienceConfig{
			RateLimitRPM:              60,
			RetryAttempts:             3,
			RetryInitialWait:          500 * time.Millisecond,
			RetryMaxWait:              10 * time.Second,
			CircuitBreakerEnabled:     true,
			CircuitBreakerThreshold:   5,
			CircuitBreakerTimeout:     30 * time.Second,
			CircuitBreakerMaxRequests: 3,
		}
		for i := 0; i < b.N; i++ {
			r := NewResilience(cfg)
			_ = r.Close()
		}
	})
}

// BenchmarkResilience_StateChecks measures resilience state check overhead.
func BenchmarkResilience_StateChecks(b *testing.B) {
	b.ReportAllocs()

	cfg := DefaultResilienceConfig()
	r := NewResilience(cfg)
	defer func() { _ = r.Close() }()

	b.Run("circuit_breaker_state", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = r.CircuitBreakerState()
		}
	})

	b.Run("rate_limit_available", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = r.RateLimitAvailable()
		}
	})
}

// BenchmarkResilience_RetryableError measures error classification overhead.
func BenchmarkResilience_RetryableError(b *testing.B) {
	b.ReportAllocs()

	errors := []error{
		fmt.Errorf("rate limit exceeded"),
		fmt.Errorf("500 internal server error"),
		fmt.Errorf("connection timeout"),
		fmt.Errorf("401 unauthorized"),
		fmt.Errorf("400 bad request"),
		nil,
	}

	b.Run("classify_errors", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = isRetryableError(errors[i%len(errors)])
		}
	})
}

// ============================================================================
// Prompt Building Benchmarks
// ============================================================================

// BenchmarkPrompt_Building measures prompt construction overhead.
func BenchmarkPrompt_Building(b *testing.B) {
	b.ReportAllocs()

	templates := newDefaultPromptTemplates()
	ver, _ := version.Parse("2.0.0")

	b.Run("system_prompt_minimal", func(b *testing.B) {
		opts := DefaultGenerateOptions()
		for i := 0; i < b.N; i++ {
			_ = buildSystemPrompt(templates.changelogSystem, opts)
		}
	})

	b.Run("system_prompt_full", func(b *testing.B) {
		opts := GenerateOptions{
			Tone:         ToneExcited,
			Audience:     AudienceMarketing,
			Language:     "Spanish",
			IncludeEmoji: true,
			MaxLength:    1000,
		}
		for i := 0; i < b.N; i++ {
			_ = buildSystemPrompt(templates.changelogSystem, opts)
		}
	})

	b.Run("user_prompt_minimal", func(b *testing.B) {
		opts := DefaultGenerateOptions()
		content := "Test content"
		for i := 0; i < b.N; i++ {
			_ = buildUserPrompt(templates.changelogUser, content, opts)
		}
	})

	b.Run("user_prompt_full", func(b *testing.B) {
		opts := GenerateOptions{
			Version:     &ver,
			ProductName: "Relicta",
			Context:     "This is a major release with significant improvements.",
		}
		content := "Breaking: API changes\nFeature: New dashboard\nFix: Memory leak"
		for i := 0; i < b.N; i++ {
			_ = buildUserPrompt(templates.changelogUser, content, opts)
		}
	})
}

// BenchmarkPrompt_ToneInstruction measures tone instruction generation overhead.
func BenchmarkPrompt_ToneInstruction(b *testing.B) {
	b.ReportAllocs()

	tones := []Tone{ToneTechnical, ToneFriendly, ToneProfessional, ToneExcited}

	b.Run("all_tones", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = getToneInstruction(tones[i%len(tones)])
		}
	})
}

// BenchmarkPrompt_AudienceInstruction measures audience instruction generation overhead.
func BenchmarkPrompt_AudienceInstruction(b *testing.B) {
	b.ReportAllocs()

	audiences := []Audience{AudienceDevelopers, AudienceUsers, AudiencePublic, AudienceMarketing}

	b.Run("all_audiences", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = getAudienceInstruction(audiences[i%len(audiences)])
		}
	})
}

// ============================================================================
// Change Formatting Benchmarks
// ============================================================================

// BenchmarkFormatChanges measures change formatting overhead.
func BenchmarkFormatChanges(b *testing.B) {
	b.ReportAllocs()

	createCommit := func(typ git.CommitType, scope, desc string, breaking bool) git.ConventionalCommit {
		return git.ConventionalCommit{
			Type:        typ,
			Scope:       scope,
			Description: desc,
			Breaking:    breaking,
		}
	}

	b.Run("small_changeset", func(b *testing.B) {
		changes := &git.CategorizedChanges{
			Features: []git.ConventionalCommit{
				createCommit(git.CommitTypeFeat, "api", "Add new endpoint", false),
			},
			Fixes: []git.ConventionalCommit{
				createCommit(git.CommitTypeFix, "auth", "Fix token validation", false),
			},
		}
		for i := 0; i < b.N; i++ {
			_ = formatChangesForPrompt(changes)
		}
	})

	b.Run("medium_changeset", func(b *testing.B) {
		changes := &git.CategorizedChanges{
			Breaking: []git.ConventionalCommit{
				createCommit(git.CommitTypeFeat, "api", "Remove deprecated endpoints", true),
			},
			Features: make([]git.ConventionalCommit, 5),
			Fixes:    make([]git.ConventionalCommit, 5),
			Other:    make([]git.ConventionalCommit, 5),
		}
		for i := 0; i < 5; i++ {
			changes.Features[i] = createCommit(git.CommitTypeFeat, fmt.Sprintf("scope%d", i), fmt.Sprintf("Feature %d", i), false)
			changes.Fixes[i] = createCommit(git.CommitTypeFix, fmt.Sprintf("scope%d", i), fmt.Sprintf("Bug fix %d", i), false)
			changes.Other[i] = createCommit(git.CommitTypeChore, fmt.Sprintf("scope%d", i), fmt.Sprintf("Chore %d", i), false)
		}
		for i := 0; i < b.N; i++ {
			_ = formatChangesForPrompt(changes)
		}
	})

	b.Run("large_changeset", func(b *testing.B) {
		changes := &git.CategorizedChanges{
			Breaking:      make([]git.ConventionalCommit, 3),
			Features:      make([]git.ConventionalCommit, 20),
			Fixes:         make([]git.ConventionalCommit, 15),
			Performance:   make([]git.ConventionalCommit, 5),
			Documentation: make([]git.ConventionalCommit, 10),
			Other:         make([]git.ConventionalCommit, 10),
		}
		for i := 0; i < 3; i++ {
			changes.Breaking[i] = createCommit(git.CommitTypeFeat, "api", fmt.Sprintf("Breaking change %d", i), true)
		}
		for i := 0; i < 20; i++ {
			changes.Features[i] = createCommit(git.CommitTypeFeat, fmt.Sprintf("mod%d", i), fmt.Sprintf("New feature %d with longer description", i), false)
		}
		for i := 0; i < 15; i++ {
			changes.Fixes[i] = createCommit(git.CommitTypeFix, fmt.Sprintf("mod%d", i), fmt.Sprintf("Bug fix %d with details", i), false)
		}
		for i := 0; i < 5; i++ {
			changes.Performance[i] = createCommit(git.CommitTypePerf, fmt.Sprintf("mod%d", i), fmt.Sprintf("Performance improvement %d", i), false)
		}
		for i := 0; i < 10; i++ {
			changes.Documentation[i] = createCommit(git.CommitTypeDocs, fmt.Sprintf("mod%d", i), fmt.Sprintf("Documentation update %d", i), false)
		}
		for i := 0; i < 10; i++ {
			changes.Other[i] = createCommit(git.CommitTypeChore, fmt.Sprintf("mod%d", i), fmt.Sprintf("Maintenance task %d", i), false)
		}
		for i := 0; i < b.N; i++ {
			_ = formatChangesForPrompt(changes)
		}
	})
}

// ============================================================================
// Custom Prompts Benchmarks
// ============================================================================

// BenchmarkCustomPrompts_Apply measures custom prompt application overhead.
func BenchmarkCustomPrompts_Apply(b *testing.B) {
	b.ReportAllocs()

	b.Run("no_custom_prompts", func(b *testing.B) {
		empty := CustomPrompts{}
		for i := 0; i < b.N; i++ {
			templates := newDefaultPromptTemplates()
			templates.applyCustomPrompts(empty)
		}
	})

	b.Run("partial_custom_prompts", func(b *testing.B) {
		partial := CustomPrompts{
			ChangelogSystem: "Custom changelog system prompt",
			ChangelogUser:   "Custom changelog user prompt",
		}
		for i := 0; i < b.N; i++ {
			templates := newDefaultPromptTemplates()
			templates.applyCustomPrompts(partial)
		}
	})

	b.Run("full_custom_prompts", func(b *testing.B) {
		full := CustomPrompts{
			ChangelogSystem:    "Custom changelog system prompt with detailed instructions",
			ChangelogUser:      "Custom changelog user prompt: {{CONTENT}}",
			ReleaseNotesSystem: "Custom release notes system prompt",
			ReleaseNotesUser:   "Custom release notes user prompt: {{CONTENT}}",
			MarketingSystem:    "Custom marketing system prompt",
			MarketingUser:      "Custom marketing user prompt: {{CONTENT}}",
		}
		for i := 0; i < b.N; i++ {
			templates := newDefaultPromptTemplates()
			templates.applyCustomPrompts(full)
		}
	})
}

// ============================================================================
// Scale Benchmarks
// ============================================================================

// BenchmarkFormatChanges_Scale measures change formatting at scale.
func BenchmarkFormatChanges_Scale(b *testing.B) {
	b.ReportAllocs()

	createCommit := func(idx int) git.ConventionalCommit {
		return git.ConventionalCommit{
			Type:        git.CommitTypeFeat,
			Scope:       fmt.Sprintf("module-%d", idx%20),
			Description: fmt.Sprintf("Feature description %d with some additional context about what changed", idx),
			Breaking:    idx%50 == 0,
		}
	}

	b.Run("100_commits", func(b *testing.B) {
		changes := &git.CategorizedChanges{
			Features: make([]git.ConventionalCommit, 100),
		}
		for i := 0; i < 100; i++ {
			changes.Features[i] = createCommit(i)
		}
		for i := 0; i < b.N; i++ {
			_ = formatChangesForPrompt(changes)
		}
	})

	b.Run("500_commits", func(b *testing.B) {
		changes := &git.CategorizedChanges{
			Features: make([]git.ConventionalCommit, 500),
		}
		for i := 0; i < 500; i++ {
			changes.Features[i] = createCommit(i)
		}
		for i := 0; i < b.N; i++ {
			_ = formatChangesForPrompt(changes)
		}
	})
}

// ============================================================================
// Target Validation Benchmark
// ============================================================================

// BenchmarkAIProvider_FullPrepOverhead validates that AI preparation overhead is minimal.
// This measures everything except actual API calls.
func BenchmarkAIProvider_FullPrepOverhead(b *testing.B) {
	b.ReportAllocs()

	createCommit := func(idx int) git.ConventionalCommit {
		return git.ConventionalCommit{
			Type:        git.CommitTypeFeat,
			Scope:       fmt.Sprintf("module-%d", idx%10),
			Description: fmt.Sprintf("Feature %d", idx),
			Breaking:    idx%20 == 0,
		}
	}

	ver, _ := version.Parse("2.0.0")

	b.Run("full_prep_under_10ms", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			start := time.Now()

			// 1. Create service config
			cfg := DefaultServiceConfig()
			WithProvider("openai")(&cfg)
			WithAPIKey("sk-test")(&cfg)
			WithModel("gpt-4")(&cfg)
			WithMaxTokens(4096)(&cfg)

			// 2. Create resilience config
			resCfg := DefaultResilienceConfig()
			r := NewResilience(resCfg)

			// 3. Create generation options
			opts := GenerateOptions{
				Version:      &ver,
				ProductName:  "Relicta",
				Tone:         ToneProfessional,
				Audience:     AudienceDevelopers,
				MaxLength:    2000,
				IncludeEmoji: false,
				Context:      "Major release with new features",
				Language:     "English",
			}

			// 4. Create changes
			changes := &git.CategorizedChanges{
				Breaking: make([]git.ConventionalCommit, 2),
				Features: make([]git.ConventionalCommit, 10),
				Fixes:    make([]git.ConventionalCommit, 5),
			}
			for j := 0; j < 2; j++ {
				changes.Breaking[j] = createCommit(j)
			}
			for j := 0; j < 10; j++ {
				changes.Features[j] = createCommit(j)
			}
			for j := 0; j < 5; j++ {
				changes.Fixes[j] = createCommit(j)
			}

			// 5. Format changes
			content := formatChangesForPrompt(changes)

			// 6. Build prompts
			templates := newDefaultPromptTemplates()
			systemPrompt := buildSystemPrompt(templates.changelogSystem, opts)
			userPrompt := buildUserPrompt(templates.changelogUser, content, opts)

			// Use variables to prevent optimization
			_ = r.CircuitBreakerState()
			_ = systemPrompt
			_ = userPrompt

			_ = r.Close()

			elapsed := time.Since(start)
			if elapsed > 10*time.Millisecond {
				b.Errorf("AI prep overhead took %v, exceeds 10ms target", elapsed)
			}
		}
	})
}

// ============================================================================
// Concurrent Access Benchmarks
// ============================================================================

// BenchmarkResilience_ConcurrentStateChecks measures concurrent state check overhead.
func BenchmarkResilience_ConcurrentStateChecks(b *testing.B) {
	b.ReportAllocs()

	cfg := DefaultResilienceConfig()
	r := NewResilience(cfg)
	defer func() { _ = r.Close() }()

	b.Run("parallel_state_checks", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = r.CircuitBreakerState()
				_ = r.RateLimitAvailable()
			}
		})
	})
}

// BenchmarkPrompt_ConcurrentBuilding measures concurrent prompt building.
func BenchmarkPrompt_ConcurrentBuilding(b *testing.B) {
	b.ReportAllocs()

	templates := newDefaultPromptTemplates()
	ver, _ := version.Parse("1.0.0")

	b.Run("parallel_prompt_building", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				opts := GenerateOptions{
					Version:     &ver,
					ProductName: fmt.Sprintf("Product-%d", i%10),
					Tone:        ToneProfessional,
					Audience:    AudienceDevelopers,
				}
				content := fmt.Sprintf("Change %d", i)
				_ = buildSystemPrompt(templates.changelogSystem, opts)
				_ = buildUserPrompt(templates.changelogUser, content, opts)
				i++
			}
		})
	})
}
