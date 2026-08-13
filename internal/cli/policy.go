package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
	"github.com/relicta-tech/relicta/v4/internal/cli/templates"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage governance policies",
	Long: `Manage CGP (Change Governance Protocol) policies.

Policies define rules for evaluating release changes, determining
risk levels, and requiring approvals based on configurable conditions.

Examples:
  # Write a starting policy (governance is on by default; this customizes it)
  relicta policy init

  # See the policy templates included in the binary
  relicta policy init --list

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
	policyValidateStrict            bool
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
	policyTestTrustLevel   string
	policyTestReputation   float64
	policyTestRepSamples   int
	policyTestRepTrend     string
	policyTestRepository   string
	policyTestBranch       string
	policyTestBreaking     int
	policyTestSecurity     int
	policyTestFeatures     int
	policyTestFixes        int
	policyTestDependencies int
	policyTestFilesChanged int
	policyTestFiles        []string
	policyTestLinesChanged int

	policyScaffoldDir              string
	policyScaffoldFile             string
	policyScaffoldInputOut         string
	policyScaffoldMatrixOut        string
	policyScaffoldForce            bool
	policyScaffoldMaxRuleScenarios int
)

var policyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starting policy",
	Long: `Write one of the example policies included in this binary into your
policy directory, so you have working DSL to edit rather than a blank file.

Governance runs on built-in defaults without any policy file. A policy is how
you encode rules those defaults do not cover — a longer freeze window, stricter
handling of agent-authored changes, a team that reviews its own releases.

The file is written to the first directory relicta searches, so it takes effect
immediately. Use --list to see what is available and --template to choose.`,
	RunE: runPolicyInit,
}

var policyFieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List the fields a policy condition can test",
	Long: `Print every field the policy evaluator can resolve, grouped by context.

