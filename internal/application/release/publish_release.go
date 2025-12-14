// Package release provides application use cases for release management.
package release

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/integration"
	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
	"github.com/felixgeelhaar/release-pilot/internal/domain/sourcecontrol"
)

// PublishReleaseInput represents the input for the PublishRelease use case.
type PublishReleaseInput struct {
	ReleaseID release.ReleaseID
	DryRun    bool
	CreateTag bool
	PushTag   bool
	TagPrefix string
	Remote    string
}

// Validate validates the PublishReleaseInput.
func (i *PublishReleaseInput) Validate() error {
	// Release ID is required
	if i.ReleaseID == "" {
		return fmt.Errorf("release ID is required")
	}

	// Tag prefix validation
	if i.TagPrefix != "" {
		if len(i.TagPrefix) > 32 {
			return fmt.Errorf("tag prefix too long (max 32 characters): %s", i.TagPrefix)
		}
		if strings.ContainsAny(i.TagPrefix, "~^:?*[\\ ") {
			return fmt.Errorf("tag prefix contains invalid characters: %s", i.TagPrefix)
		}
	}

	// Remote name validation
	if i.Remote != "" {
		if len(i.Remote) > 256 {
			return fmt.Errorf("remote name too long (max 256 characters): %s", i.Remote)
		}
		if strings.ContainsAny(i.Remote, " \t\n") {
			return fmt.Errorf("remote name contains whitespace: %s", i.Remote)
		}
	}

	return nil
}

// PublishReleaseOutput represents the output of the PublishRelease use case.
type PublishReleaseOutput struct {
	TagName       string
	ReleaseURL    string
	PluginResults []PluginResult
}

// PluginResult represents the result of a plugin execution.
type PluginResult struct {
	PluginName string
	Hook       integration.Hook
	Success    bool
	Message    string
	Duration   time.Duration
}

// PublishReleaseUseCase implements the publish release use case.
type PublishReleaseUseCase struct {
	releaseRepo       release.Repository
	unitOfWorkFactory release.UnitOfWorkFactory
	gitRepo           sourcecontrol.GitRepository
	pluginExecutor    integration.PluginExecutor
	eventPublisher    release.EventPublisher
	logger            *slog.Logger
}

// NewPublishReleaseUseCase creates a new PublishReleaseUseCase.
func NewPublishReleaseUseCase(
	releaseRepo release.Repository,
	gitRepo sourcecontrol.GitRepository,
	pluginExecutor integration.PluginExecutor,
	eventPublisher release.EventPublisher,
) *PublishReleaseUseCase {
	return &PublishReleaseUseCase{
		releaseRepo:    releaseRepo,
		gitRepo:        gitRepo,
		pluginExecutor: pluginExecutor,
		eventPublisher: eventPublisher,
		logger:         slog.Default().With("usecase", "publish_release"),
	}
}

// NewPublishReleaseUseCaseWithUoW creates a new PublishReleaseUseCase with UnitOfWork support.
func NewPublishReleaseUseCaseWithUoW(
	unitOfWorkFactory release.UnitOfWorkFactory,
	gitRepo sourcecontrol.GitRepository,
	pluginExecutor integration.PluginExecutor,
	eventPublisher release.EventPublisher,
) *PublishReleaseUseCase {
	return &PublishReleaseUseCase{
		unitOfWorkFactory: unitOfWorkFactory,
		gitRepo:           gitRepo,
		pluginExecutor:    pluginExecutor,
		eventPublisher:    eventPublisher,
		logger:            slog.Default().With("usecase", "publish_release"),
	}
}

// Execute executes the publish release use case.
func (uc *PublishReleaseUseCase) Execute(ctx context.Context, input PublishReleaseInput) (*PublishReleaseOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Use UnitOfWork if available for transactional consistency
	if uc.unitOfWorkFactory != nil {
		return uc.executeWithUnitOfWork(ctx, input)
	}

	// Legacy path without UnitOfWork
	return uc.executeWithoutUnitOfWork(ctx, input)
}

