// Package security provides security utilities for the application.
package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// VulncheckResult represents the result of a vulnerability scan.
type VulncheckResult struct {
	// Vulnerabilities is the list of found vulnerabilities.
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	// ScannedAt is when the scan was performed.
	ScannedAt time.Time `json:"scanned_at"`
	// ModulePath is the Go module path that was scanned.
	ModulePath string `json:"module_path,omitempty"`
	// Duration is how long the scan took.
	Duration time.Duration `json:"duration"`
	// Error contains any error that occurred during scanning.
	Error string `json:"error,omitempty"`
}

// Vulnerability represents a single vulnerability found by govulncheck.
type Vulnerability struct {
	// ID is the vulnerability ID (e.g., GO-2024-XXXX or CVE-XXXX-XXXX).
	ID string `json:"id"`
	// Aliases are alternative identifiers (e.g., CVE IDs).
	Aliases []string `json:"aliases,omitempty"`
	// Summary is a short description of the vulnerability.
	Summary string `json:"summary"`
	// Details is a longer description.
	Details string `json:"details,omitempty"`
	// Severity is the severity level (CRITICAL, HIGH, MEDIUM, LOW).
	Severity string `json:"severity,omitempty"`
	// AffectedPackages lists the affected Go packages.
	AffectedPackages []string `json:"affected_packages,omitempty"`
	// FixedVersion is the version that fixes this vulnerability.
	FixedVersion string `json:"fixed_version,omitempty"`
	// References are links to more information.
	References []string `json:"references,omitempty"`
}

// HasVulnerabilities returns true if any vulnerabilities were found.
func (r *VulncheckResult) HasVulnerabilities() bool {
	return len(r.Vulnerabilities) > 0
}

// HasCritical returns true if any critical vulnerabilities were found.
func (r *VulncheckResult) HasCritical() bool {
	for _, v := range r.Vulnerabilities {
		if strings.EqualFold(v.Severity, "CRITICAL") {
			return true
		}
	}
	return false
}

// HasHighOrAbove returns true if any high or critical vulnerabilities were found.
func (r *VulncheckResult) HasHighOrAbove() bool {
	for _, v := range r.Vulnerabilities {
		sev := strings.ToUpper(v.Severity)
		if sev == "CRITICAL" || sev == "HIGH" {
			return true
		}
	}
	return false
}

// Summary returns a human-readable summary of the scan results.
func (r *VulncheckResult) Summary() string {
	if r.Error != "" {
		return fmt.Sprintf("Scan failed: %s", r.Error)
	}
	if !r.HasVulnerabilities() {
		return "No known vulnerabilities found"
	}

	var critical, high, medium, low int
	for _, v := range r.Vulnerabilities {
		switch strings.ToUpper(v.Severity) {
		case "CRITICAL":
			critical++
		case "HIGH":
			high++
		case "MEDIUM":
			medium++
		default:
			low++
		}
	}

	parts := []string{}
	if critical > 0 {
		parts = append(parts, fmt.Sprintf("%d critical", critical))
	}
	if high > 0 {
		parts = append(parts, fmt.Sprintf("%d high", high))
	}
	if medium > 0 {
		parts = append(parts, fmt.Sprintf("%d medium", medium))
	}
	if low > 0 {
		parts = append(parts, fmt.Sprintf("%d low", low))
	}

	return fmt.Sprintf("Found %d vulnerabilities: %s", len(r.Vulnerabilities), strings.Join(parts, ", "))
}

// VulncheckScanner provides vulnerability scanning functionality.
type VulncheckScanner struct {
	// GovulncheckPath is the path to the govulncheck binary.
	// If empty, it will be looked up in PATH.
	GovulncheckPath string
	// Timeout is the maximum time to wait for the scan.
	Timeout time.Duration
}

// NewVulncheckScanner creates a new vulnerability scanner.
func NewVulncheckScanner() *VulncheckScanner {
	return &VulncheckScanner{
		Timeout: 5 * time.Minute,
	}
}