A condition on a field the evaluator does not provide makes its rule silently
unmatchable — the rule loads, lists itself as enabled, and never fires. This is
the list to write conditions against; 'relicta policy validate' checks a policy
against the same list.`,
	RunE: runPolicyFields,
}

func init() {
	policyCmd.AddCommand(policyInitCmd)
	policyCmd.AddCommand(policyFieldsCmd)
	policyCmd.AddCommand(policyValidateCmd)
	policyCmd.AddCommand(policyListCmd)
	policyCmd.AddCommand(policyTestCmd)
	policyCmd.AddCommand(policyScaffoldCmd)

	policyInitCmd.Flags().StringVarP(&policyInitTemplate, "template", "t", templates.DefaultPolicyStarter,
		"which included policy to write (see --list)")
	policyInitCmd.Flags().StringVarP(&policyInitDir, "dir", "d", "",
		"directory to write into (default: the first directory relicta searches)")
	policyInitCmd.Flags().BoolVar(&policyInitForce, "force", false, "overwrite an existing policy file")
	policyInitCmd.Flags().BoolVar(&policyInitList, "list", false, "list the policy templates included in this binary")

	policyValidateCmd.Flags().StringVarP(&policyValidateDir, "dir", "d", "", "directory containing policy files")
	policyValidateCmd.Flags().BoolVar(&policyValidateStrict, "strict", false,
		"fail when a condition references a field the evaluator does not provide")
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
	// Without this a rule conditioning on actor.trusted could be written and
	// validated but never exercised: every test actor would be Limited, so the
	// clause would silently evaluate false and the rule would look inert.
	policyTestCmd.Flags().StringVar(&policyTestTrustLevel, "trust-level", "limited",
		"actor trust level: untrusted, limited, trusted, full")
	// Reputation has no default on purpose: unset means the scenario has no
	// computed reputation, which is what a deployment without release history
	// looks like. Passing the flag is how an author exercises a rule that reads it
	// — otherwise the rule can only ever report a missing field.
	policyTestCmd.Flags().Float64Var(&policyTestReputation, "reputation", 0,
		"actor reputation 0.0-1.0 (unset: no reputation computed, as in a repo without history)")
	policyTestCmd.Flags().IntVar(&policyTestRepSamples, "reputation-samples", 0,
		"release records behind --reputation")
	policyTestCmd.Flags().StringVar(&policyTestRepTrend, "reputation-trend", "stable",
		"reputation trend for --reputation: improving, stable, declining")
	policyTestCmd.Flags().StringVar(&policyTestRepository, "repository", "local/repo", "repository identifier")
	policyTestCmd.Flags().StringVar(&policyTestBranch, "branch", "main", "branch name")
	policyTestCmd.Flags().IntVar(&policyTestBreaking, "breaking", 0, "breaking change count")
	policyTestCmd.Flags().IntVar(&policyTestSecurity, "security", 0, "security change count")
	policyTestCmd.Flags().IntVar(&policyTestFeatures, "features", 0, "feature change count")
	policyTestCmd.Flags().IntVar(&policyTestFixes, "fixes", 0, "fix change count")
	policyTestCmd.Flags().IntVar(&policyTestDependencies, "dependencies", 0, "dependency change count")
	policyTestCmd.Flags().IntVar(&policyTestFilesChanged, "files-changed", 0, "changed files count")
	policyTestCmd.Flags().StringSliceVar(&policyTestFiles, "files", nil,
		"changed file paths, so path- and breadth-conditioned rules can be exercised "+
			"(e.g. --files internal/a.go,web/b.ts)")
	policyTestCmd.Flags().IntVar(&policyTestLinesChanged, "lines-changed", 0, "changed lines count")

	policyScaffoldCmd.Flags().StringVarP(&policyScaffoldDir, "dir", "d", "", "directory containing policy files")
	policyScaffoldCmd.Flags().StringVarP(&policyScaffoldFile, "file", "f", "", "single policy file to inspect")
	policyScaffoldCmd.Flags().StringVar(&policyScaffoldInputOut, "input-out", "policy-input.json", "output path for single-input fixture (.json or .yaml)")
	policyScaffoldCmd.Flags().StringVar(&policyScaffoldMatrixOut, "matrix-out", "policy-matrix.yaml", "output path for matrix fixture (.json or .yaml)")
	policyScaffoldCmd.Flags().BoolVar(&policyScaffoldForce, "force", false, "overwrite output files if they already exist")
	policyScaffoldCmd.Flags().IntVar(&policyScaffoldMaxRuleScenarios, "max-rule-scenarios", 8, "maximum number of per-rule scenarios to include in matrix")
}

// runPolicyValidate is implemented in policy_validate.go.
// runPolicyList is implemented in policy_list.go.

// runPolicyScaffold is implemented in policy_scaffold.go.

type policyTestInputData struct {
	RiskScore  float64 `json:"risk_score" yaml:"risk_score"`
	BumpType   string  `json:"bump_type" yaml:"bump_type"`
	ActorType  string  `json:"actor_type" yaml:"actor_type"`
	ActorID    string  `json:"actor_id" yaml:"actor_id"`
	TrustLevel string  `json:"trust_level,omitempty" yaml:"trust_level,omitempty"`
	// Reputation is a pointer because absent and 0.0 are different scenarios: 0.0
	// is an actor with a demonstrably bad record, absent is a repository where
	// nothing computes reputation. Collapsing them would make a rule on
	// actor.reputation.* look like it fires everywhere.
	Reputation        *float64 `json:"reputation,omitempty" yaml:"reputation,omitempty"`
	ReputationSamples int      `json:"reputation_samples,omitempty" yaml:"reputation_samples,omitempty"`
	ReputationTrend   string   `json:"reputation_trend,omitempty" yaml:"reputation_trend,omitempty"`
	Repository        string   `json:"repository" yaml:"repository"`
	Branch            string   `json:"branch" yaml:"branch"`
	Breaking          int      `json:"breaking" yaml:"breaking"`
	Security          int      `json:"security" yaml:"security"`
	Features          int      `json:"features" yaml:"features"`
	Fixes             int      `json:"fixes" yaml:"fixes"`
	Dependencies      int      `json:"dependencies" yaml:"dependencies"`
	FilesChanged      int      `json:"files_changed" yaml:"files_changed"`
	LinesChanged      int      `json:"lines_changed" yaml:"lines_changed"`

	// Files are the changed paths, not only how many.
	//
	// Path-conditioned rules could not be exercised here at all: `scope.files contains
	// "terraform/"` and the breadth fields derived from the same paths both need the
	// paths themselves, and this harness carried a count. So the rules a team is most
	// likely to get wrong were the ones they could not test — the same reason
	// --trust-level was added.
	Files []string `json:"files,omitempty" yaml:"files,omitempty"`
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
	// Limited is what a real invocation starts an actor at (see createCGPActor):
	// may propose, may not auto-approve. Trust is never inferred, so the default
	// here has to be the un-elevated one.
	if out.TrustLevel == "" {
		out.TrustLevel = cgp.TrustLevelLimited.String()
	}
	if out.Repository == "" {
		out.Repository = "local/repo"
	}
	if out.Branch == "" {
		out.Branch = "main"
	}
	return out
}

// loadPolicyTestMatrix implemented in policy_matrix.go.

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
