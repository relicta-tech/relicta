package monorepo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	infraworkspace "github.com/relicta-tech/relicta/v4/internal/infrastructure/workspace"

	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// The monorepo subsystem was implemented, tested, and reachable from nothing: every field of
// the `monorepo:` section had zero production readers, so a repository with two packages at
// 1.0.0 and 2.0.0 was told its next version was 0.1.0 and neither package.json was touched.
//
// The real FileDetector, not a fake: what has to hold is that a glob in `package_paths` finds
// the packages on disk and reads their manifests. A stub detector handed a list of packages
// would assert the stub. Test-only — the application layer's production code still depends on
// the workspace.Detector port alone.
//
// These tests cover the half that closes first — a package's own commits earning it its own
// version, written to its own manifest.

// npmPackage writes a package.json, the layout `package_paths: ["packages/*"]` describes.
func npmPackage(t *testing.T, root, name, ver string) string {
	t.Helper()
	dir := filepath.Join(root, "packages", name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	body, err := json.Marshal(map[string]string{"name": name, "version": ver})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), body, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return dir
}

// commitTouching returns a commit and the diff stats that place it inside one package.
func commitTouching(hash, message, file string) (*sourcecontrol.Commit, *sourcecontrol.DiffStats) {
	return newTestCommit(hash, message), &sourcecontrol.DiffStats{
		FilesChanged: 1,
		Files:        []sourcecontrol.FileStats{{Path: file}},
	}
}

func bumpServiceFor(commits []*sourcecontrol.Commit, stats map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats) *BumpService {
	git := &mockGitRepository{commits: commits, diffStats: stats}
	analyzer := NewMonorepoAnalyzer(git, &version.DefaultVersionCalculator{}, nil, NewCompositeVersionWriter())
	return NewBumpService(infraworkspace.NewFileDetector(), analyzer, git)
}

