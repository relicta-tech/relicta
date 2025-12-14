// Package release provides domain types for release management.
package release

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewRelease(t *testing.T) {
	id := ReleaseID("test-release-1")
	branch := "main"
	repoPath := "/path/to/repo"

	r := NewRelease(id, branch, repoPath)

	if r.ID() != id {
		t.Errorf("ID() = %v, want %v", r.ID(), id)
	}
	if r.Branch() != branch {
		t.Errorf("Branch() = %v, want %v", r.Branch(), branch)
	}
	if r.RepositoryPath() != repoPath {
		t.Errorf("RepositoryPath() = %v, want %v", r.RepositoryPath(), repoPath)
	}
	if r.State() != StateInitialized {
		t.Errorf("State() = %v, want %v", r.State(), StateInitialized)
	}
	if r.IsApproved() {
		t.Error("IsApproved() = true, want false")
	}
	if len(r.DomainEvents()) != 1 {
		t.Errorf("DomainEvents() length = %d, want 1", len(r.DomainEvents()))
	}
	if r.CreatedAt().IsZero() {
		t.Error("CreatedAt() is zero")
	}
	if r.UpdatedAt().IsZero() {
		t.Error("UpdatedAt() is zero")
	}
}

func TestRelease_SetPlan(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Create a change set with commits
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	commit := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add new feature")
	changeSet.AddCommit(commit)

	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)

	err := r.SetPlan(plan)
	if err != nil {
		t.Fatalf("SetPlan() error = %v", err)
	}

	if r.State() != StatePlanned {
		t.Errorf("State() = %v, want %v", r.State(), StatePlanned)
	}
	if r.Plan() != plan {
		t.Error("Plan() did not return the set plan")
	}

	// Verify event was added
	events := r.DomainEvents()
	found := false
	for _, e := range events {
		if e.EventName() == "release.planned" {
			found = true
			break
		}
	}
	if !found {
		t.Error("release.planned event not found")
	}
}

func TestRelease_SetPlan_NilPlan(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	err := r.SetPlan(nil)
	if err == nil {
		t.Error("SetPlan(nil) should return error")
	}
	if err != ErrNilPlan {
		t.Errorf("SetPlan(nil) error = %v, want %v", err, ErrNilPlan)
	}
}

func TestRelease_SetPlan_InvalidState(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)

	// Set plan once
	_ = r.SetPlan(plan)

	// Set version to transition past planned
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")

	// Generate notes
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})

	// Approve
	_ = r.Approve("user", false)

	// Try to set plan again from approved state - should fail
	err := r.SetPlan(plan)
	if err == nil {
		t.Error("SetPlan() from StateApproved should return error")
	}
}

func TestRelease_SetVersion(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// First set plan
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)

	// Then set version
	ver := version.MustParse("1.1.0")
	tagName := "v1.1.0"

	err := r.SetVersion(ver, tagName)
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}

	if r.State() != StateVersioned {
		t.Errorf("State() = %v, want %v", r.State(), StateVersioned)
	}
	if r.Version() == nil || r.Version().String() != "1.1.0" {
		t.Errorf("Version() = %v, want 1.1.0", r.Version())
	}
	if r.TagName() != tagName {
		t.Errorf("TagName() = %v, want %v", r.TagName(), tagName)
	}
}

func TestRelease_SetNotes(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to versioned state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")

	// Set notes
	notes := &ReleaseNotes{
		Changelog:   "## 1.1.0\n\n- New feature",
		Summary:     "Release summary",
		AIGenerated: true,
		GeneratedAt: time.Now(),
	}

	err := r.SetNotes(notes)
	if err != nil {
		t.Fatalf("SetNotes() error = %v", err)
	}

	if r.State() != StateNotesGenerated {
		t.Errorf("State() = %v, want %v", r.State(), StateNotesGenerated)
	}
	if r.Notes() != notes {
		t.Error("Notes() did not return the set notes")
	}
}

func TestRelease_SetNotes_NilNotes(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")

	err := r.SetNotes(nil)
	if err == nil {
		t.Error("SetNotes(nil) should return error")
	}
	if err != ErrNilNotes {
		t.Errorf("SetNotes(nil) error = %v, want %v", err, ErrNilNotes)
	}
}

