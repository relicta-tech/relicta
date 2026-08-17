package multirepo

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Readiness built a file repository unconditionally, and once persistence.backend began
// selecting a store (ADR-013) that became a confident wrong answer rather than a gap: a member
// whose run was approved and stored in SQLite was reported as
//
//	svc-a  -  -  no release has been planned — run 'relicta plan' in ../svc-a
//
// which the operator had already done. Reproduced against the shipped binary, and it is the same
// shape as the blast-radius defect that computed a real number over the wrong file set.

// approvedRunIn returns a repository holding one planned run for the given path, standing in for
// whichever backend the member configured.
//
// A real file repository rather than a hand-written stub: the ten-method port would need one, and
// a stub that answers LoadLatest from a map proves only that the map was consulted. What these
// tests are about is that Check reads the store it was given for that member's path.
func approvedRunIn(ctx context.Context, t *testing.T, path string) ports.ReleaseRunRepository {
	t.Helper()

	repo := adapters.NewFileReleaseRunRepository()
	run := domainrelease.NewReleaseRunForTest("run-ready", "main", path)
	if err := run.Plan("test"); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.SetLatest(ctx, path, run.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}
	return repo
}

func TestAMemberIsCheckedInTheStoreItConfigured(t *testing.T) {
	member := appmultirepo.Member{Name: "svc-a", Path: t.TempDir()}
	repo := approvedRunIn(context.Background(), t, member.Path)

	asked := false
	r := &Readiness{storeFor: func(_ context.Context, root string) (ports.ReleaseRunRepository, io.Closer, error) {
		asked = true
		if root != member.Path {
			t.Errorf("the store was opened for %q, want the member's path %q", root, member.Path)
		}
		return repo, nil, nil
	}}

	states := r.Check(context.Background(), []appmultirepo.Member{member})

	if !asked {
		t.Fatal("no store was opened for the member")
	}
	if len(states) != 1 {
		t.Fatalf("got %d states, want 1", len(states))
	}
	if states[0].Blocker == "no release has been planned — run 'relicta plan' in "+member.Path {
		t.Errorf("the member's run was not found, so readiness told the operator to plan a "+
			"release they had already planned: %q", states[0].Blocker)
	}
	if states[0].State == "" {
		t.Error("the run's state was not reported, so the store was not read")
	}
}

// A database that cannot be opened and a repository that has not been planned send the operator
// to different places, so they must not share a message.
func TestAStoreThatCannotBeOpenedIsItsOwnBlocker(t *testing.T) {
	member := appmultirepo.Member{Name: "svc-a", Path: t.TempDir()}

	r := &Readiness{storeFor: func(context.Context, string) (ports.ReleaseRunRepository, io.Closer, error) {
		return nil, nil, errors.New("connection refused")
	}}

	states := r.Check(context.Background(), []appmultirepo.Member{member})

	if states[0].Ready {
		t.Fatal("a member whose store is unreachable was reported ready")
	}
	if states[0].Blocker == "no release has been planned — run 'relicta plan' in "+member.Path {
		t.Error("an unreachable store was reported as an unplanned release, which sends the " +
			"operator to 'relicta plan' for a problem plan cannot fix")
	}
	if states[0].Blocker == "" {
		t.Error("no blocker was reported at all")
	}
}

// The connection is released per member. A group of twenty would otherwise hold twenty open
// databases for the length of the check.
func TestTheMembersStoreIsClosedAfterItIsRead(t *testing.T) {
	member := appmultirepo.Member{Name: "svc-a", Path: t.TempDir()}
	closer := &countingCloser{}

	r := &Readiness{storeFor: func(ctx context.Context, path string) (ports.ReleaseRunRepository, io.Closer, error) {
		return approvedRunIn(ctx, t, path), closer, nil
	}}

	r.Check(context.Background(), []appmultirepo.Member{member})

	if closer.closed != 1 {
		t.Errorf("the store was closed %d times, want once: a check that leaks a connection "+
			"per member holds one open database for every repository in the group", closer.closed)
	}
}

// The default store resolver reads the member's own configuration, not the caller's. Asserted
// through the file the sqlite backend creates, because that is observable without a database.
func TestTheDefaultResolverHonorsTheMembersOwnConfiguration(t *testing.T) {
	member := t.TempDir()
	if err := os.WriteFile(filepath.Join(member, ".relicta.yaml"),
		[]byte("persistence:\n  backend: sqlite\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo, closer, err := openConfiguredStore(context.Background(), member)
	if err != nil {
		t.Fatalf("openConfiguredStore: %v", err)
	}
	if closer != nil {
		t.Cleanup(func() { _ = closer.Close() })
	}
	if repo == nil {
		t.Fatal("no repository was returned")
	}

	if _, err := os.Stat(filepath.Join(member, ".relicta", "relicta.db")); err != nil {
		t.Errorf("no sqlite database in the member: %v.\nThe member configured sqlite, so "+
			"opening its store must not produce the file adapter", err)
	}
}

type countingCloser struct{ closed int }

func (c *countingCloser) Close() error { c.closed++; return nil }
