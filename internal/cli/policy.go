package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/cgp/policy/dsl"
	"gopkg.in/yaml.v3"
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
  relicta policy list

  # Test policies with simulated input
  relicta policy test --risk-score 0.85 --bump-type major`,
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

var policyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test policies with simulated inputs",
	Long: `Evaluate loaded policies against simulated release input.

Inputs can be passed via flags, with --input JSON file, or with --matrix
to evaluate multiple scenarios from one file.
Flag values override --input file values when both are provided.`,
	RunE: runPolicyTest,
}

var policyScaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Scaffold policy test fixtures",
	Long: `Generate starter fixtures for policy testing.

The command creates:
  - a single input fixture (policy-input.json by default)
  - a matrix fixture with low/high risk seeds and per-rule candidate scenarios

Use generated fixtures with 'relicta policy test --input/--matrix' and
iterate by refining scenario expectations.`,
	RunE: runPolicyScaffold,
}

var (
	policyValidateDir               string
	policyValidateFile              string
	policyTestDir                   string
	policyTestFile                  string
	policyTestBaselineDir           string
	policyTestBaselineFile          string
	policyTestCandidateDir          string
	policyTestCandidateFile         string
	policyTestInput                 string
	policyTestMatrix                string
	policyTestFailOnBlocked         bool
	policyTestRequireApproved       bool
	policyTestAssertExpected        bool
	policyTestScenarios             []string
	policyTestScenarioPatterns      []string
	policyTestScenarioTags          []string
	policyTestExcludeScenarios      []string
	policyTestExcludePatterns       []string
	policyTestExcludeTags           []string
	policyTestShardIndex            int
	policyTestShardTotal            int
	policyTestListScenarios         bool
	policyTestSummary               bool
	policyTestExplain               bool
	policyTestExplainMode           string
	policyTestJUnitOut              string
	policyTestSummaryOut            string
	policyTestCompareFailOnStricter bool
	policyTestCompareFailOnLooser   bool
	policyTestCompareMaxStricter    int
	policyTestCompareMaxLooser      int

	policyTestRiskScore    float64
	policyTestBumpType     string
	policyTestActorType    string
	policyTestActorID      string
	policyTestRepository   string
	policyTestBranch       string
	policyTestBreaking     int
	policyTestSecurity     int
	policyTestFeatures     int
	policyTestFixes        int
	policyTestDependencies int
	policyTestFilesChanged int
	policyTestLinesChanged int

	policyScaffoldDir              string
	policyScaffoldFile             string
	policyScaffoldInputOut         string
	policyScaffoldMatrixOut        string
	policyScaffoldForce            bool
	policyScaffoldMaxRuleScenarios int
)

func init() {
	policyCmd.AddCommand(policyValidateCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyTestCmd)
	policyCmd.AddCommand(policyScaffoldCmd)

	policyValidateCmd.Flags().StringVarP(&policyValidateDir, "dir", "d", "", "directory containing policy files")
	policyValidateCmd.Flags().StringVarP(&policyValidateFile, "file", "f", "", "specific policy file to validate")

	policyTestCmd.Flags().StringVarP(&policyTestDir, "dir", "d", "", "directory containing policy files")
	policyTestCmd.Flags().StringVarP(&policyTestFile, "file", "f", "", "single policy file to test")
	policyTestCmd.Flags().StringVar(&policyTestBaselineDir, "baseline-dir", "", "for compare mode: baseline policy directory")
	policyTestCmd.Flags().StringVar(&policyTestBaselineFile, "baseline-file", "", "for compare mode: baseline policy file")
	policyTestCmd.Flags().StringVar(&policyTestCandidateDir, "candidate-dir", "", "for compare mode: candidate policy directory")
	policyTestCmd.Flags().StringVar(&policyTestCandidateFile, "candidate-file", "", "for compare mode: candidate policy file")
	policyTestCmd.Flags().StringVar(&policyTestInput, "input", "", "input file with test values (.json, .yaml, .yml), or '-' for stdin")
	policyTestCmd.Flags().StringVar(&policyTestMatrix, "matrix", "", "matrix file with multiple scenarios (.json, .yaml, .yml), or '-' for stdin")
	policyTestCmd.Flags().StringArrayVar(&policyTestScenarios, "scenario", nil, "matrix scenario name to run (repeatable)")
	policyTestCmd.Flags().StringArrayVar(&policyTestScenarioPatterns, "scenario-pattern", nil, "matrix scenario glob pattern to run (repeatable), e.g. 'high-*'")
	policyTestCmd.Flags().StringArrayVar(&policyTestScenarioTags, "scenario-tag", nil, "matrix scenario tag to run (repeatable)")
	policyTestCmd.Flags().StringArrayVar(&policyTestExcludeScenarios, "exclude-scenario", nil, "matrix scenario name to exclude (repeatable)")
	policyTestCmd.Flags().StringArrayVar(&policyTestExcludePatterns, "exclude-scenario-pattern", nil, "matrix scenario glob pattern to exclude (repeatable), e.g. 'flaky-*'")
	policyTestCmd.Flags().StringArrayVar(&policyTestExcludeTags, "exclude-scenario-tag", nil, "matrix scenario tag to exclude (repeatable)")
	policyTestCmd.Flags().IntVar(&policyTestShardIndex, "shard-index", 0, "matrix shard index (1-based, requires --shard-total)")
	policyTestCmd.Flags().IntVar(&policyTestShardTotal, "shard-total", 0, "matrix shard count (requires --shard-index)")
	policyTestCmd.Flags().BoolVar(&policyTestListScenarios, "list-scenarios", false, "with --matrix: list scenario names and exit")
	policyTestCmd.Flags().BoolVar(&policyTestSummary, "summary", false, "for matrix mode: include aggregate result summary")
	policyTestCmd.Flags().BoolVar(&policyTestExplain, "explain", false, "include per-rule and per-condition evaluation trace in output")
	policyTestCmd.Flags().StringVar(&policyTestExplainMode, "explain-mode", "all", "trace verbosity for --explain: all, matched")
	policyTestCmd.Flags().StringVar(&policyTestJUnitOut, "junit-out", "", "for matrix mode: write JUnit XML report to file")
	policyTestCmd.Flags().StringVar(&policyTestSummaryOut, "summary-out", "", "for matrix mode: write compact JSON summary report to file")
	policyTestCmd.Flags().BoolVar(&policyTestCompareFailOnStricter, "compare-fail-on-stricter", false, "for compare mode: fail if candidate policy is stricter in any scenario")
	policyTestCmd.Flags().BoolVar(&policyTestCompareFailOnLooser, "compare-fail-on-looser", false, "for compare mode: fail if candidate policy is looser in any scenario")
	policyTestCmd.Flags().IntVar(&policyTestCompareMaxStricter, "compare-max-stricter", -1, "for compare mode: maximum allowed stricter scenarios (-1 disables)")
	policyTestCmd.Flags().IntVar(&policyTestCompareMaxLooser, "compare-max-looser", -1, "for compare mode: maximum allowed looser scenarios (-1 disables)")
	policyTestCmd.Flags().BoolVar(&policyTestFailOnBlocked, "fail-on-blocked", false, "exit with error if evaluation result is blocked")
	policyTestCmd.Flags().BoolVar(&policyTestRequireApproved, "require-approved", false, "exit with error unless decision is approved")
	policyTestCmd.Flags().BoolVar(&policyTestAssertExpected, "assert-expected", false, "for matrix mode: fail when scenario expectations do not match actual decision")
	policyTestCmd.Flags().Float64Var(&policyTestRiskScore, "risk-score", 0.3, "risk score to evaluate (0.0-1.0)")
	policyTestCmd.Flags().StringVar(&policyTestBumpType, "bump-type", "patch", "suggested bump type: major, minor, patch")
	policyTestCmd.Flags().StringVar(&policyTestActorType, "actor-type", "human", "actor type: human, agent, ci, system")
	policyTestCmd.Flags().StringVar(&policyTestActorID, "actor-id", "human:policy-test", "actor identifier")
	policyTestCmd.Flags().StringVar(&policyTestRepository, "repository", "local/repo", "repository identifier")
	policyTestCmd.Flags().StringVar(&policyTestBranch, "branch", "main", "branch name")
	policyTestCmd.Flags().IntVar(&policyTestBreaking, "breaking", 0, "breaking change count")
	policyTestCmd.Flags().IntVar(&policyTestSecurity, "security", 0, "security change count")
	policyTestCmd.Flags().IntVar(&policyTestFeatures, "features", 0, "feature change count")
	policyTestCmd.Flags().IntVar(&policyTestFixes, "fixes", 0, "fix change count")
	policyTestCmd.Flags().IntVar(&policyTestDependencies, "dependencies", 0, "dependency change count")
	policyTestCmd.Flags().IntVar(&policyTestFilesChanged, "files-changed", 0, "changed files count")
	policyTestCmd.Flags().IntVar(&policyTestLinesChanged, "lines-changed", 0, "changed lines count")

	policyScaffoldCmd.Flags().StringVarP(&policyScaffoldDir, "dir", "d", "", "directory containing policy files")
	policyScaffoldCmd.Flags().StringVarP(&policyScaffoldFile, "file", "f", "", "single policy file to inspect")
	policyScaffoldCmd.Flags().StringVar(&policyScaffoldInputOut, "input-out", "policy-input.json", "output path for single-input fixture (.json or .yaml)")
	policyScaffoldCmd.Flags().StringVar(&policyScaffoldMatrixOut, "matrix-out", "policy-matrix.yaml", "output path for matrix fixture (.json or .yaml)")
	policyScaffoldCmd.Flags().BoolVar(&policyScaffoldForce, "force", false, "overwrite output files if they already exist")
	policyScaffoldCmd.Flags().IntVar(&policyScaffoldMaxRuleScenarios, "max-rule-scenarios", 8, "maximum number of per-rule scenarios to include in matrix")
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

func runPolicyScaffold(cmd *cobra.Command, args []string) error {
	if policyScaffoldDir != "" && policyScaffoldFile != "" {
		return fmt.Errorf("--dir and --file cannot be used together")
	}
	if policyScaffoldInputOut == "" || policyScaffoldMatrixOut == "" {
		return fmt.Errorf("--input-out and --matrix-out are required")
	}
	if policyScaffoldMaxRuleScenarios < 0 {
		return fmt.Errorf("--max-rule-scenarios must be >= 0")
	}

	policies, err := loadPoliciesForTest(policyScaffoldDir, policyScaffoldFile)
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return fmt.Errorf("no policies found for scaffolding (use --file or --dir)")
	}

	input := buildScaffoldInput(policies)
	matrix, err := buildScaffoldMatrix(cmd.Context(), policies, input, policyScaffoldMaxRuleScenarios)
	if err != nil {
		return err
	}

	if err := writeScaffoldFile(policyScaffoldInputOut, input, policyScaffoldForce); err != nil {
		return fmt.Errorf("failed to write input fixture: %w", err)
	}
	if err := writeScaffoldFile(policyScaffoldMatrixOut, matrix, policyScaffoldForce); err != nil {
		return fmt.Errorf("failed to write matrix fixture: %w", err)
	}

	scenarioNames := make([]string, 0, len(matrix))
	for _, c := range matrix {
		scenarioNames = append(scenarioNames, c.Name)
	}

	if outputJSON {
		payload := map[string]any{
			"input_file":        policyScaffoldInputOut,
			"matrix_file":       policyScaffoldMatrixOut,
			"scenarios":         scenarioNames,
			"rule_scenarios":    max(0, len(matrix)-2),
			"policies_detected": len(policies),
		}
		return json.NewEncoder(os.Stdout).Encode(payload)
	}

	fmt.Printf("Created input fixture:  %s\n", policyScaffoldInputOut)
	fmt.Printf("Created matrix fixture: %s\n", policyScaffoldMatrixOut)
	fmt.Printf("Scenarios seeded:       %d (%d from rules)\n", len(matrix), max(0, len(matrix)-2))
	fmt.Println("\nNext steps:")
	fmt.Printf("  relicta policy test --matrix %s --json\n", policyScaffoldMatrixOut)
	fmt.Printf("  relicta policy test --matrix %s --assert-expected\n", policyScaffoldMatrixOut)
	return nil
}

type policyTestInputData struct {
	RiskScore    float64 `json:"risk_score" yaml:"risk_score"`
	BumpType     string  `json:"bump_type" yaml:"bump_type"`
	ActorType    string  `json:"actor_type" yaml:"actor_type"`
	ActorID      string  `json:"actor_id" yaml:"actor_id"`
	Repository   string  `json:"repository" yaml:"repository"`
	Branch       string  `json:"branch" yaml:"branch"`
	Breaking     int     `json:"breaking" yaml:"breaking"`
	Security     int     `json:"security" yaml:"security"`
	Features     int     `json:"features" yaml:"features"`
	Fixes        int     `json:"fixes" yaml:"fixes"`
	Dependencies int     `json:"dependencies" yaml:"dependencies"`
	FilesChanged int     `json:"files_changed" yaml:"files_changed"`
	LinesChanged int     `json:"lines_changed" yaml:"lines_changed"`
}

type policyTestOutput struct {
	Decision          cgp.DecisionType     `json:"decision"`
	Blocked           bool                 `json:"blocked"`
	BlockReason       string               `json:"block_reason,omitempty"`
	MatchedRules      []string             `json:"matched_rules,omitempty"`
	RequiredApprovers int                  `json:"required_approvers,omitempty"`
	Reviewers         []string             `json:"reviewers,omitempty"`
	Rationale         []string             `json:"rationale,omitempty"`
	RequiredActions   []cgp.RequiredAction `json:"required_actions,omitempty"`
	Conditions        []cgp.Condition      `json:"conditions,omitempty"`
	RuleTrace         []policy.RuleTrace   `json:"rule_trace,omitempty"`
}

type policyTestMatrixCase struct {
	Name   string   `json:"name" yaml:"name"`
	Tags   []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Expect struct {
		Decision          *cgp.DecisionType    `json:"decision,omitempty" yaml:"decision,omitempty"`
		Blocked           *bool                `json:"blocked,omitempty" yaml:"blocked,omitempty"`
		RequiredApprovers *int                 `json:"required_approvers,omitempty" yaml:"required_approvers,omitempty"`
		BlockReason       *string              `json:"block_reason,omitempty" yaml:"block_reason,omitempty"`
		RequiredActions   []cgp.RequiredAction `json:"required_actions,omitempty" yaml:"required_actions,omitempty"`
		Rationale         []string             `json:"rationale,omitempty" yaml:"rationale,omitempty"`
		Conditions        []cgp.Condition      `json:"conditions,omitempty" yaml:"conditions,omitempty"`
		Reviewers         []string             `json:"reviewers,omitempty" yaml:"reviewers,omitempty"`
		MatchedRules      []string             `json:"matched_rules,omitempty" yaml:"matched_rules,omitempty"`
	} `json:"expect,omitempty" yaml:"expect,omitempty"`
	policyTestInputData `yaml:",inline"`
}

type policyTestMatrixResult struct {
	Name            string                   `json:"name"`
	Input           policyTestInputData      `json:"input"`
	Output          policyTestOutput         `json:"output"`
	BaselineOutput  *policyTestOutput        `json:"baseline_output,omitempty"`
	CandidateOutput *policyTestOutput        `json:"candidate_output,omitempty"`
	Comparison      *policyTestComparison    `json:"comparison,omitempty"`
	AssertionDiff   *policyTestAssertionDiff `json:"assertion_diff,omitempty"`
}

type policyTestMatrixSummary struct {
	Total      int            `json:"total"`
	Blocked    int            `json:"blocked"`
	Mismatched int            `json:"mismatched"`
	Decisions  map[string]int `json:"decisions"`
}

type policyTestAssertionDiff struct {
	Mismatches []policyTestAssertionMismatch `json:"mismatches"`
}

type policyTestAssertionMismatch struct {
	Field    string `json:"field"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

