// Package errors provides structured error types for Relicta.
// It implements error classification, wrapping, and recovery detection.
package errors

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Kind represents the category of an error.
type Kind uint8

const (
	// KindUnknown indicates an error of unknown type.
	KindUnknown Kind = iota
	// KindConfig indicates a configuration error.
	KindConfig
	// KindGit indicates a git operation error.
	KindGit
	// KindVersion indicates a versioning error.
	KindVersion
	// KindPlugin indicates a plugin error.
	KindPlugin
	// KindAI indicates an AI service error.
	KindAI
	// KindTemplate indicates a template rendering error.
	KindTemplate
	// KindState indicates a state management error.
	KindState
	// KindNetwork indicates a network error.
	KindNetwork
	// KindIO indicates a file I/O error.
	KindIO
	// KindValidation indicates a validation error.
	KindValidation
	// KindPermission indicates a permission error.
	KindPermission
	// KindNotFound indicates a resource was not found.
	KindNotFound
	// KindConflict indicates a conflict error.
	KindConflict
	// KindTimeout indicates a timeout error.
	KindTimeout
	// KindCanceled indicates the operation was canceled.
	KindCanceled
	// KindInternal indicates an internal error.
	KindInternal
)

// String returns a human-readable string for the error kind.
func (k Kind) String() string {
	switch k {
	case KindConfig:
		return "configuration"
	case KindGit:
		return "git"
	case KindVersion:
		return "version"
	case KindPlugin:
		return "plugin"
	case KindAI:
		return "ai"
	case KindTemplate:
		return "template"
	case KindState:
		return "state"
	case KindNetwork:
		return "network"
	case KindIO:
		return "io"
	case KindValidation:
		return "validation"
	case KindPermission:
		return "permission"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindTimeout:
		return "timeout"
	case KindCanceled:
		return "canceled"
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// Error is the standard error type for Relicta.
type Error struct {
	// Kind is the category of the error.
	Kind Kind
	// Op is the operation being performed when the error occurred.
	Op string
	// Message is a human-readable error message.
	Message string
	// Err is the underlying error.
	Err error
	// Recoverable indicates if the error can be recovered from.
	Recoverable bool
	// Details contains additional context about the error.
	Details map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Op != "" {
		if e.Err != nil {
			return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
		}
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Is reports whether the target error matches this error.
// For *Error types, it checks if both the Kind and Op match.
// For sentinel errors (errors without Op), only Kind is compared.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	// If target has no Op, match by Kind only (sentinel error pattern)
	if t.Op == "" {
		return e.Kind == t.Kind
	}
	// Otherwise, match both Kind and Op
	return e.Kind == t.Kind && e.Op == t.Op
}

// WithDetails adds details to the error and returns the modified error.
func (e *Error) WithDetails(details map[string]any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithDetail adds a single detail to the error and returns the modified error.
func (e *Error) WithDetail(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// New creates a new Error with the given kind and message.
func New(kind Kind, message string) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
	}
}

// Newf creates a new Error with the given kind and formatted message.
func Newf(kind Kind, format string, args ...any) *Error {
	return &Error{
		Kind:    kind,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an existing error with additional context.
func Wrap(err error, kind Kind, op string, message string) *Error {
	return &Error{
		Kind:    kind,
		Op:      op,
		Message: message,
		Err:     err,
	}
}

// Wrapf wraps an existing error with a formatted message.
func Wrapf(err error, kind Kind, op string, format string, args ...any) *Error {
	return &Error{
		Kind:    kind,
		Op:      op,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}

// E is a convenience function to create errors with various arguments.
// Arguments can be of type Kind, string (operation), error, or map[string]any (details).
func E(args ...any) *Error {
	e := &Error{}
	for _, arg := range args {
		switch a := arg.(type) {
		case Kind:
			e.Kind = a
		case string:
			if e.Op == "" {
				e.Op = a
			} else if e.Message == "" {
				e.Message = a
			}
		case *Error:
			e.Err = a
			if e.Kind == KindUnknown {
				e.Kind = a.Kind
			}
		case error:
			e.Err = a
		case map[string]any:
			e.Details = a
		case bool:
			e.Recoverable = a
		}
	}
	return e
}

// GetKind returns the Kind of an error.
// If the error is not an *Error, it returns KindUnknown.
func GetKind(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}

// IsRecoverable returns true if the error is recoverable.
func IsRecoverable(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Recoverable
	}
	return false
}

// IsKind checks if an error is of a specific kind.
func IsKind(err error, kind Kind) bool {
	return GetKind(err) == kind
}

// Common error constructors for frequently used error types.

// Config creates a configuration error.
func Config(op, message string) *Error {
	return &Error{
		Kind:    KindConfig,
		Op:      op,
		Message: message,
	}
}

// ConfigWrap wraps an error as a configuration error.
func ConfigWrap(err error, op, message string) *Error {
	return Wrap(err, KindConfig, op, message)
}

// Git creates a git operation error.
func Git(op, message string) *Error {
	return &Error{
		Kind:    KindGit,
		Op:      op,
		Message: message,
	}
}

// GitWrap wraps an error as a git error.
func GitWrap(err error, op, message string) *Error {
	return Wrap(err, KindGit, op, message)
}

// Version creates a versioning error.
func Version(op, message string) *Error {
	return &Error{
		Kind:    KindVersion,
		Op:      op,
		Message: message,
	}
}

// VersionWrap wraps an error as a versioning error.
func VersionWrap(err error, op, message string) *Error {
	return Wrap(err, KindVersion, op, message)
}

// Plugin creates a plugin error.
func Plugin(op, message string) *Error {
	return &Error{
		Kind:    KindPlugin,
		Op:      op,
		Message: message,
	}
}

// PluginWrap wraps an error as a plugin error.
func PluginWrap(err error, op, message string) *Error {
	return Wrap(err, KindPlugin, op, message)
}

// AI creates an AI service error.
func AI(op, message string) *Error {
	return &Error{
		Kind:    KindAI,
		Op:      op,
		Message: message,
	}
}

// AIWrap wraps an error as an AI service error.
func AIWrap(err error, op, message string) *Error {
	return Wrap(err, KindAI, op, message)
}

// Validation creates a validation error.
func Validation(op, message string) *Error {
	return &Error{
		Kind:        KindValidation,
		Op:          op,
		Message:     message,
		Recoverable: true,
	}
}

// ValidationWrap wraps an error as a validation error.
func ValidationWrap(err error, op, message string) *Error {
	e := Wrap(err, KindValidation, op, message)
	e.Recoverable = true
	return e
}

// NotFound creates a not found error.
func NotFound(op, message string) *Error {
	return &Error{
		Kind:    KindNotFound,
		Op:      op,
		Message: message,
	}
}

// NotFoundWrap wraps an error as a not found error.
func NotFoundWrap(err error, op, message string) *Error {
	return Wrap(err, KindNotFound, op, message)
}

// IO creates an I/O error.
func IO(op, message string) *Error {
	return &Error{
		Kind:    KindIO,
		Op:      op,
		Message: message,
	}
}

// IOWrap wraps an error as an I/O error.
func IOWrap(err error, op, message string) *Error {
	return Wrap(err, KindIO, op, message)
}

// Network creates a network error.
func Network(op, message string) *Error {
	return &Error{
		Kind:        KindNetwork,
		Op:          op,
		Message:     message,
		Recoverable: true,
	}
}

// NetworkWrap wraps an error as a network error.
func NetworkWrap(err error, op, message string) *Error {
	e := Wrap(err, KindNetwork, op, message)
	e.Recoverable = true
	return e
}

// Timeout creates a timeout error.
func Timeout(op, message string) *Error {
	return &Error{
		Kind:        KindTimeout,
		Op:          op,
		Message:     message,
		Recoverable: true,
	}
}

// TimeoutWrap wraps an error as a timeout error.
func TimeoutWrap(err error, op, message string) *Error {
	e := Wrap(err, KindTimeout, op, message)
	e.Recoverable = true
	return e
}

// Internal creates an internal error.
func Internal(op, message string) *Error {
	return &Error{
		Kind:    KindInternal,
		Op:      op,
		Message: message,
	}
}

// InternalWrap wraps an error as an internal error.
func InternalWrap(err error, op, message string) *Error {
	return Wrap(err, KindInternal, op, message)
}

// State creates a state management error.
func State(op, message string) *Error {
	return &Error{
		Kind:    KindState,
		Op:      op,
		Message: message,
	}
}

// StateWrap wraps an error as a state management error.
func StateWrap(err error, op, message string) *Error {
	return Wrap(err, KindState, op, message)
}

// Template creates a template error.
func Template(op, message string) *Error {
	return &Error{
		Kind:    KindTemplate,
		Op:      op,
		Message: message,
	}
}

// TemplateWrap wraps an error as a template error.
func TemplateWrap(err error, op, message string) *Error {
	return Wrap(err, KindTemplate, op, message)
}

// Conflict creates a conflict error.
func Conflict(op, message string) *Error {
	return &Error{
		Kind:    KindConflict,
		Op:      op,
		Message: message,
	}
}

// ConflictWrap wraps an error as a conflict error.
func ConflictWrap(err error, op, message string) *Error {
	return Wrap(err, KindConflict, op, message)
}

// Sensitive data redaction patterns.
// These patterns match common API keys and tokens that should never appear in error messages.
// Word boundaries (\b) are used where applicable to ensure patterns match complete tokens
// and don't accidentally match substrings in unrelated contexts.
var sensitivePatterns = []*regexp.Regexp{
	// OpenAI API keys: sk-..., sk-proj-..., sk-svc-...
	regexp.MustCompile(`\bsk-(?:proj-|svc-)?[a-zA-Z0-9_-]{20,}\b`),
	// Google Gemini API keys: AIza...
	regexp.MustCompile(`\bAIza[a-zA-Z0-9_-]{35,}\b`),
	// GitHub tokens: ghp_..., gho_..., ghs_..., ghr_...
	regexp.MustCompile(`\bgh[posh]_[a-zA-Z0-9]{36,}\b`),
	// Slack webhook URLs - anchored to start of URL to prevent matching embedded URLs
	regexp.MustCompile(`\bhttps://hooks\.slack\.com/services/[A-Z0-9]+/[A-Z0-9]+/[a-zA-Z0-9]+\b`),
	// Generic bearer tokens
	regexp.MustCompile(`\bBearer\s+[a-zA-Z0-9_-]{20,}\b`),
	// Basic auth with password in URL
	regexp.MustCompile(`://[^:]+:[^@]+@`),
}

// RedactSensitive removes sensitive information from an error message.
// It redacts API keys, tokens, and other secrets that should not appear in logs.
func RedactSensitive(s string) string {
	result := s
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// RedactError creates a new error with sensitive data redacted from its message.
// If the error is nil, returns nil.
func RedactError(err error) error {
	if err == nil {
		return nil
	}
	redacted := RedactSensitive(err.Error())
	if redacted == err.Error() {
		return err // No change needed
	}
	return fmt.Errorf("%s", redacted)
}

// AIWrapSafe wraps an error as an AI service error with sensitive data redacted.
// Use this instead of AIWrap when the underlying error might contain API keys or tokens.
func AIWrapSafe(err error, op, message string) *Error {
	if err == nil {
		return AI(op, message)
	}
	// Redact sensitive data from the underlying error
	redactedErr := RedactError(err)
	return Wrap(redactedErr, KindAI, op, message)
}

// WrapSafe wraps an error with sensitive data redacted.
func WrapSafe(err error, kind Kind, op, message string) *Error {
	if err == nil {
		return &Error{
			Kind:    kind,
			Op:      op,
			Message: message,
		}
	}
	redactedErr := RedactError(err)
	return Wrap(redactedErr, kind, op, message)
}

// IsSensitive checks if a string contains sensitive patterns.
func IsSensitive(s string) bool {
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return strings.Contains(s, "api_key") ||
		strings.Contains(s, "apikey") ||
		strings.Contains(s, "secret") ||
		strings.Contains(s, "password") ||
		strings.Contains(s, "token")
}
