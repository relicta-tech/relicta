package memory

import (
	"testing"
	"time"
)

// Two copies of this arithmetic existed (insights.go and file_store.go) and both treated
// the tail of the slice as the recent half. That is right for an oldest-first list and
// exactly backwards for a newest-first one — and GetReleaseHistory returns newest first.
// A trend that inverts depending on which reader asked is worse than no trend: both
// answers look equally plausible in a report, and one of them tells an operator that
// rising risk is falling.

func riskAt(score float64, at time.Time) *ReleaseRecord {
	return &ReleaseRecord{
		ID: at.Format(time.RFC3339Nano), Repository: "acme/widget",
		RiskScore: score, ReleasedAt: at, Outcome: OutcomeSuccess,
	}
}

// climbing returns four releases whose risk rises over time, in the given order.
func climbing(newestFirst bool) []*ReleaseRecord {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	out := []*ReleaseRecord{
		riskAt(0.1, base),
		riskAt(0.2, base.Add(time.Hour)),
		riskAt(0.8, base.Add(2*time.Hour)),
		riskAt(0.9, base.Add(3*time.Hour)),
	}
	if newestFirst {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

// The property that matters: the same history yields the same trend regardless of the
// order it arrives in.
func TestRiskTrendIsOrderIndependent(t *testing.T) {
	oldestFirst := RiskTrendOf(climbing(false))
	newestFirst := RiskTrendOf(climbing(true))

	if oldestFirst != newestFirst {
		t.Fatalf("same history, different answers: oldest-first = %q, newest-first = %q",
			oldestFirst, newestFirst)
	}
	if oldestFirst != TrendIncreasing {
		t.Errorf("RiskTrendOf = %q, want increasing for risk climbing 0.1 → 0.9", oldestFirst)
	}
}

func TestRiskTrendDetectsFallingRisk(t *testing.T) {
	falling := climbing(false)
	// Reverse the scores against the timestamps, so risk declines over time.
	for i, j := 0, len(falling)-1; i < j; i, j = i+1, j-1 {
		falling[i].RiskScore, falling[j].RiskScore = falling[j].RiskScore, falling[i].RiskScore
	}

	if got := RiskTrendOf(falling); got != TrendDecreasing {
		t.Errorf("RiskTrendOf = %q, want decreasing", got)
	}
}

// A direction inferred from two releases is noise, and noise presented as a finding in a
// governance decision is worse than an admission of not knowing.
func TestRiskTrendIsStableWithTooLittleHistory(t *testing.T) {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for n, releases := range map[int][]*ReleaseRecord{
		0: nil,
		1: {riskAt(0.9, base)},
		3: {riskAt(0.1, base), riskAt(0.5, base.Add(time.Hour)), riskAt(0.9, base.Add(2*time.Hour))},
	} {
		if got := RiskTrendOf(releases); got != TrendStable {
			t.Errorf("with %d releases: RiskTrendOf = %q, want stable", n, got)
		}
	}
}

// Nearly equal averages are not a trend. Without a tolerance every repository drifts to
// increasing or decreasing on rounding.
func TestRiskTrendIgnoresNoise(t *testing.T) {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	flat := []*ReleaseRecord{
		riskAt(0.50, base),
		riskAt(0.51, base.Add(time.Hour)),
		riskAt(0.49, base.Add(2*time.Hour)),
		riskAt(0.52, base.Add(3*time.Hour)),
	}

	if got := RiskTrendOf(flat); got != TrendStable {
		t.Errorf("RiskTrendOf = %q, want stable for a flat history", got)
	}
}

// A record with no timestamp cannot be placed in time, so it is excluded rather than
// sorted to year one — where it would anchor the "older" half and invert the comparison.
func TestRiskTrendSkipsUndatedReleases(t *testing.T) {
	withUndated := append(climbing(false), &ReleaseRecord{
		ID: "undated", Repository: "acme/widget", RiskScore: 0.9,
	})

	if got := RiskTrendOf(withUndated); got != TrendIncreasing {
		t.Errorf("RiskTrendOf = %q, want increasing; an undated high-risk record must not be "+
			"treated as the oldest release", got)
	}
}
