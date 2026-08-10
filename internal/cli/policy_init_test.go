package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
	"github.com/relicta-tech/relicta/v4/internal/cli/templates"
)

// Governance is enabled by default, and a policy is how a team encodes rules the
// built-in defaults do not cover. There was no command that produced a first
// policy: `policy scaffold` generates test fixtures for policies that already
// exist and exits 1 in a repository with none, `policy list` named the
// directories and extensions but not the grammar, and the documented route was
// `cp examples/policies/starter.policy` — a path that exists in a git checkout
// and not in an installed binary.

// setupPolicyInit isolates the flags and the working directory, so a test writes
// into a temporary repository rather than the one relicta is developed in.
func setupPolicyInit(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	tmpl, d, force, list := policyInitTemplate, policyInitDir, policyInitForce, policyInitList
	t.Cleanup(func() {
		policyInitTemplate, policyInitDir, policyInitForce, policyInitList = tmpl, d, force, list
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})

	policyInitTemplate = templates.DefaultPolicyStarter
	policyInitDir = ""
	policyInitForce = false
	policyInitList = false

	return dir
}

// The file has to land where the loader looks, or `policy init` produces a policy
// that governs nothing and reports success.
func TestPolicyInitWritesIntoASearchedDirectory(t *testing.T) {
	dir := setupPolicyInit(t)

	if err := runPolicyInit(policyInitCmd, nil); err != nil {
		t.Fatalf("policy init: %v", err)
	}

	searched := dsl.DefaultPolicyPaths()
	if len(searched) == 0 {
		t.Fatal("no policy search paths; the command has nowhere correct to write")
	}
	target := filepath.Join(dir, searched[0], "starter.policy")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected a policy at %s (the first searched path): %v", target, err)
	}

	// And it has to be loadable from there — a file in the right place that does
	// not parse is the same failure one step later.
	result, err := dsl.NewLoader(dsl.LoaderOptions{Recursive: true}).LoadDir(filepath.Join(dir, searched[0]))
	if err != nil {
		t.Fatalf("the written policy does not load: %v", err)
	}
	if len(result.Policies) != 1 {
		t.Fatalf("expected the loader to find 1 policy, found %d", len(result.Policies))
	}
	if len(result.Policies[0].Rules) == 0 {
		t.Error("the written policy has no rules, so it changes no decision")
	}
}

// Overwriting without being asked would discard rules someone wrote — the one
// thing a governance tool must not do to its own governance.
func TestPolicyInitRefusesToOverwriteWithoutForce(t *testing.T) {
	dir := setupPolicyInit(t)

	if err := runPolicyInit(policyInitCmd, nil); err != nil {
		t.Fatalf("first init: %v", err)
	}

	target := filepath.Join(dir, dsl.DefaultPolicyPaths()[0], "starter.policy")
	const edited = "# a rule someone wrote\n"
	if err := os.WriteFile(target, []byte(edited), 0o600); err != nil {
		t.Fatalf("write edited policy: %v", err)
	}

	err := runPolicyInit(policyInitCmd, nil)
	if err == nil {
		t.Fatal("a second init must refuse rather than overwrite")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("the refusal should name the way through; got %v", err)
	}

	after, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if string(after) != edited {
		t.Error("the refusal must leave the existing policy untouched")
	}

	// With --force it is the caller's decision.
	policyInitForce = true
	if err := runPolicyInit(policyInitCmd, nil); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	forced, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read back after force: %v", err)
	}
	if string(forced) == edited {
		t.Error("--force should have replaced the file")
	}
}

func TestPolicyInitHonoursDir(t *testing.T) {
	dir := setupPolicyInit(t)
	policyInitDir = "custom/policies"

	if err := runPolicyInit(policyInitCmd, nil); err != nil {
		t.Fatalf("policy init --dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "custom/policies/starter.policy")); err != nil {
		t.Errorf("expected the policy in the requested directory: %v", err)
	}
}

func TestPolicyInitWritesTheRequestedTemplate(t *testing.T) {
	dir := setupPolicyInit(t)
	policyInitTemplate = "enterprise"

	if err := runPolicyInit(policyInitCmd, nil); err != nil {
		t.Fatalf("policy init --template enterprise: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, dsl.DefaultPolicyPaths()[0], "enterprise.policy")); err != nil {
		t.Errorf("expected enterprise.policy: %v", err)
	}
}

// An unknown template must list the valid ones; otherwise the user is left
// guessing at the exact set the binary happens to carry.
func TestPolicyInitUnknownTemplateNamesTheAlternatives(t *testing.T) {
	setupPolicyInit(t)
	policyInitTemplate = "does-not-exist"

	err := runPolicyInit(policyInitCmd, nil)
	if err == nil {
		t.Fatal("expected an error for an unknown template")
	}
	if !strings.Contains(err.Error(), templates.DefaultPolicyStarter) {
		t.Errorf("the error should list the available templates; got %v", err)
	}
}

// --list must not write anything: it is the command someone runs to decide.
func TestPolicyInitListWritesNothing(t *testing.T) {
	dir := setupPolicyInit(t)
	policyInitList = true

	if err := runPolicyInit(policyInitCmd, nil); err != nil {
		t.Fatalf("policy init --list: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, dsl.DefaultPolicyPaths()[0])); !os.IsNotExist(err) {
		t.Error("--list created a policy directory; it should only print")
	}
}

// The subcommand has to be reachable from `relicta policy`. A correct command
// that nothing registers is the failure mode this codebase keeps producing.
func TestPolicyInitIsRegistered(t *testing.T) {
	for _, c := range policyCmd.Commands() {
		if c.Name() == "init" {
			return
		}
	}
	t.Error("`relicta policy init` is not registered under `relicta policy`")
}