func TestRelease_UpdateNotes(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to notes_generated state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	notes := &ReleaseNotes{
		Changelog:   "Initial changelog",
		Summary:     "Initial summary",
		AIGenerated: true,
		GeneratedAt: time.Now(),
	}
	_ = r.SetNotes(notes)

	// Verify we're in the right state
	if r.State() != StateNotesGenerated {
		t.Fatalf("Expected state %s, got %s", StateNotesGenerated, r.State())
	}

	// Update notes
	newChangelog := "Updated changelog content"
	err := r.UpdateNotes(newChangelog)
	if err != nil {
		t.Errorf("UpdateNotes() unexpected error: %v", err)
	}

	// Verify the update
	if r.notes.Changelog != newChangelog {
		t.Errorf("Changelog = %q, want %q", r.notes.Changelog, newChangelog)
	}
	if r.notes.Summary != notes.Summary {
		t.Errorf("Summary should be preserved, got %q, want %q", r.notes.Summary, notes.Summary)
	}
	if r.notes.AIGenerated != false {
		t.Error("AIGenerated should be false after manual update")
	}
	if r.notes.GeneratedAt != notes.GeneratedAt {
		t.Error("GeneratedAt should be preserved")
	}

	// State should remain notes_generated
	if r.State() != StateNotesGenerated {
		t.Errorf("State should remain %s after UpdateNotes, got %s", StateNotesGenerated, r.State())
	}

	// Should have generated an event
	events := r.DomainEvents()
	found := false
	for _, e := range events {
		if e.EventName() == "release.notes_updated" {
			found = true
			break
		}
	}
	if !found {
		t.Error("UpdateNotes should generate release.notes_updated event")
	}
}

func TestRelease_UpdateNotes_InvalidState(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(r *Release)
		state     ReleaseState
	}{
		{
			name:      "initialized state",
			setupFunc: func(r *Release) {},
			state:     StateInitialized,
		},
		{
			name: "planned state",
			setupFunc: func(r *Release) {
				changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
				plan := NewReleasePlan(
					version.MustParse("1.0.0"),
					version.MustParse("1.1.0"),
					changes.ReleaseTypeMinor,
					changeSet,
					false,
				)
				_ = r.SetPlan(plan)
			},
			state: StatePlanned,
		},
		{
			name: "versioned state",
			setupFunc: func(r *Release) {
				changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
				plan := NewReleasePlan(
					version.MustParse("1.0.0"),
					version.MustParse("1.1.0"),
					changes.ReleaseTypeMinor,
					changeSet,
					false,
				)
				_ = r.SetPlan(plan)
				_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
			},
			state: StateVersioned,
		},
		{
			name: "approved state",
			setupFunc: func(r *Release) {
				changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
				plan := NewReleasePlan(
					version.MustParse("1.0.0"),
					version.MustParse("1.1.0"),
					changes.ReleaseTypeMinor,
					changeSet,
					false,
				)
				_ = r.SetPlan(plan)
				_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
				notes := &ReleaseNotes{
					Changelog:   "changelog",
					Summary:     "summary",
					AIGenerated: true,
					GeneratedAt: time.Now(),
				}
				_ = r.SetNotes(notes)
				_ = r.Approve("tester", false)
			},
			state: StateApproved,
		},
		{
			name: "publishing state",
			setupFunc: func(r *Release) {
				changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
				plan := NewReleasePlan(
					version.MustParse("1.0.0"),
					version.MustParse("1.1.0"),
					changes.ReleaseTypeMinor,
					changeSet,
					false,
				)
				_ = r.SetPlan(plan)
				_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
				notes := &ReleaseNotes{
					Changelog:   "changelog",
					Summary:     "summary",
					AIGenerated: true,
					GeneratedAt: time.Now(),
				}
				_ = r.SetNotes(notes)
				_ = r.Approve("tester", false)
				_ = r.StartPublishing([]string{"github"})
			},
			state: StatePublishing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRelease("test-1", "main", "/repo")
			tt.setupFunc(r)

			if r.State() != tt.state {
				t.Fatalf("Expected state %s, got %s", tt.state, r.State())
			}

			err := r.UpdateNotes("new changelog")
			if err == nil {
				t.Errorf("UpdateNotes() should return error in state %s", tt.state)
			}
		})
	}
}