type policyTestComparison struct {
	Changed           bool            `json:"changed"`
	Direction         string          `json:"direction,omitempty"`
	Decision          policyDiffValue `json:"decision"`
	Blocked           policyDiffValue `json:"blocked"`
	RequiredApprovers policyDiffValue `json:"required_approvers"`
}

type policyDiffValue struct {
	Baseline  any  `json:"baseline"`
	Candidate any  `json:"candidate"`
	Changed   bool `json:"changed"`
}

type policyTestJUnitSuite struct {
	XMLName  xml.Name              `xml:"testsuite"`
	Name     string                `xml:"name,attr"`
	Tests    int                   `xml:"tests,attr"`
	Failures int                   `xml:"failures,attr"`
	Cases    []policyTestJUnitCase `xml:"testcase"`
}

type policyTestJUnitCase struct {
	Classname string                  `xml:"classname,attr"`
	Name      string                  `xml:"name,attr"`
	Failure   *policyTestJUnitFailure `xml:"failure,omitempty"`
}

type policyTestJUnitFailure struct {
	Type    string `xml:"type,attr,omitempty"`
	Message string `xml:"message,attr,omitempty"`
	Text    string `xml:",chardata"`
}

type policyTestCompactSummaryReport struct {
	Total           int                         `json:"total"`
	Blocked         int                         `json:"blocked"`
	Mismatched      int                         `json:"mismatched"`
	Decisions       map[string]int              `json:"decisions"`
	FailedScenarios []policyTestCompactScenario `json:"failed_scenarios,omitempty"`
}

