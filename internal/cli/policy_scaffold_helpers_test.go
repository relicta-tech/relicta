package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

func TestApplyConditionToInput(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		operator  string
		value     any
		mode      scaffoldSeedMode
		checkFunc func(t *testing.T, in *policyTestInputData)
	}{
		{
			"risk.score gt match",
			"risk.score", policy.OperatorGreaterThan, 0.5, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.RiskScore <= 0.5 {
					t.Errorf("RiskScore = %v, expected > 0.5", in.RiskScore)
				}
			},
		},
		{
			"risk_score alias",
			"risk_score", policy.OperatorGreaterThan, 0.5, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.RiskScore <= 0.5 {
					t.Errorf("RiskScore = %v, expected > 0.5", in.RiskScore)
				}
			},
		},
		{
			"change.bump_kind eq match",
			"change.bump_kind", policy.OperatorEqual, "major", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.BumpType != "major" {
					t.Errorf("BumpType = %q, want %q", in.BumpType, "major")
				}
			},
		},
		{
			"bump_type alias",
			"bump_type", policy.OperatorEqual, "minor", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.BumpType != "minor" {
					t.Errorf("BumpType = %q, want %q", in.BumpType, "minor")
				}
			},
		},
		{
			"actor.kind eq match",
			"actor.kind", policy.OperatorEqual, "agent", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.ActorType != "agent" {
					t.Errorf("ActorType = %q, want %q", in.ActorType, "agent")
				}
			},
		},
		{
			"actor_type alias",
			"actor_type", policy.OperatorEqual, "ci", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.ActorType != "ci" {
					t.Errorf("ActorType = %q, want %q", in.ActorType, "ci")
				}
			},
		},
		{
			"actor.id eq match",
			"actor.id", policy.OperatorEqual, "bot:ci", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.ActorID != "bot:ci" {
					t.Errorf("ActorID = %q, want %q", in.ActorID, "bot:ci")
				}
			},
		},
		{
			"scope.repository eq match",
			"scope.repository", policy.OperatorEqual, "org/app", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Repository != "org/app" {
					t.Errorf("Repository = %q, want %q", in.Repository, "org/app")
				}
			},
		},
		{
			"scope.branch eq match",
			"scope.branch", policy.OperatorEqual, "release", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Branch != "release" {
					t.Errorf("Branch = %q, want %q", in.Branch, "release")
				}
			},
		},
		{
			"change.breaking gt match",
			"change.breaking", policy.OperatorGreaterThan, 0, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Breaking < 1 {
					t.Errorf("Breaking = %d, expected >= 1", in.Breaking)
				}
			},
		},
		{
			"change.security gt match",
			"change.security", policy.OperatorGreaterThan, 0, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Security < 1 {
					t.Errorf("Security = %d, expected >= 1", in.Security)
				}
			},
		},
		{
			"change.features gte match",
			"change.features", policy.OperatorGreaterOrEqual, 3, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Features < 3 {
					t.Errorf("Features = %d, expected >= 3", in.Features)
				}
			},
		},
		{
			"change.fixes lt match",
			"change.fixes", policy.OperatorLessThan, 5, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Fixes >= 5 {
					t.Errorf("Fixes = %d, expected < 5", in.Fixes)
				}
			},
		},
		{
			"change.dependencies eq match",
			"change.dependencies", policy.OperatorEqual, 2, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.Dependencies != 2 {
					t.Errorf("Dependencies = %d, want 2", in.Dependencies)
				}
			},
		},
		{
			"blastradius.fileschanged gt match",
			"blastradius.fileschanged", policy.OperatorGreaterThan, 10, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.FilesChanged < 11 {
					t.Errorf("FilesChanged = %d, expected >= 11", in.FilesChanged)
				}
			},
		},
		{
			"files_changed alias",
			"files_changed", policy.OperatorGreaterThan, 5, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.FilesChanged < 6 {
					t.Errorf("FilesChanged = %d, expected >= 6", in.FilesChanged)
				}
			},
		},
		{
			"blastradius.lineschanged gte match",
			"blastradius.lineschanged", policy.OperatorGreaterOrEqual, 100, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.LinesChanged < 100 {
					t.Errorf("LinesChanged = %d, expected >= 100", in.LinesChanged)
				}
			},
		},
		{
			"lines_changed alias",
			"lines_changed", policy.OperatorGreaterOrEqual, 50, scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				if in.LinesChanged < 50 {
					t.Errorf("LinesChanged = %d, expected >= 50", in.LinesChanged)
				}
			},
		},
		{
			"unknown field is ignored",
			"unknown.field", policy.OperatorEqual, "x", scaffoldSeedMatch,
			func(t *testing.T, in *policyTestInputData) {
				// should not modify anything - just verify it doesn't panic
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &policyTestInputData{}
			cond := policy.Condition{Field: tt.field, Operator: tt.operator, Value: tt.value}
			applyConditionToInput(in, cond, tt.mode)
			tt.checkFunc(t, in)
		})
	}
}

