// Package versioning provides application use cases for version management.
package versioning

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// CalculateVersionInput represents input for the CalculateVersion use case.
type CalculateVersionInput struct {
	RepositoryPath string
	TagPrefix      string
	BumpType       version.BumpType
	Prerelease     version.Prerelease
	Auto           bool // Auto-detect bump type from commits
}

// CalculateVersionOutput represents output of the CalculateVersion use case.
type CalculateVersionOutput struct {
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	BumpType       version.BumpType
	AutoDetected   bool
}

// CalculateVersionUseCase calculates the next version.
type CalculateVersionUseCase struct {
	gitRepo     sourcecontrol.GitRepository
	versionCalc version.VersionCalculator
	logger      *slog.Logger
}

// NewCalculateVersionUseCase creates a new CalculateVersionUseCase.
func NewCalculateVersionUseCase(
	gitRepo sourcecontrol.GitRepository,
	versionCalc version.VersionCalculator,
) *CalculateVersionUseCase {
	return &CalculateVersionUseCase{
		gitRepo:     gitRepo,
		versionCalc: versionCalc,
		logger:      slog.Default().With("usecase", "calculate_version"),
	}
}

// Execute executes the calculate version use case.
func (uc *CalculateVersionUseCase) Execute(ctx context.Context, input CalculateVersionInput) (*CalculateVersionOutput, error) {
	tagPrefix := input.TagPrefix
	if tagPrefix == "" {
		tagPrefix = "v"
	}

	// Discover current version
	versionDiscovery := sourcecontrol.NewVersionDiscovery(tagPrefix)
	currentVersion, err := versionDiscovery.DiscoverCurrentVersion(ctx, uc.gitRepo)
	if err != nil {
		currentVersion = version.Initial
	}

	var bumpType version.BumpType
	autoDetected := false

	if input.Auto {
		// Auto-detect from commits
		latestTag, tagErr := uc.gitRepo.GetLatestVersionTag(ctx, tagPrefix)
		if tagErr != nil {
			// "Not found" is expected for repos with no tags yet - log at debug level
			uc.logger.Debug("no version tags found, will analyze all commits",
				"tag_prefix", tagPrefix,
				"error", tagErr)
		}

		var commits []*sourcecontrol.Commit
		if latestTag != nil {
			commits, err = uc.gitRepo.GetCommitsBetween(ctx, latestTag.Name(), "HEAD")
		} else {
			commits, err = uc.gitRepo.GetCommitsSince(ctx, "")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get commits: %w", err)
		}

		// Analyze commits
		hasBreaking := false
		hasFeature := false
		hasFix := false

		for _, commit := range commits {
			cc := changes.ParseConventionalCommit(string(commit.Hash()), commit.Message())
			if cc != nil {
				if cc.IsBreaking() {
					hasBreaking = true
				}
				switch cc.Type() {
				case changes.CommitTypeFeat:
					hasFeature = true
				case changes.CommitTypeFix:
					hasFix = true
				case changes.CommitTypeDocs, changes.CommitTypeStyle, changes.CommitTypeRefactor,
					changes.CommitTypePerf, changes.CommitTypeTest, changes.CommitTypeBuild,
					changes.CommitTypeCI, changes.CommitTypeChore, changes.CommitTypeRevert:
					// These commit types don't affect semver bump calculation
					// Breaking changes are detected separately via IsBreaking()
				}
			}
		}

		bumpType = uc.versionCalc.DetermineRequiredBump(hasBreaking, hasFeature, hasFix)
		autoDetected = true
	} else if input.BumpType.IsValid() {
		bumpType = input.BumpType
	} else {
		return nil, fmt.Errorf("bump type must be specified or auto-detection enabled")
	}

	// Calculate next version
	nextVersion := uc.versionCalc.CalculateNextVersion(currentVersion, bumpType)

	// Apply prerelease if specified
	if input.Prerelease != "" {
		nextVersion = nextVersion.WithPrerelease(input.Prerelease)
	}

	return &CalculateVersionOutput{
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		BumpType:       bumpType,
		AutoDetected:   autoDetected,
	}, nil
}

// SetVersionInput represents input for the SetVersion use case.
type SetVersionInput struct {
	Version    version.SemanticVersion
	TagPrefix  string
	CreateTag  bool
	PushTag    bool
	Remote     string
	TagMessage string
	DryRun     bool
}

// SetVersionOutput represents output of the SetVersion use case.
type SetVersionOutput struct {
	Version    version.SemanticVersion
	TagName    string
	TagCreated bool
	TagPushed  bool
}

// SetVersionUseCase sets a specific version.
type SetVersionUseCase struct {
	gitRepo sourcecontrol.GitRepository
}

// NewSetVersionUseCase creates a new SetVersionUseCase.
func NewSetVersionUseCase(gitRepo sourcecontrol.GitRepository) *SetVersionUseCase {
	return &SetVersionUseCase{gitRepo: gitRepo}
}

// Execute sets the version and optionally creates/pushes a tag.
func (uc *SetVersionUseCase) Execute(ctx context.Context, input SetVersionInput) (*SetVersionOutput, error) {
	tagPrefix := input.TagPrefix
	if tagPrefix == "" {
		tagPrefix = "v"
	}
	tagName := tagPrefix + input.Version.String()

	output := &SetVersionOutput{
		Version: input.Version,
		TagName: tagName,
	}

	if input.DryRun {
		return output, nil
	}

	if input.CreateTag {
		// Check if tag already exists (common in CI when triggered by tag push)
		existingTag, _ := uc.gitRepo.GetTag(ctx, tagName)
		if existingTag != nil {
			// Tag exists, skip creation but still push if requested
			output.TagCreated = false
			if input.PushTag {
				remote := input.Remote
				if remote == "" {
					remote = "origin"
				}
				if err := uc.gitRepo.PushTag(ctx, tagName, remote); err != nil {
					return nil, fmt.Errorf("failed to push tag: %w", err)
				}
				output.TagPushed = true
			}
			return output, nil
		}

		// Get latest commit
		repoInfo, err := uc.gitRepo.GetInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get repo info: %w", err)
		}

		latestCommit, err := uc.gitRepo.GetLatestCommit(ctx, repoInfo.CurrentBranch)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest commit: %w", err)
		}

		tagMsg := input.TagMessage
		if tagMsg == "" {
			tagMsg = fmt.Sprintf("Release %s", input.Version.String())
		}

		_, err = uc.gitRepo.CreateTag(ctx, tagName, latestCommit.Hash(), tagMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to create tag: %w", err)
		}
		output.TagCreated = true

		if input.PushTag {
			remote := input.Remote
			if remote == "" {
				remote = "origin"
			}
			if err := uc.gitRepo.PushTag(ctx, tagName, remote); err != nil {
				return nil, fmt.Errorf("failed to push tag: %w", err)
			}
			output.TagPushed = true
		}
	}

	return output, nil
}
