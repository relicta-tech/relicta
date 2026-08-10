package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// status used to print whatever the `latest` pointer named, unchecked. On this
// repository that meant reporting a run planned two months earlier — "Current:
// 4.0.3, Next: 4.1.0" — while the repository was already at 4.2.0, and then
// advising `relicta bump`, which would have produced a version that had already
// shipped. These tests pin the two independent signals that catch it.

// stalenessGitRepo is a stub whose tags and HEAD can be set per case. It embeds
// stubGitRepo so only the methods staleness detection reads are overridden.
type stalenessGitRepo struct {
	stubGitRepo
	tags       sourcecontrol.TagList
	headSHA    string
	between    int
	tagsErr    error
	headErr    error
	betweenErr error
}

func (g stalenessGitRepo) GetTags(context.Context) (sourcecontrol.TagList, error) {
	return g.tags, g.tagsErr
}

func (g stalenessGitRepo) GetLatestCommit(context.Context, string) (*sourcecontrol.Commit, error) {
	if g.headErr != nil {
		return nil, g.headErr
	}
	if g.headSHA == "" {
		return nil, nil
	}
	return sourcecontrol.NewCommit(
		sourcecontrol.CommitHash(g.headSHA), "head", sourcecontrol.Author{}, time.Now(),
	), nil
}

func (g stalenessGitRepo) GetCommitsBetween(context.Context, string, string) ([]*sourcecontrol.Commit, error) {
	if g.betweenErr != nil {
		return nil, g.betweenErr
	}
	out := make([]*sourcecontrol.Commit, 0, g.between)
	for range g.between {
		out = append(out, sourcecontrol.NewCommit("abc", "c", sourcecontrol.Author{}, time.Now()))
	}
	return out, nil
}

func tagList(t *testing.T, names ...string) sourcecontrol.TagList {
	t.Helper()
	tags := make(sourcecontrol.TagList, 0, len(names))
	for _, n := range names {
		tags = append(tags, sourcecontrol.NewTag(n, "deadbeef"))
	}
	return tags
}

// planRunAt builds a run whose recorded baseline and HEAD are known, using the
// real aggregate so the test cannot drift from how runs are actually created.
func planRunAt(t *testing.T, baseline string, headSHA string) *domain.ReleaseRun {
	t.Helper()
	run := domain.NewReleaseRun("repo", "/repo", "v"+baseline, domain.CommitSHA(headSHA),
		[]domain.CommitSHA{domain.CommitSHA(headSHA)}, "cfg", "plugins")
	cur := version.MustParse(baseline)
	next := version.MustParse("9.9.9")
	if err := run.SetVersionProposal(cur, next, domain.BumpMinor, 1.0); err != nil {
		t.Fatalf("SetVersionProposal: %v", err)
	}
	return run
}

func TestDetectRunStaleness_BaselineReleasedSince(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
	git := stalenessGitRepo{
		tags:    tagList(t, "v1.0.0", "v1.5.0"),
		headSHA: "aaaaaaaaaaaa", // HEAD unchanged, so only the baseline signal fires
	}

	reasons := detectRunStaleness(ctx, git, run, "v")
	if len(reasons) != 1 {
		t.Fatalf("expected exactly the baseline reason, got %v", reasons)
	}
	if !strings.Contains(reasons[0], "1.0.0") || !strings.Contains(reasons[0], "1.5.0") {
		t.Errorf("reason should name both versions so the reader can see the gap; got %q", reasons[0])
	}
}

func TestDetectRunStaleness_HeadMoved(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
	git := stalenessGitRepo{
		tags:    tagList(t, "v1.0.0"), // baseline still current
		headSHA: "bbbbbbbbbbbb",
		between: 3,
	}

	reasons := detectRunStaleness(ctx, git, run, "v")
	if len(reasons) != 1 {
		t.Fatalf("expected exactly the HEAD reason, got %v", reasons)
	}
	if !strings.Contains(reasons[0], "3 commit(s) since") {
		t.Errorf("reason should quantify the drift; got %q", reasons[0])
	}
}

func TestDetectRunStaleness_CurrentRunIsNotStale(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
	git := stalenessGitRepo{
		tags:    tagList(t, "v1.0.0"),
		headSHA: "aaaaaaaaaaaa",
	}

	if reasons := detectRunStaleness(ctx, git, run, "v"); len(reasons) != 0 {
		t.Errorf("a current run must not be reported stale: %v", reasons)
	}
}

