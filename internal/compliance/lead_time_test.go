package compliance

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// Lead time for changes averaged ReleaseRecord.Duration — the runtime of the release
// process, a few seconds or minutes. That is not lead time by any definition, and
// because it was compared against DORA's 24-hour "elite" threshold, every project
// scored elite for publishing quickly no matter how long its changes had waited. A
// metric that always returns the best answer measures nothing.
//
// The interval is now commit → production deployment, and the report says which
// interval it used.

func releaseWithCommit(version string, committedAt, releasedAt time.Time, processDuration time.Duration) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:            "run-" + version,
		Repository:    "acme/widget",
		Version:       version,
		Outcome:       memory.OutcomeSuccess,
		FirstCommitAt: committedAt,
		ReleasedAt:    releasedAt,
		Duration:      processDuration,
	}
}

func leadTimeFor(t *testing.T, releases []*memory.ReleaseRecord, deployments []*memory.DeploymentRecord) LeadTimeForChanges {
	t.Helper()
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	for _, r := range releases {
		if err := store.RecordRelease(ctx, r); err != nil {
			t.Fatalf("RecordRelease: %v", err)
		}
	}
	for _, d := range deployments {
		if err := store.RecordDeployment(ctx, d); err != nil {
			t.Fatalf("RecordDeployment: %v", err)
		}
	}

	now := time.Now().UTC()
	report, err := NewGenerator(store, nil).Generate(ctx, ReportConfig{
		Type:                  ReportDORA,
		Format:                FormatJSON,
		Period:                Period{Start: now.Add(-90 * 24 * time.Hour), End: now.Add(time.Hour)},
		Repository:            "acme/widget",
		ProductionEnvironment: "production",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return report.DORA.LeadTimeForChanges
}

func TestLeadTimeMeasuresCommitToProduction(t *testing.T) {
	now := time.Now().UTC()
	committed := now.Add(-72 * time.Hour)
	released := now.Add(-2 * time.Hour)
	deployed := now.Add(-time.Hour)

	// The release process took 90 seconds. That must not be the reported lead time.
	releases := []*memory.ReleaseRecord{releaseWithCommit("1.0.0", committed, released, 90*time.Second)}
	deployments := []*memory.DeploymentRecord{deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, deployed)}

	lt := leadTimeFor(t, releases, deployments)

	if lt.MeasuredFrom != leadTimeFromCommit {
		t.Errorf("MeasuredFrom = %q, want %q", lt.MeasuredFrom, leadTimeFromCommit)
	}
	// 72h commit → deploy is ~71h, which is emphatically not "elite".
	if lt.MedianHours < 70 || lt.MedianHours > 72 {
		t.Errorf("MedianHours = %.2f, want ~71 (the commit waited three days); a value near "+
			"0.025 means the release process runtime is being reported again", lt.MedianHours)
	}
	if lt.Classification == "less-than-one-day" {
		t.Error("a change that took three days to reach production was rated elite: this is " +
			"exactly the false rating the old implementation produced for every project")
	}
	if lt.SampleSize != 1 {
		t.Errorf("SampleSize = %d, want 1", lt.SampleSize)
	}
}

// The oldest commit, not the newest. A release containing a three-week-old commit and
// one from this morning has a three-week lead time; reporting the morning's would
// describe the fastest change in the batch instead of the one the metric asks about.
func TestLeadTimeUsesTheOldestChangeInARelease(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", now.Add(-21*24*time.Hour), now.Add(-2*time.Hour), time.Minute)},
		[]*memory.DeploymentRecord{deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-time.Hour))},
	)

	if lt.MedianHours < 500 {
		t.Errorf("MedianHours = %.2f, want ~503 (three weeks): the release's slowest change "+
			"is the one lead time asks about", lt.MedianHours)
	}
	if lt.Classification != "one-month" {
		t.Errorf("Classification = %q, want one-month for a three-week lead time", lt.Classification)
	}
}

// Records written before FirstCommitAt existed have no beginning, so the interval
// falls back — and says so, because release → production reads low by exactly the time
// a change spent waiting to be released.
func TestLeadTimeFallsBackToReleaseAndLabelsIt(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", time.Time{}, now.Add(-50*time.Hour), time.Minute)},
		[]*memory.DeploymentRecord{deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-2*time.Hour))},
	)

	if lt.MeasuredFrom != leadTimeFromRelease {
		t.Errorf("MeasuredFrom = %q, want %q so a reader knows the pre-release wait is "+
			"missing from this figure", lt.MeasuredFrom, leadTimeFromRelease)
	}
	if lt.MedianHours < 47 || lt.MedianHours > 49 {
		t.Errorf("MedianHours = %.2f, want ~48", lt.MedianHours)
	}
}

// A zero FirstCommitAt must read as unknown, not as the epoch. Substituting the epoch
// would make every historical release a 56-year lead time and dominate the average.
func TestAZeroCommitDateIsNotTheEpoch(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", time.Time{}, now.Add(-3*time.Hour), time.Minute)},
		[]*memory.DeploymentRecord{deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-time.Hour))},
	)

	const hoursInFiftyYears = 50 * 365 * 24
	if lt.MedianHours > hoursInFiftyYears {
		t.Errorf("MedianHours = %.0f: a zero commit date was treated as the epoch", lt.MedianHours)
	}
}

