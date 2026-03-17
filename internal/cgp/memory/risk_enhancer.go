package memory

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Change represents a file-level change for risk enhancement analysis.
type Change struct {
	// FilePath is the path of the changed file.
	FilePath string

	// Package is the package or module the file belongs to.
	Package string

	// LinesChanged is the total lines added/removed in this file.
	LinesChanged int
}

// RiskEnhancement contains the result of historical risk enhancement.
type RiskEnhancement struct {
	// OriginalScore is the base risk score before enhancement.
	OriginalScore float64 `json:"originalScore"`

	// EnhancedScore is the adjusted risk score after historical analysis.
	EnhancedScore float64 `json:"enhancedScore"`

	// Reasons lists human-readable explanations for score adjustments.
	Reasons []string `json:"reasons"`

	// PatternsApplied lists the patterns that influenced the score.
	PatternsApplied []Pattern `json:"patternsApplied,omitempty"`
}

// RiskEnhancer adjusts base risk scores using historical patterns.
type RiskEnhancer struct {
	detector   *PatternDetector
	repository string
	window     time.Duration

	// maxAdjustment caps the total adjustment to prevent extreme swings.
	maxAdjustment float64
}

// RiskEnhancerOption configures the RiskEnhancer.
type RiskEnhancerOption func(*RiskEnhancer)

// WithWindow sets the historical analysis window.
func WithWindow(d time.Duration) RiskEnhancerOption {
	return func(e *RiskEnhancer) {
		e.window = d
	}
}

// WithMaxAdjustment sets the maximum score adjustment.
func WithMaxAdjustment(maxAdj float64) RiskEnhancerOption {
	return func(e *RiskEnhancer) {
		if maxAdj > 0 && maxAdj <= 1.0 {
			e.maxAdjustment = maxAdj
		}
	}
}