type policyTestCompactScenario struct {
	Name       string `json:"name"`
	Decision   string `json:"decision"`
	Blocked    bool   `json:"blocked"`
	Mismatched bool   `json:"mismatched"`
}

func runPolicyTest(cmd *cobra.Command, args []string) error {
	if policyTestInput != "" && policyTestMatrix != "" {
		return fmt.Errorf("--input and --matrix cannot be used together")
	}
	if policyTestFile != "" && policyTestDir != "" {
		return fmt.Errorf("--file and --dir cannot be used together")
	}
	if policyTestBaselineFile != "" && policyTestBaselineDir != "" {
		return fmt.Errorf("--baseline-file and --baseline-dir cannot be used together")
	}
	if policyTestCandidateFile != "" && policyTestCandidateDir != "" {
		return fmt.Errorf("--candidate-file and --candidate-dir cannot be used together")
	}
	compareMode := policyTestBaselineFile != "" || policyTestBaselineDir != "" || policyTestCandidateFile != "" || policyTestCandidateDir != ""
	if compareMode && policyTestMatrix == "" {
		return fmt.Errorf("policy compare mode requires --matrix")
	}
	if compareMode && policyTestCandidateFile == "" && policyTestCandidateDir == "" {
		return fmt.Errorf("policy compare mode requires --candidate-file or --candidate-dir")
	}
	if (policyTestCompareFailOnStricter || policyTestCompareFailOnLooser || policyTestCompareMaxStricter >= 0 || policyTestCompareMaxLooser >= 0) && !compareMode {
		return fmt.Errorf("compare threshold flags require compare mode (--baseline-* and --candidate-*)")
	}
	if policyTestJUnitOut != "" && policyTestMatrix == "" {
		return fmt.Errorf("--junit-out requires --matrix")
	}
	if policyTestJUnitOut != "" && policyTestListScenarios {
		return fmt.Errorf("--junit-out cannot be used with --list-scenarios")
	}
	if policyTestSummaryOut != "" && policyTestMatrix == "" {
		return fmt.Errorf("--summary-out requires --matrix")
	}
	if policyTestSummaryOut != "" && policyTestListScenarios {
		return fmt.Errorf("--summary-out cannot be used with --list-scenarios")
	}
	if policyTestListScenarios && policyTestMatrix == "" {
		return fmt.Errorf("--list-scenarios requires --matrix")
	}
	if policyTestMatrix != "" && policyTestListScenarios {
		return runPolicyTestMatrix(cmd.Context(), nil)
	}
	if policyTestMatrix != "" && compareMode {
		return runPolicyTestMatrix(cmd.Context(), nil)
	}

	policies, err := loadPoliciesForTest(policyTestDir, policyTestFile)
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return fmt.Errorf("no policies found to test")
	}

	if policyTestMatrix != "" {
		return runPolicyTestMatrix(cmd.Context(), policies)
	}

	input, err := resolvePolicyTestInput(cmd)
	if err != nil {
		return err
	}

	out, err := evaluatePolicyScenario(cmd.Context(), policies, input)
	if err != nil {
		return err
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}
	} else {
		printPolicyTestText(out)
	}

	if policyTestFailOnBlocked && out.Blocked {
		return fmt.Errorf("policy test failed: decision is blocked")
	}
	if policyTestRequireApproved && out.Decision != cgp.DecisionApproved {
		return fmt.Errorf("policy test failed: decision is %q (expected %q)", out.Decision, cgp.DecisionApproved)
	}
	return nil
}

func runPolicyTestMatrix(ctx context.Context, policies []policy.Policy) error {
	baselinePolicies := policies
	candidatePolicies := []policy.Policy(nil)
	compareMode := policyTestBaselineFile != "" || policyTestBaselineDir != "" || policyTestCandidateFile != "" || policyTestCandidateDir != ""
	if compareMode {
		var err error
		baselineDir := policyTestBaselineDir
		baselineFile := policyTestBaselineFile
		if baselineDir == "" && baselineFile == "" {
			baselineDir = policyTestDir
			baselineFile = policyTestFile
		}
		baselinePolicies, err = loadPoliciesForTest(baselineDir, baselineFile)
		if err != nil {
			return err
		}
		if len(baselinePolicies) == 0 {
			return fmt.Errorf("no baseline policies found to test")
		}

		candidatePolicies, err = loadPoliciesForTest(policyTestCandidateDir, policyTestCandidateFile)
		if err != nil {
			return err
		}
		if len(candidatePolicies) == 0 {
			return fmt.Errorf("no candidate policies found to test")
		}
	}

	cases, err := loadPolicyTestMatrix(policyTestMatrix)
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return fmt.Errorf("matrix file contains no scenarios")
	}
	if policyTestListScenarios {
		selected := cases
		if len(policyTestScenarios) > 0 ||
			len(policyTestScenarioPatterns) > 0 ||
			len(policyTestScenarioTags) > 0 ||
			len(policyTestExcludeScenarios) > 0 ||
			len(policyTestExcludePatterns) > 0 ||
			len(policyTestExcludeTags) > 0 ||
			policyTestShardIndex > 0 ||
			policyTestShardTotal > 0 {
			selected, err = filterPolicyTestMatrixCases(cases)
			if err != nil {
				return err
			}
		}

		names := make([]string, 0, len(selected))
		for i, c := range selected {
			name := strings.TrimSpace(c.Name)
			if name == "" {
				name = fmt.Sprintf("scenario-%d", i+1)
			}
			names = append(names, name)
		}
		if outputJSON {
			payload := map[string]any{"scenarios": names}
			if policyTestShardTotal > 0 {
				payload["shard_index"] = policyTestShardIndex
				payload["shard_total"] = policyTestShardTotal
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(payload); err != nil {
				return err
			}
		} else {
			if policyTestShardTotal > 0 {
				fmt.Printf("Scenarios (shard %d/%d):\n", policyTestShardIndex, policyTestShardTotal)
			} else {
				fmt.Println("Scenarios:")
			}
			for _, name := range names {
				fmt.Printf("- %s\n", name)
			}
		}
		return nil
	}
	cases, err = filterPolicyTestMatrixCases(cases)
	if err != nil {
		return err
	}

	results := make([]policyTestMatrixResult, 0, len(cases))
	mismatchedScenarios := make([]string, 0)
	for i, c := range cases {
		input := applyPolicyTestDefaults(c.policyTestInputData)
		out, evalErr := evaluatePolicyScenario(ctx, baselinePolicies, &input)
		if evalErr != nil {
			return fmt.Errorf("matrix scenario %d (%s) failed: %w", i+1, c.Name, evalErr)
		}
		name := c.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("scenario-%d", i+1)
		}
		result := policyTestMatrixResult{
			Name:   name,
			Input:  input,
			Output: *out,
		}
		if compareMode {
			candidateOut, candidateErr := evaluatePolicyScenario(ctx, candidatePolicies, &input)
			if candidateErr != nil {
				return fmt.Errorf("matrix scenario %d (%s) candidate evaluation failed: %w", i+1, c.Name, candidateErr)
			}
			result.BaselineOutput = out
			result.CandidateOutput = candidateOut
			result.Comparison = buildPolicyScenarioComparison(out, candidateOut)
			result.Output = *candidateOut
		}
		if policyTestAssertExpected {
			assertOut := &result.Output
			diff := buildPolicyAssertionDiff(c, assertOut)
			if diff != nil {
				result.AssertionDiff = diff
				mismatchedScenarios = append(mismatchedScenarios, name)
			}
		}
		results = append(results, result)
	}
	summary := buildPolicyMatrixSummary(results)
	if policyTestJUnitOut != "" {
		if err := writePolicyMatrixJUnit(policyTestJUnitOut, results); err != nil {
			return err
		}
	}
	if policyTestSummaryOut != "" {
		if err := writePolicyMatrixSummaryReport(policyTestSummaryOut, summary, results); err != nil {
			return err
		}
	}

	if outputJSON {
		if policyTestSummary {
			payload := map[string]any{
				"results": results,
				"summary": summary,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(payload); err != nil {
				return err
			}
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return err
			}
		}
	} else {
		for i, r := range results {
			fmt.Printf("[%d] %s\n", i+1, r.Name)
			if r.BaselineOutput != nil && r.CandidateOutput != nil {
				fmt.Println("Baseline:")
				printPolicyTestText(r.BaselineOutput)
				fmt.Println("Candidate:")
				printPolicyTestText(r.CandidateOutput)
				if r.Comparison != nil {
					fmt.Printf("Comparison changed: %t\n", r.Comparison.Changed)
					fmt.Printf("  Direction: %s\n", r.Comparison.Direction)
					fmt.Printf("  Decision: baseline=%v candidate=%v changed=%t\n", r.Comparison.Decision.Baseline, r.Comparison.Decision.Candidate, r.Comparison.Decision.Changed)
					fmt.Printf("  Blocked: baseline=%v candidate=%v changed=%t\n", r.Comparison.Blocked.Baseline, r.Comparison.Blocked.Candidate, r.Comparison.Blocked.Changed)
					fmt.Printf("  Required approvers: baseline=%v candidate=%v changed=%t\n", r.Comparison.RequiredApprovers.Baseline, r.Comparison.RequiredApprovers.Candidate, r.Comparison.RequiredApprovers.Changed)
				}
			} else {
				printPolicyTestText(&r.Output)
			}
			if r.AssertionDiff != nil {
				printPolicyAssertionDiffText(r.AssertionDiff)
			}
			if i != len(results)-1 {
				fmt.Println()
			}
		}
		if policyTestSummary {
			fmt.Println()
			printPolicyMatrixSummaryText(summary)
		}
	}

	if policyTestFailOnBlocked {
		for _, r := range results {
			if r.Output.Blocked {
				return fmt.Errorf("policy test failed: scenario %q is blocked", r.Name)
			}
		}
	}
	if policyTestRequireApproved {
		for _, r := range results {
			if r.Output.Decision != cgp.DecisionApproved {
				return fmt.Errorf("policy test failed: scenario %q decision is %q (expected %q)", r.Name, r.Output.Decision, cgp.DecisionApproved)
			}
		}
	}
	if compareMode {
		stricter, looser := countPolicyComparisonDirections(results)
		if policyTestCompareFailOnStricter && stricter > 0 {
			return fmt.Errorf("policy compare failed: candidate is stricter in %d scenario(s)", stricter)
		}
		if policyTestCompareFailOnLooser && looser > 0 {
			return fmt.Errorf("policy compare failed: candidate is looser in %d scenario(s)", looser)
		}
		if policyTestCompareMaxStricter >= 0 && stricter > policyTestCompareMaxStricter {
			return fmt.Errorf("policy compare failed: stricter scenarios=%d exceeds limit=%d", stricter, policyTestCompareMaxStricter)
		}
		if policyTestCompareMaxLooser >= 0 && looser > policyTestCompareMaxLooser {
			return fmt.Errorf("policy compare failed: looser scenarios=%d exceeds limit=%d", looser, policyTestCompareMaxLooser)
		}
	}
	if policyTestAssertExpected && len(mismatchedScenarios) > 0 {
		return fmt.Errorf("policy test failed: %d scenario(s) did not match expected outputs (%s)", len(mismatchedScenarios), strings.Join(mismatchedScenarios, ", "))
	}
	return nil
}