func TestRelease_UpdateNotes_NilNotes(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - get to versioned state (notes not yet set)
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")

	// Manually force the state to notes_generated without setting notes
	// This tests the defensive check in UpdateNotes
	r.state = StateNotesGenerated

	err := r.UpdateNotes("new changelog")
	if err == nil {
		t.Error("UpdateNotes() should return error when notes is nil")
	}
	if err != ErrNilNotes {
		t.Errorf("UpdateNotes() error = %v, want %v", err, ErrNilNotes)
	}
}

func TestRelease_UpdateNotes_PreservesOriginalGeneratedAt(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")

	originalGeneratedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	notes := &ReleaseNotes{
		Changelog:   "Initial changelog",
		Summary:     "Initial summary",
		AIGenerated: true,
		GeneratedAt: originalGeneratedAt,
	}
	_ = r.SetNotes(notes)

	// Update notes multiple times
	_ = r.UpdateNotes("First update")
	_ = r.UpdateNotes("Second update")

	// GeneratedAt should still be the original time
	if !r.notes.GeneratedAt.Equal(originalGeneratedAt) {
		t.Errorf("GeneratedAt = %v, want %v", r.notes.GeneratedAt, originalGeneratedAt)
	}
}

func TestRelease_Approve(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to notes_generated state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})

	// Approve
	err := r.Approve("testuser", false)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if r.State() != StateApproved {
		t.Errorf("State() = %v, want %v", r.State(), StateApproved)
	}
	if !r.IsApproved() {
		t.Error("IsApproved() = false, want true")
	}
	if r.Approval() == nil {
		t.Fatal("Approval() = nil")
	}
	if r.Approval().ApprovedBy != "testuser" {
		t.Errorf("ApprovedBy = %v, want testuser", r.Approval().ApprovedBy)
	}
	if r.Approval().AutoApproved {
		t.Error("AutoApproved = true, want false")
	}
}

func TestRelease_Approve_AutoApprove(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})

	// Auto-approve
	err := r.Approve("ci-bot", true)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if !r.Approval().AutoApproved {
		t.Error("AutoApproved = false, want true")
	}
}

func TestRelease_StartPublishing(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to approved state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})
	_ = r.Approve("user", false)

	// Start publishing
	plugins := []string{"github", "npm"}
	err := r.StartPublishing(plugins)
	if err != nil {
		t.Fatalf("StartPublishing() error = %v", err)
	}

	if r.State() != StatePublishing {
		t.Errorf("State() = %v, want %v", r.State(), StatePublishing)
	}
}

func TestRelease_MarkPublished(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to publishing state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})
	_ = r.Approve("user", false)
	_ = r.StartPublishing([]string{"github"})

	// Mark published
	releaseURL := "https://github.com/owner/repo/releases/v1.1.0"
	err := r.MarkPublished(releaseURL)
	if err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	if r.State() != StatePublished {
		t.Errorf("State() = %v, want %v", r.State(), StatePublished)
	}
	if r.PublishedAt() == nil {
		t.Error("PublishedAt() = nil")
	}
}

func TestRelease_MarkFailed(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to publishing state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})
	_ = r.Approve("user", false)
	_ = r.StartPublishing([]string{"github"})

	// Mark failed
	err := r.MarkFailed("network error", true)
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	if r.State() != StateFailed {
		t.Errorf("State() = %v, want %v", r.State(), StateFailed)
	}
	if r.LastError() != "network error" {
		t.Errorf("LastError() = %v, want 'network error'", r.LastError())
	}
}

func TestRelease_Cancel(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)

	// Cancel
	err := r.Cancel("user requested", "admin")
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	if r.State() != StateCanceled {
		t.Errorf("State() = %v, want %v", r.State(), StateCanceled)
	}
	if r.LastError() != "user requested" {
		t.Errorf("LastError() = %v, want 'user requested'", r.LastError())
	}
}

