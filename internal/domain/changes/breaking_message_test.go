package changes

import "testing"

// The release analyzer passed the literal string "breaking change" for every breaking commit,
// so a changelog's BREAKING CHANGES section read "- breaking change" however carefully the
// author had written the footer. The parser already recognized the footer; it just never saw
// the full message, because the analyzer hands it a one-line subject.

func TestTheBreakingFooterIsExtracted(t *testing.T) {
	message := "feat(auth): rotate signing keys\n\n" +
		"Keys now rotate hourly.\n\n" +
		"BREAKING CHANGE: tokens issued before this release stop validating\n"

	got := BreakingMessageFromMessage(message)
	if got != "tokens issued before this release stop validating" {
		t.Errorf("BreakingMessageFromMessage() = %q, want the footer text.\nWithout it the "+
			"changelog says \"breaking change\" and the author's explanation is lost", got)
	}
}

func TestTheHyphenatedSpellingIsAcceptedToo(t *testing.T) {
	if got := BreakingMessageFromMessage("fix: x\n\nBREAKING-CHANGE: the flag is gone"); got != "the flag is gone" {
		t.Errorf("BreakingMessageFromMessage() = %q, want the footer text", got)
	}
}

// The `feat!:` form marks a break without describing one. Empty is the honest answer, and it
// tells the renderer to fall back to the subject rather than print a placeholder.
func TestAMarkerWithoutAFooterYieldsNothing(t *testing.T) {
	if got := BreakingMessageFromMessage("feat!: drop the v1 API"); got != "" {
		t.Errorf("BreakingMessageFromMessage() = %q, want empty so the renderer uses the "+
			"subject instead", got)
	}
}
