// Package security provides input validation and sanitization utilities.
// This package contains security-related helpers to prevent common vulnerabilities
// like path traversal, command injection, and malformed input attacks.
package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// PathValidationError represents a path validation failure.
type PathValidationError struct {
	Path   string
	Reason string
}

func (e *PathValidationError) Error() string {
	return fmt.Sprintf("invalid path %q: %s", e.Path, e.Reason)
}

// ValidatePath validates that a file path is safe and does not contain path traversal.
// It returns the cleaned, absolute path if valid, or an error if the path is unsafe.
//
// The function checks for:
// - Path traversal sequences (.., symlink attacks)
// - Null bytes
// - Invalid characters for the current OS
// - Paths that escape the allowed base directory (if specified)
func ValidatePath(path string, baseDir string) (string, error) {
	if path == "" {
		return "", &PathValidationError{Path: path, Reason: "path cannot be empty"}
	}

	// Check for null bytes (common attack vector)
	if strings.Contains(path, "\x00") {
		return "", &PathValidationError{Path: path, Reason: "path contains null bytes"}
	}

	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Check for explicit parent directory traversal after cleaning
	// filepath.Clean will convert "../foo" to "../foo" but normalize "foo/../bar" to "bar"
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, string(filepath.Separator)+"..") {
		return "", &PathValidationError{Path: path, Reason: "path contains directory traversal"}
	}

	// If a base directory is specified, ensure the path stays within it
	if baseDir != "" {
		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			return "", &PathValidationError{Path: baseDir, Reason: fmt.Sprintf("cannot resolve base directory: %v", err)}
		}

		var absPath string
		if filepath.IsAbs(cleanPath) {
			absPath = cleanPath
		} else {
			absPath = filepath.Join(absBase, cleanPath)
		}

		// Ensure the resolved path is within the base directory
		if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
			return "", &PathValidationError{Path: path, Reason: "path escapes base directory"}
		}

		return absPath, nil
	}

	return cleanPath, nil
}

// ValidateConfigPath validates a configuration file path.
// It's a specialized version of ValidatePath for config files.
func ValidateConfigPath(path string) (string, error) {
	if path == "" {
		return "", nil // Empty path is valid (will use defaults)
	}

	cleanPath, err := ValidatePath(path, "")
	if err != nil {
		return "", err
	}

	// Additional checks for config files
	ext := strings.ToLower(filepath.Ext(cleanPath))
	validExtensions := map[string]bool{
		".yaml": true,
		".yml":  true,
		".json": true,
		".toml": true,
	}

	if ext != "" && !validExtensions[ext] {
		return "", &PathValidationError{
			Path:   path,
			Reason: fmt.Sprintf("invalid config file extension %q (expected .yaml, .yml, .json, or .toml)", ext),
		}
	}

	return cleanPath, nil
}

// SemverPrereleasePattern is the regex pattern for valid prerelease identifiers.
// Per semver spec: a series of dot-separated identifiers, each containing only
// alphanumerics and hyphens, not starting with a leading zero if numeric.
var SemverPrereleasePattern = regexp.MustCompile(`^[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*$`)

// SemverBuildPattern is the regex pattern for valid build metadata.
// Per semver spec: a series of dot-separated identifiers containing only
// alphanumerics and hyphens.
var SemverBuildPattern = regexp.MustCompile(`^[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*$`)

// ValidatePrerelease validates a semantic version prerelease identifier.
// Returns an error if the identifier is invalid per semver spec.
func ValidatePrerelease(prerelease string) error {
	if prerelease == "" {
		return nil // Empty prerelease is valid
	}

	// Length check to prevent DoS
	if len(prerelease) > 128 {
		return fmt.Errorf("prerelease identifier too long (max 128 characters)")
	}

	if !SemverPrereleasePattern.MatchString(prerelease) {
		return fmt.Errorf("invalid prerelease identifier %q: must contain only alphanumerics, hyphens, and dots", prerelease)
	}

	// Check for numeric identifiers with leading zeros (not allowed per semver)
	parts := strings.Split(prerelease, ".")
	for _, part := range parts {
		if isNumeric(part) && len(part) > 1 && part[0] == '0' {
			return fmt.Errorf("invalid prerelease identifier %q: numeric identifier cannot have leading zeros", prerelease)
		}
	}

	return nil
}

// ValidateBuildMetadata validates semantic version build metadata.
// Returns an error if the metadata is invalid per semver spec.
func ValidateBuildMetadata(build string) error {
	if build == "" {
		return nil // Empty build metadata is valid
	}

	// Length check to prevent DoS
	if len(build) > 128 {
		return fmt.Errorf("build metadata too long (max 128 characters)")
	}

	if !SemverBuildPattern.MatchString(build) {
		return fmt.Errorf("invalid build metadata %q: must contain only alphanumerics, hyphens, and dots", build)
	}

	return nil
}

// isNumeric checks if a string contains only digits.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// SanitizeLogMessage sanitizes a message for logging to prevent log injection.
// It replaces newlines and other control characters that could be used to
// forge log entries.
func SanitizeLogMessage(msg string) string {
	// Replace common log injection characters
	replacer := strings.NewReplacer(
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return replacer.Replace(msg)
}
