package cli

// policy_validate.go: extracted from policy.go to reduce that file's size and
// make per-verb policy commands easier to navigate. Same package — no import
// or visibility changes; just file organization.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
)

// unresolvableFields returns every condition in a policy that names a field the
// evaluator cannot provide.
//
// Syntax was the only thing validated, so a policy could be entirely well-formed
// and entirely inert. The engine resolves an unknown field to "no value" and the
// rule reports itself as not matched, which is indistinguishable from a rule that
// was evaluated and did not apply — so a mistyped or renamed field produces a
// policy that loads, lists its rules as enabled, and governs nothing.
//
// This was not hypothetical: four of the five policies shipped in this repository
// were in that state, including one whose rule was named "freeze-period-block".
func unresolvableFields(source string, pol *policy.Policy) []fieldFinding {
	var findings []fieldFinding
	for _, rule := range pol.Rules {
		for _, field := range policy.UnknownFields(rule.Conditions) {
			findings = append(findings, fieldFinding{source: source, rule: rule.Name, field: field})
		}
	}
	return findings
}

// fieldFinding is one condition that can never match.
type fieldFinding struct {
	// source identifies the policy, by name rather than by a reconstructed path:
	// the loader reports a policy's name, not the file it came from, and printing
	// a guessed filename would send the reader to a path that may not exist.
	source string
	rule   string
	field  string
}

// reportUnresolvableFields prints the findings and converts them into an exit
// status when --strict is set.
//
// Findings are printed after the pass/fail line rather than interleaved with the
// per-directory loading, so the output does not report a problem above the line
// that says validation passed.
func reportUnresolvableFields(findings []fieldFinding) error {
	if len(findings) == 0 {
		return nil
	}

	fmt.Printf("\n%d condition(s) reference fields the evaluator does not provide:\n", len(findings))
	for _, f := range findings {
		fmt.Printf("\n  %s, rule %q: %s\n", f.source, f.rule, f.field)
		fmt.Print("    the evaluator does not provide this field, so the rule can never match.")
		if suggestion, ok := policy.SuggestFieldPath(f.field); ok {
			fmt.Printf("\n    Did you mean %q?", suggestion)
		}
		fmt.Println()
	}

	fmt.Println("\nList the fields it does provide with: relicta policy fields")

	if policyValidateStrict {
		return fmt.Errorf("%d condition(s) reference unknown fields", len(findings))
	}
	// A warning by default: a policy may legitimately be written ahead of a field
	// that a future release provides, and failing every such run would be worse
	// than reporting it. --strict is for CI, where an inert rule should stop the
	// pipeline.
	fmt.Println("Treat these as errors with: relicta policy validate --strict")
	return nil
}

// runPolicyValidate validates policy YAML files in configured directories.
// Reports per-file errors and aggregates them into a single exit status so
// CI runs catch any invalid policy without false-greens.
func runPolicyValidate(cmd *cobra.Command, args []string) error {
	// Validate a specific file
	if policyValidateFile != "" {
		return validatePolicyFile(policyValidateFile)
	}

	// Get directories to validate
	var dirs []string
	if policyValidateDir != "" {
		dirs = []string{policyValidateDir}
	} else {
		dirs = dsl.DefaultPolicyPaths()
	}

	// Track results
	var totalFiles int
	var validFiles int
	var invalidFiles int
	var fieldFindings []fieldFinding
	var allErrors []dsl.LoadError

	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}

		// Check if directory exists
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			continue
		}

		loadErrors, err := dsl.ValidateDir(absDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error validating directory %s: %v\n", dir, err)
			continue
		}

		// Count valid files by loading the directory
		loader := dsl.NewLoader(dsl.LoaderOptions{IgnoreErrors: true})
		result, _ := loader.LoadDir(absDir)

		if result != nil {
			totalFiles += len(result.Policies) + len(result.Errors)
			validFiles += len(result.Policies)
			invalidFiles += len(result.Errors)
			allErrors = append(allErrors, result.Errors...)

			for _, pol := range result.Policies {
				fieldFindings = append(fieldFindings,
					unresolvableFields(fmt.Sprintf("policy %q in %s", pol.Name, dir), pol)...)
			}
		}

		// Also add any validation errors not captured in LoadResult
		for _, loadErr := range loadErrors {
			found := false
			for _, existing := range allErrors {
				if existing.File == loadErr.File {
					found = true
					break
				}
			}
			if !found {
				allErrors = append(allErrors, loadErr)
				invalidFiles++
			}
		}
	}

	// Print results
	if totalFiles == 0 {
		fmt.Println("No policy files found.")
		fmt.Println("\nSearch paths:")
		for _, dir := range dirs {
			fmt.Printf("  - %s\n", dir)
		}
		return nil
	}

	// Print errors first
	if len(allErrors) > 0 {
		fmt.Println("Validation errors:")
		for _, loadErr := range allErrors {
			fmt.Printf("\n%s:\n", loadErr.File)
			// Format error message with indentation
			errLines := strings.Split(loadErr.Error.Error(), "\n")
			for _, line := range errLines {
				fmt.Printf("  %s\n", line)
			}
		}
		fmt.Println()
	}

	// Print summary
	if invalidFiles > 0 {
		fmt.Printf("Validation failed: %d/%d files have errors\n", invalidFiles, totalFiles)
		return fmt.Errorf("%d policy files have validation errors", invalidFiles)
	}

	fmt.Printf("Validation passed: %d files OK\n", validFiles)
	_ = cmd // silence unused-arg
	return reportUnresolvableFields(fieldFindings)
}

// validatePolicyFile validates a single policy file path.
// Returns an error with a stable user-facing message; details are printed
// to stdout so users see them in interactive sessions.
func validatePolicyFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", path)
	}

	err = dsl.ValidateFile(absPath)
	if err != nil {
		fmt.Printf("Validation failed for %s:\n", path)
		errLines := strings.Split(err.Error(), "\n")
		for _, line := range errLines {
			fmt.Printf("  %s\n", line)
		}
		return fmt.Errorf("policy validation failed")
	}

	fmt.Printf("Validation passed: %s\n", path)

	// Syntax is only half of it. Load the policy again to check that its
	// conditions name fields the evaluator can actually resolve.
	pol, loadErr := dsl.NewLoader(dsl.LoaderOptions{}).LoadFile(absPath)
	if loadErr != nil {
		// Unreachable in practice — ValidateFile just succeeded on this path — but
		// reporting nothing is better than reporting a field problem that is really
		// a read problem.
		return nil
	}
	return reportUnresolvableFields(unresolvableFields(path, pol))
}
