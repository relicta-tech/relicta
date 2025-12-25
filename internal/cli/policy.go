package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/cgp/policy/dsl"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage governance policies",
	Long: `Manage CGP (Change Governance Protocol) policies.

Policies define rules for evaluating release changes, determining
risk levels, and requiring approvals based on configurable conditions.

Examples:
  # Validate all policies in the default directory
  relicta policy validate

  # Validate policies in a specific directory
  relicta policy validate --dir .relicta/policies

  # Validate a specific policy file
  relicta policy validate --file security.policy

  # List all loaded policies
  relicta policy list`,
}

var policyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate policy files",
	Long: `Validate policy DSL files for syntax and semantic correctness.

By default, searches for .policy and .cgp files in:
  - .relicta/policies/
  - .github/relicta/policies/
  - policies/

Use --dir to specify a custom directory or --file to validate a single file.`,
	RunE: runPolicyValidate,
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List loaded policies",
	Long:  `Display all policies that would be loaded for the current project.`,
	RunE:  runPolicyList,
}

var (
	policyValidateDir  string
	policyValidateFile string
)

func init() {
	policyCmd.AddCommand(policyValidateCmd)
	policyCmd.AddCommand(policyListCmd)

	policyValidateCmd.Flags().StringVar(&policyValidateDir, "dir", "", "directory containing policy files")
	policyValidateCmd.Flags().StringVar(&policyValidateFile, "file", "", "specific policy file to validate")
}

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
	return nil
}

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

func runPolicyList(cmd *cobra.Command, args []string) error {
	dirs := dsl.DefaultPolicyPaths()

	loader := dsl.NewLoader(dsl.LoaderOptions{
		IgnoreErrors: true,
		Recursive:    true,
	})

	var totalPolicies int
	var totalRules int

	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}

		result, err := loader.LoadDir(absDir)
		if err != nil || result == nil || len(result.Policies) == 0 {
			continue
		}

		fmt.Printf("\nPolicies from %s:\n", dir)
		fmt.Println(strings.Repeat("-", 50))

		for _, pol := range result.Policies {
			totalPolicies++
			ruleCount := len(pol.Rules)
			totalRules += ruleCount

			fmt.Printf("\n  %s (%d rules)\n", pol.Name, ruleCount)
			if pol.Description != "" {
				fmt.Printf("    Description: %s\n", pol.Description)
			}

			for _, rule := range pol.Rules {
				status := "enabled"
				if !rule.Enabled {
					status = "disabled"
				}
				fmt.Printf("    - %s (priority: %d, %s)\n",
					rule.Name, rule.Priority, status)
				if rule.Description != "" {
					fmt.Printf("      %s\n", rule.Description)
				}
			}
		}
	}

	if totalPolicies == 0 {
		fmt.Println("No policies found.")
		fmt.Println("\nCreate policy files in one of these directories:")
		for _, dir := range dirs {
			fmt.Printf("  - %s\n", dir)
		}
		fmt.Println("\nPolicy files should have .policy or .cgp extension.")
		return nil
	}

	fmt.Printf("\n%s\n", strings.Repeat("-", 50))
	fmt.Printf("Total: %d policies, %d rules\n", totalPolicies, totalRules)
	return nil
}