// executeWithUnitOfWork executes the use case with transactional boundaries.
func (uc *PublishReleaseUseCase) executeWithUnitOfWork(ctx context.Context, input PublishReleaseInput) (*PublishReleaseOutput, error) {
	// Begin transaction via factory
	uow, err := uc.unitOfWorkFactory.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// Rollback if not committed (no-op if already committed)
		_ = uow.Rollback()
	}()

	// Get repository from UnitOfWork
	repo := uow.ReleaseRepository()

	rel, err := repo.FindByID(ctx, input.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	if !rel.CanProceedToPublish() {
		return nil, fmt.Errorf("release is not ready for publishing: current state is %s", rel.State())
	}

	tagName := uc.buildTagName(input.TagPrefix, rel.Plan().NextVersion.String())
	output := &PublishReleaseOutput{
		TagName:       tagName,
		PluginResults: make([]PluginResult, 0),
	}

	releaseCtx := uc.buildReleaseContext(rel, tagName, input.DryRun)

	if err := uc.executePrePublishPhase(ctx, rel, releaseCtx, output); err != nil {
		return nil, err
	}

	if err := uc.executeGitTagPhase(ctx, rel, tagName, input); err != nil {
		return nil, err
	}

	uc.executePostPublishPhase(ctx, rel, releaseCtx, tagName, output)

	if err := uc.finalizePublishWithUoW(ctx, rel, repo, releaseCtx, tagName, input.DryRun, output); err != nil {
		return nil, err
	}

	// Commit transaction
	if err := uow.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return output, nil
}

