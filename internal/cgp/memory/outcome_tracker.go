// Package memory provides the Release Memory store for CGP.
package memory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/release"
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
// The next parameter is optional - if nil, events are not forwarded.
func NewOutcomeTracker(store Store, next release.EventPublisher) *OutcomeTracker {
	return &OutcomeTracker{
		store:           store,
		next:            next,
		logger:          slog.Default().With("component", "outcome_tracker"),
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
func (t *OutcomeTracker) handleInitialized(e *release.RunCreatedEvent) error {
	t.releaseContexts[e.AggregateID()] = &releaseContext{
		Repository: e.RepoID,
		StartedAt:  e.OccurredAt(),
		Metadata:   make(map[string]string),
	}
	return nil
}

// handlePlanned updates the cached context with plan details.
func (t *OutcomeTracker) handlePlanned(e *release.RunPlannedEvent) error {
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

	// Cancellation is treated as a partial outcome (not success, not failure)
	record := t.buildReleaseRecord(e.AggregateID(), releaseCtx, OutcomePartial, e.OccurredAt())
	record.Metadata["canceled_by"] = e.By
	record.Metadata["cancel_reason"] = e.Reason
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

	return &ReleaseRecord{
		ID:              string(releaseID),
		Repository:      ctx.Repository,
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