func filterPolicyTestMatrixCases(cases []policyTestMatrixCase) ([]policyTestMatrixCase, error) {
	if len(policyTestScenarios) == 0 &&
		len(policyTestScenarioPatterns) == 0 &&
		len(policyTestScenarioTags) == 0 &&
		len(policyTestExcludeScenarios) == 0 &&
		len(policyTestExcludePatterns) == 0 &&
		len(policyTestExcludeTags) == 0 &&
		policyTestShardIndex == 0 &&
		policyTestShardTotal == 0 {
		return cases, nil
	}

	want := make(map[string]struct{}, len(policyTestScenarios))
	for _, s := range policyTestScenarios {
		name := strings.TrimSpace(s)
		if name != "" {
			want[name] = struct{}{}
		}
	}
	patterns := make([]string, 0, len(policyTestScenarioPatterns))
	for _, p := range policyTestScenarioPatterns {
		pattern := strings.TrimSpace(p)
		if pattern != "" {
			patterns = append(patterns, pattern)
		}
	}
	includeTags := make(map[string]struct{}, len(policyTestScenarioTags))
	for _, t := range policyTestScenarioTags {
		tag := strings.TrimSpace(t)
		if tag != "" {
			includeTags[tag] = struct{}{}
		}
	}
	excludeWant := make(map[string]struct{}, len(policyTestExcludeScenarios))
	for _, s := range policyTestExcludeScenarios {
		name := strings.TrimSpace(s)
		if name != "" {
			excludeWant[name] = struct{}{}
		}
	}
	excludePatterns := make([]string, 0, len(policyTestExcludePatterns))
	for _, p := range policyTestExcludePatterns {
		pattern := strings.TrimSpace(p)
		if pattern != "" {
			excludePatterns = append(excludePatterns, pattern)
		}
	}
	excludeTags := make(map[string]struct{}, len(policyTestExcludeTags))
	for _, t := range policyTestExcludeTags {
		tag := strings.TrimSpace(t)
		if tag != "" {
			excludeTags[tag] = struct{}{}
		}
	}
	if len(policyTestScenarios) > 0 && len(want) == 0 {
		return nil, fmt.Errorf("--scenario provided but no non-empty names were given")
	}
	if len(policyTestScenarioPatterns) > 0 && len(patterns) == 0 {
		return nil, fmt.Errorf("--scenario-pattern provided but no non-empty patterns were given")
	}
	if len(policyTestScenarioTags) > 0 && len(includeTags) == 0 {
		return nil, fmt.Errorf("--scenario-tag provided but no non-empty tags were given")
	}
	if len(policyTestExcludeScenarios) > 0 && len(excludeWant) == 0 {
		return nil, fmt.Errorf("--exclude-scenario provided but no non-empty names were given")
	}
	if len(policyTestExcludePatterns) > 0 && len(excludePatterns) == 0 {
		return nil, fmt.Errorf("--exclude-scenario-pattern provided but no non-empty patterns were given")
	}
	if len(policyTestExcludeTags) > 0 && len(excludeTags) == 0 {
		return nil, fmt.Errorf("--exclude-scenario-tag provided but no non-empty tags were given")
	}
	if (policyTestShardIndex == 0) != (policyTestShardTotal == 0) {
		return nil, fmt.Errorf("--shard-index and --shard-total must be provided together")
	}
	if policyTestShardIndex < 0 {
		return nil, fmt.Errorf("--shard-index must be >= 1")
	}
	if policyTestShardTotal < 0 {
		return nil, fmt.Errorf("--shard-total must be >= 1")
	}
	if policyTestShardTotal > 0 {
		if policyTestShardTotal < 1 {
			return nil, fmt.Errorf("--shard-total must be >= 1")
		}
		if policyTestShardIndex < 1 || policyTestShardIndex > policyTestShardTotal {
			return nil, fmt.Errorf("--shard-index must be between 1 and --shard-total (got %d of %d)", policyTestShardIndex, policyTestShardTotal)
		}
	}

	filtered := make([]policyTestMatrixCase, 0, len(cases))
	seen := make(map[string]struct{}, len(cases))
	for i, c := range cases {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = fmt.Sprintf("scenario-%d", i+1)
		}
		matchByName := false
		if _, ok := want[name]; ok {
			matchByName = true
		}
		matchByPattern := false
		for _, pattern := range patterns {
			ok, err := path.Match(pattern, name)
			if err != nil {
				return nil, fmt.Errorf("invalid --scenario-pattern %q: %w", pattern, err)
			}
			if ok {
				matchByPattern = true
				break
			}
		}
		matchByTag := false
		if len(includeTags) > 0 {
			for _, tag := range c.Tags {
				if _, ok := includeTags[strings.TrimSpace(tag)]; ok {
					matchByTag = true
					break
				}
			}
		}
		if (len(want) == 0 && len(patterns) == 0 && len(includeTags) == 0) || matchByName || matchByPattern || matchByTag {
			excluded := false
			if _, ok := excludeWant[name]; ok {
				excluded = true
			}
			if !excluded {
				for _, pattern := range excludePatterns {
					ok, err := path.Match(pattern, name)
					if err != nil {
						return nil, fmt.Errorf("invalid --exclude-scenario-pattern %q: %w", pattern, err)
					}
					if ok {
						excluded = true
						break
					}
				}
			}
			if !excluded && len(excludeTags) > 0 {
				for _, tag := range c.Tags {
					if _, ok := excludeTags[strings.TrimSpace(tag)]; ok {
						excluded = true
						break
					}
				}
			}
			if excluded {
				continue
			}
			filtered = append(filtered, c)
			seen[name] = struct{}{}
		}
	}

	missing := make([]string, 0)
	for name := range want {
		if _, ok := seen[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("matrix scenario(s) not found: %s", strings.Join(missing, ", "))
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no matrix scenarios matched the provided selectors")
	}
	if policyTestShardTotal > 0 {
		sharded := make([]policyTestMatrixCase, 0, len(filtered))
		for i, c := range filtered {
			name := matrixScenarioName(c, i)
			if matrixScenarioShard(name, policyTestShardTotal) == policyTestShardIndex {
				sharded = append(sharded, c)
			}
		}
		if len(sharded) == 0 {
			return nil, fmt.Errorf("no matrix scenarios matched shard %d/%d", policyTestShardIndex, policyTestShardTotal)
		}
		return sharded, nil
	}
	return filtered, nil
}

