package persistence

import (
	"context"
	"fmt"
	"sync"

	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

// memReleaseRepo is an in-memory release.Repository for the unit-of-work tests.
//
// They used to build a real FileReleaseRepository. That store has been deleted:
// it was the second of two implementations of the same aggregate, and reading a
// run written by the other one returned it without its changeset, HEAD SHA or
// commits — which broke governance three ways before the two were consolidated
// behind one store.
//
// A double rather than the surviving file store, because these tests are about
// the unit of work's commit and rollback semantics, not about persistence. Keeping
// them off disk also keeps them from depending on whichever store the container
// happens to use.
type memReleaseRepo struct {
	mu   sync.Mutex
	runs map[release.RunID]*release.ReleaseRun
}

func newMemReleaseRepo() *memReleaseRepo {
	return &memReleaseRepo{runs: make(map[release.RunID]*release.ReleaseRun)}
}

func (m *memReleaseRepo) Save(_ context.Context, run *release.ReleaseRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.ID()] = run
	return nil
}

func (m *memReleaseRepo) FindByID(_ context.Context, id release.RunID) (*release.ReleaseRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return nil, fmt.Errorf("release run %s not found", id)
	}
	return run, nil
}

func (m *memReleaseRepo) Delete(_ context.Context, id release.RunID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[id]; !ok {
		return fmt.Errorf("release run %s not found", id)
	}
	delete(m.runs, id)
	return nil
}

func (m *memReleaseRepo) List(_ context.Context, _ string) ([]release.RunID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]release.RunID, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	return ids, nil
}

// TestFileUnitOfWork_FindByStateActiveAndSpec exercises these three, so they are
// implemented rather than stubbed.
func (m *memReleaseRepo) FindByState(_ context.Context, state release.RunState) ([]*release.ReleaseRun, error) {
	return m.filter(func(r *release.ReleaseRun) bool { return r.State() == state }), nil
}

func (m *memReleaseRepo) FindActive(_ context.Context) ([]*release.ReleaseRun, error) {
	return m.filter(func(r *release.ReleaseRun) bool { return r.State().IsActive() }), nil
}

func (m *memReleaseRepo) FindBySpecification(_ context.Context, spec release.Specification) ([]*release.ReleaseRun, error) {
	return m.filter(spec.IsSatisfiedBy), nil
}

func (m *memReleaseRepo) filter(keep func(*release.ReleaseRun) bool) []*release.ReleaseRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*release.ReleaseRun, 0, len(m.runs))
	for _, run := range m.runs {
		if keep(run) {
			out = append(out, run)
		}
	}
	return out
}

// FindLatest and Publish are unused by these tests. They return errors rather
// than empty results, so a test that starts relying on one fails loudly instead of
// quietly asserting against nothing.
func (m *memReleaseRepo) FindLatest(context.Context, string) (*release.ReleaseRun, error) {
	return nil, fmt.Errorf("FindLatest is not implemented by memReleaseRepo")
}

func (m *memReleaseRepo) Publish(context.Context, ...release.DomainEvent) error {
	return fmt.Errorf("Publish is not implemented by memReleaseRepo")
}
