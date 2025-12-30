package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
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
	rel := release.NewReleaseRunForTest("notes-missing", "main", ".")

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
	defer func() {
		cfg = origCfg
		outputJSON = origOutput
	}()

	cfg = config.DefaultConfig()
	outputJSON = true

	rel := newNotesReadyRelease(t, "approve-json")
	app := testCLIApp{
		gitRepo:     stubGitRepo{},
		releaseRepo: testReleaseRepo{latest: rel},
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
	rel := release.NewReleaseRunForTest("no-notes", "main", ".")
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