func matrixScenarioName(c policyTestMatrixCase, idx int) string {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = fmt.Sprintf("scenario-%d", idx+1)
	}
	return name
}

func matrixScenarioShard(name string, total int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32()%uint32(total)) + 1
}

func evaluatePolicyScenario(ctx context.Context, policies []policy.Policy, input *policyTestInputData) (*policyTestOutput, error) {
	actorKind, ok := cgp.ParseActorKind(input.ActorType)
	if !ok {
		return nil, fmt.Errorf("invalid actor type %q (supported: human, agent, ci, system)", input.ActorType)
	}

	bumpType, err := parsePolicyTestBumpType(input.BumpType)
	if err != nil {
		return nil, err
	}

	proposal := cgp.NewProposal(
		cgp.Actor{
			Kind: actorKind,
			ID:   input.ActorID,
			Name: input.ActorID,
		},
		cgp.ProposalScope{
			Repository:  input.Repository,
			Branch:      input.Branch,
			CommitRange: "HEAD~1..HEAD",
		},
		cgp.ProposalIntent{
			Summary:       "policy test",
			SuggestedBump: bumpType,
			Confidence:    1.0,
		},
	)

	analysis := &cgp.ChangeAnalysis{
		Features:     input.Features,
		Fixes:        input.Fixes,
		Breaking:     input.Breaking,
		Security:     input.Security,
		Dependencies: input.Dependencies,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: input.FilesChanged,
			LinesChanged: input.LinesChanged,
			Score:        0,
		},
	}

	engine := policy.NewEngine(policies, nil)
	result, err := engine.Evaluate(ctx, proposal, analysis, input.RiskScore)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	out := policyTestOutput{
		Decision:          result.Decision,
		Blocked:           result.Blocked,
		BlockReason:       result.BlockReason,
		MatchedRules:      result.MatchedRules,
		RequiredApprovers: result.RequiredApprovers,
		Reviewers:         result.Reviewers,
		Rationale:         result.Rationale,
		RequiredActions:   result.RequiredActions,
		Conditions:        result.Conditions,
	}
	if policyTestExplain {
		trace, err := filterPolicyExplainTrace(result.RuleTrace, policyTestExplainMode)
		if err != nil {
			return nil, err
		}
		out.RuleTrace = trace
	}

	return &out, nil
}

func filterPolicyExplainTrace(trace []policy.RuleTrace, mode string) ([]policy.RuleTrace, error) {
	switch mode {
	case "", "all":
		return trace, nil
	case "matched":
		filtered := make([]policy.RuleTrace, 0, len(trace))
		for _, t := range trace {
			if t.Matched {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	default:
		return nil, fmt.Errorf("invalid --explain-mode %q (supported: all, matched)", mode)
	}
}

func printPolicyTestText(out *policyTestOutput) {
	fmt.Printf("Decision: %s\n", out.Decision)
	fmt.Printf("Blocked:  %t\n", out.Blocked)
	if out.BlockReason != "" {
		fmt.Printf("Reason:   %s\n", out.BlockReason)
	}
	if len(out.MatchedRules) > 0 {
		fmt.Printf("Matched rules: %s\n", strings.Join(out.MatchedRules, ", "))
	}
	if out.RequiredApprovers > 0 {
		fmt.Printf("Required approvers: %d\n", out.RequiredApprovers)
	}
	if len(out.Reviewers) > 0 {
		fmt.Printf("Reviewers: %s\n", strings.Join(out.Reviewers, ", "))
	}
	if len(out.Rationale) > 0 {
		fmt.Println("Rationale:")
		for _, r := range out.Rationale {
			fmt.Printf("  - %s\n", r)
		}
	}
	if len(out.RuleTrace) > 0 {
		fmt.Println("Evaluation trace:")
		for _, trace := range out.RuleTrace {
			status := "no-match"
			if trace.Matched {
				status = "matched"
			}
			fmt.Printf("  - [%s] %s (%s)\n", status, trace.RuleID, trace.RuleName)
			for _, cond := range trace.Conditions {
				condStatus := "no-match"
				if cond.Matched {
					condStatus = "matched"
				}
				fmt.Printf("      %s %s %v -> actual=%v [%s]\n", cond.Field, cond.Operator, cond.Expected, cond.Actual, condStatus)
				if cond.MissingField {
					fmt.Println("        note: field missing in evaluation context")
				}
				if cond.Error != "" {
					fmt.Printf("        error: %s\n", cond.Error)
				}
			}
		}
	}
}

func resolvePolicyTestInput(cmd *cobra.Command) (*policyTestInputData, error) {
	input := applyPolicyTestDefaults(policyTestInputData{
		RiskScore:    policyTestRiskScore,
		BumpType:     policyTestBumpType,
		ActorType:    policyTestActorType,
		ActorID:      policyTestActorID,
		Repository:   policyTestRepository,
		Branch:       policyTestBranch,
		Breaking:     policyTestBreaking,
		Security:     policyTestSecurity,
		Features:     policyTestFeatures,
		Fixes:        policyTestFixes,
		Dependencies: policyTestDependencies,
		FilesChanged: policyTestFilesChanged,
		LinesChanged: policyTestLinesChanged,
	})

	if policyTestInput == "" {
		return &input, nil
	}

	b, err := readPolicyInputSource(policyTestInput)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file: %w", err)
	}

	fromFile := &policyTestInputData{}
	if err := unmarshalPolicyInput(policyTestInput, b, fromFile); err != nil {
		return nil, err
	}

	// file defaults first, then explicit flags override
	merged := *fromFile
	if cmd.Flags().Changed("risk-score") {
		merged.RiskScore = policyTestRiskScore
	}
	if cmd.Flags().Changed("bump-type") {
		merged.BumpType = policyTestBumpType
	}
	if cmd.Flags().Changed("actor-type") {
		merged.ActorType = policyTestActorType
	}
	if cmd.Flags().Changed("actor-id") {
		merged.ActorID = policyTestActorID
	}
	if cmd.Flags().Changed("repository") {
		merged.Repository = policyTestRepository
	}
	if cmd.Flags().Changed("branch") {
		merged.Branch = policyTestBranch
	}
	if cmd.Flags().Changed("breaking") {
		merged.Breaking = policyTestBreaking
	}
	if cmd.Flags().Changed("security") {
		merged.Security = policyTestSecurity
	}
	if cmd.Flags().Changed("features") {
		merged.Features = policyTestFeatures
	}
	if cmd.Flags().Changed("fixes") {
		merged.Fixes = policyTestFixes
	}
	if cmd.Flags().Changed("dependencies") {
		merged.Dependencies = policyTestDependencies
	}
	if cmd.Flags().Changed("files-changed") {
		merged.FilesChanged = policyTestFilesChanged
	}
	if cmd.Flags().Changed("lines-changed") {
		merged.LinesChanged = policyTestLinesChanged
	}

	applied := applyPolicyTestDefaults(merged)
	return &applied, nil
}

func loadPoliciesForTest(dir, file string) ([]policy.Policy, error) {
	loader := dsl.NewLoader(dsl.LoaderOptions{
		IgnoreErrors: false,
		Recursive:    true,
	})

	if file != "" {
		pol, err := loader.LoadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to load policy file %s: %w", file, err)
		}
		return []policy.Policy{*pol}, nil
	}

	var searchDirs []string
	if dir != "" {
		searchDirs = []string{dir}
	} else {
		searchDirs = dsl.DefaultPolicyPaths()
	}

	var loaded []policy.Policy
	for _, d := range searchDirs {
		absDir, err := filepath.Abs(d)
		if err != nil {
			absDir = d
		}
		result, err := loader.LoadDir(absDir)
		if err != nil || result == nil {
			continue
		}
		for _, pol := range result.Policies {
			loaded = append(loaded, *pol)
		}
	}

	return loaded, nil
}

