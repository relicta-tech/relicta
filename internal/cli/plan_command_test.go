package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	domainversion "github.com/relicta-tech/relicta/internal/domain/version"
)

func TestBuildPlanAnalysisConfig_Flags(t *testing.T) {
	origAnalyze := planAnalyze
	origReview := planReview
	origMinConf := planMinConfidence
	origDisableAI := planDisableAI
	defer func() {
		planAnalyze = origAnalyze
		planReview = origReview
		planMinConfidence = origMinConf
		planDisableAI = origDisableAI
	}()

	planAnalyze = true
	planReview = true
	planMinConfidence = 0.5
	planDisableAI = true

	cfg, updated := buildPlanAnalysisConfig(true)
	if !updated {
		t.Fatalf("expected config to be updated")
	}
	if cfg.MinConfidence != 0.5 {
		t.Fatalf("min confidence = %.2f, want %.2f", cfg.MinConfidence, 0.5)
	}
	if cfg.EnableAI {
		t.Fatal("expected AI to be disabled")
	}
}

func TestOutputAnalysisJSON(t *testing.T) {
	result := &analysis.AnalysisResult{}
	commitInfos := []analysis.CommitInfo{}
	out := captureStdout(func() {
		if err := outputAnalysisJSON(result, commitInfos); err != nil {
			t.Fatalf("outputAnalysisJSON error: %v", err)
		}
	})
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if decoded["total_commits"] == nil {
		t.Fatal("expected total_commits field")
	}
	if tc, ok := decoded["total_commits"].(float64); !ok || tc != 0 {
		t.Fatalf("unexpected total_commits: %v", decoded["total_commits"])
	}
}

func TestRunPlanAnalyze_Success(t *testing.T) {
	outputJSON = true
	t.Cleanup(func() { outputJSON = false })

	hash := sourcecontrol.CommitHash("abc123")
	result := &analysis.AnalysisResult{
		Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
			hash: {
				CommitHash: hash,
				Type:       changes.CommitTypeFeat,
			},
		},
	}
	app := spyCLIApp{
		plan: stubPlanUseCase{
			analysisResult: result,
			commitInfos: []analysis.CommitInfo{
				{Hash: hash, Subject: "add feature"},
			},
		},
	}

	if err := runPlanAnalyze(context.Background(), app, apprelease.PlanReleaseInput{}); err != nil {
		t.Fatalf("runPlanAnalyze error: %v", err)
	}
}

