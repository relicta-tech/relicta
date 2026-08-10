package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// `relicta bump` computed its own version and forced it onto the run through
// OverrideVersion, even though BumpVersionUseCase already reads run.VersionNext()
// and documents OverrideVersion as "if not provided, uses the version proposal
// from planning". Two components answering the same question is how they came to
// disagree — on a first release plan said 0.0.0 -> 0.1.0 while bump said
// 0.1.0 -> 0.2.0 (#264). Fixing that made the two agree; this removes the
// duplication that let them disagree at all.

// An explicit instruction outranks the recorded plan, and every flag that changes
// the answer counts. Reading only --level would silently discard --prerelease or
// --force in favor of the plan: the same class of surprise pointing the other way.
func TestBumpRequestIsExplicit(t *testing.T) {
	cases := []struct {
		name                                    string
		level, prerelease, build, forced, chann string
		want                                    bool
	}{
		{name: "nothing named", want: false},
		{name: "level", level: "major", want: true},
		{name: "prerelease", prerelease: "alpha", want: true},
		{name: "build metadata", build: "sha.abc123", want: true},
		{name: "forced version", forced: "2.0.0", want: true},
		{name: "channel", chann: "beta", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bumpRequestIsExplicit(tc.level, tc.prerelease, tc.build, tc.forced, tc.chann)
			if got != tc.want {
				t.Errorf("bumpRequestIsExplicit(%q,%q,%q,%q,%q) = %v, want %v",
					tc.level, tc.prerelease, tc.build, tc.forced, tc.chann, got, tc.want)
			}
		})
	}
}

// The plan has to reach the display and JSON paths in the shape they already
// expect, or honoring it would fork the output code and the two would drift.
func TestPlannedBumpPresentsAsACalculationResult(t *testing.T) {
	planned := plannedBump{
		Current: version.MustParse("1.0.0"),
		Next:    version.MustParse("1.1.0"),
		Kind:    domain.BumpMinor,
		RunID:   domain.RunID("run-abc123"),
	}

	out := planned.asCalculateOutput()
	if out.CurrentVersion.String() != "1.0.0" {
		t.Errorf("CurrentVersion = %s, want 1.0.0", out.CurrentVersion)
	}
	if out.NextVersion.String() != "1.1.0" {
		t.Errorf("NextVersion = %s, want 1.1.0", out.NextVersion)
	}
	if out.BumpType != version.BumpMinor {
		t.Errorf("BumpType = %s, want minor", out.BumpType)
	}
	// The bump kind came from commits when the plan was made, not from a flag on
	// this invocation.
	if !out.AutoDetected {
		t.Error("AutoDetected should be true: the kind was derived, not named here")
	}
}

// An explicit request must not even look at the plan. Guarded here because the
// lookup loads a run and runs staleness detection, and doing that work only to
// discard it would also print a stale-plan warning for a bump the plan does not
// govern.
func TestPlannedVersionIsNotConsultedForAnExplicitRequest(t *testing.T) {
	if got := plannedVersionToApply(context.TODO(), nil, true); got != nil {
		t.Errorf("an explicit request must bypass the plan entirely, got %+v", got)
	}
}
