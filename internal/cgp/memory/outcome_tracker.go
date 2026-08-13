// Package memory provides the Release Memory store for CGP.
package memory

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
)

// OutcomeTracker implements release.EventPublisher and records release outcomes
// to the CGP memory store. It wraps another EventPublisher to allow chaining
// with other event handlers.
//
// This service provides the feedback loop for CGP risk assessment by tracking:
// - Release outcomes (success, failure, rollback)
// - Actor reliability metrics
// - Historical patterns for risk prediction
type OutcomeTracker struct {
	store  Store
	next   release.EventPublisher
	logger *slog.Logger

	// repository is the governance identity to record against when the cached context
	// has none.
	//
	// The cache is per-process, and the CLI runs one command per process: `relicta
	// cancel` loads a run from disk and raises a single RunCanceledEvent, so nothing
	// ever supplied the RunCreatedEvent that carries the repository. Every such record
	// was rejected by the store with "repository is required" — a warning in a log
	// nobody reads, and no record at all. It is supplied by the caller rather than
	// derived from the event because the identity comes from the git remote, and a
	// second derivation from the run's path would produce "local:checkout" where the
	// publish path produces "acme/widget", splitting one repository's history in two.
	repository string

	// releaseContexts caches release context for building complete records.
	// This is needed because outcome events don't contain all release metadata.
	releaseContexts map[release.RunID]*releaseContext
}

// releaseContext caches release information needed for building ReleaseRecord.
type releaseContext struct {
	Repository      string
	Version         string
	Actor           cgp.Actor
	RiskScore       float64
	BreakingChanges int
	SecurityChanges int
	FilesChanged    int
	LinesChanged    int
	Decision        cgp.DecisionType
	StartedAt       time.Time
	Tags            []string
	Metadata        map[string]string
}

// NewOutcomeTracker creates a new OutcomeTracker.
//
// next is optional — if nil, events are not forwarded. repository is the governance
// identity to fall back on, and should be the same value the rest of the process records
// against (repoInfo.GovernanceID()); empty is allowed, and then a terminal event arriving
// without a cached context cannot be recorded.
func NewOutcomeTracker(store Store, next release.EventPublisher, repository string) *OutcomeTracker {
	return &OutcomeTracker{
		store:           store,
		next:            next,
		logger:          slog.Default().With("component", "outcome_tracker"),
		repository:      repository,
		releaseContexts: make(map[release.RunID]*releaseContext),
	}
}

// Publish processes domain events and records outcomes to the memory store.
// It forwards all events to the next publisher (if configured) regardless
// of outcome tracking success.
func (t *OutcomeTracker) Publish(ctx context.Context, events ...release.DomainEvent) error {
	for _, event := range events {
		if err := t.processEvent(ctx, event); err != nil {
			// Log but don't fail - outcome tracking is non-critical
			t.logger.Warn("failed to process event for outcome tracking",
				"event", event.EventName(),
				"release_id", event.AggregateID(),
				"error", err)
		}
	}

	// Forward to next publisher
	if t.next != nil {
		return t.next.Publish(ctx, events...)
	}

	return nil
}

// processEvent routes events to the appropriate handler.
func (t *OutcomeTracker) processEvent(ctx context.Context, event release.DomainEvent) error {
	switch e := event.(type) {
	case *release.RunCreatedEvent:
		return t.handleInitialized(e)
	case *release.RunPlannedEvent:
		return t.handlePlanned(e)
	case *release.RunVersionedEvent:
		return t.handleVersioned(e)
	case *release.RunApprovedEvent:
		return t.handleApproved(e)
	case *release.RunPublishedEvent:
		return t.handlePublished(ctx, e)
	case *release.RunFailedEvent:
		return t.handleFailed(ctx, e)
	case *release.RunCanceledEvent:
		return t.handleCanceled(ctx, e)
	default:
		// Other events don't affect outcome tracking
		return nil
	}
}

// handleInitialized caches initial release context.
//
// RepoID is a raw git remote URL (the plan use case falls back to the checkout path),
// while every reader — history, the DORA and SOC 2 reports, reconcile, the deployment
// gate — queries by governance identity. Recording the URL verbatim put these records
// under a key nothing reads, so they accumulated and were never found: the same defect
// that made `relicta history` empty in every repository, surviving in a second writer.
//
// Normalized through the shared helper rather than a local variant, because two
// normalizers eventually disagree and the disagreement is invisible — it looks like an
// empty store.
func (t *OutcomeTracker) handleInitialized(e *release.RunCreatedEvent) error {
	t.releaseContexts[e.AggregateID()] = &releaseContext{
		Repository: governanceIdentity(e.RepoID),
		StartedAt:  e.OccurredAt(),
		Metadata:   make(map[string]string),
	}
	return nil
}

