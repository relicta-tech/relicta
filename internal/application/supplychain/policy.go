package supplychain

// Decision represents the governance outcome for a supply chain analysis.
type Decision struct {
	// Action is the governance action: "approve", "review", or "block".
	Action string
	// Rationale explains the decision.
	Rationale []string
}

// Policy defines governance rules for dependency changes.
type Policy struct {
	// AutoApproveCVEFixes allows automatic approval of CVE fix updates.
	AutoApproveCVEFixes bool
	// AutoApprovePatch allows automatic approval of patch-level updates.
	AutoApprovePatch bool
	// AutoApproveMinor allows automatic approval of minor-level updates.
	AutoApproveMinor bool
	// RequireReviewForMajor forces review for major-level updates.
	RequireReviewForMajor bool
	// MaxRiskThreshold is the maximum risk score before blocking.
	MaxRiskThreshold float64
}

// DefaultPolicy returns the recommended supply chain governance policy.
func DefaultPolicy() Policy {
	return Policy{
		AutoApproveCVEFixes:   true,
		AutoApprovePatch:      true,
		AutoApproveMinor:      false,
		RequireReviewForMajor: true,
		MaxRiskThreshold:      0.5,
	}
}

// Evaluate applies the policy to an analysis result and returns a governance decision.
func (p Policy) Evaluate(analysis *Analysis) Decision {
	if analysis == nil || len(analysis.Changes) == 0 {
		return Decision{
			Action:    "approve",
			Rationale: []string{"No dependency changes to evaluate"},
		}
	}

	decision := Decision{
		Action: "approve",
	}

	hasMajor := false
	allCVEFixes := true
	allPatch := true
	allMinorOrLess := true

	for _, change := range analysis.Changes {
		if !change.HasCVEFix {
			allCVEFixes = false
		}
		if change.ChangeType != ChangePatch {
			allPatch = false
		}
		if change.ChangeType != ChangePatch && change.ChangeType != ChangeMinor {
			allMinorOrLess = false
		}
		if change.ChangeType == ChangeMajor {
			hasMajor = true
		}
	}

	// Check risk threshold first: if exceeded, block regardless of change types.
	if analysis.RiskScore > p.MaxRiskThreshold {
		decision.Action = "block"
		decision.Rationale = append(decision.Rationale,
			"Risk score exceeds maximum threshold")
		return decision
	}

	// CVE-only changes can be auto-approved if policy allows.
	if allCVEFixes && p.AutoApproveCVEFixes {
		decision.Action = "approve"
		decision.Rationale = append(decision.Rationale,
			"All changes are CVE fixes, auto-approved by policy")
		return decision
	}

	// Major updates require review if policy demands it.
	if hasMajor && p.RequireReviewForMajor {
		decision.Action = "review"
		decision.Rationale = append(decision.Rationale,
			"Major version update requires review")
		return decision
	}

	// Patch-only changes can be auto-approved if policy allows.
	if allPatch && p.AutoApprovePatch {
		decision.Action = "approve"
		decision.Rationale = append(decision.Rationale,
			"All changes are patch updates, auto-approved by policy")
		return decision
	}

	// Minor-or-less changes can be auto-approved if policy allows.
	if allMinorOrLess && p.AutoApproveMinor {
		decision.Action = "approve"
		decision.Rationale = append(decision.Rationale,
			"All changes are minor or patch updates, auto-approved by policy")
		return decision
	}

	// Default: require review for anything not explicitly auto-approved.
	decision.Action = "review"
	decision.Rationale = append(decision.Rationale,
		"Dependency changes require review per policy")
	return decision
}
