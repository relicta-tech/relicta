package cli

// policy_scaffold.go: extracted from policy.go to reduce that file's size and
// keep per-verb command logic navigable. Same package — uses the shared
// policyTest* types declared in policy.go (policyTestInputData, etc.).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// runPolicyScaffold seeds an input fixture and a matrix fixture from the
// configured policies. Used to bootstrap policy regression suites without
// hand-writing every scenario.
//
// Required flags: --input-out + --matrix-out. Optional: --dir / --file
// (mutually exclusive), --max-rule-scenarios, --force.
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

// ============================================================================
// Scaffold helpers — extracted from policy.go.
// These functions support runPolicyScaffold above; they live in the same
// package so internal types (policyTestInputData, policyTestMatrixCase,
// policyTestOutput) stay reachable without exporting.
// ============================================================================

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
