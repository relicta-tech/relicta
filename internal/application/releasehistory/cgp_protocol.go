package releasehistory

import (
	"context"
	"errors"
	"fmt"
	"sort"

	cgpprotocol "github.com/relicta-tech/relicta/v5/internal/cgp/protocol"
	cgpsdk "github.com/relicta-tech/relicta/v5/pkg/cgp"
)

// CGPProtocolSource is the read-only side of importing protocol handshake records.
type CGPProtocolSource interface {
	ListProposals(context.Context) ([]string, error)
	GetProposal(context.Context, string) (*cgpsdk.ChangeProposal, error)
	GetDecision(context.Context, string) (*cgpsdk.GovernanceDecision, error)
	GetAuthorization(context.Context, string) (*cgpsdk.ExecutionAuthorization, error)
}

// CGPProtocolDestination is the writable side of importing protocol handshake records.
type CGPProtocolDestination interface {
	SaveProposal(context.Context, *cgpsdk.ChangeProposal) error
	SaveDecision(context.Context, *cgpsdk.GovernanceDecision) error
	SaveAuthorization(context.Context, *cgpsdk.ExecutionAuthorization) error
}

type CGPProtocolReport struct {
	Proposals      int
	Decisions      int
	Authorizations int
	DryRun         bool
}

func (r CGPProtocolReport) Records() int { return r.Proposals + r.Decisions + r.Authorizations }
func (r CGPProtocolReport) Written() int { return r.Records() }

// ImportCGPProtocol copies complete handshakes in deterministic order. The file source is
// never modified and destination writes are keyed by proposal ID, making a rerun safe.
func ImportCGPProtocol(ctx context.Context, src CGPProtocolSource, dst CGPProtocolDestination, opts Options) (CGPProtocolReport, error) {
	report := CGPProtocolReport{DryRun: opts.DryRun}
	if src == nil || dst == nil {
		return report, fmt.Errorf("importing CGP protocol records: a source and destination are required")
	}
	ids, err := src.ListProposals(ctx)
	if err != nil {
		return report, fmt.Errorf("listing CGP proposals: %w", err)
	}
	sort.Strings(ids)
	type handshake struct {
		proposal *cgpsdk.ChangeProposal
		decision *cgpsdk.GovernanceDecision
		auth     *cgpsdk.ExecutionAuthorization
	}
	items := make([]handshake, 0, len(ids))
	for _, id := range ids {
		item := handshake{}
		item.proposal, err = src.GetProposal(ctx, id)
		if err != nil {
			return report, fmt.Errorf("reading CGP proposal %s: %w", id, err)
		}
		item.decision, err = src.GetDecision(ctx, id)
		if err != nil {
			if !isProtocolRecordMissing(err) {
				return report, fmt.Errorf("reading CGP decision for %s: %w", id, err)
			}
			item.decision = nil
		}
		item.auth, err = src.GetAuthorization(ctx, id)
		if err != nil {
			if !isProtocolRecordMissing(err) {
				return report, fmt.Errorf("reading CGP authorization for %s: %w", id, err)
			}
			item.auth = nil
		}
		items = append(items, item)
	}
	if opts.DryRun {
		for _, item := range items {
			report.Proposals++
			if item.decision != nil {
				report.Decisions++
			}
			if item.auth != nil {
				report.Authorizations++
			}
		}
		return report, nil
	}
	for _, item := range items {
		id := item.proposal.ID
		if err := dst.SaveProposal(ctx, item.proposal); err != nil {
			return report, fmt.Errorf("importing CGP proposal %s after %d records; rerun is safe: %w", id, report.Records(), err)
		}
		report.Proposals++
		if item.decision != nil {
			if err := dst.SaveDecision(ctx, item.decision); err != nil {
				return report, fmt.Errorf("importing CGP decision for %s after %d records; rerun is safe: %w", id, report.Records(), err)
			}
			report.Decisions++
		}
		if item.auth != nil {
			if err := dst.SaveAuthorization(ctx, item.auth); err != nil {
				return report, fmt.Errorf("importing CGP authorization for %s after %d records; rerun is safe: %w", id, report.Records(), err)
			}
			report.Authorizations++
		}
	}
	return report, nil
}

func isProtocolRecordMissing(err error) bool {
	return errors.Is(err, cgpprotocol.ErrRecordNotFound)
}
