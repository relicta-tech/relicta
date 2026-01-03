// Package container provides dependency injection for the Relicta application.
package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/integration"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

// NotesGeneratorAdapter adapts the AI service to the ports.NotesGenerator interface.
type NotesGeneratorAdapter struct {
	aiService  ai.Service
	gitAdapter *git.Adapter
}

// NewNotesGeneratorAdapter creates a new NotesGeneratorAdapter.
func NewNotesGeneratorAdapter(aiService ai.Service, gitAdapter *git.Adapter) *NotesGeneratorAdapter {
	return &NotesGeneratorAdapter{
		aiService:  aiService,
		gitAdapter: gitAdapter,
	}
}

// Generate creates release notes for the given run.
func (a *NotesGeneratorAdapter) Generate(ctx context.Context, run *domain.ReleaseRun, options ports.NotesOptions) (*domain.ReleaseNotes, error) {
	if a.aiService == nil || !a.aiService.IsAvailable() {
		// Fallback to basic changelog without AI enhancement
		return a.generateBasicNotes(ctx, run, options)
	}

	// Get the changeset from the run
	changeSet := run.ChangeSet()
	if changeSet == nil {
		return nil, fmt.Errorf("no changeset available in run")
	}

	// Convert changeset to git.CategorizedChanges for AI service
	categorized := a.convertToCategorizedChanges(changeSet)

	// Configure generation options
	genOpts := ai.GenerateOptions{
		Version:     ptrTo(run.VersionNext()),
		ProductName: "",
		Tone:        a.mapTone(options.TonePreset),
		Audience:    a.mapAudience(options.AudiencePreset),
	}

	// Generate changelog using AI
	changelog, err := a.aiService.GenerateChangelog(ctx, categorized, genOpts)
	if err != nil {
		return nil, fmt.Errorf("AI changelog generation failed: %w", err)
	}

	// Generate release notes from changelog
	releaseNotes, err := a.aiService.GenerateReleaseNotes(ctx, changelog, genOpts)
	if err != nil {
		return nil, fmt.Errorf("AI release notes generation failed: %w", err)
	}

	// Combine changelog and release notes into Text field
	combinedText := changelog
	if releaseNotes != "" && releaseNotes != changelog {
		combinedText = releaseNotes + "\n\n## Changelog\n\n" + changelog
	}

	return &domain.ReleaseNotes{
		Text:           combinedText,
		AudiencePreset: options.AudiencePreset,
		TonePreset:     options.TonePreset,
		Provider:       options.Provider,
		Model:          options.Model,
		GeneratedAt:    time.Now(),
	}, nil
}

