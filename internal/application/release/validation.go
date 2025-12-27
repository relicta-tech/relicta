// Package release provides application use cases for release management.
package release

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/release"
)

// Validation constants for input limits.
const (
	MaxReleaseIDLength = 64
	MaxApproverLength  = 256
	MaxNotesLength     = 1024 * 1024 // 1MB
	MaxURLLength       = 2048
)

// releaseIDPattern validates release ID format: alphanumeric with hyphens and underscores.
var releaseIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ValidateReleaseID validates a release ID format and length.
func ValidateReleaseID(id release.RunID) error {
	if id == "" {
		return fmt.Errorf("release ID is required")
	}
	if len(id) > MaxReleaseIDLength {
		return fmt.Errorf("release ID too long (max %d characters)", MaxReleaseIDLength)
	}
	if !releaseIDPattern.MatchString(string(id)) {
		return fmt.Errorf("invalid release ID format: must be alphanumeric with hyphens and underscores")
	}
	return nil
}

// ValidateSafeString validates a string for safe CLI usage.
// It checks length limits and rejects control characters that could cause issues.
func ValidateSafeString(s string, fieldName string, maxLen int) error {
	if len(s) > maxLen {
		return fmt.Errorf("%s too long (max %d characters)", fieldName, maxLen)
	}
	// Reject null bytes and common control characters
	if strings.ContainsAny(s, "\x00\n\r\t") {
		return fmt.Errorf("%s contains invalid control characters", fieldName)
	}
	return nil
}

// ValidateURL validates a URL string for format and length.
func ValidateURL(urlStr string, fieldName string) error {
	if urlStr == "" {
		return nil // Empty is valid (optional)
	}
	if len(urlStr) > MaxURLLength {
		return fmt.Errorf("%s too long (max %d characters)", fieldName, MaxURLLength)
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	// Require scheme for remote URLs
	if parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid %s scheme: must be http or https", fieldName)
	}
	return nil
}

// ValidationError collects multiple validation errors.
type ValidationError struct {
	errors []string
}

// NewValidationError creates a new ValidationError.
func NewValidationError() *ValidationError {
	return &ValidationError{errors: make([]string, 0)}
}

// Add adds an error to the collection.
func (v *ValidationError) Add(err error) {
	if err != nil {
		v.errors = append(v.errors, err.Error())
	}
}

// AddMessage adds an error message to the collection.
func (v *ValidationError) AddMessage(msg string) {
	v.errors = append(v.errors, msg)
}

// HasErrors returns true if there are validation errors.
func (v *ValidationError) HasErrors() bool {
	return len(v.errors) > 0
}

// Error returns the combined error message.
func (v *ValidationError) Error() string {
	if len(v.errors) == 0 {
		return ""
	}
	if len(v.errors) == 1 {
		return v.errors[0]
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(v.errors, "; "))
}

// ToError returns nil if no errors, otherwise returns the ValidationError.
func (v *ValidationError) ToError() error {
	if !v.HasErrors() {
		return nil
	}
	return v
}
