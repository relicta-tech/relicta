package multirepo

import (
	"context"
	"fmt"
	"strings"

	appmultirepo "github.com/relicta-tech/relicta/v5/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v5/internal/domain/changes"
	"github.com/relicta-tech/relicta/v5/internal/domain/version"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/git"
)

// Planner answers what each member repository would release next. The group executor owns
// actual publication because it must build release services with that member's configuration.
//
// This adapter supplies the deterministic next-version estimate used in group plans. It does
// not perform a release; groupExecutor performs publication of an already-approved run inside
// the member repository's own configuration and checkout.
type Planner struct {
	tagPrefix string
}

// NewPlanner returns a planner that reads each repository at its configured path.
func NewPlanner(tagPrefix string) *Planner {
	if tagPrefix == "" {
		tagPrefix = "v"
	}
	return &Planner{tagPrefix: tagPrefix}
}

var _ appmultirepo.ReleaseExecutor = (*Planner)(nil)

// Plan reports the version repoPath would release next, derived from its conventional
// commits since its last version tag — the same rule the single-repository path applies.
func (p *Planner) Plan(ctx context.Context, repoPath string) (*appmultirepo.RepoResult, error) {
	prefix, err := effectiveTagPrefix(repoPath, p.tagPrefix)
	if err != nil {
		return nil, fmt.Errorf("loading version settings for %s: %w", repoPath, err)
	}
	svc, err := git.NewService(git.WithRepoPath(repoPath))
	if err != nil {
		return nil, fmt.Errorf("opening repository %s: %w", repoPath, err)
	}

	current := version.SemanticVersion{}
	ref := ""
	tag, err := latestVersionTag(ctx, svc, prefix)
	if err != nil {
		return nil, fmt.Errorf("reading latest version tag in %s: %w", repoPath, err)
	}
	if tag != nil {
		ref = tag.Name
		// A tag that does not parse leaves current at 0.0.0 rather than failing the whole
		// group: one member with an odd tag must not stop the others being planned.
		if parsed, parseErr := version.Parse(strings.TrimPrefix(tag.Name, prefix)); parseErr == nil {
			current = parsed
		}
	}

	commits, err := svc.GetCommitsSince(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("reading commits in %s: %w", repoPath, err)
	}

	conventional, err := svc.ParseConventionalCommits(commits, git.ParseOptions{})
	if err != nil {
		return nil, fmt.Errorf("parsing commits in %s: %w", repoPath, err)
	}

	releaseType := svc.DetectReleaseType(conventional)
	result := &appmultirepo.RepoResult{
		CurrentVersion: current.String(),
		ChangeCount:    len(commits),
		HasChanges:     len(commits) > 0,
	}

	// No conventional commits worth releasing means no next version, rather than a version
	// bumped for changes that do not warrant one. The coordinator already skips a
	// repository with no changes at all; this covers the one whose changes are all chores.
	if releaseType == changes.ReleaseTypeNone {
		return result, nil
	}

	result.NextVersion = bumpFor(releaseType).Apply(current).String()
	return result, nil
}

// Release is intentionally unsupported here. The container-level groupExecutor is the only
// component with enough context to publish a member's already-approved run safely.
func (p *Planner) Release(_ context.Context, repoPath string) (*appmultirepo.RepoResult, error) {
	return nil, fmt.Errorf("releasing %s requires an approved Relicta run; the plan-only multirepository planner cannot load or publish that approval", repoPath)
}

// bumpFor maps a detected release type onto the version bump it implies.
func bumpFor(t changes.ReleaseType) version.VersionBump {
	switch t {
	case changes.ReleaseTypeMajor:
		return version.NewVersionBump(version.BumpMajor)
	case changes.ReleaseTypeMinor:
		return version.NewVersionBump(version.BumpMinor)
	default:
		return version.NewVersionBump(version.BumpPatch)
	}
}
