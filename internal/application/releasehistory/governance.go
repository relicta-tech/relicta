package releasehistory

// governance.go moves the other half of ADR-013's system of record: the governance memory
// store, which import.go's release runs say nothing about.
//
// It exists because of what `relicta db import` would otherwise be. Until governance memory
// became selectable, an operator switching persistence.backend moved their release runs and
// kept reading their governance history out of .relicta/governance/memory.json, because that
// is where every reader looked. Once the setting selects the governance store too, the same
// switch makes that file invisible: `relicta history` reports nothing, the DORA and SOC 2
// reports compute from an empty history, and the deployment gate authorizes against a record
// that says no release has ever happened. Nothing fails. The audit trail simply is not there.
//
// So the importer covers both, and for the same three reasons import.go gives:
//
//   - Non-destructive. The source is only ever read. memory.json stays as an export until the
//     operator removes it.
//   - Idempotent. Releases, incidents, decisions and authorizations are all keyed by ID in
//     both database adapters, so a second import converges rather than duplicating.
//   - Complete or reported. The source is read in full before the first write, and a write
//     that fails stops the import with a count of what was written. What cannot be moved at
//     all — deployments, which no database adapter can hold — is counted and reported rather
//     than dropped in silence.

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// GovernanceSource is the store being read. Reads only — see the file comment.
//
// An interface over the one method rather than *memory.FileStore, so this package keeps
// knowing nothing about which backends exist. Any store that can enumerate what it holds can
// be a source; today exactly one can.
type GovernanceSource interface {
	Snapshot(ctx context.Context) (*memory.Snapshot, error)
}

// GovernanceDestination is the store being written. A subset of memory.Store, listing the
// four record types that have somewhere to go.
type GovernanceDestination interface {
	RecordRelease(ctx context.Context, record *memory.ReleaseRecord) error
	RecordIncident(ctx context.Context, incident *memory.IncidentRecord) error
	RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error
	RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error
}

// GovernanceReport is what the import did, in terms an operator can check against.
type GovernanceReport struct {
	// Releases, Incidents, Decisions and Authorizations count what was written. After a
	// dry run they count what would be.
	Releases       int
	Incidents      int
	Decisions      int
	Authorizations int

	// Repositories names the repositories the source holds records for, sorted.
	//
	// Worth reporting because memory.json is per checkout but not per repository: a store
	// written before the governance identity was made canonical holds records keyed by
	// checkout path alongside records keyed by owner/name, and an operator who sees two
	// entries here knows why `relicta history` will show them half their history.
	Repositories []string

	// Deployments counts records that were read and not moved.
	//
	// memory.DeploymentStore is segregated from memory.Store because not every
	// implementation can hold a deployment, and neither database adapter does. Counting
	// them is the whole point: an operator switching backends is entitled to know that
	// this is the part of their evidence that stays behind.
	Deployments int

	// DryRun records that nothing was written.
	DryRun bool
}

// Written is how many records reached the destination.
func (r GovernanceReport) Written() int {
	return r.Releases + r.Incidents + r.Decisions + r.Authorizations
}

// Records is how many records the source holds, including the ones that cannot move.
func (r GovernanceReport) Records() int { return r.Written() + r.Deployments }

// ImportGovernance copies every governance record in src into dst.
//
// The order is releases, then incidents, then decisions, then authorizations, and it is not
// arbitrary. Incidents after releases because an actor's metrics are derived from both and an
// interrupted import should leave a destination understating an actor's *releases* rather
// than overstating their reliability by holding incidents for releases it has not got.
// Authorizations after decisions because an authorization hangs off one, and a trail that
// grants execution for a decision it cannot show is the wrong half to keep.
//
// On failure the report is returned alongside the error, populated with what was written, so
// the caller can say how far it got instead of only that it stopped.
func ImportGovernance(
	ctx context.Context, src GovernanceSource, dst GovernanceDestination, opts Options,
) (GovernanceReport, error) {
	report := GovernanceReport{DryRun: opts.DryRun}

	if src == nil || dst == nil {
		return report, errors.New("importing governance memory: a source and a destination are required")
	}

	snapshot, err := src.Snapshot(ctx)
	if err != nil {
		return report, fmt.Errorf("reading the governance memory store: %w", err)
	}

	releases := flattenReleases(snapshot.Releases)
	incidents := flattenIncidents(snapshot.Incidents)
	decisions := sortedDecisions(snapshot.Decisions)
	authorizations := sortedAuthorizations(snapshot.Authorizations)

	report.Repositories = repositoriesIn(snapshot)
	for _, records := range snapshot.Deployments {
		report.Deployments += len(records)
	}

	if opts.DryRun {
		report.Releases = len(releases)
		report.Incidents = len(incidents)
		report.Decisions = len(decisions)
		report.Authorizations = len(authorizations)
		return report, nil
	}

	for _, record := range releases {
		if err := dst.RecordRelease(ctx, record); err != nil {
			return report, governanceWriteError("release", record.ID, report, err)
		}
		report.Releases++
	}
	for _, incident := range incidents {
		if err := dst.RecordIncident(ctx, incident); err != nil {
			return report, governanceWriteError("incident", incident.ID, report, err)
		}
		report.Incidents++
	}
	for _, decision := range decisions {
		if err := dst.RecordDecision(ctx, decision); err != nil {
			return report, governanceWriteError("decision", decision.ID, report, err)
		}
		report.Decisions++
	}
	for _, auth := range authorizations {
		if err := dst.RecordAuthorization(ctx, auth); err != nil {
			return report, governanceWriteError("authorization", auth.ID, report, err)
		}
		report.Authorizations++
	}

	return report, nil
}

