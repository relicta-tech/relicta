package cli

import "testing"

// Both renderings in `relicta cgp list` and `relicta cgp status` composed "<kind>:<id>"
// unconditionally, and the IDs agents actually send are already qualified — that is the
// convention pkg/cgp uses, and what NewAgentActor and NewCIActor produce. So a proposal from
// "agent:probe" was listed as "agent:agent:probe".
//
// Nothing downstream breaks on it, which is why it survived: it is wrong only on the screen,
// in the command whose entire purpose is letting a person audit what an agent did. Found by
// running the command against a proposal made through the MCP tool rather than by reading the
// formatting code.
func TestActorRenderingDoesNotRepeatTheKind(t *testing.T) {
	for _, tc := range []struct {
		name, kind, id, want string
	}{
		{"already qualified", "agent", "agent:probe", "agent:probe"},
		{"bare identity", "agent", "probe", "agent:probe"},
		{"human, already qualified", "human", "human:alice", "human:alice"},
		{"human, bare", "human", "alice", "human:alice"},
		{"ci", "ci", "github-actions", "ci:github-actions"},

		// A kind-less actor is rendered as whatever identity it has rather than
		// gaining a leading colon.
		{"no kind", "", "probe", "probe"},

		// A different kind is not a prefix match, so it is still qualified — an ID
		// that names another kind is a data problem, and hiding it would be worse
		// than showing it.
		{"mismatched prefix", "human", "agent:probe", "human:agent:probe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := qualifiedActor(tc.kind, tc.id); got != tc.want {
				t.Errorf("qualifiedActor(%q, %q) = %q, want %q", tc.kind, tc.id, got, tc.want)
			}
		})
	}
}
