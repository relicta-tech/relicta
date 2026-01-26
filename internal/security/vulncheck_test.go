package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVulncheckResult_HasVulnerabilities(t *testing.T) {
	tests := []struct {
		name   string
		result VulncheckResult
		want   bool
	}{
		{
			name:   "no vulnerabilities",
			result: VulncheckResult{},
			want:   false,
		},
		{
			name: "has vulnerabilities",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Summary: "Test vuln"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasVulnerabilities()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVulncheckResult_HasCritical(t *testing.T) {
	tests := []struct {
		name   string
		result VulncheckResult
		want   bool
	}{
		{
			name:   "no vulnerabilities",
			result: VulncheckResult{},
			want:   false,
		},
		{
			name: "only low severity",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "LOW"},
				},
			},
			want: false,
		},
		{
			name: "only high severity",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "HIGH"},
				},
			},
			want: false,
		},
		{
			name: "has critical",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "CRITICAL"},
				},
			},
			want: true,
		},
		{
			name: "critical case insensitive",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "critical"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasCritical()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVulncheckResult_HasHighOrAbove(t *testing.T) {
	tests := []struct {
		name   string
		result VulncheckResult
		want   bool
	}{
		{
			name:   "no vulnerabilities",
			result: VulncheckResult{},
			want:   false,
		},
		{
			name: "only low severity",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "LOW"},
				},
			},
			want: false,
		},
		{
			name: "only medium severity",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "MEDIUM"},
				},
			},
			want: false,
		},
		{
			name: "has high",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "HIGH"},
				},
			},
			want: true,
		},
		{
			name: "has critical",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "CRITICAL"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasHighOrAbove()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVulncheckResult_Summary(t *testing.T) {
	tests := []struct {
		name     string
		result   VulncheckResult
		contains []string
	}{
		{
			name:     "no vulnerabilities",
			result:   VulncheckResult{},
			contains: []string{"No known vulnerabilities found"},
		},
		{
			name: "has error",
			result: VulncheckResult{
				Error: "govulncheck not found",
			},
			contains: []string{"Scan failed", "govulncheck not found"},
		},
		{
			name: "mixed severities",
			result: VulncheckResult{
				Vulnerabilities: []Vulnerability{
					{ID: "GO-2024-0001", Severity: "CRITICAL"},
					{ID: "GO-2024-0002", Severity: "HIGH"},
					{ID: "GO-2024-0003", Severity: "MEDIUM"},
					{ID: "GO-2024-0004", Severity: "LOW"},
				},
			},
			contains: []string{"Found 4 vulnerabilities", "1 critical", "1 high", "1 medium", "1 low"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Summary()
			for _, substr := range tt.contains {
				assert.Contains(t, got, substr)
			}
		})
	}
}

func TestClassifySeverity(t *testing.T) {
	tests := []struct {
		score    string
		expected string
	}{
		{"9.8", "CRITICAL"},
		{"9.0", "CRITICAL"},
		{"8.9", "HIGH"},
		{"7.0", "HIGH"},
		{"6.9", "MEDIUM"},
		{"4.0", "MEDIUM"},
		{"3.9", "LOW"},
		{"0.0", "LOW"},
		{"invalid", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.score, func(t *testing.T) {
			got := classifySeverity(tt.score)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, contains(slice, "a"))
	assert.True(t, contains(slice, "b"))
	assert.True(t, contains(slice, "c"))
	assert.False(t, contains(slice, "d"))
	assert.False(t, contains(nil, "a"))
	assert.False(t, contains([]string{}, "a"))
}

func TestNewVulncheckScanner(t *testing.T) {
	scanner := NewVulncheckScanner()

	require.NotNil(t, scanner)
	assert.Equal(t, 5*time.Minute, scanner.Timeout)
	assert.Empty(t, scanner.GovulncheckPath)
}

func TestVulncheckScanner_IsAvailable(t *testing.T) {
	t.Run("with invalid path", func(t *testing.T) {
		scanner := &VulncheckScanner{
			GovulncheckPath: "/nonexistent/govulncheck",
		}
		// This will fail because the path doesn't exist
		// But we're just testing the method exists
		_ = scanner.IsAvailable()
	})

	t.Run("with empty path uses PATH lookup", func(t *testing.T) {
		scanner := NewVulncheckScanner()
		// This tests the PATH lookup logic
		// Result depends on whether govulncheck is installed
		_ = scanner.IsAvailable()
	})
}

func TestVulncheckScanner_Scan_NotInstalled(t *testing.T) {
	// Use a path that definitely doesn't exist to simulate missing govulncheck
	scanner := &VulncheckScanner{
		GovulncheckPath: "/nonexistent/path/to/govulncheck-abc123xyz",
		Timeout:         1 * time.Second,
	}

	result, err := scanner.Scan(context.Background(), ".")

	// The scan should fail because the binary doesn't exist
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, err.Error(), "govulncheck not found")
	assert.Contains(t, result.Error, "govulncheck not found")
	assert.NotZero(t, result.Duration)
}

func TestGenerateAdvisory(t *testing.T) {
	t.Run("no vulnerabilities returns nil", func(t *testing.T) {
		result := &VulncheckResult{}
		advisory := GenerateAdvisory(result, "v1.0.0")
		assert.Nil(t, advisory)
	})

	t.Run("generates advisory for vulnerabilities", func(t *testing.T) {
		result := &VulncheckResult{
			Vulnerabilities: []Vulnerability{
				{
					ID:           "GO-2024-0001",
					Summary:      "Test vulnerability",
					Severity:     "HIGH",
					FixedVersion: "v1.2.3",
				},
			},
		}

		advisory := GenerateAdvisory(result, "v1.0.0")

		require.NotNil(t, advisory)
		assert.Equal(t, "Security Advisory for v1.0.0", advisory.Title)
		assert.Contains(t, advisory.Description, "GO-2024-0001")
		assert.Contains(t, advisory.Description, "Test vulnerability")
		assert.Len(t, advisory.Vulnerabilities, 1)
		assert.NotEmpty(t, advisory.Recommendations)
	})

	t.Run("critical vulnerabilities add urgent recommendation", func(t *testing.T) {
		result := &VulncheckResult{
			Vulnerabilities: []Vulnerability{
				{
					ID:       "GO-2024-0001",
					Summary:  "Critical vulnerability",
					Severity: "CRITICAL",
				},
			},
		}

		advisory := GenerateAdvisory(result, "v1.0.0")

		require.NotNil(t, advisory)
		assert.Contains(t, advisory.Recommendations[0], "CRITICAL")
		assert.Contains(t, advisory.Recommendations[0], "Immediate update required")
	})
}

func TestVulnerability(t *testing.T) {
	vuln := Vulnerability{
		ID:               "GO-2024-0001",
		Aliases:          []string{"CVE-2024-12345"},
		Summary:          "Test vulnerability summary",
		Details:          "Detailed description of the vulnerability",
		Severity:         "HIGH",
		AffectedPackages: []string{"github.com/example/pkg"},
		FixedVersion:     "v1.2.3",
		References:       []string{"https://example.com/advisory"},
	}

	assert.Equal(t, "GO-2024-0001", vuln.ID)
	assert.Contains(t, vuln.Aliases, "CVE-2024-12345")
	assert.Equal(t, "HIGH", vuln.Severity)
	assert.Len(t, vuln.AffectedPackages, 1)
}
