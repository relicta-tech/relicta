package cli

import (
	"encoding/json"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(7), 7.0, true},
		{"int64", int64(42), 42.0, true},
		{"int32", int32(10), 10.0, true},
		{"json.Number float", json.Number("0.85"), 0.85, true},
		{"json.Number int", json.Number("42"), 42.0, true},
		{"string float", "0.75", 0.75, true},
		{"string int", "10", 10.0, true},
		{"string with spaces", " 3.5 ", 3.5, true},
		{"invalid string", "abc", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
		{"slice", []int{1}, 0, false},
		{"invalid json.Number", json.Number("nope"), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   int
		wantOK bool
	}{
		{"int", int(5), 5, true},
		{"int64", int64(99), 99, true},
		{"int32", int32(33), 33, true},
		{"float64", float64(7.9), 7, true},
		{"float32", float32(3.2), 3, true},
		{"json.Number int", json.Number("42"), 42, true},
		{"json.Number float", json.Number("3.7"), 3, true},
		{"string", "12", 12, true},
		{"string with spaces", " 8 ", 8, true},
		{"invalid string", "xyz", 0, false},
		{"bool", false, 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toInt(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toInt(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toInt(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{1, 2, 1},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
	}
	for _, tt := range tests {
		if got := minInt(tt.a, tt.b); got != tt.want {
			t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
	}
	for _, tt := range tests {
		if got := maxInt(tt.a, tt.b); got != tt.want {
			t.Errorf("maxInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMinFloat(t *testing.T) {
	tests := []struct{ a, b, want float64 }{
		{1.0, 2.0, 1.0},
		{5.5, 3.3, 3.3},
		{0.0, 0.0, 0.0},
	}
	for _, tt := range tests {
		if got := minFloat(tt.a, tt.b); got != tt.want {
			t.Errorf("minFloat(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMaxFloat(t *testing.T) {
	tests := []struct{ a, b, want float64 }{
		{1.0, 2.0, 2.0},
		{5.5, 3.3, 5.5},
		{0.0, 0.0, 0.0},
	}
	for _, tt := range tests {
		if got := maxFloat(tt.a, tt.b); got != tt.want {
			t.Errorf("maxFloat(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMatrixScenarioName(t *testing.T) {
	tests := []struct {
		name string
		c    policyTestMatrixCase
		idx  int
		want string
	}{
		{"named", policyTestMatrixCase{Name: "high-risk"}, 0, "high-risk"},
		{"named with spaces", policyTestMatrixCase{Name: "  trimmed  "}, 0, "trimmed"},
		{"unnamed index 0", policyTestMatrixCase{}, 0, "scenario-1"},
		{"unnamed index 4", policyTestMatrixCase{}, 4, "scenario-5"},
		{"empty string", policyTestMatrixCase{Name: ""}, 2, "scenario-3"},
		{"whitespace only", policyTestMatrixCase{Name: "   "}, 1, "scenario-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matrixScenarioName(tt.c, tt.idx)
			if got != tt.want {
				t.Errorf("matrixScenarioName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatrixScenarioShard(t *testing.T) {
	// deterministic: same name always yields same shard
	s1 := matrixScenarioShard("test-scenario", 4)
	s2 := matrixScenarioShard("test-scenario", 4)
	if s1 != s2 {
		t.Errorf("expected deterministic shard, got %d and %d", s1, s2)
	}
	// shard range: 1 <= shard <= total
	for _, name := range []string{"a", "b", "c", "alpha", "beta", "scenario-99"} {
		shard := matrixScenarioShard(name, 3)
		if shard < 1 || shard > 3 {
			t.Errorf("matrixScenarioShard(%q, 3) = %d, want 1-3", name, shard)
		}
	}
}

func TestSanitizeScenarioName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"Hello World", "hello-world"},
		{"check-risk", "check-risk"},
		{"  spaces  ", "spaces"},
		{"UPPER_CASE", "upper-case"},
		{"multi---dash", "multi-dash"},
		{"special!@#chars", "special-chars"},
		{"123numbers", "123numbers"},
		{"", ""},
		{"---", ""},
		{"a.b.c", "a-b-c"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeScenarioName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeScenarioName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePolicyTestBumpType(t *testing.T) {
	tests := []struct {
		input   string
		want    cgp.BumpType
		wantErr bool
	}{
		{"major", cgp.BumpTypeMajor, false},
		{"Minor", cgp.BumpTypeMinor, false},
		{"PATCH", cgp.BumpTypePatch, false},
		{" major ", cgp.BumpTypeMajor, false},
		{"invalid", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parsePolicyTestBumpType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePolicyTestBumpType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parsePolicyTestBumpType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestApplyPolicyTestDefaults(t *testing.T) {
	t.Run("fills empty fields", func(t *testing.T) {
		in := policyTestInputData{}
		out := applyPolicyTestDefaults(in)
		if out.BumpType != "patch" {
			t.Errorf("BumpType = %q, want %q", out.BumpType, "patch")
		}
		if out.ActorType != "human" {
			t.Errorf("ActorType = %q, want %q", out.ActorType, "human")
		}
		if out.ActorID != "human:policy-test" {
			t.Errorf("ActorID = %q, want %q", out.ActorID, "human:policy-test")
		}
		if out.Repository != "local/repo" {
			t.Errorf("Repository = %q, want %q", out.Repository, "local/repo")
		}
		if out.Branch != "main" {
			t.Errorf("Branch = %q, want %q", out.Branch, "main")
		}
	})

	t.Run("preserves existing values", func(t *testing.T) {
		in := policyTestInputData{
			BumpType:   "major",
			ActorType:  "agent",
			ActorID:    "bot:ci",
			Repository: "org/repo",
			Branch:     "develop",
			RiskScore:  0.9,
			Breaking:   2,
		}
		out := applyPolicyTestDefaults(in)
		if out.BumpType != "major" {
			t.Errorf("BumpType = %q, want %q", out.BumpType, "major")
		}
		if out.ActorType != "agent" {
			t.Errorf("ActorType = %q, want %q", out.ActorType, "agent")
		}
		if out.RiskScore != 0.9 {
			t.Errorf("RiskScore = %v, want %v", out.RiskScore, 0.9)
		}
		if out.Breaking != 2 {
			t.Errorf("Breaking = %d, want %d", out.Breaking, 2)
		}
	})
}

func TestUnmarshalPolicyInput(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		data := []byte(`{"risk_score": 0.5, "bump_type": "minor"}`)
		var out policyTestInputData
		if err := unmarshalPolicyInput("test.json", data, &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.RiskScore != 0.5 {
			t.Errorf("RiskScore = %v, want 0.5", out.RiskScore)
		}
		if out.BumpType != "minor" {
			t.Errorf("BumpType = %q, want %q", out.BumpType, "minor")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		data := []byte("risk_score: 0.8\nbump_type: major\n")
		var out policyTestInputData
		if err := unmarshalPolicyInput("test.yaml", data, &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.RiskScore != 0.8 {
			t.Errorf("RiskScore = %v, want 0.8", out.RiskScore)
		}
	})

	t.Run("yml extension", func(t *testing.T) {
		data := []byte("risk_score: 0.3\n")
		var out policyTestInputData
		if err := unmarshalPolicyInput("test.yml", data, &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.RiskScore != 0.3 {
			t.Errorf("RiskScore = %v, want 0.3", out.RiskScore)
		}
	})

	t.Run("auto-detect json from extensionless", func(t *testing.T) {
		data := []byte(`{"risk_score": 0.6}`)
		var out policyTestInputData
		if err := unmarshalPolicyInput("-", data, &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.RiskScore != 0.6 {
			t.Errorf("RiskScore = %v, want 0.6", out.RiskScore)
		}
	})

	t.Run("auto-detect yaml from extensionless", func(t *testing.T) {
		data := []byte("risk_score: 0.4\n")
		var out policyTestInputData
		if err := unmarshalPolicyInput("-", data, &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.RiskScore != 0.4 {
			t.Errorf("RiskScore = %v, want 0.4", out.RiskScore)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		data := []byte(`{bad json}`)
		var out policyTestInputData
		if err := unmarshalPolicyInput("test.json", data, &out); err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		data := []byte(":\n  :\n    - :\n      bad: [")
		var out policyTestInputData
		if err := unmarshalPolicyInput("test.yaml", data, &out); err == nil {
			t.Error("expected error for invalid YAML")
		}
	})

	t.Run("unparseable extensionless", func(t *testing.T) {
		data := []byte{0x00, 0x01, 0x02}
		var out policyTestInputData
		if err := unmarshalPolicyInput("-", data, &out); err == nil {
			t.Error("expected error for unparseable data")
		}
	})
}

func TestMarshalScaffoldFile(t *testing.T) {
	data := map[string]string{"key": "value"}

	t.Run("json", func(t *testing.T) {
		b, err := marshalScaffoldFile("out.json", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(b) == 0 {
			t.Error("expected non-empty output")
		}
		if b[len(b)-1] != '\n' {
			t.Error("expected trailing newline for JSON")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		b, err := marshalScaffoldFile("out.yaml", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(b) == 0 {
			t.Error("expected non-empty output")
		}
	})

	t.Run("yml", func(t *testing.T) {
		b, err := marshalScaffoldFile("out.yml", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(b) == 0 {
			t.Error("expected non-empty output")
		}
	})

	t.Run("empty extension defaults to json", func(t *testing.T) {
		b, err := marshalScaffoldFile("output", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(b) == 0 {
			t.Error("expected non-empty output")
		}
	})

	t.Run("unsupported extension", func(t *testing.T) {
		_, err := marshalScaffoldFile("out.xml", data)
		if err == nil {
			t.Error("expected error for unsupported extension")
		}
	})
}

func TestEqualStringSets(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", []string{}, []string{}, true},
		{"equal", []string{"a", "b"}, []string{"b", "a"}, true},
		{"different order same content", []string{"x", "y", "z"}, []string{"z", "x", "y"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"duplicates match", []string{"a", "a", "b"}, []string{"a", "b", "a"}, true},
		{"duplicates differ", []string{"a", "a"}, []string{"a", "b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalStringSets(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalStringSets(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestEqualRequiredActionSets(t *testing.T) {
	a1 := cgp.RequiredAction{Type: "review", Description: "desc", Assignee: "user1", Deadline: "2025-01-01"}
	a2 := cgp.RequiredAction{Type: "approval", Description: "desc2", Assignee: "user2", Deadline: "2025-02-01"}
	a3 := cgp.RequiredAction{Type: "different", Description: "other", Assignee: "user3", Deadline: "2025-03-01"}

	tests := []struct {
		name string
		a, b []cgp.RequiredAction
		want bool
	}{
		{"both nil", nil, nil, true},
		{"equal same order", []cgp.RequiredAction{a1, a2}, []cgp.RequiredAction{a1, a2}, true},
		{"equal different order", []cgp.RequiredAction{a1, a2}, []cgp.RequiredAction{a2, a1}, true},
		{"different length", []cgp.RequiredAction{a1}, []cgp.RequiredAction{a1, a2}, false},
		{"different content", []cgp.RequiredAction{a1, a2}, []cgp.RequiredAction{a1, a3}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalRequiredActionSets(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalRequiredActionSets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEqualConditionSets(t *testing.T) {
	c1 := cgp.Condition{Type: "time_window", Value: "business_hours"}
	c2 := cgp.Condition{Type: "feature_flag", Value: "enabled"}
	c3 := cgp.Condition{Type: "manual_gate", Value: "blocked"}

	tests := []struct {
		name string
		a, b []cgp.Condition
		want bool
	}{
		{"both nil", nil, nil, true},
		{"equal same order", []cgp.Condition{c1, c2}, []cgp.Condition{c1, c2}, true},
		{"equal different order", []cgp.Condition{c1, c2}, []cgp.Condition{c2, c1}, true},
		{"different length", []cgp.Condition{c1}, []cgp.Condition{c1, c2}, false},
		{"different content", []cgp.Condition{c1, c2}, []cgp.Condition{c1, c3}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalConditionSets(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalConditionSets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScaffoldStringValue(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   string
		wantOK bool
	}{
		{"string", "hello", "hello", true},
		{"string with spaces", " world ", "world", true},
		{"empty string", "", "", true},
		{"string slice", []string{"first", "second"}, "first", true},
		{"empty string slice", []string{}, "", false},
		{"any slice with strings", []any{"alpha", "beta"}, "alpha", true},
		{"any slice with empty string first", []any{"", "beta"}, "beta", true},
		{"any slice empty", []any{}, "", false},
		{"any slice non-string", []any{42, true}, "", false},
		{"int", 42, "", false},
		{"nil", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := scaffoldStringValue(tt.input)
			if ok != tt.wantOK {
				t.Errorf("scaffoldStringValue(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("scaffoldStringValue(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPolicyDecisionStrictnessRank(t *testing.T) {
	tests := []struct {
		decision cgp.DecisionType
		want     int
	}{
		{cgp.DecisionApproved, 0},
		{cgp.DecisionApprovalRequired, 1},
		{cgp.DecisionDeferred, 1},
		{cgp.DecisionRejected, 2},
		{cgp.DecisionType("unknown"), 0},
	}
	for _, tt := range tests {
		t.Run(string(tt.decision), func(t *testing.T) {
			got := policyDecisionStrictnessRank(tt.decision)
			if got != tt.want {
				t.Errorf("policyDecisionStrictnessRank(%q) = %d, want %d", tt.decision, got, tt.want)
			}
		})
	}
}

func TestComparisonDirectionLabel(t *testing.T) {
	tests := []struct {
		v    int
		want string
	}{
		{1, "stricter"},
		{5, "stricter"},
		{-1, "looser"},
		{-3, "looser"},
		{0, "same"},
	}
	for _, tt := range tests {
		got := comparisonDirectionLabel(tt.v)
		if got != tt.want {
			t.Errorf("comparisonDirectionLabel(%d) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestComparePolicyStrictness(t *testing.T) {
	tests := []struct {
		name      string
		baseline  policyTestOutput
		candidate policyTestOutput
		want      int
	}{
		{
			"candidate stricter by decision",
			policyTestOutput{Decision: cgp.DecisionApproved, RequiredApprovers: 0},
			policyTestOutput{Decision: cgp.DecisionRejected, RequiredApprovers: 0},
			1,
		},
		{
			"candidate looser by decision",
			policyTestOutput{Decision: cgp.DecisionRejected, RequiredApprovers: 0},
			policyTestOutput{Decision: cgp.DecisionApproved, RequiredApprovers: 0},
			-1,
		},
		{
			"same decision candidate stricter by approvers",
			policyTestOutput{Decision: cgp.DecisionApprovalRequired, RequiredApprovers: 1},
			policyTestOutput{Decision: cgp.DecisionApprovalRequired, RequiredApprovers: 3},
			1,
		},
		{
			"same decision candidate looser by approvers",
			policyTestOutput{Decision: cgp.DecisionApprovalRequired, RequiredApprovers: 3},
			policyTestOutput{Decision: cgp.DecisionApprovalRequired, RequiredApprovers: 1},
			-1,
		},
		{
			"equivalent",
			policyTestOutput{Decision: cgp.DecisionApproved, RequiredApprovers: 0},
			policyTestOutput{Decision: cgp.DecisionApproved, RequiredApprovers: 0},
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePolicyStrictness(&tt.baseline, &tt.candidate)
			if got != tt.want {
				t.Errorf("comparePolicyStrictness() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountPolicyComparisonDirections(t *testing.T) {
	results := []policyTestMatrixResult{
		{Comparison: &policyTestComparison{Direction: "stricter"}},
		{Comparison: &policyTestComparison{Direction: "looser"}},
		{Comparison: &policyTestComparison{Direction: "same"}},
		{Comparison: &policyTestComparison{Direction: "stricter"}},
		{Comparison: nil},
	}
	stricter, looser := countPolicyComparisonDirections(results)
	if stricter != 2 {
		t.Errorf("stricter = %d, want 2", stricter)
	}
	if looser != 1 {
		t.Errorf("looser = %d, want 1", looser)
	}
}

func TestBuildPolicyMatrixSummary(t *testing.T) {
	results := []policyTestMatrixResult{
		{
			Output:        policyTestOutput{Decision: cgp.DecisionApproved, Blocked: false},
			AssertionDiff: nil,
		},
		{
			Output:        policyTestOutput{Decision: cgp.DecisionRejected, Blocked: true},
			AssertionDiff: nil,
		},
		{
			Output: policyTestOutput{Decision: cgp.DecisionApproved, Blocked: false},
			AssertionDiff: &policyTestAssertionDiff{
				Mismatches: []policyTestAssertionMismatch{{Field: "decision"}},
			},
		},
	}
	summary := buildPolicyMatrixSummary(results)
	if summary.Total != 3 {
		t.Errorf("Total = %d, want 3", summary.Total)
	}
	if summary.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", summary.Blocked)
	}
	if summary.Mismatched != 1 {
		t.Errorf("Mismatched = %d, want 1", summary.Mismatched)
	}
	if summary.Decisions[string(cgp.DecisionApproved)] != 2 {
		t.Errorf("Decisions[approved] = %d, want 2", summary.Decisions[string(cgp.DecisionApproved)])
	}
	if summary.Decisions[string(cgp.DecisionRejected)] != 1 {
		t.Errorf("Decisions[rejected] = %d, want 1", summary.Decisions[string(cgp.DecisionRejected)])
	}
}

func TestBuildPolicyScenarioComparison(t *testing.T) {
	t.Run("nil inputs", func(t *testing.T) {
		got := buildPolicyScenarioComparison(nil, nil)
		if got != nil {
			t.Error("expected nil for nil inputs")
		}
		got = buildPolicyScenarioComparison(&policyTestOutput{}, nil)
		if got != nil {
			t.Error("expected nil for nil candidate")
		}
	})

	t.Run("no change", func(t *testing.T) {
		base := &policyTestOutput{Decision: cgp.DecisionApproved, Blocked: false, RequiredApprovers: 0}
		cand := &policyTestOutput{Decision: cgp.DecisionApproved, Blocked: false, RequiredApprovers: 0}
		got := buildPolicyScenarioComparison(base, cand)
		if got.Changed {
			t.Error("expected Changed=false")
		}
		if got.Direction != "same" {
			t.Errorf("Direction = %q, want %q", got.Direction, "same")
		}
	})

	t.Run("stricter", func(t *testing.T) {
		base := &policyTestOutput{Decision: cgp.DecisionApproved, Blocked: false, RequiredApprovers: 0}
		cand := &policyTestOutput{Decision: cgp.DecisionRejected, Blocked: true, RequiredApprovers: 0}
		got := buildPolicyScenarioComparison(base, cand)
		if !got.Changed {
			t.Error("expected Changed=true")
		}
		if got.Direction != "stricter" {
			t.Errorf("Direction = %q, want %q", got.Direction, "stricter")
		}
	})
}

func TestBuildPolicyJUnitFailureText(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		r := policyTestMatrixResult{
			Name:   "test-scenario",
			Output: policyTestOutput{Decision: cgp.DecisionRejected, Blocked: true, BlockReason: "too risky"},
		}
		text := buildPolicyJUnitFailureText(r)
		if text == "" {
			t.Error("expected non-empty text")
		}
		for _, want := range []string{"test-scenario", "rejected", "blocked: true", "block_reason: too risky"} {
			if !contains(text, want) {
				t.Errorf("expected text to contain %q, got %q", want, text)
			}
		}
	})

	t.Run("with assertion diff", func(t *testing.T) {
		r := policyTestMatrixResult{
			Name:   "diff-scenario",
			Output: policyTestOutput{Decision: cgp.DecisionApproved, Blocked: false},
			AssertionDiff: &policyTestAssertionDiff{
				Mismatches: []policyTestAssertionMismatch{
					{Field: "decision", Expected: "rejected", Actual: "approved"},
				},
			},
		}
		text := buildPolicyJUnitFailureText(r)
		if !contains(text, "mismatch decision") {
			t.Errorf("expected mismatch info, got %q", text)
		}
	})
}

func TestFilterPolicyExplainTrace(t *testing.T) {
	traces := []policy.RuleTrace{
		{RuleID: "r1", Matched: true},
		{RuleID: "r2", Matched: false},
		{RuleID: "r3", Matched: true},
	}

	t.Run("all", func(t *testing.T) {
		got, err := filterPolicyExplainTrace(traces, "all")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("len = %d, want 3", len(got))
		}
	})

	t.Run("empty string means all", func(t *testing.T) {
		got, err := filterPolicyExplainTrace(traces, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("len = %d, want 3", len(got))
		}
	})

	t.Run("matched", func(t *testing.T) {
		got, err := filterPolicyExplainTrace(traces, "matched")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		_, err := filterPolicyExplainTrace(traces, "invalid")
		if err == nil {
			t.Error("expected error for invalid mode")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