// NewRiskEnhancer creates a new risk enhancer for a specific repository.
func NewRiskEnhancer(detector *PatternDetector, repository string, opts ...RiskEnhancerOption) *RiskEnhancer {
	e := &RiskEnhancer{
		detector:      detector,
		repository:    repository,
		window:        90 * 24 * time.Hour, // Default: 90 days
		maxAdjustment: 0.3,                 // Default: max 0.3 adjustment
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// EnhanceRiskScore adjusts a base risk score using historical patterns.
// It returns the adjusted score and a list of reasons for the adjustment.
func (e *RiskEnhancer) EnhanceRiskScore(ctx context.Context, baseScore float64, changes []Change) (float64, []string) {
	patterns, err := e.detector.DetectPatterns(ctx, e.repository, e.window)
	if err != nil || len(patterns) == 0 {
		return baseScore, nil
	}

	totalAdjustment := 0.0
	var reasons []string

	for _, pattern := range patterns {
		if !e.patternApplies(pattern, changes) {
			continue
		}

		// Weight the modifier by confidence.
		weightedModifier := pattern.RiskModifier * pattern.Confidence
		totalAdjustment += weightedModifier

		reasons = append(reasons, fmt.Sprintf("[%.0f%% confidence] %s (risk %+.2f)",
			pattern.Confidence*100, pattern.Description, weightedModifier))
	}

	// Cap the adjustment.
	if totalAdjustment > e.maxAdjustment {
		totalAdjustment = e.maxAdjustment
	} else if totalAdjustment < -e.maxAdjustment {
		totalAdjustment = -e.maxAdjustment
	}

	enhancedScore := baseScore + totalAdjustment
	if enhancedScore < 0.0 {
		enhancedScore = 0.0
	}
	if enhancedScore > 1.0 {
		enhancedScore = 1.0
	}

	return enhancedScore, reasons
}

// EnhanceRiskScoreDetailed returns a full RiskEnhancement with pattern details.
func (e *RiskEnhancer) EnhanceRiskScoreDetailed(ctx context.Context, baseScore float64, changes []Change) (*RiskEnhancement, error) {
	patterns, err := e.detector.DetectPatterns(ctx, e.repository, e.window)
	if err != nil {
		return nil, fmt.Errorf("failed to detect patterns: %w", err)
	}

	result := &RiskEnhancement{
		OriginalScore: baseScore,
		EnhancedScore: baseScore,
	}

	if len(patterns) == 0 {
		return result, nil
	}

	totalAdjustment := 0.0

	for _, pattern := range patterns {
		if !e.patternApplies(pattern, changes) {
			continue
		}

		weightedModifier := pattern.RiskModifier * pattern.Confidence
		totalAdjustment += weightedModifier

		result.PatternsApplied = append(result.PatternsApplied, pattern)
		result.Reasons = append(result.Reasons, fmt.Sprintf("[%.0f%% confidence] %s (risk %+.2f)",
			pattern.Confidence*100, pattern.Description, weightedModifier))
	}

	// Cap and clamp.
	if totalAdjustment > e.maxAdjustment {
		totalAdjustment = e.maxAdjustment
	} else if totalAdjustment < -e.maxAdjustment {
		totalAdjustment = -e.maxAdjustment
	}

	result.EnhancedScore = clampScore(baseScore + totalAdjustment)

	return result, nil
}

// patternApplies checks whether a detected pattern is relevant to the current changes.
func (e *RiskEnhancer) patternApplies(pattern Pattern, changes []Change) bool {
	switch pattern.Category {
	case PatternRiskyFiles:
		return e.riskyFileApplies(pattern, changes)
	case PatternChangeSize:
		return e.changeSizeApplies(pattern, changes)
	case PatternTimeOfDay:
		return e.timeOfDayApplies(pattern)
	case PatternDayOfWeek:
		return e.dayOfWeekApplies(pattern)
	case PatternPackageCombination:
		return e.packageCombinationApplies(pattern, changes)
	default:
		return false
	}
}

// riskyFileApplies checks if any changed file matches the risky file pattern.
func (e *RiskEnhancer) riskyFileApplies(pattern Pattern, changes []Change) bool {
	filePath, ok := pattern.Details["file_path"].(string)
	if !ok {
		return false
	}
	for _, c := range changes {
		if c.FilePath == filePath {
			return true
		}
	}
	return false
}

// changeSizeApplies checks if the total change size falls into the risky bucket.
func (e *RiskEnhancer) changeSizeApplies(pattern Pattern, changes []Change) bool {
	totalLines := 0
	for _, c := range changes {
		totalLines += c.LinesChanged
	}

	minLines, minOk := toInt(pattern.Details["min_lines"])
	maxLines, maxOk := toInt(pattern.Details["max_lines"])
	if !minOk || !maxOk {
		return false
	}

	return totalLines >= minLines && totalLines <= maxLines
}

// timeOfDayApplies checks if the current time falls into the risky time period.
func (e *RiskEnhancer) timeOfDayApplies(pattern Pattern) bool {
	currentHour := time.Now().Hour()

	startHour, startOk := toInt(pattern.Details["start_hour"])
	endHour, endOk := toInt(pattern.Details["end_hour"])
	if !startOk || !endOk {
		return false
	}

	return currentHour >= startHour && currentHour < endHour
}

// dayOfWeekApplies checks if the current day matches the risky day.
func (e *RiskEnhancer) dayOfWeekApplies(pattern Pattern) bool {
	currentDay := int(time.Now().Weekday())
	patternDay, ok := toInt(pattern.Details["day_of_week"])
	if !ok {
		return false
	}
	return currentDay == patternDay
}

// packageCombinationApplies checks if the current changes include the risky package pair.
func (e *RiskEnhancer) packageCombinationApplies(pattern Pattern, changes []Change) bool {
	pkgA, aOk := pattern.Details["package_a"].(string)
	pkgB, bOk := pattern.Details["package_b"].(string)
	if !aOk || !bOk {
		return false
	}

	packages := make(map[string]bool)
	for _, c := range changes {
		packages[c.Package] = true
	}

	return packages[pkgA] && packages[pkgB]
}

// toInt converts an interface{} value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// clampScore restricts a risk score to the valid range [0.0, 1.0].
func clampScore(score float64) float64 {
	if score < 0.0 {
		return 0.0
	}
	if score > 1.0 {
		return 1.0
	}
	return score
}

// SortPatternsByRisk sorts patterns by risk modifier descending.
func SortPatternsByRisk(patterns []Pattern) {
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].RiskModifier > patterns[j].RiskModifier
	})
}
