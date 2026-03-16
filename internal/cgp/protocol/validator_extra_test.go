package protocol

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestValidator_ValidateDecision_StrictMode(t *testing.T) {
	v := NewValidator()
	v.StrictMode = true

	decision := cgp.NewDecision("prop-1", cgp.DecisionApproved)
	decision.RiskScore = 0.3
	decision.Rationale = []string{"Low risk change"}
	decision.RiskFactors = []cgp.RiskFactor{
		{Category: "size", Score: 0.2, Description: "small change"},
	}

	result := v.ValidateDecision(decision)
	if !result.Valid {
		t.Errorf("valid strict-mode decision should pass: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateDecision_StrictNoRationale(t *testing.T) {
	v := NewValidator()
	v.StrictMode = true

	decision := cgp.NewDecision("prop-1", cgp.DecisionApproved)
	decision.RiskScore = 0.3
	decision.Rationale = nil

	result := v.ValidateDecision(decision)
	if len(result.Warnings) == 0 {
		t.Error("strict mode should warn about missing rationale")
	}
}

func TestValidator_ValidateDecision_StrictInvalidRiskFactor(t *testing.T) {
	v := NewValidator()
	v.StrictMode = true

	decision := cgp.NewDecision("prop-1", cgp.DecisionApproved)
	decision.RiskScore = 0.3
	decision.RiskFactors = []cgp.RiskFactor{
		{Category: "invalid", Score: 1.5, Description: "out of range"},
	}

	result := v.ValidateDecision(decision)
	if result.Valid {
		t.Error("invalid risk factor score should cause validation error")
	}
}
