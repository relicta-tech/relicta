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
	run.ReconstructState(
		domain.RunID(id),                    // use provided id
		"",                                  // planHash
		repoPath,                            // repoID
		repoPath,                            // repoRoot
		branch,                              // baseRef
		"",                                  // headSHA
		nil,                                 // commits
		"",                                  // configHash
		"",                                  // pluginPlanHash
		version.SemanticVersion{},           // versionCurrent
		version.SemanticVersion{},           // versionNext
		domain.BumpNone,                     // bumpKind
		0.0,                                 // confidence
		0.0,                                 // riskScore
		nil,                                 // reasons
		domain.ActorHuman,                   // actorType
		"",                                  // actorID
		domain.PolicyThresholds{},           // thresholds
		"",                                  // tagName
		nil,                                 // notes
		"",                                  // notesInputsHash
		nil,                                 // approval
		nil,                                 // steps
		make(map[string]*domain.StepStatus), // stepStatus
		domain.StateDraft,                   // state
		nil,                                 // history
		"",                                  // lastError
		"",                                  // changesetID
		run.CreatedAt(),                     // createdAt
		run.UpdatedAt(),                     // updatedAt
		nil,                                 // publishedAt
	)

	// Emit creation event (ReconstructState clears events, so we emit after)
	run.EmitCreatedEvent()

	return run
}
