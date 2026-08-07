package memory

import (
	"fmt"
	"testing"
	"time"
)

// TestWithWindow verifies the WithWindow functional option.
func TestWithWindow(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)

	window := 14 * 24 * time.Hour
	enhancer := NewRiskEnhancer(detector, "owner/repo", WithWindow(window))

	if enhancer.window != window {
		t.Errorf("window = %v, want %v", enhancer.window, window)
	}
}

// TestWithWindow_DefaultValue ensures the default window is 90 days.
func TestWithWindow_DefaultValue(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)

	enhancer := NewRiskEnhancer(detector, "owner/repo")

	expected := 90 * 24 * time.Hour
	if enhancer.window != expected {
		t.Errorf("default window = %v, want %v", enhancer.window, expected)
	}
}

// TestChangeSizeApplies exercises the changeSizeApplies method directly.
func TestChangeSizeApplies(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	tests := []struct {
		name     string
		pattern  Pattern
		changes  []Change
		expected bool
	}{
		{
			name: "in range",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"min_lines": 100,
					"max_lines": 500,
				},
			},
			changes:  []Change{{LinesChanged: 200}},
			expected: true,
		},
		{
			name: "below range",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"min_lines": 100,
					"max_lines": 500,
				},
			},
			changes:  []Change{{LinesChanged: 50}},
			expected: false,
		},
		{
			name: "above range",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"min_lines": 100,
					"max_lines": 500,
				},
			},
			changes:  []Change{{LinesChanged: 600}},
			expected: false,
		},
		{
			name: "missing min_lines",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"max_lines": 500,
				},
			},
			changes:  []Change{{LinesChanged: 200}},
			expected: false,
		},
		{
			name: "missing max_lines",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"min_lines": 100,
				},
			},
			changes:  []Change{{LinesChanged: 200}},
			expected: false,
		},
		{
			name: "multiple changes summed",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"min_lines": float64(100),
					"max_lines": float64(500),
				},
			},
			changes:  []Change{{LinesChanged: 100}, {LinesChanged: 150}},
			expected: true,
		},
		{
			name: "int64 type in details",
			pattern: Pattern{
				Category: PatternChangeSize,
				Details: map[string]any{
					"min_lines": int64(100),
					"max_lines": int64(500),
				},
			},
			changes:  []Change{{LinesChanged: 200}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enhancer.changeSizeApplies(tt.pattern, tt.changes)
			if got != tt.expected {
				t.Errorf("changeSizeApplies() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTimeOfDayApplies exercises the timeOfDayApplies method.
func TestTimeOfDayApplies(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	currentHour := time.Now().Hour()

	tests := []struct {
		name      string
		startHour any
		endHour   any
		wantTrue  bool // whether we expect it to match current time
	}{
		{
			// Windows are half-open [start, end), matching how detectTimePatterns
			// buckets outcomes (its "evening" period is 18-24). A full day is
			// therefore 0-24; end_hour 23 would exclude the 23:00 hour and make
			// this case fail for one hour every day.
			name:      "full day range",
			startHour: 0,
			endHour:   24,
			wantTrue:  true,
		},
		{
			name:      "empty range (start == end)",
			startHour: currentHour,
			endHour:   currentHour,
			wantTrue:  false, // currentHour >= currentHour && currentHour < currentHour → false
		},
		{
			name:      "missing start_hour",
			startHour: nil,
			endHour:   23,
			wantTrue:  false,
		},
		{
			name:      "missing end_hour",
			startHour: 0,
			endHour:   nil,
			wantTrue:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := map[string]any{}
			if tt.startHour != nil {
				details["start_hour"] = tt.startHour
			}
			if tt.endHour != nil {
				details["end_hour"] = tt.endHour
			}
			pattern := Pattern{Category: PatternTimeOfDay, Details: details}
			got := enhancer.timeOfDayApplies(pattern)
			if got != tt.wantTrue {
				t.Errorf("timeOfDayApplies() = %v, want %v (currentHour=%d, start=%v, end=%v)",
					got, tt.wantTrue, currentHour, tt.startHour, tt.endHour)
			}
		})
	}
}

// TestDayOfWeekApplies exercises the dayOfWeekApplies method.
func TestDayOfWeekApplies(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	currentDay := int(time.Now().Weekday())

	tests := []struct {
		name     string
		dayVal   any
		expected bool
	}{
		{"current day matches", currentDay, true},
		{"wrong day", (currentDay + 1) % 7, false},
		{"missing day_of_week", nil, false},
		{"float64 day", float64(currentDay), true},
		{"int64 day", int64(currentDay), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := map[string]any{}
			if tt.dayVal != nil {
				details["day_of_week"] = tt.dayVal
			}
			pattern := Pattern{Category: PatternDayOfWeek, Details: details}
			got := enhancer.dayOfWeekApplies(pattern)
			if got != tt.expected {
				t.Errorf("dayOfWeekApplies() = %v, want %v (currentDay=%d, val=%v)",
					got, tt.expected, currentDay, tt.dayVal)
			}
		})
	}
}

// TestToInt covers all type conversions in the toInt helper.
func TestToInt(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantVal int
		wantOk  bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(100), 100, true},
		{"float64", float64(3.7), 3, true},
		{"string", "42", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toInt(tt.input)
			if ok != tt.wantOk {
				t.Errorf("toInt(%v) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && got != tt.wantVal {
				t.Errorf("toInt(%v) = %v, want %v", tt.input, got, tt.wantVal)
			}
		})
	}
}

