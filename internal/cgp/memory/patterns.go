package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Pattern represents a historical risk pattern detected from release outcomes.
type Pattern struct {
	// ID is a unique identifier for this pattern.
	ID string `json:"id"`

	// Category classifies the pattern type.
	Category PatternCategory `json:"category"`

	// Description is a human-readable explanation of the pattern.
	Description string `json:"description"`

	// Confidence is how confident we are in this pattern (0.0-1.0).
	// Higher values mean more evidence supports the pattern.
	Confidence float64 `json:"confidence"`

	// EvidenceCount is the number of releases supporting this pattern.
	EvidenceCount int `json:"evidenceCount"`

	// RiskModifier is the adjustment to apply to the base risk score.
	// Positive values increase risk, negative values decrease it.
	RiskModifier float64 `json:"riskModifier"`

	// Details contains pattern-specific data.
	Details map[string]any `json:"details,omitempty"`

	// DetectedAt is when this pattern was identified.
	DetectedAt time.Time `json:"detectedAt"`
}

// PatternCategory classifies detected patterns.
type PatternCategory string

const (
	// PatternRiskyFiles identifies file paths that frequently cause incidents.
	PatternRiskyFiles PatternCategory = "risky_files"

	// PatternChangeSize identifies change sizes correlated with rollbacks.
	PatternChangeSize PatternCategory = "change_size"

	// PatternTimeOfDay identifies time-of-day patterns for failed releases.
	PatternTimeOfDay PatternCategory = "time_of_day"

	// PatternDayOfWeek identifies day-of-week patterns for failed releases.
	PatternDayOfWeek PatternCategory = "day_of_week"

	// PatternPackageCombination identifies package combinations that are risky together.
	PatternPackageCombination PatternCategory = "package_combination"
)

// PatternDetector analyzes historical release outcomes to identify risk patterns.
type PatternDetector struct {
	outcomeStore OutcomeStore
	memoryStore  Store

	// minSampleSize is the minimum number of releases needed to detect a pattern.
	minSampleSize int

	// minConfidence is the minimum confidence threshold for reporting a pattern.
	minConfidence float64
}

// PatternDetectorOption configures the PatternDetector.
type PatternDetectorOption func(*PatternDetector)

// WithMinSampleSize sets the minimum sample size for pattern detection.
func WithMinSampleSize(n int) PatternDetectorOption {
	return func(d *PatternDetector) {
		if n > 0 {
			d.minSampleSize = n
		}
	}
}

// WithMinConfidence sets the minimum confidence threshold for pattern reporting.
func WithMinConfidence(c float64) PatternDetectorOption {
	return func(d *PatternDetector) {
		if c >= 0 && c <= 1 {
			d.minConfidence = c
		}
	}
}

