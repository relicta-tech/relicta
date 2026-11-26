package multirepo

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v5/internal/config"
	rperrors "github.com/relicta-tech/relicta/v5/internal/errors"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/git"
)

// effectiveTagPrefix returns the member repository's configured prefix when it
// has a Relicta config. Repositories without one inherit the group-level value.
func effectiveTagPrefix(repoPath, fallback string) (string, error) {
	if fallback == "" {
		fallback = "v"
	}
	if !config.ConfigExists(repoPath) {
		return fallback, nil
	}
	cfg, err := config.LoadFromDirectory(repoPath)
	if err != nil {
		return "", err
	}
	if cfg == nil || cfg.Versioning.TagPrefix == "" {
		return fallback, nil
	}
	return cfg.Versioning.TagPrefix, nil
}

// latestVersionTag treats only the expected missing-tag condition as an
// unreleased repository. Cancellation and repository read failures must stop
// planning instead of being mistaken for an empty release history.
func latestVersionTag(ctx context.Context, svc git.Service, prefix string) (*git.Tag, error) {
	tag, err := svc.GetLatestVersionTag(ctx, prefix)
	if err != nil && !rperrors.IsKind(err, rperrors.KindNotFound) {
		return nil, fmt.Errorf("reading latest version tag: %w", err)
	}
	return tag, nil
}
