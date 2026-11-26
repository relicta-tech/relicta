package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appmonorepo "github.com/relicta-tech/relicta/v5/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v5/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v5/internal/domain/version"
)

type versionFileRootGitRepo struct {
	stubGitRepo
	root string
}

type latestTagGitRepo struct {
	stubGitRepo
	tag *sourcecontrol.Tag
	err error
}

func (g latestTagGitRepo) GetLatestVersionTag(context.Context, string) (*sourcecontrol.Tag, error) {
	return g.tag, g.err
}

func (g versionFileRootGitRepo) GetInfo(context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{Path: g.root}, nil
}

func TestMonorepoBumpConfigHelpers(t *testing.T) {
	oldCfg := cfg
	t.Cleanup(func() { cfg = oldCfg })
	cfg = config.DefaultConfig()
	if got := monorepoTagPrefixes(); got != nil {
		t.Fatalf("empty tag overrides = %#v, want nil", got)
	}
	if got := monorepoSkipped(); got != nil {
		t.Fatalf("empty skip overrides = %#v, want nil", got)
	}
	cfg.Monorepo.PackageOverrides = map[string]config.PackageOverrideConfig{
		"packages/api":  {TagPrefix: "api-v", SkipVersioning: true},
		"packages/web":  {TagPrefix: "", SkipVersioning: false},
		"packages/core": {TagPrefix: "core-"},
	}
	if got := monorepoTagPrefixes(); len(got) != 2 || got["packages/api"] != "api-v" || got["packages/core"] != "core-" {
		t.Errorf("monorepoTagPrefixes() = %#v", got)
	}
	if got := monorepoSkipped(); len(got) != 1 || !got["packages/api"] {
		t.Errorf("monorepoSkipped() = %#v", got)
	}
	if got := refDisplay(""); got != "the start of history" {
		t.Errorf("refDisplay(empty) = %q", got)
	}
	if got := refDisplay("v1.0.0"); got != "v1.0.0" {
		t.Errorf("refDisplay(tag) = %q", got)
	}
	if got := plural(1); got != "" {
		t.Errorf("plural(1) = %q", got)
	}
	if got := plural(2); got != "s" {
		t.Errorf("plural(2) = %q", got)
	}
	root := t.TempDir()
	if got := displayPath(filepath.Join(root, "packages", "api"), root); got != filepath.Join("packages", "api") {
		t.Errorf("displayPath() = %q", got)
	}
	if got := displayPath("relative/path", root); got != "relative/path" {
		t.Errorf("displayPath() for relative path = %q", got)
	}
}

func TestEmitMonorepoBumpJSON(t *testing.T) {
	oldCfg, oldJSON := cfg, outputJSON
	cfg = config.DefaultConfig()
	cfg.Monorepo.Strategy = config.MonorepoStrategyIndependent
	outputJSON = true
	t.Cleanup(func() { cfg, outputJSON = oldCfg, oldJSON })

	root := t.TempDir()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })
	current, _ := version.Parse("1.2.0")
	next, _ := version.Parse("1.3.0")
	plan := &appmonorepo.BumpPlan{
		FromRef: "v1.2.0", ToRef: "HEAD", Discovered: 2,
		Packages: []appmonorepo.PackageBump{{
			Name: "api", Path: filepath.Join(root, "packages", "api"),
			Type: monorepo.PackageTypeGoModule, Current: current, Next: next,
			Bump: monorepo.BumpTypeMinor, Commits: 3, Tag: "api-v1.3.0", BaseRef: "api-v1.2.0",
		}},
	}
	if err := emitMonorepoBumpJSON(plan, root, []string{filepath.Join(root, "packages", "api", "go.mod")}); err != nil {
		t.Fatalf("emitMonorepoBumpJSON: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout
	var got struct {
		Strategy   string `json:"strategy"`
		Discovered int    `json:"packages_discovered"`
		Packages   []struct {
			Name    string `json:"name"`
			Path    string `json:"path"`
			Current string `json:"current_version"`
			Next    string `json:"next_version"`
			Tag     string `json:"tag"`
			BaseRef string `json:"base_ref"`
		} `json:"packages"`
		Written []string `json:"files_written"`
	}
	if err := json.NewDecoder(r).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if got.Strategy != "independent" || got.Discovered != 2 || len(got.Packages) != 1 {
		t.Fatalf("unexpected plan metadata: %+v", got)
	}
	pkg := got.Packages[0]
	if pkg.Name != "api" || pkg.Path != filepath.Join("packages", "api") || pkg.Current != "1.2.0" || pkg.Next != "1.3.0" || pkg.Tag != "api-v1.3.0" || pkg.BaseRef != "api-v1.2.0" {
		t.Errorf("unexpected package output: %+v", pkg)
	}
	if len(got.Written) != 1 || got.Written[0] != filepath.Join("packages", "api", "go.mod") {
		t.Errorf("files_written = %v", got.Written)
	}
}

func TestAdvanceRepositoryRunSkipsDryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = oldDryRun })
	if err := advanceRepositoryRun(context.Background(), commandTestApp{}); err != nil {
		t.Fatalf("dry run should not advance a release: %v", err)
	}
}

