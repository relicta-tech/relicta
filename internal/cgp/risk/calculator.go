// Package risk implements CGP risk assessment for change proposals.
//
// The risk calculator evaluates multiple factors to produce a normalized
// risk score (0.0-1.0) that informs governance decisions.
package risk

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// Calculator computes risk scores for changes.
type Calculator struct {
	weights WeightConfig
	history HistoryProvider
}

// WeightConfig defines the contribution of each factor to overall risk.
// All weights should sum to approximately 1.0 for balanced scoring.
type WeightConfig struct {
	APIChanges       float64 `json:"apiChanges" yaml:"apiChanges"`
	DependencyImpact float64 `json:"dependencyImpact" yaml:"dependencyImpact"`
	BlastRadius      float64 `json:"blastRadius" yaml:"blastRadius"`
	CodeComplexity   float64 `json:"codeComplexity" yaml:"codeComplexity"`
	TestCoverage     float64 `json:"testCoverage" yaml:"testCoverage"`
	ActorTrust       float64 `json:"actorTrust" yaml:"actorTrust"`
	HistoricalRisk   float64 `json:"historicalRisk" yaml:"historicalRisk"`
	SecurityImpact   float64 `json:"securityImpact" yaml:"securityImpact"`
}

// HistoryProvider supplies historical release data for risk assessment.
type HistoryProvider interface {
	// GetRecentIncidents returns recent incidents for the repository.
	GetRecentIncidents(ctx context.Context, repository string, limit int) ([]Incident, error)

	// GetRollbackRate returns the rollback rate for recent releases.
	GetRollbackRate(ctx context.Context, repository string) (float64, error)

	// GetActorHistory returns release history for an actor.
	GetActorHistory(ctx context.Context, actorID string) (*ActorHistory, error)
}

// Incident represents a past release issue.
type Incident struct {
	ReleaseID string
	Severity  string
	Category  string
}

// ActorHistory tracks an actor's release history.
type ActorHistory struct {
	TotalReleases    int
	SuccessfulCount  int
	RollbackCount    int
	IncidentCount    int
	AverageRiskScore float64
}

// Assessment contains the complete risk evaluation.
type Assessment struct {
	// Score is the overall risk score (0.0-1.0).
	Score float64

	// Factors lists individual risk contributions.
	Factors []cgp.RiskFactor

	// Severity is the human-readable severity level.
	Severity cgp.Severity

	// Summary is a brief description of the risk assessment.
	Summary string
}

// DefaultWeights returns sensible default risk weights.
func DefaultWeights() WeightConfig {
	return WeightConfig{
		APIChanges:       0.25,
		DependencyImpact: 0.20,
		BlastRadius:      0.15,
		CodeComplexity:   0.10,
		TestCoverage:     0.10,
		ActorTrust:       0.05,
		HistoricalRisk:   0.10,
		SecurityImpact:   0.05,
	}
}

// NewCalculator creates a risk calculator with the given configuration.
func NewCalculator(weights WeightConfig) *Calculator {
	return &Calculator{
		weights: weights,
	}
}

// NewCalculatorWithDefaults creates a calculator with default weights.
func NewCalculatorWithDefaults() *Calculator {
	return NewCalculator(DefaultWeights())
}

// WithHistory sets the history provider for the calculator.
func (c *Calculator) WithHistory(history HistoryProvider) *Calculator {
	c.history = history
	return c
}

// Calculate computes the overall risk score.
func (c *Calculator) Calculate(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) (*Assessment, error) {
	factors := []cgp.RiskFactor{}
	totalScore := 0.0
	totalWeight := 0.0

	// API Changes
	if apiScore, factor := c.assessAPIChanges(analysis); factor != nil {
		factors = append(factors, *factor)
		totalScore += apiScore * c.weights.APIChanges
		totalWeight += c.weights.APIChanges
	}

	// Dependency Impact
	if depScore, factor := c.assessDependencyImpact(analysis); factor != nil {
		factors = append(factors, *factor)
		totalScore += depScore * c.weights.DependencyImpact
		totalWeight += c.weights.DependencyImpact
	}

	// Blast Radius
	if blastScore, factor := c.assessBlastRadius(analysis); factor != nil {
		factors = append(factors, *factor)
		totalScore += blastScore * c.weights.BlastRadius
		totalWeight += c.weights.BlastRadius
	}

	// Actor Trust
	if proposal != nil {
		if trustScore, factor := c.assessActorTrust(proposal.Actor); factor != nil {
			factors = append(factors, *factor)
			totalScore += trustScore * c.weights.ActorTrust
			totalWeight += c.weights.ActorTrust
		}
	}

	// Security Impact
	if secScore, factor := c.assessSecurityImpact(analysis); factor != nil {
		factors = append(factors, *factor)
		totalScore += secScore * c.weights.SecurityImpact
		totalWeight += c.weights.SecurityImpact
	}

	// Historical Risk (if history provider available)
	if c.history != nil && proposal != nil {
		if histScore, factor := c.assessHistoricalRisk(ctx, proposal); factor != nil {
			factors = append(factors, *factor)
			totalScore += histScore * c.weights.HistoricalRisk
			totalWeight += c.weights.HistoricalRisk
		}
	}

	// Normalize to 0-1 range
	normalizedScore := 0.0
	if totalWeight > 0 {
		normalizedScore = totalScore / totalWeight
	}
	normalizedScore = clamp(normalizedScore, 0.0, 1.0)

	severity := scoreSeverity(normalizedScore)

	return &Assessment{
		Score:    normalizedScore,
		Factors:  factors,
		Severity: severity,
		Summary:  generateSummary(normalizedScore, factors),
	}, nil
}

