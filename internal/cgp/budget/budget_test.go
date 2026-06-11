package budget

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

func TestCheckBudget(t *testing.T) {
	tests := []struct {
		name          string
		currentRisk   float64
		cfg           *config.RiskBudgetConfig
		releases      []*memory.ReleaseRecord
		wantAllowed   bool
		wantReasonSub string
	}{
		{
			name:          "nil config allows all",
			currentRisk:   0.9,
			cfg:           nil,
			releases:      nil,
			wantAllowed:   true,
			wantReasonSub: "no risk budget configured",
		},
		{
			name:        "within budget with no prior releases",
			currentRisk: 0.3,
			cfg: &config.RiskBudgetConfig{
				WeeklyLimit: 2.0,
			},
			releases:      nil,
			wantAllowed:   true,
			wantReasonSub: "within weekly risk budget",
		},
		{
			name:        "within budget with prior releases",
			currentRisk: 0.3,
			cfg: &config.RiskBudgetConfig{
				WeeklyLimit: 2.0,
			},
			releases: []*memory.ReleaseRecord{
				{RiskScore: 0.5, ReleasedAt: time.Now().Add(-1 * time.Hour)},
				{RiskScore: 0.4, ReleasedAt: time.Now().Add(-2 * time.Hour)},
			},
			wantAllowed:   true,
			wantReasonSub: "within weekly risk budget",
		},
		{
			name:        "exceeds budget",
			currentRisk: 0.8,
			cfg: &config.RiskBudgetConfig{
				WeeklyLimit: 2.0,
			},
			releases: []*memory.ReleaseRecord{
				{RiskScore: 0.7, ReleasedAt: time.Now().Add(-1 * time.Hour)},
				{RiskScore: 0.6, ReleasedAt: time.Now().Add(-2 * time.Hour)},
			},
			wantAllowed:   false,
			wantReasonSub: "weekly risk budget exceeded",
		},
		{
			name:        "exactly at limit is allowed",
			currentRisk: 0.5,
			cfg: &config.RiskBudgetConfig{
				WeeklyLimit: 1.0,
			},
			releases: []*memory.ReleaseRecord{
				{RiskScore: 0.5, ReleasedAt: time.Now().Add(-1 * time.Hour)},
			},
			wantAllowed:   true,
			wantReasonSub: "within weekly risk budget",
		},
		{
			name:        "old releases outside current week are excluded",
			currentRisk: 0.5,
			cfg: &config.RiskBudgetConfig{
				WeeklyLimit: 1.0,
			},
			releases: []*memory.ReleaseRecord{
				{RiskScore: 0.9, ReleasedAt: time.Now().Add(-8 * 24 * time.Hour)}, // last week
			},
			wantAllowed:   true,
			wantReasonSub: "within weekly risk budget",
		},
		{
			name:        "zero weekly limit allows all",
			currentRisk: 5.0,
			cfg: &config.RiskBudgetConfig{
				WeeklyLimit: 0,
			},
			releases:      nil,
			wantAllowed:   true,
			wantReasonSub: "within weekly risk budget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckBudget(tt.currentRisk, tt.cfg, tt.releases)

			if result.Allowed != tt.wantAllowed {
				t.Errorf("Allowed = %v, want %v (reason: %s)", result.Allowed, tt.wantAllowed, result.Reason)
			}

			if tt.wantReasonSub != "" && !containsSubstring(result.Reason, tt.wantReasonSub) {
				t.Errorf("Reason = %q, want substring %q", result.Reason, tt.wantReasonSub)
			}
		})
	}
}

func TestCheckBudget_WeeklyReset(t *testing.T) {
	cfg := &config.RiskBudgetConfig{
		WeeklyLimit: 1.0,
	}

	// A release from exactly 7 days ago should be outside the current week window.
	oldRelease := &memory.ReleaseRecord{
		RiskScore:  0.9,
		ReleasedAt: time.Now().Add(-7 * 24 * time.Hour),
	}

	result := CheckBudget(0.5, cfg, []*memory.ReleaseRecord{oldRelease})
	if !result.Allowed {
		t.Errorf("expected release to be allowed after weekly reset, got denied: %s", result.Reason)
	}
}

