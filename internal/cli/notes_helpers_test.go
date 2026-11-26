package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/relicta-tech/relicta/v5/internal/config"
	releaseapp "github.com/relicta-tech/relicta/v5/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
)

func TestPrintNotesNextSteps(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printNotesNextSteps()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	result := buf.String()

	assert.Contains(t, result, "Next Steps")
	assert.Contains(t, result, "relicta approve")
	assert.Contains(t, result, "relicta publish")
}

func TestBuildNotesInputForServices(t *testing.T) {
	// Save and restore global config
	origCfg := cfg
	defer func() { cfg = origCfg }()
	cfg = config.DefaultConfig()

	// AI notes follow ai.enabled in config now; the --ai flag is gone (ADR-009,
	// ADR-010). The three cases below are the same three as before, driven by the
	// setting that replaced the flag.
	t.Run("AI enabled in config", func(t *testing.T) {
		oldAudience, oldTone := notesAudience, notesTone
		defer func() { notesAudience, notesTone = oldAudience, oldTone }()

		cfg.AI.Enabled = true
		notesAudience = "technical"
		notesTone = "professional"

		input := buildNotesInputForServices("/test/repo", true)

		assert.Equal(t, "/test/repo", input.RepoRoot)
		assert.True(t, input.Options.UseAI)
		assert.Equal(t, "technical", input.Options.AudiencePreset)
		assert.Equal(t, "professional", input.Options.TonePreset)
		assert.Equal(t, domain.ActorType("user"), input.Actor.Type)
		assert.Equal(t, "cli", input.Actor.ID)
	})

	t.Run("AI disabled in config", func(t *testing.T) {
		cfg.AI.Enabled = false

		input := buildNotesInputForServices("/test/repo", true)
		assert.False(t, input.Options.UseAI)
	})

	// Configuration asking for AI cannot conjure a provider: without one the
	// deterministic path is used rather than failing.
	t.Run("AI enabled but no provider available", func(t *testing.T) {
		cfg.AI.Enabled = true

		input := buildNotesInputForServices("/test/repo", false) // hasAI = false
		assert.False(t, input.Options.UseAI)
	})
}

func TestNotesSourceLabel(t *testing.T) {
	tests := map[string]string{
		"":                   "deterministic template",
		"basic":              "deterministic template",
		"agent":              "calling agent draft",
		"changelog-fallback": "AI changelog only; release-note prose unavailable",
		"openai":             "AI-generated (openai)",
	}
	for provider, want := range tests {
		t.Run(provider, func(t *testing.T) {
			if got := notesSourceLabel(provider); got != want {
				t.Errorf("notesSourceLabel(%q) = %q, want %q", provider, got, want)
			}
		})
	}
}

func TestOutputNotesJSONFromServices(t *testing.T) {
	oldStdout, oldJSON := os.Stdout, outputJSON
	oldCfg := cfg
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	outputJSON = true
	t.Cleanup(func() {
		os.Stdout = oldStdout
		outputJSON = oldJSON
		cfg = oldCfg
	})

	cfg = config.DefaultConfig()
	cfg.AI.Enabled = true
	output := &releaseapp.GenerateNotesOutput{
		RunID: "run-1", InputsHash: "hash-1",
		Notes: &domain.ReleaseNotes{
			Text: "A customer friendly summary", Provider: "changelog-fallback",
			TonePreset: "friendly", AudiencePreset: "users", Model: "model-1",
		},
	}
	if err := outputNotesJSONFromServices(context.Background(), output, ".", commandTestApp{}); err != nil {
		t.Fatalf("outputNotesJSONFromServices: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout

	var got map[string]any
	if err := json.NewDecoder(r).Decode(&got); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if got["release_id"] != "run-1" || got["inputs_hash"] != "hash-1" || got["provider"] != "changelog-fallback" {
		t.Errorf("missing release note metadata: %#v", got)
	}
	if got["ai_generated"] != true || got["warning"] == nil || got["release_notes"] != "A customer friendly summary" {
		t.Errorf("unexpected fallback warning/output: %#v", got)
	}
}
