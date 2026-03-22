// Package supplychain provides dependency change analysis and governance
// for supply chain security. It extends CGP risk assessment to dependency
// updates, enabling governance decisions for changes like Dependabot PRs.
package supplychain

import (
	"fmt"
	"log/slog"
)

// ChangeType classifies the nature of a dependency version change.
type ChangeType string

const (
	// ChangePatch is a patch-level version bump (e.g. 1.2.3 -> 1.2.4).
	ChangePatch ChangeType = "patch"
	// ChangeMinor is a minor-level version bump (e.g. 1.2.0 -> 1.3.0).
	ChangeMinor ChangeType = "minor"
	// ChangeMajor is a major-level version bump (e.g. 1.0.0 -> 2.0.0).
	ChangeMajor ChangeType = "major"
	// ChangeNew indicates a newly added dependency.
	ChangeNew ChangeType = "new"
	// ChangeRemoved indicates a removed dependency.
	ChangeRemoved ChangeType = "removed"
)

// DependencyChange represents a single dependency update.
type DependencyChange struct {
	// Name is the module/package path (e.g. "google.golang.org/grpc").
	Name string
	// Ecosystem identifies the package manager (e.g. "go", "npm", "pip", "cargo").
	Ecosystem string
	// OldVersion is the previous version string. Empty for new dependencies.
	OldVersion string
	// NewVersion is the updated version string. Empty for removed dependencies.
	NewVersion string
	// ChangeType classifies the version change.
	ChangeType ChangeType
	// HasCVEFix indicates whether this update addresses a known vulnerability.
	HasCVEFix bool
	// CVEs lists specific CVE identifiers addressed by this update.
	CVEs []string
	// IsTransitive indicates whether this is an indirect/transitive dependency.
	IsTransitive bool
}

// RiskFactor describes a single component contributing to the overall risk score.
type RiskFactor struct {
	// Name identifies this risk factor.
	Name string
	// Score is the risk contribution (0.0-1.0).
	Score float64
	// Description provides a human-readable explanation.
	Description string
}

// Analysis is the result of analyzing a set of dependency changes.
type Analysis struct {
	// Changes is the list of dependency changes that were analyzed.
	Changes []DependencyChange
	// RiskScore is the overall risk score (0.0-1.0).
	RiskScore float64
	// RiskFactors lists the individual risk contributions.
	RiskFactors []RiskFactor
	// Recommendation is the governance recommendation: "auto-approve", "review", or "block".
	Recommendation string
	// Rationale explains the reasoning behind the recommendation.
	Rationale []string
}

// Analyzer evaluates dependency changes for governance decisions.
type Analyzer struct {
	logger *slog.Logger
}

// NewAnalyzer creates a new dependency change analyzer.
func NewAnalyzer(logger *slog.Logger) *Analyzer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Analyzer{
		logger: logger,
	}
}

// baseRiskScore returns the base risk score for a change type.
func baseRiskScore(ct ChangeType) float64 {
	switch ct {
	case ChangePatch:
		return 0.15
	case ChangeMinor:
		return 0.3
	case ChangeMajor:
		return 0.7
	case ChangeNew:
		return 0.5
	case ChangeRemoved:
		return 0.4
	default:
		return 0.3
	}
}

// Analyze evaluates a set of dependency changes and produces a risk analysis.
func (a *Analyzer) Analyze(changes []DependencyChange) *Analysis {
	result := &Analysis{
		Changes: changes,
	}

	if len(changes) == 0 {
		result.Recommendation = "auto-approve"
		result.Rationale = []string{"No dependency changes detected"}
		return result
	}

	totalScore := 0.0
	majorCount := 0

	for _, change := range changes {
		score := a.scoreChange(change)

		var factorName string
		var desc string

		switch {
		case change.HasCVEFix:
			factorName = "cve_fix"
			desc = fmt.Sprintf("%s: CVE fix (%s)", change.Name, formatCVEs(change.CVEs))
		default:
			factorName = string(change.ChangeType)
			desc = fmt.Sprintf("%s: %s update %s -> %s",
				change.Name, change.ChangeType, change.OldVersion, change.NewVersion)
			if change.ChangeType == ChangeNew {
				desc = fmt.Sprintf("%s: new dependency %s", change.Name, change.NewVersion)
			} else if change.ChangeType == ChangeRemoved {
				desc = fmt.Sprintf("%s: dependency removed (was %s)", change.Name, change.OldVersion)
			}
		}

		result.RiskFactors = append(result.RiskFactors, RiskFactor{
			Name:        factorName,
			Score:       score,
			Description: desc,
		})

		totalScore += score

		if change.ChangeType == ChangeMajor {
			majorCount++
		}
	}

	// Apply penalty for multiple simultaneous major updates.
	// The first major is already scored; each additional one adds +0.1.
	if majorCount > 1 {
		penalty := float64(majorCount-1) * 0.1
		totalScore += penalty
		result.RiskFactors = append(result.RiskFactors, RiskFactor{
			Name:        "multiple_majors_penalty",
			Score:       penalty,
			Description: fmt.Sprintf("%d simultaneous major updates (+%.1f penalty)", majorCount, penalty),
		})
	}

	// Overall risk is the weighted average across all changes,
	// including any penalty contributions.
	result.RiskScore = totalScore / float64(len(changes))
	if result.RiskScore > 1.0 {
		result.RiskScore = 1.0
	}

	// Determine recommendation based on risk score.
	switch {
	case result.RiskScore <= 0.2:
		result.Recommendation = "auto-approve"
		result.Rationale = append(result.Rationale, "Low risk dependency changes")
	case result.RiskScore <= 0.5:
		result.Recommendation = "review"
		result.Rationale = append(result.Rationale, "Moderate risk dependency changes require review")
	default:
		result.Recommendation = "block"
		result.Rationale = append(result.Rationale, "High risk dependency changes should be blocked until reviewed")
	}

	a.logger.Info("dependency analysis complete",
		"changes", len(changes),
		"risk_score", result.RiskScore,
		"recommendation", result.Recommendation,
	)

	return result
}

// scoreChange computes the risk score for a single dependency change.
func (a *Analyzer) scoreChange(change DependencyChange) float64 {
	// CVE fixes are inherently low risk regardless of version bump magnitude.
	if change.HasCVEFix {
		return 0.1
	}

	score := baseRiskScore(change.ChangeType)

	// Transitive dependencies carry lower risk since they are not
	// directly consumed by the project's code.
	if change.IsTransitive {
		score *= 0.5
	}

	return score
}

// formatCVEs produces a comma-separated string of CVE identifiers.
func formatCVEs(cves []string) string {
	if len(cves) == 0 {
		return "unspecified CVE"
	}
	result := cves[0]
	for i := 1; i < len(cves); i++ {
		result += ", " + cves[i]
	}
	return result
}
