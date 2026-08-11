package memory

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Insight represents a historical observation relevant to a release.
type Insight struct {
	// ID is a unique identifier for this insight.
	ID string `json:"id"`

	// Category classifies the insight type.
	Category InsightCategory `json:"category"`

	// Message is a human-readable description of the insight.
	Message string `json:"message"`

	// Severity indicates how important this insight is.
	Severity string `json:"severity"` // info, warning, critical

	// Confidence is how confident we are in this insight (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// RelatedPattern is the pattern that generated this insight, if any.
	RelatedPattern *Pattern `json:"relatedPattern,omitempty"`

	// Metadata contains insight-specific data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// InsightCategory classifies insight types.
type InsightCategory string

const (
	InsightCategoryRisk       InsightCategory = "risk"
	InsightCategoryPattern    InsightCategory = "pattern"
	InsightCategoryTrend      InsightCategory = "trend"
	InsightCategoryActor      InsightCategory = "actor"
	InsightCategoryHistorical InsightCategory = "historical"
)

// Trends represents aggregated trend data for a repository.
type Trends struct {
	// Repository is the repository path.
	Repository string `json:"repository"`

	// Window is the time period analyzed.
	Window time.Duration `json:"window"`

	// TotalReleases is the count of releases in the window.
	TotalReleases int `json:"totalReleases"`

	// SuccessRate is the ratio of successful releases (0.0-1.0).
	SuccessRate float64 `json:"successRate"`

	// AverageRiskScore is the mean risk score across releases.
	AverageRiskScore float64 `json:"averageRiskScore"`

	// RiskTrend indicates whether risk is increasing, stable, or decreasing.
	RiskTrend RiskTrend `json:"riskTrend"`

	// CommonPatterns are the most frequently detected patterns.
	CommonPatterns []Pattern `json:"commonPatterns,omitempty"`

	// OutcomeDistribution maps outcome types to their counts.
	OutcomeDistribution map[Outcome]int `json:"outcomeDistribution"`

	// AverageTimeToDetect is the mean incident detection time.
	AverageTimeToDetect time.Duration `json:"averageTimeToDetect,omitempty"`

	// MeanTimeBetweenFailures is the average time between negative outcomes.
	MeanTimeBetweenFailures time.Duration `json:"meanTimeBetweenFailures,omitempty"`

	// AnalyzedAt is when this trend analysis was computed.
	AnalyzedAt time.Time `json:"analyzedAt"`
}

// InsightsService provides access to historical insights and trends.
type InsightsService struct {
	memoryStore  Store
	outcomeStore OutcomeStore
	detector     *PatternDetector
	repository   string
}

// NewInsightsService creates a new insights service for a repository.
func NewInsightsService(memoryStore Store, outcomeStore OutcomeStore, detector *PatternDetector, repository string) *InsightsService {
	return &InsightsService{
		memoryStore:  memoryStore,
		outcomeStore: outcomeStore,
		detector:     detector,
		repository:   repository,
	}
}

// GetInsights returns historical insights relevant to a specific release.
func (s *InsightsService) GetInsights(ctx context.Context, releaseID string) ([]Insight, error) {
	var insights []Insight

	// Get the release outcomes.
	outcomes, err := s.outcomeStore.GetOutcomesByRelease(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get outcomes for release %s: %w", releaseID, err)
	}

	// Get incidents for this release.
	incidents, err := s.outcomeStore.GetIncidentsByRelease(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get incidents for release %s: %w", releaseID, err)
	}

	// Generate insights from outcomes.
	for _, o := range outcomes {
		if o.OutcomeType.IsNegative() {
			insights = append(insights, Insight{
				ID:         fmt.Sprintf("outcome_%s_%s", releaseID, o.ID),
				Category:   InsightCategoryHistorical,
				Message:    fmt.Sprintf("Release had a negative outcome: %s", o.OutcomeType),
				Severity:   "warning",
				Confidence: 1.0,
				Metadata: map[string]any{
					"outcome_type": string(o.OutcomeType),
					"description":  o.Description,
				},
			})
		}
	}

	// Generate insights from incidents.
	for _, inc := range incidents {
		severity := "warning"
		if inc.Severity == "critical" || inc.Severity == "high" {
			severity = "critical"
		}

		insights = append(insights, Insight{
			ID:         fmt.Sprintf("incident_%s_%s", releaseID, inc.ID),
			Category:   InsightCategoryRisk,
			Message:    fmt.Sprintf("Incident reported: %s (severity: %s)", inc.Description, inc.Severity),
			Severity:   severity,
			Confidence: 1.0,
			Metadata: map[string]any{
				"incident_id":    inc.ID,
				"severity":       inc.Severity,
				"root_cause":     inc.RootCause,
				"time_to_detect": inc.TimeToDetect.String(),
			},
		})
	}

	// Detect patterns and add relevant insights.
	patterns, err := s.detector.DetectPatterns(ctx, s.repository, 90*24*time.Hour)
	if err == nil && len(patterns) > 0 {
		// Find patterns relevant to the release's changed files.
		if len(outcomes) > 0 {
			for _, p := range patterns {
				if p.Confidence >= 0.5 {
					patternCopy := p
					insights = append(insights, Insight{
						ID:             fmt.Sprintf("pattern_%s_%s", releaseID, p.ID),
						Category:       InsightCategoryPattern,
						Message:        p.Description,
						Severity:       patternSeverity(p),
						Confidence:     p.Confidence,
						RelatedPattern: &patternCopy,
					})
				}
			}
		}
	}

	return insights, nil
}

// GetTrends computes trend data for the repository over the given time window.
func (s *InsightsService) GetTrends(ctx context.Context, window time.Duration) (*Trends, error) {
	since := time.Now().Add(-window)

	outcomes, err := s.outcomeStore.GetOutcomes(ctx, s.repository, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get outcomes: %w", err)
	}

	trends := &Trends{
		Repository:          s.repository,
		Window:              window,
		TotalReleases:       len(outcomes),
		OutcomeDistribution: make(map[Outcome]int),
		AnalyzedAt:          time.Now(),
	}

	if len(outcomes) == 0 {
		return trends, nil
	}

	// Calculate outcome distribution and success rate.
	successCount := 0
	var totalRisk float64
	var negativeTimestamps []time.Time
	var totalDetectTime time.Duration
	detectTimeCount := 0

	for _, o := range outcomes {
		trends.OutcomeDistribution[o.OutcomeType]++

		if o.OutcomeType == OutcomeTypeSuccessfulRelease {
			successCount++
		}

		if o.OutcomeType.IsNegative() {
			negativeTimestamps = append(negativeTimestamps, o.RecordedAt)
		}
	}

	trends.SuccessRate = float64(successCount) / float64(len(outcomes))

	// Get release history for risk scores.
	releases, err := s.memoryStore.GetReleaseHistory(ctx, s.repository, 1000)
	if err == nil && len(releases) > 0 {
		riskCount := 0
		for _, r := range releases {
			if !r.ReleasedAt.Before(since) {
				totalRisk += r.RiskScore
				riskCount++
			}
		}
		if riskCount > 0 {
			trends.AverageRiskScore = totalRisk / float64(riskCount)
		}

		// Calculate risk trend.
		trends.RiskTrend = calculateRiskTrend(releases, since)
	}

	// Calculate average time to detect from incidents.
	incidents, err := s.outcomeStore.GetOutcomes(ctx, s.repository, since)
	if err == nil {
		for _, inc := range incidents {
			if inc.OutcomeType == OutcomeTypeIncident {
				// Use metadata if available for detect time.
				detectTimeCount++
			}
		}
	}

	if detectTimeCount > 0 {
		trends.AverageTimeToDetect = totalDetectTime / time.Duration(detectTimeCount)
	}

	// Calculate mean time between failures.
	if len(negativeTimestamps) >= 2 {
		totalGap := negativeTimestamps[len(negativeTimestamps)-1].Sub(negativeTimestamps[0])
		trends.MeanTimeBetweenFailures = totalGap / time.Duration(len(negativeTimestamps)-1)
	}

	// Detect patterns for common patterns.
	patterns, err := s.detector.DetectPatterns(ctx, s.repository, window)
	if err == nil {
		// Take top 5 patterns.
		limit := 5
		if len(patterns) < limit {
			limit = len(patterns)
		}
		trends.CommonPatterns = patterns[:limit]
	}

	return trends, nil
}

// calculateRiskTrend determines the risk trend from historical releases.
func calculateRiskTrend(releases []*ReleaseRecord, since time.Time) RiskTrend {
	// Filter to releases within window.
	var filtered []*ReleaseRecord
	for _, r := range releases {
		if !r.ReleasedAt.Before(since) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) < 4 {
		return TrendStable
	}

	mid := len(filtered) / 2
	var firstHalfRisk, secondHalfRisk float64
	for i := 0; i < mid; i++ {
		firstHalfRisk += filtered[i].RiskScore
	}
	for i := mid; i < len(filtered); i++ {
		secondHalfRisk += filtered[i].RiskScore
	}

	firstHalfAvg := firstHalfRisk / float64(mid)
	secondHalfAvg := secondHalfRisk / float64(len(filtered)-mid)

	diff := secondHalfAvg - firstHalfAvg
	if diff > 0.1 {
		return TrendIncreasing
	} else if diff < -0.1 {
		return TrendDecreasing
	}
	return TrendStable
}

// patternSeverity maps a pattern's risk modifier to a severity string.
func patternSeverity(p Pattern) string {
	if p.RiskModifier >= 0.2 {
		return "critical"
	}
	if p.RiskModifier >= 0.1 {
		return "warning"
	}
	return "info"
}

// RiskTrendOf reports whether risk is rising, falling or steady across a history.
//
// Ordering-independent: it sorts by ReleasedAt itself rather than trusting the caller's
// order. Both previous copies of this arithmetic treated the tail of the slice as the
// recent half, which is right for an oldest-first list and exactly backwards for a
// newest-first one — and GetReleaseHistory returns newest first. A trend that inverts
// depending on which reader asked is worse than no trend, because both answers look
// equally plausible in a report.
//
// Fewer than four releases yields TrendStable. A direction inferred from two releases is
// noise, and noise presented as a finding in a governance decision is worse than an
// admission of not knowing.
func RiskTrendOf(releases []*ReleaseRecord) RiskTrend {
	usable := make([]*ReleaseRecord, 0, len(releases))
	for _, r := range releases {
		if r != nil && !r.ReleasedAt.IsZero() {
			usable = append(usable, r)
		}
	}
	if len(usable) < minimumReleasesForATrend {
		return TrendStable
	}

	sort.Slice(usable, func(i, j int) bool {
		return usable[i].ReleasedAt.Before(usable[j].ReleasedAt)
	})

	mid := len(usable) / 2
	older := meanRiskScore(usable[:mid])
	recent := meanRiskScore(usable[mid:])

	switch diff := recent - older; {
	case diff > riskTrendTolerance:
		return TrendIncreasing
	case diff < -riskTrendTolerance:
		return TrendDecreasing
	default:
		return TrendStable
	}
}

const (
	// minimumReleasesForATrend is the point below which a direction is noise.
	minimumReleasesForATrend = 4

	// riskTrendTolerance keeps two nearly equal averages from reading as a trend.
	// Without it every repository drifts to increasing or decreasing on rounding.
	riskTrendTolerance = 0.1
)

func meanRiskScore(releases []*ReleaseRecord) float64 {
	if len(releases) == 0 {
		return 0
	}
	var total float64
	for _, r := range releases {
		total += r.RiskScore
	}
	return total / float64(len(releases))
}