func TestSetScaffoldFloat(t *testing.T) {
	tests := []struct {
		name     string
		initial  float64
		operator string
		value    any
		mode     scaffoldSeedMode
		check    func(t *testing.T, got float64)
	}{
		{
			"gt match", 0.0, policy.OperatorGreaterThan, 0.5, scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got <= 0.5 {
					t.Errorf("got %v, expected > 0.5", got)
				}
			},
		},
		{
			"gt inverse", 0.8, policy.OperatorGreaterThan, 0.5, scaffoldSeedInverse,
			func(t *testing.T, got float64) {
				if got > 0.5 {
					t.Errorf("got %v, expected <= 0.5", got)
				}
			},
		},
		{
			"gte match", 0.0, policy.OperatorGreaterOrEqual, 0.7, scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got < 0.7 {
					t.Errorf("got %v, expected >= 0.7", got)
				}
			},
		},
		{
			"gte inverse", 0.9, policy.OperatorGreaterOrEqual, 0.7, scaffoldSeedInverse,
			func(t *testing.T, got float64) {
				if got >= 0.7 {
					t.Errorf("got %v, expected < 0.7", got)
				}
			},
		},
		{
			"lt match", 0.8, policy.OperatorLessThan, 0.5, scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got >= 0.5 {
					t.Errorf("got %v, expected < 0.5", got)
				}
			},
		},
		{
			"lt inverse", 0.2, policy.OperatorLessThan, 0.5, scaffoldSeedInverse,
			func(t *testing.T, got float64) {
				if got < 0.5 {
					t.Errorf("got %v, expected >= 0.5", got)
				}
			},
		},
		{
			"lte match", 0.8, policy.OperatorLessOrEqual, 0.5, scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got > 0.5 {
					t.Errorf("got %v, expected <= 0.5", got)
				}
			},
		},
		{
			"lte inverse", 0.2, policy.OperatorLessOrEqual, 0.5, scaffoldSeedInverse,
			func(t *testing.T, got float64) {
				if got <= 0.5 {
					t.Errorf("got %v, expected > 0.5", got)
				}
			},
		},
		{
			"eq match", 0.0, policy.OperatorEqual, 0.75, scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got != 0.75 {
					t.Errorf("got %v, expected 0.75", got)
				}
			},
		},
		{
			"eq inverse leaves unchanged", 0.3, policy.OperatorEqual, 0.75, scaffoldSeedInverse,
			func(t *testing.T, got float64) {
				// eq with inverse should not set the value, but the clamping at the end still runs
				if got < 0 || got > 1 {
					t.Errorf("got %v, expected within [0, 1]", got)
				}
			},
		},
		{
			"invalid value skipped", 0.5, policy.OperatorGreaterThan, "notanumber", scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got != 0.5 {
					t.Errorf("got %v, expected unchanged 0.5", got)
				}
			},
		},
		{
			"result clamped to 0-1", 0.0, policy.OperatorGreaterThan, 0.99, scaffoldSeedMatch,
			func(t *testing.T, got float64) {
				if got < 0 || got > 1 {
					t.Errorf("got %v, expected within [0, 1]", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.initial
			cond := policy.Condition{Operator: tt.operator, Value: tt.value}
			setScaffoldFloat(&dst, cond, tt.mode)
			tt.check(t, dst)
		})
	}
}

