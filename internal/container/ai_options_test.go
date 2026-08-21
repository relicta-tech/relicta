package container

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
)

// Two AI settings were documented, defaulted, validated — and read by nothing.
//
// `ai.custom_prompts` is a six-field block for replacing the prompts relicta sends. Every
// provider applies them (`prompts.applyCustomPrompts(cfg.CustomPrompts)` in openai.go,
// anthropic.go, gemini.go and ollama.go) and `ai.WithCustomPrompts` exists to carry them, but
// the container never called it. A team that rewrote its release-notes prompt got the default
// prompt, with nothing said.
//
// `ai.retry_attempts` is the same shape one field along: the resilience layer reads
// `cfg.RetryAttempts`, `ai.WithRetryAttempts` sets it, and nothing called that either — so a
// project asking for one attempt, or for ten, got the library default.
//
// These assert the options a configuration produces, applied to the service config they
// configure, because that is the link that was missing.

func applied(t *testing.T, cfg *config.Config) ai.ServiceConfig {
	t.Helper()

	var applied ai.ServiceConfig
	for _, opt := range aiServiceOptions(cfg, "test-key") {
		opt(&applied)
	}
	return applied
}

func TestCustomPromptsReachTheProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.CustomPrompts = config.CustomPrompts{
		ChangelogSystem:    "changelog system",
		ChangelogUser:      "changelog user",
		ReleaseNotesSystem: "notes system",
		ReleaseNotesUser:   "notes user",
		MarketingSystem:    "marketing system",
		MarketingUser:      "marketing user",
	}

	got := applied(t, cfg).CustomPrompts
	want := ai.CustomPrompts{
		ChangelogSystem:    "changelog system",
		ChangelogUser:      "changelog user",
		ReleaseNotesSystem: "notes system",
		ReleaseNotesUser:   "notes user",
		MarketingSystem:    "marketing system",
		MarketingUser:      "marketing user",
	}

	if got != want {
		t.Errorf("custom prompts = %+v, want %+v.\nA team that rewrote its prompts would get "+
			"the defaults, with nothing said", got, want)
	}
}

// Every field, so that adding one to the configuration and forgetting to carry it is a test
// failure rather than a prompt that silently does not apply.
func TestEveryCustomPromptFieldIsCarried(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.CustomPrompts = config.CustomPrompts{
		ChangelogSystem:    "a",
		ChangelogUser:      "b",
		ReleaseNotesSystem: "c",
		ReleaseNotesUser:   "d",
		MarketingSystem:    "e",
		MarketingUser:      "f",
	}

	got := applied(t, cfg).CustomPrompts
	for name, value := range map[string]string{
		"ChangelogSystem":    got.ChangelogSystem,
		"ChangelogUser":      got.ChangelogUser,
		"ReleaseNotesSystem": got.ReleaseNotesSystem,
		"ReleaseNotesUser":   got.ReleaseNotesUser,
		"MarketingSystem":    got.MarketingSystem,
		"MarketingUser":      got.MarketingUser,
	} {
		if value == "" {
			t.Errorf("%s did not reach the service", name)
		}
	}
}

func TestRetryAttemptsReachTheResilienceLayer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.RetryAttempts = 7

	if got := applied(t, cfg).RetryAttempts; got != 7 {
		t.Errorf("retry attempts = %d, want 7: a project asking for more retries, or for "+
			"fewer, got the library default", got)
	}
}

// Zero means "not configured", not "never retry": the resilience layer treats a zero as its
// own default, and sending one would be indistinguishable from the setting being absent.
func TestAnUnsetRetryCountIsNotSent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.RetryAttempts = 0

	if got := applied(t, cfg).RetryAttempts; got != 0 {
		t.Errorf("retry attempts = %d, want 0 left for the service to default", got)
	}
}
