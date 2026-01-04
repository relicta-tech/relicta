// Package container provides dependency injection for Relicta services.
package container

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// mockTagCreator implements ports.TagCreator for testing.
type mockTagCreator struct {
	createTagErr   error
	pushTagErr     error
	tagExistsValue bool
	tagExistsErr   error
	createTagCalls []struct {
		name    string
		message string
	}
	pushTagCalls []struct {
		name   string
		remote string
	}
}

func (m *mockTagCreator) CreateTag(_ context.Context, name, message string) error {
	m.createTagCalls = append(m.createTagCalls, struct {
		name    string
		message string
	}{name, message})
	return m.createTagErr
}

func (m *mockTagCreator) PushTag(_ context.Context, name, remote string) error {
	m.pushTagCalls = append(m.pushTagCalls, struct {
		name   string
		remote string
	}{name, remote})
	return m.pushTagErr
}

func (m *mockTagCreator) TagExists(_ context.Context, _ string) (bool, error) {
	return m.tagExistsValue, m.tagExistsErr
}

func TestNewTagCreatorAdapter(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil)
	if adapter == nil {
		t.Error("NewTagCreatorAdapter should return non-nil adapter")
	}
}

func TestTagCreatorAdapter_CreateTag_NilGitAdapter(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil)
	err := adapter.CreateTag(context.Background(), "v1.0.0", "Release 1.0.0")
	if err == nil {
		t.Error("CreateTag should return error when git adapter is nil")
	}
	if err.Error() != "git adapter not configured" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestTagCreatorAdapter_PushTag_NilGitAdapter(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil)
	err := adapter.PushTag(context.Background(), "v1.0.0", "origin")
	if err == nil {
		t.Error("PushTag should return error when git adapter is nil")
	}
	if err.Error() != "git adapter not configured" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestTagCreatorAdapter_TagExists_NilGitAdapter(t *testing.T) {
	adapter := NewTagCreatorAdapter(nil)
	exists, err := adapter.TagExists(context.Background(), "v1.0.0")
	if err == nil {
		t.Error("TagExists should return error when git adapter is nil")
	}
	if exists {
		t.Error("TagExists should return false when git adapter is nil")
	}
}

func TestNewPublisherAdapter(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC)
	if adapter == nil {
		t.Error("NewPublisherAdapter should return non-nil adapter")
	}
}

