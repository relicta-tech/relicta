// Package release provides the release governance bounded context.
// This file provides test helpers for creating releases.
package release

import (
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// TestCommit is a dummy commit SHA for testing purposes.
const TestCommit = CommitSHA("abc123def456")

// NewReleaseRunForTest creates a ReleaseRun with a specific ID for testing purposes.
// This is the replacement for the deprecated NewRelease function.
// It creates a release in StateDraft state with minimal initialization.
func NewReleaseRunForTest(id RunID, branch, repoPath string) *ReleaseRun {
	run := domain.NewReleaseRun(
		repoPath, // repoID - use path as ID
		repoPath, // repoRoot
		branch,   // baseRef
		"",       // headSHA - will be set later
		nil,      // commits - will be set later
		"",       // configHash
		"",       // pluginPlanHash
	)

	// Override the auto-generated ID with the provided ID
	run.ReconstructState(domain.RunSnapshot{
		ID:              domain.RunID(id),
		PlanHash:        "",
		RepoID:          repoPath,
		RepoRoot:        repoPath,
		BaseRef:         branch,
		HeadSHA:         "",
		Commits:         nil,
		ConfigHash:      "",
		PluginPlanHash:  "",
		VersionCurrent:  version.SemanticVersion{},
		VersionNext:     version.SemanticVersion{},
		BumpKind:        domain.BumpNone,
		Confidence:      0.0,
		RiskScore:       0.0,
		Reasons:         nil,
		ActorType:       domain.ActorHuman,
		ActorID:         "",
		Thresholds:      domain.PolicyThresholds{},
		TagName:         "",
		Notes:           nil,
		NotesInputsHash: "",
		Approval:        nil,
		Steps:           nil,
		StepStatus:      make(map[string]*domain.StepStatus),
		State:           domain.StateDraft,
		History:         nil,
		LastError:       "",
		ChangesetID:     "",
		CreatedAt:       run.CreatedAt(),
		UpdatedAt:       run.UpdatedAt(),
		PublishedAt:     nil,
	})

	// Emit creation event (ReconstructState clears events, so we emit after)
	run.EmitCreatedEvent()

	return run
}

// NewReleaseRunForTestWithCommits creates a ReleaseRun with commit data for governance testing.
// This variant includes a commit range to satisfy CGP scope requirements.
// It sets up a proper ReleasePlan with a ChangeSet so governance can evaluate the scope.
func NewReleaseRunForTestWithCommits(id RunID, branch, repoPath string) *ReleaseRun {
	commits := []domain.CommitSHA{domain.CommitSHA(TestCommit)}
	headSHA := domain.CommitSHA(TestCommit)

	run := domain.NewReleaseRun(
		repoPath, // repoID - use path as ID
		repoPath, // repoRoot
		"v0.0.0", // baseRef - tag-like ref for proper commit range
		headSHA,  // headSHA
		commits,  // commits
		"",       // configHash
		"",       // pluginPlanHash
	)

	// Override the auto-generated ID with the provided ID
	run.ReconstructState(domain.RunSnapshot{
		ID:              domain.RunID(id),
		PlanHash:        run.PlanHash(),
		RepoID:          repoPath,
		RepoRoot:        repoPath,
		BaseRef:         "v0.0.0",
		HeadSHA:         headSHA,
		Commits:         commits,
		ConfigHash:      "",
		PluginPlanHash:  "",
		VersionCurrent:  version.SemanticVersion{},
		VersionNext:     version.SemanticVersion{},
		BumpKind:        domain.BumpNone,
		Confidence:      0.0,
		RiskScore:       0.0,
		Reasons:         nil,
		ActorType:       domain.ActorHuman,
		ActorID:         "",
		Thresholds:      domain.PolicyThresholds{},
		TagName:         "",
		Notes:           nil,
		NotesInputsHash: "",
		Approval:        nil,
		Steps:           nil,
		StepStatus:      make(map[string]*domain.StepStatus),
		State:           domain.StateDraft,
		History:         nil,
		LastError:       "",
		ChangesetID:     "",
		CreatedAt:       run.CreatedAt(),
		UpdatedAt:       run.UpdatedAt(),
		PublishedAt:     nil,
	})

	// Emit creation event (ReconstructState clears events, so we emit after)
	run.EmitCreatedEvent()

	// Set up a plan with a changeset for governance testing
	// The changeset needs valid fromRef/toRef for CGP scope validation
	changeSet := changes.NewChangeSet(
		changes.ChangeSetID("test-changeset"),
		"v0.0.0",        // fromRef
		string(headSHA), // toRef
	)

	v0, _ := version.Parse("0.0.0")
	v1, _ := version.Parse("0.1.0")

	plan := NewReleasePlan(
		v0,
		v1,
		changes.ReleaseTypeMinor,
		changeSet,
		false, // dryRun
	)

	// SetPlan transitions to Planned state and stores the changeset
	_ = SetPlan(run, plan)

	return run
}
