// Package release provides application use cases for release management.
package release

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
)

// ApproveReleaseInput represents the input for the ApproveRelease use case.
type ApproveReleaseInput struct {
	ReleaseID   release.ReleaseID
	ApprovedBy  string
	AutoApprove bool
	// EditedNotes contains user-edited release notes. If non-nil, the notes
	// will be updated before approval.
	EditedNotes *string
}

// ApproveReleaseOutput represents the output of the ApproveRelease use case.
type ApproveReleaseOutput struct {
	Approved    bool
	ApprovedBy  string
	ReleasePlan *release.ReleasePlan
}

// ApproveReleaseUseCase implements the approve release use case.
type ApproveReleaseUseCase struct {
	releaseRepo    release.Repository
	eventPublisher release.EventPublisher
	logger         *slog.Logger
}

// NewApproveReleaseUseCase creates a new ApproveReleaseUseCase.
func NewApproveReleaseUseCase(
	releaseRepo release.Repository,
	eventPublisher release.EventPublisher,
) *ApproveReleaseUseCase {
	return &ApproveReleaseUseCase{
		releaseRepo:    releaseRepo,
		eventPublisher: eventPublisher,
		logger:         slog.Default().With("usecase", "approve_release"),
	}
}

// Execute executes the approve release use case.
func (uc *ApproveReleaseUseCase) Execute(ctx context.Context, input ApproveReleaseInput) (*ApproveReleaseOutput, error) {
	// Retrieve release
	rel, err := uc.releaseRepo.FindByID(ctx, input.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	// Verify release can be approved (must have notes generated)
	if rel.State() != release.StateNotesGenerated {
		return nil, fmt.Errorf("release is not ready for approval: current state is %s", rel.State())
	}

	// Update release notes if edited notes were provided
	if input.EditedNotes != nil {
		if err := rel.UpdateNotes(*input.EditedNotes); err != nil {
			return nil, fmt.Errorf("failed to update release notes: %w", err)
		}
		uc.logger.Info("release notes updated",
			"release_id", rel.ID(),
			"notes_length", len(*input.EditedNotes))
	}

	// Approve the release
	if err := rel.Approve(input.ApprovedBy, input.AutoApprove); err != nil {
		return nil, fmt.Errorf("failed to approve release: %w", err)
	}

	// Save release
	if err := uc.releaseRepo.Save(ctx, rel); err != nil {
		return nil, fmt.Errorf("failed to save release: %w", err)
	}

	// Publish domain events
	if uc.eventPublisher != nil {
		if err := uc.eventPublisher.Publish(ctx, rel.DomainEvents()...); err != nil {
			uc.logger.Warn("failed to publish domain events",
				"error", err,
				"release_id", rel.ID())
		}
		rel.ClearDomainEvents()
	}

	return &ApproveReleaseOutput{
		Approved:    true,
		ApprovedBy:  input.ApprovedBy,
		ReleasePlan: rel.Plan(),
	}, nil
}

// GetReleaseForApprovalInput represents input for getting release details.
type GetReleaseForApprovalInput struct {
	ReleaseID release.ReleaseID
}

// GetReleaseForApprovalOutput represents output with release details for approval.
type GetReleaseForApprovalOutput struct {
	Release     *release.Release
	Summary     release.ReleaseSummary
	CanApprove  bool
	ApprovalMsg string
}

// GetReleaseForApprovalUseCase retrieves release details for approval review.
type GetReleaseForApprovalUseCase struct {
	releaseRepo release.Repository
}

// NewGetReleaseForApprovalUseCase creates a new use case.
func NewGetReleaseForApprovalUseCase(releaseRepo release.Repository) *GetReleaseForApprovalUseCase {
	return &GetReleaseForApprovalUseCase{releaseRepo: releaseRepo}
}

// Execute retrieves release details for approval.
func (uc *GetReleaseForApprovalUseCase) Execute(ctx context.Context, input GetReleaseForApprovalInput) (*GetReleaseForApprovalOutput, error) {
	rel, err := uc.releaseRepo.FindByID(ctx, input.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	output := &GetReleaseForApprovalOutput{
		Release: rel,
		Summary: rel.Summary(),
	}

	// Determine if release can be approved
	if rel.State() == release.StateNotesGenerated {
		output.CanApprove = true
		output.ApprovalMsg = "Release is ready for approval"
	} else if rel.State() == release.StateApproved {
		output.CanApprove = false
		output.ApprovalMsg = "Release is already approved"
	} else {
		output.CanApprove = false
		output.ApprovalMsg = fmt.Sprintf("Release cannot be approved in state: %s", rel.State())
	}

	return output, nil
}
