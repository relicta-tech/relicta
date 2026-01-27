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