// executeWithoutUnitOfWork executes the use case without transactional boundaries (legacy).
func (uc *PublishReleaseUseCase) executeWithoutUnitOfWork(ctx context.Context, input PublishReleaseInput) (*PublishReleaseOutput, error) {
	rel, err := uc.releaseRepo.FindByID(ctx, input.ReleaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	if !rel.CanProceedToPublish() {
		return nil, fmt.Errorf("release is not ready for publishing: current state is %s", rel.State())
	}

	tagName := uc.buildTagName(input.TagPrefix, rel.Plan().NextVersion.String())
	output := &PublishReleaseOutput{
		TagName:       tagName,
		PluginResults: make([]PluginResult, 0),
	}

	releaseCtx := uc.buildReleaseContext(rel, tagName, input.DryRun)

	if err := uc.executePrePublishPhase(ctx, rel, releaseCtx, output); err != nil {
		return nil, err
	}

	if err := uc.executeGitTagPhase(ctx, rel, tagName, input); err != nil {
		return nil, err
	}

	uc.executePostPublishPhase(ctx, rel, releaseCtx, tagName, output)

	if err := uc.finalizePublish(ctx, rel, releaseCtx, tagName, input.DryRun, output); err != nil {
		return nil, err
	}

	return output, nil
}

// buildTagName constructs the tag name from prefix and version.
func (uc *PublishReleaseUseCase) buildTagName(prefix, version string) string {
	if prefix == "" {
		prefix = "v"
	}
	return prefix + version
}

// buildReleaseContext creates the integration context for plugins.
func (uc *PublishReleaseUseCase) buildReleaseContext(rel *release.Release, tagName string, dryRun bool) integration.ReleaseContext {
	plan := rel.Plan()
	ctx := integration.ReleaseContext{
		Version:         plan.NextVersion,
		PreviousVersion: plan.CurrentVersion,
		ReleaseType:     plan.ReleaseType,
		RepositoryName:  rel.RepositoryName(),
		RepositoryPath:  rel.RepositoryPath(),
		Branch:          rel.Branch(),
		TagName:         tagName,
		Changes:         plan.GetChangeSet(),
		DryRun:          dryRun,
		Timestamp:       time.Now(),
	}

	if rel.Notes() != nil {
		ctx.Changelog = rel.Notes().Changelog
		ctx.ReleaseNotes = rel.Notes().Summary
	}

	return ctx
}

// executePrePublishPhase runs pre-publish hooks and starts publishing.
func (uc *PublishReleaseUseCase) executePrePublishPhase(
	ctx context.Context,
	rel *release.Release,
	releaseCtx integration.ReleaseContext,
	output *PublishReleaseOutput,
) error {
	if uc.pluginExecutor != nil {
		preResults, err := uc.executeHook(ctx, rel, integration.HookPrePublish, releaseCtx)
		if err != nil {
			return fmt.Errorf("pre-publish hook failed: %w", err)
		}
		output.PluginResults = append(output.PluginResults, preResults...)
	}

	var pluginNames []string
	if err := rel.StartPublishing(pluginNames); err != nil {
		return fmt.Errorf("failed to start publishing: %w", err)
	}

	return nil
}

// executeGitTagPhase creates and optionally pushes the git tag.
func (uc *PublishReleaseUseCase) executeGitTagPhase(
	ctx context.Context,
	rel *release.Release,
	tagName string,
	input PublishReleaseInput,
) error {
	if !input.CreateTag || input.DryRun {
		return nil
	}

	// Check if tag already exists (may have been created by bump command)
	existingTag, _ := uc.gitRepo.GetTag(ctx, tagName)
	if existingTag != nil {
		uc.logger.Info("tag already exists, skipping creation",
			"tag", tagName,
			"release_id", rel.ID())
		// Tag exists, just push if needed
		if input.PushTag {
			if err := uc.pushTag(ctx, rel, tagName, input.Remote); err != nil {
				return err
			}
		}
		return nil
	}

	latestCommit, err := uc.gitRepo.GetLatestCommit(ctx, rel.Branch())
	if err != nil {
		uc.markReleaseFailed(rel, fmt.Sprintf("failed to get latest commit: %v", err))
		return fmt.Errorf("failed to get latest commit: %w", err)
	}

	tagMessage := uc.buildTagMessage(rel)
	if _, err = uc.gitRepo.CreateTag(ctx, tagName, latestCommit.Hash(), tagMessage); err != nil {
		uc.markReleaseFailed(rel, fmt.Sprintf("failed to create tag: %v", err))
		return fmt.Errorf("failed to create tag: %w", err)
	}

	if input.PushTag {
		if err := uc.pushTag(ctx, rel, tagName, input.Remote); err != nil {
			return err
		}
	}

	return nil
}

// buildTagMessage creates the tag message from release notes or default.
func (uc *PublishReleaseUseCase) buildTagMessage(rel *release.Release) string {
	if rel.Notes() != nil && rel.Notes().Summary != "" {
		return rel.Notes().Summary
	}
	return fmt.Sprintf("Release %s", rel.Plan().NextVersion.String())
}

// pushTag pushes the tag to the remote repository.
func (uc *PublishReleaseUseCase) pushTag(ctx context.Context, rel *release.Release, tagName, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	if err := uc.gitRepo.PushTag(ctx, tagName, remote); err != nil {
		uc.markReleaseFailed(rel, fmt.Sprintf("failed to push tag: %v", err))
		return fmt.Errorf("failed to push tag: %w", err)
	}
	return nil
}

// markReleaseFailed marks the release as failed and logs any errors.
func (uc *PublishReleaseUseCase) markReleaseFailed(rel *release.Release, reason string) {
	if markErr := rel.MarkFailed(reason, true); markErr != nil {
		uc.logger.Warn("failed to mark release as failed",
			"error", markErr,
			"release_id", rel.ID(),
			"reason", reason)
	}
}

// executePostPublishPhase runs post-publish hooks (errors are non-fatal).
func (uc *PublishReleaseUseCase) executePostPublishPhase(
	ctx context.Context,
	rel *release.Release,
	releaseCtx integration.ReleaseContext,
	tagName string,
	output *PublishReleaseOutput,
) {
	if uc.pluginExecutor == nil {
		return
	}

	postResults, err := uc.executeHook(ctx, rel, integration.HookPostPublish, releaseCtx)
	if err != nil {
		uc.logger.Warn("post-publish plugin hook failed",
			"error", err,
			"release_id", rel.ID(),
			"tag", tagName)
	}
	output.PluginResults = append(output.PluginResults, postResults...)
}

// finalizePublish marks release as published and executes success hooks.
func (uc *PublishReleaseUseCase) finalizePublish(
	ctx context.Context,
	rel *release.Release,
	releaseCtx integration.ReleaseContext,
	tagName string,
	dryRun bool,
	output *PublishReleaseOutput,
) error {
	if dryRun {
		return nil
	}

	if err := rel.MarkPublished(output.ReleaseURL); err != nil {
		return fmt.Errorf("failed to mark release as published: %w", err)
	}

	uc.executeSuccessHooks(ctx, rel, releaseCtx, tagName, output)

	if err := uc.releaseRepo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	uc.publishDomainEvents(ctx, rel)

	return nil
}

// finalizePublishWithUoW marks release as published using UnitOfWork repository.
// Events are collected by the UoW and published on commit.
func (uc *PublishReleaseUseCase) finalizePublishWithUoW(
	ctx context.Context,
	rel *release.Release,
	repo release.Repository,
	releaseCtx integration.ReleaseContext,
	tagName string,
	dryRun bool,
	output *PublishReleaseOutput,
) error {
	if dryRun {
		return nil
	}

	if err := rel.MarkPublished(output.ReleaseURL); err != nil {
		return fmt.Errorf("failed to mark release as published: %w", err)
	}

	uc.executeSuccessHooks(ctx, rel, releaseCtx, tagName, output)

	// Save through UoW repository - events will be collected and published on commit
	if err := repo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	return nil
}

// executeSuccessHooks runs on-success hooks (errors are non-fatal).
func (uc *PublishReleaseUseCase) executeSuccessHooks(
	ctx context.Context,
	rel *release.Release,
	releaseCtx integration.ReleaseContext,
	tagName string,
	output *PublishReleaseOutput,
) {
	if uc.pluginExecutor == nil {
		return
	}

	successResults, err := uc.executeHook(ctx, rel, integration.HookOnSuccess, releaseCtx)
	if err != nil {
		uc.logger.Warn("on-success plugin hook failed",
			"error", err,
			"release_id", rel.ID(),
			"tag", tagName)
	}
	output.PluginResults = append(output.PluginResults, successResults...)
}

// publishDomainEvents publishes domain events (errors are non-fatal).
func (uc *PublishReleaseUseCase) publishDomainEvents(ctx context.Context, rel *release.Release) {
	if uc.eventPublisher == nil {
		return
	}

	if err := uc.eventPublisher.Publish(ctx, rel.DomainEvents()...); err != nil {
		uc.logger.Warn("failed to publish domain events",
			"error", err,
			"release_id", rel.ID())
	}
	rel.ClearDomainEvents()
}

// executeHook executes plugins for a hook and records results.
func (uc *PublishReleaseUseCase) executeHook(
	ctx context.Context,
	rel *release.Release,
	hook integration.Hook,
	releaseCtx integration.ReleaseContext,
) ([]PluginResult, error) {
	start := time.Now()
	responses, err := uc.pluginExecutor.ExecuteHook(ctx, hook, releaseCtx)

	results := make([]PluginResult, 0, len(responses))
	for i, resp := range responses {
		result := PluginResult{
			PluginName: fmt.Sprintf("plugin-%d", i),
			Hook:       hook,
			Success:    resp.Success,
			Message:    resp.Message,
			Duration:   time.Since(start),
		}
		results = append(results, result)

		// Record in release
		rel.RecordPluginExecution(result.PluginName, string(hook), resp.Success, resp.Message, result.Duration)
	}

	return results, err
}
