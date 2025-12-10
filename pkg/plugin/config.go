// Package plugin provides the public interface for ReleasePilot plugins.
package plugin

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConfigParser provides utilities for parsing plugin configurations.
// It handles type-safe extraction of values from map[string]any config
// with support for environment variable fallbacks.
type ConfigParser struct {
	raw map[string]any
}

// NewConfigParser creates a new ConfigParser for the given config map.
func NewConfigParser(config map[string]any) *ConfigParser {
	if config == nil {
		config = make(map[string]any)
	}
	return &ConfigParser{raw: config}
}

// GetString extracts a string field with optional fallback to environment variables.
// Returns empty string if the field is not found or not a string.
func (p *ConfigParser) GetString(field string, envVars ...string) string {
	if v, ok := p.raw[field].(string); ok && v != "" {
		return v
	}
	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	return ""
}

// GetBool extracts a boolean field.
// Returns false if the field is not found or not a boolean.
func (p *ConfigParser) GetBool(field string) bool {
	if v, ok := p.raw[field].(bool); ok {
		return v
	}
	return false
}

// GetBoolDefault extracts a boolean field with a default value.
func (p *ConfigParser) GetBoolDefault(field string, defaultVal bool) bool {
	if v, ok := p.raw[field].(bool); ok {
		return v
	}
	return defaultVal
}

// GetInt extracts an integer field.
// Handles both int and float64 (JSON numbers are unmarshaled as float64).
// Returns 0 if the field is not found or not a number.
func (p *ConfigParser) GetInt(field string) int {
	switch v := p.raw[field].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	default:
		return 0
	}
}

// GetIntDefault extracts an integer field with a default value.
func (p *ConfigParser) GetIntDefault(field string, defaultVal int) int {
	switch v := p.raw[field].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	default:
		return defaultVal
	}
}

// GetFloat extracts a float64 field.
// Returns 0 if the field is not found or not a number.
func (p *ConfigParser) GetFloat(field string) float64 {
	if v, ok := p.raw[field].(float64); ok {
		return v
	}
	return 0
}

