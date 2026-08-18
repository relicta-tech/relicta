package differential

// differential_test.go drives the built relicta binary through the same release lifecycle once
// per persistence backend and compares the transcripts.
//
// Runtime cost: roughly 10-15s for file and sqlite together, plus however long it takes Docker
// to start a postgres:16-alpine container (~10-20s warm, longer on a cold image pull). Three
// releases per backend, not twenty, because the fourth release exercises no code path the third
// did not and the harness has to stay cheap enough that people actually run it.

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// relictaBin is the binary under test, built once by TestMain.
var relictaBin string

func TestMain(m *testing.M) {
	// os.Exit skips deferred calls, so the build and the cleanup live in a helper that
	// returns an exit code instead.
	os.Exit(buildAndRun(m))
}

func buildAndRun(m *testing.M) int {
	// Build once. Every test execs this binary; building per test would dominate the runtime
	// and, worse, would let two tests disagree about what "the binary" means.
	dir, err := os.MkdirTemp("", "relicta-differential-build")
	if err != nil {
		fmt.Fprintf(os.Stderr, "differential: temp dir: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()

	relictaBin = filepath.Join(dir, "relicta")
	build := exec.Command("go", "build", "-o", relictaBin, "./cmd/relicta")
	build.Dir = moduleRoot()
	// GOWORK=off: a go.work above the checkout pulls sibling modules into the build and can
	// resolve this module's own dependencies differently from a clean CI build. The binary
	// under test has to be the one CI would produce.
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		fmt.Fprintf(os.Stderr, "differential: build relicta: %v\n%s\n", buildErr, out)
		return 1
	}

	return m.Run()
}

// moduleRoot returns the repository root, two levels up from test/differential.
func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "..", "..")
}

// sharedConfig is every configuration setting the backends have in common. The per-backend
// persistence block is appended to it, so a difference in the transcripts cannot be blamed on
// a difference in configuration.
//
// auto_commit_changelog is off deliberately. relicta's own changelog commit takes its timestamp
// from the wall clock, and a commit whose timestamp moves has a hash that moves, which would
// make every descendant commit hash differ between backends for reasons that have nothing to do
// with persistence. Turning it off — and gitignoring the changelog, config and event store —
// keeps every commit in the fixture byte-identical across backends, so the hashes stay real
// evidence instead of noise the normalizer has to launder away.
const sharedConfig = `ai:
  provider: none
  enabled: false
output:
  color: false
  format: text
  quiet: false
workflow:
  require_approval: false
  auto_commit_changelog: false
  require_clean_working_tree: true
  allowed_branches:
    - main
versioning:
  strategy: conventional
  tag_prefix: v
  git_tag: true
  git_push: false
changelog:
  file: CHANGELOG.md
  include_commit_hash: true
  include_date: true
`

// backend names one configuration under test.
type backend struct {
	name string
	// persistence is the YAML block appended to sharedConfig — the only thing that varies.
	persistence string
	// dsn, when set, is exported to the binary and normalized out of the transcript.
	dsn string
}

// fixtureCommit is one commit in the fixture repository. Dates are fixed so the hashes are
// identical in every backend's repository; a differing hash is then a finding, not noise.
type fixtureCommit struct {
	date    string
	file    string
	content string
	message string
}

var (
	// The two commits that exist before the first release.
	initialCommits = []fixtureCommit{
		{"2026-01-01T00:00:00+00:00", "go.mod", "module example.com/fixture\n\ngo 1.25\n", "chore: initial commit"},
		{"2026-01-02T00:00:00+00:00", "alpha.go", "package fixture\n\nfunc Alpha() string { return \"alpha\" }\n", "feat: add alpha"},
	}
	// One commit before each subsequent release, chosen so the bump kind differs each time:
	// a fix takes a patch, a feature takes a minor.
	followUpCommits = []fixtureCommit{
		{"2026-01-03T00:00:00+00:00", "beta.go", "package fixture\n\nfunc Beta() string { return \"beta\" }\n", "fix: correct beta handling"},
		{"2026-01-04T00:00:00+00:00", "gamma.go", "package fixture\n\nfunc Gamma() string { return \"gamma\" }\n", "feat: add gamma endpoint"},
	}
)

