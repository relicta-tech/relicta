package container

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// Pushing a tag is the one action in a release that cannot be undone: in any
// repository whose workflows trigger on tag push, it starts a real public
// release. versioning.git_push exists to gate exactly that, defaults to false,
// and the generated config warns about it in capitals.
//
// It did nothing. WithSkipPush was defined and called from nowhere, so skipPush
// stayed false and executeTagStep pushed on every publish. `relicta publish`
// printed "Push: false", reported "push_tag": false in --json, and pushed the tag
// anyway — a safety control reporting itself as engaged while doing nothing,
// which is worse than not having one.
//
// These tests pin the wiring rather than the push mechanics, because the wiring
// is what was missing.

// recordingTagCreator observes what the publisher asks of git without touching a
// repository.
type recordingTagCreator struct {
	created  []string
	pushed   []string
	existing map[string]bool
}

func (r *recordingTagCreator) CreateTag(_ context.Context, name, _ string) error {
	r.created = append(r.created, name)
	return nil
}

func (r *recordingTagCreator) PushTag(_ context.Context, name, _ string) error {
	r.pushed = append(r.pushed, name)
	return nil
}

func (r *recordingTagCreator) TagExists(_ context.Context, name string) (bool, error) {
	return r.existing[name], nil
}

func tagStepRun(t *testing.T) *domain.ReleaseRun {
	t.Helper()
	run := domain.NewReleaseRun("repo", t.TempDir(), "v1.0.0", "abc123",
		[]domain.CommitSHA{"abc123"}, "cfg", "plugins")
	return run
}

func TestPublisher_PushDisabledPreventsThePush(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags, WithPushTags(false))

	run := tagStepRun(t)
	if _, err := publisher.executeTagStep(context.Background(), run); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.created) != 1 {
		t.Errorf("expected the tag to be created locally, got %v", tags.created)
	}
	if len(tags.pushed) != 0 {
		t.Errorf("tag was pushed despite pushing being disabled: %v — this is the irreversible "+
			"action git_push is meant to prevent", tags.pushed)
	}
}

func TestPublisher_PushEnabledPushes(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags, WithPushTags(true))

	run := tagStepRun(t)
	if _, err := publisher.executeTagStep(context.Background(), run); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.pushed) != 1 {
		t.Errorf("expected the tag to be pushed when pushing is enabled, got %v", tags.pushed)
	}
}

// The default must be safe. A publisher built without options previously pushed,
// which meant every construction site that forgot the option got the dangerous
// behavior — and every construction site did forget it.
func TestPublisher_DefaultDoesNotPush(t *testing.T) {
	tags := &recordingTagCreator{existing: map[string]bool{}}
	publisher := NewPublisherAdapter(nil, nil, tags)

	run := tagStepRun(t)
	if _, err := publisher.executeTagStep(context.Background(), run); err != nil {
		t.Fatalf("executeTagStep: %v", err)
	}

	if len(tags.pushed) != 0 {
		t.Errorf("a publisher constructed without options pushed %v; the default for "+
			"an irreversible action must be not to take it", tags.pushed)
	}
}