func parsePolicyTestBumpType(value string) (cgp.BumpType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "major":
		return cgp.BumpTypeMajor, nil
	case "minor":
		return cgp.BumpTypeMinor, nil
	case "patch":
		return cgp.BumpTypePatch, nil
	default:
		return "", fmt.Errorf("invalid bump type %q (supported: major, minor, patch)", value)
	}
}

func buildScaffoldInput(policies []policy.Policy) policyTestInputData {
	input := applyPolicyTestDefaults(policyTestInputData{
		RiskScore:    0.30,
		BumpType:     "patch",
		ActorType:    "human",
		Breaking:     0,
		Security:     0,
		Features:     1,
		Fixes:        1,
		Dependencies: 0,
		FilesChanged: 3,
		LinesChanged: 60,
	})

	for _, pol := range policies {
		for _, rule := range pol.Rules {
			if !rule.Enabled {
				continue
			}
			for _, cond := range rule.Conditions {
				applyConditionToInput(&input, cond, scaffoldSeedNeutral)
			}
		}
	}
	return applyPolicyTestDefaults(input)
}

func buildScaffoldMatrix(ctx context.Context, policies []policy.Policy, base policyTestInputData, maxRuleScenarios int) ([]policyTestMatrixCase, error) {
	seen := map[string]struct{}{}
	scenarios := make([]policyTestMatrixCase, 0, max(2, maxRuleScenarios+2))

	addScenario := func(name string, tags []string, input policyTestInputData) error {
		if _, ok := seen[name]; ok {
			return nil
		}
		seen[name] = struct{}{}

		out, err := evaluatePolicyScenarioForScaffold(ctx, policies, &input)
		if err != nil {
			return err
		}
		scenarios = append(scenarios, buildScaffoldMatrixCase(name, tags, input, out))
		return nil
	}

	low := applyPolicyTestDefaults(policyTestInputData{
		RiskScore:    0.10,
		BumpType:     "patch",
		ActorType:    "human",
		Breaking:     0,
		Security:     0,
		Features:     0,
		Fixes:        1,
		Dependencies: 0,
		FilesChanged: 1,
		LinesChanged: 20,
	})
	high := applyPolicyTestDefaults(policyTestInputData{
		RiskScore:    0.90,
		BumpType:     "major",
		ActorType:    "agent",
		Breaking:     1,
		Security:     1,
		Features:     3,
		Fixes:        0,
		Dependencies: 2,
		FilesChanged: 30,
		LinesChanged: 600,
	})

	for _, pol := range policies {
		for _, rule := range pol.Rules {
			if !rule.Enabled {
				continue
			}
			for _, cond := range rule.Conditions {
				highMode := scaffoldSeedMatch
				lowMode := scaffoldSeedMatch
				switch cond.Operator {
				case policy.OperatorLessThan, policy.OperatorLessOrEqual:
					highMode = scaffoldSeedInverse
					lowMode = scaffoldSeedMatch
				case policy.OperatorGreaterThan, policy.OperatorGreaterOrEqual:
					highMode = scaffoldSeedMatch
					lowMode = scaffoldSeedInverse
				}
				applyConditionToInput(&high, cond, highMode)
				applyConditionToInput(&low, cond, lowMode)
			}
		}
	}

	if err := addScenario("low-risk-seed", []string{"seed", "low-risk"}, low); err != nil {
		return nil, err
	}
	if err := addScenario("high-risk-seed", []string{"seed", "high-risk"}, high); err != nil {
		return nil, err
	}

	var rules []policy.Rule
	for _, pol := range policies {
		for _, rule := range pol.Rules {
			if rule.Enabled {
				rules = append(rules, rule)
			}
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].Priority > rules[j].Priority
	})

	for i, rule := range rules {
		if i >= maxRuleScenarios {
			break
		}
		scenarioInput := base
		for _, cond := range rule.Conditions {
			applyConditionToInput(&scenarioInput, cond, scaffoldSeedMatch)
		}
		name := "rule-" + sanitizeScenarioName(rule.ID)
		if name == "rule-" {
			name = fmt.Sprintf("rule-%d", i+1)
		}
		if err := addScenario(name, []string{"seed", "rule-derived"}, scenarioInput); err != nil {
			return nil, err
		}
	}

	return scenarios, nil
}

func buildScaffoldMatrixCase(name string, tags []string, input policyTestInputData, out *policyTestOutput) policyTestMatrixCase {
	c := policyTestMatrixCase{
		Name:                name,
		Tags:                tags,
		policyTestInputData: input,
	}
	if out != nil {
		decision := out.Decision
		blocked := out.Blocked
		c.Expect.Decision = &decision
		c.Expect.Blocked = &blocked
		if out.BlockReason != "" {
			blockReason := out.BlockReason
			c.Expect.BlockReason = &blockReason
		}
		if out.RequiredApprovers > 0 {
			requiredApprovers := out.RequiredApprovers
			c.Expect.RequiredApprovers = &requiredApprovers
		}
	}
	return c
}

func evaluatePolicyScenarioForScaffold(ctx context.Context, policies []policy.Policy, input *policyTestInputData) (*policyTestOutput, error) {
	prevExplain := policyTestExplain
	prevMode := policyTestExplainMode
	policyTestExplain = false
	policyTestExplainMode = "all"
	defer func() {
		policyTestExplain = prevExplain
		policyTestExplainMode = prevMode
	}()
	return evaluatePolicyScenario(ctx, policies, input)
}

type scaffoldSeedMode int

const (
	scaffoldSeedMatch scaffoldSeedMode = iota
	scaffoldSeedInverse
	scaffoldSeedNeutral
)

func applyConditionToInput(in *policyTestInputData, cond policy.Condition, mode scaffoldSeedMode) {
	field := strings.ToLower(strings.TrimSpace(cond.Field))
	switch field {
	case "risk.score", "risk_score":
		setScaffoldFloat(&in.RiskScore, cond, mode)
	case "change.bump_kind", "intent.suggestedbump", "bump_type":
		setScaffoldBumpType(&in.BumpType, cond, mode)
	case "actor.kind", "actor_type":
		setScaffoldActorType(&in.ActorType, cond, mode)
	case "actor.id", "actor_id":
		setScaffoldString(&in.ActorID, cond, mode)
	case "scope.repository", "repository":
		setScaffoldString(&in.Repository, cond, mode)
	case "scope.branch", "branch":
		setScaffoldString(&in.Branch, cond, mode)
	case "change.breaking", "breaking":
		setScaffoldInt(&in.Breaking, cond, mode)
	case "change.security", "security":
		setScaffoldInt(&in.Security, cond, mode)
	case "change.features", "features":
		setScaffoldInt(&in.Features, cond, mode)
	case "change.fixes", "fixes":
		setScaffoldInt(&in.Fixes, cond, mode)
	case "change.dependencies", "dependencies":
		setScaffoldInt(&in.Dependencies, cond, mode)
	case "blastradius.fileschanged", "change.files_changed", "files_changed":
		setScaffoldInt(&in.FilesChanged, cond, mode)
	case "blastradius.lineschanged", "change.lines_changed", "lines_changed":
		setScaffoldInt(&in.LinesChanged, cond, mode)
	}
}

func setScaffoldFloat(dst *float64, cond policy.Condition, mode scaffoldSeedMode) {
	v, ok := toFloat64(cond.Value)
	if !ok {
		return
	}
	switch cond.Operator {
	case policy.OperatorGreaterThan:
		if mode == scaffoldSeedInverse {
			*dst = maxFloat(0, minFloat(*dst, v-0.1))
			return
		}
		*dst = maxFloat(*dst, minFloat(1, v+0.1))
	case policy.OperatorGreaterOrEqual:
		if mode == scaffoldSeedInverse {
			*dst = maxFloat(0, minFloat(*dst, v-0.1))
			return
		}
		*dst = maxFloat(*dst, minFloat(1, v))
	case policy.OperatorLessThan:
		if mode == scaffoldSeedInverse {
			*dst = maxFloat(*dst, minFloat(1, v+0.1))
			return
		}
		*dst = minFloat(*dst, maxFloat(0, v-0.1))
	case policy.OperatorLessOrEqual:
		if mode == scaffoldSeedInverse {
			*dst = maxFloat(*dst, minFloat(1, v+0.1))
			return
		}
		*dst = minFloat(*dst, minFloat(1, v))
	case policy.OperatorEqual:
		if mode != scaffoldSeedInverse {
			*dst = v
		}
	}
	*dst = maxFloat(0, minFloat(*dst, 1))
}

