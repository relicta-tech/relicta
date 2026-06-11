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

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
)

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
	return nil
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
	return nil
}
