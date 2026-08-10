package cli

// policy_test_cmd.go: `relicta policy test` command implementation.
// Filename uses `_test_cmd.go` (NOT `_test.go`) so the Go test runner does
// not treat it as a test file — these are production CLI handlers, not unit
// tests.
//
// Extracted from policy.go to reduce that file's size. Shared helpers
// (loadPoliciesForTest, parsePolicyTestBumpType, applyPolicyTestDefaults,
// the policyTest* type set) live in policy.go in the same package.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/reputation"
)

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

// matrixScenarioName + matrixScenarioShard implemented in policy_matrix.go.

func evaluatePolicyScenario(ctx context.Context, policies []policy.Policy, input *policyTestInputData) (*policyTestOutput, error) {
	actorKind, ok := cgp.ParseActorKind(input.ActorType)
	if !ok {
		return nil, fmt.Errorf("invalid actor type %q (supported: human, agent, ci, system)", input.ActorType)
	}

	bumpType, err := parsePolicyTestBumpType(input.BumpType)
	if err != nil {
		return nil, err
	}

	trustLevel, ok := cgp.ParseTrustLevel(input.TrustLevel)
	if !ok {
		return nil, fmt.Errorf("invalid trust level %q (supported: untrusted, limited, trusted, full)", input.TrustLevel)
	}

	actorReputation, err := policyTestReputationContext(input)
	if err != nil {
		return nil, err
	}

	proposal := cgp.NewProposal(
		cgp.Actor{
			Kind:       actorKind,
			ID:         input.ActorID,
			Name:       input.ActorID,
			TrustLevel: trustLevel,
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
	if actorReputation != nil {
		proposal.Context = &cgp.ProposalContext{ActorReputation: actorReputation}
	}

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

// policyTestReputationFlag returns the --reputation value only when it was
// actually passed. A default value here would claim a reputation the scenario
// never stated, which is the difference between "this actor has no record" and
// "this actor has a perfect/terrible one".
func policyTestReputationFlag(cmd *cobra.Command) *float64 {
	if cmd == nil || !cmd.Flags().Changed("reputation") {
		return nil
	}
	value := policyTestReputation
	return &value
}

// policyTestReputationContext builds the reputation a scenario claims for its
// actor, or nil when the scenario claims none.
//
// Nil is the honest default: `relicta policy test` has no memory store, so a
// scenario that says nothing about reputation is a repository where reputation is
// not computed, and a condition on actor.reputation.* must report itself as
// missing there rather than comparing against a fabricated zero. The level is
// derived from the same thresholds the reputation engine uses, so a scenario
// cannot claim a score and a band that disagree.
func policyTestReputationContext(input *policyTestInputData) (*cgp.ActorReputation, error) {
	if input.Reputation == nil {
		return nil, nil
	}

	overall := *input.Reputation
	if overall < 0 || overall > 1 {
		return nil, fmt.Errorf("reputation must be between 0.0 and 1.0, got %v", overall)
	}
	if input.ReputationSamples < 0 {
		return nil, fmt.Errorf("reputation samples cannot be negative, got %d", input.ReputationSamples)
	}

	trend := reputation.Trend(strings.ToLower(strings.TrimSpace(input.ReputationTrend)))
	switch trend {
	case "":
		trend = reputation.TrendStable
	case reputation.TrendImproving, reputation.TrendStable, reputation.TrendDeclining:
	default:
		return nil, fmt.Errorf("invalid reputation trend %q (supported: improving, stable, declining)", input.ReputationTrend)
	}

	score := reputation.Score{Overall: overall, SampleSize: input.ReputationSamples, Trend: trend}
	return &cgp.ActorReputation{
		Overall:    score.Overall,
		Level:      score.Level(),
		SampleSize: score.SampleSize,
		Trend:      string(score.Trend),
	}, nil
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
		RiskScore:         policyTestRiskScore,
		BumpType:          policyTestBumpType,
		ActorType:         policyTestActorType,
		ActorID:           policyTestActorID,
		TrustLevel:        policyTestTrustLevel,
		Reputation:        policyTestReputationFlag(cmd),
		ReputationSamples: policyTestRepSamples,
		ReputationTrend:   policyTestRepTrend,
		Repository:        policyTestRepository,
		Branch:            policyTestBranch,
		Breaking:          policyTestBreaking,
		Security:          policyTestSecurity,
		Features:          policyTestFeatures,
		Fixes:             policyTestFixes,
		Dependencies:      policyTestDependencies,
		FilesChanged:      policyTestFilesChanged,
		LinesChanged:      policyTestLinesChanged,
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
	if cmd.Flags().Changed("trust-level") {
		merged.TrustLevel = policyTestTrustLevel
	}
	if cmd.Flags().Changed("reputation") {
		merged.Reputation = policyTestReputationFlag(cmd)
	}
	if cmd.Flags().Changed("reputation-samples") {
		merged.ReputationSamples = policyTestRepSamples
	}
	if cmd.Flags().Changed("reputation-trend") {
		merged.ReputationTrend = policyTestRepTrend
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