func TestPrintMonorepoBumpTextReportsDryRun(t *testing.T) {
	oldDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = oldDryRun })
	root := t.TempDir()
	current, _ := version.Parse("1.0.0")
	next, _ := version.Parse("1.0.1")
	var out bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	printMonorepoBumpText(&appmonorepo.BumpPlan{
		Discovered: 2,
		Packages:   []appmonorepo.PackageBump{{Name: "api", Path: filepath.Join(root, "api"), Current: current, Next: next, Tag: "api-v1.0.1", Bump: monorepo.BumpTypePatch, Commits: 1}},
	}, root, nil)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = out.ReadFrom(r)
	if !bytes.Contains(out.Bytes(), []byte("api-v1.0.1")) || !bytes.Contains(out.Bytes(), []byte("Dry run")) {
		t.Errorf("dry run output omitted package result: %s", out.String())
	}
}

func TestVersionFilePlanDoesNotWriteAndApplyDoes(t *testing.T) {
	oldCfg, oldJSON := cfg, outputJSON
	cfg = config.DefaultConfig()
	outputJSON = true
	t.Cleanup(func() { cfg, outputJSON = oldCfg, oldJSON })
	root := t.TempDir()
	manifest := filepath.Join(root, "package.json")
	if err := os.WriteFile(manifest, []byte("{\n  \"name\": \"demo\",\n  \"version\": \"1.0.0\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Versioning.VersionFiles = []config.VersionTarget{{Path: "package.json", Key: "version"}}
	app := commandTestApp{gitRepo: versionFileRootGitRepo{root: root}}
	next := version.MustParse("1.2.3")
	if err := reportVersionFilePlan(context.Background(), app, next); err != nil {
		t.Fatalf("reportVersionFilePlan: %v", err)
	}
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(before, []byte("1.0.0")) {
		t.Fatalf("dry run changed manifest: %s", before)
	}
	if err := applyVersionFiles(context.Background(), app, next); err != nil {
		t.Fatalf("applyVersionFiles: %v", err)
	}
	after, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte("1.2.3")) || bytes.Contains(after, []byte("1.0.0")) {
		t.Errorf("bump did not update package.json version: %s", after)
	}
}

func TestMonorepoBumpRejectsRepositoryWideOverrides(t *testing.T) {
	oldForce, oldLevel := bumpForce, bumpLevel
	t.Cleanup(func() { bumpForce, bumpLevel = oldForce, oldLevel })
	bumpForce, bumpLevel = "2.0.0", ""
	err := runMonorepoBump(context.Background(), commandTestApp{}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "single version") {
		t.Fatalf("--force error = %v", err)
	}
	bumpForce, bumpLevel = "", "major"
	err = runMonorepoBump(context.Background(), commandTestApp{}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "every package") {
		t.Fatalf("--level error = %v", err)
	}
}

func TestLastReleaseTagUsesRepositoryFallback(t *testing.T) {
	oldCfg := cfg
	cfg = config.DefaultConfig()
	cfg.Versioning.TagPrefix = "rel-"
	t.Cleanup(func() { cfg = oldCfg })
	if got := lastReleaseTag(context.Background(), commandTestApp{gitRepo: latestTagGitRepo{}}); got != "" {
		t.Errorf("missing release tag = %q, want empty fallback", got)
	}
	tag := sourcecontrol.NewTag("rel-1.4.0", "abc123")
	app := commandTestApp{gitRepo: latestTagGitRepo{tag: tag}}
	if got := lastReleaseTag(context.Background(), app); got != "rel-1.4.0" {
		t.Errorf("latest release tag = %q", got)
	}
	app = commandTestApp{gitRepo: latestTagGitRepo{err: os.ErrNotExist}}
	if got := lastReleaseTag(context.Background(), app); got != "" {
		t.Errorf("tag lookup error should fall back to empty ref, got %q", got)
	}
}

func TestConfiguredCurrentVersionHonorsManifestSource(t *testing.T) {
	oldCfg := cfg
	cfg = config.DefaultConfig()
	t.Cleanup(func() { cfg = oldCfg })
	app := commandTestApp{gitRepo: versionFileRootGitRepo{root: t.TempDir()}}
	if got, err := configuredCurrentVersion(context.Background(), app); err != nil || got != nil {
		t.Fatalf("default tag source = %v, %v; want nil, nil", got, err)
	}
	cfg.Versioning.BumpFrom = "file"
	if _, err := configuredCurrentVersion(context.Background(), app); err == nil || !strings.Contains(err.Error(), "no versioning.version_files") {
		t.Fatalf("file source without targets error = %v", err)
	}
	root := app.gitRepo.(versionFileRootGitRepo).root
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"version\":\"2.5.1\"}"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Versioning.BumpFrom = "package.json"
	got, err := configuredCurrentVersion(context.Background(), app)
	if err != nil {
		t.Fatalf("read package.json version: %v", err)
	}
	if got == nil || got.String() != "2.5.1" {
		t.Errorf("manifest version = %v, want 2.5.1", got)
	}
}

func TestVersionFileFailuresAreReportedBeforeChangingState(t *testing.T) {
	oldCfg, oldJSON := cfg, outputJSON
	cfg = config.DefaultConfig()
	outputJSON = true
	t.Cleanup(func() { cfg, outputJSON = oldCfg, oldJSON })
	app := commandTestApp{gitRepo: versionFileRootGitRepo{root: t.TempDir()}}
	next := version.MustParse("1.2.3")
	if err := applyVersionFiles(context.Background(), app, next); err != nil {
		t.Fatalf("no configured version files should be a no-op: %v", err)
	}
	if err := reportVersionFilePlan(context.Background(), app, next); err != nil {
		t.Fatalf("no configured version files should have no plan: %v", err)
	}
	cfg.Versioning.VersionFiles = []config.VersionTarget{{Path: "missing.json", Key: "version"}}
	if err := reportVersionFilePlan(context.Background(), app, next); err == nil || !strings.Contains(err.Error(), "failed to plan") {
		t.Errorf("missing planned file error = %v", err)
	}
	if err := applyVersionFiles(context.Background(), app, next); err == nil || !strings.Contains(err.Error(), "failed to update") {
		t.Errorf("missing applied file error = %v", err)
	}
}

func TestMonorepoAnnouncementOnlyAppearsWhenEnabled(t *testing.T) {
	oldCfg := cfg
	cfg = config.DefaultConfig()
	t.Cleanup(func() { cfg = oldCfg })
	out := captureImportReportOutput(t, func() {
		warnRepositoryWideInAMonorepo("approve")
	})
	if out != "" {
		t.Errorf("disabled monorepo emitted a warning: %q", out)
	}
	cfg.Monorepo.Enabled = true
	out = captureImportReportOutput(t, func() {
		warnRepositoryWideInAMonorepo("approve")
	})
	if !strings.Contains(out, "approve --package <name>") {
		t.Errorf("enabled monorepo guidance omitted per-package approval: %q", out)
	}
}
