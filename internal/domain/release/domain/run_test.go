package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestRunID_String(t *testing.T) {
	id := RunID("run-abc123def456")
	if id.String() != "run-abc123def456" {
		t.Errorf("RunID.String() = %v, want %v", id.String(), "run-abc123def456")
	}
}

func TestRunID_Short(t *testing.T) {
	tests := []struct {
		id   RunID
		want string
	}{
		{RunID("run-abc123def456"), "run-abc1"},
		{RunID("short"), "short"},
		{RunID("12345678"), "12345678"},
		{RunID(""), ""},
	}

	for _, tt := range tests {
		got := tt.id.Short()
		if got != tt.want {
			t.Errorf("RunID(%q).Short() = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestCommitSHA_String(t *testing.T) {
	sha := CommitSHA("abc123def456")
	if sha.String() != "abc123def456" {
		t.Errorf("CommitSHA.String() = %v, want %v", sha.String(), "abc123def456")
	}
}

func TestCommitSHA_Short(t *testing.T) {
	tests := []struct {
		sha  CommitSHA
		want string
	}{
		{CommitSHA("abc123def456789"), "abc123d"},
		{CommitSHA("short"), "short"},
		{CommitSHA("1234567"), "1234567"},
		{CommitSHA(""), ""},
	}

	for _, tt := range tests {
		got := tt.sha.Short()
		if got != tt.want {
			t.Errorf("CommitSHA(%q).Short() = %v, want %v", tt.sha, got, tt.want)
		}
	}
}

func TestNewReleaseRun(t *testing.T) {
	run := NewReleaseRun(
		"github.com/example/repo",
		"/path/to/repo",
		"v1.0.0",
		CommitSHA("abc123"),
		[]CommitSHA{"abc123", "def456"},
		"config-hash",
		"plugin-hash",
	)

	// Check initial state
	if run.State() != StateDraft {
		t.Errorf("State() = %v, want %v", run.State(), StateDraft)
	}

	if run.RepoID() != "github.com/example/repo" {
		t.Errorf("RepoID() = %v, want %v", run.RepoID(), "github.com/example/repo")
	}

	if run.RepoRoot() != "/path/to/repo" {
		t.Errorf("RepoRoot() = %v, want %v", run.RepoRoot(), "/path/to/repo")
	}

	if run.HeadSHA() != CommitSHA("abc123") {
		t.Errorf("HeadSHA() = %v, want %v", run.HeadSHA(), CommitSHA("abc123"))
	}

	if len(run.Commits()) != 2 {
		t.Errorf("len(Commits()) = %d, want %d", len(run.Commits()), 2)
	}

	// Check ID was generated
	if run.ID() == "" {
		t.Error("ID() is empty, expected non-empty")
	}

	// Check plan hash was computed
	if run.PlanHash() == "" {
		t.Error("PlanHash() is empty, expected non-empty")
	}

	// Check domain event was emitted
	events := run.DomainEvents()
	if len(events) != 1 {
		t.Errorf("len(DomainEvents()) = %d, want %d", len(events), 1)
	}

	if events[0].EventName() != "run.created" {
		t.Errorf("EventName() = %v, want %v", events[0].EventName(), "run.created")
	}
}

func TestReleaseRun_SetVersionProposal(t *testing.T) {
	run := newTestRun()

	current := version.MustParse("1.0.0")
	next := version.MustParse("1.1.0")

	err := run.SetVersionProposal(current, next, BumpMinor, 0.95)
	if err != nil {
		t.Fatalf("SetVersionProposal() error = %v", err)
	}

	if run.VersionCurrent() != current {
		t.Errorf("VersionCurrent() = %v, want %v", run.VersionCurrent(), current)
	}

	if run.VersionNext() != next {
		t.Errorf("VersionNext() = %v, want %v", run.VersionNext(), next)
	}

	if run.BumpKind() != BumpMinor {
		t.Errorf("BumpKind() = %v, want %v", run.BumpKind(), BumpMinor)
	}
}

func TestReleaseRun_SetVersionProposal_WrongState(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test-actor")

	err := run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), BumpMinor, 0.95)
	if err == nil {
		t.Error("SetVersionProposal() expected error in Planned state")
	}
}

func TestReleaseRun_Plan(t *testing.T) {
	run := newTestRun()

	err := run.Plan("test-actor")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if run.State() != StatePlanned {
		t.Errorf("State() = %v, want %v", run.State(), StatePlanned)
	}

	// Check history was recorded
	if len(run.History()) != 1 {
		t.Errorf("len(History()) = %d, want %d", len(run.History()), 1)
	}
}

func TestReleaseRun_Plan_WrongState(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test-actor")

	err := run.Plan("test-actor")
	if err == nil {
		t.Error("Plan() expected error when already planned")
	}
}

func TestReleaseRun_SetVersion(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test-actor")

	next := version.MustParse("1.1.0")
	err := run.SetVersion(next, "v1.1.0")
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}

	if run.TagName() != "v1.1.0" {
		t.Errorf("TagName() = %v, want %v", run.TagName(), "v1.1.0")
	}
}

func TestReleaseRun_SetVersion_WrongState(t *testing.T) {
	run := newTestRun()

	err := run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	if err == nil {
		t.Error("SetVersion() expected error in Draft state")
	}
}

func TestReleaseRun_Bump(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test-actor")
	_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")

	err := run.Bump("test-actor")
	if err != nil {
		t.Fatalf("Bump() error = %v", err)
	}

	if run.State() != StateVersioned {
		t.Errorf("State() = %v, want %v", run.State(), StateVersioned)
	}
}

func TestReleaseRun_Bump_NoVersion(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test-actor")

	err := run.Bump("test-actor")
	if err == nil {
		t.Error("Bump() expected error without version set")
	}
}

