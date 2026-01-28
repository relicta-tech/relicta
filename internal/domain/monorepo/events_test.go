package monorepo

import (
	"testing"
	"time"
)

func TestDomainEvents(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		event     DomainEvent
		eventName string
	}{
		{"MonorepoReleaseCreated", MonorepoReleaseCreated{Timestamp: now}, "monorepo.release.created"},
		{"PackageAddedToRelease", PackageAddedToRelease{Timestamp: now}, "monorepo.package.added"},
		{"MonorepoReleasePlanned", MonorepoReleasePlanned{Timestamp: now}, "monorepo.release.planned"},
		{"MonorepoReleaseVersioned", MonorepoReleaseVersioned{Timestamp: now}, "monorepo.release.versioned"},
		{"MonorepoReleaseNotesReady", MonorepoReleaseNotesReady{Timestamp: now}, "monorepo.release.notes_ready"},
		{"MonorepoReleaseApproved", MonorepoReleaseApproved{Timestamp: now}, "monorepo.release.approved"},
		{"MonorepoReleasePublishing", MonorepoReleasePublishing{Timestamp: now}, "monorepo.release.publishing"},
		{"MonorepoReleasePublished", MonorepoReleasePublished{Timestamp: now}, "monorepo.release.published"},
		{"MonorepoReleaseFailed", MonorepoReleaseFailed{Timestamp: now}, "monorepo.release.failed"},
		{"MonorepoReleaseCanceled", MonorepoReleaseCanceled{Timestamp: now}, "monorepo.release.canceled"},
		{"PackageVersionBumped", PackageVersionBumped{Timestamp: now}, "monorepo.package.version_bumped"},
		{"PackageNotesGenerated", PackageNotesGenerated{Timestamp: now}, "monorepo.package.notes_generated"},
		{"PackagePublished", PackagePublished{Timestamp: now}, "monorepo.package.published"},
		{"DependencyVersionUpdated", DependencyVersionUpdated{Timestamp: now}, "monorepo.dependency.updated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.EventName() != tt.eventName {
				t.Errorf("EventName() = %s, want %s", tt.event.EventName(), tt.eventName)
			}
			if !tt.event.OccurredAt().Equal(now) {
				t.Errorf("OccurredAt() = %v, want %v", tt.event.OccurredAt(), now)
			}
		})
	}
}