func TestRelease_Retry_FromFailed(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Setup - transition to failed state
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)
	_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})
	_ = r.Approve("user", false)
	_ = r.StartPublishing([]string{"github"})
	_ = r.MarkFailed("error", true)

	// Retry
	err := r.Retry()
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	// Should reset to planned state since plan exists
	if r.State() != StatePlanned {
		t.Errorf("State() = %v, want %v", r.State(), StatePlanned)
	}
	if r.LastError() != "" {
		t.Errorf("LastError() = %v, want empty", r.LastError())
	}
}

func TestRelease_Retry_FromCanceled(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Cancel from initialized
	_ = r.Cancel("canceled", "user")

	// Retry
	err := r.Retry()
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	// Should reset to initialized since no plan
	if r.State() != StateInitialized {
		t.Errorf("State() = %v, want %v", r.State(), StateInitialized)
	}
}

func TestRelease_Retry_InvalidState(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	// Try to retry from initialized state
	err := r.Retry()
	if err == nil {
		t.Error("Retry() from StateInitialized should return error")
	}
}

func TestRelease_CanProceedToPublish(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Release)
		expected bool
	}{
		{
			name: "approved with all required fields",
			setup: func(r *Release) {
				changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
				plan := NewReleasePlan(
					version.MustParse("1.0.0"),
					version.MustParse("1.1.0"),
					changes.ReleaseTypeMinor,
					changeSet,
					false,
				)
				_ = r.SetPlan(plan)
				_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
				_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})
				_ = r.Approve("user", false)
			},
			expected: true,
		},
		{
			name: "not approved",
			setup: func(r *Release) {
				changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
				plan := NewReleasePlan(
					version.MustParse("1.0.0"),
					version.MustParse("1.1.0"),
					changes.ReleaseTypeMinor,
					changeSet,
					false,
				)
				_ = r.SetPlan(plan)
				_ = r.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
				_ = r.SetNotes(&ReleaseNotes{Changelog: "test"})
			},
			expected: false,
		},
		{
			name:     "initialized only",
			setup:    func(_ *Release) {},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRelease("test-1", "main", "/repo")
			tt.setup(r)

			if got := r.CanProceedToPublish(); got != tt.expected {
				t.Errorf("CanProceedToPublish() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRelease_RecordPluginExecution(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	initialEventCount := len(r.DomainEvents())

	r.RecordPluginExecution("github", "post_publish", true, "success", 5*time.Second)

	events := r.DomainEvents()
	if len(events) != initialEventCount+1 {
		t.Errorf("DomainEvents() length = %d, want %d", len(events), initialEventCount+1)
	}

	lastEvent := events[len(events)-1]
	if lastEvent.EventName() != "release.plugin_executed" {
		t.Errorf("EventName() = %v, want release.plugin_executed", lastEvent.EventName())
	}
}

func TestRelease_ClearDomainEvents(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	if len(r.DomainEvents()) == 0 {
		t.Fatal("Expected initial event")
	}

	r.ClearDomainEvents()

	if len(r.DomainEvents()) != 0 {
		t.Errorf("DomainEvents() length = %d, want 0", len(r.DomainEvents()))
	}
}

func TestRelease_SetRepositoryName(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")

	r.SetRepositoryName("my-repo")

	if r.RepositoryName() != "my-repo" {
		t.Errorf("RepositoryName() = %v, want my-repo", r.RepositoryName())
	}
}

func TestRelease_Summary(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")
	r.SetRepositoryName("my-repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	commit := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add feature")
	changeSet.AddCommit(commit)

	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = r.SetPlan(plan)

	summary := r.Summary()

	if summary.ID != "test-1" {
		t.Errorf("ID = %v, want test-1", summary.ID)
	}
	if summary.State != StatePlanned {
		t.Errorf("State = %v, want %v", summary.State, StatePlanned)
	}
	if summary.Branch != "main" {
		t.Errorf("Branch = %v, want main", summary.Branch)
	}
	if summary.Repository != "my-repo" {
		t.Errorf("Repository = %v, want my-repo", summary.Repository)
	}
	if summary.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %v, want 1.0.0", summary.CurrentVersion)
	}
	if summary.NextVersion != "1.1.0" {
		t.Errorf("NextVersion = %v, want 1.1.0", summary.NextVersion)
	}
	if summary.ReleaseType != "minor" {
		t.Errorf("ReleaseType = %v, want minor", summary.ReleaseType)
	}
	if summary.CommitCount != 1 {
		t.Errorf("CommitCount = %d, want 1", summary.CommitCount)
	}
	if summary.IsApproved {
		t.Error("IsApproved = true, want false")
	}
}

func TestRelease_ReconstructState(t *testing.T) {
	r := NewRelease("test-1", "main", "/repo")
	r.ClearDomainEvents() // Clear initial events

	ver := version.MustParse("2.0.0")
	notes := &ReleaseNotes{Changelog: "test changelog"}
	approval := &Approval{ApprovedBy: "admin", AutoApproved: false}
	createdAt := time.Now().Add(-1 * time.Hour)
	updatedAt := time.Now()
	publishedAt := time.Now()

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("2.0.0"),
		changes.ReleaseTypeMajor,
		changeSet,
		false,
	)

	r.ReconstructState(
		StatePublished,
		plan,
		&ver,
		"v2.0.0",
		notes,
		approval,
		createdAt,
		updatedAt,
		&publishedAt,
		"",
	)

	if r.State() != StatePublished {
		t.Errorf("State() = %v, want %v", r.State(), StatePublished)
	}
	if r.Plan() != plan {
		t.Error("Plan() mismatch")
	}
	if r.Version().String() != "2.0.0" {
		t.Errorf("Version() = %v, want 2.0.0", r.Version())
	}
	if r.TagName() != "v2.0.0" {
		t.Errorf("TagName() = %v, want v2.0.0", r.TagName())
	}
	if r.Notes() != notes {
		t.Error("Notes() mismatch")
	}
	// Approval() returns a copy to preserve aggregate boundary, so compare values
	gotApproval := r.Approval()
	if gotApproval == nil || gotApproval.ApprovedBy != approval.ApprovedBy ||
		gotApproval.AutoApproved != approval.AutoApproved {
		t.Error("Approval() mismatch")
	}
	if r.CreatedAt() != createdAt {
		t.Error("CreatedAt() mismatch")
	}
	if r.UpdatedAt() != updatedAt {
		t.Error("UpdatedAt() mismatch")
	}
	if r.PublishedAt() == nil || *r.PublishedAt() != publishedAt {
		t.Error("PublishedAt() mismatch")
	}
	// Should not generate events during reconstruction
	if len(r.DomainEvents()) != 0 {
		t.Errorf("DomainEvents() length = %d, want 0", len(r.DomainEvents()))
	}
}