func TestReleaseRun_GenerateNotes(t *testing.T) {
	run := newVersionedRun()

	notes := &ReleaseNotes{
		Text:        "## Release Notes\n- Feature A",
		Provider:    "ai",
		GeneratedAt: time.Now(),
	}

	err := run.GenerateNotes(notes, "input-hash", "test-actor")
	if err != nil {
		t.Fatalf("GenerateNotes() error = %v", err)
	}

	if run.State() != StateNotesReady {
		t.Errorf("State() = %v, want %v", run.State(), StateNotesReady)
	}

	if run.Notes() == nil {
		t.Error("Notes() = nil, expected non-nil")
	}

	if run.Notes().Text != notes.Text {
		t.Errorf("Notes().Text = %v, want %v", run.Notes().Text, notes.Text)
	}
}

func TestReleaseRun_Approve(t *testing.T) {
	run := newNotesReadyRun()

	err := run.Approve("approver", false)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if run.State() != StateApproved {
		t.Errorf("State() = %v, want %v", run.State(), StateApproved)
	}

	if !run.IsApproved() {
		t.Error("IsApproved() = false, want true")
	}

	approval := run.Approval()
	if approval == nil {
		t.Fatal("Approval() = nil, expected non-nil")
	}

	if approval.ApprovedBy != "approver" {
		t.Errorf("Approval().ApprovedBy = %v, want %v", approval.ApprovedBy, "approver")
	}

	if approval.AutoApproved {
		t.Error("Approval().AutoApproved = true, want false")
	}
}

func TestReleaseRun_ApproveWithOptions(t *testing.T) {
	run := newNotesReadyRun()

	err := run.ApproveWithOptions("ci-bot", true, ActorCI, "Automated approval")
	if err != nil {
		t.Fatalf("ApproveWithOptions() error = %v", err)
	}

	approval := run.Approval()
	if approval.AutoApproved != true {
		t.Error("Approval().AutoApproved = false, want true")
	}

	if approval.ApproverType != ActorCI {
		t.Errorf("Approval().ApproverType = %v, want %v", approval.ApproverType, ActorCI)
	}

	if approval.Justification != "Automated approval" {
		t.Errorf("Approval().Justification = %v, want %v", approval.Justification, "Automated approval")
	}
}

func TestReleaseRun_ApprovalStatus(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() *ReleaseRun
		canApprove bool
	}{
		{
			name:       "draft state",
			setup:      newTestRun,
			canApprove: false,
		},
		{
			name: "notes ready state",
			setup: func() *ReleaseRun {
				return newNotesReadyRun()
			},
			canApprove: true,
		},
		{
			name: "already approved",
			setup: func() *ReleaseRun {
				run := newNotesReadyRun()
				_ = run.Approve("test", false)
				return run
			},
			canApprove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := tt.setup()
			status := run.ApprovalStatus()
			if status.CanApprove != tt.canApprove {
				t.Errorf("ApprovalStatus().CanApprove = %v, want %v", status.CanApprove, tt.canApprove)
			}
		})
	}
}

func TestReleaseRun_StartPublishing(t *testing.T) {
	run := newApprovedRun()

	err := run.StartPublishing("test-actor")
	if err != nil {
		t.Fatalf("StartPublishing() error = %v", err)
	}

	if run.State() != StatePublishing {
		t.Errorf("State() = %v, want %v", run.State(), StatePublishing)
	}
}

func TestReleaseRun_StepManagement(t *testing.T) {
	run := newApprovedRun()

	// Set execution plan
	steps := []StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
	}
	run.SetExecutionPlan(steps)

	if len(run.Steps()) != 2 {
		t.Errorf("len(Steps()) = %d, want %d", len(run.Steps()), 2)
	}

	// Start publishing
	_ = run.StartPublishing("test-actor")

	// Mark step started
	err := run.MarkStepStarted("tag")
	if err != nil {
		t.Fatalf("MarkStepStarted() error = %v", err)
	}

	status := run.StepStatus("tag")
	if status.State != StepRunning {
		t.Errorf("StepStatus().State = %v, want %v", status.State, StepRunning)
	}

	if status.Attempts != 1 {
		t.Errorf("StepStatus().Attempts = %d, want %d", status.Attempts, 1)
	}

	// Mark step done
	err = run.MarkStepDone("tag", "tag created")
	if err != nil {
		t.Fatalf("MarkStepDone() error = %v", err)
	}

	status = run.StepStatus("tag")
	if status.State != StepDone {
		t.Errorf("StepStatus().State = %v, want %v", status.State, StepDone)
	}

	if status.Output != "tag created" {
		t.Errorf("StepStatus().Output = %v, want %v", status.Output, "tag created")
	}
}

func TestReleaseRun_StepNotFound(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")

	err := run.MarkStepStarted("nonexistent")
	if err == nil {
		t.Error("MarkStepStarted() expected error for nonexistent step")
	}
}

func TestReleaseRun_MarkStepFailed(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepStarted("tag")

	err := run.MarkStepFailed("tag", ErrStepNotFound)
	if err != nil {
		t.Fatalf("MarkStepFailed() error = %v", err)
	}

	status := run.StepStatus("tag")
	if status.State != StepFailed {
		t.Errorf("StepStatus().State = %v, want %v", status.State, StepFailed)
	}

	if run.LastError() == "" {
		t.Error("LastError() is empty after step failure")
	}
}

func TestReleaseRun_MarkStepSkipped(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")

	err := run.MarkStepSkipped("tag", "already exists")
	if err != nil {
		t.Fatalf("MarkStepSkipped() error = %v", err)
	}

	status := run.StepStatus("tag")
	if status.State != StepSkipped {
		t.Errorf("StepStatus().State = %v, want %v", status.State, StepSkipped)
	}
}

func TestReleaseRun_AllStepsDone(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
	})
	_ = run.StartPublishing("test-actor")

	if run.AllStepsDone() {
		t.Error("AllStepsDone() = true before any steps done")
	}

	_ = run.MarkStepDone("tag", "done")
	_ = run.MarkStepSkipped("notify", "skipped")

	if !run.AllStepsDone() {
		t.Error("AllStepsDone() = false after all steps done/skipped")
	}
}