func TestCheckFreeze(t *testing.T) {
	tests := []struct {
		name          string
		now           time.Time
		freezes       []config.FreezePeriodConfig
		releaseRisk   float64
		wantFrozen    bool
		wantAllowed   bool
		wantReasonSub string
	}{
		{
			name:          "no freeze periods configured",
			now:           time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC), // Friday
			freezes:       nil,
			releaseRisk:   0.5,
			wantFrozen:    false,
			wantAllowed:   true,
			wantReasonSub: "no freeze periods configured",
		},
		{
			name: "outside freeze period",
			now:  time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), // Wednesday
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.1,
				},
			},
			releaseRisk:   0.5,
			wantFrozen:    false,
			wantAllowed:   true,
			wantReasonSub: "no active freeze periods",
		},
		{
			name: "inside freeze period - risk exceeds max",
			now:  time.Date(2026, 3, 21, 20, 0, 0, 0, time.UTC), // Saturday 20:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.1,
				},
			},
			releaseRisk:   0.5,
			wantFrozen:    true,
			wantAllowed:   false,
			wantReasonSub: "freeze period",
		},
		{
			name: "inside freeze period - risk within threshold",
			now:  time.Date(2026, 3, 21, 20, 0, 0, 0, time.UTC), // Saturday 20:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.5,
				},
			},
			releaseRisk:   0.3,
			wantFrozen:    true,
			wantAllowed:   true,
			wantReasonSub: "within allowed threshold",
		},
		{
			name: "freeze period wrapping across midnight",
			now:  time.Date(2026, 3, 21, 2, 0, 0, 0, time.UTC), // Saturday 02:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "late night freeze",
					Start:   "Friday 22:00",
					End:     "Saturday 06:00",
					MaxRisk: 0.0,
				},
			},
			releaseRisk:   0.1,
			wantFrozen:    true,
			wantAllowed:   false,
			wantReasonSub: "freeze period",
		},
		{
			name: "freeze wrapping across week boundary - inside on Sunday",
			now:  time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), // Sunday 10:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.0,
				},
			},
			releaseRisk:   0.1,
			wantFrozen:    true,
			wantAllowed:   false,
			wantReasonSub: "freeze period",
		},
		{
			name: "freeze wrapping across week boundary - outside on Tuesday",
			now:  time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC), // Tuesday 10:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.0,
				},
			},
			releaseRisk:   0.5,
			wantFrozen:    false,
			wantAllowed:   true,
			wantReasonSub: "no active freeze periods",
		},
		{
			name: "multiple freeze periods - first match wins",
			now:  time.Date(2026, 3, 21, 14, 0, 0, 0, time.UTC), // Saturday 14:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "strict weekend",
					Start:   "Saturday 00:00",
					End:     "Sunday 00:00",
					MaxRisk: 0.0,
				},
				{
					Name:    "loose weekend",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.5,
				},
			},
			releaseRisk:   0.1,
			wantFrozen:    true,
			wantAllowed:   false,
			wantReasonSub: "strict weekend",
		},
		{
			name: "malformed freeze period is skipped",
			now:  time.Date(2026, 3, 21, 14, 0, 0, 0, time.UTC), // Saturday
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "bad freeze",
					Start:   "InvalidDay 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.0,
				},
			},
			releaseRisk:   0.5,
			wantFrozen:    false,
			wantAllowed:   true,
			wantReasonSub: "no active freeze periods",
		},
		{
			name: "exact start time is inside freeze",
			now:  time.Date(2026, 3, 20, 16, 0, 0, 0, time.UTC), // Friday 16:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.0,
				},
			},
			releaseRisk:   0.1,
			wantFrozen:    true,
			wantAllowed:   false,
			wantReasonSub: "freeze period",
		},
		{
			name: "exact end time is outside freeze",
			now:  time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC), // Monday 09:00
			freezes: []config.FreezePeriodConfig{
				{
					Name:    "weekend freeze",
					Start:   "Friday 16:00",
					End:     "Monday 09:00",
					MaxRisk: 0.0,
				},
			},
			releaseRisk:   0.5,
			wantFrozen:    false,
			wantAllowed:   true,
			wantReasonSub: "no active freeze periods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckFreeze(tt.now, tt.freezes, tt.releaseRisk)

			if result.Frozen != tt.wantFrozen {
				t.Errorf("Frozen = %v, want %v", result.Frozen, tt.wantFrozen)
			}

			if result.Allowed != tt.wantAllowed {
				t.Errorf("Allowed = %v, want %v (reason: %s)", result.Allowed, tt.wantAllowed, result.Reason)
			}

			if tt.wantReasonSub != "" && !containsSubstring(result.Reason, tt.wantReasonSub) {
				t.Errorf("Reason = %q, want substring %q", result.Reason, tt.wantReasonSub)
			}
		})
	}
}

