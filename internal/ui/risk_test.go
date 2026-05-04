package ui

import (
	"os"
	"strings"
	"testing"
)

func TestTierForScore(t *testing.T) {
	cases := []struct {
		score float64
		want  RiskTier
	}{
		{0.0, RiskTierLow},
		{0.39, RiskTierLow},
		{0.4, RiskTierMedium},
		{0.69, RiskTierMedium},
		{0.7, RiskTierHigh},
		{0.84, RiskTierHigh},
		{0.85, RiskTierCritical},
		{1.0, RiskTierCritical},
	}
	for _, c := range cases {
		if got := TierForScore(c.score); got != c.want {
			t.Errorf("score %.2f: got %q, want %q", c.score, got, c.want)
		}
	}
}

func TestTierGlyph_DistinctPerTier(t *testing.T) {
	got := map[RiskTier]string{
		RiskTierLow:      tierGlyph(RiskTierLow),
		RiskTierMedium:   tierGlyph(RiskTierMedium),
		RiskTierHigh:     tierGlyph(RiskTierHigh),
		RiskTierCritical: tierGlyph(RiskTierCritical),
	}
	seen := make(map[string]RiskTier)
	for tier, glyph := range got {
		if existing, dup := seen[glyph]; dup {
			t.Errorf("glyph %q used by both %q and %q — should be distinct", glyph, existing, tier)
		}
		seen[glyph] = tier
	}
}

func TestRenderRisk_NoColorMode_HasGlyph(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := RenderRisk(0.75, []RiskMeterFactor{
		{Category: "auth", Description: "touches authentication", Score: 0.6},
	})
	// Severity glyph must appear regardless of color mode.
	if !strings.Contains(out, "▲▲▲") {
		t.Errorf("expected severity glyph for HIGH; got %q", out)
	}
	if !strings.Contains(out, "HIGH") {
		t.Errorf("expected HIGH label; got %q", out)
	}
	if !strings.Contains(out, "Risk Factors") {
		t.Errorf("expected risk factors header")
	}
	if !strings.Contains(out, "auth") {
		t.Errorf("expected factor category in output")
	}
}

func TestRenderRisk_ProgressBar_FillsProportionally(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	low := RenderRisk(0.1, nil)
	high := RenderRisk(0.9, nil)

	lowFilled := strings.Count(low, "█")
	highFilled := strings.Count(high, "█")

	if lowFilled >= highFilled {
		t.Errorf("expected high score to render more filled blocks than low; got low=%d high=%d", lowFilled, highFilled)
	}
}

func TestRenderRisk_HandlesEdgeScores(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	negative := RenderRisk(-0.5, nil)
	overflow := RenderRisk(1.5, nil)

	if !strings.Contains(negative, "LOW") {
		t.Errorf("negative score should clamp to LOW")
	}
	if !strings.Contains(overflow, "CRITICAL") {
		t.Errorf("score > 1 should clamp to CRITICAL")
	}
	// Output must remain a complete string with progress bar present.
	for _, out := range []string{negative, overflow} {
		if !strings.ContainsAny(out, "█░") {
			t.Errorf("missing progress bar runes in %q", out)
		}
	}
}

func TestColorEnabled_NoColorEnvDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if colorEnabled() {
		t.Error("NO_COLOR should disable color")
	}
}

func TestColorEnabled_NonTTYDisables(t *testing.T) {
	// In `go test`, stdout is typically not a char device; this should be false.
	os.Unsetenv("NO_COLOR")
	if colorEnabled() {
		t.Skip("stdout is a char device in this test environment; cannot verify")
	}
}