func TestReleaseRun_AllStepsSucceeded(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
	})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepStarted("tag")
	_ = run.MarkStepFailed("tag", ErrStepNotFound)

	if run.AllStepsSucceeded() {
		t.Error("AllStepsSucceeded() = true with failed step")
	}
}

func TestReleaseRun_NextPendingStep(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
	})
	_ = run.StartPublishing("test-actor")

	next := run.NextPendingStep()
	if next == nil || next.Name != "tag" {
		t.Errorf("NextPendingStep() = %v, want tag step", next)
	}

	_ = run.MarkStepDone("tag", "done")

	next = run.NextPendingStep()
	if next == nil || next.Name != "notify" {
		t.Errorf("NextPendingStep() = %v, want notify step", next)
	}

	_ = run.MarkStepDone("notify", "done")

	next = run.NextPendingStep()
	if next != nil {
		t.Errorf("NextPendingStep() = %v, want nil", next)
	}
}

func TestReleaseRun_MarkPublished(t *testing.T) {
	run := newPublishingRun()

	err := run.MarkPublished("test-actor")
	if err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	if run.State() != StatePublished {
		t.Errorf("State() = %v, want %v", run.State(), StatePublished)
	}

	if run.PublishedAt() == nil {
		t.Error("PublishedAt() = nil, expected non-nil")
	}
}

func TestReleaseRun_MarkFailed(t *testing.T) {
	run := newApprovedRun()
	_ = run.StartPublishing("test-actor")

	err := run.MarkFailed("something went wrong", "test-actor")
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	if run.State() != StateFailed {
		t.Errorf("State() = %v, want %v", run.State(), StateFailed)
	}

	if run.LastError() != "something went wrong" {
		t.Errorf("LastError() = %v, want %v", run.LastError(), "something went wrong")
	}
}

func TestReleaseRun_Cancel(t *testing.T) {
	run := newApprovedRun()

	err := run.Cancel("user requested", "test-actor")
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	if run.State() != StateCanceled {
		t.Errorf("State() = %v, want %v", run.State(), StateCanceled)
	}
}

func TestReleaseRun_CancelAlreadyPublished(t *testing.T) {
	run := newPublishingRun()
	_ = run.MarkPublished("test-actor")

	err := run.Cancel("too late", "test-actor")
	if err != ErrAlreadyPublished {
		t.Errorf("Cancel() error = %v, want %v", err, ErrAlreadyPublished)
	}
}

func TestReleaseRun_RetryPublish(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepStarted("tag")
	_ = run.MarkStepFailed("tag", ErrStepNotFound)
	_ = run.MarkFailed("step failed", "test-actor")

	err := run.RetryPublish("test-actor")
	if err != nil {
		t.Fatalf("RetryPublish() error = %v", err)
	}

	if run.State() != StatePublishing {
		t.Errorf("State() = %v, want %v", run.State(), StatePublishing)
	}

	// Failed step should be reset to pending
	status := run.StepStatus("tag")
	if status.State != StepPending {
		t.Errorf("StepStatus().State = %v, want %v", status.State, StepPending)
	}

	// Attempts should be preserved
	if status.Attempts != 1 {
		t.Errorf("StepStatus().Attempts = %d, want %d", status.Attempts, 1)
	}
}

func TestReleaseRun_ValidateHeadMatch(t *testing.T) {
	run := newTestRun()

	err := run.ValidateHeadMatch(run.HeadSHA())
	if err != nil {
		t.Errorf("ValidateHeadMatch() error = %v, want nil", err)
	}

	err = run.ValidateHeadMatch("different-sha")
	if err == nil {
		t.Error("ValidateHeadMatch() expected error for mismatched SHA")
	}
}

func TestReleaseRun_PolicyChecks(t *testing.T) {
	run := newTestRun()
	run.SetPolicyEvaluation(0.3, []string{"reason"}, PolicyThresholds{
		AutoApproveRiskThreshold: 0.5,
		RequireApprovalAbove:     0.3,
		BlockReleaseAbove:        0.9,
	})

	if !run.CanAutoApprove() {
		t.Error("CanAutoApprove() = false, want true (risk 0.3 < threshold 0.5)")
	}

	if run.IsBlocked() {
		t.Error("IsBlocked() = true, want false (risk 0.3 < block 0.9)")
	}

	// Test with high risk
	run.SetPolicyEvaluation(0.95, []string{"high risk"}, PolicyThresholds{
		AutoApproveRiskThreshold: 0.5,
		RequireApprovalAbove:     0.3,
		BlockReleaseAbove:        0.9,
	})

	if run.CanAutoApprove() {
		t.Error("CanAutoApprove() = true, want false (risk 0.95 > threshold 0.5)")
	}

	if !run.IsBlocked() {
		t.Error("IsBlocked() = false, want true (risk 0.95 >= block 0.9)")
	}

	if !run.RequiresApproval() {
		t.Error("RequiresApproval() = false, want true")
	}
}

func TestReleaseRun_Summary(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
	})

	summary := run.Summary()

	if summary.ID != run.ID() {
		t.Errorf("Summary().ID = %v, want %v", summary.ID, run.ID())
	}

	if summary.State != run.State() {
		t.Errorf("Summary().State = %v, want %v", summary.State, run.State())
	}

	if summary.StepsTotal != 2 {
		t.Errorf("Summary().StepsTotal = %d, want %d", summary.StepsTotal, 2)
	}
}

func TestReleaseRun_ValidateInvariants(t *testing.T) {
	run := newTestRun()
	invariants := run.ValidateInvariants()

	// All invariants should be valid for a new run
	for _, inv := range invariants {
		if !inv.Valid {
			t.Errorf("Invariant %s failed: %s", inv.Name, inv.Message)
		}
	}

	if !run.IsValid() {
		t.Error("IsValid() = false for new run")
	}
}