func TestParseDayTime(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantDay    time.Weekday
		wantHour   int
		wantMinute int
		wantErr    bool
	}{
		{
			name:       "Friday 16:00",
			input:      "Friday 16:00",
			wantDay:    time.Friday,
			wantHour:   16,
			wantMinute: 0,
		},
		{
			name:       "Monday 09:00",
			input:      "Monday 09:00",
			wantDay:    time.Monday,
			wantHour:   9,
			wantMinute: 0,
		},
		{
			name:       "Sunday 23:59",
			input:      "Sunday 23:59",
			wantDay:    time.Sunday,
			wantHour:   23,
			wantMinute: 59,
		},
		{
			name:       "case insensitive",
			input:      "friday 16:00",
			wantDay:    time.Friday,
			wantHour:   16,
			wantMinute: 0,
		},
		{
			name:    "missing time",
			input:   "Friday",
			wantErr: true,
		},
		{
			name:    "invalid day",
			input:   "Fryday 16:00",
			wantErr: true,
		},
		{
			name:    "invalid time format",
			input:   "Friday 16-00",
			wantErr: true,
		},
		{
			name:    "hour out of range",
			input:   "Friday 25:00",
			wantErr: true,
		},
		{
			name:    "minute out of range",
			input:   "Friday 16:60",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, hour, minute, err := parseDayTime(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if day != tt.wantDay {
				t.Errorf("day = %v, want %v", day, tt.wantDay)
			}
			if hour != tt.wantHour {
				t.Errorf("hour = %d, want %d", hour, tt.wantHour)
			}
			if minute != tt.wantMinute {
				t.Errorf("minute = %d, want %d", minute, tt.wantMinute)
			}
		})
	}
}

func TestWeekStart(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		wantDay  time.Weekday
		wantHour int
	}{
		{
			name:     "Wednesday returns Monday",
			input:    time.Date(2026, 3, 18, 15, 30, 0, 0, time.UTC),
			wantDay:  time.Monday,
			wantHour: 0,
		},
		{
			name:     "Monday returns same Monday",
			input:    time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC),
			wantDay:  time.Monday,
			wantHour: 0,
		},
		{
			name:     "Sunday returns previous Monday",
			input:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
			wantDay:  time.Monday,
			wantHour: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := weekStart(tt.input)

			if result.Weekday() != tt.wantDay {
				t.Errorf("weekday = %v, want %v", result.Weekday(), tt.wantDay)
			}
			if result.Hour() != tt.wantHour || result.Minute() != 0 || result.Second() != 0 {
				t.Errorf("time = %02d:%02d:%02d, want 00:00:00", result.Hour(), result.Minute(), result.Second())
			}
		})
	}
}

func TestIsInFreezePeriod(t *testing.T) {
	tests := []struct {
		name    string
		now     time.Time
		start   string
		end     string
		wantIn  bool
		wantErr bool
	}{
		{
			name:   "non-wrapping - inside",
			now:    time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), // Wednesday
			start:  "Monday 09:00",
			end:    "Friday 17:00",
			wantIn: true,
		},
		{
			name:   "non-wrapping - outside before",
			now:    time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC), // Monday 08:00
			start:  "Monday 09:00",
			end:    "Friday 17:00",
			wantIn: false,
		},
		{
			name:   "non-wrapping - outside after",
			now:    time.Date(2026, 3, 20, 18, 0, 0, 0, time.UTC), // Friday 18:00
			start:  "Monday 09:00",
			end:    "Friday 17:00",
			wantIn: false,
		},
		{
			name:   "wrapping - inside on Saturday",
			now:    time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC), // Saturday
			start:  "Friday 16:00",
			end:    "Monday 09:00",
			wantIn: true,
		},
		{
			name:   "wrapping - inside on Sunday",
			now:    time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC), // Sunday
			start:  "Friday 16:00",
			end:    "Monday 09:00",
			wantIn: true,
		},
		{
			name:   "wrapping - outside on Wednesday",
			now:    time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC), // Wednesday
			start:  "Friday 16:00",
			end:    "Monday 09:00",
			wantIn: false,
		},
		{
			name:    "invalid start",
			now:     time.Now(),
			start:   "BadDay 12:00",
			end:     "Monday 09:00",
			wantErr: true,
		},
		{
			name:    "invalid end",
			now:     time.Now(),
			start:   "Friday 16:00",
			end:     "BadDay 09:00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := isInFreezePeriod(tt.now, tt.start, tt.end)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.wantIn {
				t.Errorf("isInFreezePeriod = %v, want %v (now=%s, start=%s, end=%s)",
					result, tt.wantIn, tt.now.Format("Monday 15:04"), tt.start, tt.end)
			}
		})
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstringHelper(s, sub))
}

func containsSubstringHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