// govulncheckOutput represents the JSON output structure from govulncheck.
// This is a simplified version - govulncheck outputs multiple JSON objects.
type govulncheckFinding struct {
	Finding *struct {
		OSV          string `json:"osv"`
		FixedVersion string `json:"fixed_version"`
		Trace        []struct {
			Module  string `json:"module"`
			Package string `json:"package"`
		} `json:"trace"`
	} `json:"finding"`
	OSV *struct {
		ID       string   `json:"id"`
		Aliases  []string `json:"aliases"`
		Summary  string   `json:"summary"`
		Details  string   `json:"details"`
		Severity []struct {
			Type  string `json:"type"`
			Score string `json:"score"`
		} `json:"severity"`
		References []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"references"`
	} `json:"osv"`
}

// Scan runs govulncheck on the specified directory and returns the results.
func (s *VulncheckScanner) Scan(ctx context.Context, dir string) (*VulncheckResult, error) {
	start := time.Now()
	result := &VulncheckResult{
		ScannedAt: start,
	}

	// Find govulncheck binary
	govulncheckPath := s.GovulncheckPath
	if govulncheckPath == "" {
		var err error
		govulncheckPath, err = exec.LookPath("govulncheck")
		if err != nil {
			result.Error = "govulncheck not found in PATH - install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
			result.Duration = time.Since(start)
			return result, fmt.Errorf("govulncheck not found: %w", err)
		}
	} else {
		// Validate that the explicit path exists
		if _, err := exec.LookPath(govulncheckPath); err != nil {
			result.Error = fmt.Sprintf("govulncheck not found at %s", govulncheckPath)
			result.Duration = time.Since(start)
			return result, fmt.Errorf("govulncheck not found at %s: %w", govulncheckPath, err)
		}
	}

	// Set up timeout
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run govulncheck with JSON output
	cmd := exec.CommandContext(ctx, govulncheckPath, "-json", "./...")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(start)

	// Parse the JSON output (govulncheck outputs multiple JSON objects, one per line)
	vulnMap := make(map[string]*Vulnerability)
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var finding govulncheckFinding
		if decErr := decoder.Decode(&finding); decErr != nil {
			continue // Skip malformed entries
		}

		// Process OSV entries (vulnerability definitions)
		if finding.OSV != nil {
			osv := finding.OSV
			vuln := &Vulnerability{
				ID:      osv.ID,
				Aliases: osv.Aliases,
				Summary: osv.Summary,
				Details: osv.Details,
			}

			// Extract severity
			for _, sev := range osv.Severity {
				if sev.Type == "CVSS_V3" {
					vuln.Severity = classifySeverity(sev.Score)
					break
				}
			}

			// Extract references
			for _, ref := range osv.References {
				if ref.URL != "" {
					vuln.References = append(vuln.References, ref.URL)
				}
			}

			vulnMap[osv.ID] = vuln
		}

		// Process finding entries (actual matches in code)
		if finding.Finding != nil {
			if vuln, ok := vulnMap[finding.Finding.OSV]; ok {
				vuln.FixedVersion = finding.Finding.FixedVersion
				for _, trace := range finding.Finding.Trace {
					if trace.Package != "" && !contains(vuln.AffectedPackages, trace.Package) {
						vuln.AffectedPackages = append(vuln.AffectedPackages, trace.Package)
					}
				}
			}
		}
	}

	// Collect vulnerabilities that have actual findings
	for _, vuln := range vulnMap {
		if len(vuln.AffectedPackages) > 0 {
			result.Vulnerabilities = append(result.Vulnerabilities, *vuln)
		}
	}

	// Check for errors
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "scan timed out"
			return result, fmt.Errorf("govulncheck timed out after %v", timeout)
		}
		// govulncheck exits with non-zero if vulnerabilities are found
		// This is expected behavior, not an error
		if len(result.Vulnerabilities) == 0 && stderr.Len() > 0 {
			result.Error = stderr.String()
		}
	}

	return result, nil
}

// IsAvailable checks if govulncheck is available.
func (s *VulncheckScanner) IsAvailable() bool {
	if s.GovulncheckPath != "" {
		_, err := exec.LookPath(s.GovulncheckPath)
		return err == nil
	}
	_, err := exec.LookPath("govulncheck")
	return err == nil
}

// classifySeverity converts a CVSS score to a severity level.
func classifySeverity(score string) string {
	// CVSS v3 scores: 0.0-3.9 Low, 4.0-6.9 Medium, 7.0-8.9 High, 9.0-10.0 Critical
	var scoreFloat float64
	if _, err := fmt.Sscanf(score, "%f", &scoreFloat); err != nil {
		return "UNKNOWN"
	}

	switch {
	case scoreFloat >= 9.0:
		return "CRITICAL"
	case scoreFloat >= 7.0:
		return "HIGH"
	case scoreFloat >= 4.0:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SecurityAdvisory generates a security advisory from scan results.
type SecurityAdvisory struct {
	// Title is the advisory title.
	Title string `json:"title"`
	// Description is the advisory description.
	Description string `json:"description"`
	// Vulnerabilities lists the vulnerabilities addressed.
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	// Recommendations lists recommended actions.
	Recommendations []string `json:"recommendations"`
	// GeneratedAt is when the advisory was generated.
	GeneratedAt time.Time `json:"generated_at"`
}

// GenerateAdvisory creates a security advisory from scan results.
func GenerateAdvisory(result *VulncheckResult, version string) *SecurityAdvisory {
	if !result.HasVulnerabilities() {
		return nil
	}

	advisory := &SecurityAdvisory{
		Title:           fmt.Sprintf("Security Advisory for %s", version),
		Vulnerabilities: result.Vulnerabilities,
		GeneratedAt:     time.Now(),
	}

	// Generate description
	var desc strings.Builder
	fmt.Fprintf(&desc, "This release addresses %d known vulnerabilities:\n\n", len(result.Vulnerabilities))

	for i, v := range result.Vulnerabilities {
		fmt.Fprintf(&desc, "%d. **%s**: %s\n", i+1, v.ID, v.Summary)
		if v.FixedVersion != "" {
			fmt.Fprintf(&desc, "   - Fixed in: %s\n", v.FixedVersion)
		}
	}

	advisory.Description = desc.String()

	// Generate recommendations
	advisory.Recommendations = []string{
		"Update to this version as soon as possible",
		"Review the vulnerability details for potential impact assessment",
		"Check for any security-related configuration changes needed",
	}

	if result.HasCritical() {
		advisory.Recommendations = append([]string{
			"CRITICAL: Immediate update required - critical vulnerabilities found",
		}, advisory.Recommendations...)
	}

	return advisory
}
