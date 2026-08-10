// Package app provides application services (use cases) for release governance.
package app

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// PlanReleaseInput contains the input for planning a release.
// tagPrefixOrDefault returns the configured prefix, defaulting to "v" — the same
// default config declares — so a caller that does not set it keeps the previous
// behavior rather than searching for bare semver tags.
func (in PlanReleaseInput) tagPrefixOrDefault() string {
	if in.TagPrefix == "" {
		return "v"
	}
	return in.TagPrefix
}

type PlanReleaseInput struct {
	RepoRoot       string
	RepoID         string
	BaseRef        string // Base reference (tag or commit) - if empty, auto-detect
	ConfigHash     string // Hash of the config snapshot
	PluginPlanHash string // Hash of the plugin configuration
	Actor          ports.ActorInfo
	Force          bool // Force planning even if there's an active run

	// DiscardExisting allows re-planning to overwrite a run that already exists
	// for these exact inputs, throwing away whatever state it had reached.
	//
	// Separate from Force on purpose. Force means "supersede stale runs because
	// HEAD moved" (issue #128) and the CLI sets it unconditionally; reusing it
	// here would make every plan destructive again. This one answers a different
	// question — "I know there is an approved release for these commits, discard
	// it" — and is wired to an explicit `relicta plan --force`.
	DiscardExisting bool

	// Optional pre-computed data from commit analysis
	// If provided, these bypass the basic commit resolution and enable full release planning
	ChangeSet      *changes.ChangeSet       // Pre-computed changeset from analysis
	CurrentVersion *version.SemanticVersion // Current version
	NextVersion    *version.SemanticVersion // Proposed next version
	BumpKind       *domain.BumpKind         // Determined bump type (major/minor/patch)
	Confidence     float64                  // Version calculation confidence (0.0-1.0)

	// Tag-push mode: when HEAD is already tagged, skip directly to versioned state
	// This enables notes/approve/publish without running bump
	TagPushMode bool   // If true, transition directly to versioned state
	TagName     string // The existing tag name (required if TagPushMode is true)

	// TagPrefix is the configured prefix for version tags.
	//
	// Baseline detection hardcoded "v", so a project configuring anything else got
	// a baseline of "no previous release" and a changeset spanning its whole
	// history — silently, since "no tags found" and "no tags with this prefix" were
	// indistinguishable. Empty means the default, "v".
	TagPrefix string
}

// PlanReleaseOutput contains the output from planning a release.
type PlanReleaseOutput struct {
	RunID          domain.RunID
	HeadSHA        domain.CommitSHA
	Commits        []domain.CommitSHA
	PlanHash       string
	CurrentVersion version.SemanticVersion
	VersionNext    version.SemanticVersion
	BumpKind       domain.BumpKind
	RiskScore      float64
	ChangeSet      *changes.ChangeSet

	// AlreadyExisted reports that a run for these exact inputs was already
	// present and was returned untouched, rather than a new plan being created.
	// Callers should say so: reporting "plan saved" when nothing was written
	// hides that an in-progress release is waiting.
	AlreadyExisted bool

	// ExistingState is the state that run had reached. Only meaningful when
	// AlreadyExisted is true.
	ExistingState domain.RunState

	// FirstRelease reports that no previous release was found, so the changeset is
	// the whole history rather than the commits since a tag.
	//
	// Reported because it changes what the numbers mean. A first release's commit
	// count and version are derived from everything in the repository, and a caller
	// that cannot tell this apart from an ordinary release reads a large changeset
	// as unusual activity rather than as the baseline.
	FirstRelease bool
}

// PlanReleaseUseCase handles the plan release use case.
type PlanReleaseUseCase struct {
	repo          ports.ReleaseRunRepository
	repoInspector ports.RepoInspector
	stateMachine  *domain.StateMachineService
}

// NewPlanReleaseUseCase creates a new PlanReleaseUseCase.
func NewPlanReleaseUseCase(
	repo ports.ReleaseRunRepository,
	repoInspector ports.RepoInspector,
	stateMachine *domain.StateMachineService,
) *PlanReleaseUseCase {
	return &PlanReleaseUseCase{
		repo:          repo,
		repoInspector: repoInspector,
		stateMachine:  stateMachine,
	}
}