// TestEveryBackendTellsTheSameStoryAsTheFileBackend is the harness proper.
//
// The file backend is the reference for the reason both conformance packages give: it is what
// every caller in the tree was written against. A backend that disagrees is wrong even where
// its answer would be more defensible in isolation.
func TestEveryBackendTellsTheSameStoryAsTheFileBackend(t *testing.T) {
	reference := runBackend(t, backend{
		name:        "file",
		persistence: "persistence:\n  backend: file\n  file_path: .relicta/events\n",
	})

	t.Run("sqlite matches file", func(t *testing.T) {
		got := runBackend(t, backend{
			name: "sqlite",
			// migration_mode is auto because relicta creates the SQLite file itself in
			// .relicta/ and `relicta db migrate` does not speak SQLite — honoring "manual"
			// there would leave no way to migrate at all.
			persistence: "persistence:\n  backend: sqlite\n  migration_mode: auto\n",
		})
		requireSameTranscript(t, "file", reference, "sqlite", got)
	})

	t.Run("postgres matches file", func(t *testing.T) {
		dsn := startPostgres(t)
		got := runBackend(t, backend{
			name: "postgres",
			// migration_mode: auto so the container's empty database gets the schema without
			// a separate `relicta db migrate` step. A real operator provisioning their own
			// PostgreSQL would use manual and migrate deliberately; here the database is
			// created and destroyed by the test, so there is no operator to ask.
			//
			// The DSN arrives through the environment rather than being written into the
			// file: the container's port is assigned at startup, and a connection string
			// with credentials has no business being in a fixture on disk.
			persistence: "persistence:\n  backend: postgres\n  migration_mode: auto\n  pool_size: 5\n  connection_string: ${DIFFERENTIAL_PG_DSN}\n",
			dsn:         dsn,
		})
		requireSameTranscript(t, "file", reference, "postgres", got)
	})
}

// TestTheNormalizerLeavesNothingVaryingBetweenTwoRunsOfOneBackend is the control.
//
// It runs the *file* backend twice in two different directories. Anything that differs is by
// construction run-to-run variance rather than backend behavior, so if this passes, a failure
// in the test above is attributable to the backend and not to a normalization gap. Without it,
// a missing rule would look exactly like a real divergence.
func TestTheNormalizerLeavesNothingVaryingBetweenTwoRunsOfOneBackend(t *testing.T) {
	spec := backend{name: "file", persistence: "persistence:\n  backend: file\n  file_path: .relicta/events\n"}
	first := runBackend(t, spec)
	second := runBackend(t, spec)
	requireSameTranscript(t, "file run 1", first, "file run 2", second)
}

func requireSameTranscript(t *testing.T, refName, ref, gotName, got string) {
	t.Helper()
	if d := diffTranscripts(refName, ref, gotName, got); d != "" {
		t.Errorf("the %s backend does not behave like the %s backend:\n\n%s", gotName, refName, d)
	}
}

// runBackend builds a fresh repository, runs the lifecycle, and returns the normalized
// transcript.
func runBackend(t *testing.T, b backend) string {
	t.Helper()

	// A leaf directory named "repo" in every backend's own parent. The name is part of the
	// repository identity relicta prints ("local:repo"), so keeping it identical makes that
	// line real evidence rather than something the normalizer has to erase.
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}

	// A HOME of its own, so a developer's ~/.relicta or ~/.gitconfig cannot change the answer.
	home := filepath.Join(parent, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("create home dir: %v", err)
	}

	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		// UTC everywhere so the timestamp *format* is identical and only the value varies;
		// a machine in a half-hour offset zone would otherwise render offsets differently.
		"TZ=UTC",
		"NO_COLOR=1",
		// Isolate git from the developer's global and system config, which may enable commit
		// signing or set a different default branch.
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	}
	if b.dsn != "" {
		env = append(env, "DIFFERENTIAL_PG_DSN="+b.dsn)
	}

	initRepo(t, repo, env)

	if err := os.WriteFile(filepath.Join(repo, ".relicta.yaml"), []byte(sharedConfig+b.persistence), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var tr transcript

	// Three releases. The first covers "no previous release"; the later two cover the path
	// where a baseline tag already exists, with a different bump kind each time.
	release(t, &tr, repo, env)
	for _, c := range followUpCommits {
		commitFixture(t, repo, env, c)
		release(t, &tr, repo, env)
	}

	// The readers. These are the commands the ports conformance suites cannot reach: each one
	// assembles its answer from a store through a different query.
	for _, args := range [][]string{
		{"status"},
		{"history"},
		// JSON exposes fields the human rendering rounds off or omits, so a backend that
		// returned a subtly different record shows up here even when the table looks the same.
		{"history", "--json"},
		// --period is required, and a fixed range keeps it out of the normalizer's way.
		{"report", "--type", "summary", "--period", "2020-01-01:2099-12-31"},
		{"report", "--type", "dora", "--period", "2020-01-01:2099-12-31"},
		{"audit"},
		{"clean", "--dry-run"},
	} {
		tr.exec(t, repo, env, relictaBin, args...)
	}

	// Observable side effects. A backend could produce identical console output while having
	// tagged something different or written a different changelog.
	tr.exec(t, repo, env, "git", "tag", "--sort=refname")
	tr.exec(t, repo, env, "git", "log", "--oneline")
	tr.file(t, repo, "CHANGELOG.md")

	// Register both the raw temp path and its symlink-resolved form: on macOS t.TempDir()
	// hands back /var/folders/... while a resolved path reads /private/var/folders/...,
	// and relicta prints whichever one it happened to compute.
	paths := []string{repo, parent}
	if resolved, err := filepath.EvalSymlinks(parent); err == nil && resolved != parent {
		paths = append(paths, filepath.Join(resolved, "repo"), resolved)
	}

	return newNormalizer(paths, b.dsn).normalize(tr.render())
}