func setScaffoldInt(dst *int, cond policy.Condition, mode scaffoldSeedMode) {
	v, ok := toInt(cond.Value)
	if !ok {
		return
	}
	switch cond.Operator {
	case policy.OperatorGreaterThan:
		if mode == scaffoldSeedInverse {
			*dst = maxInt(0, minInt(*dst, v))
			return
		}
		*dst = maxInt(*dst, v+1)
	case policy.OperatorGreaterOrEqual:
		if mode == scaffoldSeedInverse {
			*dst = maxInt(0, minInt(*dst, v-1))
			return
		}
		*dst = maxInt(*dst, v)
	case policy.OperatorLessThan:
		if mode == scaffoldSeedInverse {
			*dst = maxInt(*dst, v)
			return
		}
		*dst = minInt(*dst, maxInt(0, v-1))
	case policy.OperatorLessOrEqual:
		if mode == scaffoldSeedInverse {
			*dst = maxInt(*dst, v+1)
			return
		}
		*dst = minInt(*dst, maxInt(0, v))
	case policy.OperatorEqual:
		if mode != scaffoldSeedInverse {
			*dst = maxInt(0, v)
		}
	}
	if *dst < 0 {
		*dst = 0
	}
}

func setScaffoldString(dst *string, cond policy.Condition, mode scaffoldSeedMode) {
	value, ok := scaffoldStringValue(cond.Value)
	if !ok {
		return
	}
	switch cond.Operator {
	case policy.OperatorEqual, policy.OperatorContains, policy.OperatorMatches:
		if mode == scaffoldSeedInverse {
			*dst = "other"
			return
		}
		*dst = value
	case policy.OperatorNotEqual:
		if mode == scaffoldSeedInverse {
			*dst = value
			return
		}
		*dst = "other"
	case policy.OperatorIn:
		if mode == scaffoldSeedInverse {
			*dst = "other"
			return
		}
		*dst = value
	}
}

func setScaffoldBumpType(dst *string, cond policy.Condition, mode scaffoldSeedMode) {
	value, ok := scaffoldStringValue(cond.Value)
	if !ok {
		return
	}
	value = strings.ToLower(strings.TrimSpace(value))
	switch cond.Operator {
	case policy.OperatorEqual, policy.OperatorIn, policy.OperatorContains, policy.OperatorMatches:
		if mode == scaffoldSeedInverse {
			switch value {
			case "major":
				*dst = "patch"
			case "minor":
				*dst = "patch"
			default:
				*dst = "major"
			}
			return
		}
		if value == "major" || value == "minor" || value == "patch" {
			*dst = value
		}
	case policy.OperatorNotEqual:
		if mode == scaffoldSeedInverse {
			if value == "major" || value == "minor" || value == "patch" {
				*dst = value
			}
			return
		}
		if value == "patch" {
			*dst = "major"
		} else {
			*dst = "patch"
		}
	}
}

func setScaffoldActorType(dst *string, cond policy.Condition, mode scaffoldSeedMode) {
	value, ok := scaffoldStringValue(cond.Value)
	if !ok {
		return
	}
	value = strings.ToLower(strings.TrimSpace(value))
	switch cond.Operator {
	case policy.OperatorEqual, policy.OperatorIn, policy.OperatorContains, policy.OperatorMatches:
		if mode == scaffoldSeedInverse {
			switch value {
			case "human":
				*dst = "agent"
			case "agent":
				*dst = "human"
			case "ci":
				*dst = "human"
			case "system":
				*dst = "human"
			default:
				*dst = "human"
			}
			return
		}
		if value == "human" || value == "agent" || value == "ci" || value == "system" {
			*dst = value
		}
	case policy.OperatorNotEqual:
		if mode == scaffoldSeedInverse {
			if value == "human" || value == "agent" || value == "ci" || value == "system" {
				*dst = value
			}
			return
		}
		if value == "human" {
			*dst = "agent"
		} else {
			*dst = "human"
		}
	}
}

func scaffoldStringValue(v any) (string, bool) {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value), true
	case []string:
		if len(value) > 0 {
			return strings.TrimSpace(value[0]), true
		}
	case []any:
		for _, item := range value {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s), true
			}
		}
	}
	return "", false
}

func writeScaffoldFile(path string, data any, force bool) error {
	if path == "-" {
		return fmt.Errorf("output path '-' is not supported for scaffold files")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	encoded, err := marshalScaffoldFile(path, data)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, encoded, 0o644)
}

func marshalScaffoldFile(path string, data any) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		b, err := yaml.Marshal(data)
		if err != nil {
			return nil, err
		}
		return b, nil
	case ".json", "":
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(b, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported scaffold output extension for %s (use .json, .yaml, or .yml)", path)
	}
}

func sanitizeScenarioName(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range in {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case int32:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i), true
		}
		f, err := n.Float64()
		return int(f), err == nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		return i, err == nil
	default:
		return 0, false
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func applyPolicyTestDefaults(in policyTestInputData) policyTestInputData {
	out := in
	if out.BumpType == "" {
		out.BumpType = "patch"
	}
	if out.ActorType == "" {
		out.ActorType = "human"
	}
	if out.ActorID == "" {
		out.ActorID = "human:policy-test"
	}
	if out.Repository == "" {
		out.Repository = "local/repo"
	}
	if out.Branch == "" {
		out.Branch = "main"
	}
	return out
}

func loadPolicyTestMatrix(path string) ([]policyTestMatrixCase, error) {
	b, err := readPolicyInputSource(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read matrix file: %w", err)
	}
	var scenarios []policyTestMatrixCase
	if err := unmarshalPolicyInput(path, b, &scenarios); err != nil {
		return nil, err
	}
	return scenarios, nil
}

func unmarshalPolicyInput(path string, data []byte, out any) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, out); err != nil {
			return fmt.Errorf("failed to parse YAML %s: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("failed to parse JSON %s: %w", path, err)
		}
	default:
		// Extension-less inputs (e.g. stdin '-') are auto-detected: try JSON first, then YAML.
		if err := json.Unmarshal(data, out); err == nil {
			return nil
		}
		if err := yaml.Unmarshal(data, out); err == nil {
			return nil
		}
		return fmt.Errorf("failed to parse %s as JSON or YAML", path)
	}
	return nil
}