func TestSetScaffoldInt(t *testing.T) {
	tests := []struct {
		name     string
		initial  int
		operator string
		value    any
		mode     scaffoldSeedMode
		check    func(t *testing.T, got int)
	}{
		{
			"gt match", 0, policy.OperatorGreaterThan, 5, scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got <= 5 {
					t.Errorf("got %d, expected > 5", got)
				}
			},
		},
		{
			"gt inverse", 10, policy.OperatorGreaterThan, 5, scaffoldSeedInverse,
			func(t *testing.T, got int) {
				if got > 5 {
					t.Errorf("got %d, expected <= 5", got)
				}
			},
		},
		{
			"gte match", 0, policy.OperatorGreaterOrEqual, 3, scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got < 3 {
					t.Errorf("got %d, expected >= 3", got)
				}
			},
		},
		{
			"gte inverse", 10, policy.OperatorGreaterOrEqual, 3, scaffoldSeedInverse,
			func(t *testing.T, got int) {
				if got >= 3 {
					t.Errorf("got %d, expected < 3", got)
				}
			},
		},
		{
			"lt match", 10, policy.OperatorLessThan, 5, scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got >= 5 {
					t.Errorf("got %d, expected < 5", got)
				}
			},
		},
		{
			"lt inverse", 2, policy.OperatorLessThan, 5, scaffoldSeedInverse,
			func(t *testing.T, got int) {
				if got < 5 {
					t.Errorf("got %d, expected >= 5", got)
				}
			},
		},
		{
			"lte match", 10, policy.OperatorLessOrEqual, 5, scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got > 5 {
					t.Errorf("got %d, expected <= 5", got)
				}
			},
		},
		{
			"lte inverse", 2, policy.OperatorLessOrEqual, 5, scaffoldSeedInverse,
			func(t *testing.T, got int) {
				if got <= 5 {
					t.Errorf("got %d, expected > 5", got)
				}
			},
		},
		{
			"eq match", 0, policy.OperatorEqual, 7, scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got != 7 {
					t.Errorf("got %d, expected 7", got)
				}
			},
		},
		{
			"eq inverse leaves unchanged", 3, policy.OperatorEqual, 7, scaffoldSeedInverse,
			func(t *testing.T, got int) {
				if got != 3 {
					t.Errorf("got %d, expected unchanged 3", got)
				}
			},
		},
		{
			"invalid value skipped", 5, policy.OperatorGreaterThan, "notanumber", scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got != 5 {
					t.Errorf("got %d, expected unchanged 5", got)
				}
			},
		},
		{
			"result non-negative", 0, policy.OperatorLessThan, 0, scaffoldSeedMatch,
			func(t *testing.T, got int) {
				if got < 0 {
					t.Errorf("got %d, expected >= 0", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.initial
			cond := policy.Condition{Operator: tt.operator, Value: tt.value}
			setScaffoldInt(&dst, cond, tt.mode)
			tt.check(t, dst)
		})
	}
}

func TestSetScaffoldString(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		operator string
		value    any
		mode     scaffoldSeedMode
		want     string
	}{
		{"eq match", "", policy.OperatorEqual, "hello", scaffoldSeedMatch, "hello"},
		{"eq inverse", "", policy.OperatorEqual, "hello", scaffoldSeedInverse, "other"},
		{"ne match", "", policy.OperatorNotEqual, "hello", scaffoldSeedMatch, "other"},
		{"ne inverse", "", policy.OperatorNotEqual, "hello", scaffoldSeedInverse, "hello"},
		{"contains match", "", policy.OperatorContains, "main", scaffoldSeedMatch, "main"},
		{"contains inverse", "", policy.OperatorContains, "main", scaffoldSeedInverse, "other"},
		{"matches match", "", policy.OperatorMatches, "release", scaffoldSeedMatch, "release"},
		{"matches inverse", "", policy.OperatorMatches, "release", scaffoldSeedInverse, "other"},
		{"in match", "", policy.OperatorIn, "alpha", scaffoldSeedMatch, "alpha"},
		{"in inverse", "", policy.OperatorIn, "alpha", scaffoldSeedInverse, "other"},
		{"invalid value", "orig", policy.OperatorEqual, 42, scaffoldSeedMatch, "orig"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.initial
			cond := policy.Condition{Operator: tt.operator, Value: tt.value}
			setScaffoldString(&dst, cond, tt.mode)
			if dst != tt.want {
				t.Errorf("setScaffoldString() dst = %q, want %q", dst, tt.want)
			}
		})
	}
}

