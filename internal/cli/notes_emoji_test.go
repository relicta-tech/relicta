package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// `ai.include_emoji` and `relicta notes --emoji` were both dead. The prompt templates have
// honored an IncludeEmoji option since they were written, but the port carried no such field,
// the adapter set none, the configuration key had no production reader, and the flag variable
// was registered and read nowhere. A user could ask for emoji two different ways and get none.

func TestTheEmojiSettingReachesTheNotesRequest(t *testing.T) {
	origFlag, origCfg := notesIncludeEmoji, cfg
	t.Cleanup(func() { notesIncludeEmoji, cfg = origFlag, origCfg })
	cfg = config.DefaultConfig()

	notesIncludeEmoji = true
	if got := buildNotesInputForServices("/repo", true); !got.Options.IncludeEmoji {
		t.Error("the notes request does not ask for emoji.\nThe flag and the setting both " +
			"stopped at the CLI, so the prompt never heard about either")
	}

	notesIncludeEmoji = false
	if got := buildNotesInputForServices("/repo", true); got.Options.IncludeEmoji {
		t.Error("the notes request asks for emoji when nothing asked for them")
	}
}
