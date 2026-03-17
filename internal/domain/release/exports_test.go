package release

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewReleasePlan(t *testing.T) {
	currentVersion := version.MustParse("1.0.0")
	nextVersion := version.MustParse("1.1.0")
	releaseType := changes.ReleaseTypeMinor

	// Test with nil changeset
	plan := NewReleasePlan(currentVersion, nextVersion, releaseType, nil, false)
	if plan == nil {
		t.Fatal("NewReleasePlan should not return nil")
	}
	if plan.CurrentVersion.String() != "1.0.0" {
		t.Errorf("CurrentVersion = %s, want 1.0.0", plan.CurrentVersion.String())
	}
	if plan.NextVersion.String() != "1.1.0" {
		t.Errorf("NextVersion = %s, want 1.1.0", plan.NextVersion.String())
	}
	if plan.ReleaseType != releaseType {
		t.Errorf("ReleaseType = %s, want %s", plan.ReleaseType, releaseType)
	}
	if plan.DryRun {
		t.Error("DryRun should be false")
	}

	// Test with changeset
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan = NewReleasePlan(currentVersion, nextVersion, releaseType, changeSet, true)
	if plan.ChangeSetID != changeSet.ID() {
		t.Errorf("ChangeSetID = %s, want %s", plan.ChangeSetID, changeSet.ID())
	}
	if !plan.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestReleasePlan_ChangeSet(t *testing.T) {
	currentVersion := version.MustParse("1.0.0")
	nextVersion := version.MustParse("1.1.0")
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")

	plan := NewReleasePlan(currentVersion, nextVersion, changes.ReleaseTypeMinor, changeSet, false)

	// Test GetChangeSet
	if plan.GetChangeSet() != changeSet {
		t.Error("GetChangeSet should return the original changeset")
	}

	// Test HasChangeSet
	if !plan.HasChangeSet() {
		t.Error("HasChangeSet should return true when changeset is set")
	}

	// Test SetChangeSet
	newChangeSet := changes.NewChangeSet("cs-2", "v1.0.0", "HEAD")
	plan.SetChangeSet(newChangeSet)
	if plan.GetChangeSet() != newChangeSet {
		t.Error("GetChangeSet should return the new changeset after SetChangeSet")
	}

	// Test CommitCount with empty changeset
	if plan.CommitCount() != 0 {
		t.Errorf("CommitCount = %d, want 0 for empty changeset", plan.CommitCount())
	}

	// Test without changeset
	plan.SetChangeSet(nil)
	if plan.HasChangeSet() {
		t.Error("HasChangeSet should return false when changeset is nil")
	}
	if plan.CommitCount() != 0 {
		t.Errorf("CommitCount = %d, want 0 when changeset is nil", plan.CommitCount())
	}
}

func TestGetPlan_NilRun(t *testing.T) {
	plan := GetPlan(nil)
	if plan != nil {
		t.Error("GetPlan(nil) should return nil")
	}
}

func TestSetPlan(t *testing.T) {
	// Create a release run
	run := NewReleaseRun(
		"org/repo",
		"/tmp/repo",
		"v1.0.0",
		"abc123",
		[]CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	currentVersion := version.MustParse("1.0.0")
	nextVersion := version.MustParse("1.1.0")
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(currentVersion, nextVersion, changes.ReleaseTypeMinor, changeSet, false)

	err := SetPlan(run, plan)
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Verify the run has the plan data
	if run.VersionNext().String() != "1.1.0" {
		t.Errorf("VersionNext = %s, want 1.1.0", run.VersionNext().String())
	}
	if !run.HasChangeSet() {
		t.Error("Run should have changeset")
	}
}

func TestSetPlan_NilPlan(t *testing.T) {
	run := NewReleaseRun(
		"org/repo",
		"/tmp/repo",
		"v1.0.0",
		"abc123",
		[]CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	err := SetPlan(run, nil)
	if err == nil {
		t.Error("SetPlan(run, nil) should return error")
	}
}

func TestGetPlan_WithRun(t *testing.T) {
	run := NewReleaseRun(
		"org/repo",
		"/tmp/repo",
		"v1.0.0",
		"abc123",
		[]CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	currentVersion := version.MustParse("1.0.0")
	nextVersion := version.MustParse("1.1.0")
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(currentVersion, nextVersion, changes.ReleaseTypeMinor, changeSet, false)

	err := SetPlan(run, plan)
	if err != nil {
		t.Fatalf("SetPlan failed: %v", err)
	}

	// Now get the plan back
	retrievedPlan := GetPlan(run)
	if retrievedPlan == nil {
		t.Fatal("GetPlan returned nil")
	}

	if retrievedPlan.NextVersion.String() != "1.1.0" {
		t.Errorf("NextVersion = %s, want 1.1.0", retrievedPlan.NextVersion.String())
	}
	if !retrievedPlan.HasChangeSet() {
		t.Error("Plan should have changeset")
	}
}
