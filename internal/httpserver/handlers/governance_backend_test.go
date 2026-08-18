package handlers

// governance_backend_test.go covers which governance store the dashboard endpoints open.
//
// The endpoints build a store per request, so "which backend" is decided per request too, and
// it has to be decided from the repository's own configuration. A server started in a
// subdirectory that read the working directory's configuration would answer the deployment
// gate out of a JSON file for a repository whose .relicta.yaml says otherwise — a governance
// decision made against evidence the operator does not think exists.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The configuration is read from the repository root, not from wherever the process happens to
// be standing. Run from a subdirectory, a server that read the working directory would find no
// .relicta.yaml and quietly resolve the file backend.
func TestTheGovernanceStoreIsSelectedByTheRepositorysOwnConfiguration(t *testing.T) {
	repo := gitRepoWithConfig(t, "persistence:\n  backend: sqlite\n")
	subdirectory := filepath.Join(repo, "cmd", "service")
	if err := os.MkdirAll(subdirectory, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Chdir(subdirectory)

	store, _, release, err := governanceStoreForRequest(
		httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil))
	defer release()

	if err != nil {
		t.Fatalf("governanceStoreForRequest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".relicta", "relicta.db")); err != nil {
		t.Errorf("no .relicta/relicta.db after resolving a store in a repository configured "+
			"for sqlite: %v — the handler read a configuration other than the repository's, "+
			"so it is serving governance evidence out of the wrong store", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".relicta", "governance", "memory.json")); err == nil {
		t.Error("memory.json was created for a repository configured for sqlite: the handler " +
			"resolved the file backend")
	}
	_ = store
}

// The refusal an operator meets when they configure a database and post a deployment.
//
// memory.DeploymentStore is segregated from memory.Store because not every implementation can
// hold a deployment, and neither database backend has a deployments table. Accepting the POST
// and dropping the record would put a hole in the evidence the DORA report and the deployment
// gate are computed from, with a 202 over it.
func TestRecordingADeploymentIsRefusedByNameOnABackendThatCannotHoldOne(t *testing.T) {
	repo := gitRepoWithConfig(t, "persistence:\n  backend: sqlite\n")
	t.Chdir(repo)

	_, _, release, err := deploymentStoreForRequest(
		httptest.NewRequest(http.MethodPost, "/api/v1/deployments", nil))
	defer release()

	if err == nil {
		t.Fatal("the sqlite backend accepted a deployment store: the endpoint would answer " +
			"202 and record nothing")
	}
	if !strings.Contains(err.Error(), "does not record deployments") {
		t.Errorf("the refusal is %q and does not say what is wrong", err)
	}
}

// The gate only reads release history, which every backend can answer. Narrowing it to
// memory.DeploymentStore would have refused a decision under `backend: sqlite` for want of a
// table it never touches, and a gate that cannot decide blocks every deployment.
func TestTheAuthorizationGateWorksOnABackendWithNoDeploymentsTable(t *testing.T) {
	repo := gitRepoWithConfig(t, "persistence:\n  backend: sqlite\n")
	t.Chdir(repo)

	store, repository, release, err := governanceStoreForRequest(
		httptest.NewRequest(http.MethodPost, "/api/v1/authorize", nil))
	defer release()

	if err != nil {
		t.Fatalf("governanceStoreForRequest under sqlite: %v — the deployment gate cannot "+
			"decide, so it refuses every deployment", err)
	}
	if _, err := store.GetReleaseHistory(httptest.NewRequest(
		http.MethodGet, "/", nil).Context(), repository, 10); err != nil {
		t.Errorf("the gate cannot read release history from the sqlite store: %v", err)
	}
}

// gitRepoWithConfig builds a one-commit repository with a .relicta.yaml at its root.
func gitRepoWithConfig(t *testing.T, configYAML string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		// macOS /tmp is a symlink to /private/tmp, and the repository root the git adapter
		// reports is the resolved one. Comparing paths written here against paths it
		// reports needs them to agree.
		dir = resolved
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	run("remote", "add", "origin", "https://github.com/owner/repo.git")
	if err := os.WriteFile(filepath.Join(dir, ".relicta.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "chore: initial commit")

	return dir
}