func TestRunPlanAnalyze_Error(t *testing.T) {
	outputJSON = true
	t.Cleanup(func() { outputJSON = false })

	app := spyCLIApp{
		plan: stubPlanUseCase{
			analyzeErr: assertErr("boom"),
		},
	}

	if err := runPlanAnalyze(context.Background(), app, apprelease.PlanReleaseInput{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunPlanReview_Success(t *testing.T) {
	outputJSON = true
	t.Cleanup(func() { outputJSON = false })

	hash := sourcecontrol.CommitHash("deadbeef")
	result := &analysis.AnalysisResult{}
	app := spyCLIApp{
		plan: stubPlanUseCase{
			analysisResult: result,
			commitInfos: []analysis.CommitInfo{
				{Hash: hash, Subject: "fix bug"},
			},
			executeOutput: &apprelease.PlanReleaseOutput{
				ReleaseID:      domainrelease.ReleaseID("test-release"),
				CurrentVersion: domainversion.MustParse("1.0.0"),
				NextVersion:    domainversion.MustParse("1.1.0"),
				ReleaseType:    changes.ReleaseTypeMinor,
				ChangeSet:      newTestChangeSet(),
				RepositoryName: "example",
				Branch:         "main",
				Analysis:       result,
			},
		},
	}

	withStdin("\n", func() {
		if err := runPlanReview(context.Background(), app, apprelease.PlanReleaseInput{}, "https://example.com"); err != nil {
			t.Fatalf("runPlanReview error: %v", err)
		}
	})
}

func TestReviewCommitClassifications_ParseOverride(t *testing.T) {
	hash := sourcecontrol.CommitHash("override")
	result := &analysis.AnalysisResult{
		Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
			hash: {
				CommitHash: hash,
				Type:       changes.CommitTypeDocs,
				Confidence: 0.1,
			},
		},
	}

	infos := []analysis.CommitInfo{{Hash: hash, Subject: "docs update"}}
	withStdin("feat\n", func() {
		got, err := reviewCommitClassifications(result, infos)
		if err != nil {
			t.Fatalf("reviewCommitClassifications error: %v", err)
		}
		if got[hash].Type != changes.CommitTypeFeat {
			t.Fatalf("expected manual override to feature, got %s", got[hash].Type)
		}
	})
}

func TestParseClassificationOverride_Skip(t *testing.T) {
	hash := sourcecontrol.CommitHash("skip")
	current := &analysis.CommitClassification{CommitHash: hash}
	got, err := parseClassificationOverride("skip", current)
	if err != nil {
		t.Fatalf("parseClassificationOverride error: %v", err)
	}
	if !got.ShouldSkip {
		t.Fatal("expected ShouldSkip")
	}
}

func TestParseClassificationOverride_InvalidType(t *testing.T) {
	current := &analysis.CommitClassification{CommitHash: sourcecontrol.CommitHash("invalid")}
	if _, err := parseClassificationOverride("bogus", current); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestClassificationTypeLabelVariants(t *testing.T) {
	if classificationTypeLabel(nil) != "unknown" {
		t.Fatal("expected unknown label for nil classification")
	}
	cl := &analysis.CommitClassification{
		Type: changes.CommitTypeFix,
	}
	if classificationTypeLabel(cl) != string(changes.CommitTypeFix) {
		t.Fatalf("unexpected label: %s", classificationTypeLabel(cl))
	}
	cl.ShouldSkip = true
	if classificationTypeLabel(cl) != "skip" {
		t.Fatal("expected skip label for ShouldSkip")
	}
}

func TestTrimList_Limit(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := trimList(items, 4)
	if len(got) != 5 || got[len(got)-1] != "..." {
		t.Fatalf("unexpected trimmed list: %#v", got)
	}
}

type stubPlanUseCase struct {
	analysisResult *analysis.AnalysisResult
	commitInfos    []analysis.CommitInfo
	analyzeErr     error
	executeOutput  *apprelease.PlanReleaseOutput
	executeErr     error
}

func (s stubPlanUseCase) Execute(context.Context, apprelease.PlanReleaseInput) (*apprelease.PlanReleaseOutput, error) {
	if s.executeErr != nil {
		return nil, s.executeErr
	}
	if s.executeOutput != nil {
		return s.executeOutput, nil
	}
	return &apprelease.PlanReleaseOutput{}, nil
}

func (s stubPlanUseCase) AnalyzeCommits(context.Context, apprelease.PlanReleaseInput) (*analysis.AnalysisResult, []analysis.CommitInfo, error) {
	return s.analysisResult, s.commitInfos, s.analyzeErr
}

type fakePlanUseCase struct {
	analysisResult *analysis.AnalysisResult
	commitInfos    []analysis.CommitInfo
	executeOutput  *apprelease.PlanReleaseOutput

	analyzeCalled bool
	executeCalled bool
}

func (f *fakePlanUseCase) Execute(ctx context.Context, input apprelease.PlanReleaseInput) (*apprelease.PlanReleaseOutput, error) {
	f.executeCalled = true
	return f.executeOutput, nil
}

func (f *fakePlanUseCase) AnalyzeCommits(ctx context.Context, input apprelease.PlanReleaseInput) (*analysis.AnalysisResult, []analysis.CommitInfo, error) {
	f.analyzeCalled = true
	return f.analysisResult, f.commitInfos, nil
}

type spyCLIApp struct {
	plan planReleaseUseCase
}

func (s spyCLIApp) Close() error                                { return nil }
func (s spyCLIApp) GitAdapter() sourcecontrol.GitRepository     { return nil }
func (s spyCLIApp) ReleaseRepository() domainrelease.Repository { return nil }
func (s spyCLIApp) PlanRelease() planReleaseUseCase             { return s.plan }
func (s spyCLIApp) GenerateNotes() generateNotesUseCase         { return nil }
func (s spyCLIApp) ApproveRelease() approveReleaseUseCase       { return nil }
func (s spyCLIApp) PublishRelease() publishReleaseUseCase       { return nil }
func (s spyCLIApp) CalculateVersion() calculateVersionUseCase   { return nil }
func (s spyCLIApp) SetVersion() setVersionUseCase               { return nil }
func (s spyCLIApp) HasAI() bool                                 { return false }
func (s spyCLIApp) HasGovernance() bool                         { return false }
func (s spyCLIApp) GovernanceService() *governance.Service      { return nil }

type testCLIApp struct {
	plan         planReleaseUseCase
	gitRepo      sourcecontrol.GitRepository
	setVersionUC setVersionUseCase
	releaseRepo  domainrelease.Repository
	hasAI        bool
	hasGov       bool
	govSvc       *governance.Service
}

func (t testCLIApp) Close() error                                { return nil }
func (t testCLIApp) GitAdapter() sourcecontrol.GitRepository     { return t.gitRepo }
func (t testCLIApp) ReleaseRepository() domainrelease.Repository { return t.releaseRepo }
func (t testCLIApp) PlanRelease() planReleaseUseCase             { return t.plan }
func (t testCLIApp) GenerateNotes() generateNotesUseCase         { return nil }
func (t testCLIApp) ApproveRelease() approveReleaseUseCase       { return nil }
func (t testCLIApp) PublishRelease() publishReleaseUseCase       { return nil }
func (t testCLIApp) CalculateVersion() calculateVersionUseCase   { return nil }
func (t testCLIApp) SetVersion() setVersionUseCase               { return t.setVersionUC }
func (t testCLIApp) HasAI() bool                                 { return t.hasAI }
func (t testCLIApp) HasGovernance() bool                         { return t.hasGov }
func (t testCLIApp) GovernanceService() *governance.Service      { return t.govSvc }

type stubGitRepo struct{}

func (stubGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{
		Path:          ".",
		Name:          "repo",
		CurrentBranch: "main",
		RemoteURL:     "https://example.com",
	}, nil
}
func (stubGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (stubGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (stubGitRepo) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}
func (stubGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return nil, nil
}
func (stubGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (stubGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (stubGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return nil, nil
}
func (stubGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (stubGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (stubGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (stubGitRepo) DeleteTag(ctx context.Context, name string) error {
	return nil
}
func (stubGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}
func (stubGitRepo) IsDirty(ctx context.Context) (bool, error) {
	return false, nil
}
func (stubGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}
func (stubGitRepo) Fetch(ctx context.Context, remote string) error {
	return nil
}
func (stubGitRepo) Pull(ctx context.Context, remote, branch string) error {
	return nil
}
func (stubGitRepo) Push(ctx context.Context, remote, branch string) error {
	return nil
}

func assertErr(message string) error {
	return &assertError{message}
}

type assertError struct {
	s string
}

func (e *assertError) Error() string {
	return e.s
}

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old
	return buf.String()
}

func withStdin(input string, fn func()) {
	old := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.Write([]byte(input))
	_ = w.Close()
	os.Stdin = r
	fn()
	_ = r.Close()
	os.Stdin = old
}

func newTestChangeSet() *changes.ChangeSet {
	cs := changes.NewChangeSet(changes.ChangeSetID("cs-test"), "main", "feature")
	cs.AddCommit(changes.NewConventionalCommit("deadbeef", changes.CommitTypeFix, "fix bug"))
	return cs
}

func newPlanOutput() *apprelease.PlanReleaseOutput {
	return &apprelease.PlanReleaseOutput{
		ReleaseID:      domainrelease.ReleaseID("release-123"),
		CurrentVersion: domainversion.MustParse("1.0.0"),
		NextVersion:    domainversion.MustParse("1.1.0"),
		ReleaseType:    changes.ReleaseTypeMinor,
		ChangeSet:      newTestChangeSet(),
		RepositoryName: "relicta",
		Branch:         "main",
	}
}

func TestRunPlan_AnalyzeFlag(t *testing.T) {
	origPlanAnalyze := planAnalyze
	origPlanReview := planReview
	origOutputJSON := outputJSON
	origCfg := cfg
	origNewContainerApp := newContainerApp
	defer func() {
		planAnalyze = origPlanAnalyze
		planReview = origPlanReview
		outputJSON = origOutputJSON
		cfg = origCfg
		newContainerApp = origNewContainerApp
	}()

	planAnalyze = true
	planReview = false
	outputJSON = true
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	fakePlan := &fakePlanUseCase{
		analysisResult: &analysis.AnalysisResult{
			Stats: analysis.AnalysisStats{TotalCommits: 1},
			Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
				sourcecontrol.CommitHash("abc123"): {
					CommitHash: sourcecontrol.CommitHash("abc123"),
					Type:       changes.CommitTypeFix,
				},
			},
		},
		commitInfos: []analysis.CommitInfo{
			{Hash: sourcecontrol.CommitHash("abc123"), Subject: "fix bug"},
		},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return testCLIApp{
			plan:    fakePlan,
			gitRepo: stubGitRepo{},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPlan(cmd, nil); err != nil {
		t.Fatalf("runPlan error: %v", err)
	}
	if !fakePlan.analyzeCalled {
		t.Fatal("expected analyze to be called")
	}
}

func TestRunPlan_NormalFlowJSON(t *testing.T) {
	origPlanAnalyze := planAnalyze
	origPlanReview := planReview
	origOutputJSON := outputJSON
	origCfg := cfg
	origNewContainerApp := newContainerApp
	defer func() {
		planAnalyze = origPlanAnalyze
		planReview = origPlanReview
		outputJSON = origOutputJSON
		cfg = origCfg
		newContainerApp = origNewContainerApp
	}()

	planAnalyze = false
	planReview = false
	outputJSON = true
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	fakePlan := &fakePlanUseCase{
		analysisResult: &analysis.AnalysisResult{
			Stats: analysis.AnalysisStats{TotalCommits: 1},
		},
		commitInfos: []analysis.CommitInfo{
			{Hash: sourcecontrol.CommitHash("def456"), Subject: "feat: add feature"},
		},
		executeOutput: newPlanOutput(),
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return testCLIApp{
			plan:    fakePlan,
			gitRepo: stubGitRepo{},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPlan(cmd, nil); err != nil {
		t.Fatalf("runPlan error: %v", err)
	}
	if !fakePlan.executeCalled {
		t.Fatal("expected execute to be called")
	}
}

func TestRunPlanReviewFlow(t *testing.T) {
	origPlanAnalyze := planAnalyze
	origPlanReview := planReview
	origOutputJSON := outputJSON
	origCfg := cfg
	origNewContainerApp := newContainerApp
	defer func() {
		planAnalyze = origPlanAnalyze
		planReview = origPlanReview
		outputJSON = origOutputJSON
		cfg = origCfg
		newContainerApp = origNewContainerApp
	}()

	planReview = true
	outputJSON = false
	cfg = &config.Config{Versioning: config.VersioningConfig{TagPrefix: "v"}}

	fakePlan := &fakePlanUseCase{
		analysisResult: &analysis.AnalysisResult{
			Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
				sourcecontrol.CommitHash("cafebabe"): {
					CommitHash: sourcecontrol.CommitHash("cafebabe"),
					Type:       changes.CommitTypeFix,
				},
			},
		},
		commitInfos: []analysis.CommitInfo{
			{Hash: sourcecontrol.CommitHash("cafebabe"), Subject: "fix bug"},
		},
		executeOutput: newPlanOutput(),
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return testCLIApp{
			plan:    fakePlan,
			gitRepo: stubGitRepo{},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	withStdin("\n", func() {
		if err := runPlan(cmd, nil); err != nil {
			t.Fatalf("runPlan error: %v", err)
		}
	})

	if !fakePlan.analyzeCalled {
		t.Fatal("expected analyze to be called")
	}
	if !fakePlan.executeCalled {
		t.Fatal("expected execute to be called")
	}
}

func TestRunPlanFlagConflicts(t *testing.T) {
	origPlanAnalyze := planAnalyze
	origPlanReview := planReview
	origOutputJSON := outputJSON
	defer func() {
		planAnalyze = origPlanAnalyze
		planReview = origPlanReview
		outputJSON = origOutputJSON
	}()

	planAnalyze = true
	planReview = true
	outputJSON = false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPlan(cmd, nil); err == nil {
		t.Fatal("expected error when --analyze and --review are both set")
	}

	outputJSON = true
	planAnalyze = false
	planReview = true
	if err := runPlan(cmd, nil); err == nil || !strings.Contains(err.Error(), "--review is not supported with --json output") {
		t.Fatalf("expected review/json conflict error, got: %v", err)
	}
}

func TestOutputAnalysisText_IncludesDetails(t *testing.T) {
	hash1 := sourcecontrol.CommitHash("abc123")
	hash2 := sourcecontrol.CommitHash("def456")
	result := &analysis.AnalysisResult{
		Stats: analysis.AnalysisStats{
			TotalCommits:         2,
			AverageConfidence:    0.54,
			ConventionalCount:    1,
			HeuristicCount:       1,
			ASTCount:             0,
			AICount:              0,
			SkippedCount:         0,
			LowConfidenceCount:   1,
			LowConfidenceCommits: []sourcecontrol.CommitHash{hash2},
		},
		Classifications: map[sourcecontrol.CommitHash]*analysis.CommitClassification{
			hash1: {
				CommitHash: hash1,
				Type:       changes.CommitTypeFeat,
				Method:     analysis.MethodHeuristic,
				Confidence: 0.92,
				Reasoning:  "new API",
			},
			hash2: {
				CommitHash: hash2,
				Type:       changes.CommitTypeFix,
				Method:     analysis.MethodManual,
				Confidence: 0.2,
				ShouldSkip: true,
				SkipReason: "manual override",
				Reasoning:  "low relevance",
			},
		},
	}
	infos := []analysis.CommitInfo{
		{Hash: hash1, Subject: "feat: add feature"},
		{Hash: hash2, Subject: "fix: cleanup", Files: []string{"file.go"}},
	}

	out := captureStdout(func() {
		if err := outputAnalysisText(result, infos); err != nil {
			t.Fatalf("outputAnalysisText error: %v", err)
		}
	})

	if !strings.Contains(out, "Commit Breakdown") {
		t.Fatalf("expected breakdown header, got %q", out)
	}
	if !strings.Contains(out, "Low confidence commits") {
		t.Fatalf("expected low confidence section, got %q", out)
	}
}

func TestGetGovernanceRiskPreview_NoService(t *testing.T) {
	output := newPlanOutput()
	app := testCLIApp{}
	if got := getGovernanceRiskPreview(context.Background(), app, output, "https://example.com"); got != nil {
		t.Fatalf("expected nil risk preview when service missing, got %v", got)
	}
}