func TestReleaseRun_DomainEvents(t *testing.T) {
	run := newTestRun()
	initialEvents := len(run.DomainEvents())

	_ = run.Plan("test-actor")
	_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = run.Bump("test-actor")

	events := run.DomainEvents()
	if len(events) <= initialEvents {
		t.Error("DomainEvents() should have more events after transitions")
	}

	run.ClearDomainEvents()
	if len(run.DomainEvents()) != 0 {
		t.Errorf("ClearDomainEvents() should clear all events, got %d", len(run.DomainEvents()))
	}
}

func TestReleaseRun_UpdateNotes(t *testing.T) {
	run := newNotesReadyRun()

	newNotes := &ReleaseNotes{
		Text:        "Updated notes",
		Provider:    "manual",
		GeneratedAt: time.Now(),
	}

	err := run.UpdateNotes(newNotes, "editor")
	if err != nil {
		t.Fatalf("UpdateNotes() error = %v", err)
	}

	if run.Notes().Text != "Updated notes" {
		t.Errorf("Notes().Text = %v, want %v", run.Notes().Text, "Updated notes")
	}
}

func TestReleaseRun_UpdateNotes_NilNotes(t *testing.T) {
	run := newNotesReadyRun()

	err := run.UpdateNotes(nil, "editor")
	if err != ErrNilNotes {
		t.Errorf("UpdateNotes(nil) error = %v, want %v", err, ErrNilNotes)
	}
}

func TestReleaseRun_UpdateNotesText(t *testing.T) {
	run := newNotesReadyRun()

	err := run.UpdateNotesText("Simple text update")
	if err != nil {
		t.Fatalf("UpdateNotesText() error = %v", err)
	}

	if run.Notes().Text != "Simple text update" {
		t.Errorf("Notes().Text = %v, want %v", run.Notes().Text, "Simple text update")
	}
}

func TestReleaseRun_RecordPluginExecution(t *testing.T) {
	run := newApprovedRun()
	initialEvents := len(run.DomainEvents())

	run.RecordPluginExecution("github", "PostPublish", true, "success", 2*time.Second)

	events := run.DomainEvents()
	if len(events) <= initialEvents {
		t.Error("RecordPluginExecution() should emit event")
	}
}

