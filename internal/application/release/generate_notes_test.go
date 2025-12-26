// Package release provides application use cases for release management.
package release

import (
	"context"
	"errors"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/communication"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// mockAINotesGenerator implements AINotesGenerator for testing.
type mockAINotesGenerator struct {
	notes *communication.ReleaseNotes
	err   error
}

func (m *mockAINotesGenerator) GenerateReleaseNotes(ctx context.Context, input AIGenerateInput) (*communication.ReleaseNotes, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.notes, nil
}

// createReleaseWithPlan creates a release with a plan ready for notes generation.
func createReleaseWithPlan(id release.ReleaseID, branch, repoPath string) *release.Release {
	r := release.NewRelease(id, branch, repoPath)

	// Create a changeset with various commit types
	cs := changes.NewChangeSet("cs-test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add new feature"))
	cs.AddCommit(changes.NewConventionalCommit("def456", changes.CommitTypeFix, "fix bug in login"))

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
	_ = r.SetVersion(nextVersion, "v1.1.0")
	_ = r.Bump("test-actor")

	return r
}

func TestGenerateNotesUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          GenerateNotesInput
		setupRelease   func(repo *mockReleaseRepository)
		aiGenerator    AINotesGenerator
		eventPublisher *mockEventPublisher
		wantErr        bool
		errMsg         string
		wantChangelog  bool
	}{
		{
			name: "successful standard generation without AI",
			input: GenerateNotesInput{
				ReleaseID:        "release-123",
				UseAI:            false,
				Tone:             communication.ToneProfessional,
				Audience:         communication.AudienceDevelopers,
				IncludeChangelog: false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator:    nil,
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantChangelog:  false,
		},
		{
			name: "successful standard generation with changelog",
			input: GenerateNotesInput{
				ReleaseID:        "release-123",
				UseAI:            false,
				IncludeChangelog: true,
				RepositoryURL:    "https://github.com/owner/repo",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator:    nil,
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantChangelog:  true,
		},
		{
			name: "successful AI generation",
			input: GenerateNotesInput{
				ReleaseID:        "release-123",
				UseAI:            true,
				Tone:             communication.ToneFriendly,
				Audience:         communication.AudienceUsers,
				IncludeChangelog: true,
				RepositoryURL:    "https://github.com/owner/repo",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator: &mockAINotesGenerator{
				notes: communication.NewReleaseNotesBuilder(version.MustParse("1.1.0")).
					WithTitle("Release 1.1.0 - AI Generated").
					WithSummary("AI-generated release notes with enhanced descriptions").
					AIGenerated().
					WithTone(communication.ToneFriendly).
					WithAudience(communication.AudienceUsers).
					Build(),
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantChangelog:  true,
		},
		{
			name: "AI generation fails, falls back to standard",
			input: GenerateNotesInput{
				ReleaseID:        "release-123",
				UseAI:            true,
				Tone:             communication.ToneProfessional,
				Audience:         communication.AudienceDevelopers,
				IncludeChangelog: false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator: &mockAINotesGenerator{
				err: errors.New("AI service unavailable"),
			},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false, // Should fall back gracefully
			wantChangelog:  false,
		},
		{
			name: "release not found",
			input: GenerateNotesInput{
				ReleaseID: "nonexistent",
				UseAI:     false,
			},
			setupRelease:   func(repo *mockReleaseRepository) {},
			aiGenerator:    nil,
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to find release",
		},
		{
			name: "release has no plan",
			input: GenerateNotesInput{
				ReleaseID: "release-123",
				UseAI:     false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				// Create release without plan - state is Draft, not Versioned
				r := release.NewRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator:    nil,
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "can only generate notes from Versioned state",
		},
		{
			name: "repository save fails",
			input: GenerateNotesInput{
				ReleaseID: "release-123",
				UseAI:     false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
				repo.saveErr = errors.New("database error")
			},
			aiGenerator:    nil,
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to save release",
		},
		{
			name: "event publisher failure does not fail generation",
			input: GenerateNotesInput{
				ReleaseID: "release-123",
				UseAI:     false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator: nil,
			eventPublisher: &mockEventPublisher{
				publishErr: errors.New("event bus error"),
			},
			wantErr: false,
		},
		{
			name: "UseAI true but aiGenerator is nil - falls back to standard",
			input: GenerateNotesInput{
				ReleaseID:        "release-123",
				UseAI:            true,
				Tone:             communication.ToneProfessional,
				Audience:         communication.AudienceDevelopers,
				IncludeChangelog: false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			aiGenerator:    nil, // AI requested but generator not available
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantChangelog:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			releaseRepo := newMockReleaseRepository()
			tt.setupRelease(releaseRepo)

			uc := NewGenerateNotesUseCase(releaseRepo, tt.aiGenerator, tt.eventPublisher)

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

			if output.ReleaseNotes == nil {
				t.Error("expected ReleaseNotes, got nil")
			}

			if tt.wantChangelog && output.Changelog == nil {
				t.Error("expected Changelog, got nil")
			}

			if !tt.wantChangelog && output.Changelog != nil {
				t.Error("expected no Changelog, got one")
			}

			// Verify the release was saved with notes
			savedRelease := releaseRepo.releases[tt.input.ReleaseID]
			if savedRelease != nil && savedRelease.Notes() == nil {
				t.Error("expected release to have notes set after generation")
			}
		})
	}
}

func TestGenerateNotesUseCase_DomainEventsPublished(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	eventPublisher := &mockEventPublisher{}

	uc := NewGenerateNotesUseCase(releaseRepo, nil, eventPublisher)

	input := GenerateNotesInput{
		ReleaseID:        "release-123",
		UseAI:            false,
		IncludeChangelog: false,
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

func TestGenerateNotesUseCase_AIInputContext(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	var capturedInput AIGenerateInput
	// Create a custom AI generator that captures input
	customAIGen := &customAIGenerator{
		capturedInput: &capturedInput,
		notes: communication.NewReleaseNotesBuilder(version.MustParse("1.1.0")).
			WithTitle("AI Generated").
			AIGenerated().
			Build(),
	}

	eventPublisher := &mockEventPublisher{}
	uc := NewGenerateNotesUseCase(releaseRepo, customAIGen, eventPublisher)

	input := GenerateNotesInput{
		ReleaseID: "release-123",
		UseAI:     true,
		Tone:      communication.ToneFriendly,
		Audience:  communication.AudienceUsers,
	}

	_, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify AI input context
	if capturedInput.ReleaseContext == nil {
		t.Error("expected ReleaseContext to be set")
	}

	if capturedInput.Tone != communication.ToneFriendly {
		t.Errorf("Tone = %v, want %v", capturedInput.Tone, communication.ToneFriendly)
	}

	if capturedInput.Audience != communication.AudienceUsers {
		t.Errorf("Audience = %v, want %v", capturedInput.Audience, communication.AudienceUsers)
	}
}

// customAIGenerator captures AI generation input for testing.
type customAIGenerator struct {
	capturedInput *AIGenerateInput
	notes         *communication.ReleaseNotes
}

func (c *customAIGenerator) GenerateReleaseNotes(ctx context.Context, input AIGenerateInput) (*communication.ReleaseNotes, error) {
	*c.capturedInput = input
	return c.notes, nil
}

func TestGenerateNotesUseCase_ChangelogGeneration(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	eventPublisher := &mockEventPublisher{}
	uc := NewGenerateNotesUseCase(releaseRepo, nil, eventPublisher)

	input := GenerateNotesInput{
		ReleaseID:        "release-123",
		UseAI:            false,
		IncludeChangelog: true,
		RepositoryURL:    "https://github.com/owner/repo",
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Changelog == nil {
		t.Fatal("expected Changelog to be generated")
	}

	// Verify changelog has entries
	rendered := output.Changelog.Render()
	if rendered == "" {
		t.Error("expected non-empty changelog render")
	}

	// Verify the release has changelog content saved
	savedRelease := releaseRepo.releases["release-123"]
	if savedRelease.Notes() == nil {
		t.Fatal("expected release to have notes")
	}

	if savedRelease.Notes().Text == "" {
		t.Error("expected release notes to include changelog content")
	}
}

func TestGenerateNotesUseCase_NotesContentValidation(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createReleaseWithPlan("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	eventPublisher := &mockEventPublisher{}
	uc := NewGenerateNotesUseCase(releaseRepo, nil, eventPublisher)

	input := GenerateNotesInput{
		ReleaseID:        "release-123",
		UseAI:            false,
		IncludeChangelog: false,
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify notes content
	if output.ReleaseNotes.Summary() == "" {
		t.Error("expected non-empty summary")
	}

	// Verify the release has complete notes
	savedRelease := releaseRepo.releases["release-123"]
	notes := savedRelease.Notes()

	if notes == nil {
		t.Fatal("expected notes to be set")
	}

	if notes.Text == "" {
		t.Error("expected non-empty Text in saved notes")
	}

	if notes.GeneratedAt.IsZero() {
		t.Error("expected GeneratedAt to be set")
	}
}