func TestSetScaffoldBumpType(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		operator string
		value    any
		mode     scaffoldSeedMode
		want     string
	}{
		{"eq major match", "", policy.OperatorEqual, "major", scaffoldSeedMatch, "major"},
		{"eq minor match", "", policy.OperatorEqual, "minor", scaffoldSeedMatch, "minor"},
		{"eq patch match", "", policy.OperatorEqual, "patch", scaffoldSeedMatch, "patch"},
		{"eq major inverse", "", policy.OperatorEqual, "major", scaffoldSeedInverse, "patch"},
		{"eq minor inverse", "", policy.OperatorEqual, "minor", scaffoldSeedInverse, "patch"},
		{"eq patch inverse", "", policy.OperatorEqual, "patch", scaffoldSeedInverse, "major"},
		{"ne patch match", "", policy.OperatorNotEqual, "patch", scaffoldSeedMatch, "major"},
		{"ne major match", "", policy.OperatorNotEqual, "major", scaffoldSeedMatch, "patch"},
		{"ne minor match", "", policy.OperatorNotEqual, "minor", scaffoldSeedMatch, "patch"},
		{"ne major inverse", "", policy.OperatorNotEqual, "major", scaffoldSeedInverse, "major"},
		{"ne patch inverse", "", policy.OperatorNotEqual, "patch", scaffoldSeedInverse, "patch"},
		{"in match", "", policy.OperatorIn, "major", scaffoldSeedMatch, "major"},
		{"in inverse", "", policy.OperatorIn, "major", scaffoldSeedInverse, "patch"},
		{"contains match", "", policy.OperatorContains, "minor", scaffoldSeedMatch, "minor"},
		{"matches match", "", policy.OperatorMatches, "patch", scaffoldSeedMatch, "patch"},
		{"invalid value", "orig", policy.OperatorEqual, 42, scaffoldSeedMatch, "orig"},
		{"invalid bump value not set", "", policy.OperatorEqual, "unknown", scaffoldSeedMatch, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.initial
			cond := policy.Condition{Operator: tt.operator, Value: tt.value}
			setScaffoldBumpType(&dst, cond, tt.mode)
			if dst != tt.want {
				t.Errorf("setScaffoldBumpType() dst = %q, want %q", dst, tt.want)
			}
		})
	}
}

func TestSetScaffoldActorType(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		operator string
		value    any
		mode     scaffoldSeedMode
		want     string
	}{
		{"eq human match", "", policy.OperatorEqual, "human", scaffoldSeedMatch, "human"},
		{"eq agent match", "", policy.OperatorEqual, "agent", scaffoldSeedMatch, "agent"},
		{"eq ci match", "", policy.OperatorEqual, "ci", scaffoldSeedMatch, "ci"},
		{"eq system match", "", policy.OperatorEqual, "system", scaffoldSeedMatch, "system"},
		{"eq human inverse", "", policy.OperatorEqual, "human", scaffoldSeedInverse, "agent"},
		{"eq agent inverse", "", policy.OperatorEqual, "agent", scaffoldSeedInverse, "human"},
		{"eq ci inverse", "", policy.OperatorEqual, "ci", scaffoldSeedInverse, "human"},
		{"eq system inverse", "", policy.OperatorEqual, "system", scaffoldSeedInverse, "human"},
		{"eq unknown inverse", "", policy.OperatorEqual, "unknown", scaffoldSeedInverse, "human"},
		{"ne human match", "", policy.OperatorNotEqual, "human", scaffoldSeedMatch, "agent"},
		{"ne agent match", "", policy.OperatorNotEqual, "agent", scaffoldSeedMatch, "human"},
		{"ne ci match", "", policy.OperatorNotEqual, "ci", scaffoldSeedMatch, "human"},
		{"ne human inverse", "", policy.OperatorNotEqual, "human", scaffoldSeedInverse, "human"},
		{"ne agent inverse", "", policy.OperatorNotEqual, "agent", scaffoldSeedInverse, "agent"},
		{"in match", "", policy.OperatorIn, "ci", scaffoldSeedMatch, "ci"},
		{"in inverse", "", policy.OperatorIn, "ci", scaffoldSeedInverse, "human"},
		{"contains match", "", policy.OperatorContains, "system", scaffoldSeedMatch, "system"},
		{"matches match", "", policy.OperatorMatches, "agent", scaffoldSeedMatch, "agent"},
		{"invalid value", "orig", policy.OperatorEqual, 42, scaffoldSeedMatch, "orig"},
		{"invalid actor value not set", "", policy.OperatorEqual, "unknown", scaffoldSeedMatch, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := tt.initial
			cond := policy.Condition{Operator: tt.operator, Value: tt.value}
			setScaffoldActorType(&dst, cond, tt.mode)
			if dst != tt.want {
				t.Errorf("setScaffoldActorType() dst = %q, want %q", dst, tt.want)
			}
		})
	}
}