// governanceIdentity normalizes a run's repository reference to the key readers use.
//
// RepoID arrives in three shapes, because the plan use case takes whichever it can get:
// an explicit identity supplied by the caller, the git remote URL, or the checkout path
// as a last resort. Each needs different handling, and getting that wrong is silent —
// a misfiled record looks exactly like an empty store.
//
//	owner/repo                          already the identity, passed through
//	https://github.com/acme/widget.git  normalized to acme/widget
//	git@github.com:acme/widget.git      the same, so one repository is not keyed twice
//	/Users/dev/checkout                 local:checkout
//
// A path must not reach the remote parser: its last two segments form a
// plausible-looking pair — "/Users/dev/checkout" becomes "dev/checkout" — that cannot
// be told apart from a real owner/repo, so it keys records to a repository nobody has.
// This is the malformed identity RepositoryInfo.GovernanceID exists to avoid, and the
// "local:" prefix is what that function produces for the same case, so both writers
// agree on the local repository too. Agreement is the whole point: a second normalizer
// that is merely close splits the history in a way that reads as no history at all.
func governanceIdentity(repoRef string) string {
	ref := strings.TrimSpace(repoRef)
	if ref == "" {
		return ""
	}

	if looksLikeRemote(ref) {
		if id := sourcecontrol.GovernanceIDFromRemote(ref); id != "" {
			return id
		}
	}

	if isBareIdentity(ref) {
		return ref
	}

	name := path.Base(filepath.ToSlash(ref))
	if name == "" || name == "." || name == "/" {
		return ref
	}
	return "local:" + name
}