// TestClampScore ensures scores are clamped to [0, 1].
func TestClampScore(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.5, 0.5},
		{-0.1, 0.0},
		{1.5, 1.0},
		{0.0, 0.0},
		{1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%.1f", tt.input), func(t *testing.T) {
			got := clampScore(tt.input)
			if got != tt.expected {
				t.Errorf("clampScore(%.1f) = %.1f, want %.1f", tt.input, got, tt.expected)
			}
		})
	}
}

// TestRiskyFileApplies exercises the riskyFileApplies method.
func TestRiskyFileApplies(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	tests := []struct {
		name     string
		pattern  Pattern
		changes  []Change
		expected bool
	}{
		{
			name: "matching file path",
			pattern: Pattern{
				Category: PatternRiskyFiles,
				Details:  map[string]any{"file_path": "main.go"},
			},
			changes:  []Change{{FilePath: "main.go", LinesChanged: 10}},
			expected: true,
		},
		{
			name: "non-matching file path",
			pattern: Pattern{
				Category: PatternRiskyFiles,
				Details:  map[string]any{"file_path": "config.go"},
			},
			changes:  []Change{{FilePath: "main.go", LinesChanged: 10}},
			expected: false,
		},
		{
			name: "missing file_path in details",
			pattern: Pattern{
				Category: PatternRiskyFiles,
				Details:  map[string]any{},
			},
			changes:  []Change{{FilePath: "main.go"}},
			expected: false,
		},
		{
			name: "wrong type for file_path",
			pattern: Pattern{
				Category: PatternRiskyFiles,
				Details:  map[string]any{"file_path": 123},
			},
			changes:  []Change{{FilePath: "main.go"}},
			expected: false,
		},
		{
			name: "multiple changes one matches",
			pattern: Pattern{
				Category: PatternRiskyFiles,
				Details:  map[string]any{"file_path": "db.go"},
			},
			changes:  []Change{{FilePath: "main.go"}, {FilePath: "db.go"}},
			expected: true,
		},
		{
			name: "empty changes",
			pattern: Pattern{
				Category: PatternRiskyFiles,
				Details:  map[string]any{"file_path": "main.go"},
			},
			changes:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enhancer.riskyFileApplies(tt.pattern, tt.changes)
			if got != tt.expected {
				t.Errorf("riskyFileApplies() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestPackageCombinationApplies exercises the packageCombinationApplies method.
func TestPackageCombinationApplies(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	tests := []struct {
		name     string
		pattern  Pattern
		changes  []Change
		expected bool
	}{
		{
			name: "both packages present",
			pattern: Pattern{
				Category: PatternPackageCombination,
				Details: map[string]any{
					"package_a": "auth",
					"package_b": "db",
				},
			},
			changes:  []Change{{Package: "auth"}, {Package: "db"}},
			expected: true,
		},
		{
			name: "only one package present",
			pattern: Pattern{
				Category: PatternPackageCombination,
				Details: map[string]any{
					"package_a": "auth",
					"package_b": "db",
				},
			},
			changes:  []Change{{Package: "auth"}, {Package: "api"}},
			expected: false,
		},
		{
			name: "neither package present",
			pattern: Pattern{
				Category: PatternPackageCombination,
				Details: map[string]any{
					"package_a": "auth",
					"package_b": "db",
				},
			},
			changes:  []Change{{Package: "api"}},
			expected: false,
		},
		{
			name: "missing package_a in details",
			pattern: Pattern{
				Category: PatternPackageCombination,
				Details: map[string]any{
					"package_b": "db",
				},
			},
			changes:  []Change{{Package: "auth"}, {Package: "db"}},
			expected: false,
		},
		{
			name: "missing package_b in details",
			pattern: Pattern{
				Category: PatternPackageCombination,
				Details: map[string]any{
					"package_a": "auth",
				},
			},
			changes:  []Change{{Package: "auth"}, {Package: "db"}},
			expected: false,
		},
		{
			name: "wrong type for package_a",
			pattern: Pattern{
				Category: PatternPackageCombination,
				Details: map[string]any{
					"package_a": 123,
					"package_b": "db",
				},
			},
			changes:  []Change{{Package: "auth"}, {Package: "db"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enhancer.packageCombinationApplies(tt.pattern, tt.changes)
			if got != tt.expected {
				t.Errorf("packageCombinationApplies() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestPatternApplies_AllCategories tests patternApplies dispatches correctly for all categories.
func TestPatternApplies_AllCategories(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	changes := []Change{{FilePath: "main.go", Package: "api", LinesChanged: 50}}

	// PatternRiskyFiles path
	got := enhancer.patternApplies(Pattern{
		Category: PatternRiskyFiles,
		Details:  map[string]any{"file_path": "main.go"},
	}, changes)
	if !got {
		t.Error("patternApplies(PatternRiskyFiles) should match main.go")
	}

	// PatternPackageCombination path with single package
	got = enhancer.patternApplies(Pattern{
		Category: PatternPackageCombination,
		Details: map[string]any{
			"package_a": "api",
			"package_b": "db",
		},
	}, changes)
	if got {
		t.Error("patternApplies(PatternPackageCombination) should not match with only one package")
	}
}

// TestPatternApplies_UnknownCategory checks the default false branch.
func TestPatternApplies_UnknownCategory(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	pattern := Pattern{
		Category: PatternCategory("unknown_category"),
		Details:  map[string]any{},
	}
	changes := []Change{{FilePath: "main.go", LinesChanged: 10}}

	got := enhancer.patternApplies(pattern, changes)
	if got {
		t.Error("patternApplies() should return false for unknown category")
	}
}

// TestEnhanceRiskScoreDetailed_NoPatterns tests detailed mode with no patterns.
func TestEnhanceRiskScoreDetailed_NoPatterns(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	enhancer := NewRiskEnhancer(detector, "owner/repo")

	result, err := enhancer.EnhanceRiskScoreDetailed(t.Context(), 0.5, []Change{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OriginalScore != 0.5 {
		t.Errorf("original score = %v, want 0.5", result.OriginalScore)
	}
	if result.EnhancedScore != 0.5 {
		t.Errorf("enhanced score = %v, want 0.5 (no patterns)", result.EnhancedScore)
	}
}

// TestEnhanceRiskScore_NegativeAdjustmentClamped tests negative adjustment capping.
func TestEnhanceRiskScore_NegativeAdjustmentClamped(t *testing.T) {
	store := NewInMemoryOutcomeStore()
	memStore := NewInMemoryStore()
	detector := NewPatternDetector(store, memStore)
	maxAdj := 0.2
	enhancer := NewRiskEnhancer(detector, "owner/repo", WithMaxAdjustment(maxAdj))

	// maxAdjustment out of range should be ignored.
	enhancerBad := NewRiskEnhancer(detector, "owner/repo", WithMaxAdjustment(1.5))
	if enhancerBad.maxAdjustment != 0.3 { // should stay at default
		t.Errorf("invalid maxAdjustment should be ignored, got %v", enhancerBad.maxAdjustment)
	}

	if enhancer.maxAdjustment != maxAdj {
		t.Errorf("maxAdjustment = %v, want %v", enhancer.maxAdjustment, maxAdj)
	}
}