// Execute plans a new release.
func (uc *PlanReleaseUseCase) Execute(ctx context.Context, input PlanReleaseInput) (*PlanReleaseOutput, error) {
	// Validate Actor ID for audit trail
	if input.Actor.ID == "" {
		return nil, ErrActorIDRequired
	}

	// Validate tag-push mode requirements
	if input.TagPushMode && input.NextVersion == nil {
		return nil, ErrTagPushMissingVersion
	}

	// Get current HEAD SHA
	headSHA, err := uc.repoInspector.HeadSHA(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD SHA: %w", err)
	}

	// Get base ref if not provided.
	//
	// An empty baseRef means "no previous release", and ResolveCommits reads it as
	// "everything up to HEAD" — the first release. The comment here used to promise
	// "initial commit or empty" and only the empty half existed, which was fine
	// until it reached git: GetCommitsBetween resolved "" as a reference and failed,
	// so planning was impossible in a repository with no version tags. Fixed in the
	// git service, which now treats an empty from as all of history, matching what
	// GetCommitsSince has always done.
	//
	// Two situations produce no tag and both are legitimately a first release: a
	// repository with no tags at all, and one whose tags all carry a different
	// prefix — a monorepo at its first `web-v` release while `app-v` tags already
	// exist. Neither is an error.
	baseRef := input.BaseRef
	firstRelease := false
	if baseRef == "" {
		tag, err := uc.repoInspector.GetLatestVersionTag(ctx, input.tagPrefixOrDefault())
		if err != nil || tag == "" {
			baseRef = ""
			firstRelease = true
		} else {
			baseRef = tag
		}
	}

	// Resolve commits between base and head
	commits, err := uc.repoInspector.ResolveCommits(ctx, baseRef, headSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve commits: %w", err)
	}

	// Get repo ID if not provided
	repoID := input.RepoID
	if repoID == "" {
		url, err := uc.repoInspector.GetRemoteURL(ctx)
		if err == nil {
			repoID = url
		} else {
			repoID = input.RepoRoot // Fallback to path
		}
	}

	// Create the release run aggregate
	run := domain.NewReleaseRun(
		repoID,
		input.RepoRoot,
		baseRef,
		headSHA,
		commits,
		input.ConfigHash,
		input.PluginPlanHash,
	)

	// Re-planning identical inputs must not undo work already done.
	//
	// A run's ID is derived from its plan hash (see domain.ReleaseRun.recomputeID),
	// so planning the same commits against the same base produces the same ID —
	// and Save then overwrites the stored run with this fresh, freshly-Planned
	// one. Before this check, running `relicta plan` while a release sat at
	// approved silently reset it to planned and discarded the state-machine
	// history that recorded the bump, the notes and the approval. For a tool
	// whose product is the audit trail, that is the worst possible failure: no
	// error, no warning, and the evidence of the approval simply gone.
	//
	// Keyed on the run ID, not the plan hash. FindByPlanHash exists for this and
	// looks like the obvious tool, but it does not work here: the aggregate
	// recomputes its plan hash when the changeset and the version proposal are
	// set (run.go:663, run.go:749) while the ID keeps the value derived at
	// construction. So a stored run reaches `approved` with an ID of
	// run-b715bb72... and a plan_hash of 998dbac9... — the two no longer agree,
	// and a hash lookup misses the very run whose ID is about to collide. The ID
	// is what collides, so the ID is what to look up.
	//
	// Force still supersedes, because re-planning is the documented recovery
	// path once HEAD has moved (issue #128). The difference is that discarding an
	// approval now requires asking for it.
	// LoadBatch, not Load: Load(ctx, runID) resolves the run by scanning repo
	// roots it has already seen in this process, and a fresh CLI invocation has
	// seen none — it returns "release run not found" for a run sitting on disk.
	// LoadBatch takes the root explicitly, so it works on the first call.
	existingRuns, _ := uc.repo.LoadBatch(ctx, input.RepoRoot, []domain.RunID{run.ID()})
	if existing := existingRuns[run.ID()]; existing != nil {
		if !input.DiscardExisting && existing.State() != domain.StateDraft {
			// Keep `latest` pointing at it so status/notes/approve continue to
			// operate on the run the caller just asked about.
			if err := uc.repo.SetLatest(ctx, input.RepoRoot, existing.ID()); err != nil {
				return nil, fmt.Errorf("failed to set latest run: %w", err)
			}
			return &PlanReleaseOutput{
				RunID:          existing.ID(),
				HeadSHA:        existing.HeadSHA(),
				Commits:        existing.Commits(),
				PlanHash:       existing.PlanHash(),
				CurrentVersion: existing.VersionCurrent(),
				VersionNext:    existing.VersionNext(),
				BumpKind:       existing.BumpKind(),
				RiskScore:      existing.RiskScore(),
				ChangeSet:      input.ChangeSet,

				// Lets the caller say "this already exists, at this state"
				// instead of reporting a plan it did not actually create.
				AlreadyExisted: true,
				ExistingState:  existing.State(),
			}, nil
		}
	}

	// Supersede active runs whose plan no longer matches.
	//
	// Deliberately after the identical-plan check above: this loop cancels every
	// active run, so running first would cancel the very run that check exists to
	// protect — the approved release would be gone before we looked for it. Now it
	// only sees runs that genuinely describe a different plan, which is the
	// case issue #128 is about (HEAD moved, re-planning is the recovery path).
	if activeRuns, findErr := uc.repo.FindActive(ctx, input.RepoRoot); findErr == nil && len(activeRuns) > 0 {
		// DiscardExisting also satisfies this: someone who explicitly asked to
		// throw away an in-progress run should not then be refused because a run
		// is in progress.
		if !input.Force && !input.DiscardExisting {
			return nil, fmt.Errorf("active release run exists: %s (run 'relicta cancel' to abort it, or 'relicta plan --force' to supersede it)", activeRuns[0].ID())
		}
		for _, stale := range activeRuns {
			if stale.ID() == run.ID() {
				// Same run we are about to write. Canceling it here would put a
				// spurious cancellation in the audit trail for a plan that is
				// simply being refreshed.
				continue
			}
			if cancelErr := stale.Cancel("superseded by re-plan", input.Actor.ID); cancelErr != nil {
				continue // already published or otherwise terminal — leave as-is
			}
			if saveErr := uc.repo.Save(ctx, stale); saveErr != nil {
				return nil, fmt.Errorf("failed to cancel superseded run %s: %w", stale.ID(), saveErr)
			}
		}
	}

	// Set actor
	run.SetActor(input.Actor.Type, input.Actor.ID)

	// Set pre-computed ChangeSet if provided
	if input.ChangeSet != nil {
		run.SetChangeSet(input.ChangeSet)
	}

	// Set version proposal if provided
	if input.CurrentVersion != nil && input.NextVersion != nil && input.BumpKind != nil {
		if err := run.SetVersionProposal(*input.CurrentVersion, *input.NextVersion, *input.BumpKind, input.Confidence); err != nil {
			return nil, fmt.Errorf("failed to set version proposal: %w", err)
		}
	}

	// Transition to Planned state
	if err := run.Plan(input.Actor.ID); err != nil {
		return nil, fmt.Errorf("failed to transition to planned state: %w", err)
	}

	// Handle tag-push mode: transition directly to versioned state
	// This enables notes/approve/publish workflow without running bump
	if input.TagPushMode && input.NextVersion != nil {
		tagName := input.TagName
		if tagName == "" {
			tagName = "v" + input.NextVersion.String()
		}

		// Set the version on the run
		if err := run.SetVersion(*input.NextVersion, tagName); err != nil {
			return nil, fmt.Errorf("tag-push mode: failed to set version: %w", err)
		}

		// Transition to versioned state (skipping the need for bump command)
		if err := run.Bump(input.Actor.ID); err != nil {
			return nil, fmt.Errorf("tag-push mode: failed to transition to versioned state: %w", err)
		}

		// Record tag-push mode for audit trail
		run.RecordTagPushMode(tagName, input.Actor.ID)
	}

	// Save the run
	if err := uc.repo.Save(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	// Set as latest
	if err := uc.repo.SetLatest(ctx, input.RepoRoot, run.ID()); err != nil {
		return nil, fmt.Errorf("failed to set latest run: %w", err)
	}

	// Export state machine JSON
	if uc.stateMachine != nil {
		if machineJSON, err := uc.stateMachine.ExportMachineJSON(); err == nil {
			// Best effort - don't fail if export fails
			if fileRepo, ok := uc.repo.(interface {
				SaveMachineJSON(string, domain.RunID, []byte) error
			}); ok {
				_ = fileRepo.SaveMachineJSON(input.RepoRoot, run.ID(), machineJSON)
			}
		}
	}

	return &PlanReleaseOutput{
		RunID:          run.ID(),
		HeadSHA:        run.HeadSHA(),
		Commits:        run.Commits(),
		PlanHash:       run.PlanHash(),
		CurrentVersion: run.VersionCurrent(),
		VersionNext:    run.VersionNext(),
		BumpKind:       run.BumpKind(),
		RiskScore:      run.RiskScore(),
		ChangeSet:      input.ChangeSet,
		FirstRelease:   firstRelease,
	}, nil
}
