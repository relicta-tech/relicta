package sourcecontrol

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Two constants describe a repository that has never released, and they mean
// different things:
//
//	version.Zero    = 0.0.0  — nothing has shipped
//	version.Initial = 0.1.0  — the first version to publish
//
// DiscoverCurrentVersion answered "what has shipped?" with Initial, so `relicta
// bump` bumped from 0.1.0 and produced 0.2.0 for a first release, while `relicta
// plan` reported 0.0.0 and predicted 0.1.0 for the same repository. The plan output
// could not be read as a preview of the bump, and 0.1.0 was never produced at all.
//
// Both constants remain — "the first version to publish" is a real idea — but only
// one of them answers this question.

func TestDiscoverCurrentVersionReportsNothingShippedAsZero(t *testing.T) {
	cases := map[string]*stubGitRepo{
		"no tags at all":             {latestTag: nil},
		"a tag with no version":      {latestTag: NewTag("nightly-2026-08-10", "abc123")},
		"a tag under another prefix": {latestTag: NewTag("app-v1.0.0", "abc123")},
	}

	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := NewVersionDiscovery("v").DiscoverCurrentVersion(context.Background(), repo)
			if err != nil {
				t.Fatalf("DiscoverCurrentVersion: %v", err)
			}
			if got != version.Zero {
				t.Errorf("got %v, want %v — nothing has been released, and reporting 0.1.0 "+
					"makes bump produce 0.2.0 for a first release", got, version.Zero)
			}
		})
	}
}

// The property the two commands disagreed on, asserted directly: a minor bump from
// "nothing shipped" is 0.1.0, which is what plan predicts. If this ever returns
// 0.2.0 again, the first release has silently moved.
func TestAMinorFirstReleaseIsZeroPointOne(t *testing.T) {
	current, err := NewVersionDiscovery("v").
		DiscoverCurrentVersion(context.Background(), &stubGitRepo{latestTag: nil})
	if err != nil {
		t.Fatalf("DiscoverCurrentVersion: %v", err)
	}

	next := version.NewVersionBump(version.BumpMinor).Apply(current)
	if next.String() != "0.1.0" {
		t.Errorf("a first minor release is %s, want 0.1.0", next)
	}
}

// A released version must still be reported as itself — the change must not make
// every repository look unreleased.
func TestDiscoverCurrentVersionReportsAReleasedVersion(t *testing.T) {
	repo := &stubGitRepo{latestTag: NewTag("v1.2.3", "abc123")}

	got, err := NewVersionDiscovery("v").DiscoverCurrentVersion(context.Background(), repo)
	if err != nil {
		t.Fatalf("DiscoverCurrentVersion: %v", err)
	}
	if got.String() != "1.2.3" {
		t.Errorf("got %v, want 1.2.3", got)
	}
}
