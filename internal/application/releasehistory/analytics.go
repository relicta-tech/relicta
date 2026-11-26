package releasehistory

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
)

// AnalyticsReport describes analytics events copied to the selected backend.
type AnalyticsReport struct {
	Events   int
	Existing int
	Imported int
	DryRun   bool
}

// ImportAnalytics copies the append-only analytics history without changing its source.
// Existing event IDs are skipped so rerunning after an interrupted import is safe.
func ImportAnalytics(ctx context.Context, source, destination analytics.Store, opts Options) (AnalyticsReport, error) {
	report := AnalyticsReport{DryRun: opts.DryRun}
	events, err := source.Query(ctx, analytics.QueryFilter{})
	if err != nil {
		return report, fmt.Errorf("read analytics source: %w", err)
	}
	report.Events = len(events)
	if opts.DryRun {
		return report, nil
	}
	existing, err := destination.Query(ctx, analytics.QueryFilter{})
	if err != nil {
		return report, fmt.Errorf("read analytics destination: %w", err)
	}
	seen := make(map[string]struct{}, len(existing))
	for _, event := range existing {
		seen[event.ID] = struct{}{}
	}
	for _, event := range events {
		if _, ok := seen[event.ID]; ok {
			report.Existing++
			continue
		}
		if err := destination.Append(ctx, event); err != nil {
			return report, fmt.Errorf("import analytics event %s: %w", event.ID, err)
		}
		report.Imported++
		seen[event.ID] = struct{}{}
	}
	return report, nil
}