func TestParseBumpKind(t *testing.T) {
	tests := []struct {
		input   string
		want    BumpKind
		wantErr bool
	}{
		{"major", BumpMajor, false},
		{"MAJOR", BumpMajor, false},
		{"minor", BumpMinor, false},
		{"patch", BumpPatch, false},
		{"prerelease", BumpPrerelease, false},
		{"none", BumpNone, false},
		{"", BumpNone, false},
		{"invalid", BumpNone, true},
	}

	for _, tt := range tests {
		got, err := ParseBumpKind(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseBumpKind(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("ParseBumpKind(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBumpKind_ToBumpType(t *testing.T) {
	tests := []struct {
		kind BumpKind
		want version.BumpType
	}{
		{BumpMajor, version.BumpMajor},
		{BumpMinor, version.BumpMinor},
		{BumpPatch, version.BumpPatch},
		{BumpNone, version.BumpPatch}, // Default
	}

	for _, tt := range tests {
		got := tt.kind.ToBumpType()
		if got != tt.want {
			t.Errorf("BumpKind(%v).ToBumpType() = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestBumpKindFromReleaseType(t *testing.T) {
	tests := []struct {
		rt   changes.ReleaseType
		want BumpKind
	}{
		{changes.ReleaseTypeMajor, BumpMajor},
		{changes.ReleaseTypeMinor, BumpMinor},
		{changes.ReleaseTypePatch, BumpPatch},
	}

	for _, tt := range tests {
		got := BumpKindFromReleaseType(tt.rt)
		if got != tt.want {
			t.Errorf("BumpKindFromReleaseType(%v) = %v, want %v", tt.rt, got, tt.want)
		}
	}
}

func TestBuildIdempotencyKey(t *testing.T) {
	key1 := BuildIdempotencyKey("run-123", "tag", "config-abc")
	key2 := BuildIdempotencyKey("run-123", "tag", "config-abc")
	key3 := BuildIdempotencyKey("run-456", "tag", "config-abc")

	if key1 != key2 {
		t.Error("BuildIdempotencyKey() should return same key for same inputs")
	}

	if key1 == key3 {
		t.Error("BuildIdempotencyKey() should return different keys for different inputs")
	}

	if len(key1) != 16 {
		t.Errorf("BuildIdempotencyKey() length = %d, want 16", len(key1))
	}
}

func TestApproval_IsManual(t *testing.T) {
	manual := &Approval{AutoApproved: false}
	if !manual.IsManual() {
		t.Error("IsManual() = false for manual approval")
	}

	auto := &Approval{AutoApproved: true}
	if auto.IsManual() {
		t.Error("IsManual() = true for auto approval")
	}
}

// Helper functions to create runs in various states

func newTestRun() *ReleaseRun {
	return NewReleaseRun(
		"github.com/test/repo",
		"/path/to/repo",
		"v1.0.0",
		CommitSHA("abc123"),
		[]CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)
}

func newVersionedRun() *ReleaseRun {
	run := newTestRun()
	_ = run.Plan("test-actor")
	_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = run.Bump("test-actor")
	return run
}

func newNotesReadyRun() *ReleaseRun {
	run := newVersionedRun()
	notes := &ReleaseNotes{
		Text:        "## Release Notes",
		Provider:    "test",
		GeneratedAt: time.Now(),
	}
	_ = run.GenerateNotes(notes, "input-hash", "test-actor")
	return run
}

func newApprovedRun() *ReleaseRun {
	run := newNotesReadyRun()
	_ = run.Approve("approver", false)
	return run
}

func newPublishingRun() *ReleaseRun {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
	})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepDone("tag", "done")
	return run
}

func TestReleaseRun_ValidateApprovalPlanHash(t *testing.T) {
	t.Run("valid approval matches plan hash", func(t *testing.T) {
		run := newApprovedRun()
		err := run.ValidateApprovalPlanHash()
		if err != nil {
			t.Errorf("ValidateApprovalPlanHash() error = %v, want nil", err)
		}
	})

	t.Run("not approved returns error", func(t *testing.T) {
		run := newNotesReadyRun()
		err := run.ValidateApprovalPlanHash()
		if err == nil {
			t.Error("ValidateApprovalPlanHash() expected error for unapproved run")
		}
		if !errors.Is(err, ErrNotApproved) {
			t.Errorf("ValidateApprovalPlanHash() error = %v, want ErrNotApproved", err)
		}
	})

	t.Run("mismatched plan hash returns error", func(t *testing.T) {
		run := newApprovedRun()

		// Get the original approval with its plan hash
		originalApproval := run.Approval()

		// Manually corrupt the plan hash to simulate tampering
		run.ReconstructState(RunSnapshot{
			ID:              run.ID(),
			PlanHash:        "tampered-plan-hash", // Different plan hash than what approval has
			RepoID:          run.RepoID(),
			RepoRoot:        run.RepoRoot(),
			BaseRef:         run.Branch(),
			HeadSHA:         run.HeadSHA(),
			Commits:         nil,
			ConfigHash:      "",
			PluginPlanHash:  "",
			VersionCurrent:  run.VersionCurrent(),
			VersionNext:     run.VersionNext(),
			BumpKind:        run.BumpKind(),
			Confidence:      0.95,
			RiskScore:       run.RiskScore(),
			Reasons:         nil,
			ActorType:       run.ActorType(),
			ActorID:         run.ActorID(),
			Thresholds:      PolicyThresholds{},
			TagName:         run.TagName(),
			Notes:           run.Notes(),
			NotesInputsHash: "",
			Approval:        originalApproval, // Keep original approval with old plan hash
			Steps:           nil,
			StepStatus:      nil,
			State:           StateApproved,
			History:         nil,
			LastError:       "",
			ChangesetID:     "",
			CreatedAt:       run.CreatedAt(),
			UpdatedAt:       run.UpdatedAt(),
			PublishedAt:     nil,
		})

		err := run.ValidateApprovalPlanHash()
		if err == nil {
			t.Error("ValidateApprovalPlanHash() expected error for mismatched hash")
		}
		if !errors.Is(err, ErrApprovalBoundToHash) {
			t.Errorf("ValidateApprovalPlanHash() error = %v, want ErrApprovalBoundToHash", err)
		}
	})
}

func TestReleaseRun_MultiLevelApproval(t *testing.T) {
	t.Run("default policy single approval", func(t *testing.T) {
		run := newNotesReadyRun()

		// Set default policy
		run.SetApprovalPolicy(DefaultApprovalPolicy())

		// Grant release approval
		err := run.ApproveAtLevel(ApprovalLevelRelease, "release-manager", ActorHuman, "Ready to ship")
		if err != nil {
			t.Fatalf("ApproveAtLevel failed: %v", err)
		}

		// Complete approval
		err = run.CompleteMultiLevelApproval("release-manager")
		if err != nil {
			t.Fatalf("CompleteMultiLevelApproval failed: %v", err)
		}

		if run.State() != StateApproved {
			t.Errorf("State = %s, want %s", run.State(), StateApproved)
		}
	})

	t.Run("high risk policy sequential approvals", func(t *testing.T) {
		run := newNotesReadyRun()

		// Set high-risk policy requiring sequential approvals
		run.SetApprovalPolicy(HighRiskApprovalPolicy())

		// Should have 3 pending approvals initially
		pending := run.PendingApprovalLevels()
		if len(pending) != 3 {
			t.Errorf("PendingApprovalLevels = %d, want 3", len(pending))
		}

		// Technical approval first
		err := run.ApproveAtLevel(ApprovalLevelTechnical, "tech-lead", ActorHuman, "Code reviewed")
		if err != nil {
			t.Fatalf("ApproveAtLevel(technical) failed: %v", err)
		}

		// Now 2 pending
		pending = run.PendingApprovalLevels()
		if len(pending) != 2 {
			t.Errorf("PendingApprovalLevels = %d, want 2", len(pending))
		}

		// Security approval
		err = run.ApproveAtLevel(ApprovalLevelSecurity, "security-team", ActorHuman, "Security review passed")
		if err != nil {
			t.Fatalf("ApproveAtLevel(security) failed: %v", err)
		}

		// Final release approval
		err = run.ApproveAtLevel(ApprovalLevelRelease, "release-manager", ActorHuman, "Approved for release")
		if err != nil {
			t.Fatalf("ApproveAtLevel(release) failed: %v", err)
		}

		// Complete approval
		err = run.CompleteMultiLevelApproval("system")
		if err != nil {
			t.Fatalf("CompleteMultiLevelApproval failed: %v", err)
		}

		if run.State() != StateApproved {
			t.Errorf("State = %s, want %s", run.State(), StateApproved)
		}
	})

	t.Run("incomplete approvals prevented", func(t *testing.T) {
		run := newNotesReadyRun()

		// Set high-risk policy
		run.SetApprovalPolicy(HighRiskApprovalPolicy())

		// Only grant technical approval
		err := run.ApproveAtLevel(ApprovalLevelTechnical, "tech-lead", ActorHuman, "Code reviewed")
		if err != nil {
			t.Fatalf("ApproveAtLevel(technical) failed: %v", err)
		}

		// Try to complete - should fail
		err = run.CompleteMultiLevelApproval("system")
		if err == nil {
			t.Error("CompleteMultiLevelApproval should fail with incomplete approvals")
		}

		// State should still be NotesReady
		if run.State() != StateNotesReady {
			t.Errorf("State = %s, want %s", run.State(), StateNotesReady)
		}
	})

	t.Run("sequential policy enforces order", func(t *testing.T) {
		run := newNotesReadyRun()

		// Set high-risk policy (sequential)
		run.SetApprovalPolicy(HighRiskApprovalPolicy())

		// Try to skip to security approval - should fail
		err := run.ApproveAtLevel(ApprovalLevelSecurity, "security-team", ActorHuman, "Security review")
		if err == nil {
			t.Error("ApproveAtLevel should fail when skipping required levels in sequential policy")
		}
	})
}

func TestMultiLevelApproval_PolicyHelpers(t *testing.T) {
	t.Run("default policy has release level", func(t *testing.T) {
		policy := DefaultApprovalPolicy()

		if len(policy.Requirements) != 1 {
			t.Errorf("DefaultApprovalPolicy requirements = %d, want 1", len(policy.Requirements))
		}
		if policy.Requirements[0].Level != ApprovalLevelRelease {
			t.Errorf("DefaultApprovalPolicy level = %s, want %s", policy.Requirements[0].Level, ApprovalLevelRelease)
		}
	})

	t.Run("high risk policy has three levels", func(t *testing.T) {
		policy := HighRiskApprovalPolicy()

		if len(policy.Requirements) != 3 {
			t.Errorf("HighRiskApprovalPolicy requirements = %d, want 3", len(policy.Requirements))
		}
		if !policy.Sequential {
			t.Error("HighRiskApprovalPolicy should be sequential")
		}
	})
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestReleaseRun_BaseRef(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")
	baseRef := run.BaseRef()
	if baseRef != "v1.0.0" {
		t.Errorf("BaseRef() = %s, want v1.0.0", baseRef)
	}
}

func TestReleaseRun_Reasons(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// Initially empty
	reasons := run.Reasons()
	if reasons != nil {
		t.Errorf("Reasons() = %v, want nil", reasons)
	}
}

func TestReleaseRun_ChangeSet(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// Initially no changeset
	if run.HasChangeSet() {
		t.Error("HasChangeSet() should be false initially")
	}
	if run.ChangeSet() != nil {
		t.Error("ChangeSet() should be nil initially")
	}
	if run.ChangesetID() != "" {
		t.Errorf("ChangesetID() = %s, want empty", run.ChangesetID())
	}

	// Set changeset
	cs := changes.NewChangeSet("cs-123", "v1.0.0", "HEAD")
	run.SetChangeSet(cs)

	if !run.HasChangeSet() {
		t.Error("HasChangeSet() should be true after SetChangeSet")
	}
	if run.ChangeSet() != cs {
		t.Error("ChangeSet() should return the set changeset")
	}
	if run.ChangesetID() != string(cs.ID()) {
		t.Errorf("ChangesetID() = %s, want %s", run.ChangesetID(), cs.ID())
	}

	// Clear changeset
	run.SetChangeSet(nil)
	if run.HasChangeSet() {
		t.Error("HasChangeSet() should be false after clearing")
	}
}

func TestReleaseRun_EmitCreatedEvent(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// Clear any existing events
	run.ClearDomainEvents()

	// Emit created event
	run.EmitCreatedEvent()

	events := run.DomainEvents()
	if len(events) == 0 {
		t.Fatal("EmitCreatedEvent should add an event")
	}

	if events[0].EventName() != "run.created" {
		t.Errorf("Event name = %s, want run.created", events[0].EventName())
	}
}

func TestReleaseRun_SetActor(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	run.SetActor(ActorHuman, "user@example.com")

	if run.ActorType() != ActorHuman {
		t.Errorf("ActorType() = %s, want %s", run.ActorType(), ActorHuman)
	}
	if run.ActorID() != "user@example.com" {
		t.Errorf("ActorID() = %s, want user@example.com", run.ActorID())
	}
}

func TestReleaseRun_SetChangesetID(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	run.SetChangesetID("changeset-456")

	if run.ChangesetID() != "changeset-456" {
		t.Errorf("ChangesetID() = %s, want changeset-456", run.ChangesetID())
	}
}

func TestReleaseRun_CanApprove(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// In draft state, cannot approve
	if run.CanApprove() {
		t.Error("CanApprove() should be false in draft state")
	}

	// Plan the run
	_ = run.Plan("test-actor")
	if run.CanApprove() {
		t.Error("CanApprove() should be false in planned state")
	}
}

func TestReleaseRun_CanProceedToPublish(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// In draft state, cannot proceed
	if run.CanProceedToPublish() {
		t.Error("CanProceedToPublish() should be false in draft state")
	}
}

func TestReleaseRun_MultiLevelApprovalStatus(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// Initially nil
	status := run.MultiLevelApprovalStatus()
	if status != nil {
		t.Error("MultiLevelApprovalStatus() should be nil initially")
	}
}

func TestReleaseRun_IsMultiLevelApprovalEnabled(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// Initially disabled
	if run.IsMultiLevelApprovalEnabled() {
		t.Error("IsMultiLevelApprovalEnabled() should be false initially")
	}
}

func TestReleaseRun_InvariantViolations(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	violations := run.InvariantViolations()
	if violations == nil {
		t.Fatal("InvariantViolations should not return nil")
	}

	// A valid new run should have no violations
	if len(violations) != 0 {
		t.Errorf("InvariantViolations() = %v, want empty", violations)
	}
}

func TestMultiLevelApproval_AllApprovals(t *testing.T) {
	policy := DefaultApprovalPolicy()
	mla := NewMultiLevelApproval(policy)

	// Initially empty
	approvals := mla.AllApprovals()
	if len(approvals) != 0 {
		t.Errorf("AllApprovals() = %d, want 0", len(approvals))
	}

	// Grant an approval
	approval := &Approval{
		ApprovedBy:    "approver",
		ApprovedAt:    time.Now(),
		ApproverType:  ActorHuman,
		Justification: "Approved",
	}
	err := mla.Grant(ApprovalLevelRelease, approval)
	if err != nil {
		t.Fatalf("Grant failed: %v", err)
	}

	approvals = mla.AllApprovals()
	if len(approvals) != 1 {
		t.Errorf("AllApprovals() = %d, want 1", len(approvals))
	}
}

func TestReleaseRun_ApprovalStatus_PublishingState(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")

	status := run.ApprovalStatus()
	if status.CanApprove {
		t.Error("CanApprove should be false in publishing state")
	}
}

func TestReleaseRun_ApprovalStatus_FailedState(t *testing.T) {
	run := newApprovedRun()
	_ = run.StartPublishing("test-actor")
	_ = run.MarkFailed("error", "test-actor")

	status := run.ApprovalStatus()
	if status.CanApprove {
		t.Error("CanApprove should be false in failed state")
	}
}

func TestReleaseRun_MarkPublished_NotAllStepsSucceeded(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
	})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepStarted("tag")
	_ = run.MarkStepFailed("tag", ErrStepNotFound)

	err := run.MarkPublished("test-actor")
	if err == nil {
		t.Error("MarkPublished() should fail when not all steps succeeded")
	}
}

func TestReleaseRun_MarkPublished_WrongState(t *testing.T) {
	run := newApprovedRun()
	err := run.MarkPublished("test-actor")
	if err == nil {
		t.Error("MarkPublished() should fail in approved state")
	}
}

func TestReleaseRun_StartPublishing_WrongState(t *testing.T) {
	run := newNotesReadyRun()
	err := run.StartPublishing("test-actor")
	if err == nil {
		t.Error("StartPublishing() should fail in notes_ready state")
	}
}

func TestReleaseRun_GenerateNotes_WrongState(t *testing.T) {
	run := newTestRun()
	notes := &ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
	err := run.GenerateNotes(notes, "hash", "actor")
	if err == nil {
		t.Error("GenerateNotes() should fail in draft state")
	}
}

func TestReleaseRun_Approve_WrongState(t *testing.T) {
	run := newVersionedRun()
	err := run.Approve("actor", false)
	if err == nil {
		t.Error("Approve() should fail in versioned state")
	}
}

func TestReleaseRun_ApproveWithOptions_WrongState(t *testing.T) {
	run := newVersionedRun()
	err := run.ApproveWithOptions("actor", false, ActorHuman, "")
	if err == nil {
		t.Error("ApproveWithOptions() should fail in versioned state")
	}
}

func TestReleaseRun_RetryPublish_WrongState(t *testing.T) {
	run := newApprovedRun()
	err := run.RetryPublish("actor")
	if err == nil {
		t.Error("RetryPublish() should fail in approved state")
	}
}

func TestReleaseRun_TransitionTo_InvalidTransition(t *testing.T) {
	run := newTestRun()
	err := run.TransitionTo(StatePublished, "FORCE", "actor", "reason", nil)
	if err == nil {
		t.Error("TransitionTo() should fail for invalid transition from draft to published")
	}
}

func TestBumpKindFromReleaseType_None(t *testing.T) {
	got := BumpKindFromReleaseType(changes.ReleaseTypeNone)
	if got != BumpPatch {
		t.Errorf("BumpKindFromReleaseType(None) = %v, want BumpPatch", got)
	}
}

func TestReleaseRun_Summary_WithStepCounts(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{
		{Name: "tag", Type: StepTypeTag},
		{Name: "notify", Type: StepTypeNotify},
		{Name: "publish", Type: StepTypeFinalize},
	})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepDone("tag", "done")
	_ = run.MarkStepStarted("notify")
	_ = run.MarkStepFailed("notify", ErrStepNotFound)

	summary := run.Summary()
	if summary.StepsDone != 1 {
		t.Errorf("Summary().StepsDone = %d, want 1", summary.StepsDone)
	}
	if summary.StepsFailed != 1 {
		t.Errorf("Summary().StepsFailed = %d, want 1", summary.StepsFailed)
	}
	if summary.StepsTotal != 3 {
		t.Errorf("Summary().StepsTotal = %d, want 3", summary.StepsTotal)
	}
}

func TestReleaseRun_PendingApprovalLevels_NilMultiLevel(t *testing.T) {
	run := newTestRun()
	pending := run.PendingApprovalLevels()
	if pending != nil {
		t.Errorf("PendingApprovalLevels() = %v, want nil", pending)
	}
}

func TestReleaseRun_Approval_Nil(t *testing.T) {
	run := newTestRun()
	if run.Approval() != nil {
		t.Error("Approval() should be nil for unapproved run")
	}
}

func TestReleaseRun_CompleteMultiLevelApproval_NoPolicy(t *testing.T) {
	run := newNotesReadyRun()
	err := run.CompleteMultiLevelApproval("actor")
	if err == nil {
		t.Error("CompleteMultiLevelApproval should fail with no policy")
	}
}

func TestReleaseRun_CompleteMultiLevelApproval_WithoutReleaseLevel(t *testing.T) {
	run := newNotesReadyRun()

	// Custom policy with only technical level (no release level)
	policy := ApprovalPolicy{
		Requirements: []ApprovalRequirement{
			{Level: ApprovalLevelTechnical, Description: "Tech review", Required: true},
		},
		Sequential: false,
	}
	run.SetApprovalPolicy(policy)

	err := run.ApproveAtLevel(ApprovalLevelTechnical, "tech-lead", ActorHuman, "OK")
	if err != nil {
		t.Fatalf("ApproveAtLevel failed: %v", err)
	}

	err = run.CompleteMultiLevelApproval("system")
	if err != nil {
		t.Fatalf("CompleteMultiLevelApproval failed: %v", err)
	}

	if run.State() != StateApproved {
		t.Errorf("State = %s, want %s", run.State(), StateApproved)
	}
}

func TestReleaseRun_ApproveAtLevel_WrongState(t *testing.T) {
	run := newVersionedRun()
	err := run.ApproveAtLevel(ApprovalLevelRelease, "actor", ActorHuman, "")
	if err == nil {
		t.Error("ApproveAtLevel should fail in versioned state")
	}
}

func TestReleaseRun_ApproveAtLevel_InitializesDefaultPolicy(t *testing.T) {
	run := newNotesReadyRun()
	// Do not set policy explicitly - it should auto-initialize
	err := run.ApproveAtLevel(ApprovalLevelRelease, "actor", ActorHuman, "OK")
	if err != nil {
		t.Fatalf("ApproveAtLevel failed: %v", err)
	}
	if !run.IsMultiLevelApprovalEnabled() {
		t.Error("Multi-level approval should be enabled after ApproveAtLevel")
	}
}

func TestReleaseRun_MarkStepStarted_AlreadyDone(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")
	_ = run.MarkStepDone("tag", "done")

	err := run.MarkStepStarted("tag")
	if err == nil {
		t.Error("MarkStepStarted should fail for already-done step")
	}
}

func TestReleaseRun_MarkStepDone_NotFound(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")

	err := run.MarkStepDone("nonexistent", "output")
	if err == nil {
		t.Error("MarkStepDone should fail for nonexistent step")
	}
}

func TestReleaseRun_MarkStepFailed_NotFound(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")

	err := run.MarkStepFailed("nonexistent", ErrStepNotFound)
	if err == nil {
		t.Error("MarkStepFailed should fail for nonexistent step")
	}
}

func TestReleaseRun_MarkStepSkipped_NotFound(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
	_ = run.StartPublishing("test-actor")

	err := run.MarkStepSkipped("nonexistent", "reason")
	if err == nil {
		t.Error("MarkStepSkipped should fail for nonexistent step")
	}
}

func TestRunState_NextValidStates_InvalidState(t *testing.T) {
	invalid := RunState("bogus")
	next := invalid.NextValidStates()
	if next != nil {
		t.Errorf("NextValidStates() for invalid state = %v, want nil", next)
	}
}

func TestRunState_CanTransitionTo_InvalidState(t *testing.T) {
	invalid := RunState("bogus")
	if invalid.CanTransitionTo(StateDraft) {
		t.Error("CanTransitionTo should be false for invalid source state")
	}
}

func TestRunState_Description_Unknown(t *testing.T) {
	unknown := RunState("unknown")
	desc := unknown.Description()
	if desc == "" {
		t.Error("Description() for unknown state should return non-empty string")
	}
}

func TestReleaseRun_Bump_WrongState(t *testing.T) {
	run := newTestRun()
	err := run.Bump("actor")
	if err == nil {
		t.Error("Bump() should fail in draft state")
	}
}

func TestReleaseRun_UpdateNotes_WrongState(t *testing.T) {
	run := newVersionedRun()
	notes := &ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
	err := run.UpdateNotes(notes, "editor")
	if err == nil {
		t.Error("UpdateNotes() should fail in versioned state")
	}
}

func TestReleaseRun_IsValid_WithViolation(t *testing.T) {
	// Create a run in an invalid state by reconstructing with mismatched data
	run := newTestRun()
	// Reconstruct with approved state but nil approval - this violates an invariant
	run.ReconstructState(RunSnapshot{
		ID:         run.ID(),
		PlanHash:   run.PlanHash(),
		RepoID:     run.RepoID(),
		RepoRoot:   run.RepoRoot(),
		BaseRef:    "v1.0.0",
		HeadSHA:    run.HeadSHA(),
		Commits:    []CommitSHA{"abc123"},
		State:      StateApproved,
		StepStatus: make(map[string]*StepStatus),
		CreatedAt:  run.CreatedAt(),
		UpdatedAt:  run.UpdatedAt(),
	})

	if run.IsValid() {
		t.Error("IsValid() should be false when invariants are violated")
	}

	violations := run.InvariantViolations()
	if len(violations) == 0 {
		t.Error("InvariantViolations() should return violations")
	}
}

func TestMultiLevelApproval_NilApprovalsMap(t *testing.T) {
	mla := &MultiLevelApproval{
		Policy: DefaultApprovalPolicy(),
	}

	// IsLevelApproved with nil map
	if mla.IsLevelApproved(ApprovalLevelRelease) {
		t.Error("IsLevelApproved should be false with nil map")
	}

	// GetApproval with nil map
	if mla.GetApproval(ApprovalLevelRelease) != nil {
		t.Error("GetApproval should be nil with nil map")
	}

	// Grant should initialize the map
	err := mla.Grant(ApprovalLevelRelease, &Approval{ApprovedBy: "test"})
	if err != nil {
		t.Fatalf("Grant failed: %v", err)
	}
	if !mla.IsLevelApproved(ApprovalLevelRelease) {
		t.Error("IsLevelApproved should be true after Grant")
	}
}

func TestMultiLevelApproval_NextRequiredLevel_AllApproved(t *testing.T) {
	policy := DefaultApprovalPolicy()
	mla := NewMultiLevelApproval(policy)
	_ = mla.Grant(ApprovalLevelRelease, &Approval{ApprovedBy: "test"})

	next := mla.NextRequiredLevel()
	if next != nil {
		t.Errorf("NextRequiredLevel() = %v, want nil when all approved", next)
	}
}

func TestReleaseRun_RecordTagPushMode(t *testing.T) {
	run := NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []CommitSHA{"abc123"}, "cfg", "plug")

	// Clear any existing events
	run.ClearDomainEvents()

	// Record tag push mode
	run.RecordTagPushMode("v1.1.0", "ci-bot")

	events := run.DomainEvents()
	if len(events) == 0 {
		t.Fatal("RecordTagPushMode should add an event")
	}

	if events[0].EventName() != "run.tag_push_mode_detected" {
		t.Errorf("Event name = %s, want run.tag_push_mode_detected", events[0].EventName())
	}

	// Verify it's the correct event type with correct tag name
	tagEvent, ok := events[0].(*TagPushModeDetectedEvent)
	if !ok {
		t.Fatal("Event is not TagPushModeDetectedEvent")
	}
	if tagEvent.TagName != "v1.1.0" {
		t.Errorf("TagName = %s, want v1.1.0", tagEvent.TagName)
	}
	if tagEvent.Actor != "ci-bot" {
		t.Errorf("Actor = %s, want ci-bot", tagEvent.Actor)
	}
}
