package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/config"
)

func TestRunReleaseWorkflowAutoApprove(t *testing.T) {
	origCfg := cfg
	origAutoApprove := releaseAutoApprove
	origSkipPush := releaseSkipPush
	origReleaseForce := releaseForce
	origNewContainerApp := newContainerApp
	defer func() {
		cfg = origCfg
		releaseAutoApprove = origAutoApprove
		releaseSkipPush = origSkipPush
		releaseForce = origReleaseForce
		newContainerApp = origNewContainerApp
	}()

	cfg = config.DefaultConfig()
	cfg.Versioning.GitTag = true
	cfg.Versioning.GitPush = true
	releaseAutoApprove = true
	releaseSkipPush = false
	releaseForce = ""

	planOutput := newPlanOutput()
	planOutput.ChangeSet = newTestChangeSet()

	fakeSet := &fakeSetVersionUseCase{}
	fakePlan := &fakePlanUseCase{executeOutput: planOutput}
	fakeNotes := &fakeGenerateNotesUseCase{}
	fakeApprove := &fakeApproveReleaseUseCase{}
	fakePublish := &fakePublishReleaseUseCase{}

	app := releaseTestApp{
		gitRepo:    stubGitRepo{},
		plan:       fakePlan,
		setVersion: fakeSet,
		generate:   fakeNotes,
		approve:    fakeApprove,
		publish:    fakePublish,
		releaseRepo: testReleaseRepo{
			latest: newTestRelease(t, "release-flow"),
		},
	}

	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := captureCLIStdout(func() {
		if err := runRelease(cmd, nil); err != nil {
			t.Fatalf("runRelease error: %v", err)
		}
	})

	if !strings.Contains(out, "Release workflow completed") && !strings.Contains(out, "Released") {
		t.Fatalf("expected release success output, got: %s", out)
	}
	if !fakePublish.executeCalled {
		t.Fatal("expected publish use case to execute")
	}
}

func TestRunReleaseDryRunSkipsPublish(t *testing.T) {
	origCfg := cfg
	origAutoApprove := releaseAutoApprove
	origNewContainerApp := newContainerApp
	origDryRun := dryRun
	defer func() {
		cfg = origCfg
		releaseAutoApprove = origAutoApprove
		newContainerApp = origNewContainerApp
		dryRun = origDryRun
	}()

	cfg = config.DefaultConfig()
	cfg.Versioning.GitTag = true
	cfg.Versioning.GitPush = true
	releaseAutoApprove = true
	dryRun = true

	planOutput := newPlanOutput()
	planOutput.ChangeSet = newTestChangeSet()

	app := releaseTestApp{
		gitRepo:    stubGitRepo{},
		plan:       &fakePlanUseCase{executeOutput: planOutput},
		setVersion: &fakeSetVersionUseCase{},
		generate:   &fakeGenerateNotesUseCase{},
		approve:    &fakeApproveReleaseUseCase{},
		publish:    &fakePublishReleaseUseCase{},
		releaseRepo: testReleaseRepo{
			latest: newTestRelease(t, "release-dryrun"),
		},
	}
	newContainerApp = func(ctx context.Context, cfg *config.Config) (cliApp, error) {
		return app, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := captureCLIStdout(func() {
		if err := runRelease(cmd, nil); err != nil {
			t.Fatalf("runRelease error: %v", err)
		}
	})

	if !strings.Contains(out, "Dry run - skipping actual publish") {
		t.Fatalf("expected dry run message, got: %s", out)
	}
}
