// Package security provides security utilities for the application.
package security

import (
	"io"
	"os"
	"sync"

	"github.com/relicta-tech/relicta/internal/errors"
)

// Masker provides secret masking functionality for CLI output.
// It wraps the existing RedactSensitive functionality with additional
// features like global enable/disable and writer wrapping.
type Masker struct {
	enabled bool
	mu      sync.RWMutex
}

// globalMasker is the singleton instance used throughout the application.
var (
	globalMasker = &Masker{enabled: false}
	globalMu     sync.RWMutex
)

// Enable enables secret masking globally.
func Enable() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalMasker.enabled = true
}

// Disable disables secret masking globally.
func Disable() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalMasker.enabled = false
}

// IsEnabled returns true if secret masking is enabled globally.
func IsEnabled() bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalMasker.enabled
}

// EnableInCI automatically enables masking if running in a CI environment.
// Detects common CI environment variables: CI, GITHUB_ACTIONS, GITLAB_CI, etc.
func EnableInCI() {
	ciEnvVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"JENKINS_URL",
		"TRAVIS",
		"BITBUCKET_PIPELINES",
		"AZURE_PIPELINES",
		"TEAMCITY_VERSION",
		"BUILDKITE",
	}

	for _, env := range ciEnvVars {
		if os.Getenv(env) != "" {
			Enable()
			return
		}
	}
}

// Mask redacts sensitive data from a string if masking is enabled.
// If masking is disabled, returns the original string unchanged.
func Mask(s string) string {
	if !IsEnabled() {
		return s
	}
	return errors.RedactSensitive(s)
}

// MaskBytes redacts sensitive data from a byte slice if masking is enabled.
// If masking is disabled, returns the original bytes unchanged.
func MaskBytes(b []byte) []byte {
	if !IsEnabled() {
		return b
	}
	return []byte(errors.RedactSensitive(string(b)))
}

// MaskedWriter wraps an io.Writer to automatically mask sensitive data.
type MaskedWriter struct {
	w io.Writer
}

// NewMaskedWriter creates a new MaskedWriter that wraps the given writer.
func NewMaskedWriter(w io.Writer) *MaskedWriter {
	return &MaskedWriter{w: w}
}

// Write implements io.Writer, masking sensitive data before writing.
func (mw *MaskedWriter) Write(p []byte) (n int, err error) {
	masked := MaskBytes(p)
	// Write the masked data but return the original length
	// to satisfy the io.Writer contract
	_, err = mw.w.Write(masked)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// MaskMap redacts sensitive data from map values.
// This is useful for JSON output where certain fields might contain secrets.
func MaskMap(m map[string]interface{}) map[string]interface{} {
	if !IsEnabled() {
		return m
	}
	return maskMapRecursive(m)
}

// maskMapRecursive recursively masks sensitive data in nested maps.
func maskMapRecursive(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = errors.RedactSensitive(val)
		case map[string]interface{}:
			result[k] = maskMapRecursive(val)
		case []interface{}:
			result[k] = maskSlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

// maskSlice recursively masks sensitive data in slices.
func maskSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case string:
			result[i] = errors.RedactSensitive(val)
		case map[string]interface{}:
			result[i] = maskMapRecursive(val)
		case []interface{}:
			result[i] = maskSlice(val)
		default:
			result[i] = v
		}
	}
	return result
}

// NewMasker creates a new Masker instance.
// This can be used for testing or when you need independent masking control.
func NewMasker() *Masker {
	return &Masker{enabled: false}
}

// Enable enables masking for this Masker instance.
func (m *Masker) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable disables masking for this Masker instance.
func (m *Masker) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled returns true if masking is enabled for this Masker instance.
func (m *Masker) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Mask redacts sensitive data from a string using this Masker instance.
func (m *Masker) Mask(s string) string {
	if !m.IsEnabled() {
		return s
	}
	return errors.RedactSensitive(s)
}
