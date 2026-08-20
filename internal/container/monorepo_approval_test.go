package container

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/config"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/adapters"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// A decision per package only means something if a package that was not decided does not ship.
// A tag is the release — anything watching for `api-v1.5.0` starts on it — so tagging a package
// whose run nobody approved would publish it, whatever the run says.

// runFor plants a release run for a package, in the given state.
func runFor(t *testing.T, root, relPath string, approve bool) {
	t.Helper()
	dir := filepath.Join(root, relPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	ctx := context.Background()
	repo := adapters.NewFileReleaseRunRepository()
	run := domainrelease.NewReleaseRunForTestWithCommits(domainrelease.RunID("run-"+filepath.Base(relPath)), "main", dir)
	if approve {
		if err := run.SetVersion(version.MustParse("1.5.0"), "api-v1.5.0"); err != nil {
			t.Fatalf("SetVersion: %v", err)
		}
		if err := run.Bump("test"); err != nil {
			t.Fatalf("Bump: %v", err)
		}
		if err := run.GenerateNotes(&domainrelease.ReleaseNotes{Text: "notes"}, "hash", "test"); err != nil {
			t.Fatalf("GenerateNotes: %v", err)
		}
		if err := run.Approve("test", false); err != nil {
			t.Fatalf("Approve: %v", err)
		}
	}
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.SetLatest(ctx, dir, run.ID()); err != nil {
		t.Fatalf("SetLatest: %v", err)
	}
}

func appWithFileRuns(root string) *App {
	app := &App{config: config.DefaultConfig()}
	app.releaseRepo = newReleaseRepoBridge(root, nil, nil)
	return app
}

func TestAHeldPackageIsNotTagged(t *testing.T) {
	root := t.TempDir()
	runFor(t, root, filepath.Join("packages", "api"), true)
	runFor(t, root, filepath.Join("packages", "web"), false)

	app := appWithFileRuns(root)
	tags := []appmonorepo.PackageTag{
		{Name: "api", RelPath: filepath.Join("packages", "api"), Tag: "api-v1.5.0"},
		{Name: "web", RelPath: filepath.Join("packages", "web"), Tag: "web-v3.0.0"},
	}

	kept := app.approvedOnly(context.Background(), root, tags)

	if len(kept) != 1 || kept[0].Name != "api" {
		t.Fatalf("kept %+v, want api alone.\nA package whose run was never approved would be "+
			"released by the tag, whatever the decision says", kept)
	}
}

// A package with no run of its own is still tagged: tagging predates per-package runs, and an
// upgrade that silently stopped tagging would be worse than one that tags too much.
func TestAPackageWithNoRunIsStillTagged(t *testing.T) {
	root := t.TempDir()
	app := appWithFileRuns(root)

	tags := []appmonorepo.PackageTag{{Name: "api", RelPath: filepath.Join("packages", "api"), Tag: "api-v1.5.0"}}
	if kept := app.approvedOnly(context.Background(), root, tags); len(kept) != 1 {
		t.Errorf("kept %+v, want the package with no run of its own", kept)
	}
}