func TestOnlyThePackageAKommitTouchedIsBumped(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.4.0")
	npmPackage(t, root, "web", "2.1.3")

	commit, stats := commitTouching("aaa1", "feat: add an endpoint", "packages/api/index.js")
	svc := bumpServiceFor([]*sourcecontrol.Commit{commit},
		map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{commit.Hash(): stats})

	plan, err := svc.Plan(context.Background(), PlanInput{
		RepoRoot:     root,
		PackagePaths: []string{"packages/*"},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if plan.Discovered != 2 {
		t.Errorf("discovered %d packages, want 2: the globs did not match the layout",
			plan.Discovered)
	}
	if len(plan.Packages) != 1 {
		t.Fatalf("planned %d bumps, want 1 — only api was touched: %+v", len(plan.Packages), plan.Packages)
	}

	got := plan.Packages[0]
	if got.Name != "api" {
		t.Errorf("bumped %q, want api", got.Name)
	}
	if got.Current.String() != "1.4.0" {
		t.Errorf("current = %s, want 1.4.0: the package's own manifest was not read",
			got.Current.String())
	}
	if got.Next.String() != "1.5.0" {
		t.Errorf("next = %s, want 1.5.0 for a feat on 1.4.0", got.Next.String())
	}
	if got.Type != monorepo.PackageTypeNPM {
		t.Errorf("type = %s, want npm", got.Type)
	}
}

func TestApplyWritesEachPackagesOwnManifestAndLeavesTheRest(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.4.0")
	npmPackage(t, root, "web", "2.1.3")

	commit, stats := commitTouching("bbb1", "fix: correct a header", "packages/api/index.js")
	svc := bumpServiceFor([]*sourcecontrol.Commit{commit},
		map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{commit.Hash(): stats})

	plan, err := svc.Plan(context.Background(), PlanInput{RepoRoot: root, PackagePaths: []string{"packages/*"}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	written, err := svc.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if versionIn(t, root, "api") != "1.4.1" {
		t.Errorf("api is at %s, want 1.4.1: the repository-wide version was written, or none was",
			versionIn(t, root, "api"))
	}
	if versionIn(t, root, "web") != "2.1.3" {
		t.Errorf("web is at %s, want 2.1.3 untouched: a package no commit touched was versioned",
			versionIn(t, root, "web"))
	}
	if len(written) != 1 || filepath.Base(written[0]) != "package.json" {
		t.Errorf("wrote %v, want exactly api's package.json", written)
	}
}

// Planning writes nothing. --dry-run reports the same numbers as the run that applies them,
// so the two must come from one call that only computes.
func TestPlanningWritesNothing(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.4.0")

	commit, stats := commitTouching("ccc1", "feat!: replace the endpoint", "packages/api/index.js")
	svc := bumpServiceFor([]*sourcecontrol.Commit{commit},
		map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{commit.Hash(): stats})

	plan, err := svc.Plan(context.Background(), PlanInput{RepoRoot: root, PackagePaths: []string{"packages/*"}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Packages[0].Next.String() != "2.0.0" {
		t.Errorf("next = %s, want 2.0.0 for a breaking change on 1.4.0", plan.Packages[0].Next.String())
	}
	if versionIn(t, root, "api") != "1.4.0" {
		t.Errorf("api is at %s after planning alone, want 1.4.0 — Plan wrote to the manifest",
			versionIn(t, root, "api"))
	}
}

// A package is versioned from the manifest in its own directory, not from a manager declared
// once for the whole workspace: `packages/*` may hold an npm package beside a Go module.
func TestEachPackageIsTypedByItsOwnManifest(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.4.0")

	goDir := filepath.Join(root, "packages", "svc")
	if err := os.MkdirAll(goDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module example.com/svc\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if got := DetectPackageType(goDir); got != monorepo.PackageTypeGoModule {
		t.Errorf("DetectPackageType(go.mod dir) = %s, want go_module", got)
	}
	if got := DetectPackageType(filepath.Join(root, "packages", "api")); got != monorepo.PackageTypeNPM {
		t.Errorf("DetectPackageType(package.json dir) = %s, want npm", got)
	}
	if got := DetectPackageType(root); got != monorepo.PackageTypeDirectory {
		t.Errorf("DetectPackageType(no manifest) = %s, want directory", got)
	}
}

// The two failures a monorepo user actually hits have to be told apart: globs that match
// nothing, and no globs at all.
func TestPlanRefusesAnEmptyOrUnmatchedGlobDistinctly(t *testing.T) {
	svc := bumpServiceFor(nil, nil)

	_, err := svc.Plan(context.Background(), PlanInput{RepoRoot: t.TempDir()})
	if err == nil {
		t.Fatal("planning with no package_paths succeeded")
	}
	if got := err.Error(); !contains(got, "package_paths") {
		t.Errorf("error = %q, want it to name monorepo.package_paths", got)
	}

	_, err = svc.Plan(context.Background(), PlanInput{RepoRoot: t.TempDir(), PackagePaths: []string{"packages/*"}})
	if err == nil {
		t.Fatal("planning with globs that match nothing succeeded")
	}
	if got := err.Error(); !contains(got, "no packages matched") {
		t.Errorf("error = %q, want it to say the globs matched nothing", got)
	}
}

// Bumping twice without releasing must report the same version twice. The analyzer reads the
// manifest on disk — the file the previous bump wrote — so a package went 1.4.0 -> 1.5.0 and
// then 1.5.0 -> 1.6.0 off one commit. The base version has to come from the last release, which
// is what the repository-wide path gets from its tag.
func TestBumpingTwiceOffOneCommitReportsTheSameVersion(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.4.0")

	commit, stats := commitTouching("ddd1", "feat: add an endpoint", "packages/api/index.js")
	git := &mockGitRepository{
		commits:   []*sourcecontrol.Commit{commit},
		diffStats: map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{commit.Hash(): stats},
		filesAtRef: map[string][]byte{
			filepath.Join("packages", "api", "package.json"): []byte(`{"name":"api","version":"1.4.0"}`),
		},
	}
	analyzer := NewMonorepoAnalyzer(git, &version.DefaultVersionCalculator{}, nil, NewCompositeVersionWriter())
	svc := NewBumpService(infraworkspace.NewFileDetector(), analyzer, git)

	input := PlanInput{RepoRoot: root, PackagePaths: []string{"packages/*"}, FromRef: "v0.1.0"}

	first, err := svc.Plan(context.Background(), input)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := svc.Apply(context.Background(), first); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	second, err := svc.Plan(context.Background(), input)
	if err != nil {
		t.Fatalf("Plan (second): %v", err)
	}

	if got := second.Packages[0].Next.String(); got != first.Packages[0].Next.String() {
		t.Errorf("the second run says %s, the first said %s.\nRunning bump twice off one commit "+
			"inflates the version, because the base came from the manifest the first run wrote",
			got, first.Packages[0].Next.String())
	}
	if got := second.Packages[0].Current.String(); got != "1.4.0" {
		t.Errorf("current = %s on the second run, want the released 1.4.0", got)
	}
}

func versionIn(t *testing.T, root, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "packages", name, "package.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return manifest.Version
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Each package is measured from its own last release, not from the repository's.
//
// Before per-package tags existed, every package counted from the repository tag, so a package
// released last week and a package released last year were both told "everything since v0.1.0".
// The base has to be the package's own tag where it has one.
func TestAPackageIsMeasuredFromItsOwnLastTag(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.4.0")
	npmPackage(t, root, "web", "2.1.3")

	// Two commits: one before api's tag, one after. Only the later one is api's to count.
	old, oldStats := commitTouching("old1", "feat: earlier api work", "packages/api/old.js")
	recent, recentStats := commitTouching("new1", "fix: later api work", "packages/api/new.js")

	git := &mockGitRepository{
		commits: []*sourcecontrol.Commit{recent},
		diffStats: map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
			old.Hash():    oldStats,
			recent.Hash(): recentStats,
		},
		filesAtRef: map[string][]byte{
			filepath.Join("packages", "api", "package.json"): []byte(`{"name":"api","version":"1.4.0"}`),
		},
		tags: sourcecontrol.TagList{
			sourcecontrol.NewTag("api-v1.4.0", "aaaa"),
			sourcecontrol.NewTag("v0.1.0", "bbbb"),
		},
	}
	analyzer := NewMonorepoAnalyzer(git, &version.DefaultVersionCalculator{}, nil, NewCompositeVersionWriter())
	svc := NewBumpService(infraworkspace.NewFileDetector(), analyzer, git)

	plan, err := svc.Plan(context.Background(), PlanInput{
		RepoRoot:     root,
		PackagePaths: []string{"packages/*"},
		FromRef:      "v0.1.0",
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var api *PackageBump
	for i := range plan.Packages {
		if plan.Packages[i].Name == "api" {
			api = &plan.Packages[i]
		}
	}
	if api == nil {
		t.Fatalf("api was not planned: %+v", plan.Packages)
	}

	if api.BaseRef != "api-v1.4.0" {
		t.Errorf("api was measured from %q, want its own tag api-v1.4.0", api.BaseRef)
	}
	if api.Tag != "api-v1.4.1" {
		t.Errorf("api's release tag is %q, want api-v1.4.1", api.Tag)
	}
}

// The highest version wins, not the most recent tag: a patch on an older line is tagged after
// a newer minor, and taking the newest would measure the next release from the wrong place.
func TestTheHighestTagWinsNotTheLatest(t *testing.T) {
	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("api-v1.4.0", "aaaa"),
		sourcecontrol.NewTag("api-v2.0.0", "bbbb"),
		sourcecontrol.NewTag("api-v1.4.1", "cccc"), // tagged last, older line
	}

	got, ok := latestTagWithPrefix(tags, "api-v")
	if !ok {
		t.Fatal("no tag was found for the api- prefix")
	}
	if got != "api-v2.0.0" {
		t.Errorf("base tag = %q, want api-v2.0.0", got)
	}
}

// publish tags what the manifests claim, because bump has already written them and somebody
// may have edited one by hand.
func TestReleaseTagsComeFromTheManifests(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.5.0")
	npmPackage(t, root, "web", "2.1.4")

	svc := bumpServiceFor(nil, nil)
	tags, err := svc.ReleaseTags(context.Background(), PlanInput{
		RepoRoot:     root,
		PackagePaths: []string{"packages/*"},
		TagPrefixes:  map[string]string{filepath.Join("packages", "web"): "webapp-v"},
	})
	if err != nil {
		t.Fatalf("ReleaseTags: %v", err)
	}

	if len(tags) != 2 {
		t.Fatalf("got %d tags, want one per package: %+v", len(tags), tags)
	}
	if tags[0].Tag != "api-v1.5.0" {
		t.Errorf("tags[0] = %q, want api-v1.5.0", tags[0].Tag)
	}
	if tags[1].Tag != "webapp-v2.1.4" {
		t.Errorf("tags[1] = %q, want webapp-v2.1.4 from the configured prefix", tags[1].Tag)
	}
}

// The release commit has to cover the manifests bump wrote, or the tag points at a commit that
// does not contain the version it claims — and the clean-tree gate refuses the publish.
func TestManifestPathsAreRepositoryRelative(t *testing.T) {
	root := t.TempDir()
	npmPackage(t, root, "api", "1.5.0")

	svc := bumpServiceFor(nil, nil)
	paths, err := svc.ManifestPaths(context.Background(), PlanInput{
		RepoRoot:     root,
		PackagePaths: []string{"packages/*"},
	})
	if err != nil {
		t.Fatalf("ManifestPaths: %v", err)
	}

	want := filepath.Join("packages", "api", "package.json")
	if len(paths) != 1 || paths[0] != want {
		t.Errorf("paths = %v, want [%s]: git takes a pathspec relative to the repository",
			paths, want)
	}
}
