// Package cli provides the command-line interface for Relicta.
package cli

import (
	"testing"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestOutputPlanText_Minimal(t *testing.T) {
	// Save original dryRun flag
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = false

	// Create test output
	v1, _ := version.Parse("1.0.0")
	v2, _ := version.Parse("1.1.0")

	changeSet := changes.NewChangeSet(
		changes.ChangeSetID("test-changeset"),
		"test-repo",
		"main",
	)

	output := &apprelease.PlanReleaseOutput{
		ReleaseID:      "test-release",
		CurrentVersion: v1,
		NextVersion:    v2,
		ReleaseType:    changes.ReleaseTypeMinor,
		ChangeSet:      changeSet,
		RepositoryName: "test-repo",
		Branch:         "main",
	}

	// Test with minimal=true
	err := outputPlanText(output, false, true, nil)
	if err != nil {
		t.Errorf("outputPlanText() with minimal=true error = %v", err)
	}
}

func TestOutputPlanText_WithShowAll(t *testing.T) {
	// Save original dryRun flag
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = false

	// Create test output
	v1, _ := version.Parse("1.0.0")
	v2, _ := version.Parse("1.1.0")

	changeSet := changes.NewChangeSet(
		changes.ChangeSetID("test-changeset"),
		"test-repo",
		"main",
	)

	// Add some commits to test all paths
	commit1 := changes.NewConventionalCommit(
		"abc123",
		changes.CommitTypeFeat,
		"add new feature",
		changes.WithScope("core"),
	)
	commit2 := changes.NewConventionalCommit(
		"def456",
		changes.CommitTypeFix,
		"fix bug",
	)
	commit3 := changes.NewConventionalCommit(
		"ghi789",
		changes.CommitTypePerf,
		"improve performance",
	)
	commit4 := changes.NewConventionalCommit(
		"jkl012",
		changes.CommitTypeDocs,
		"update docs",
	)
	commit5 := changes.NewConventionalCommit(
		"mno345",
		changes.CommitTypeFeat,
		"breaking change",
		changes.WithBreaking("API changed"),
	)

	changeSet.AddCommit(commit1)
	changeSet.AddCommit(commit2)
	changeSet.AddCommit(commit3)
	changeSet.AddCommit(commit4)
	changeSet.AddCommit(commit5)

	output := &apprelease.PlanReleaseOutput{
		ReleaseID:      "test-release",
		CurrentVersion: v1,
		NextVersion:    v2,
		ReleaseType:    changes.ReleaseTypeMinor,
		ChangeSet:      changeSet,
		RepositoryName: "test-repo",
		Branch:         "main",
	}

	// Test with showAll=true to cover all paths
	err := outputPlanText(output, true, false, nil)
	if err != nil {
		t.Errorf("outputPlanText() with showAll=true error = %v", err)
	}
}

func TestOutputPlanText_DryRun(t *testing.T) {
	// Save original dryRun flag
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = true

	// Create test output
	v1, _ := version.Parse("1.0.0")
	v2, _ := version.Parse("1.1.0")

	changeSet := changes.NewChangeSet(
		changes.ChangeSetID("test-changeset"),
		"test-repo",
		"main",
	)

	output := &apprelease.PlanReleaseOutput{
		ReleaseID:      "test-release",
		CurrentVersion: v1,
		NextVersion:    v2,
		ReleaseType:    changes.ReleaseTypeMinor,
		ChangeSet:      changeSet,
		RepositoryName: "test-repo",
		Branch:         "main",
	}

	// Test with dryRun=true
	err := outputPlanText(output, false, false, nil)
	if err != nil {
		t.Errorf("outputPlanText() with dryRun=true error = %v", err)
	}
}

func TestOutputPlanText_AllCommitTypes(t *testing.T) {
	// Save original dryRun flag
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = false

	// Create test output
	v1, _ := version.Parse("1.0.0")
	v2, _ := version.Parse("2.0.0")

	changeSet := changes.NewChangeSet(
		changes.ChangeSetID("test-changeset"),
		"test-repo",
		"main",
	)

	// Add commits for each category to test all output paths
	// Breaking changes
	breaking := changes.NewConventionalCommit(
		"break1",
		changes.CommitTypeFeat,
		"breaking API change",
		changes.WithBreaking("API signature changed"),
	)
	changeSet.AddCommit(breaking)

	// Features
	feat := changes.NewConventionalCommit(
		"feat1",
		changes.CommitTypeFeat,
		"add new feature",
		changes.WithScope("api"),
	)
	changeSet.AddCommit(feat)

	// Fixes
	fix := changes.NewConventionalCommit(
		"fix1",
		changes.CommitTypeFix,
		"fix critical bug",
	)
	changeSet.AddCommit(fix)

	// Performance
	perf := changes.NewConventionalCommit(
		"perf1",
		changes.CommitTypePerf,
		"optimize algorithm",
	)
	changeSet.AddCommit(perf)

	output := &apprelease.PlanReleaseOutput{
		ReleaseID:      "test-release",
		CurrentVersion: v1,
		NextVersion:    v2,
		ReleaseType:    changes.ReleaseTypeMajor,
		ChangeSet:      changeSet,
		RepositoryName: "test-repo",
		Branch:         "main",
	}

	// Test with all commit types to ensure all sections are printed
	err := outputPlanText(output, false, false, nil)
	if err != nil {
		t.Errorf("outputPlanText() with all commit types error = %v", err)
	}
}
