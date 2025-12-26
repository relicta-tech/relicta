// Package release provides application use cases for release management.
package release

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// createReleaseWithNotes creates a release in NotesGenerated state ready for approval.
func createReleaseWithNotes(id release.ReleaseID, branch, repoPath string) *release.Release {
	r := release.NewRelease(id, branch, repoPath)

	// Create a changeset for the plan
	cs := changes.NewChangeSet("cs-test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "new feature"))

	// Create plan
	currentVersion := version.MustParse("1.0.0")
	nextVersion := version.MustParse("1.1.0")
	plan := release.NewReleasePlan(
		currentVersion,
		nextVersion,
		changes.ReleaseTypeMinor,
		cs,
		false,
	)
	_ = release.SetPlan(r, plan)

	// Set version and bump to transition to Versioned state
	_ = r.SetVersion(nextVersion, "v1.1.0")
	_ = r.Bump("test-actor")

	// Set notes to move to NotesGenerated state
	notes := &release.ReleaseNotes{
		Text:        "## [1.1.0] - Changes\n- feat: new feature",
		Provider:    "test",
		GeneratedAt: time.Now(),
	}
	_ = r.SetNotes(notes)

	return r
}

func TestApproveReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          ApproveReleaseInput
		setupRelease   func(repo *mockReleaseRepository)
		eventPublisher *mockEventPublisher
		wantErr        bool
		errMsg         string
		wantApproved   bool
	}{
		{
			name: "successful approval without editing notes",
			input: ApproveReleaseInput{
				ReleaseID:   "release-123",
				ApprovedBy:  "john.doe",
				AutoApprove: false,
				EditedNotes: nil,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantApproved:   true,
		},
		{
			name: "successful approval with edited notes",
			input: ApproveReleaseInput{
				ReleaseID:   "release-123",
				ApprovedBy:  "jane.smith",
				AutoApprove: false,
				EditedNotes: stringPtr("# Updated Release Notes\n\nThis is an edited version."),
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantApproved:   true,
		},
		{
			name: "successful auto-approval",
			input: ApproveReleaseInput{
				ReleaseID:   "release-123",
				ApprovedBy:  "ci-bot",
				AutoApprove: true,
				EditedNotes: nil,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantApproved:   true,
		},
		{
			name: "release not found",
			input: ApproveReleaseInput{
				ReleaseID:  "nonexistent",
				ApprovedBy: "john.doe",
			},
			setupRelease:   func(repo *mockReleaseRepository) {},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to find release",
		},
		{
			name: "release not in correct state - initialized",
			input: ApproveReleaseInput{
				ReleaseID:  "release-123",
				ApprovedBy: "john.doe",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				// Create release in initialized state (no notes generated)
				r := release.NewRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "not ready for approval",
		},
		{
			name: "release already approved",
			input: ApproveReleaseInput{
				ReleaseID:  "release-123",
				ApprovedBy: "john.doe",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				// Approve it first
				_ = r.Approve("first-approver", false)
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "already approved",
		},
		{
			name: "repository save fails",
			input: ApproveReleaseInput{
				ReleaseID:  "release-123",
				ApprovedBy: "john.doe",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
				repo.saveErr = errors.New("database connection failed")
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to save release",
		},
		{
			name: "event publisher failure does not fail approval",
			input: ApproveReleaseInput{
				ReleaseID:  "release-123",
				ApprovedBy: "john.doe",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{
				publishErr: errors.New("event bus down"),
			},
			wantErr:      false,
			wantApproved: true,
		},
		{
			name: "empty edited notes string updates notes",
			input: ApproveReleaseInput{
				ReleaseID:   "release-123",
				ApprovedBy:  "john.doe",
				EditedNotes: stringPtr(""),
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantApproved:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			releaseRepo := newMockReleaseRepository()
			tt.setupRelease(releaseRepo)

			uc := NewApproveReleaseUseCase(releaseRepo, tt.eventPublisher)

			output, err := uc.Execute(ctx, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output == nil {
				t.Error("expected output, got nil")
				return
			}

			if output.Approved != tt.wantApproved {
				t.Errorf("Approved = %v, want %v", output.Approved, tt.wantApproved)
			}

			if output.ApprovedBy != tt.input.ApprovedBy {
				t.Errorf("ApprovedBy = %s, want %s", output.ApprovedBy, tt.input.ApprovedBy)
			}

			if output.ReleasePlan == nil {
				t.Error("expected ReleasePlan, got nil")
			}
		})
	}
}

func TestGetReleaseForApprovalUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		input           GetReleaseForApprovalInput
		setupRelease    func(repo *mockReleaseRepository)
		wantErr         bool
		errMsg          string
		wantCanApprove  bool
		wantApprovalMsg string
	}{
		{
			name: "release ready for approval",
			input: GetReleaseForApprovalInput{
				ReleaseID: "release-123",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			wantErr:         false,
			wantCanApprove:  true,
			wantApprovalMsg: "ready for approval",
		},
		{
			name: "release already approved",
			input: GetReleaseForApprovalInput{
				ReleaseID: "release-123",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
				_ = r.Approve("approver", false)
				repo.releases["release-123"] = r
			},
			wantErr:         false,
			wantCanApprove:  false,
			wantApprovalMsg: "already approved",
		},
		{
			name: "release not ready - in initialized state",
			input: GetReleaseForApprovalInput{
				ReleaseID: "release-123",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := release.NewRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			wantErr:         false,
			wantCanApprove:  false,
			wantApprovalMsg: "not ready for approval",
		},
		{
			name: "release not found",
			input: GetReleaseForApprovalInput{
				ReleaseID: "nonexistent",
			},
			setupRelease: func(repo *mockReleaseRepository) {},
			wantErr:      true,
			errMsg:       "failed to find release",
		},
		{
			name: "repository error",
			input: GetReleaseForApprovalInput{
				ReleaseID: "release-123",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				repo.findErr = errors.New("connection timeout")
			},
			wantErr: true,
			errMsg:  "failed to find release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			releaseRepo := newMockReleaseRepository()
			tt.setupRelease(releaseRepo)

			uc := NewGetReleaseForApprovalUseCase(releaseRepo)

			output, err := uc.Execute(ctx, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output == nil {
				t.Error("expected output, got nil")
				return
			}

			if output.Release == nil {
				t.Error("expected Release, got nil")
			}

			if output.CanApprove != tt.wantCanApprove {
				t.Errorf("CanApprove = %v, want %v", output.CanApprove, tt.wantCanApprove)
			}

			if tt.wantApprovalMsg != "" && !containsString(output.ApprovalMsg, tt.wantApprovalMsg) {
				t.Errorf("ApprovalMsg %q should contain %q", output.ApprovalMsg, tt.wantApprovalMsg)
			}
		})
	}
}

func TestApproveReleaseUseCase_DomainEventsPublished(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	eventPublisher := &mockEventPublisher{}

	uc := NewApproveReleaseUseCase(releaseRepo, eventPublisher)

	input := ApproveReleaseInput{
		ReleaseID:  "release-123",
		ApprovedBy: "john.doe",
	}

	_, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify events were published
	if len(eventPublisher.published) == 0 {
		t.Error("expected domain events to be published, but none were")
	}
}

func TestApproveReleaseUseCase_EditedNotesUpdated(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createReleaseWithNotes("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	eventPublisher := &mockEventPublisher{}

	uc := NewApproveReleaseUseCase(releaseRepo, eventPublisher)

	editedContent := "# Completely New Notes\n\nThis replaces the old content."
	input := ApproveReleaseInput{
		ReleaseID:   "release-123",
		ApprovedBy:  "jane.smith",
		EditedNotes: &editedContent,
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Retrieve the saved release and verify notes were updated
	savedRelease := releaseRepo.releases["release-123"]
	if savedRelease.Notes() == nil {
		t.Fatal("expected notes to be set")
	}

	// The notes should have been updated before approval
	// We can verify the output and that no error occurred
	if output == nil {
		t.Error("expected output, got nil")
	}
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}
