// Package architecture houses fitness functions that lock in the
// architectural decisions Relicta has already made. Adding a forbidden
// import path makes a test fail at PR time — well before reviewers
// have to remember the convention.
//
// FF#1 (this file): hexagonal layering — internal/cli MUST NOT import
// internal/infrastructure/* directly except through a curated allowlist.
// The allowlist captures known existing debt; new infrastructure imports
// from CLI are blocked.
package architecture

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// allowedDirectInfrastructureImports lists the infrastructure packages the
// CLI is permitted to import directly today. Each entry represents either
// (a) infrastructure that's adapter-shaped and CLI legitimately drives, or
// (b) known debt to be refactored later.
//
// Adding a new entry requires reviewer approval — that's the whole point.
// Removing an entry tightens the architecture; preferred direction.
var allowedDirectInfrastructureImports = map[string]string{
	"github.com/relicta-tech/relicta/internal/infrastructure/ai":                   "AI provider abstraction is a leaf concern; CLI selects + injects providers directly",
	"github.com/relicta-tech/relicta/internal/infrastructure/git":                  "Git access is a leaf concern; CLI uses go-git directly via this adapter",
	"github.com/relicta-tech/relicta/internal/infrastructure/persistence/postgres": "DB migration CLI commands operate on the adapter directly",
}

// TestCLIDoesNotImportInfrastructureDirectly is FF#1 — the hexagonal
// boundary fitness function. It asserts that internal/cli only imports
// infrastructure packages that appear in the allowlist above.
//
// To add a new infrastructure dependency from CLI:
//  1. Decide whether it should route through internal/application instead.
//  2. If direct import is genuinely correct, add it to the allowlist with
//     a one-line justification — and expect a reviewer to push back.
func TestCLIDoesNotImportInfrastructureDirectly(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", `{{range .Imports}}{{.}}{{"\n"}}{{end}}`, "./internal/cli/...")
	cmd.Dir = repoRoot(t)

	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list failed (likely build-tag stub): %v", err)
		return
	}

	const prefix = "github.com/relicta-tech/relicta/internal/infrastructure/"
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, prefix) {
			continue
		}
		seen[line] = true
	}

	var unauthorized []string
	for imp := range seen {
		if _, ok := allowedDirectInfrastructureImports[imp]; !ok {
			unauthorized = append(unauthorized, imp)
		}
	}
	sort.Strings(unauthorized)

	if len(unauthorized) > 0 {
		t.Errorf(
			"internal/cli imports %d unallowed infrastructure package(s) directly:\n  %s\n"+
				"Either route through internal/application/* OR add an explicit allowlist entry "+
				"in internal/architecture/layering_test.go with justification.",
			len(unauthorized),
			strings.Join(unauthorized, "\n  "),
		)
	}
}

// TestAllowlistIsCurrent reports allowlist entries that no longer match a
// real CLI dependency — drift cleanup hint, not a hard failure.
func TestAllowlistIsCurrent(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", `{{range .Imports}}{{.}}{{"\n"}}{{end}}`, "./internal/cli/...")
	cmd.Dir = repoRoot(t)

	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list failed: %v", err)
		return
	}

	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			seen[line] = true
		}
	}

	for imp := range allowedDirectInfrastructureImports {
		if !seen[imp] {
			t.Logf("note: allowlist entry %q is no longer imported by internal/cli — consider removing", imp)
		}
	}
}

// repoRoot returns the absolute path to the repository root via `go list -m`.
func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list -m failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}
