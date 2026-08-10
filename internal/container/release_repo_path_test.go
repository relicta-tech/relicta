package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The release repository used to be constructed with the relative path
// ".relicta/releases", resolved against the process working directory rather than
// the repository root. Two visible consequences:
//
//   - `relicta cancel` from a subdirectory reported "No release run found" for a
//     repository that had a planned run — while printing the correct root in the
//     same message, because only the message resolved it
//   - NewFileReleaseRepository's MkdirAll created a stray .relicta/releases in
//     whatever subdirectory the command ran from, so merely invoking relicta
//     littered the working tree
//
// Both are the same defect: a store addressed by cwd inside a tool whose other
// paths are anchored to the repository. This test reads the construction rather
// than building a container, because building one needs a git repository and a
// working directory dance, while the property at issue is simply "not a relative
// path".

func TestReleaseRepositoryPathIsAnchoredToTheRepoRoot(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("container.go"))
	if err != nil {
		t.Fatalf("read container.go: %v", err)
	}
	s := string(source)

	// The root still has to come from git rather than the caller's cwd. The
	// mechanism changed when the two stores were consolidated — the bridge is
	// handed the repository root and the underlying store appends
	// .relicta/releases itself — but the property is the same one.
	if !strings.Contains(s, "c.gitService.GetRepositoryRoot(ctx)") {
		t.Error("the release store must be anchored to the repository root; without it, " +
			"running from a subdirectory addresses a different store and creates a " +
			"stray .relicta/releases there")
	}

	if !strings.Contains(s, "newReleaseRepoBridge(repoRoot)") {
		t.Error("expected the release store to be the bridge over the services " +
			"repository, constructed with the resolved root")
	}

	// The old construction addressed the store by a path relative to the process
	// working directory. Its absence is the fix.
	if strings.Contains(s, `persistence.NewFileReleaseRepository(`) {
		t.Error("container.go still constructs the second release store; the point of " +
			"the bridge is that there is one")
	}

	// Deliberately not counting occurrences of the literal ".relicta/releases":
	// the first version of this test did, and failed on correct code because the
	// explanatory comment above the fix contains that string. Counting text that
	// appears in prose is a test that breaks when someone documents the thing it
	// guards.
}