// GetStringSlice extracts a string array field.
// Returns nil if the field is not found or not an array.
// Non-string elements are silently skipped.
func (p *ConfigParser) GetStringSlice(field string) []string {
	arr, ok := p.raw[field].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// GetStringMap extracts a map[string]string field.
// Returns nil if the field is not found or not a map.
// Non-string values are silently skipped.
func (p *ConfigParser) GetStringMap(field string) map[string]string {
	m, ok := p.raw[field].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// Has returns true if the field exists in the config (even if nil).
func (p *ConfigParser) Has(field string) bool {
	_, ok := p.raw[field]
	return ok
}

// Raw returns the underlying config map.
func (p *ConfigParser) Raw() map[string]any {
	return p.raw
}

// ValidationBuilder provides a fluent API for building validation responses.
type ValidationBuilder struct {
	errors []ValidationError
}

// NewValidationBuilder creates a new validation builder.
func NewValidationBuilder() *ValidationBuilder {
	return &ValidationBuilder{
		errors: make([]ValidationError, 0),
	}
}

// AddError adds a validation error with field, message, and code.
func (vb *ValidationBuilder) AddError(field, message, code string) *ValidationBuilder {
	vb.errors = append(vb.errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	})
	return vb
}

// AddRequired adds a "required" validation error for a field.
func (vb *ValidationBuilder) AddRequired(field string) *ValidationBuilder {
	return vb.AddError(field, fmt.Sprintf("%s is required", field), "required")
}

// AddTypeError adds a "type" validation error for a field.
func (vb *ValidationBuilder) AddTypeError(field, expectedType string) *ValidationBuilder {
	return vb.AddError(field, fmt.Sprintf("%s must be %s", field, expectedType), "type")
}

// AddEnumError adds an "enum" validation error for invalid values.
func (vb *ValidationBuilder) AddEnumError(field string, validValues []string) *ValidationBuilder {
	msg := fmt.Sprintf("%s must be one of: %s", field, strings.Join(validValues, ", "))
	return vb.AddError(field, msg, "enum")
}

// AddFormatError adds a "format" validation error.
func (vb *ValidationBuilder) AddFormatError(field, message string) *ValidationBuilder {
	return vb.AddError(field, message, "format")
}

// RequireString validates that a string field is present and non-empty.
func (vb *ValidationBuilder) RequireString(config map[string]any, field string) *ValidationBuilder {
	val, ok := config[field].(string)
	if !ok || val == "" {
		vb.AddRequired(field)
	}
	return vb
}

// RequireStringWithEnv validates a string field with environment variable fallback.
func (vb *ValidationBuilder) RequireStringWithEnv(config map[string]any, field string, envVars ...string) *ValidationBuilder {
	val, ok := config[field].(string)
	if ok && val != "" {
		return vb
	}
	for _, envVar := range envVars {
		if os.Getenv(envVar) != "" {
			return vb
		}
	}
	var envHint string
	if len(envVars) > 0 {
		envHint = fmt.Sprintf(" (or set %s)", strings.Join(envVars, " or "))
	}
	vb.AddError(field, fmt.Sprintf("%s is required%s", field, envHint), "required")
	return vb
}

// ValidateStringSlice validates that all elements in an array are strings.
func (vb *ValidationBuilder) ValidateStringSlice(config map[string]any, field string) *ValidationBuilder {
	arr, ok := config[field].([]any)
	if !ok {
		return vb
	}
	for i, item := range arr {
		if _, ok := item.(string); !ok {
			fieldPath := fmt.Sprintf("%s[%d]", field, i)
			vb.AddTypeError(fieldPath, "string")
		}
	}
	return vb
}

// ValidateRegex validates that a field contains a valid regex pattern.
func (vb *ValidationBuilder) ValidateRegex(config map[string]any, field string) *ValidationBuilder {
	pattern, ok := config[field].(string)
	if !ok || pattern == "" {
		return vb
	}
	if _, err := regexp.Compile(pattern); err != nil {
		vb.AddFormatError(field, fmt.Sprintf("invalid regex pattern: %v", err))
	}
	return vb
}

// ValidateURL validates that a field contains a valid URL.
func (vb *ValidationBuilder) ValidateURL(config map[string]any, field string) *ValidationBuilder {
	urlStr, ok := config[field].(string)
	if !ok || urlStr == "" {
		return vb
	}
	if _, err := url.Parse(urlStr); err != nil {
		vb.AddFormatError(field, "invalid URL format")
	}
	return vb
}

// ValidateEnum validates that a string field contains one of the allowed values.
func (vb *ValidationBuilder) ValidateEnum(config map[string]any, field string, validValues []string) *ValidationBuilder {
	val, ok := config[field].(string)
	if !ok || val == "" {
		return vb
	}
	for _, v := range validValues {
		if val == v {
			return vb
		}
	}
	vb.AddEnumError(field, validValues)
	return vb
}

// HasErrors returns true if any validation errors have been recorded.
func (vb *ValidationBuilder) HasErrors() bool {
	return len(vb.errors) > 0
}

// Errors returns the recorded validation errors.
func (vb *ValidationBuilder) Errors() []ValidationError {
	return vb.errors
}

// Build returns the validation response.
func (vb *ValidationBuilder) Build() *ValidateResponse {
	return &ValidateResponse{
		Valid:  len(vb.errors) == 0,
		Errors: vb.errors,
	}
}

// URLValidator provides URL validation with SSRF protection.
type URLValidator struct {
	scheme       string
	allowedHosts []string
	pathPrefix   string
}

// NewURLValidator creates a new URL validator requiring the given scheme.
func NewURLValidator(scheme string) *URLValidator {
	return &URLValidator{
		scheme: scheme,
	}
}

// WithHosts restricts URLs to specific hosts (SSRF protection).
func (uv *URLValidator) WithHosts(hosts ...string) *URLValidator {
	uv.allowedHosts = hosts
	return uv
}

// WithPathPrefix requires URLs to have a specific path prefix.
func (uv *URLValidator) WithPathPrefix(prefix string) *URLValidator {
	uv.pathPrefix = prefix
	return uv
}

// Validate validates a URL string against the configured rules.
func (uv *URLValidator) Validate(urlString string) error {
	if urlString == "" {
		return fmt.Errorf("URL is required")
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if uv.scheme != "" && parsedURL.Scheme != uv.scheme {
		return fmt.Errorf("URL must use %s scheme", uv.scheme)
	}

	if len(uv.allowedHosts) > 0 {
		hostAllowed := false
		for _, host := range uv.allowedHosts {
			if parsedURL.Host == host {
				hostAllowed = true
				break
			}
		}
		if !hostAllowed {
			return fmt.Errorf("URL host %s is not allowed, must be one of: %s",
				parsedURL.Host, strings.Join(uv.allowedHosts, ", "))
		}
	}

	if uv.pathPrefix != "" && !strings.HasPrefix(parsedURL.Path, uv.pathPrefix) {
		return fmt.Errorf("URL path must start with %s", uv.pathPrefix)
	}

	return nil
}

// ValidateAssetPath validates a file path for use as a release asset.
// It prevents path traversal attacks and ensures the file exists.
func ValidateAssetPath(assetPath string) (string, error) {
	if assetPath == "" {
		return "", fmt.Errorf("asset path cannot be empty")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Clean the path
	cleanPath := filepath.Clean(assetPath)

	// Check for path traversal attempts
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal not allowed in asset path: %s", assetPath)
	}

	// Resolve to absolute path
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(cwd, cleanPath)
	}

	// Evaluate symlinks to get real path
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("asset file does not exist: %s", assetPath)
		}
		return "", fmt.Errorf("failed to resolve asset path: %w", err)
	}

	// Ensure the resolved path is still within the working directory
	// Add trailing separator to prevent partial path matches
	// e.g., /home/user vs /home/user2/evil
	cwdWithSep := cwd
	if !strings.HasSuffix(cwd, string(filepath.Separator)) {
		cwdWithSep = cwd + string(filepath.Separator)
	}

	// Check if realPath equals cwd exactly, or is within cwd
	if realPath != cwd && !strings.HasPrefix(realPath, cwdWithSep) {
		return "", fmt.Errorf("asset path resolves outside working directory: %s", assetPath)
	}

	// Check it's a regular file
	info, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat asset file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("asset path is a directory, not a file: %s", assetPath)
	}

	return realPath, nil
}

