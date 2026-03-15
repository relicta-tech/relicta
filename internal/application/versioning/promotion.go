// Package versioning provides application use cases for version management.
package versioning

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// PromoteReleaseInput represents input for the PromoteRelease use case.
type PromoteReleaseInput struct {
	// TagPrefix is the prefix for version tags (default: "v").
	TagPrefix string
	// FromChannel is the source channel name.
	FromChannel string
	// ToChannel is the target channel name.
	ToChannel string
	// Version is the specific version to promote. If empty, the latest version
	// on the source channel is used.
	Version string
	// DryRun indicates whether to simulate without making changes.
	DryRun bool
}

// PromoteReleaseOutput represents output of the PromoteRelease use case.
type PromoteReleaseOutput struct {
	// SourceVersion is the version being promoted from.
	SourceVersion version.SemanticVersion
	// TargetVersion is the new version after promotion.
	TargetVersion version.SemanticVersion
	// FromChannel is the source channel name.
	FromChannel string
	// ToChannel is the target channel name.
	ToChannel string
	// PromotionPath is the full promotion path taken.
	PromotionPath []string
	// PromotedAt is the timestamp of the promotion.
	PromotedAt time.Time
}

// PromoteReleaseUseCase handles promoting releases between channels.
type PromoteReleaseUseCase struct {
	gitRepo  sourcecontrol.GitRepository
	registry *version.ChannelRegistry
	logger   *slog.Logger
}

// NewPromoteReleaseUseCase creates a new PromoteReleaseUseCase.
func NewPromoteReleaseUseCase(
	gitRepo sourcecontrol.GitRepository,
	registry *version.ChannelRegistry,
) *PromoteReleaseUseCase {
	if registry == nil {
		registry = version.NewChannelRegistry()
	}
	return &PromoteReleaseUseCase{
		gitRepo:  gitRepo,
		registry: registry,
		logger:   slog.Default().With("usecase", "promote_release"),
	}
}

// Execute promotes a release from one channel to another.
// The promotion creates a new tag on the same commit with the target channel's
// prerelease identifier.
func (uc *PromoteReleaseUseCase) Execute(ctx context.Context, input PromoteReleaseInput) (*PromoteReleaseOutput, error) {
	tagPrefix := input.TagPrefix
	if tagPrefix == "" {
		tagPrefix = "v"
	}

	// Validate channels
	fromCh, err := uc.registry.Get(input.FromChannel)
	if err != nil {
		return nil, fmt.Errorf("invalid source channel: %w", err)
	}

	toCh, err := uc.registry.Get(input.ToChannel)
	if err != nil {
		return nil, fmt.Errorf("invalid target channel: %w", err)
	}

	// Validate the promotion is allowed
	if err := fromCh.ValidatePromotion(toCh); err != nil {
		return nil, fmt.Errorf("invalid promotion: %w", err)
	}

	// Resolve source version
	var sourceVersion version.SemanticVersion
	if input.Version != "" {
		sourceVersion, err = version.Parse(input.Version)
		if err != nil {
			return nil, fmt.Errorf("invalid version: %w", err)
		}
	} else {
		// Find latest version on the source channel
		sourceVersion, err = uc.findLatestChannelVersion(ctx, tagPrefix, fromCh)
		if err != nil {
			return nil, fmt.Errorf("failed to find latest version on channel %q: %w", input.FromChannel, err)
		}
	}

	uc.logger.Info("promoting release",
		"from_channel", input.FromChannel,
		"to_channel", input.ToChannel,
		"source_version", sourceVersion.String(),
	)

	// Calculate target version
	var targetVersion version.SemanticVersion
	if toCh.IsStableChannel() {
		// Promoting to stable: strip prerelease suffix
		targetVersion = sourceVersion.PromoteToRelease()
	} else {
		// Promoting to another prerelease channel: keep the same major.minor.patch
		// and start with the target channel's prerelease at .1
		targetVersion = version.NewSemanticVersionWithPrerelease(
			sourceVersion.Major(),
			sourceVersion.Minor(),
			sourceVersion.Patch(),
			version.Prerelease(fmt.Sprintf("%s.%d", toCh.PrereleaseID(), 1)),
		)
	}

	output := &PromoteReleaseOutput{
		SourceVersion: sourceVersion,
		TargetVersion: targetVersion,
		FromChannel:   input.FromChannel,
		ToChannel:     input.ToChannel,
		PromotionPath: []string{input.FromChannel, input.ToChannel},
		PromotedAt:    time.Now(),
	}

	if input.DryRun {
		uc.logger.Info("dry run: would promote",
			"source", sourceVersion.String(),
			"target", targetVersion.String(),
		)
		return output, nil
	}

	// Find the commit hash for the source version tag
	sourceTagName := tagPrefix + sourceVersion.String()
	sourceTag, err := uc.gitRepo.GetTag(ctx, sourceTagName)
	if err != nil {
		return nil, fmt.Errorf("failed to find source tag %q: %w", sourceTagName, err)
	}

	// Create new tag on the same commit
	targetTagName := tagPrefix + targetVersion.String()
	tagMsg := fmt.Sprintf("Promote %s from %s to %s", sourceVersion.String(), input.FromChannel, input.ToChannel)

	_, err = uc.gitRepo.CreateTag(ctx, targetTagName, sourceTag.Hash(), tagMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to create target tag %q: %w", targetTagName, err)
	}

	uc.logger.Info("promotion complete",
		"source_tag", sourceTagName,
		"target_tag", targetTagName,
	)

	return output, nil
}

// findLatestChannelVersion finds the latest version tag that matches the given channel.
func (uc *PromoteReleaseUseCase) findLatestChannelVersion(ctx context.Context, tagPrefix string, ch version.Channel) (version.SemanticVersion, error) {
	tags, err := uc.gitRepo.GetTags(ctx)
	if err != nil {
		return version.Zero, fmt.Errorf("failed to get tags: %w", err)
	}

	var latest *version.SemanticVersion
	for _, t := range tags.FilterByPrefix(tagPrefix).VersionTags() {
		ver := t.Version()
		if ver == nil {
			continue
		}

		// Match channel by prerelease type
		if ch.IsStableChannel() {
			if ver.IsPrerelease() {
				continue
			}
		} else {
			if ver.PrereleaseType() != ch.PrereleaseID() {
				continue
			}
		}

		if latest == nil || ver.GreaterThan(*latest) {
			v := *ver
			latest = &v
		}
	}

	if latest == nil {
		return version.Zero, fmt.Errorf("no versions found for channel %q", ch.Name())
	}

	return *latest, nil
}
