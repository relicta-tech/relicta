package policy

import (
	"testing"
	"time"
)

func TestDefaultTimeContext(t *testing.T) {
	tc := DefaultTimeContext()

	if tc == nil {
		t.Fatal("DefaultTimeContext returned nil")
	}

	if tc.Now.IsZero() {
		t.Error("Now should not be zero")
	}

	if tc.BusinessHours.StartHour != 9 {
		t.Errorf("expected StartHour 9, got %d", tc.BusinessHours.StartHour)
	}

	if tc.BusinessHours.EndHour != 17 {
		t.Errorf("expected EndHour 17, got %d", tc.BusinessHours.EndHour)
	}

	if tc.BusinessHours.AllowWeekends {
		t.Error("AllowWeekends should default to false")
	}
}

func TestTimeContext_IsBusinessHours(t *testing.T) {
	tests := []struct {
		name     string
		hour     int
		weekday  time.Weekday
		config   BusinessHoursConfig
		expected bool
	}{
		{
			name:    "during business hours",
			hour:    10,
			weekday: time.Tuesday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: true,
		},
		{
			name:    "before business hours",
			hour:    7,
			weekday: time.Monday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: false,
		},
		{
			name:    "after business hours",
			hour:    18,
			weekday: time.Wednesday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: false,
		},
		{
			name:    "at start of business hours",
			hour:    9,
			weekday: time.Thursday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: true,
		},
		{
			name:    "at end of business hours",
			hour:    17,
			weekday: time.Friday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: false, // 17:00 is end, not included
		},
		{
			name:    "weekend without allow",
			hour:    10,
			weekday: time.Saturday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: false,
		},
		{
			name:    "weekend with allow",
			hour:    10,
			weekday: time.Saturday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: true,
			},
			expected: true,
		},
		{
			name:    "sunday without allow",
			hour:    12,
			weekday: time.Sunday,
			config: BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				AllowWeekends: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a time for the specified hour and weekday
			// Use a known date to control the weekday
			baseDate := time.Date(2024, 1, 1, tt.hour, 0, 0, 0, time.Local)
			// Adjust to get the right weekday
			daysToAdd := int(tt.weekday - baseDate.Weekday())
			if daysToAdd < 0 {
				daysToAdd += 7
			}
			testTime := baseDate.AddDate(0, 0, daysToAdd)

			tc := &TimeContext{
				Now:           testTime,
				BusinessHours: tt.config,
			}

			result := tc.IsBusinessHours()
			if result != tt.expected {
				t.Errorf("IsBusinessHours() = %v, want %v (hour=%d, weekday=%s)",
					result, tt.expected, tt.hour, tt.weekday)
			}
		})
	}
}

func TestTimeContext_IsWeekend(t *testing.T) {
	tests := []struct {
		weekday  time.Weekday
		expected bool
	}{
		{time.Monday, false},
		{time.Tuesday, false},
		{time.Wednesday, false},
		{time.Thursday, false},
		{time.Friday, false},
		{time.Saturday, true},
		{time.Sunday, true},
	}

	for _, tt := range tests {
		t.Run(tt.weekday.String(), func(t *testing.T) {
			baseDate := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)
			daysToAdd := int(tt.weekday - baseDate.Weekday())
			if daysToAdd < 0 {
				daysToAdd += 7
			}
			testTime := baseDate.AddDate(0, 0, daysToAdd)

			tc := &TimeContext{Now: testTime}
			result := tc.IsWeekend()
			if result != tt.expected {
				t.Errorf("IsWeekend() for %s = %v, want %v", tt.weekday, result, tt.expected)
			}
		})
	}
}

func TestTimeContext_FreezePeriods(t *testing.T) {
	now := time.Date(2024, 12, 20, 12, 0, 0, 0, time.UTC)

	tc := &TimeContext{
		Now: now,
		FreezePeriods: []FreezePeriod{
			{
				Name:     "Holiday Freeze",
				Start:    time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
				End:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
				Reason:   "Holiday code freeze",
				Severity: "hard",
			},
		},
	}

	if !tc.IsFreezePeriod() {
		t.Error("expected to be in freeze period")
	}

	if !tc.IsHardFreeze() {
		t.Error("expected hard freeze")
	}

	if tc.IsSoftFreeze() {
		t.Error("expected not to be soft freeze")
	}

	freeze, found := tc.ActiveFreezePeriod()
	if !found {
		t.Error("expected to find active freeze period")
	}
	if freeze.Name != "Holiday Freeze" {
		t.Errorf("expected 'Holiday Freeze', got '%s'", freeze.Name)
	}
}

func TestTimeContext_SoftFreeze(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tc := &TimeContext{
		Now: now,
		FreezePeriods: []FreezePeriod{
			{
				Name:     "Soft Freeze",
				Start:    time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC),
				End:      time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC),
				Reason:   "Minor release window",
				Severity: "soft",
			},
		},
	}

	if !tc.IsFreezePeriod() {
		t.Error("expected to be in freeze period")
	}

	if tc.IsHardFreeze() {
		t.Error("expected not to be hard freeze")
	}

	if !tc.IsSoftFreeze() {
		t.Error("expected soft freeze")
	}
}

func TestTimeContext_NoFreeze(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	tc := &TimeContext{
		Now: now,
		FreezePeriods: []FreezePeriod{
			{
				Name:     "Past Freeze",
				Start:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				End:      time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				Severity: "hard",
			},
		},
	}

	if tc.IsFreezePeriod() {
		t.Error("expected not to be in freeze period")
	}

	if tc.IsHardFreeze() {
		t.Error("expected not to be hard freeze")
	}

	if tc.IsSoftFreeze() {
		t.Error("expected not to be soft freeze")
	}
}