// ComputeInputsHash computes a hash of the inputs used to generate notes.
func (a *NotesGeneratorAdapter) ComputeInputsHash(run *domain.ReleaseRun, options ports.NotesOptions) string {
	h := sha256.New()

	// Include version in hash
	h.Write([]byte(run.VersionNext().String()))

	// Include HEAD SHA
	h.Write([]byte(run.HeadSHA().String()))

	// Include options
	h.Write([]byte(options.AudiencePreset))
	h.Write([]byte(options.TonePreset))
	if options.UseAI {
		h.Write([]byte("ai:true"))
		h.Write([]byte(options.Provider))
		h.Write([]byte(options.Model))
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// generateBasicNotes creates basic release notes without AI.
func (a *NotesGeneratorAdapter) generateBasicNotes(ctx context.Context, run *domain.ReleaseRun, options ports.NotesOptions) (*domain.ReleaseNotes, error) {
	changeSet := run.ChangeSet()
	if changeSet == nil {
		return &domain.ReleaseNotes{
			Text:           "Release " + run.VersionNext().String(),
			AudiencePreset: options.AudiencePreset,
			TonePreset:     options.TonePreset,
			Provider:       "basic",
			Model:          "",
			GeneratedAt:    time.Now(),
		}, nil
	}

	// Build basic changelog from commits
	var changelog string
	for _, commit := range changeSet.Commits() {
		changelog += fmt.Sprintf("- %s\n", commit.Subject())
	}

	return &domain.ReleaseNotes{
		Text:           changelog,
		AudiencePreset: options.AudiencePreset,
		TonePreset:     options.TonePreset,
		Provider:       "basic",
		Model:          "",
		GeneratedAt:    time.Now(),
	}, nil
}

// convertToCategorizedChanges converts a ChangeSet to git.CategorizedChanges.
func (a *NotesGeneratorAdapter) convertToCategorizedChanges(cs *changes.ChangeSet) *git.CategorizedChanges {
	result := &git.CategorizedChanges{
		Features:      []git.ConventionalCommit{},
		Fixes:         []git.ConventionalCommit{},
		Performance:   []git.ConventionalCommit{},
		Documentation: []git.ConventionalCommit{},
		Refactoring:   []git.ConventionalCommit{},
		Breaking:      []git.ConventionalCommit{},
		Other:         []git.ConventionalCommit{},
		All:           []git.ConventionalCommit{},
	}

	for _, commit := range cs.Commits() {
		gitConventionalCommit := git.ConventionalCommit{
			Commit: git.Commit{
				Hash:    commit.Hash(),
				Message: commit.RawMessage(),
				Subject: commit.Subject(),
				Body:    commit.Body(),
			},
			Type:           git.CommitType(commit.Type()),
			Scope:          commit.Scope(),
			Description:    commit.Subject(),
			Body:           commit.Body(),
			Breaking:       commit.IsBreaking(),
			IsConventional: true,
		}

		// Add to All slice
		result.All = append(result.All, gitConventionalCommit)

		// Map commit type to category
		switch commit.Type() {
		case changes.CommitTypeFeat:
			result.Features = append(result.Features, gitConventionalCommit)
		case changes.CommitTypeFix:
			result.Fixes = append(result.Fixes, gitConventionalCommit)
		case changes.CommitTypeDocs:
			result.Documentation = append(result.Documentation, gitConventionalCommit)
		case changes.CommitTypePerf:
			result.Performance = append(result.Performance, gitConventionalCommit)
		case changes.CommitTypeRefactor:
			result.Refactoring = append(result.Refactoring, gitConventionalCommit)
		default:
			result.Other = append(result.Other, gitConventionalCommit)
		}

		// Also add to Breaking if it's a breaking change
		if commit.IsBreaking() {
			result.Breaking = append(result.Breaking, gitConventionalCommit)
		}
	}

	return result
}

// mapTone maps tone preset string to ai.Tone.
func (a *NotesGeneratorAdapter) mapTone(preset string) ai.Tone {
	switch preset {
	case "technical":
		return ai.ToneTechnical
	case "friendly", "casual":
		return ai.ToneFriendly
	case "professional", "formal":
		return ai.ToneProfessional
	case "excited", "marketing":
		return ai.ToneExcited
	default:
		return ai.ToneProfessional
	}
}

// mapAudience maps audience preset string to ai.Audience.
func (a *NotesGeneratorAdapter) mapAudience(preset string) ai.Audience {
	switch preset {
	case "developer", "developers":
		return ai.AudienceDevelopers
	case "user", "users":
		return ai.AudienceUsers
	case "public", "all":
		return ai.AudiencePublic
	case "marketing":
		return ai.AudienceMarketing
	default:
		return ai.AudienceDevelopers
	}
}

// PublisherAdapter adapts the plugin executor to the ports.Publisher interface.
type PublisherAdapter struct {
	executor   integration.PluginExecutor
	gitAdapter *git.Adapter
	tagCreator ports.TagCreator
	skipPush   bool // Skip pushing tags (useful for dry-run or local testing)
}

// PublisherAdapterOption configures the PublisherAdapter.
type PublisherAdapterOption func(*PublisherAdapter)

// WithSkipPush configures the PublisherAdapter to skip pushing tags.
func WithSkipPush(skip bool) PublisherAdapterOption {
	return func(a *PublisherAdapter) {
		a.skipPush = skip
	}
}

// NewPublisherAdapter creates a new PublisherAdapter.
func NewPublisherAdapter(executor integration.PluginExecutor, gitAdapter *git.Adapter, tagCreator ports.TagCreator, opts ...PublisherAdapterOption) *PublisherAdapter {
	a := &PublisherAdapter{
		executor:   executor,
		gitAdapter: gitAdapter,
		tagCreator: tagCreator,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// ExecuteStep executes a single step in the publishing plan.
func (a *PublisherAdapter) ExecuteStep(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (*ports.StepResult, error) {
	// Handle tag step specially - this is where tags are created during publish
	if step.Type == domain.StepTypeTag {
		return a.executeTagStep(ctx, run)
	}

	// For other steps, use the plugin executor
	if a.executor == nil {
		return nil, fmt.Errorf("no plugin executor configured")
	}

	// Build release context from run
	releaseCtx := a.buildReleaseContext(run)

	// Map step type to hook
	hook := a.mapStepTypeToHook(step.Type)

	// Execute the hook
	responses, err := a.executor.ExecuteHook(ctx, hook, releaseCtx)
	if err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Check if any plugin failed
	for _, resp := range responses {
		if !resp.Success {
			return &ports.StepResult{
				Success: false,
				Output:  resp.Message,
				Error:   fmt.Errorf("%s", resp.Error),
			}, nil
		}
	}

	// Collect outputs
	var output string
	for _, resp := range responses {
		if resp.Message != "" {
			output += resp.Message + "\n"
		}
	}

	return &ports.StepResult{
		Success: true,
		Output:  output,
	}, nil
}

// executeTagStep creates and pushes the git tag for the release.
func (a *PublisherAdapter) executeTagStep(ctx context.Context, run *domain.ReleaseRun) (*ports.StepResult, error) {
	if a.tagCreator == nil {
		return nil, fmt.Errorf("tag creator not configured")
	}

	tagName := run.TagName()
	if tagName == "" {
		tagName = "v" + run.VersionNext().String()
	}

	// Check if tag already exists (idempotency)
	exists, err := a.tagCreator.TagExists(ctx, tagName)
	if err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   fmt.Errorf("failed to check tag existence: %w", err),
		}, err
	}
	if exists {
		return &ports.StepResult{
			Success: true,
			Output:  fmt.Sprintf("Tag %s already exists (idempotent)", tagName),
		}, nil
	}

	// Create the annotated tag
	message := fmt.Sprintf("Release %s", run.VersionNext().String())
	if run.Notes() != nil && run.Notes().Text != "" {
		// Include a summary in the tag message
		message = fmt.Sprintf("Release %s\n\n%s", run.VersionNext().String(), run.Notes().Text)
	}

	if err := a.tagCreator.CreateTag(ctx, tagName, message); err != nil {
		return &ports.StepResult{
			Success: false,
			Error:   err,
		}, err
	}

	output := fmt.Sprintf("Created tag %s", tagName)

	// Push the tag unless skipPush is set
	if !a.skipPush {
		if err := a.tagCreator.PushTag(ctx, tagName, "origin"); err != nil {
			return &ports.StepResult{
				Success: false,
				Output:  output,
				Error:   fmt.Errorf("tag created but push failed: %w", err),
			}, err
		}
		output = fmt.Sprintf("Created and pushed tag %s", tagName)
	}

	return &ports.StepResult{
		Success: true,
		Output:  output,
	}, nil
}

// CheckIdempotency checks if a step has already been executed.
func (a *PublisherAdapter) CheckIdempotency(ctx context.Context, run *domain.ReleaseRun, step *domain.StepPlan) (bool, error) {
	// Check specific step types for idempotency
	switch step.Type {
	case domain.StepTypeTag:
		// Check if tag already exists
		if a.gitAdapter != nil {
			tagName := run.TagName()
			if tagName == "" {
				tagName = "v" + run.VersionNext().String()
			}
			// GetTag returns nil and error if tag doesn't exist
			tag, err := a.gitAdapter.GetTag(ctx, tagName)
			if err != nil {
				// Tag not found is expected, not an error
				return false, nil
			}
			// Tag exists if we got a non-nil tag
			return tag != nil, nil
		}
	}

	// Default: not idempotent check available
	return false, nil
}

// buildReleaseContext builds an integration.ReleaseContext from a ReleaseRun.
func (a *PublisherAdapter) buildReleaseContext(run *domain.ReleaseRun) integration.ReleaseContext {
	ctx := integration.ReleaseContext{
		Version:         run.VersionNext(),
		PreviousVersion: run.VersionCurrent(),
		ReleaseType:     changes.ReleaseType(run.BumpKind()),
		RepositoryPath:  run.RepoRoot(),
		TagName:         run.TagName(),
	}

	// Add notes if available
	if run.Notes() != nil {
		ctx.Changelog = run.Notes().Text
		ctx.ReleaseNotes = run.Notes().Text
	}

	// Add changeset if available
	if run.HasChangeSet() {
		ctx.Changes = run.ChangeSet()
	}

	return ctx
}

// mapStepTypeToHook maps a step type to an integration hook.
func (a *PublisherAdapter) mapStepTypeToHook(stepType domain.StepType) integration.Hook {
	switch stepType {
	case domain.StepTypeTag:
		return integration.HookPostVersion
	case domain.StepTypeBuild:
		return integration.HookPostVersion
	case domain.StepTypeArtifact:
		return integration.HookPostPublish
	case domain.StepTypeNotify:
		return integration.HookPostPublish
	case domain.StepTypePlugin:
		return integration.HookPostPublish
	case domain.StepTypeChangelog:
		return integration.HookPostNotes
	default:
		return integration.HookPostPublish
	}
}

// VersionWriterAdapter adapts git operations to the ports.VersionWriter interface.
type VersionWriterAdapter struct {
	gitAdapter *git.Adapter
	repoRoot   string
}

// NewVersionWriterAdapter creates a new VersionWriterAdapter.
func NewVersionWriterAdapter(gitAdapter *git.Adapter, repoRoot string) *VersionWriterAdapter {
	return &VersionWriterAdapter{
		gitAdapter: gitAdapter,
		repoRoot:   repoRoot,
	}
}

// WriteVersion writes the version to configured files.
func (a *VersionWriterAdapter) WriteVersion(ctx context.Context, ver version.SemanticVersion) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	// Write version to VERSION file if it exists
	// The actual file writing is typically handled by the version service
	// This adapter just ensures the git adapter can be used for commits if needed
	return nil
}

// WriteChangelog writes or updates the changelog file.
func (a *VersionWriterAdapter) WriteChangelog(ctx context.Context, ver version.SemanticVersion, notes string) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	// Changelog writing is typically handled during the publish step
	// This is a placeholder for the port interface
	return nil
}

// TagCreatorAdapter adapts git operations to the ports.TagCreator interface.
// It handles creating and pushing git tags during the publish step.
type TagCreatorAdapter struct {
	gitAdapter *git.Adapter
}

// NewTagCreatorAdapter creates a new TagCreatorAdapter.
func NewTagCreatorAdapter(gitAdapter *git.Adapter) *TagCreatorAdapter {
	return &TagCreatorAdapter{
		gitAdapter: gitAdapter,
	}
}

// CreateTag creates an annotated git tag with the given name and message.
func (a *TagCreatorAdapter) CreateTag(ctx context.Context, name, message string) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	// Get the HEAD commit to tag
	headCommit, err := a.gitAdapter.GetLatestCommit(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Create the tag at HEAD
	_, err = a.gitAdapter.CreateTag(ctx, name, headCommit.Hash(), message)
	if err != nil {
		return fmt.Errorf("failed to create tag %s: %w", name, err)
	}

	return nil
}

// PushTag pushes the specified tag to the remote repository.
func (a *TagCreatorAdapter) PushTag(ctx context.Context, name, remote string) error {
	if a.gitAdapter == nil {
		return fmt.Errorf("git adapter not configured")
	}

	if err := a.gitAdapter.PushTag(ctx, name, remote); err != nil {
		return fmt.Errorf("failed to push tag %s to %s: %w", name, remote, err)
	}

	return nil
}

// TagExists checks if a tag with the given name already exists.
func (a *TagCreatorAdapter) TagExists(ctx context.Context, name string) (bool, error) {
	if a.gitAdapter == nil {
		return false, fmt.Errorf("git adapter not configured")
	}

	tag, err := a.gitAdapter.GetTag(ctx, name)
	if err != nil {
		// Tag not found is expected, not an error
		return false, nil
	}

	return tag != nil, nil
}

// Helper function to create a pointer to a value.
func ptrTo[T any](v T) *T {
	return &v
}
