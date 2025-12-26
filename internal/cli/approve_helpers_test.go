// Package cli provides the command-line interface for Relicta.
package cli

import (
	"os"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/ui"
)

func TestShouldUseInteractiveApproval(t *testing.T) {
	// Save original state
	origInteractive := approveInteractive
	origCIMode := ciMode
	origApproveYes := approveYes
	defer func() {
		approveInteractive = origInteractive
		ciMode = origCIMode
		approveYes = origApproveYes
	}()

	tests := []struct {
		name        string
		interactive bool
		ciMode      bool
		approveYes  bool
		want        bool
	}{
		{"interactive enabled", true, false, false, true},
		{"interactive disabled", false, false, false, false},
		{"CI mode overrides", true, true, false, false},
		{"CI mode disabled", false, true, false, false},
		{"approveYes overrides", true, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approveInteractive = tt.interactive
			ciMode = tt.ciMode
			approveYes = tt.approveYes
			got := shouldUseInteractiveApproval()
			if got != tt.want {
				t.Errorf("shouldUseInteractiveApproval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetApproverName(t *testing.T) {
	tests := []struct {
		name    string
		envUser string
		want    string
	}{
		{
			name:    "with USER env var",
			envUser: "testuser",
			want:    "testuser",
		},
		{
			name:    "without USER env var",
			envUser: "",
			want:    "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore USER env var
			oldUser := os.Getenv("USER")
			defer func() {
				if oldUser != "" {
					os.Setenv("USER", oldUser)
				} else {
					os.Unsetenv("USER")
				}
			}()

			// Set test value
			if tt.envUser != "" {
				os.Setenv("USER", tt.envUser)
			} else {
				os.Unsetenv("USER")
			}

			got := getApproverName()
			if got != tt.want {
				t.Errorf("getApproverName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintApproveNextSteps(t *testing.T) {
	// Just verify it doesn't panic
	printApproveNextSteps()
}

func TestIsReleaseAlreadyApproved(t *testing.T) {
	tests := []struct {
		name  string
		state release.ReleaseState
		want  bool
	}{
		{
			name:  "approved state",
			state: release.StateApproved,
			want:  true,
		},
		{
			name:  "planned state",
			state: release.StatePlanned,
			want:  false,
		},
		{
			name:  "published state",
			state: release.StatePublished,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a release with the specified state
			v1, _ := version.Parse("1.0.0")
			v2, _ := version.Parse("1.1.0")

			changeSet := changes.NewChangeSet(
				changes.ChangeSetID("test-changeset"),
				"test-repo",
				"main",
			)

			plan := release.NewReleasePlan(
				v1,
				v2,
				changes.ReleaseTypeMinor,
				changeSet,
				false,
			)

			rel := release.NewRelease(
				release.ReleaseID("test-release"),
				"main",
				"test-repo",
			)

			// Set plan to transition to StatePlanned
			if err := release.SetPlan(rel, plan); err != nil {
				t.Fatalf("failed to set plan: %v", err)
			}

			// For approved state, approve the release
			if tt.state == release.StateApproved {
				// First set version and notes to allow approval
				if err := rel.SetVersion(v2, "v1.1.0"); err != nil {
					t.Fatalf("failed to set version: %v", err)
				}

				notes := &release.ReleaseNotes{
					Text:        "Test changelog",
					Provider:    "test",
					GeneratedAt: time.Now(),
				}
				if err := rel.SetNotes(notes); err != nil {
					t.Fatalf("failed to set notes: %v", err)
				}

				if err := rel.Approve("test-user", false); err != nil {
					t.Fatalf("failed to approve release: %v", err)
				}
			}

			got := isReleaseAlreadyApproved(rel)
			if got != tt.want {
				t.Errorf("isReleaseAlreadyApproved() = %v, want %v for state %v", got, tt.want, tt.state)
			}
		})
	}
}

func TestHandleEditApprovalResultWithoutNotes(t *testing.T) {
	rel := release.NewRelease(release.ReleaseID("no-notes-edit"), "main", ".")
	notes, proceed, err := handleEditApprovalResult(rel)
	if err != nil {
		t.Fatalf("handleEditApprovalResult returned error: %v", err)
	}
	if notes != nil {
		t.Fatalf("expected nil notes when none exist, got %v", notes)
	}
	if proceed {
		t.Fatal("expected proceed=false when no notes to edit")
	}
}

func TestProcessTUIApprovalResultPaths(t *testing.T) {
	rel := release.NewRelease(release.ReleaseID("tui-result"), "main", ".")
	tests := []struct {
		name    string
		result  ui.ApprovalResult
		wantErr bool
		wantOk  bool
	}{
		{"accepted", ui.ApprovalAccepted, false, true},
		{"rejected", ui.ApprovalRejected, false, false},
		{"edit without notes", ui.ApprovalEdit, false, false},
		{"unknown", ui.ApprovalRejected + 100, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notes, ok, err := processTUIApprovalResult(tt.result, rel)
			if (err != nil) != tt.wantErr {
				t.Fatalf("processTUIApprovalResult error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantOk {
				t.Fatalf("processTUIApprovalResult proceed = %v, want %v", ok, tt.wantOk)
			}
			if tt.result != ui.ApprovalEdit && notes != nil {
				t.Fatalf("expected nil notes for %v", tt.name)
			}
		})
	}
}

func TestBuildTUISummary_IncludesReleaseNotesAndPlugins(t *testing.T) {
	origCfg := cfg
	defer func() { cfg = origCfg }()

	cfg = config.DefaultConfig()
	enabled := true
	cfg.Plugins = []config.PluginConfig{
		{Name: "test-plugin", Enabled: &enabled},
	}

	rel := newTestRelease(t, "tui-summary-test")

	summary := buildTUISummary(rel)
	if summary.FeatureCount == 0 {
		t.Fatalf("expected feature count > 0, got %d", summary.FeatureCount)
	}
	if summary.ReleaseNotes != "changelog" {
		t.Fatalf("unexpected release notes: %s", summary.ReleaseNotes)
	}
	if len(summary.Plugins) != 1 || summary.Plugins[0] != "test-plugin" {
		t.Fatalf("unexpected plugins: %v", summary.Plugins)
	}
}
