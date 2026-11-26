package releasehistory

import (
	"context"
	"fmt"
	"sort"

	"github.com/relicta-tech/relicta/v5/internal/cgp/identity"
)

type IdentityRegistrySource interface {
	LoadAll(context.Context) ([]*identity.ActorIdentity, error)
}

type IdentityRegistryDestination interface {
	Save(context.Context, *identity.ActorIdentity) error
}

type IdentityRegistryReport struct {
	Identities int
	DryRun     bool
}

func (r IdentityRegistryReport) Records() int { return r.Identities }

// ImportIdentityRegistry copies configured identity grants without changing their source.
// Records are sorted by ID and upserted, so a partial migration can safely be rerun.
func ImportIdentityRegistry(ctx context.Context, src IdentityRegistrySource, dst IdentityRegistryDestination, opts Options) (IdentityRegistryReport, error) {
	report := IdentityRegistryReport{DryRun: opts.DryRun}
	if src == nil || dst == nil {
		return report, fmt.Errorf("importing identity registry: source and destination are required")
	}
	actors, err := src.LoadAll(ctx)
	if err != nil {
		return report, fmt.Errorf("reading identity registry: %w", err)
	}
	actors = append([]*identity.ActorIdentity(nil), actors...)
	sort.Slice(actors, func(i, j int) bool {
		if actors[i] == nil {
			return actors[j] != nil
		}
		if actors[j] == nil {
			return false
		}
		return actors[i].ID < actors[j].ID
	})
	for _, actor := range actors {
		if err := identity.ValidateActorIdentity(actor); err != nil {
			return report, fmt.Errorf("reading identity registry: invalid identity: %w", err)
		}
	}
	if opts.DryRun {
		report.Identities = len(actors)
		return report, nil
	}
	for _, actor := range actors {
		if err := dst.Save(ctx, actor); err != nil {
			return report, fmt.Errorf("importing identity %s after %d record(s); rerunning is safe: %w", actor.ID, report.Identities, err)
		}
		report.Identities++
	}
	return report, nil
}