// Nothing deployed means nothing reached production, so there is no end to measure to.
// Deliberately not falling back to "committed to tagged": that is a different quantity,
// and presenting it as lead time is the error being fixed.
func TestWithNoDeploymentsLeadTimeIsUnknownRatherThanElite(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", now.Add(-72*time.Hour), now.Add(-time.Hour), 30*time.Second)},
		nil,
	)

	if lt.MeasuredFrom != leadTimeUnavailable {
		t.Errorf("MeasuredFrom = %q, want %q", lt.MeasuredFrom, leadTimeUnavailable)
	}
	if lt.Classification == "less-than-one-day" {
		t.Error("with nothing deployed the report claimed elite lead time: an unmeasurable " +
			"metric must read as unknown, or a 30-second publish looks like fast delivery")
	}
	if lt.MedianHours != 0 {
		t.Errorf("MedianHours = %.2f, want 0 when nothing could be measured", lt.MedianHours)
	}
}

// A failed deployment did not reach users, so it is not the end of a lead time. It
// belongs to change failure rate.
func TestAFailedDeploymentIsNotAnArrival(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", now.Add(-72*time.Hour), now.Add(-2*time.Hour), time.Minute)},
		[]*memory.DeploymentRecord{deploymentAt("production", "1.0.0", memory.DeploymentFailed, now.Add(-time.Hour))},
	)

	if lt.MeasuredFrom != leadTimeUnavailable {
		t.Errorf("MeasuredFrom = %q, want %q: a failed deployment is not an arrival",
			lt.MeasuredFrom, leadTimeUnavailable)
	}
}

// A version redeployed later did not reach users later. Counting the redeploy would
// inflate the lead time of a change that shipped on time.
func TestARedeployDoesNotLengthenLeadTime(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", now.Add(-10*time.Hour), now.Add(-9*time.Hour), time.Minute)},
		[]*memory.DeploymentRecord{
			deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-8*time.Hour)),
			// Same version, redeployed a week later.
			deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-time.Hour)),
		},
	)

	if lt.MedianHours < 1 || lt.MedianHours > 3 {
		t.Errorf("MedianHours = %.2f, want ~2 (the first arrival); the redeploy must not "+
			"be counted as when the change reached users", lt.MedianHours)
	}
}

// Deployers read versions off image tags, where "v1.0.0" and "1.0.0" are both common.
// Failing to match would silently drop the sample and report a shorter history.
func TestAVPrefixStillMatchesItsRelease(t *testing.T) {
	now := time.Now().UTC()
	lt := leadTimeFor(t,
		[]*memory.ReleaseRecord{releaseWithCommit("1.0.0", now.Add(-30*time.Hour), now.Add(-2*time.Hour), time.Minute)},
		[]*memory.DeploymentRecord{deploymentAt("production", "v1.0.0", memory.DeploymentSucceeded, now.Add(-time.Hour))},
	)

	if lt.MeasuredFrom != leadTimeFromCommit {
		t.Errorf("MeasuredFrom = %q: a deployment reporting v1.0.0 must match release 1.0.0, "+
			"or the sample is dropped and the report reads shorter than reality", lt.MeasuredFrom)
	}
}

// classifyDORA picked the winning level by iterating a map, and Go randomizes map
// iteration — so with two levels tied the overall rating differed between runs on
// identical data. An audit artifact that changes when nothing changed cannot be
// evidence of anything.
//
// The votes below are a genuine 2-2 tie: deployment frequency and change failure rate
// vote elite, MTTR and lead time vote low. An earlier version of this test was not
// actually tied — two metrics fell through to "low", giving it a unique maximum that
// the map iteration could not disturb — so it passed against the bug it was meant to
// catch.
func TestOverallClassificationIsDeterministic(t *testing.T) {
	report := &DORAReport{
		DeploymentFrequency: DeploymentFrequency{Classification: "on-demand"},  // elite
		ChangeFailureRate:   ChangeFailureRate{Classification: "0-15%"},        // elite
		MTTR:                MTTRMetrics{Classification: "more-than-one-week"}, // low
		LeadTimeForChanges: LeadTimeForChanges{ // low
			MeasuredFrom:   leadTimeFromCommit,
			Classification: "more-than-six-months",
		},
	}

	if got := scoreVotes(report); got["elite"] != got["low"] || got["elite"] != 2 {
		t.Fatalf("precondition: this test needs a real 2-2 tie, got %v", got)
	}

	first := classifyDORA(report)
	for i := 0; i < 500; i++ {
		if got := classifyDORA(report); got != first {
			t.Fatalf("classifyDORA returned %q then %q for identical input: the rating must "+
				"not depend on map iteration order", first, got)
		}
	}

	// And the tie must resolve downward: a report is not the place to round a
	// disagreement between "elite" and "low" upward.
	if first != "low" {
		t.Errorf("a 2-2 tie between elite and low resolved to %q; the conservative answer is "+
			"the lower rating", first)
	}
}

// An unmeasurable lead time must not be scored as poor performance. That would punish
// the honest state — a project that has not wired up deployment reporting is not
// thereby a low performer.
//
// Constructed so the extra vote decides the outcome: excluded, elite wins 2-1;
// counted as low, it becomes a 2-2 tie that resolves down to low.
func TestAnUnmeasurableLeadTimeDoesNotCountAgainstTheScore(t *testing.T) {
	report := &DORAReport{
		DeploymentFrequency: DeploymentFrequency{Classification: "on-demand"},  // elite
		ChangeFailureRate:   ChangeFailureRate{Classification: "0-15%"},        // elite
		MTTR:                MTTRMetrics{Classification: "more-than-one-week"}, // low
		LeadTimeForChanges:  LeadTimeForChanges{MeasuredFrom: leadTimeUnavailable},
	}

	if got := classifyDORA(report); got != "elite" {
		t.Errorf("classifyDORA = %q, want elite: two elite metrics against one low must not "+
			"be dragged to a tie by the metric that has no data at all", got)
	}
}