// NewPatternDetector creates a new pattern detector.
func NewPatternDetector(outcomeStore OutcomeStore, memoryStore Store, opts ...PatternDetectorOption) *PatternDetector {
	d := &PatternDetector{
		outcomeStore:  outcomeStore,
		memoryStore:   memoryStore,
		minSampleSize: 3,
		minConfidence: 0.3,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// DetectPatterns analyzes historical releases within the given time window
// to identify risk patterns.
func (d *PatternDetector) DetectPatterns(ctx context.Context, repository string, window time.Duration) ([]Pattern, error) {
	since := time.Now().Add(-window)

	outcomes, err := d.outcomeStore.GetOutcomes(ctx, repository, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get outcomes: %w", err)
	}

	if len(outcomes) < d.minSampleSize {
		return []Pattern{}, nil
	}

	var patterns []Pattern

	// Detect risky file patterns
	if filePatterns := d.detectRiskyFiles(outcomes); len(filePatterns) > 0 {
		patterns = append(patterns, filePatterns...)
	}

	// Detect change size patterns
	if sizePatterns := d.detectChangeSizePatterns(outcomes); len(sizePatterns) > 0 {
		patterns = append(patterns, sizePatterns...)
	}

	// Detect time-of-day patterns
	if timePatterns := d.detectTimePatterns(outcomes); len(timePatterns) > 0 {
		patterns = append(patterns, timePatterns...)
	}

	// Detect day-of-week patterns
	if dayPatterns := d.detectDayPatterns(outcomes); len(dayPatterns) > 0 {
		patterns = append(patterns, dayPatterns...)
	}

	// Detect package combination patterns
	if pkgPatterns := d.detectPackageCombinations(outcomes); len(pkgPatterns) > 0 {
		patterns = append(patterns, pkgPatterns...)
	}

	// Filter by minimum confidence
	var filtered []Pattern
	for _, p := range patterns {
		if p.Confidence >= d.minConfidence {
			filtered = append(filtered, p)
		}
	}

	// Sort by confidence descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})

	return filtered, nil
}

// detectRiskyFiles identifies file paths that frequently appear in negative outcomes.
func (d *PatternDetector) detectRiskyFiles(outcomes []*OutcomeRecord) []Pattern {
	// Count how often each file appears in negative vs positive outcomes.
	type fileStats struct {
		negativeCount int
		totalCount    int
	}

	stats := make(map[string]*fileStats)
	for _, o := range outcomes {
		for _, f := range o.FilesAffected {
			if stats[f] == nil {
				stats[f] = &fileStats{}
			}
			stats[f].totalCount++
			if o.OutcomeType.IsNegative() {
				stats[f].negativeCount++
			}
		}
	}

	var patterns []Pattern
	now := time.Now()
	for filePath, fs := range stats {
		if fs.totalCount < d.minSampleSize {
			continue
		}

		failRate := float64(fs.negativeCount) / float64(fs.totalCount)
		if failRate < 0.3 {
			continue // Not risky enough
		}

		confidence := calculateConfidence(fs.totalCount, failRate)
		riskModifier := failRate * 0.3 // Max +0.3 risk modifier

		patterns = append(patterns, Pattern{
			ID:            fmt.Sprintf("risky_file_%s", sanitizeID(filePath)),
			Category:      PatternRiskyFiles,
			Description:   fmt.Sprintf("File %q has a %.0f%% failure rate across %d releases", filePath, failRate*100, fs.totalCount),
			Confidence:    confidence,
			EvidenceCount: fs.totalCount,
			RiskModifier:  riskModifier,
			Details: map[string]any{
				"file_path":      filePath,
				"failure_rate":   failRate,
				"negative_count": fs.negativeCount,
				"total_count":    fs.totalCount,
			},
			DetectedAt: now,
		})
	}

	return patterns
}

// detectChangeSizePatterns identifies change sizes correlated with rollbacks.
func (d *PatternDetector) detectChangeSizePatterns(outcomes []*OutcomeRecord) []Pattern {
	// Bucket change sizes and compare failure rates.
	type bucket struct {
		label         string
		min, max      int
		negativeCount int
		totalCount    int
	}

	buckets := []*bucket{
		{label: "small", min: 0, max: 50},
		{label: "medium", min: 51, max: 200},
		{label: "large", min: 201, max: 500},
		{label: "very_large", min: 501, max: math.MaxInt64},
	}

	for _, o := range outcomes {
		for _, b := range buckets {
			if o.ChangeSize >= b.min && o.ChangeSize <= b.max {
				b.totalCount++
				if o.OutcomeType.IsNegative() {
					b.negativeCount++
				}
				break
			}
		}
	}

	// Calculate overall failure rate for baseline comparison.
	totalNeg, total := 0, 0
	for _, o := range outcomes {
		total++
		if o.OutcomeType.IsNegative() {
			totalNeg++
		}
	}
	baselineRate := 0.0
	if total > 0 {
		baselineRate = float64(totalNeg) / float64(total)
	}

	var patterns []Pattern
	now := time.Now()
	for _, b := range buckets {
		if b.totalCount < d.minSampleSize {
			continue
		}

		failRate := float64(b.negativeCount) / float64(b.totalCount)
		// Only report if significantly above baseline.
		if failRate <= baselineRate*1.5 {
			continue
		}

		confidence := calculateConfidence(b.totalCount, failRate)
		riskModifier := (failRate - baselineRate) * 0.25

		patterns = append(patterns, Pattern{
			ID:            fmt.Sprintf("change_size_%s", b.label),
			Category:      PatternChangeSize,
			Description:   fmt.Sprintf("%s changes (%d-%d lines) have a %.0f%% failure rate vs %.0f%% baseline", b.label, b.min, b.max, failRate*100, baselineRate*100),
			Confidence:    confidence,
			EvidenceCount: b.totalCount,
			RiskModifier:  riskModifier,
			Details: map[string]any{
				"size_bucket":   b.label,
				"min_lines":     b.min,
				"max_lines":     b.max,
				"failure_rate":  failRate,
				"baseline_rate": baselineRate,
			},
			DetectedAt: now,
		})
	}

	return patterns
}

// detectTimePatterns identifies hours of day with elevated failure rates.
func (d *PatternDetector) detectTimePatterns(outcomes []*OutcomeRecord) []Pattern {
	// Group outcomes by time period (morning, afternoon, evening, night).
	type period struct {
		label     string
		startHour int
		endHour   int
		negCount  int
		total     int
	}

	periods := []*period{
		{label: "night", startHour: 0, endHour: 6},
		{label: "morning", startHour: 6, endHour: 12},
		{label: "afternoon", startHour: 12, endHour: 18},
		{label: "evening", startHour: 18, endHour: 24},
	}

	for _, o := range outcomes {
		for _, p := range periods {
			if o.HourOfDay >= p.startHour && o.HourOfDay < p.endHour {
				p.total++
				if o.OutcomeType.IsNegative() {
					p.negCount++
				}
				break
			}
		}
	}

	totalNeg, total := countNegativeOutcomes(outcomes)
	baselineRate := 0.0
	if total > 0 {
		baselineRate = float64(totalNeg) / float64(total)
	}

	var patterns []Pattern
	now := time.Now()
	for _, p := range periods {
		if p.total < d.minSampleSize {
			continue
		}

		failRate := float64(p.negCount) / float64(p.total)
		if failRate <= baselineRate*1.5 {
			continue
		}

		confidence := calculateConfidence(p.total, failRate)
		riskModifier := (failRate - baselineRate) * 0.2

		patterns = append(patterns, Pattern{
			ID:            fmt.Sprintf("time_%s", p.label),
			Category:      PatternTimeOfDay,
			Description:   fmt.Sprintf("Releases during %s (%02d:00-%02d:00) have a %.0f%% failure rate vs %.0f%% baseline", p.label, p.startHour, p.endHour, failRate*100, baselineRate*100),
			Confidence:    confidence,
			EvidenceCount: p.total,
			RiskModifier:  riskModifier,
			Details: map[string]any{
				"period":        p.label,
				"start_hour":    p.startHour,
				"end_hour":      p.endHour,
				"failure_rate":  failRate,
				"baseline_rate": baselineRate,
			},
			DetectedAt: now,
		})
	}

	return patterns
}

// detectDayPatterns identifies days of week with elevated failure rates.
func (d *PatternDetector) detectDayPatterns(outcomes []*OutcomeRecord) []Pattern {
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	type dayStats struct {
		negCount int
		total    int
	}
	stats := make([]dayStats, 7)

	for _, o := range outcomes {
		if o.DayOfWeek >= 0 && o.DayOfWeek < 7 {
			stats[o.DayOfWeek].total++
			if o.OutcomeType.IsNegative() {
				stats[o.DayOfWeek].negCount++
			}
		}
	}

	totalNeg, total := countNegativeOutcomes(outcomes)
	baselineRate := 0.0
	if total > 0 {
		baselineRate = float64(totalNeg) / float64(total)
	}

	var patterns []Pattern
	now := time.Now()
	for day := 0; day < 7; day++ {
		s := stats[day]
		if s.total < d.minSampleSize {
			continue
		}

		failRate := float64(s.negCount) / float64(s.total)
		if failRate <= baselineRate*1.5 {
			continue
		}

		confidence := calculateConfidence(s.total, failRate)
		riskModifier := (failRate - baselineRate) * 0.2

		patterns = append(patterns, Pattern{
			ID:            fmt.Sprintf("day_%s", strings.ToLower(dayNames[day])),
			Category:      PatternDayOfWeek,
			Description:   fmt.Sprintf("Releases on %s have a %.0f%% failure rate vs %.0f%% baseline", dayNames[day], failRate*100, baselineRate*100),
			Confidence:    confidence,
			EvidenceCount: s.total,
			RiskModifier:  riskModifier,
			Details: map[string]any{
				"day_of_week":   day,
				"day_name":      dayNames[day],
				"failure_rate":  failRate,
				"baseline_rate": baselineRate,
			},
			DetectedAt: now,
		})
	}

	return patterns
}

// detectPackageCombinations identifies package pairs that are risky when changed together.
func (d *PatternDetector) detectPackageCombinations(outcomes []*OutcomeRecord) []Pattern {
	// Count co-occurrences of package pairs in negative outcomes.
	type pairStats struct {
		negativeCount int
		totalCount    int
	}
	pairMap := make(map[string]*pairStats)

	for _, o := range outcomes {
		if len(o.PackagesAffected) < 2 {
			continue
		}

		// Generate sorted pairs to avoid duplicates.
		pkgs := make([]string, len(o.PackagesAffected))
		copy(pkgs, o.PackagesAffected)
		sort.Strings(pkgs)

		for i := 0; i < len(pkgs); i++ {
			for j := i + 1; j < len(pkgs); j++ {
				key := pkgs[i] + "+" + pkgs[j]
				if pairMap[key] == nil {
					pairMap[key] = &pairStats{}
				}
				pairMap[key].totalCount++
				if o.OutcomeType.IsNegative() {
					pairMap[key].negativeCount++
				}
			}
		}
	}

	var patterns []Pattern
	now := time.Now()
	for key, ps := range pairMap {
		if ps.totalCount < d.minSampleSize {
			continue
		}

		failRate := float64(ps.negativeCount) / float64(ps.totalCount)
		if failRate < 0.4 {
			continue // Not risky enough for package combinations
		}

		parts := strings.SplitN(key, "+", 2)
		confidence := calculateConfidence(ps.totalCount, failRate)
		riskModifier := failRate * 0.25

		patterns = append(patterns, Pattern{
			ID:            fmt.Sprintf("pkg_combo_%s", sanitizeID(key)),
			Category:      PatternPackageCombination,
			Description:   fmt.Sprintf("Changing %q and %q together has a %.0f%% failure rate across %d releases", parts[0], parts[1], failRate*100, ps.totalCount),
			Confidence:    confidence,
			EvidenceCount: ps.totalCount,
			RiskModifier:  riskModifier,
			Details: map[string]any{
				"package_a":      parts[0],
				"package_b":      parts[1],
				"failure_rate":   failRate,
				"negative_count": ps.negativeCount,
				"total_count":    ps.totalCount,
			},
			DetectedAt: now,
		})
	}

	return patterns
}

// calculateConfidence computes a confidence score based on sample size and effect strength.
// Uses a logistic function to scale confidence with sample size.
func calculateConfidence(sampleSize int, effectStrength float64) float64 {
	// Logistic growth: confidence increases with sample size, plateauing near 1.0.
	// k controls steepness, x0 controls midpoint.
	k := 0.3
	x0 := 10.0
	sizeComponent := 1.0 / (1.0 + math.Exp(-k*(float64(sampleSize)-x0)))

	// Combine with effect strength.
	confidence := sizeComponent * effectStrength
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// countNegativeOutcomes counts total and negative outcomes.
func countNegativeOutcomes(outcomes []*OutcomeRecord) (negative, total int) {
	for _, o := range outcomes {
		total++
		if o.OutcomeType.IsNegative() {
			negative++
		}
	}
	return
}

// sanitizeID produces a safe identifier from a string.
func sanitizeID(s string) string {
	replacer := strings.NewReplacer("/", "_", ".", "_", " ", "_", "+", "_and_")
	return replacer.Replace(s)
}
