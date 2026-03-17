// Package errors provides tests for error handling utilities.
package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestRedactSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no sensitive data",
			input:    "connection failed to server",
			expected: "connection failed to server",
		},
		{
			name:     "OpenAI API key",
			input:    "error: invalid key sk-abcdefghijklmnopqrstuvwxyz123456",
			expected: "error: invalid key [REDACTED]",
		},
		{
			name:     "OpenAI project key",
			input:    "failed with sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			expected: "failed with [REDACTED]",
		},
		{
			name:     "GitHub token ghp",
			input:    "auth error: ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			expected: "auth error: [REDACTED]",
		},
		{
			name:     "GitHub token gho",
			input:    "oauth error: gho_abcdefghijklmnopqrstuvwxyz1234567890",
			expected: "oauth error: [REDACTED]",
		},
		{
			name:     "Slack webhook URL",
			input:    "webhook failed: https://hooks.slack.com/services/TTEST/BTEST/testtoken",
			expected: "webhook failed: [REDACTED]",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: [REDACTED]",
		},
		{
			name:     "Basic auth in URL",
			input:    "connecting to https://user:secret123@api.example.com/data",
			expected: "connecting to https[REDACTED]api.example.com/data",
		},
		{
			name:     "multiple sensitive values",
			input:    "key1: sk-abcdefghijklmnopqrstuvwxyz123456, key2: ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			expected: "key1: [REDACTED], key2: [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactSensitive(tt.input)
			if result != tt.expected {
				t.Errorf("RedactSensitive(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRedactError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantNil  bool
		contains string
	}{
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
		{
			name:     "error without sensitive data",
			err:      errors.New("connection timeout"),
			contains: "connection timeout",
		},
		{
			name:     "error with API key",
			err:      fmt.Errorf("failed with key sk-abcdefghijklmnopqrstuvwxyz123456"),
			contains: "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactError(tt.err)
			if tt.wantNil {
				if result != nil {
					t.Errorf("RedactError() = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("RedactError() = nil, want non-nil")
			}
			if tt.contains != "" && !containsString(result.Error(), tt.contains) {
				t.Errorf("RedactError().Error() = %q, want to contain %q", result.Error(), tt.contains)
			}
		})
	}
}

func TestAIWrapSafe(t *testing.T) {
	// Test with sensitive data in underlying error
	sensitiveErr := errors.New("API call failed: sk-abcdefghijklmnopqrstuvwxyz123456")
	wrapped := AIWrapSafe(sensitiveErr, "TestOp", "operation failed")

	if wrapped == nil {
		t.Fatal("AIWrapSafe returned nil")
	}
	if wrapped.Kind != KindAI {
		t.Errorf("AIWrapSafe kind = %v, want KindAI", wrapped.Kind)
	}
	if wrapped.Op != "TestOp" {
		t.Errorf("AIWrapSafe op = %v, want TestOp", wrapped.Op)
	}
	// Check that the error message is redacted
	errStr := wrapped.Error()
	if containsString(errStr, "sk-") {
		t.Errorf("AIWrapSafe error contains sensitive data: %v", errStr)
	}
	if !containsString(errStr, "[REDACTED]") {
		t.Errorf("AIWrapSafe error should contain [REDACTED]: %v", errStr)
	}
}

func TestIsSensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"regular text", false},
		{"sk-abcdefghijklmnopqrstuvwxyz123456", true},
		{"contains api_key reference", true},
		{"has apikey in text", true},
		{"my secret value", true},
		{"password field", true},
		{"access token here", true},
		{"ghp_abcdefghijklmnopqrstuvwxyz1234567890", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsSensitive(tt.input)
			if result != tt.expected {
				t.Errorf("IsSensitive(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestErrorKind(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want Kind
	}{
		{"config error", Config("test", "msg"), KindConfig},
		{"git error", Git("test", "msg"), KindGit},
		{"version error", Version("test", "msg"), KindVersion},
		{"plugin error", Plugin("test", "msg"), KindPlugin},
		{"ai error", AI("test", "msg"), KindAI},
		{"validation error", Validation("test", "msg"), KindValidation},
		{"not found error", NotFound("test", "msg"), KindNotFound},
		{"io error", IO("test", "msg"), KindIO},
		{"network error", Network("test", "msg"), KindNetwork},
		{"timeout error", Timeout("test", "msg"), KindTimeout},
		{"internal error", Internal("test", "msg"), KindInternal},
		{"state error", State("test", "msg"), KindState},
		{"template error", Template("test", "msg"), KindTemplate},
		{"conflict error", Conflict("test", "msg"), KindConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Kind != tt.want {
				t.Errorf("Error kind = %v, want %v", tt.err.Kind, tt.want)
			}
		})
	}
}

func TestGetKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Kind
	}{
		{"nil error", nil, KindUnknown},
		{"standard error", errors.New("test"), KindUnknown},
		{"custom error", Config("op", "msg"), KindConfig},
		{"wrapped custom error", ConfigWrap(errors.New("inner"), "op", "msg"), KindConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetKind(tt.err)
			if got != tt.want {
				t.Errorf("GetKind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRecoverable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"standard error", errors.New("test"), false},
		{"non-recoverable error", Config("op", "msg"), false},
		{"validation error (recoverable)", Validation("op", "msg"), true},
		{"network error (recoverable)", Network("op", "msg"), true},
		{"timeout error (recoverable)", Timeout("op", "msg"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRecoverable(tt.err)
			if got != tt.want {
				t.Errorf("IsRecoverable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorWithDetails(t *testing.T) {
	err := Config("op", "msg")
	err.WithDetail("key1", "value1")
	err.WithDetails(map[string]any{"key2": "value2", "key3": 123})

	if err.Details["key1"] != "value1" {
		t.Errorf("WithDetail key1 = %v, want value1", err.Details["key1"])
	}
	if err.Details["key2"] != "value2" {
		t.Errorf("WithDetails key2 = %v, want value2", err.Details["key2"])
	}
	if err.Details["key3"] != 123 {
		t.Errorf("WithDetails key3 = %v, want 123", err.Details["key3"])
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestKindString tests the String() method of Kind.
func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindUnknown, "unknown"},
		{KindConfig, "configuration"},
		{KindGit, "git"},
		{KindVersion, "version"},
		{KindPlugin, "plugin"},
		{KindAI, "ai"},
		{KindTemplate, "template"},
		{KindState, "state"},
		{KindNetwork, "network"},
		{KindIO, "io"},
		{KindValidation, "validation"},
		{KindPermission, "permission"},
		{KindNotFound, "not_found"},
		{KindConflict, "conflict"},
		{KindTimeout, "timeout"},
		{KindCanceled, "canceled"},
		{KindInternal, "internal"},
		{Kind(255), "unknown"}, // Invalid kind
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("Kind.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorError tests the Error() method with various configurations.
func TestErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "with op and message only",
			err: &Error{
				Op:      "TestOp",
				Message: "test message",
			},
			want: "TestOp: test message",
		},
		{
			name: "with op, message, and underlying error",
			err: &Error{
				Op:      "TestOp",
				Message: "test message",
				Err:     errors.New("underlying error"),
			},
			want: "TestOp: test message: underlying error",
		},
		{
			name: "message only (no op)",
			err: &Error{
				Message: "test message",
			},
			want: "test message",
		},
		{
			name: "message with underlying error (no op)",
			err: &Error{
				Message: "test message",
				Err:     errors.New("underlying error"),
			},
			want: "test message: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorUnwrap tests the Unwrap() method.
func TestErrorUnwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	err := &Error{
		Op:      "TestOp",
		Message: "test message",
		Err:     underlyingErr,
	}

	unwrapped := err.Unwrap()
	if unwrapped != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlyingErr)
	}

	// Test with no underlying error
	errNoUnderlying := &Error{
		Op:      "TestOp",
		Message: "test message",
	}
	if errNoUnderlying.Unwrap() != nil {
		t.Errorf("Unwrap() of error without underlying error should return nil")
	}
}

// TestErrorIs tests the Is() method for error matching.
func TestErrorIs(t *testing.T) {
	tests := []struct {
		name   string
		err    *Error
		target error
		want   bool
	}{
		{
			name:   "match by kind only (sentinel pattern)",
			err:    Config("op", "msg"),
			target: &Error{Kind: KindConfig},
			want:   true,
		},
		{
			name:   "match by kind and op",
			err:    Config("op", "msg"),
			target: Config("op", "different msg"),
			want:   true,
		},
		{
			name:   "different kind",
			err:    Config("op", "msg"),
			target: &Error{Kind: KindGit},
			want:   false,
		},
		{
			name:   "same kind different op",
			err:    Config("op1", "msg"),
			target: Config("op2", "msg"),
			want:   false,
		},
		{
			name:   "non-Error target",
			err:    Config("op", "msg"),
			target: errors.New("standard error"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Is(tt.target)
			if got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNew tests the New() function.
func TestNew(t *testing.T) {
	err := New(KindConfig, "test message")
	if err == nil {
		t.Fatal("New() returned nil")
	}
	if err.Kind != KindConfig {
		t.Errorf("Kind = %v, want %v", err.Kind, KindConfig)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %v, want %v", err.Message, "test message")
	}
}

// TestNewf tests the Newf() function.
func TestNewf(t *testing.T) {
	err := Newf(KindConfig, "test message: %s %d", "foo", 123)
	if err == nil {
		t.Fatal("Newf() returned nil")
	}
	if err.Kind != KindConfig {
		t.Errorf("Kind = %v, want %v", err.Kind, KindConfig)
	}
	if err.Message != "test message: foo 123" {
		t.Errorf("Message = %v, want %v", err.Message, "test message: foo 123")
	}
}

// TestWrap tests the Wrap() function.
func TestWrap(t *testing.T) {
	underlyingErr := errors.New("underlying")
	err := Wrap(underlyingErr, KindConfig, "op", "wrapper message")

	if err.Kind != KindConfig {
		t.Errorf("Kind = %v, want %v", err.Kind, KindConfig)
	}
	if err.Op != "op" {
		t.Errorf("Op = %v, want op", err.Op)
	}
	if err.Message != "wrapper message" {
		t.Errorf("Message = %v, want wrapper message", err.Message)
	}
	if err.Err != underlyingErr {
		t.Errorf("Err = %v, want %v", err.Err, underlyingErr)
	}
}

// TestWrapf tests the Wrapf() function.
func TestWrapf(t *testing.T) {
	underlyingErr := errors.New("underlying")
	err := Wrapf(underlyingErr, KindConfig, "op", "wrapper: %s %d", "test", 456)

	if err.Message != "wrapper: test 456" {
		t.Errorf("Message = %v, want 'wrapper: test 456'", err.Message)
	}
}

// TestE tests the E() convenience function.
func TestE(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want *Error
	}{
		{
			name: "kind only",
			args: []any{KindConfig},
			want: &Error{Kind: KindConfig},
		},
		{
			name: "kind and op",
			args: []any{KindConfig, "operation"},
			want: &Error{Kind: KindConfig, Op: "operation"},
		},
		{
			name: "kind, op, and message",
			args: []any{KindConfig, "operation", "message"},
			want: &Error{Kind: KindConfig, Op: "operation", Message: "message"},
		},
		{
			name: "with error",
			args: []any{KindConfig, errors.New("wrapped")},
			want: &Error{Kind: KindConfig, Err: errors.New("wrapped")},
		},
		{
			name: "with custom error",
			args: []any{KindConfig, Config("inner", "msg")},
			want: &Error{Kind: KindConfig, Err: Config("inner", "msg")},
		},
		{
			name: "with details",
			args: []any{KindConfig, map[string]any{"key": "value"}},
			want: &Error{Kind: KindConfig, Details: map[string]any{"key": "value"}},
		},
		{
			name: "with recoverable",
			args: []any{KindConfig, true},
			want: &Error{Kind: KindConfig, Recoverable: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := E(tt.args...)
			if got.Kind != tt.want.Kind {
				t.Errorf("E() Kind = %v, want %v", got.Kind, tt.want.Kind)
			}
			if got.Op != tt.want.Op {
				t.Errorf("E() Op = %v, want %v", got.Op, tt.want.Op)
			}
			if got.Message != tt.want.Message {
				t.Errorf("E() Message = %v, want %v", got.Message, tt.want.Message)
			}
			if got.Recoverable != tt.want.Recoverable {
				t.Errorf("E() Recoverable = %v, want %v", got.Recoverable, tt.want.Recoverable)
			}
		})
	}
}

// TestIsKind tests the IsKind() function.
func TestIsKind(t *testing.T) {
	configErr := Config("op", "msg")
	gitErr := Git("op", "msg")
	stdErr := errors.New("standard error")

	if !IsKind(configErr, KindConfig) {
		t.Error("IsKind(configErr, KindConfig) = false, want true")
	}
	if IsKind(configErr, KindGit) {
		t.Error("IsKind(configErr, KindGit) = true, want false")
	}
	if IsKind(gitErr, KindConfig) {
		t.Error("IsKind(gitErr, KindConfig) = true, want false")
	}
	if IsKind(stdErr, KindConfig) {
		t.Error("IsKind(stdErr, KindConfig) = true, want false")
	}
	if IsKind(nil, KindConfig) {
		t.Error("IsKind(nil, KindConfig) = true, want false")
	}
}

// TestWrapFunctions tests all the *Wrap functions.
func TestWrapFunctions(t *testing.T) {
	underlyingErr := errors.New("underlying")

	tests := []struct {
		name string
		fn   func() *Error
		kind Kind
	}{
		{"GitWrap", func() *Error { return GitWrap(underlyingErr, "op", "msg") }, KindGit},
		{"VersionWrap", func() *Error { return VersionWrap(underlyingErr, "op", "msg") }, KindVersion},
		{"PluginWrap", func() *Error { return PluginWrap(underlyingErr, "op", "msg") }, KindPlugin},
		{"AIWrap", func() *Error { return AIWrap(underlyingErr, "op", "msg") }, KindAI},
		{"ValidationWrap", func() *Error { return ValidationWrap(underlyingErr, "op", "msg") }, KindValidation},
		{"NotFoundWrap", func() *Error { return NotFoundWrap(underlyingErr, "op", "msg") }, KindNotFound},
		{"IOWrap", func() *Error { return IOWrap(underlyingErr, "op", "msg") }, KindIO},
		{"NetworkWrap", func() *Error { return NetworkWrap(underlyingErr, "op", "msg") }, KindNetwork},
		{"TimeoutWrap", func() *Error { return TimeoutWrap(underlyingErr, "op", "msg") }, KindTimeout},
		{"InternalWrap", func() *Error { return InternalWrap(underlyingErr, "op", "msg") }, KindInternal},
		{"StateWrap", func() *Error { return StateWrap(underlyingErr, "op", "msg") }, KindState},
		{"TemplateWrap", func() *Error { return TemplateWrap(underlyingErr, "op", "msg") }, KindTemplate},
		{"ConflictWrap", func() *Error { return ConflictWrap(underlyingErr, "op", "msg") }, KindConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", err.Kind, tt.kind)
			}
			if err.Op != "op" {
				t.Errorf("Op = %v, want op", err.Op)
			}
			if err.Message != "msg" {
				t.Errorf("Message = %v, want msg", err.Message)
			}
			if err.Err != underlyingErr {
				t.Errorf("Err = %v, want %v", err.Err, underlyingErr)
			}
		})
	}

	// Test recoverable wrap functions
	recoverableTests := []struct {
		name string
		fn   func() *Error
	}{
		{"ValidationWrap", func() *Error { return ValidationWrap(underlyingErr, "op", "msg") }},
		{"NetworkWrap", func() *Error { return NetworkWrap(underlyingErr, "op", "msg") }},
		{"TimeoutWrap", func() *Error { return TimeoutWrap(underlyingErr, "op", "msg") }},
	}

	for _, tt := range recoverableTests {
		t.Run(tt.name+"_recoverable", func(t *testing.T) {
			err := tt.fn()
			if !err.Recoverable {
				t.Errorf("Recoverable = false, want true")
			}
		})
	}
}

// TestAIWrapSafeWithNilError tests AIWrapSafe with nil error.
func TestAIWrapSafeWithNilError(t *testing.T) {
	err := AIWrapSafe(nil, "op", "msg")
	if err == nil {
		t.Fatal("AIWrapSafe(nil) returned nil")
	}
	if err.Kind != KindAI {
		t.Errorf("Kind = %v, want %v", err.Kind, KindAI)
	}
	if err.Err != nil {
		t.Errorf("Err = %v, want nil", err.Err)
	}
}

// TestWrapSafe tests the WrapSafe function.
func TestWrapSafe(t *testing.T) {
	// Test with nil error
	err := WrapSafe(nil, KindConfig, "op", "msg")
	if err.Err != nil {
		t.Errorf("WrapSafe(nil).Err = %v, want nil", err.Err)
	}

	// Test with sensitive error
	sensitiveErr := errors.New("API key: sk-abcdefghijklmnopqrstuvwxyz123456")
	err = WrapSafe(sensitiveErr, KindConfig, "op", "msg")
	if err.Kind != KindConfig {
		t.Errorf("Kind = %v, want %v", err.Kind, KindConfig)
	}
	errStr := err.Error()
	if containsString(errStr, "sk-") {
		t.Errorf("WrapSafe error contains sensitive data: %v", errStr)
	}
	if !containsString(errStr, "[REDACTED]") {
		t.Errorf("WrapSafe error should contain [REDACTED]: %v", errStr)
	}
}

// TestFormatUserError tests the FormatUserError function.
func TestFormatUserError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "simple error",
			err:      errors.New("working tree has uncommitted changes"),
			expected: "working tree has uncommitted changes",
		},
		{
			name:     "single wrap with failed",
			err:      fmt.Errorf("plan failed: %w", errors.New("working tree has uncommitted changes")),
			expected: "Plan failed: working tree has uncommitted changes",
		},
		{
			name: "double wrap with redundant failed",
			err: fmt.Errorf("plan failed: %w",
				fmt.Errorf("failed to plan release: %w",
					errors.New("working tree has uncommitted changes"))),
			expected: "Plan failed: working tree has uncommitted changes",
		},
		{
			name: "triple wrap with multiple failed messages",
			err: fmt.Errorf("release failed: %w",
				fmt.Errorf("plan failed: %w",
					fmt.Errorf("failed to analyze commits: %w",
						errors.New("repository not found")))),
			expected: "Release failed: repository not found",
		},
		{
			name:     "structured error with op",
			err:      Wrap(errors.New("file not found"), KindIO, "read-config", "failed to read config"),
			expected: "Read-config failed: file not found",
		},
		{
			name: "mixed structured and fmt.Errorf",
			err: fmt.Errorf("plan failed: %w",
				Wrap(errors.New("no commits found"), KindGit, "analyze", "failed to analyze")),
			expected: "Plan failed: no commits found",
		},
		{
			name:     "error message without 'failed' prefix",
			err:      fmt.Errorf("bump version: %w", errors.New("invalid version format")),
			expected: "Bump version failed: invalid version format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUserError(tt.err)
			if result != tt.expected {
				t.Errorf("FormatUserError() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestCleanOperation tests the cleanOperation helper function.
func TestCleanOperation(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plan", "plan"},
		{"plan failed", "plan"},
		{"failed to plan", "plan"},
		{"failed: plan", "plan"},
		{"error: plan", "plan"},
		{"error plan", "plan"},
		{"  plan  ", "plan"},
		{"failed to plan release", "plan release"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanOperation(tt.input)
			if result != tt.expected {
				t.Errorf("cleanOperation(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCapitalizeFirst tests the capitalizeFirst helper function.
func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plan", "Plan"},
		{"Plan", "Plan"},
		{"PLAN", "PLAN"},
		{"", ""},
		{"a", "A"},
		{"already Good", "Already Good"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalizeFirst(tt.input)
			if result != tt.expected {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsRedundantMessage tests the isRedundantMessage helper function.
func TestIsRedundantMessage(t *testing.T) {
	tests := []struct {
		msg         string
		existingOps []string
		expected    bool
	}{
		{"plan", []string{}, false},
		{"plan", []string{"plan"}, true},
		{"plan failed", []string{"plan"}, true},
		{"failed to plan", []string{"plan"}, true},
		{"bump", []string{"plan"}, false},
		{"", []string{"plan"}, true},
		{"plan", []string{"bump", "publish"}, false},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%q_in_%v", tt.msg, tt.existingOps)
		t.Run(name, func(t *testing.T) {
			result := isRedundantMessage(tt.msg, tt.existingOps)
			if result != tt.expected {
				t.Errorf("isRedundantMessage(%q, %v) = %v, want %v", tt.msg, tt.existingOps, result, tt.expected)
			}
		})
	}
}

// TestFindBestOperation tests the findBestOperation helper function.
func TestFindBestOperation(t *testing.T) {
	tests := []struct {
		name     string
		ops      []string
		expected string
	}{
		{"empty list", []string{}, ""},
		{"single short op", []string{"plan"}, "plan"},
		{"single long op", []string{"failed to plan release"}, "plan release"},
		{"prefer short over long", []string{"failed to plan release", "plan"}, "plan"},
		{"multiple short ops uses first", []string{"bump", "plan"}, "bump"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findBestOperation(tt.ops)
			if result != tt.expected {
				t.Errorf("findBestOperation(%v) = %q, want %q", tt.ops, result, tt.expected)
			}
		})
	}
}