// assessAPIChanges evaluates public API modifications.
func (c *Calculator) assessAPIChanges(analysis *cgp.ChangeAnalysis) (float64, *cgp.RiskFactor) {
	if analysis == nil || len(analysis.APIChanges) == 0 {
		return 0, nil
	}

	score := 0.0
	breakingCount := 0

	for _, change := range analysis.APIChanges {
		switch change.Type {
		case "removed":
			score += 1.0
			breakingCount++
		case "modified":
			if change.Breaking {
				score += 0.8
				breakingCount++
			} else {
				score += 0.3
			}
		case "deprecated":
			score += 0.2
		case "added":
			score += 0.1
		}
	}

	// Normalize based on number of changes
	normalizedScore := clamp(score/float64(len(analysis.APIChanges)), 0.0, 1.0)

	var severity cgp.Severity
	if breakingCount > 0 {
		severity = cgp.SeverityHigh
	} else if normalizedScore > 0.5 {
		severity = cgp.SeverityMedium
	} else {
		severity = cgp.SeverityLow
	}

	return normalizedScore, &cgp.RiskFactor{
		Category:    "api_change",
		Description: fmt.Sprintf("%d API changes, %d breaking", len(analysis.APIChanges), breakingCount),
		Score:       normalizedScore,
		Severity:    severity,
	}
}

// assessDependencyImpact evaluates the impact on downstream consumers.
func (c *Calculator) assessDependencyImpact(analysis *cgp.ChangeAnalysis) (float64, *cgp.RiskFactor) {
	if analysis == nil || analysis.DependencyImpact == nil {
		return 0, nil
	}

	impact := analysis.DependencyImpact
	score := 0.0

	// Score based on number of dependents
	if impact.DirectDependents > 100 {
		score = 1.0
	} else if impact.DirectDependents > 50 {
		score = 0.8
	} else if impact.DirectDependents > 10 {
		score = 0.5
	} else if impact.DirectDependents > 0 {
		score = 0.3
	}

	// Increase score for transitive impact
	if impact.TransitiveDependents > impact.DirectDependents*10 {
		score = clamp(score+0.2, 0.0, 1.0)
	}

	var severity cgp.Severity
	if score >= 0.8 {
		severity = cgp.SeverityHigh
	} else if score >= 0.5 {
		severity = cgp.SeverityMedium
	} else {
		severity = cgp.SeverityLow
	}

	return score, &cgp.RiskFactor{
		Category:    "dependency_impact",
		Description: fmt.Sprintf("%d direct dependents, %d transitive", impact.DirectDependents, impact.TransitiveDependents),
		Score:       score,
		Severity:    severity,
	}
}

// assessBlastRadius evaluates the potential impact scope.
func (c *Calculator) assessBlastRadius(analysis *cgp.ChangeAnalysis) (float64, *cgp.RiskFactor) {
	if analysis == nil || analysis.BlastRadius == nil {
		return 0, nil
	}

	blast := analysis.BlastRadius

	// Use the pre-calculated blast radius score if available
	score := blast.Score

	// If no score, calculate from files/lines
	if score == 0 {
		fileScore := 0.0
		if blast.FilesChanged > 50 {
			fileScore = 1.0
		} else if blast.FilesChanged > 20 {
			fileScore = 0.7
		} else if blast.FilesChanged > 10 {
			fileScore = 0.5
		} else if blast.FilesChanged > 5 {
			fileScore = 0.3
		} else {
			fileScore = 0.1
		}

		lineScore := 0.0
		if blast.LinesChanged > 1000 {
			lineScore = 1.0
		} else if blast.LinesChanged > 500 {
			lineScore = 0.7
		} else if blast.LinesChanged > 100 {
			lineScore = 0.4
		} else {
			lineScore = 0.1
		}

		score = (fileScore + lineScore) / 2
	}

	var severity cgp.Severity
	if score >= 0.7 {
		severity = cgp.SeverityHigh
	} else if score >= 0.4 {
		severity = cgp.SeverityMedium
	} else {
		severity = cgp.SeverityLow
	}

	return score, &cgp.RiskFactor{
		Category:    "blast_radius",
		Description: fmt.Sprintf("%d files, %d lines changed", blast.FilesChanged, blast.LinesChanged),
		Score:       score,
		Severity:    severity,
	}
}

