package container

import (
	"strings"
	"testing"
)

func TestRenderStructuredReleaseNotes_HappyPath(t *testing.T) {
	payload := []byte(`{
		"summary": "v1.4.1 fixes a webhook race and a token-refresh nil-ptr.",
		"sections": [
			{"category": "fixes", "items": ["nil pointer in token refresh", "webhook delivery race"]},
			{"category": "performance", "items": ["bumped semaphore from 5 to 10"]}
		],
		"breaking_changes": ["renamed Approve to ApproveRelease"],
		"upgrade_notes": "Re-run 'go install' to pick up the new SDK."
	}`)

	out, err := renderStructuredReleaseNotes(payload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, marker := range []string{
		"v1.4.1 fixes",
		"## Fixes",
		"- nil pointer in token refresh",
		"## Performance",
		"## Breaking Changes",
		"- renamed Approve to ApproveRelease",
		"## Upgrade Notes",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("missing marker %q in:\n%s", marker, out)
		}
	}
}

func TestRenderStructuredReleaseNotes_EmptyPayload(t *testing.T) {
	if _, err := renderStructuredReleaseNotes([]byte(`{}`)); err == nil {
		t.Error("expected error on empty payload")
	}
}

func TestRenderStructuredReleaseNotes_GarbageReturnsError(t *testing.T) {
	if _, err := renderStructuredReleaseNotes([]byte(`{not-json`)); err == nil {
		t.Error("expected JSON error")
	}
}

func TestRenderStructuredReleaseNotes_SkipsEmptySections(t *testing.T) {
	payload := []byte(`{
		"summary": "x",
		"sections": [
			{"category": "features", "items": []},
			{"category": "fixes",    "items": ["one fix"]}
		]
	}`)
	out, err := renderStructuredReleaseNotes(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Features") {
		t.Errorf("empty Features section should be skipped; got:\n%s", out)
	}
	if !strings.Contains(out, "Fixes") {
		t.Errorf("non-empty Fixes section should appear; got:\n%s", out)
	}
}

func TestCapitalizeCategory(t *testing.T) {
	cases := map[string]string{
		"features": "Features",
		"breaking": "Breaking",
		"":         "",
	}
	for in, want := range cases {
		if got := capitalizeCategory(in); got != want {
			t.Errorf("capitalizeCategory(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReleaseNotesSchemaShim_StructuralCompat(t *testing.T) {
	s := releaseNotesSchemaShim{}
	if s.Name() != "ReleaseNotes" {
		t.Errorf("name: %q", s.Name())
	}
	if s.Strict() {
		t.Error("ReleaseNotes is cosmetic prose; strict should be false")
	}
	b, err := s.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "summary") {
		t.Errorf("schema JSON missing summary property: %s", b)
	}
}