func TestRelease_FullWorkflow(t *testing.T) {
	// Test a complete release workflow
	r := NewRelease("release-001", "main", "/path/to/repo")
	r.SetRepositoryName("test-repo")

	// 1. Plan
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	commit1 := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add feature A")
	commit2 := changes.NewConventionalCommit("def456", changes.CommitTypeFix, "fix bug B")
	changeSet.AddCommit(commit1)
	changeSet.AddCommit(commit2)

	plan := NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	if err := r.SetPlan(plan); err != nil {
		t.Fatalf("SetPlan() error = %v", err)
	}

	// 2. Version
	if err := r.SetVersion(version.MustParse("1.1.0"), "v1.1.0"); err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}

	// 3. Generate Notes
	notes := &ReleaseNotes{
		Changelog:   "## [1.1.0]\n\n### Features\n- Add feature A\n\n### Fixes\n- Fix bug B",
		Summary:     "Minor release with new feature and bug fix",
		AIGenerated: false,
		GeneratedAt: time.Now(),
	}
	if err := r.SetNotes(notes); err != nil {
		t.Fatalf("SetNotes() error = %v", err)
	}

	// 4. Approve
	if err := r.Approve("releaser", false); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	// Verify can proceed to publish
	if !r.CanProceedToPublish() {
		t.Error("CanProceedToPublish() = false after approval")
	}

	// 5. Start Publishing
	if err := r.StartPublishing([]string{"github", "npm"}); err != nil {
		t.Fatalf("StartPublishing() error = %v", err)
	}

	// 6. Record plugin execution
	r.RecordPluginExecution("github", "post_publish", true, "Created release", 2*time.Second)
	r.RecordPluginExecution("npm", "post_publish", true, "Published to npm", 5*time.Second)

	// 7. Mark Published
	if err := r.MarkPublished("https://github.com/owner/repo/releases/v1.1.0"); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	// Final assertions
	if r.State() != StatePublished {
		t.Errorf("Final state = %v, want %v", r.State(), StatePublished)
	}
	if r.PublishedAt() == nil {
		t.Error("PublishedAt() should not be nil")
	}

	// Check all events were recorded
	events := r.DomainEvents()
	expectedEvents := []string{
		"release.initialized",
		"release.planned",
		"release.versioned",
		"release.notes_generated",
		"release.approved",
		"release.publishing_started",
		"release.plugin_executed",
		"release.plugin_executed",
		"release.published",
	}

	if len(events) != len(expectedEvents) {
		t.Errorf("DomainEvents() count = %d, want %d", len(events), len(expectedEvents))
	}

	for i, expected := range expectedEvents {
		if i < len(events) && events[i].EventName() != expected {
			t.Errorf("Event[%d] = %v, want %v", i, events[i].EventName(), expected)
		}
	}
}

