package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
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