// governanceWriteError says which record failed, how far the import got, and that re-running
// is safe. "failed to import governance memory" would tell an operator none of the three, and
// the first thing they would do is go and find out.
func governanceWriteError(kind, id string, report GovernanceReport, err error) error {
	return fmt.Errorf(
		"importing %s %s after %d record(s): the governance memory under "+
			".relicta/governance was not modified, and every record is written by ID, so "+
			"running the import again is safe and resumes: %w",
		kind, id, report.Written(), err)
}

// flattenReleases returns every release record, oldest first.
//
// Oldest first for the reason import.go writes runs oldest first: an interrupted import should
// hold the beginning of a history, which reads as unfinished, rather than the end, which reads
// as complete. The ID breaks ties so the sequence is the same on every run.
func flattenReleases(byRepository map[string][]*memory.ReleaseRecord) []*memory.ReleaseRecord {
	var all []*memory.ReleaseRecord
	for _, records := range byRepository {
		for _, record := range records {
			if record != nil {
				all = append(all, record)
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].ReleasedAt.Equal(all[j].ReleasedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].ReleasedAt.Before(all[j].ReleasedAt)
	})
	return all
}

func flattenIncidents(byRepository map[string][]*memory.IncidentRecord) []*memory.IncidentRecord {
	var all []*memory.IncidentRecord
	for _, records := range byRepository {
		for _, record := range records {
			if record != nil {
				all = append(all, record)
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].DetectedAt.Equal(all[j].DetectedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].DetectedAt.Before(all[j].DetectedAt)
	})
	return all
}

// sortedDecisions returns the decisions by ID, so an import writes the same sequence twice.
//
// Map iteration order is randomized in Go, and an importer whose write order changes between
// runs is one whose partial results cannot be compared with each other.
func sortedDecisions(byID map[string]*cgp.GovernanceDecision) []*cgp.GovernanceDecision {
	ids := make([]string, 0, len(byID))
	for id, decision := range byID {
		if decision != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	decisions := make([]*cgp.GovernanceDecision, 0, len(ids))
	for _, id := range ids {
		decisions = append(decisions, byID[id])
	}
	return decisions
}

func sortedAuthorizations(byID map[string]*cgp.ExecutionAuthorization) []*cgp.ExecutionAuthorization {
	ids := make([]string, 0, len(byID))
	for id, auth := range byID {
		if auth != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	auths := make([]*cgp.ExecutionAuthorization, 0, len(ids))
	for _, id := range ids {
		auths = append(auths, byID[id])
	}
	return auths
}

// repositoriesIn names every repository the snapshot holds any record for.
func repositoriesIn(snapshot *memory.Snapshot) []string {
	seen := map[string]bool{}
	for repository, records := range snapshot.Releases {
		if len(records) > 0 {
			seen[repository] = true
		}
	}
	for repository, records := range snapshot.Incidents {
		if len(records) > 0 {
			seen[repository] = true
		}
	}
	for repository, records := range snapshot.Deployments {
		if len(records) > 0 {
			seen[repository] = true
		}
	}

	repositories := make([]string, 0, len(seen))
	for repository := range seen {
		repositories = append(repositories, repository)
	}
	sort.Strings(repositories)
	return repositories
}
