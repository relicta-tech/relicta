package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

func TestGetOutcomeSymbol(t *testing.T) {
	tests := []struct {
		outcome  memory.ReleaseOutcome
		expected string
	}{
		{memory.OutcomeSuccess, "✓"},
		{memory.OutcomeFailed, "✗"},
		{memory.OutcomeRollback, "↩"},
		{memory.OutcomePartial, "◐"},
		{memory.ReleaseOutcome("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.outcome), func(t *testing.T) {
			result := getOutcomeSymbol(tt.outcome)
			if result != tt.expected {
				t.Errorf("getOutcomeSymbol(%q) = %q, want %q", tt.outcome, result, tt.expected)
			}
		})
	}
}

func TestGetTrendSymbol(t *testing.T) {
	tests := []struct {
		trend    memory.RiskTrend
		expected string
	}{
		{memory.TrendIncreasing, "↑"},
		{memory.TrendDecreasing, "↓"},
		{memory.TrendStable, "→"},
		{memory.RiskTrend("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.trend), func(t *testing.T) {
			result := getTrendSymbol(tt.trend)
			if result != tt.expected {
				t.Errorf("getTrendSymbol(%q) = %q, want %q", tt.trend, result, tt.expected)
			}
		})
	}
}

func TestGetReliabilityLabel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.95, "Excellent"},
		{0.9, "Excellent"},
		{0.85, "Very Good"},
		{0.8, "Very Good"},
		{0.75, "Good"},
		{0.7, "Good"},
		{0.65, "Fair"},
		{0.6, "Fair"},
		{0.55, "Needs Improvement"},
		{0.5, "Needs Improvement"},
		{0.4, "Poor"},
		{0.0, "Poor"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getReliabilityLabel(tt.score)
			if result != tt.expected {
				t.Errorf("getReliabilityLabel(%.2f) = %q, want %q", tt.score, result, tt.expected)
			}
		})
	}
}

func TestCalculateReleaseStats(t *testing.T) {
	tests := []struct {
		name     string
		releases []*memory.ReleaseRecord
		expected releaseStats
	}{
		{
			name:     "empty releases",
			releases: []*memory.ReleaseRecord{},
			expected: releaseStats{total: 0, successful: 0, failed: 0, successRate: 0},
		},
		{
			name: "all successful",
			releases: []*memory.ReleaseRecord{
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeSuccess},
			},
			expected: releaseStats{total: 3, successful: 3, failed: 0, successRate: 1.0},
		},
		{
			name: "mixed outcomes",
			releases: []*memory.ReleaseRecord{
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeFailed},
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomeRollback},
			},
			expected: releaseStats{total: 4, successful: 2, failed: 2, successRate: 0.5},
		},
		{
			name: "partial outcomes",
			releases: []*memory.ReleaseRecord{
				{Outcome: memory.OutcomeSuccess},
				{Outcome: memory.OutcomePartial},
			},
			expected: releaseStats{total: 2, successful: 1, failed: 0, successRate: 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateReleaseStats(tt.releases)
			if result.total != tt.expected.total {
				t.Errorf("total = %d, want %d", result.total, tt.expected.total)
			}
			if result.successful != tt.expected.successful {
				t.Errorf("successful = %d, want %d", result.successful, tt.expected.successful)
			}
			if result.failed != tt.expected.failed {
				t.Errorf("failed = %d, want %d", result.failed, tt.expected.failed)
			}
			if result.successRate != tt.expected.successRate {
				t.Errorf("successRate = %.2f, want %.2f", result.successRate, tt.expected.successRate)
			}
		})
	}
}

func TestExtractRepoFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"http://github.com/owner/repo.git", "owner/repo"},
		{"git@gitlab.com:org/project.git", "org/project"},
		{"https://gitlab.com/org/project.git", "org/project"},
		{"invalid-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := extractRepoFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("extractRepoFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}
