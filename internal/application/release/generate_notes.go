// Package release provides application use cases for release management.
package release

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/relicta-tech/relicta/internal/domain/communication"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

// GenerateNotesInput represents the input for the GenerateNotes use case.
type GenerateNotesInput struct {
	ReleaseID        release.ReleaseID
	UseAI            bool
	Tone             communication.NoteTone
	Audience         communication.NoteAudience
	IncludeChangelog bool
	RepositoryURL    string
}

// Validate validates the GenerateNotesInput.
func (i *GenerateNotesInput) Validate() error {
	v := NewValidationError()

	// ReleaseID validation
	v.Add(ValidateReleaseID(i.ReleaseID))

	// Tone validation (empty is allowed, uses default)
	if i.Tone != "" && !i.Tone.IsValid() {
		v.AddMessage(fmt.Sprintf("invalid tone: %s", i.Tone))
	}

	// Audience validation (empty is allowed, uses default)
	if i.Audience != "" && !i.Audience.IsValid() {
		v.AddMessage(fmt.Sprintf("invalid audience: %s", i.Audience))
	}

	// RepositoryURL validation
	v.Add(ValidateURL(i.RepositoryURL, "repository URL"))

	return v.ToError()
}

// GenerateNotesOutput represents the output of the GenerateNotes use case.
type GenerateNotesOutput struct {
	ReleaseNotes *communication.ReleaseNotes
	Changelog    *communication.Changelog
}

// AINotesGenerator defines the interface for AI-based notes generation.
type AINotesGenerator interface {
	GenerateReleaseNotes(ctx context.Context, input AIGenerateInput) (*communication.ReleaseNotes, error)
}

// AIGenerateInput represents input for AI generation.
type AIGenerateInput struct {
	ReleaseContext *release.Release
	Tone           communication.NoteTone
	Audience       communication.NoteAudience
}

// GenerateNotesUseCase implements the generate notes use case.
type GenerateNotesUseCase struct {
	releaseRepo    release.Repository
	aiGenerator    AINotesGenerator
	eventPublisher release.EventPublisher
	logger         *slog.Logger
}

// NewGenerateNotesUseCase creates a new GenerateNotesUseCase.
func NewGenerateNotesUseCase(
	releaseRepo release.Repository,
	aiGenerator AINotesGenerator,
	eventPublisher release.EventPublisher,
) *GenerateNotesUseCase {
	return &GenerateNotesUseCase{
		releaseRepo:    releaseRepo,
		aiGenerator:    aiGenerator,
		eventPublisher: eventPublisher,
		logger:         slog.Default().With("usecase", "generate_notes"),
	}
}

// Execute executes the generate notes use case.
func (uc *GenerateNotesUseCase) Execute(ctx context.Context, input GenerateNotesInput) (*GenerateNotesOutput, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Retrieve release
	rel, err := uc.releaseRepo.FindByID(ctx, input.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	if rel.Plan() == nil {
		return nil, release.ErrNilPlan
	}

	plan := rel.Plan()
	changeSet := plan.GetChangeSet()

	// Use actual release version if set (by SetVersion in bump step),
	// otherwise fall back to planned version. This handles tag-push mode
	// where the existing tag version may differ from the calculated one.
	releaseVersion := plan.NextVersion
	if rel.Version() != nil {
		releaseVersion = *rel.Version()
	}

	var notes *communication.ReleaseNotes
	var changelog *communication.Changelog

	// Generate release notes
	if input.UseAI && uc.aiGenerator != nil {
		// Use AI to generate enhanced notes
		aiInput := AIGenerateInput{
			ReleaseContext: rel,
			Tone:           input.Tone,
			Audience:       input.Audience,
		}
		notes, err = uc.aiGenerator.GenerateReleaseNotes(ctx, aiInput)
		if err != nil {
			uc.logger.Warn("AI generation failed, falling back to standard generation",
				"error", err,
				"release_id", rel.ID())
			// Fall back to standard generation
			notes = communication.CreateFromChangeSet(releaseVersion, changeSet)
		}
	} else {
		// Standard generation from changeset
		notes = communication.CreateFromChangeSet(releaseVersion, changeSet)
	}

	// Generate changelog if requested
	if input.IncludeChangelog {
		changelog = communication.NewChangelog("Changelog", communication.FormatKeepAChangelog)
		entry := communication.CreateEntryFromChangeSet(releaseVersion, changeSet, input.RepositoryURL)
		changelog.AddEntry(entry)
	}

	// Update release with notes
	// Use RenderEntries() to get just the version entry without the "# Changelog" header
	// This is important because when updating an existing file, we don't want duplicate headers
	var changelogContent string
	if changelog != nil {
		changelogContent = changelog.RenderEntries()
	}

	releaseNotes := &release.ReleaseNotes{
		Changelog:   changelogContent,
		Summary:     notes.Summary(),
		AIGenerated: notes.IsAIGenerated(),
		GeneratedAt: notes.GeneratedAt(),
	}

	if err := rel.SetNotes(releaseNotes); err != nil {
		return nil, fmt.Errorf("failed to set release notes: %w", err)
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

	return &GenerateNotesOutput{
		ReleaseNotes: notes,
		Changelog:    changelog,
	}, nil
}
