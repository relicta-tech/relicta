// Package budget provides risk budget tracking and freeze period enforcement
// for the Change Governance Protocol (CGP).
//
// Risk budgets limit cumulative risk across a time period (weekly), preventing
// teams from shipping too much risk in a short window. Freeze periods define
// recurring time windows where releases are restricted or blocked based on
// risk thresholds.
package budget

import (
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/config"
)

// BudgetResult contains the outcome of a risk budget check.
type BudgetResult struct {
	// Allowed indicates whether the release is permitted under the budget.
	Allowed bool
	// Reason explains why the release was allowed or denied.
	Reason string
	// RemainingBudget is the risk budget remaining after this release (if allowed).
	RemainingBudget float64
	// UsedBudget is the cumulative risk consumed in the current period.
	UsedBudget float64
	// PeriodStart is the beginning of the current budget period.
	PeriodStart time.Time
}

// FreezeResult contains the outcome of a freeze period check.
type FreezeResult struct {
	// Frozen indicates whether a freeze period is active.
	Frozen bool
	// Allowed indicates whether the release is permitted during the freeze.
	Allowed bool
	// Reason explains why the release was allowed or denied.
	Reason string
	// FreezeName is the name of the active freeze period (if any).
	FreezeName string
	// MaxRisk is the maximum risk allowed during the freeze period.
	MaxRisk float64
}

// CheckBudget evaluates whether a release with the given risk score is
// permitted under the configured risk budget. It sums risk scores from
// recent releases within the current weekly period.
func CheckBudget(currentRisk float64, cfg *config.RiskBudgetConfig, recentReleases []*memory.ReleaseRecord) BudgetResult {
	if cfg == nil {
		return BudgetResult{
			Allowed: true,
			Reason:  "no risk budget configured",
		}
	}

	now := time.Now()
	periodStart := weekStart(now)

	// Sum risk from releases in the current weekly period.
	var usedBudget float64
	for _, r := range recentReleases {
		if !r.ReleasedAt.Before(periodStart) {
			usedBudget += r.RiskScore
		}
	}

	if cfg.WeeklyLimit > 0 {
		projected := usedBudget + currentRisk
		if projected > cfg.WeeklyLimit {
			return BudgetResult{
				Allowed:         false,
				Reason:          fmt.Sprintf("weekly risk budget exceeded: %.2f used + %.2f proposed = %.2f (limit: %.2f)", usedBudget, currentRisk, projected, cfg.WeeklyLimit),
				RemainingBudget: cfg.WeeklyLimit - usedBudget,
				UsedBudget:      usedBudget,
				PeriodStart:     periodStart,
			}
		}
	}

	remaining := float64(0)
	if cfg.WeeklyLimit > 0 {
		remaining = cfg.WeeklyLimit - usedBudget - currentRisk
	}

	return BudgetResult{
		Allowed:         true,
		Reason:          fmt.Sprintf("within weekly risk budget: %.2f used + %.2f proposed (limit: %.2f)", usedBudget, currentRisk, cfg.WeeklyLimit),
		RemainingBudget: remaining,
		UsedBudget:      usedBudget,
		PeriodStart:     periodStart,
	}
}

// CheckFreeze evaluates whether the current time falls within a configured
// freeze period and whether the release risk exceeds the freeze threshold.
func CheckFreeze(now time.Time, freezes []config.FreezePeriodConfig, releaseRisk float64) FreezeResult {
	if len(freezes) == 0 {
		return FreezeResult{
			Allowed: true,
			Reason:  "no freeze periods configured",
		}
	}

	for _, f := range freezes {
		active, err := isInFreezePeriod(now, f.Start, f.End)
		if err != nil {
			// Skip malformed freeze periods rather than blocking releases.
			continue
		}
		if !active {
			continue
		}

		// We are in a freeze period.
		if releaseRisk > f.MaxRisk {
			return FreezeResult{
				Frozen:     true,
				Allowed:    false,
				Reason:     fmt.Sprintf("freeze period %q active: risk %.2f exceeds max allowed %.2f", f.Name, releaseRisk, f.MaxRisk),
				FreezeName: f.Name,
				MaxRisk:    f.MaxRisk,
			}
		}

		return FreezeResult{
			Frozen:     true,
			Allowed:    true,
			Reason:     fmt.Sprintf("freeze period %q active but risk %.2f is within allowed threshold %.2f", f.Name, releaseRisk, f.MaxRisk),
			FreezeName: f.Name,
			MaxRisk:    f.MaxRisk,
		}
	}

	return FreezeResult{
		Allowed: true,
		Reason:  "no active freeze periods",
	}
}

// weekStart returns the start of the ISO week (Monday 00:00:00) containing t.
func weekStart(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - int(time.Monday)
	y, m, d := t.Date()
	return time.Date(y, m, d-daysSinceMonday, 0, 0, 0, 0, t.Location())
}

// parseDayTime parses a "Day HH:MM" string into a weekday and time-of-day.
// Supported day names: Monday through Sunday (case-insensitive).
func parseDayTime(s string) (time.Weekday, int, int, error) {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) != 2 {
		return 0, 0, 0, fmt.Errorf("invalid day+time format %q: expected \"Day HH:MM\"", s)
	}

	day, err := parseWeekday(parts[0])
	if err != nil {
		return 0, 0, 0, err
	}

	var hour, minute int
	if _, err := fmt.Sscanf(parts[1], "%d:%d", &hour, &minute); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid time format %q: expected HH:MM", parts[1])
	}

	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, 0, fmt.Errorf("time out of range: %02d:%02d", hour, minute)
	}

	return day, hour, minute, nil
}

// parseWeekday converts a day name string to time.Weekday.
func parseWeekday(s string) (time.Weekday, error) {
	switch strings.ToLower(s) {
	case "sunday":
		return time.Sunday, nil
	case "monday":
		return time.Monday, nil
	case "tuesday":
		return time.Tuesday, nil
	case "wednesday":
		return time.Wednesday, nil
	case "thursday":
		return time.Thursday, nil
	case "friday":
		return time.Friday, nil
	case "saturday":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday: %q", s)
	}
}

// isInFreezePeriod determines whether the given time falls within the
// recurring weekly freeze window defined by startSpec and endSpec.
// The window may wrap around the end of the week (e.g., Friday to Monday).
func isInFreezePeriod(now time.Time, startSpec, endSpec string) (bool, error) {
	startDay, startHour, startMin, err := parseDayTime(startSpec)
	if err != nil {
		return false, fmt.Errorf("parsing freeze start: %w", err)
	}

	endDay, endHour, endMin, err := parseDayTime(endSpec)
	if err != nil {
		return false, fmt.Errorf("parsing freeze end: %w", err)
	}

	// Convert everything to minutes-since-Sunday-00:00 for comparison.
	startMinutes := dayMinutes(startDay, startHour, startMin)
	endMinutes := dayMinutes(endDay, endHour, endMin)
	nowMinutes := dayMinutes(now.Weekday(), now.Hour(), now.Minute())

	if startMinutes <= endMinutes {
		// Non-wrapping window: e.g., Monday 09:00 to Friday 16:00
		return nowMinutes >= startMinutes && nowMinutes < endMinutes, nil
	}

	// Wrapping window: e.g., Friday 16:00 to Monday 09:00
	// Active if now >= start OR now < end
	return nowMinutes >= startMinutes || nowMinutes < endMinutes, nil
}

// dayMinutes converts a weekday + time to minutes since Sunday 00:00.
func dayMinutes(day time.Weekday, hour, minute int) int {
	return int(day)*24*60 + hour*60 + minute
}