// BuildMentionText formats a list of user mentions for messaging platforms.
// It handles various mention formats (with/without @ prefix, special syntax).
func BuildMentionText(mentions []string, format MentionFormat) string {
	if len(mentions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(mentions) * 20)

	for i, m := range mentions {
		if i > 0 {
			sb.WriteString(" ")
		}

		switch format {
		case MentionFormatSlack:
			// Slack uses <@USER_ID> or <!channel>
			if strings.HasPrefix(m, "<") || strings.HasPrefix(m, "<!") {
				sb.WriteString(m)
			} else if strings.HasPrefix(m, "@") {
				sb.WriteString("<")
				sb.WriteString(m)
				sb.WriteString(">")
			} else {
				sb.WriteString("<@")
				sb.WriteString(m)
				sb.WriteString(">")
			}
		case MentionFormatDiscord:
			// Discord uses <@USER_ID>, <@&ROLE_ID>, <#CHANNEL_ID>
			// Special mentions @everyone and @here are used as-is
			if strings.HasPrefix(m, "<@") || strings.HasPrefix(m, "<#") {
				// Already formatted user/role/channel mention
				sb.WriteString(m)
			} else if m == "@everyone" || m == "@here" {
				// Special broadcast mentions - use as-is
				sb.WriteString(m)
			} else {
				// Plain user ID - wrap in <@...>
				sb.WriteString("<@")
				sb.WriteString(m)
				sb.WriteString(">")
			}
		default:
			// Plain format - just add @ if not present
			if !strings.HasPrefix(m, "@") {
				sb.WriteString("@")
			}
			sb.WriteString(m)
		}
	}

	return sb.String()
}

// MentionFormat specifies the format for user mentions.
type MentionFormat int

const (
	// MentionFormatPlain uses simple @username format.
	MentionFormatPlain MentionFormat = iota
	// MentionFormatSlack uses Slack's <@USER_ID> format.
	MentionFormatSlack
	// MentionFormatDiscord uses Discord's <@USER_ID> format.
	MentionFormatDiscord
)