// isBareIdentity reports whether a reference is already an "owner/repo" pair.
//
// Two non-empty segments and no leading separator or dot. A relative two-segment path
// is indistinguishable from an identity, and this resolves the ambiguity toward the
// identity: that is the documented governance form, and rewriting one that a caller
// supplied deliberately would move records away from where that caller reads them.
func isBareIdentity(ref string) bool {
	if strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, ".") || strings.Contains(ref, `\`) {
		return false
	}
	parts := strings.Split(ref, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// looksLikeRemote reports whether a reference is a git remote rather than a path.
//
// A scheme covers https/ssh/git URLs. The scp form (git@host:owner/repo) has no
// scheme, so it is recognized by an "@" appearing before the colon — which a Windows
// drive letter or a plain path does not have.
func looksLikeRemote(ref string) bool {
	if strings.Contains(ref, "://") {
		return true
	}
	colon := strings.Index(ref, ":")
	return colon > 0 && strings.Contains(ref[:colon], "@")
}

// handlePlanned updates the cached context with plan details.
//
// Retained because the event type is part of the public event surface, but note that
// the aggregate never raises it: run.go raises RunCreated, RunVersioned, RunApproved,
// RunPublished, RunFailed, RunCanceled, RunRetried and RunNotesUpdated. This handler
// was therefore the only writer of the cached version and never ran.
func (t *OutcomeTracker) handlePlanned(e *release.RunPlannedEvent) error {
	ctx := t.getOrCreateContext(e.AggregateID())
	ctx.Version = e.VersionNext.String()
	return nil
}

// handleVersioned records the version the run settled on.
//
// This is the event the aggregate actually raises when a version is decided. Without
// it the cached version stayed empty, and only the success path recovered — because
// handlePublished overwrites the version from RunPublishedEvent, which carries one.
// RunFailedEvent and RunCanceledEvent do not, so every failed and canceled release was
// recorded with no version at all and could not be tied to the version that failed.
// That is the half of the history change failure rate and reputation are computed from.
func (t *OutcomeTracker) handleVersioned(e *release.RunVersionedEvent) error {
	ctx := t.getOrCreateContext(e.AggregateID())
	ctx.Version = e.VersionNext.String()
	return nil
}

// handleApproved updates the cached context with approval info.
func (t *OutcomeTracker) handleApproved(e *release.RunApprovedEvent) error {
	ctx := t.getOrCreateContext(e.AggregateID())
	ctx.Metadata["approved_by"] = e.ApprovedBy
	return nil
}

// handlePublished records a successful release outcome.
func (t *OutcomeTracker) handlePublished(ctx context.Context, e *release.RunPublishedEvent) error {
	releaseCtx := t.getOrCreateContext(e.AggregateID())

	record := t.buildReleaseRecord(e.AggregateID(), releaseCtx, OutcomeSuccess, e.OccurredAt())
	record.Version = e.Version.String()

	if err := t.store.RecordRelease(ctx, record); err != nil {
		return fmt.Errorf("failed to record successful release: %w", err)
	}

	t.logger.Info("recorded successful release outcome",
		"release_id", e.AggregateID(),
		"version", e.Version.String())

	// Clean up context cache
	delete(t.releaseContexts, e.AggregateID())

	return nil
}

// handleFailed records a failed release outcome.
func (t *OutcomeTracker) handleFailed(ctx context.Context, e *release.RunFailedEvent) error {
	releaseCtx := t.getOrCreateContext(e.AggregateID())

	record := t.buildReleaseRecord(e.AggregateID(), releaseCtx, OutcomeFailed, e.OccurredAt())
	record.Metadata["failure_reason"] = e.Reason
	// The event's version is the only source in a fresh process, where the cached
	// context from RunVersionedEvent does not exist.
	if record.Version == "" {
		record.Version = e.Version
	}

	if err := t.store.RecordRelease(ctx, record); err != nil {
		return fmt.Errorf("failed to record failed release: %w", err)
	}

	t.logger.Info("recorded failed release outcome",
		"release_id", e.AggregateID(),
		"reason", e.Reason)

	// Clean up context cache
	delete(t.releaseContexts, e.AggregateID())

	return nil
}

// handleCanceled records a canceled release outcome.
func (t *OutcomeTracker) handleCanceled(ctx context.Context, e *release.RunCanceledEvent) error {
	releaseCtx := t.getOrCreateContext(e.AggregateID())

	// Recorded as canceled, not partial. OutcomePartial is a negative outcome that
	// counts toward change failure rate and an actor's failed releases, so recording a
	// deliberate cancellation that way punished the governance gate for working.
	record := t.buildReleaseRecord(e.AggregateID(), releaseCtx, OutcomeCanceled, e.OccurredAt())
	record.Metadata["canceled_by"] = e.By
	record.Metadata["cancel_reason"] = e.Reason
	if record.Version == "" {
		record.Version = e.Version
	}
	if record.Actor.ID == "" && e.By != "" {
		// A fresh process has no cached context, so the event's own actor is the only
		// one available — and an audit entry nobody can attribute is half an entry.
		record.Actor = cgp.NewActor(cgp.ActorKindHuman,
			cgp.QualifiedActorID(cgp.ActorKindHuman, e.By))
	}
	record.Tags = append(record.Tags, "canceled")

	if err := t.store.RecordRelease(ctx, record); err != nil {
		return fmt.Errorf("failed to record canceled release: %w", err)
	}

	t.logger.Info("recorded canceled release outcome",
		"release_id", e.AggregateID(),
		"canceled_by", e.By)

	// Clean up context cache
	delete(t.releaseContexts, e.AggregateID())

	return nil
}

// buildReleaseRecord constructs a ReleaseRecord from cached context.
func (t *OutcomeTracker) buildReleaseRecord(
	releaseID release.RunID,
	ctx *releaseContext,
	outcome ReleaseOutcome,
	occurredAt time.Time,
) *ReleaseRecord {
	duration := time.Duration(0)
	if !ctx.StartedAt.IsZero() {
		duration = occurredAt.Sub(ctx.StartedAt)
	}

	metadata := make(map[string]string)
	for k, v := range ctx.Metadata {
		metadata[k] = v
	}

	// The cached context wins when it has one, because it came from this run's own
	// RunCreatedEvent; the fallback covers the fresh-process case where there is no
	// cached context at all.
	repository := ctx.Repository
	if repository == "" {
		repository = t.repository
	}

	return &ReleaseRecord{
		ID:              string(releaseID),
		Repository:      repository,
		Version:         ctx.Version,
		Actor:           ctx.Actor,
		RiskScore:       ctx.RiskScore,
		Decision:        ctx.Decision,
		BreakingChanges: ctx.BreakingChanges,
		SecurityChanges: ctx.SecurityChanges,
		FilesChanged:    ctx.FilesChanged,
		LinesChanged:    ctx.LinesChanged,
		Outcome:         outcome,
		ReleasedAt:      occurredAt,
		Duration:        duration,
		Tags:            append([]string{}, ctx.Tags...),
		Metadata:        metadata,
	}
}

// getOrCreateContext retrieves or creates a release context.
func (t *OutcomeTracker) getOrCreateContext(id release.RunID) *releaseContext {
	if ctx, ok := t.releaseContexts[id]; ok {
		return ctx
	}
	ctx := &releaseContext{
		Metadata: make(map[string]string),
	}
	t.releaseContexts[id] = ctx
	return ctx
}

// SetReleaseContext allows external code to provide full release context
// before outcome events arrive. This is useful when the outcome tracker
// doesn't observe all events from the beginning.
func (t *OutcomeTracker) SetReleaseContext(
	releaseID release.RunID,
	repository string,
	version string,
	actor cgp.Actor,
	riskScore float64,
	decision cgp.DecisionType,
) {
	ctx := t.getOrCreateContext(releaseID)
	ctx.Repository = repository
	ctx.Version = version
	ctx.Actor = actor
	ctx.RiskScore = riskScore
	ctx.Decision = decision
	if ctx.StartedAt.IsZero() {
		ctx.StartedAt = time.Now()
	}
}

// SetChangeMetrics sets change metrics for a release context.
func (t *OutcomeTracker) SetChangeMetrics(
	releaseID release.RunID,
	breakingChanges, securityChanges, filesChanged, linesChanged int,
) {
	ctx := t.getOrCreateContext(releaseID)
	ctx.BreakingChanges = breakingChanges
	ctx.SecurityChanges = securityChanges
	ctx.FilesChanged = filesChanged
	ctx.LinesChanged = linesChanged
}

// AddTags adds tags to a release context.
func (t *OutcomeTracker) AddTags(releaseID release.RunID, tags ...string) {
	ctx := t.getOrCreateContext(releaseID)
	ctx.Tags = append(ctx.Tags, tags...)
}

// Ensure OutcomeTracker implements release.EventPublisher.
var _ release.EventPublisher = (*OutcomeTracker)(nil)
