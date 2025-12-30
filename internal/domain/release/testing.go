// Package release provides the release governance bounded context.
// This file provides test helpers for creating releases.
package release

import (
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

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
