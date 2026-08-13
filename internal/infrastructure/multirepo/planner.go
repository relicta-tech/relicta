package multirepo

import (
	"context"
	"fmt"
	"strings"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// Planner answers what each member repository would release next.
//
// The coordinator consults a ReleaseExecutor for this, and the CLI passed nil — so
// `relicta group plan` reported which repositories had changes and how many, with the NEXT
// column blank. That is the column the command exists for: "what would this group release"
// is the question, and the answer was missing while the arithmetic to produce it already
// existed for the single-repository path.
//
// Only Plan is implemented. Release runs the full pipeline — bump, notes, approval,
// publish — inside another checkout, with that repository's own config, plugins and approval
// state, and it raises a question this type cannot answer on its own: whether a group release
// may auto-approve a member whose policy demands a human. Coordinator.Execute refuses clearly
// while that remains undecided, which is better than a plausible guess about somebody's
// governance.
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
	svc, err := git.NewService(git.WithRepoPath(repoPath))
	if err != nil {
		return nil, fmt.Errorf("opening repository %s: %w", repoPath, err)
	}

	current := version.SemanticVersion{}
	ref := ""
	if tag, tagErr := svc.GetLatestVersionTag(ctx, p.tagPrefix); tagErr == nil && tag != nil {
		ref = tag.Name
		// A tag that does not parse leaves current at 0.0.0 rather than failing the whole
		// group: one member with an odd tag must not stop the others being planned.
		if parsed, parseErr := version.Parse(strings.TrimPrefix(tag.Name, p.tagPrefix)); parseErr == nil {
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

// Release is not implemented. See the type comment: running the pipeline inside another
// checkout needs a decision about approval that this type cannot make.
func (p *Planner) Release(_ context.Context, repoPath string) (*appmultirepo.RepoResult, error) {
	return nil, fmt.Errorf("releasing %s from a group is not implemented: a group release has "+
		"to decide whether it may approve on behalf of a member whose policy requires a human, "+
		"and that decision is not made yet", repoPath)
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