// release runs one full lifecycle.
func release(t *testing.T, tr *transcript, repo string, env []string) {
	t.Helper()
	for _, args := range [][]string{
		{"plan"}, {"bump"}, {"notes"}, {"approve", "-y"}, {"publish"},
	} {
		tr.exec(t, repo, env, relictaBin, args...)
	}
}

// initRepo creates the fixture repository.
//
// The config, the event store and the changelog are gitignored. That is what makes every commit
// hash identical across backends: nothing a backend writes ever becomes commit content.
func initRepo(t *testing.T, repo string, env []string) {
	t.Helper()

	mustGit(t, repo, env, "init", "-q", "-b", "main")
	mustGit(t, repo, env, "config", "user.name", "Differential Harness")
	mustGit(t, repo, env, "config", "user.email", "harness@relicta.test")
	// Signing would make the hashes depend on a key that is not in the fixture.
	mustGit(t, repo, env, "config", "commit.gpgsign", "false")
	mustGit(t, repo, env, "config", "tag.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(repo, ".gitignore"),
		[]byte(".relicta.yaml\n.relicta/\nCHANGELOG.md\n"), 0o600); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}

	for _, c := range initialCommits {
		commitFixture(t, repo, env, c)
	}
}

func commitFixture(t *testing.T, repo string, env []string, c fixtureCommit) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, c.file), []byte(c.content), 0o600); err != nil {
		t.Fatalf("write %s: %v", c.file, err)
	}
	mustGit(t, repo, env, "add", "-A")
	// Both dates pinned: the author date alone leaves the committer date on the wall clock,
	// and the hash covers both.
	dated := append(append([]string(nil), env...),
		"GIT_AUTHOR_DATE="+c.date, "GIT_COMMITTER_DATE="+c.date)
	mustGit(t, repo, dated, "commit", "-q", "-m", c.message)
}

func mustGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// transcript accumulates every command, its exit code and its combined output.
type transcript struct {
	b strings.Builder
}

// exec runs one command and records it. Failures are recorded rather than fatal: a command that
// exits non-zero on one backend and zero on another is precisely the kind of difference this
// harness exists to surface, so the exit code is part of the transcript.
func (tr *transcript) exec(t *testing.T, dir string, env []string, bin string, args ...string) {
	t.Helper()

	name := filepath.Base(bin)
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()

	exit := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s %s: %v", name, strings.Join(args, " "), err)
	}

	fmt.Fprintf(&tr.b, "$ %s %s\n[exit %d]\n%s\n=====\n", name, strings.Join(args, " "), exit, out)
}

// file records a file's contents as a transcript entry, for side effects that never reach
// stdout.
func (tr *transcript) file(t *testing.T, dir, name string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		// A missing file is itself a difference worth comparing, so record it.
		fmt.Fprintf(&tr.b, "# file %s\n[error %v]\n=====\n", name, err)
		return
	}
	fmt.Fprintf(&tr.b, "# file %s\n%s\n=====\n", name, content)
}

func (tr *transcript) render() string { return tr.b.String() }

// startPostgres brings up a one-shot PostgreSQL container and returns its DSN.
//
// Follows the pattern the postgres adapter's own testcontainer suite uses: skip loudly when
// Docker is genuinely unavailable or in short mode, and fail on anything else. CI must not go
// red for want of a Docker daemon, and it must not go green by quietly skipping a container
// that failed for a real reason either.
func startPostgres(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping the postgres differential run in short mode (-short)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("relicta_differential"),
		tcpostgres.WithUsername("relicta"),
		tcpostgres.WithPassword("relicta"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		cancel()
		if isDockerUnavailable(err) {
			t.Skipf("docker unavailable, skipping the postgres differential run: %v", err)
		}
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
		cancel()
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	// Assembled with net/url rather than formatted into a string, so no credential-bearing
	// DSN literal exists anywhere in this package for a secret scanner to find.
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword("relicta", "relicta"),
		Host:     host + ":" + port.Port(),
		Path:     "/relicta_differential",
		RawQuery: "sslmode=disable",
	}
	return dsn.String()
}

// isDockerUnavailable distinguishes "there is no Docker here" from "Docker is here and
// something went wrong", so only the first becomes a skip.
func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{
		"Cannot connect to the Docker daemon",
		"docker daemon is not running",
		"connection refused",
		"no such file or directory",
		"Cannot find docker",
		"docker not found",
		"rootless Docker not found",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
