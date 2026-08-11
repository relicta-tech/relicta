package memory

import (
	"context"
	"fmt"
	"sort"
)

// DeploymentStore records and reads deployments.
//
// A segregated interface rather than an addition to Store, for the same reason the
// recommendation store is separate: not every Store implementation can hold these,
// and widening the full interface would oblige the remote adapters (chronos,
// mnemos) to invent behavior for a record they do not yet carry. Callers
// type-assert, and a store without deployment support is an honest absence rather
// than a silent no-op.
type DeploymentStore interface {
	// RecordDeployment stores one version reaching one environment.
	RecordDeployment(ctx context.Context, record *DeploymentRecord) error

	// GetDeploymentHistory returns a repository's deployments, newest first.
	// environment filters when non-empty.
	GetDeploymentHistory(ctx context.Context, repository, environment string, limit int) ([]*DeploymentRecord, error)
}

// validateDeployment rejects a record that cannot be counted.
//
// Validated on the way in, because these arrive from outside — a GitOps controller,
// a CI step, a script. An unrecognized outcome stored silently would be counted as
// neither success nor failure and would bias every rate derived from it, and a
// record with no environment cannot answer the question deployment frequency asks.
func validateDeployment(record *DeploymentRecord) error {
	switch {
	case record == nil:
		return fmt.Errorf("deployment record is nil")
	case record.Repository == "":
		return fmt.Errorf("deployment record requires a repository")
	case record.Environment == "":
		return fmt.Errorf("deployment record requires an environment")
	case record.Version == "":
		return fmt.Errorf("deployment record requires a version")
	case !record.Outcome.IsValid():
		return fmt.Errorf("unknown deployment outcome %q", record.Outcome)
	case !record.Provenance.IsValid():
		return fmt.Errorf("unknown deployment provenance %q", record.Provenance)
	}
	return nil
}

// newestFirst orders deployments by completion time, descending.
//
// Ordered on read rather than trusting insertion order: records arrive from several
// reporters and a controller can report a sync that finished before one already
// stored.
func newestFirst(records []*DeploymentRecord, environment string, limit int) []*DeploymentRecord {
	filtered := make([]*DeploymentRecord, 0, len(records))
	for _, r := range records {
		if environment != "" && r.Environment != environment {
			continue
		}
		filtered = append(filtered, r)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].DeployedAt.After(filtered[j].DeployedAt)
	})

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// RecordDeployment stores a deployment in memory.
func (s *InMemoryStore) RecordDeployment(_ context.Context, record *DeploymentRecord) error {
	if err := validateDeployment(record); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments[record.Repository] = append(s.deployments[record.Repository], record)
	return nil
}

// GetDeploymentHistory returns in-memory deployments, newest first.
func (s *InMemoryStore) GetDeploymentHistory(_ context.Context, repository, environment string, limit int) ([]*DeploymentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return newestFirst(s.deployments[repository], environment, limit), nil
}

// RecordDeployment stores a deployment and persists it.
func (s *FileStore) RecordDeployment(_ context.Context, record *DeploymentRecord) error {
	if err := validateDeployment(record); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deployments == nil {
		s.deployments = make(map[string][]*DeploymentRecord)
	}
	s.deployments[record.Repository] = append(s.deployments[record.Repository], record)
	return s.save()
}

// GetDeploymentHistory returns persisted deployments, newest first.
func (s *FileStore) GetDeploymentHistory(_ context.Context, repository, environment string, limit int) ([]*DeploymentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return newestFirst(s.deployments[repository], environment, limit), nil
}