// A false "stale" is its own failure: it would push someone to re-plan and
// discard a legitimate approval. So an unreadable repository reports nothing
// rather than guessing.
func TestDetectRunStaleness_GitErrorsReportNothing(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")

	cases := map[string]stalenessGitRepo{
		"tags unreadable": {tagsErr: context.DeadlineExceeded, headSHA: "aaaaaaaaaaaa"},
		"head unreadable": {tags: tagList(t, "v1.0.0"), headErr: context.DeadlineExceeded},
		"no tags at all":  {headSHA: "aaaaaaaaaaaa"},
	}
	for name, git := range cases {
		t.Run(name, func(t *testing.T) {
			if reasons := detectRunStaleness(ctx, git, run, "v"); len(reasons) != 0 {
				t.Errorf("expected no staleness claim, got %v", reasons)
			}
		})
	}
}

// The count is a detail; losing it must not lose the warning.
func TestDetectRunStaleness_HeadMovedWithoutCommitCount(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
	git := stalenessGitRepo{
		tags:       tagList(t, "v1.0.0"),
		headSHA:    "bbbbbbbbbbbb",
		betweenErr: context.DeadlineExceeded,
	}

	reasons := detectRunStaleness(ctx, git, run, "v")
	if len(reasons) != 1 {
		t.Fatalf("HEAD drift must still be reported without a count, got %v", reasons)
	}
	if strings.Contains(reasons[0], "commit(s) since") {
		t.Errorf("should omit the count when it could not be computed; got %q", reasons[0])
	}
}

// Tags outside the configured prefix are not this project's versions, so they
// must not read as staleness.
func TestDetectRunStaleness_IgnoresTagsOutsidePrefix(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
	git := stalenessGitRepo{
		tags:    tagList(t, "v1.0.0", "nightly-2026-08-08"),
		headSHA: "aaaaaaaaaaaa",
	}

	if reasons := detectRunStaleness(ctx, git, run, "v"); len(reasons) != 0 {
		t.Errorf("non-version tags must not trigger staleness: %v", reasons)
	}
}

// TestDetectRunStaleness_NonVTagPrefixesAreDetected replaces a test that pinned
// the opposite.
//
// This detector could not see a release made with any prefix other than "v",
// because sourcecontrol.NewTag decided version-ness by parsing the whole tag name
// and version.Parse accepts only bare semver or a leading "v". Such tags were
// dropped by VersionTags() before prefix filtering ran, so
// FilterByPrefix(prefix).VersionTags() — the sequence used here and in six other
// places — discarded exactly what the filter had selected.
//
// The earlier test asserted that non-"v" prefixes were *not* detected and said so
// in its name, deliberately failing once the limitation was lifted. It failed on
// the change that lifted it, which is what it was for.
func TestDetectRunStaleness_NonVTagPrefixesAreDetected(t *testing.T) {
	ctx := context.Background()

	// Each of these is a prefix a real project uses, and none of them worked.
	cases := []struct {
		prefix string
		tag    string
	}{
		{"release-", "release-1.5.0"},
		{"app-v", "app-v1.5.0"},
		{"rel/", "rel/1.5.0"},
		{"v", "v1.5.0"}, // the case that always worked, so it stays working
	}

	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
			git := stalenessGitRepo{
				tags:    tagList(t, tc.tag),
				headSHA: "aaaaaaaaaaaa",
			}

			reasons := detectRunStaleness(ctx, git, run, tc.prefix)
			if len(reasons) == 0 {
				t.Fatalf("a run planned against 1.0.0 is stale once %s exists, but nothing "+
					"was reported", tc.tag)
			}
			if !strings.Contains(reasons[0], "1.5.0") {
				t.Errorf("the reason should name the version now in the repository; got %q", reasons[0])
			}
		})
	}
}

// A tag carrying a different prefix is not this project's release, and must not
// be read as one. Without this, widening prefix support would make every
// component's tags look like every other component's in a monorepo.
func TestDetectRunStaleness_IgnoresOtherPrefixes(t *testing.T) {
	ctx := context.Background()
	run := planRunAt(t, "1.0.0", "aaaaaaaaaaaa")
	git := stalenessGitRepo{
		tags:    tagList(t, "web-v9.9.9"),
		headSHA: "aaaaaaaaaaaa",
	}

	if reasons := detectRunStaleness(ctx, git, run, "app-v"); len(reasons) != 0 {
		t.Errorf("web-v9.9.9 is not an app-v release; got %v", reasons)
	}
}