func TestNewPublisherAdapter_WithSkipPush(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC, WithSkipPush(true))
	if adapter == nil {
		t.Fatal("NewPublisherAdapter should return non-nil adapter")
	}
	if !adapter.skipPush {
		t.Error("skipPush should be true")
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_NilTagCreator(t *testing.T) {
	adapter := NewPublisherAdapter(nil, nil, nil)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Error("ExecuteStep should return error when tag creator is nil")
	}
	if result != nil {
		t.Error("result should be nil on error")
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_TagExists(t *testing.T) {
	mockTC := &mockTagCreator{tagExistsValue: true}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Errorf("ExecuteStep should not return error when tag exists: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Success {
		t.Error("result should be successful for idempotent tag")
	}
	if result.Output != "Tag v1.0.0 already exists (idempotent)" {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_TagExistsError(t *testing.T) {
	expectedErr := errors.New("failed to check tag")
	mockTC := &mockTagCreator{tagExistsErr: expectedErr}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Error("ExecuteStep should return error when tag exists check fails")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Success {
		t.Error("result should not be successful")
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_CreateTagError(t *testing.T) {
	expectedErr := errors.New("failed to create tag")
	mockTC := &mockTagCreator{
		tagExistsValue: false,
		createTagErr:   expectedErr,
	}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Error("ExecuteStep should return error when tag creation fails")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Success {
		t.Error("result should not be successful")
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_PushTagError(t *testing.T) {
	expectedErr := errors.New("failed to push tag")
	mockTC := &mockTagCreator{
		tagExistsValue: false,
		pushTagErr:     expectedErr,
	}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Error("ExecuteStep should return error when tag push fails")
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Success {
		t.Error("result should not be successful")
	}
	if result.Output != "Created tag v1.0.0" {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_Success(t *testing.T) {
	mockTC := &mockTagCreator{tagExistsValue: false}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Errorf("ExecuteStep should not return error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Success {
		t.Error("result should be successful")
	}
	if result.Output != "Created and pushed tag v1.0.0" {
		t.Errorf("unexpected output: %s", result.Output)
	}
	if len(mockTC.createTagCalls) != 1 {
		t.Errorf("expected 1 create tag call, got %d", len(mockTC.createTagCalls))
	}
	if len(mockTC.pushTagCalls) != 1 {
		t.Errorf("expected 1 push tag call, got %d", len(mockTC.pushTagCalls))
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_SkipPush(t *testing.T) {
	mockTC := &mockTagCreator{tagExistsValue: false}
	adapter := NewPublisherAdapter(nil, nil, mockTC, WithSkipPush(true))

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Errorf("ExecuteStep should not return error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Success {
		t.Error("result should be successful")
	}
	if result.Output != "Created tag v1.0.0" {
		t.Errorf("unexpected output: %s", result.Output)
	}
	if len(mockTC.createTagCalls) != 1 {
		t.Errorf("expected 1 create tag call, got %d", len(mockTC.createTagCalls))
	}
	if len(mockTC.pushTagCalls) != 0 {
		t.Errorf("expected 0 push tag calls, got %d", len(mockTC.pushTagCalls))
	}
}

func TestPublisherAdapter_ExecuteStep_NonTagStep_NilExecutor(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "notify",
		Type: domain.StepTypeNotify,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err == nil {
		t.Error("ExecuteStep should return error when executor is nil")
	}
	if result != nil {
		t.Error("result should be nil on error")
	}
}

func TestPublisherAdapter_CheckIdempotency_TagStep(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	// Without git adapter, should return false
	exists, err := adapter.CheckIdempotency(context.Background(), run, step)
	if err != nil {
		t.Errorf("CheckIdempotency should not return error: %v", err)
	}
	if exists {
		t.Error("CheckIdempotency should return false without git adapter")
	}
}

func TestPublisherAdapter_CheckIdempotency_NonTagStep(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRun(t)
	step := &domain.StepPlan{
		Name: "notify",
		Type: domain.StepTypeNotify,
	}

	exists, err := adapter.CheckIdempotency(context.Background(), run, step)
	if err != nil {
		t.Errorf("CheckIdempotency should not return error: %v", err)
	}
	if exists {
		t.Error("CheckIdempotency should return false for non-tag steps")
	}
}

func TestPublisherAdapter_mapStepTypeToHook(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	tests := []struct {
		stepType domain.StepType
	}{
		{domain.StepTypeTag},
		{domain.StepTypeBuild},
		{domain.StepTypeArtifact},
		{domain.StepTypeNotify},
		{domain.StepTypePlugin},
		{domain.StepTypeChangelog},
		{domain.StepType("unknown")},
	}

	for _, tt := range tests {
		hook := adapter.mapStepTypeToHook(tt.stepType)
		// Just verify it doesn't panic and returns something
		_ = hook
	}
}

func TestNewNotesGeneratorAdapter(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)
	if adapter == nil {
		t.Error("NewNotesGeneratorAdapter should return non-nil adapter")
	}
}

func TestNotesGeneratorAdapter_Generate_NoAIService(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)

	run := createTestReleaseRunWithChangeset(t)
	options := ports.NotesOptions{
		AudiencePreset: "developers",
		TonePreset:     "professional",
	}

	notes, err := adapter.Generate(context.Background(), run, options)
	if err != nil {
		t.Errorf("Generate should not return error: %v", err)
	}
	if notes == nil {
		t.Fatal("notes should not be nil")
	}
	if notes.Provider != "basic" {
		t.Errorf("expected basic provider, got: %s", notes.Provider)
	}
}

func TestNotesGeneratorAdapter_Generate_NoChangeset(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)

	run := createTestReleaseRun(t)
	options := ports.NotesOptions{
		AudiencePreset: "developers",
		TonePreset:     "professional",
	}

	notes, err := adapter.Generate(context.Background(), run, options)
	if err != nil {
		t.Errorf("Generate should not return error: %v", err)
	}
	if notes == nil {
		t.Fatal("notes should not be nil")
	}
	if notes.Text != "Release 1.0.0" {
		t.Errorf("unexpected notes text: %s", notes.Text)
	}
}

func TestNotesGeneratorAdapter_ComputeInputsHash(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)

	run := createTestReleaseRun(t)
	options := ports.NotesOptions{
		AudiencePreset: "developers",
		TonePreset:     "professional",
		UseAI:          false,
	}

	hash1 := adapter.ComputeInputsHash(run, options)
	if hash1 == "" {
		t.Error("ComputeInputsHash should return non-empty hash")
	}
	if len(hash1) != 16 {
		t.Errorf("expected 16 character hash, got: %d", len(hash1))
	}

	// Different options should produce different hash
	options.UseAI = true
	options.Provider = "openai"
	options.Model = "gpt-4"
	hash2 := adapter.ComputeInputsHash(run, options)
	if hash1 == hash2 {
		t.Error("different options should produce different hashes")
	}
}

func TestNotesGeneratorAdapter_mapTone(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)

	tests := []struct {
		preset string
	}{
		{"technical"},
		{"friendly"},
		{"casual"},
		{"professional"},
		{"formal"},
		{"excited"},
		{"marketing"},
		{"unknown"},
		{""},
	}

	for _, tt := range tests {
		tone := adapter.mapTone(tt.preset)
		// Just verify it doesn't panic and returns something
		_ = tone
	}
}

func TestNotesGeneratorAdapter_mapAudience(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)

	tests := []struct {
		preset string
	}{
		{"developer"},
		{"developers"},
		{"user"},
		{"users"},
		{"public"},
		{"all"},
		{"marketing"},
		{"unknown"},
		{""},
	}

	for _, tt := range tests {
		audience := adapter.mapAudience(tt.preset)
		// Just verify it doesn't panic and returns something
		_ = audience
	}
}

func TestNewVersionWriterAdapter(t *testing.T) {
	adapter := NewVersionWriterAdapter(nil, "/tmp")
	if adapter == nil {
		t.Error("NewVersionWriterAdapter should return non-nil adapter")
	}
}

func TestVersionWriterAdapter_WriteVersion_NilGitAdapter(t *testing.T) {
	adapter := NewVersionWriterAdapter(nil, "/tmp")
	ver, _ := version.Parse("1.0.0")
	err := adapter.WriteVersion(context.Background(), ver)
	if err == nil {
		t.Error("WriteVersion should return error when git adapter is nil")
	}
}

func TestVersionWriterAdapter_WriteChangelog_NilGitAdapter(t *testing.T) {
	adapter := NewVersionWriterAdapter(nil, "/tmp")
	ver, _ := version.Parse("1.0.0")
	err := adapter.WriteChangelog(context.Background(), ver, "Release notes")
	if err == nil {
		t.Error("WriteChangelog should return error when git adapter is nil")
	}
}

func TestNotesGeneratorAdapter_convertToCategorizedChanges(t *testing.T) {
	adapter := NewNotesGeneratorAdapter(nil, nil)

	// Create a changeset with various commit types
	cs := changes.NewChangeSet("test-cs", "v0.9.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add new endpoint", changes.WithScope("api")))
	cs.AddCommit(changes.NewConventionalCommit("def456", changes.CommitTypeFix, "fix login bug", changes.WithScope("auth")))
	cs.AddCommit(changes.NewConventionalCommit("ghi789", changes.CommitTypeDocs, "update README"))
	cs.AddCommit(changes.NewConventionalCommit("jkl012", changes.CommitTypePerf, "improve query speed"))
	cs.AddCommit(changes.NewConventionalCommit("mno345", changes.CommitTypeRefactor, "clean up code"))
	cs.AddCommit(changes.NewConventionalCommit("pqr678", changes.CommitTypeFeat, "breaking change", changes.WithBreaking("breaks API")))
	cs.AddCommit(changes.NewConventionalCommit("stu901", changes.CommitTypeChore, "update deps"))

	result := adapter.convertToCategorizedChanges(cs)

	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(result.Features))
	}
	if len(result.Fixes) != 1 {
		t.Errorf("expected 1 fix, got %d", len(result.Fixes))
	}
	if len(result.Documentation) != 1 {
		t.Errorf("expected 1 doc, got %d", len(result.Documentation))
	}
	if len(result.Performance) != 1 {
		t.Errorf("expected 1 perf, got %d", len(result.Performance))
	}
	if len(result.Refactoring) != 1 {
		t.Errorf("expected 1 refactor, got %d", len(result.Refactoring))
	}
	if len(result.Breaking) != 1 {
		t.Errorf("expected 1 breaking, got %d", len(result.Breaking))
	}
	if len(result.Other) != 1 {
		t.Errorf("expected 1 other, got %d", len(result.Other))
	}
	if len(result.All) != 7 {
		t.Errorf("expected 7 total, got %d", len(result.All))
	}
}

func TestPublisherAdapter_buildReleaseContext(t *testing.T) {
	mockTC := &mockTagCreator{}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRunWithChangeset(t)
	ctx := adapter.buildReleaseContext(run)

	if ctx.Version.String() != "1.0.0" {
		t.Errorf("unexpected version: %s", ctx.Version.String())
	}
	if ctx.TagName != "v1.0.0" {
		t.Errorf("unexpected tag name: %s", ctx.TagName)
	}
}

func TestPublisherAdapter_ExecuteStep_TagStep_WithNotes(t *testing.T) {
	mockTC := &mockTagCreator{tagExistsValue: false}
	adapter := NewPublisherAdapter(nil, nil, mockTC)

	run := createTestReleaseRunWithNotes(t)
	step := &domain.StepPlan{
		Name: "create-tag",
		Type: domain.StepTypeTag,
	}

	result, err := adapter.ExecuteStep(context.Background(), run, step)
	if err != nil {
		t.Errorf("ExecuteStep should not return error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Success {
		t.Error("result should be successful")
	}

	// Verify the tag message includes notes
	if len(mockTC.createTagCalls) != 1 {
		t.Fatalf("expected 1 create tag call, got %d", len(mockTC.createTagCalls))
	}
	if mockTC.createTagCalls[0].message == "" {
		t.Error("tag message should not be empty")
	}
}

// Helper functions for creating test data

func createTestReleaseRun(t *testing.T) *domain.ReleaseRun {
	t.Helper()
	ver, _ := version.Parse("1.0.0")
	prevVer, _ := version.Parse("0.9.0")
	now := time.Now()

	run := domain.NewReleaseRun(
		"test-repo",                      // repoID
		"/tmp/test-repo",                 // repoRoot
		"v0.9.0",                         // baseRef
		domain.CommitSHA("abc123def456"), // headSHA
		[]domain.CommitSHA{},             // commits
		"config-hash",                    // configHash
		"plugin-plan-hash",               // pluginPlanHash
	)

	// Use ReconstructState to set the run in a usable state for testing
	run.ReconstructState(domain.RunSnapshot{
		ID:              run.ID(),
		PlanHash:        run.PlanHash(),
		RepoID:          "test-repo",
		RepoRoot:        "/tmp/test-repo",
		BaseRef:         "v0.9.0",
		HeadSHA:         domain.CommitSHA("abc123def456"),
		Commits:         nil,
		ConfigHash:      "config-hash",
		PluginPlanHash:  "plugin-plan-hash",
		VersionCurrent:  prevVer,
		VersionNext:     ver,
		BumpKind:        domain.BumpMinor,
		Confidence:      1.0,
		RiskScore:       0.0,
		Reasons:         nil,
		ActorType:       domain.ActorHuman,
		ActorID:         "test-user",
		Thresholds:      domain.PolicyThresholds{},
		TagName:         "v1.0.0",
		Notes:           nil,
		NotesInputsHash: "",
		Approval:        nil,
		Steps:           nil,
		StepStatus:      make(map[string]*domain.StepStatus),
		State:           domain.StateVersioned,
		History:         nil,
		LastError:       "",
		ChangesetID:     "",
		CreatedAt:       now,
		UpdatedAt:       now,
		PublishedAt:     nil,
	})

	return run
}

func createTestReleaseRunWithChangeset(t *testing.T) *domain.ReleaseRun {
	t.Helper()
	run := createTestReleaseRun(t)

	cs := changes.NewChangeSet("test-cs", "v0.9.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add feature"))
	run.SetChangeSet(cs)

	return run
}

func createTestReleaseRunWithNotes(t *testing.T) *domain.ReleaseRun {
	t.Helper()
	ver, _ := version.Parse("1.0.0")
	prevVer, _ := version.Parse("0.9.0")
	now := time.Now()

	run := domain.NewReleaseRun(
		"test-repo",
		"/tmp/test-repo",
		"v0.9.0",
		domain.CommitSHA("abc123def456"),
		[]domain.CommitSHA{},
		"config-hash",
		"plugin-plan-hash",
	)

	notes := &domain.ReleaseNotes{
		Text:        "## What's New\n\n- Feature A\n- Feature B",
		GeneratedAt: now,
	}

	// Use ReconstructState to set the run with notes
	run.ReconstructState(domain.RunSnapshot{
		ID:              run.ID(),
		PlanHash:        run.PlanHash(),
		RepoID:          "test-repo",
		RepoRoot:        "/tmp/test-repo",
		BaseRef:         "v0.9.0",
		HeadSHA:         domain.CommitSHA("abc123def456"),
		Commits:         nil,
		ConfigHash:      "config-hash",
		PluginPlanHash:  "plugin-plan-hash",
		VersionCurrent:  prevVer,
		VersionNext:     ver,
		BumpKind:        domain.BumpMinor,
		Confidence:      1.0,
		RiskScore:       0.0,
		Reasons:         nil,
		ActorType:       domain.ActorHuman,
		ActorID:         "test-user",
		Thresholds:      domain.PolicyThresholds{},
		TagName:         "v1.0.0",
		Notes:           notes,
		NotesInputsHash: "",
		Approval:        nil,
		Steps:           nil,
		StepStatus:      make(map[string]*domain.StepStatus),
		State:           domain.StateNotesReady,
		History:         nil,
		LastError:       "",
		ChangesetID:     "",
		CreatedAt:       now,
		UpdatedAt:       now,
		PublishedAt:     nil,
	})

	return run
}
