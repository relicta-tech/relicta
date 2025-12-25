package policy

import (
	"time"
)

// TimeContext provides time-related fields for policy evaluation.
type TimeContext struct {
	// Now is the current timestamp.
	Now time.Time

	// BusinessHours defines when releases are allowed.
	BusinessHours BusinessHoursConfig

	// FreezePeriods defines blackout windows.
	FreezePeriods []FreezePeriod
}

// BusinessHoursConfig defines business hours for releases.
type BusinessHoursConfig struct {
	// StartHour is the start of business hours (0-23, local time).
	StartHour int

	// EndHour is the end of business hours (0-23, local time).
	EndHour int

	// Timezone is the timezone for business hours (e.g., "America/New_York").
	// If empty, uses local time.
	Timezone string

	// AllowWeekends allows releases on weekends.
	AllowWeekends bool
}

// FreezePeriod represents a release blackout window.
type FreezePeriod struct {
	// Name identifies the freeze period.
	Name string

	// Start is the beginning of the freeze period.
	Start time.Time

	// End is the end of the freeze period.
	End time.Time

	// Reason explains why releases are frozen.
	Reason string

	// Severity indicates how strict the freeze is.
	// "soft" allows overrides, "hard" blocks all releases.
	Severity string
}

// DefaultBusinessHours returns standard business hours config.
func DefaultBusinessHours() BusinessHoursConfig {
	return BusinessHoursConfig{
		StartHour:     9,  // 9 AM
		EndHour:       17, // 5 PM
		AllowWeekends: false,
	}
}

// DefaultTimeContext returns a time context with current time and defaults.
func DefaultTimeContext() *TimeContext {
	return &TimeContext{
		Now:           time.Now(),
		BusinessHours: DefaultBusinessHours(),
		FreezePeriods: []FreezePeriod{},
	}
}

// NewTimeContext creates a time context with custom configuration.
func NewTimeContext(businessHours BusinessHoursConfig, freezePeriods []FreezePeriod) *TimeContext {
	return &TimeContext{
		Now:           time.Now(),
		BusinessHours: businessHours,
		FreezePeriods: freezePeriods,
	}
}

// WithTime sets a specific time (useful for testing).
func (tc *TimeContext) WithTime(t time.Time) *TimeContext {
	tc.Now = t
	return tc
}

// IsBusinessHours returns true if the current time is within business hours.
func (tc *TimeContext) IsBusinessHours() bool {
	t := tc.getLocalTime()

	// Check weekend
	if !tc.BusinessHours.AllowWeekends && tc.IsWeekend() {
		return false
	}

	hour := t.Hour()
	return hour >= tc.BusinessHours.StartHour && hour < tc.BusinessHours.EndHour
}

