package template

import (
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// ChangelogData.Changes and ReleaseNotesData.Changes are pointers, so nil is
// representable — and the built-in templates dereference them directly
// ({{ if .Changes.HasBreakingChanges }}, {{ range .Changes.Breaking }}). A nil there
// aborted the render with "invalid memory address or nil pointer dereference", which
// reaches a user as a failed release explained by a Go runtime message.
//
// Found because BenchmarkE2E_NotesCommand builds ChangelogData without Changes and had
// been failing on main.

func renderService(t *testing.T) Service {
	t.Helper()
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestChangelogRendersWithoutAChangeSet(t *testing.T) {
	v := version.MustParse("1.2.0")
	prev := version.MustParse("1.1.0")

	out, err := renderService(t).Render("changelog", &ChangelogData{
		Version:         &v,
		PreviousVersion: &prev,
		Date:            time.Unix(1_700_000_000, 0).UTC(),
		RepositoryURL:   "https://github.com/acme/widget",
		// Changes deliberately nil.
	})
	if err != nil {
		t.Fatalf("a nil change set must render an empty changelog, not fail: %v", err)
	}
	if !strings.Contains(out, "1.2.0") {
		t.Errorf("output does not mention the version being released:\n%s", out)
	}
}

func TestReleaseNotesRenderWithoutAChangeSet(t *testing.T) {
	v := version.MustParse("2.0.0")

	if _, err := renderService(t).Render("release-notes", &ReleaseNotesData{
		Version: &v,
		Date:    time.Unix(1_700_000_000, 0).UTC(),
	}); err != nil {
		t.Fatalf("a nil change set must render, not fail: %v", err)
	}
}

// The caller's struct is not the renderer's to modify. A caller that later checks
// whether it supplied changes must still see its own nil.
func TestRenderingDoesNotMutateTheCallersData(t *testing.T) {
	v := version.MustParse("1.0.0")
	data := &ChangelogData{Version: &v, Date: time.Unix(1_700_000_000, 0).UTC()}

	if _, err := renderService(t).Render("changelog", data); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if data.Changes != nil {
		t.Error("the renderer wrote a substituted change set back into the caller's struct")
	}
}

// Real changes must still reach the output — the substitution must not shadow them.
func TestSuppliedChangesStillRender(t *testing.T) {
	v := version.MustParse("1.3.0")

	out, err := renderService(t).Render("changelog", &ChangelogData{
		Version: &v,
		Date:    time.Unix(1_700_000_000, 0).UTC(),
		Changes: &git.CategorizedChanges{
			Breaking: []git.ConventionalCommit{{
				Type:        "feat",
				Description: "drop the v1 endpoint",
				Breaking:    true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "drop the v1 endpoint") {
		t.Errorf("a supplied breaking change did not reach the output:\n%s", out)
	}
}

// The second layer: a nil receiver reached directly, rather than through the renderer.
func TestNilChangeSetAccessorsDoNotPanic(t *testing.T) {
	var nilChanges *git.CategorizedChanges

	if nilChanges.HasBreakingChanges() {
		t.Error("a nil change set reported breaking changes")
	}
	if nilChanges.HasChanges() {
		t.Error("a nil change set reported changes")
	}
	if got := nilChanges.TotalCount(); got != 0 {
		t.Errorf("TotalCount = %d, want 0", got)
	}
}