// assessActorTrust evaluates trust based on actor type.
func (c *Calculator) assessActorTrust(actor cgp.Actor) (float64, *cgp.RiskFactor) {
	// Lower trust = higher risk score
	var score float64
	var severity cgp.Severity
	var desc string

	switch actor.Kind {
	case cgp.ActorKindHuman:
		score = 0.1
		severity = cgp.SeverityLow
		desc = "Human developer (trusted)"
	case cgp.ActorKindCI:
		score = 0.2
		severity = cgp.SeverityLow
		desc = "CI/CD system (trusted)"
	case cgp.ActorKindSystem:
		score = 0.4
		severity = cgp.SeverityMedium
		desc = "Automated system (moderate trust)"
	case cgp.ActorKindAgent:
		score = 0.6
		severity = cgp.SeverityMedium
		desc = "AI agent (requires oversight)"
	default:
		score = 0.8
		severity = cgp.SeverityHigh
		desc = "Unknown actor type"
	}

	return score, &cgp.RiskFactor{
		Category:    "actor_trust",
		Description: desc,
		Score:       score,
		Severity:    severity,
	}
}

// assessSecurityImpact evaluates security-related changes.
func (c *Calculator) assessSecurityImpact(analysis *cgp.ChangeAnalysis) (float64, *cgp.RiskFactor) {
	if analysis == nil || analysis.Security == 0 {
		return 0, nil
	}

	// Any security change warrants attention
	score := 0.5
	if analysis.Security > 3 {
		score = 0.9
	} else if analysis.Security > 1 {
		score = 0.7
	}

	var severity cgp.Severity
	if score >= 0.7 {
		severity = cgp.SeverityHigh
	} else {
		severity = cgp.SeverityMedium
	}

	return score, &cgp.RiskFactor{
		Category:    "security_impact",
		Description: fmt.Sprintf("%d security-related changes", analysis.Security),
		Score:       score,
		Severity:    severity,
	}
}

// assessHistoricalRisk uses release memory to evaluate patterns.
func (c *Calculator) assessHistoricalRisk(ctx context.Context, proposal *cgp.ChangeProposal) (float64, *cgp.RiskFactor) {
	if c.history == nil {
		return 0, nil
	}

	// Check rollback rate
	rollbackRate, err := c.history.GetRollbackRate(ctx, proposal.Scope.Repository)
	if err != nil {
		return 0, nil
	}

	score := rollbackRate
	var severity cgp.Severity
	var desc string

	if rollbackRate > 0.2 {
		severity = cgp.SeverityHigh
		desc = fmt.Sprintf("High rollback rate: %.0f%%", rollbackRate*100)
	} else if rollbackRate > 0.1 {
		severity = cgp.SeverityMedium
		desc = fmt.Sprintf("Moderate rollback rate: %.0f%%", rollbackRate*100)
	} else {
		severity = cgp.SeverityLow
		desc = fmt.Sprintf("Low rollback rate: %.0f%%", rollbackRate*100)
	}

	return score, &cgp.RiskFactor{
		Category:    "historical_risk",
		Description: desc,
		Score:       score,
		Severity:    severity,
	}
}

// scoreSeverity converts a risk score to severity level.
func scoreSeverity(score float64) cgp.Severity {
	switch {
	case score >= 0.8:
		return cgp.SeverityCritical
	case score >= 0.6:
		return cgp.SeverityHigh
	case score >= 0.4:
		return cgp.SeverityMedium
	default:
		return cgp.SeverityLow
	}
}

// generateSummary creates a human-readable summary.
func generateSummary(score float64, factors []cgp.RiskFactor) string {
	if len(factors) == 0 {
		return "No risk factors identified"
	}

	severity := scoreSeverity(score)
	highRiskFactors := 0
	for _, f := range factors {
		if f.Severity == cgp.SeverityHigh || f.Severity == cgp.SeverityCritical {
			highRiskFactors++
		}
	}

	if highRiskFactors > 0 {
		return fmt.Sprintf("%s risk: %d high-severity factors detected", severity, highRiskFactors)
	}
	return fmt.Sprintf("%s risk based on %d factors", severity, len(factors))
}

// clamp restricts a value to a range.
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
