package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
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

	t.Run("with AI enabled", func(t *testing.T) {
		// Save and restore global vars
		oldNotesUseAI := notesUseAI
		oldNotesAudience := notesAudience
		oldNotesTone := notesTone
		defer func() {
			notesUseAI = oldNotesUseAI
			notesAudience = oldNotesAudience
			notesTone = oldNotesTone
		}()

		notesUseAI = true
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

	t.Run("with AI disabled by flag", func(t *testing.T) {
		oldNotesUseAI := notesUseAI
		defer func() {
			notesUseAI = oldNotesUseAI
		}()

		notesUseAI = false

		input := buildNotesInputForServices("/test/repo", true)
		assert.False(t, input.Options.UseAI)
	})

	t.Run("with AI disabled by capability", func(t *testing.T) {
		oldNotesUseAI := notesUseAI
		defer func() {
			notesUseAI = oldNotesUseAI
		}()

		notesUseAI = true

		input := buildNotesInputForServices("/test/repo", false) // hasAI = false
		assert.False(t, input.Options.UseAI)
	})
}
