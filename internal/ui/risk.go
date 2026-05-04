// Package ui provides reusable terminal UI components for Relicta.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RiskTier categorizes risk score into discrete severity buckets.
type RiskTier string

const (
	RiskTierLow      RiskTier = "LOW"
	RiskTierMedium   RiskTier = "MEDIUM"
	RiskTierHigh     RiskTier = "HIGH"
	RiskTierCritical RiskTier = "CRITICAL"
)

// TierForScore maps a risk score in [0.0, 1.0] to a tier.
func TierForScore(score float64) RiskTier {
	switch {
	case score >= 0.85:
		return RiskTierCritical
	case score >= 0.7:
		return RiskTierHigh
	case score >= 0.4:
		return RiskTierMedium
	default:
		return RiskTierLow
	}
}

// RiskMeterFactor is a single contributing factor displayed alongside the score.
type RiskMeterFactor struct {
	Category    string
	Description string
	Score       float64 // 0.0–1.0
}

// RenderRisk produces a styled lipgloss block for risk presentation.
//
// Goals:
//   - Highest visual weight on the decision-driving signal (Von Restorff)
//   - Color-blind safe: glyph (▲▲▲ HIGH) carries severity even with NO_COLOR
//   - APCA-compliant pairs (no red-on-black or green-only encoding)
//   - Renders consistently across TUI and (via the same shape) future web/MCP
//     surfaces — see Approval Card schema work in 5.21
//
// When stdout is not a TTY or NO_COLOR is set, output degrades to plain text
// with severity glyphs intact.
func RenderRisk(score float64, factors []RiskMeterFactor) string {
	tier := TierForScore(score)

	header := riskHeader(tier, score)
	bar := progressBar(score, 40)
	lines := []string{header, bar}

	if len(factors) > 0 {
		lines = append(lines, "")
		lines = append(lines, riskFactorsHeader())
		for _, f := range factors {
			lines = append(lines, riskFactorLine(f))
		}
	}

	body := strings.Join(lines, "\n")

	if !colorEnabled() {
		// Plain output keeps glyphs but drops border + colors so output is
		// pipe-friendly for CI logs.
		return body + "\n"
	}

	return riskBoxStyle(tier).Render(body) + "\n"
}

// riskHeader renders the title line with severity glyph, percent, and tier label.
func riskHeader(tier RiskTier, score float64) string {
	pct := score * 100
	glyph := tierGlyph(tier)
	title := fmt.Sprintf("%s  Risk %.1f%% — %s", glyph, pct, tier)

	if !colorEnabled() {
		return title
	}
	return tierTitleStyle(tier).Render(title)
}

// progressBar renders a horizontal score bar `width` runes wide.
func progressBar(score float64, width int) string {
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	filled := int(float64(width) * score)
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	if !colorEnabled() {
		return bar
	}
	tier := TierForScore(score)
	return tierBarStyle(tier).Render(bar)
}

func riskFactorsHeader() string {
	if !colorEnabled() {
		return "Risk Factors:"
	}
	return lipgloss.NewStyle().Bold(true).Render("Risk Factors:")
}

func riskFactorLine(f RiskMeterFactor) string {
	pct := f.Score * 100
	line := fmt.Sprintf("  • [%s] %s (%.1f%%)", f.Category, f.Description, pct)
	if !colorEnabled() {
		return line
	}
	tier := TierForScore(f.Score)
	return tierFactorStyle(tier).Render(line)
}

// tierGlyph returns a Unicode glyph that signals severity even when colors are off.
// Triangle stack pattern is widely intelligible: empty for low, filled rising for high/critical.
func tierGlyph(t RiskTier) string {
	switch t {
	case RiskTierCritical:
		return "▲▲▲▲"
	case RiskTierHigh:
		return "▲▲▲"
	case RiskTierMedium:
		return "▲▲"
	default:
		return "▲"
	}
}

// tierBoxStyle returns the bordered container style for a tier.
func riskBoxStyle(t RiskTier) lipgloss.Style {
	border := lipgloss.RoundedBorder()
	base := lipgloss.NewStyle().
		Border(border).
		Padding(0, 1).
		Margin(0, 0, 1, 0)

	switch t {
	case RiskTierCritical:
		return base.BorderForeground(lipgloss.Color("196"))
	case RiskTierHigh:
		return base.BorderForeground(lipgloss.Color("202"))
	case RiskTierMedium:
		return base.BorderForeground(lipgloss.Color("214"))
	default:
		return base.BorderForeground(lipgloss.Color("42"))
	}
}

// tierTitleStyle returns a bold colored style for the title line.
func tierTitleStyle(t RiskTier) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)
	switch t {
	case RiskTierCritical, RiskTierHigh:
		return base.Foreground(lipgloss.Color("196"))
	case RiskTierMedium:
		return base.Foreground(lipgloss.Color("214"))
	default:
		return base.Foreground(lipgloss.Color("42"))
	}
}

// tierBarStyle colors the progress bar by tier.
func tierBarStyle(t RiskTier) lipgloss.Style {
	switch t {
	case RiskTierCritical, RiskTierHigh:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case RiskTierMedium:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	}
}

// tierFactorStyle subtly colors per-factor lines.
func tierFactorStyle(t RiskTier) lipgloss.Style {
	switch t {
	case RiskTierCritical, RiskTierHigh:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	case RiskTierMedium:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	}
}

// colorEnabled reports whether colored output should render. Honors:
//   - NO_COLOR env (any value disables)
//   - non-TTY stdout (e.g. piped to file or CI log)
func colorEnabled() bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