// IsWeekend returns true if the current time is on a weekend.
func (tc *TimeContext) IsWeekend() bool {
	t := tc.getLocalTime()
	weekday := t.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// IsFreezePeriod returns true if currently in any freeze period.
func (tc *TimeContext) IsFreezePeriod() bool {
	_, found := tc.ActiveFreezePeriod()
	return found
}

// IsSoftFreeze returns true if in a soft freeze (can be overridden).
func (tc *TimeContext) IsSoftFreeze() bool {
	freeze, found := tc.ActiveFreezePeriod()
	return found && freeze.Severity == "soft"
}

// IsHardFreeze returns true if in a hard freeze (no releases allowed).
func (tc *TimeContext) IsHardFreeze() bool {
	freeze, found := tc.ActiveFreezePeriod()
	return found && freeze.Severity == "hard"
}

// ActiveFreezePeriod returns the current active freeze period, if any.
func (tc *TimeContext) ActiveFreezePeriod() (FreezePeriod, bool) {
	for _, freeze := range tc.FreezePeriods {
		if tc.Now.After(freeze.Start) && tc.Now.Before(freeze.End) {
			return freeze, true
		}
	}
	return FreezePeriod{}, false
}

// Hour returns the current hour (0-23).
func (tc *TimeContext) Hour() int {
	return tc.getLocalTime().Hour()
}

// Weekday returns the current weekday name.
func (tc *TimeContext) Weekday() string {
	return tc.getLocalTime().Weekday().String()
}

// WeekdayNum returns the weekday as a number (0 = Sunday, 6 = Saturday).
func (tc *TimeContext) WeekdayNum() int {
	return int(tc.getLocalTime().Weekday())
}

// DayOfMonth returns the day of the month (1-31).
func (tc *TimeContext) DayOfMonth() int {
	return tc.getLocalTime().Day()
}

// Month returns the month number (1-12).
func (tc *TimeContext) Month() int {
	return int(tc.getLocalTime().Month())
}

// MonthName returns the month name.
func (tc *TimeContext) MonthName() string {
	return tc.getLocalTime().Month().String()
}

// IsEndOfWeek returns true if it's Friday.
func (tc *TimeContext) IsEndOfWeek() bool {
	return tc.getLocalTime().Weekday() == time.Friday
}

// IsEndOfMonth returns true if it's the last day of the month.
func (tc *TimeContext) IsEndOfMonth() bool {
	t := tc.getLocalTime()
	nextDay := t.AddDate(0, 0, 1)
	return nextDay.Day() == 1
}

// IsEndOfQuarter returns true if it's the last day of a quarter.
func (tc *TimeContext) IsEndOfQuarter() bool {
	t := tc.getLocalTime()
	month := t.Month()
	// End of quarter is March 31, June 30, Sept 30, Dec 31
	quarterEndMonths := map[time.Month]int{
		time.March:     31,
		time.June:      30,
		time.September: 30,
		time.December:  31,
	}
	if endDay, ok := quarterEndMonths[month]; ok {
		return t.Day() == endDay
	}
	return false
}

// HoursUntilEndOfBusinessDay returns hours until business hours end.
// Returns 0 if outside business hours.
func (tc *TimeContext) HoursUntilEndOfBusinessDay() int {
	if !tc.IsBusinessHours() {
		return 0
	}
	t := tc.getLocalTime()
	return tc.BusinessHours.EndHour - t.Hour()
}

// getLocalTime returns the time in the configured timezone.
func (tc *TimeContext) getLocalTime() time.Time {
	if tc.BusinessHours.Timezone == "" {
		return tc.Now.Local()
	}
	loc, err := time.LoadLocation(tc.BusinessHours.Timezone)
	if err != nil {
		return tc.Now.Local()
	}
	return tc.Now.In(loc)
}

// ToEvalContext converts the time context to a map for policy evaluation.
func (tc *TimeContext) ToEvalContext() map[string]any {
	freeze, inFreeze := tc.ActiveFreezePeriod()
	freezeCtx := map[string]any{
		"active":   inFreeze,
		"isSoft":   tc.IsSoftFreeze(),
		"isHard":   tc.IsHardFreeze(),
		"name":     "",
		"reason":   "",
		"severity": "",
	}
	if inFreeze {
		freezeCtx["name"] = freeze.Name
		freezeCtx["reason"] = freeze.Reason
		freezeCtx["severity"] = freeze.Severity
	}

	return map[string]any{
		"hour":             tc.Hour(),
		"weekday":          tc.Weekday(),
		"weekdayNum":       tc.WeekdayNum(),
		"dayOfMonth":       tc.DayOfMonth(),
		"month":            tc.Month(),
		"monthName":        tc.MonthName(),
		"isBusinessHours":  tc.IsBusinessHours(),
		"isWeekend":        tc.IsWeekend(),
		"isEndOfWeek":      tc.IsEndOfWeek(),
		"isEndOfMonth":     tc.IsEndOfMonth(),
		"isEndOfQuarter":   tc.IsEndOfQuarter(),
		"hoursUntilEOD":    tc.HoursUntilEndOfBusinessDay(),
		"freeze":           freezeCtx,
		"timestamp":        tc.Now.Unix(),
		"timestampRFC3339": tc.Now.Format(time.RFC3339),
	}
}
