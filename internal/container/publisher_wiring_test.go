package container

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// The container assembled OutcomeTracker → WebhookPublisher → InMemoryEventPublisher,
// logged it as initialized, and handed it to nobody: the only production caller of any
// EventPublisher was FileUnitOfWork, which nothing constructs outside a test. So no
// release ever published an event. Configured webhooks never fired, and no failed or
// canceled run was ever recorded, which left change failure rate to be computed from a
// history containing almost only successes.
//
// The seam is the repository's Save, because every use case already persists through it —
// ten calls across plan, bump, notes, approve, publish and retry. These tests assert the
// wiring at both of the seams that exist, since the CLI reaches the aggregate two ways and
// only one of them was covered when the first fix landed: the release services, and
// app.ReleaseRepository() via the bridge, which is what cancel, clean, rollback and bump
// use.

type recordingPublisher struct {
	mu     sync.Mutex
	events []domain.DomainEvent
}

func (p *recordingPublisher) Publish(_ context.Context, events ...domain.DomainEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, events...)
	return nil
}

func (p *recordingPublisher) names() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.events))
	for _, e := range p.events {
		out = append(out, e.EventName())
	}
	return out
}

// runIn builds an aggregate with an uncommitted event, in a repository directory the
// file-based repository can write to.
func runIn(t *testing.T, repoRoot string) *domain.ReleaseRun {
	t.Helper()
	return domain.NewReleaseRun(
		"acme/widget", repoRoot, "main",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"},
		"config-hash", "plugin-hash",
	)
}

func TestTheReleaseServicesRepositoryPublishesOnSave(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".relicta"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	publisher := &recordingPublisher{}
	services, err := domainrelease.NewServices(domainrelease.Config{
		RepoRoot:       repoRoot,
		EventPublisher: publisher,
	})
	if err != nil {
		t.Fatalf("NewServices: %v", err)
	}

	run := runIn(t, repoRoot)
	if err := services.Repository.Save(context.Background(), run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := publisher.names(); len(got) == 0 {
		t.Fatal("saving a run published no events: the outcome tracker and every webhook " +
			"subscriber are unreachable, so a release records nothing and notifies nobody")
	}

	// And the aggregate must not keep them, or the next save republishes the same events.
	if len(run.DomainEvents()) != 0 {
		t.Errorf("%d events remain on the aggregate after publication, so the next save "+
			"would emit them again", len(run.DomainEvents()))
	}
}

// Without a publisher the services must still work — the field is optional, and every
// existing caller that passes no publisher would otherwise break.
func TestTheReleaseServicesWorkWithoutAPublisher(t *testing.T) {
	repoRoot := t.TempDir()

	services, err := domainrelease.NewServices(domainrelease.Config{RepoRoot: repoRoot})
	if err != nil {
		t.Fatalf("NewServices: %v", err)
	}
	if err := services.Repository.Save(context.Background(), runIn(t, repoRoot)); err != nil {
		t.Fatalf("Save without a publisher: %v", err)
	}
}

// The bridge is the other seam. cancel, clean, rollback and bump save through
// app.ReleaseRepository(), and the first version of this fix decorated only the release
// services — so canceling a release still published nothing, and the outcome tracker still
// never saw a terminal event it could record.
func TestTheBridgePublishesOnSave(t *testing.T) {
	repoRoot := t.TempDir()
	publisher := &recordingPublisher{}

	// nil store, so the bridge falls back to the file adapter: this case is about the
	// publisher seam, and the store the container resolves is covered in
	// release_run_backend_test.go.
	bridge := newReleaseRepoBridge(repoRoot, publisher, nil)
	if err := bridge.Save(context.Background(), runIn(t, repoRoot)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := publisher.names(); len(got) == 0 {
		t.Fatal("saving through the bridge published no events: every command that reaches " +
			"the aggregate this way — cancel, clean, rollback, bump — records nothing")
	}
}

func TestTheBridgeWorksWithoutAPublisher(t *testing.T) {
	repoRoot := t.TempDir()
	bridge := newReleaseRepoBridge(repoRoot, nil, nil)
	if err := bridge.Save(context.Background(), runIn(t, repoRoot)); err != nil {
		t.Fatalf("Save without a publisher: %v", err)
	}
}
