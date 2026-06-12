package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"strings"

	"github.com/relicta-tech/relicta/v4/internal/config"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

func TestPromptForApprovalReadsYes(t *testing.T) {
	origApprove := approveYes
	origCIMode := ciMode
	origCfg := cfg
	origStdin := os.Stdin
	defer func() {
		approveYes = origApprove
		ciMode = origCIMode
		cfg = origCfg
		os.Stdin = origStdin
	}()

	cfg = config.DefaultConfig()
	cfg.Workflow.RequireApproval = true
	approveYes = false
	ciMode = false

	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.Write([]byte("y\n"))
	_ = w.Close()

	approved, err := promptForApproval()
	if err != nil {
		t.Fatalf("promptForApproval error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval to be true")
	}
}

func TestHandleNotesEditingNoNotes(t *testing.T) {
	origEdit := approveEdit
	defer func() { approveEdit = origEdit }()

	approveEdit = true
	rel := domainrelease.NewReleaseRunForTest("notes-missing", "main", ".")

	edited, err := handleNotesEditing(rel)
	if err != nil {
		t.Fatalf("handleNotesEditing error: %v", err)
	}
	if edited != nil {
		t.Fatal("expected no edited notes when notes are missing")
	}
}

func TestRunApproveOutputsJSONWithStub(t *testing.T) {
	origCfg := cfg
	origOutput := outputJSON
	origApproveYes := approveYes
	defer func() {
		cfg = origCfg
		outputJSON = origOutput
		approveYes = origApproveYes
	}()

	cfg = config.DefaultConfig()
	outputJSON = true
	approveYes = true // JSON mode is non-interactive; --yes authorizes the approval

	rel := newNotesReadyRelease(t, "approve-json")
	portsRepo := &portsReleaseRepoStub{run: rel}
	app := testCLIApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
		releaseServices: &domainrelease.Services{
			ApproveRelease: releaseapp.NewApproveReleaseUseCase(portsRepo, nil, nil, nil),
			Repository:     portsRepo,
		},
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runApprove(cmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runApprove error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	if !bytes.Contains(buf.Bytes(), []byte("\"release_id\"")) {
		t.Fatalf("expected JSON output, got: %s", buf.String())
	}
	if rel.State() != domain.StateApproved {
		t.Fatalf("approve --json --yes did not approve: state %s", rel.State())
	}
}

func TestRunApproveDryRunAutoApprove(t *testing.T) {
	origCfg := cfg
	origOutput := outputJSON
	origApproveYes := approveYes
	origDryRun := dryRun
	origCIMode := ciMode
	defer func() {
		cfg = origCfg
		outputJSON = origOutput
		approveYes = origApproveYes
		dryRun = origDryRun
		ciMode = origCIMode
	}()

	cfg = config.DefaultConfig()
	outputJSON = false
	approveYes = true
	dryRun = true
	ciMode = false

	rel := newNotesReadyRelease(t, "approve-dry")
	app := testCLIApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	if err := runApprove(cmd, nil); err != nil {
		t.Fatalf("runApprove error: %v", err)
	}
}

func TestHandleEditApprovalResultNoNotes(t *testing.T) {
	rel := domainrelease.NewReleaseRunForTest("no-notes", "main", ".")
	edited, proceed, err := handleEditApprovalResult(rel)
	if err != nil {
		t.Fatalf("handleEditApprovalResult error: %v", err)
	}
	if edited != nil || proceed {
		t.Fatalf("expected no edits and no proceed, got edited=%v proceed=%v", edited != nil, proceed)
	}
}

func TestHandleEditApprovalResultInvalidEditor(t *testing.T) {
	origEditor := approveEditor
	origCfg := cfg
	defer func() {
		approveEditor = origEditor
		cfg = origCfg
	}()

	approveEditor = "not-allowed"
	cfg = config.DefaultConfig()
	rel := newNotesReadyRelease(t, "edit-notes")

	if _, _, err := handleEditApprovalResult(rel); err == nil {
		t.Fatal("expected error from invalid editor")
	}
}

// portsReleaseRepoStub implements ports.ReleaseRunRepository over a single run.
type portsReleaseRepoStub struct {
	run   *domain.ReleaseRun
	saved bool
}

func (s *portsReleaseRepoStub) Load(_ context.Context, _ domain.RunID) (*domain.ReleaseRun, error) {
	return s.run, nil
}
func (s *portsReleaseRepoStub) LoadBatch(_ context.Context, _ string, _ []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error) {
	return map[domain.RunID]*domain.ReleaseRun{s.run.ID(): s.run}, nil
}
func (s *portsReleaseRepoStub) LoadLatest(_ context.Context, _ string) (*domain.ReleaseRun, error) {
	return s.run, nil
}
func (s *portsReleaseRepoStub) List(_ context.Context, _ string) ([]domain.RunID, error) {
	return []domain.RunID{s.run.ID()}, nil
}
func (s *portsReleaseRepoStub) Save(_ context.Context, run *domain.ReleaseRun) error {
	s.run = run
	s.saved = true
	return nil
}
func (s *portsReleaseRepoStub) SetLatest(_ context.Context, _ string, _ domain.RunID) error {
	return nil
}
func (s *portsReleaseRepoStub) Delete(_ context.Context, _ domain.RunID) error { return nil }
func (s *portsReleaseRepoStub) FindByState(_ context.Context, _ string, _ domain.RunState) ([]*domain.ReleaseRun, error) {
	return []*domain.ReleaseRun{s.run}, nil
}
func (s *portsReleaseRepoStub) FindActive(_ context.Context, _ string) ([]*domain.ReleaseRun, error) {
	return []*domain.ReleaseRun{s.run}, nil
}
func (s *portsReleaseRepoStub) FindByPlanHash(_ context.Context, _ string, _ string) (*domain.ReleaseRun, error) {
	return nil, nil
}

// TestRunApproveCIModeApproves is the regression test for issue #136:
// `approve --ci` must actually approve and persist the release, not dump
// status JSON and exit 0 as a no-op.
func TestRunApproveCIModeApproves(t *testing.T) {
	origCfg := cfg
	origOutput := outputJSON
	origApproveYes := approveYes
	origDryRun := dryRun
	origCIMode := ciMode
	defer func() {
		cfg = origCfg
		outputJSON = origOutput
		approveYes = origApproveYes
		dryRun = origDryRun
		ciMode = origCIMode
	}()

	cfg = config.DefaultConfig()
	ciMode = true
	outputJSON = true // applyCIModeFlag forces this in production
	approveYes = false
	dryRun = false

	rel := newNotesReadyRelease(t, "approve-ci")
	portsRepo := &portsReleaseRepoStub{run: rel}
	services := &domainrelease.Services{
		ApproveRelease: releaseapp.NewApproveReleaseUseCase(portsRepo, nil, nil, nil),
		Repository:     portsRepo,
	}
	app := testCLIApp{
		gitRepo:         stubGitRepo{},
		releaseRepo:     testReleaseRepo{latest: rel},
		releaseServices: services,
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runApprove(cmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runApprove error: %v", err)
	}

	if rel.State() != domain.StateApproved {
		t.Fatalf("approve --ci did not advance state: got %s, want %s", rel.State(), domain.StateApproved)
	}
	if !portsRepo.saved {
		t.Fatal("approve --ci never persisted the approval")
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	out := buf.String()
	if !strings.Contains(out, `"approved": true`) {
		t.Fatalf("expected JSON output with approved=true, got: %s", out)
	}
	if strings.Contains(out, `"tag_name": "v0.0.0"`) {
		t.Fatalf("output reports zero-value tag: %s", out)
	}
	if !strings.Contains(out, `"tag_name": "v1.0.0"`) {
		t.Fatalf("expected planned tag v1.0.0 in output, got: %s", out)
	}
}

// TestRunApproveJSONWithoutCIRefusesToPrompt locks in the new contract:
// plain --json with approval prompting required must error instead of
// silently skipping the approval.
func TestRunApproveJSONWithoutCIRefusesToPrompt(t *testing.T) {
	origCfg := cfg
	origOutput := outputJSON
	origApproveYes := approveYes
	origCIMode := ciMode
	defer func() {
		cfg = origCfg
		outputJSON = origOutput
		approveYes = origApproveYes
		ciMode = origCIMode
	}()

	cfg = config.DefaultConfig()
	cfg.Workflow.RequireApproval = true
	outputJSON = true
	approveYes = false
	ciMode = false

	rel := newNotesReadyRelease(t, "approve-json-prompt")
	app := testCLIApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
	}
	withStubContainerApp(t, app)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runApprove(cmd, nil)
	if err == nil {
		t.Fatal("expected error: --json without --ci/--yes must refuse to prompt")
	}
	if !strings.Contains(err.Error(), "non-interactive") {
		t.Fatalf("unexpected error: %v", err)
	}
}