func TestRelease_ValidateInvariants(t *testing.T) {
	t.Run("new release is valid", func(t *testing.T) {
		r := NewRelease("test-1", "main", "/repo")
		if !r.IsValid() {
			violations := r.InvariantViolations()
			t.Errorf("expected new release to be valid, got violations: %v", violations)
		}
	})

	t.Run("empty ID is invalid", func(t *testing.T) {
		r := NewRelease("", "main", "/repo")
		violations := r.InvariantViolations()
		found := false
		for _, v := range violations {
			if v.Name == "NonEmptyID" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected NonEmptyID violation")
		}
	})

	t.Run("empty branch is invalid", func(t *testing.T) {
		r := NewRelease("test-1", "", "/repo")
		violations := r.InvariantViolations()
		found := false
		for _, v := range violations {
			if v.Name == "NonEmptyBranch" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected NonEmptyBranch violation")
		}
	})

	t.Run("release through full workflow is valid", func(t *testing.T) {
		r := NewRelease("test-1", "main", "/repo")

		changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
		changeSet.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "feature"))

		plan := NewReleasePlan(version.Initial, version.MustParse("1.0.0"), changes.ReleaseTypeMinor, changeSet, false)
		_ = r.SetPlan(plan)
		_ = r.SetVersion(version.MustParse("1.0.0"), "v1.0.0")
		_ = r.SetNotes(&ReleaseNotes{Changelog: "test", Summary: "test"})
		_ = r.Approve("tester", false)
		_ = r.StartPublishing([]string{"github"})
		_ = r.MarkPublished("http://example.com")

		if !r.IsValid() {
			violations := r.InvariantViolations()
			t.Errorf("expected published release to be valid, got violations: %v", violations)
		}
	})

	t.Run("ValidateInvariants returns all invariants", func(t *testing.T) {
		r := NewRelease("test-1", "main", "/repo")
		invariants := r.ValidateInvariants()

		// Should have 8 invariants checked
		if len(invariants) != 8 {
			t.Errorf("expected 8 invariants, got %d", len(invariants))
		}

		// All should be valid for a new release
		for _, inv := range invariants {
			if !inv.Valid {
				t.Errorf("invariant %s should be valid for new release", inv.Name)
			}
		}
	})

	t.Run("InvariantViolations returns empty for valid release", func(t *testing.T) {
		r := NewRelease("test-1", "main", "/repo")
		violations := r.InvariantViolations()
		if len(violations) != 0 {
			t.Errorf("expected no violations for valid release, got %d", len(violations))
		}
	})
}