func readPolicyInputSource(path string) ([]byte, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return os.ReadFile(path)
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		if counts[s] == 0 {
			return false
		}
		counts[s]--
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

func equalRequiredActionSets(a, b []cgp.RequiredAction) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, action := range a {
		counts[requiredActionKey(action)]++
	}
	for _, action := range b {
		key := requiredActionKey(action)
		if counts[key] == 0 {
			return false
		}
		counts[key]--
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

func requiredActionKey(action cgp.RequiredAction) string {
	return action.Type + "|" + action.Description + "|" + action.Assignee + "|" + action.Deadline
}

func equalConditionSets(a, b []cgp.Condition) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, cond := range a {
		counts[conditionKey(cond)]++
	}
	for _, cond := range b {
		key := conditionKey(cond)
		if counts[key] == 0 {
			return false
		}
		counts[key]--
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}

func conditionKey(cond cgp.Condition) string {
	return cond.Type + "|" + cond.Value
}

func buildPolicyAssertionDiff(c policyTestMatrixCase, out *policyTestOutput) *policyTestAssertionDiff {
	mismatches := make([]policyTestAssertionMismatch, 0)
	if c.Expect.Decision != nil && out.Decision != *c.Expect.Decision {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "decision",
			Expected: *c.Expect.Decision,
			Actual:   out.Decision,
		})
	}
	if c.Expect.Blocked != nil && out.Blocked != *c.Expect.Blocked {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "blocked",
			Expected: *c.Expect.Blocked,
			Actual:   out.Blocked,
		})
	}
	if c.Expect.RequiredApprovers != nil && out.RequiredApprovers != *c.Expect.RequiredApprovers {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "required_approvers",
			Expected: *c.Expect.RequiredApprovers,
			Actual:   out.RequiredApprovers,
		})
	}
	if c.Expect.BlockReason != nil && out.BlockReason != *c.Expect.BlockReason {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "block_reason",
			Expected: *c.Expect.BlockReason,
			Actual:   out.BlockReason,
		})
	}
	if c.Expect.RequiredActions != nil && !equalRequiredActionSets(c.Expect.RequiredActions, out.RequiredActions) {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "required_actions",
			Expected: c.Expect.RequiredActions,
			Actual:   out.RequiredActions,
		})
	}
	if c.Expect.Rationale != nil && !equalStringSets(c.Expect.Rationale, out.Rationale) {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "rationale",
			Expected: c.Expect.Rationale,
			Actual:   out.Rationale,
		})
	}
	if c.Expect.Conditions != nil && !equalConditionSets(c.Expect.Conditions, out.Conditions) {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "conditions",
			Expected: c.Expect.Conditions,
			Actual:   out.Conditions,
		})
	}
	if c.Expect.Reviewers != nil && !equalStringSets(c.Expect.Reviewers, out.Reviewers) {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "reviewers",
			Expected: c.Expect.Reviewers,
			Actual:   out.Reviewers,
		})
	}
	if c.Expect.MatchedRules != nil && !equalStringSets(c.Expect.MatchedRules, out.MatchedRules) {
		mismatches = append(mismatches, policyTestAssertionMismatch{
			Field:    "matched_rules",
			Expected: c.Expect.MatchedRules,
			Actual:   out.MatchedRules,
		})
	}
	if len(mismatches) == 0 {
		return nil
	}
	return &policyTestAssertionDiff{Mismatches: mismatches}
}

func printPolicyAssertionDiffText(diff *policyTestAssertionDiff) {
	fmt.Println("Assertion diff:")
	for _, m := range diff.Mismatches {
		fmt.Printf("  - %s: expected=%v actual=%v\n", m.Field, m.Expected, m.Actual)
	}
}

func buildPolicyMatrixSummary(results []policyTestMatrixResult) policyTestMatrixSummary {
	summary := policyTestMatrixSummary{
		Total:     len(results),
		Decisions: map[string]int{},
	}
	for _, r := range results {
		if r.Output.Blocked {
			summary.Blocked++
		}
		if r.AssertionDiff != nil && len(r.AssertionDiff.Mismatches) > 0 {
			summary.Mismatched++
		}
		summary.Decisions[string(r.Output.Decision)]++
	}
	return summary
}

func buildPolicyScenarioComparison(baseline, candidate *policyTestOutput) *policyTestComparison {
	if baseline == nil || candidate == nil {
		return nil
	}
	decisionChanged := baseline.Decision != candidate.Decision
	blockedChanged := baseline.Blocked != candidate.Blocked
	approversChanged := baseline.RequiredApprovers != candidate.RequiredApprovers
	return &policyTestComparison{
		Changed:   decisionChanged || blockedChanged || approversChanged,
		Direction: comparisonDirectionLabel(comparePolicyStrictness(baseline, candidate)),
		Decision: policyDiffValue{
			Baseline:  baseline.Decision,
			Candidate: candidate.Decision,
			Changed:   decisionChanged,
		},
		Blocked: policyDiffValue{
			Baseline:  baseline.Blocked,
			Candidate: candidate.Blocked,
			Changed:   blockedChanged,
		},
		RequiredApprovers: policyDiffValue{
			Baseline:  baseline.RequiredApprovers,
			Candidate: candidate.RequiredApprovers,
			Changed:   approversChanged,
		},
	}
}

func countPolicyComparisonDirections(results []policyTestMatrixResult) (stricter int, looser int) {
	for _, r := range results {
		if r.Comparison == nil {
			continue
		}
		switch r.Comparison.Direction {
		case "stricter":
			stricter++
		case "looser":
			looser++
		}
	}
	return stricter, looser
}

func comparisonDirectionLabel(v int) string {
	switch {
	case v > 0:
		return "stricter"
	case v < 0:
		return "looser"
	default:
		return "same"
	}
}

// comparePolicyStrictness returns:
// 1 when candidate is stricter, -1 when candidate is looser, 0 when equivalent.
func comparePolicyStrictness(baseline, candidate *policyTestOutput) int {
	baseRank := policyDecisionStrictnessRank(baseline.Decision)
	candRank := policyDecisionStrictnessRank(candidate.Decision)
	if candRank > baseRank {
		return 1
	}
	if candRank < baseRank {
		return -1
	}
	if candidate.RequiredApprovers > baseline.RequiredApprovers {
		return 1
	}
	if candidate.RequiredApprovers < baseline.RequiredApprovers {
		return -1
	}
	return 0
}

func policyDecisionStrictnessRank(decision cgp.DecisionType) int {
	switch decision {
	case cgp.DecisionApproved:
		return 0
	case cgp.DecisionApprovalRequired, cgp.DecisionDeferred:
		return 1
	case cgp.DecisionRejected:
		return 2
	default:
		return 0
	}
}

func printPolicyMatrixSummaryText(summary policyTestMatrixSummary) {
	fmt.Println("Summary:")
	fmt.Printf("  Total:      %d\n", summary.Total)
	fmt.Printf("  Blocked:    %d\n", summary.Blocked)
	fmt.Printf("  Mismatched: %d\n", summary.Mismatched)
	if len(summary.Decisions) > 0 {
		fmt.Println("  Decisions:")
		for decision, count := range summary.Decisions {
			fmt.Printf("    - %s: %d\n", decision, count)
		}
	}
}

func writePolicyMatrixJUnit(path string, results []policyTestMatrixResult) error {
	suite := policyTestJUnitSuite{
		Name:  "relicta.policy.matrix",
		Tests: len(results),
		Cases: make([]policyTestJUnitCase, 0, len(results)),
	}

	for _, r := range results {
		tc := policyTestJUnitCase{
			Classname: "relicta.policy.matrix",
			Name:      r.Name,
		}
		reasons := make([]string, 0, 2)
		failureType := ""
		if r.Output.Blocked {
			reasons = append(reasons, "blocked")
			failureType = "blocked"
		}
		if r.AssertionDiff != nil && len(r.AssertionDiff.Mismatches) > 0 {
			reasons = append(reasons, fmt.Sprintf("assertion mismatches: %d", len(r.AssertionDiff.Mismatches)))
			if failureType == "" {
				failureType = "assertion_mismatch"
			}
		}
		if len(reasons) > 0 {
			suite.Failures++
			tc.Failure = &policyTestJUnitFailure{
				Type:    failureType,
				Message: strings.Join(reasons, "; "),
				Text:    buildPolicyJUnitFailureText(r),
			}
		}
		suite.Cases = append(suite.Cases, tc)
	}

	b, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JUnit XML: %w", err)
	}
	content := []byte(xml.Header + string(b) + "\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write JUnit file %q: %w", path, err)
	}
	return nil
}

func buildPolicyJUnitFailureText(r policyTestMatrixResult) string {
	lines := []string{
		fmt.Sprintf("scenario: %s", r.Name),
		fmt.Sprintf("decision: %s", r.Output.Decision),
		fmt.Sprintf("blocked: %t", r.Output.Blocked),
	}
	if r.Output.BlockReason != "" {
		lines = append(lines, fmt.Sprintf("block_reason: %s", r.Output.BlockReason))
	}
	if r.AssertionDiff != nil {
		for _, m := range r.AssertionDiff.Mismatches {
			lines = append(lines, fmt.Sprintf("mismatch %s expected=%v actual=%v", m.Field, m.Expected, m.Actual))
		}
	}
	return strings.Join(lines, "\n")
}

func writePolicyMatrixSummaryReport(path string, summary policyTestMatrixSummary, results []policyTestMatrixResult) error {
	report := policyTestCompactSummaryReport{
		Total:      summary.Total,
		Blocked:    summary.Blocked,
		Mismatched: summary.Mismatched,
		Decisions:  summary.Decisions,
	}
	for _, r := range results {
		mismatched := r.AssertionDiff != nil && len(r.AssertionDiff.Mismatches) > 0
		if r.Output.Blocked || mismatched {
			report.FailedScenarios = append(report.FailedScenarios, policyTestCompactScenario{
				Name:       r.Name,
				Decision:   string(r.Output.Decision),
				Blocked:    r.Output.Blocked,
				Mismatched: mismatched,
			})
		}
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary report JSON: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("failed to write summary report %q: %w", path, err)
	}
	return nil
}