func TestTimeContext_IsEndOfWeek(t *testing.T) {
	tests := []struct {
		weekday  time.Weekday
		expected bool
	}{
		{time.Monday, false},
		{time.Tuesday, false},
		{time.Wednesday, false},
		{time.Thursday, false},
		{time.Friday, true},
		{time.Saturday, false},
		{time.Sunday, false},
	}

	for _, tt := range tests {
		t.Run(tt.weekday.String(), func(t *testing.T) {
			baseDate := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)
			daysToAdd := int(tt.weekday - baseDate.Weekday())
			if daysToAdd < 0 {
				daysToAdd += 7
			}
			testTime := baseDate.AddDate(0, 0, daysToAdd)

			tc := &TimeContext{Now: testTime}
			result := tc.IsEndOfWeek()
			if result != tt.expected {
				t.Errorf("IsEndOfWeek() for %s = %v, want %v", tt.weekday, result, tt.expected)
			}
		})
	}
}

func TestTimeContext_IsEndOfMonth(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected bool
	}{
		{
			name:     "end of January",
			date:     time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "mid January",
			date:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "end of February (leap year)",
			date:     time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "end of April",
			date:     time.Date(2024, 4, 30, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TimeContext{Now: tt.date}
			result := tc.IsEndOfMonth()
			if result != tt.expected {
				t.Errorf("IsEndOfMonth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTimeContext_IsEndOfQuarter(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected bool
	}{
		{
			name:     "end of Q1 (March 31)",
			date:     time.Date(2024, 3, 31, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "end of Q2 (June 30)",
			date:     time.Date(2024, 6, 30, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "end of Q3 (Sept 30)",
			date:     time.Date(2024, 9, 30, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "end of Q4 (Dec 31)",
			date:     time.Date(2024, 12, 31, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "mid quarter",
			date:     time.Date(2024, 2, 15, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "end of month but not quarter",
			date:     time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TimeContext{Now: tt.date}
			result := tc.IsEndOfQuarter()
			if result != tt.expected {
				t.Errorf("IsEndOfQuarter() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTimeContext_ToEvalContext(t *testing.T) {
	// Use local time to avoid timezone conversion issues in tests
	now := time.Date(2024, 12, 20, 14, 30, 0, 0, time.Local)

	tc := &TimeContext{
		Now: now,
		BusinessHours: BusinessHoursConfig{
			StartHour:     9,
			EndHour:       17,
			AllowWeekends: false,
		},
		FreezePeriods: []FreezePeriod{
			{
				Name:     "Holiday Freeze",
				Start:    time.Date(2024, 12, 15, 0, 0, 0, 0, time.Local),
				End:      time.Date(2025, 1, 2, 0, 0, 0, 0, time.Local),
				Reason:   "Holiday code freeze",
				Severity: "hard",
			},
		},
	}

	ctx := tc.ToEvalContext()

	// Check basic fields - hour should be 14 in local time
	if ctx["hour"] != 14 {
		t.Errorf("expected hour 14, got %v", ctx["hour"])
	}

	// Check that weekday is a string (actual day depends on the date)
	if _, ok := ctx["weekday"].(string); !ok {
		t.Errorf("expected weekday to be string, got %T", ctx["weekday"])
	}

	// Check isWeekend is a bool
	if _, ok := ctx["isWeekend"].(bool); !ok {
		t.Errorf("expected isWeekend to be bool, got %T", ctx["isWeekend"])
	}

	// Check freeze context
	freezeCtx, ok := ctx["freeze"].(map[string]any)
	if !ok {
		t.Fatal("freeze context should be a map")
	}

	if freezeCtx["active"] != true {
		t.Errorf("expected freeze active true, got %v", freezeCtx["active"])
	}

	if freezeCtx["isHard"] != true {
		t.Errorf("expected isHard true, got %v", freezeCtx["isHard"])
	}

	if freezeCtx["name"] != "Holiday Freeze" {
		t.Errorf("expected freeze name 'Holiday Freeze', got %v", freezeCtx["name"])
	}
}

func TestEngine_WithTimeContext(t *testing.T) {
	engine := NewEngine(nil, nil)

	customTime := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	tc := &TimeContext{
		Now: customTime,
		BusinessHours: BusinessHoursConfig{
			StartHour:     8,
			EndHour:       18,
			AllowWeekends: true,
		},
	}

	engine.WithTimeContext(tc)

	if engine.timeContext != tc {
		t.Error("WithTimeContext should set the time context")
	}
}

func TestEngine_SetBusinessHours(t *testing.T) {
	engine := NewEngine(nil, nil)

	config := BusinessHoursConfig{
		StartHour:     8,
		EndHour:       20,
		Timezone:      "America/New_York",
		AllowWeekends: true,
	}

	engine.SetBusinessHours(config)

	if engine.timeContext.BusinessHours != config {
		t.Error("SetBusinessHours should update the business hours config")
	}
}

func TestEngine_AddFreezePeriod(t *testing.T) {
	engine := NewEngine(nil, nil)

	freeze := FreezePeriod{
		Name:     "Test Freeze",
		Start:    time.Now(),
		End:      time.Now().Add(24 * time.Hour),
		Reason:   "Testing",
		Severity: "soft",
	}

	engine.AddFreezePeriod(freeze)

	if len(engine.timeContext.FreezePeriods) != 1 {
		t.Errorf("expected 1 freeze period, got %d", len(engine.timeContext.FreezePeriods))
	}

	if engine.timeContext.FreezePeriods[0].Name != "Test Freeze" {
		t.Error("freeze period should be added correctly")
	}
}
