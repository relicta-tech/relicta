package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

func TestRollbackCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"to-version flag", "to-version"},
		{"to-tag flag", "to-tag"},
		{"dry-run flag", "dry-run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rollbackCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("rollback command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestRollbackCommand_Configuration(t *testing.T) {
	if rollbackCmd == nil {
		t.Fatal("rollbackCmd is nil")
	}
	if rollbackCmd.Use != "rollback" {
		t.Errorf("rollbackCmd.Use = %v, want rollback", rollbackCmd.Use)
	}
	if rollbackCmd.RunE == nil {
		t.Error("rollbackCmd.RunE is nil")
	}
}

func TestResolveRollbackTarget(t *testing.T) {
	tests := []struct {
		name      string
		toVersion string
		toTag     string
		want      string
		wantErr   bool
	}{
		{
			name:    "neither flag specified",
			wantErr: true,
		},
		{
			name:      "both flags specified",
			toVersion: "1.2.3",
			toTag:     "v1.2.3",
			wantErr:   true,
		},
		{
			name:      "version flag",
			toVersion: "1.2.3",
			want:      "v1.2.3",
		},
		{
			name:  "tag flag",
			toTag: "v1.2.3",
			want:  "v1.2.3",
		},
		{
			name:  "tag flag without v prefix",
			toTag: "release-1.0",
			want:  "release-1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global state
			origVersion := rollbackToVersion
			origTag := rollbackToTag
			defer func() {
				rollbackToVersion = origVersion
				rollbackToTag = origTag
			}()

			rollbackToVersion = tt.toVersion
			rollbackToTag = tt.toTag

			got, err := resolveRollbackTarget()
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveRollbackTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolveRollbackTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

// rollbackTestGitRepo extends stubGitRepo to support tag lookup for rollback tests.
type rollbackTestGitRepo struct {
	stubGitRepo
	tags map[string]*sourcecontrol.Tag
}

func (r *rollbackTestGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	if tag, ok := r.tags[name]; ok {
		return tag, nil
	}
	return nil, nil
}

func TestExecuteRollback_TagNotFound(t *testing.T) {
	gitRepo := &rollbackTestGitRepo{
		tags: make(map[string]*sourcecontrol.Tag),
	}

	app := &commandTestApp{
		gitRepo:     gitRepo,
		releaseRepo: testReleaseRepo{err: release.ErrRunNotFound},
	}

	_, err := executeRollback(context.Background(), app, "v99.99.99", false)
	if err == nil {
		t.Error("executeRollback() should return error for non-existent tag")
	}
}

func TestExecuteRollback_DryRun(t *testing.T) {
	tag := sourcecontrol.NewTag("v1.2.3", sourcecontrol.CommitHash("abc123"))
	gitRepo := &rollbackTestGitRepo{
		tags: map[string]*sourcecontrol.Tag{
			"v1.2.3": tag,
		},
	}

	app := &commandTestApp{
		gitRepo:     gitRepo,
		releaseRepo: testReleaseRepo{err: release.ErrRunNotFound},
	}

	result, err := executeRollback(context.Background(), app, "v1.2.3", true)
	if err != nil {
		t.Fatalf("executeRollback() dry run error = %v", err)
	}

	if result.TargetTag != "v1.2.3" {
		t.Errorf("result.TargetTag = %v, want v1.2.3", result.TargetTag)
	}
	if result.TargetCommit != "abc123" {
		t.Errorf("result.TargetCommit = %v, want abc123", result.TargetCommit)
	}
	if !result.DryRun {
		t.Error("result.DryRun should be true")
	}
}

func TestExecuteRollback_Success(t *testing.T) {
	tag := sourcecontrol.NewTag("v1.2.3", sourcecontrol.CommitHash("abc123"))
	gitRepo := &rollbackTestGitRepo{
		tags: map[string]*sourcecontrol.Tag{
			"v1.2.3": tag,
		},
	}

	app := &commandTestApp{
		gitRepo:     gitRepo,
		releaseRepo: testReleaseRepo{err: release.ErrRunNotFound},
	}

	result, err := executeRollback(context.Background(), app, "v1.2.3", false)
	if err != nil {
		t.Fatalf("executeRollback() error = %v", err)
	}

	if result.TargetTag != "v1.2.3" {
		t.Errorf("result.TargetTag = %v, want v1.2.3", result.TargetTag)
	}
	if result.TargetCommit != "abc123" {
		t.Errorf("result.TargetCommit = %v, want abc123", result.TargetCommit)
	}
	if result.DryRun {
		t.Error("result.DryRun should be false")
	}
	if result.RevertTag == "" {
		t.Error("result.RevertTag should not be empty")
	}
	if result.RolledBackBy == "" {
		t.Error("result.RolledBackBy should not be empty")
	}
}

func TestOutputRollbackJSON(t *testing.T) {
	result := &rollbackResult{
		TargetTag:    "v1.2.3",
		TargetCommit: "abc123",
		RevertTag:    "rollback-to-v1.2.3-20260101-120000",
		DryRun:       false,
		RolledBackBy: "user@example.com",
		RolledBackAt: "2026-01-01T12:00:00Z",
	}

	err := outputRollbackJSON(result)
	if err != nil {
		t.Errorf("outputRollbackJSON() error = %v", err)
	}
}

func TestRecordRollbackAudit_NilReleaseRepo(t *testing.T) {
	gitRepo := &rollbackTestGitRepo{
		tags: make(map[string]*sourcecontrol.Tag),
	}

	app := &commandTestApp{
		gitRepo:     gitRepo,
		releaseRepo: nil,
	}

	result := &rollbackResult{
		TargetTag:    "v1.0.0",
		TargetCommit: "abc123",
		RevertTag:    "rollback-to-v1.0.0",
		RolledBackBy: "user",
	}

	// Should not panic with nil release repo
	recordRollbackAudit(context.Background(), app, result)
}

func TestRecordRollbackAudit_WithReleaseRepo(t *testing.T) {
	gitRepo := &rollbackTestGitRepo{
		tags: make(map[string]*sourcecontrol.Tag),
	}

	app := &commandTestApp{
		gitRepo:     gitRepo,
		releaseRepo: testReleaseRepo{err: release.ErrRunNotFound},
	}

	result := &rollbackResult{
		TargetTag:    "v1.0.0",
		TargetCommit: "abc123",
		RevertTag:    "rollback-to-v1.0.0",
		RolledBackBy: "user",
	}

	// Should handle ErrRunNotFound gracefully
	recordRollbackAudit(context.Background(), app, result)
}

func TestRunRollback_NoFlags(t *testing.T) {
	// Save and restore global state
	origVersion := rollbackToVersion
	origTag := rollbackToTag
	origCfg := cfg
	defer func() {
		rollbackToVersion = origVersion
		rollbackToTag = origTag
		cfg = origCfg
	}()

	rollbackToVersion = ""
	rollbackToTag = ""
	cfg = &config.Config{}

	err := runRollback(rollbackCmd, nil)
	if err == nil {
		t.Error("runRollback() should return error when no flags specified")
	}
}
