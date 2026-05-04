package cli

// policy_matrix.go: matrix verb implementation — runPolicyTestMatrix and
// the helpers exclusive to it. Extracted from policy.go to reduce that
// file's size; same package so internal helpers (loadPoliciesForTest,
// applyPolicyTestDefaults, evaluatePolicyScenario, etc.) remain reachable.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"os"
	"path"
	"strings"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// matrixScenarioName resolves a scenario's display name. Falls back to a
// stable indexed name when the scenario doesn't declare one.
func matrixScenarioName(c policyTestMatrixCase, idx int) string {
	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = fmt.Sprintf("scenario-%d", idx+1)
	}
	return name
}

// matrixScenarioShard maps a scenario name onto a 1..total shard via fnv32a
// so distributed runners can deterministically partition matrix execution.
func matrixScenarioShard(name string, total int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32()%uint32(total)) + 1
}

// loadPolicyTestMatrix reads + parses a matrix fixture file (YAML or JSON).
// Returns the scenarios in declaration order.
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

// buildPolicyMatrixSummary aggregates per-scenario results into the summary
// shape rendered as text + JSON output. AssertionDiff with mismatches counts
// toward the Mismatched total; Output.Blocked counts toward Blocked.
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

// printPolicyMatrixSummaryText renders the summary in the human-readable
// trailing block of `relicta policy test --matrix ...`.
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

// runPolicyTestMatrix evaluates a matrix fixture file against the configured
// policies. Supports baseline-vs-candidate comparison, scenario filtering
// (include/exclude by name + tag), shard partitioning, JUnit + summary
// report emission, and assertion diffing against expected outputs.
//
// This is the workhorse of `relicta policy test --matrix`. Long because it
// orchestrates many cross-cutting concerns (compare mode, sharding, output
// modes, assertion checks, fail-on-* gates); each branch corresponds to a
// documented flag.
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

// ============================================================================
// Matrix output / comparison helpers — extracted from policy.go.
// ============================================================================

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

// buildPolicyMatrixSummary implemented in policy_matrix.go.

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

// printPolicyMatrixSummaryText implemented in policy_matrix.go.

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
