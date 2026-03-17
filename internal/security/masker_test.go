package security

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestMaskerGlobal(t *testing.T) {
	// Reset state after test
	defer Disable()

	t.Run("disabled by default", func(t *testing.T) {
		Disable() // Ensure clean state
		if IsEnabled() {
			t.Error("Masker should be disabled by default")
		}
	})

	t.Run("enable and disable", func(t *testing.T) {
		Enable()
		if !IsEnabled() {
			t.Error("Masker should be enabled after Enable()")
		}
		Disable()
		if IsEnabled() {
			t.Error("Masker should be disabled after Disable()")
		}
	})
}

func TestMask(t *testing.T) {
	defer Disable()

	tests := []struct {
		name           string
		input          string
		maskEnabled    bool
		wantRedacted   bool
		expectedSubstr string
	}{
		{
			name:           "OpenAI key masked when enabled",
			input:          "API key is sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:         "OpenAI key not masked when disabled",
			input:        "API key is sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			maskEnabled:  false,
			wantRedacted: false,
		},
		{
			name:           "GitHub token masked when enabled",
			input:          "Token: ghp_0123456789abcdefghijklmnopqrstuvwxyz",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:           "Anthropic key masked when enabled",
			input:          "API key is sk-ant-abcdefghijklmnopqrstuvwxyz",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:           "GitLab token masked when enabled",
			input:          "Token: glpat-abcdefghij1234567890",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:           "Slack token masked when enabled",
			input:          "Token: xoxb-123456789012-123456789012-abcdefghij",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:           "Jira token masked when enabled",
			input:          "Token: ATATTabcdefghijklmnopqrstuvwxyz123456",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:           "Discord webhook masked when enabled",
			input:          "Webhook: https://discord.com/api/webhooks/1234567890/abcdefgh_ijklmnop",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:           "AWS access key masked when enabled",
			input:          "Key: AKIAIOSFODNN7EXAMPLE",
			maskEnabled:    true,
			wantRedacted:   true,
			expectedSubstr: "[REDACTED]",
		},
		{
			name:         "safe string not changed",
			input:        "This is a safe string with no secrets",
			maskEnabled:  true,
			wantRedacted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.maskEnabled {
				Enable()
			} else {
				Disable()
			}

			result := Mask(tt.input)

			if tt.wantRedacted {
				if !strings.Contains(result, tt.expectedSubstr) {
					t.Errorf("Mask() = %q, want to contain %q", result, tt.expectedSubstr)
				}
				if result == tt.input {
					t.Errorf("Mask() should have redacted the input, but got unchanged")
				}
			} else {
				if result != tt.input {
					t.Errorf("Mask() = %q, want %q (unchanged)", result, tt.input)
				}
			}
		})
	}
}

func TestMaskBytes(t *testing.T) {
	defer Disable()

	t.Run("masks bytes when enabled", func(t *testing.T) {
		Enable()
		input := []byte("Token: ghp_0123456789abcdefghijklmnopqrstuvwxyz")
		result := MaskBytes(input)
		if bytes.Equal(result, input) {
			t.Error("MaskBytes should have redacted the secret")
		}
		if !bytes.Contains(result, []byte("[REDACTED]")) {
			t.Error("MaskBytes should contain [REDACTED]")
		}
	})

	t.Run("returns unchanged when disabled", func(t *testing.T) {
		Disable()
		input := []byte("Token: ghp_0123456789abcdefghijklmnopqrstuvwxyz")
		result := MaskBytes(input)
		if !bytes.Equal(result, input) {
			t.Error("MaskBytes should return unchanged bytes when disabled")
		}
	})
}

func TestMaskedWriter(t *testing.T) {
	defer Disable()

	t.Run("masks output when enabled", func(t *testing.T) {
		Enable()
		var buf bytes.Buffer
		mw := NewMaskedWriter(&buf)

		input := []byte("Secret: ghp_0123456789abcdefghijklmnopqrstuvwxyz\n")
		n, err := mw.Write(input)

		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if n != len(input) {
			t.Errorf("Write() returned %d, want %d", n, len(input))
		}

		output := buf.String()
		if strings.Contains(output, "ghp_") {
			t.Error("MaskedWriter should have redacted the token")
		}
		if !strings.Contains(output, "[REDACTED]") {
			t.Error("MaskedWriter output should contain [REDACTED]")
		}
	})

	t.Run("passes through when disabled", func(t *testing.T) {
		Disable()
		var buf bytes.Buffer
		mw := NewMaskedWriter(&buf)

		input := []byte("Secret: ghp_0123456789abcdefghijklmnopqrstuvwxyz\n")
		_, err := mw.Write(input)

		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "ghp_") {
			t.Error("MaskedWriter should pass through unchanged when disabled")
		}
	})
}

func TestMaskMap(t *testing.T) {
	defer Disable()

	t.Run("masks nested map values when enabled", func(t *testing.T) {
		Enable()
		input := map[string]interface{}{
			"name": "test",
			"config": map[string]interface{}{
				"api_key": "sk-proj-abcdefghijklmnopqrstuvwxyz123456",
				"url":     "https://example.com",
			},
			"tokens": []interface{}{
				"ghp_0123456789abcdefghijklmnopqrstuvwxyz",
				"safe-value",
			},
		}

		result := MaskMap(input)

		// Check nested map
		config, ok := result["config"].(map[string]interface{})
		if !ok {
			t.Fatal("config should be a map")
		}
		apiKey := config["api_key"].(string)
		if strings.Contains(apiKey, "sk-proj-") {
			t.Error("api_key should be redacted")
		}
		if apiKey != "[REDACTED]" {
			t.Errorf("api_key = %q, want [REDACTED]", apiKey)
		}

		// Check URL is unchanged
		url := config["url"].(string)
		if url != "https://example.com" {
			t.Error("url should be unchanged")
		}

		// Check array values
		tokens, ok := result["tokens"].([]interface{})
		if !ok {
			t.Fatal("tokens should be a slice")
		}
		if strings.Contains(tokens[0].(string), "ghp_") {
			t.Error("first token should be redacted")
		}
		if tokens[1].(string) != "safe-value" {
			t.Error("safe value should be unchanged")
		}
	})

	t.Run("returns unchanged when disabled", func(t *testing.T) {
		Disable()
		input := map[string]interface{}{
			"api_key": "sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		}

		result := MaskMap(input)

		apiKey := result["api_key"].(string)
		if !strings.Contains(apiKey, "sk-proj-") {
			t.Error("api_key should be unchanged when masking is disabled")
		}
	})
}

func TestMaskerInstance(t *testing.T) {
	t.Run("independent instance", func(t *testing.T) {
		m := NewMasker()

		if m.IsEnabled() {
			t.Error("New Masker should be disabled by default")
		}

		m.Enable()
		if !m.IsEnabled() {
			t.Error("Masker should be enabled after Enable()")
		}

		// Global should not be affected
		Disable()
		if !m.IsEnabled() {
			t.Error("Instance Masker should be independent of global")
		}

		m.Disable()
		if m.IsEnabled() {
			t.Error("Masker should be disabled after Disable()")
		}
	})

	t.Run("instance masking", func(t *testing.T) {
		m := NewMasker()
		input := "Token: ghp_0123456789abcdefghijklmnopqrstuvwxyz"

		// Disabled - should not mask
		result := m.Mask(input)
		if result != input {
			t.Error("Should not mask when disabled")
		}

		// Enabled - should mask
		m.Enable()
		result = m.Mask(input)
		if !strings.Contains(result, "[REDACTED]") {
			t.Error("Should mask when enabled")
		}
	})
}

func TestMaskConfigMap(t *testing.T) {
	t.Run("masks sensitive keys by name", func(t *testing.T) {
		input := map[string]any{
			"url":      "https://example.com",
			"token":    "my-secret-token",
			"password": "super-secret-password",
			"api_key":  "sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			"username": "admin",
		}

		result := MaskConfigMap(input)

		// Sensitive keys should be redacted
		if result["token"] != "[REDACTED]" {
			t.Errorf("token should be redacted, got %v", result["token"])
		}
		if result["password"] != "[REDACTED]" {
			t.Errorf("password should be redacted, got %v", result["password"])
		}
		if result["api_key"] != "[REDACTED]" {
			t.Errorf("api_key should be redacted, got %v", result["api_key"])
		}

		// Non-sensitive keys should be unchanged
		if result["url"] != "https://example.com" {
			t.Errorf("url should be unchanged, got %v", result["url"])
		}
		if result["username"] != "admin" {
			t.Errorf("username should be unchanged, got %v", result["username"])
		}
	})

	t.Run("masks nested configs", func(t *testing.T) {
		input := map[string]any{
			"plugin": map[string]any{
				"name":   "github",
				"token":  "ghp_0123456789abcdefghijklmnopqrstuvwxyz",
				"secret": "my-webhook-secret",
			},
		}

		result := MaskConfigMap(input)

		nested := result["plugin"].(map[string]any)
		if nested["name"] != "github" {
			t.Errorf("name should be unchanged, got %v", nested["name"])
		}
		if nested["token"] != "[REDACTED]" {
			t.Errorf("nested token should be redacted, got %v", nested["token"])
		}
		if nested["secret"] != "[REDACTED]" {
			t.Errorf("nested secret should be redacted, got %v", nested["secret"])
		}
	})

	t.Run("also redacts values that look like secrets", func(t *testing.T) {
		input := map[string]any{
			"callback_url": "https://example.com",
			"data":         "ghp_0123456789abcdefghijklmnopqrstuvwxyz", // looks like a token
		}

		result := MaskConfigMap(input)

		// URL should be unchanged
		if result["callback_url"] != "https://example.com" {
			t.Errorf("callback_url should be unchanged, got %v", result["callback_url"])
		}

		// Value that looks like a secret should be redacted
		if result["data"] == "ghp_0123456789abcdefghijklmnopqrstuvwxyz" {
			t.Errorf("data that looks like a token should be redacted, got %v", result["data"])
		}
	})

	t.Run("handles empty and nil maps", func(t *testing.T) {
		result := MaskConfigMap(nil)
		if result != nil {
			t.Error("nil input should return nil")
		}

		result = MaskConfigMap(map[string]any{})
		if len(result) != 0 {
			t.Error("empty input should return empty map")
		}
	})

	t.Run("case insensitive key matching", func(t *testing.T) {
		input := map[string]any{
			"Token":    "my-token",
			"PASSWORD": "my-password",
			"Api_Key":  "my-api-key",
		}

		result := MaskConfigMap(input)

		if result["Token"] != "[REDACTED]" {
			t.Errorf("Token should be redacted, got %v", result["Token"])
		}
		if result["PASSWORD"] != "[REDACTED]" {
			t.Errorf("PASSWORD should be redacted, got %v", result["PASSWORD"])
		}
		if result["Api_Key"] != "[REDACTED]" {
			t.Errorf("Api_Key should be redacted, got %v", result["Api_Key"])
		}
	})

	t.Run("preserves empty sensitive values", func(t *testing.T) {
		input := map[string]any{
			"token":  "",
			"secret": "",
		}

		result := MaskConfigMap(input)

		// Empty values should remain empty (not replaced with [REDACTED])
		if result["token"] != "" {
			t.Errorf("empty token should remain empty, got %v", result["token"])
		}
		if result["secret"] != "" {
			t.Errorf("empty secret should remain empty, got %v", result["secret"])
		}
	})
}

func TestEnableInCI(t *testing.T) {
	// Save original state
	wasEnabled := IsEnabled()
	defer func() {
		if wasEnabled {
			Enable()
		} else {
			Disable()
		}
	}()

	t.Run("enables when CI env var is set", func(t *testing.T) {
		Disable()
		t.Setenv("CI", "true")
		EnableInCI()
		if !IsEnabled() {
			t.Error("Masker should be enabled when CI=true")
		}
	})

	t.Run("enables when GITHUB_ACTIONS env var is set", func(t *testing.T) {
		Disable()
		t.Setenv("GITHUB_ACTIONS", "true")
		EnableInCI()
		if !IsEnabled() {
			t.Error("Masker should be enabled when GITHUB_ACTIONS=true")
		}
	})

	t.Run("enables when GITLAB_CI env var is set", func(t *testing.T) {
		Disable()
		t.Setenv("GITLAB_CI", "true")
		EnableInCI()
		if !IsEnabled() {
			t.Error("Masker should be enabled when GITLAB_CI=true")
		}
	})

	t.Run("stays disabled when no CI env vars", func(t *testing.T) {
		Disable()
		// Unset all CI env vars that EnableInCI checks
		for _, env := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "JENKINS_URL", "TRAVIS", "BITBUCKET_PIPELINES", "AZURE_PIPELINES", "TEAMCITY_VERSION", "BUILDKITE"} {
			t.Setenv(env, "")
		}
		EnableInCI()
		if IsEnabled() {
			t.Error("Masker should remain disabled when no CI env vars are set")
		}
	})
}

func TestMaskConfigSlice(t *testing.T) {
	t.Run("masks strings containing secrets", func(t *testing.T) {
		input := []any{
			"normal string",
			"token: ghp_0123456789abcdefghijklmnopqrstuvwxyz",
			42,
		}

		result := maskConfigSlice(input)

		if result[0] != "normal string" {
			t.Errorf("normal string should be unchanged, got %v", result[0])
		}
		if !strings.Contains(result[1].(string), "[REDACTED]") {
			t.Errorf("token should be redacted, got %v", result[1])
		}
		if result[2] != 42 {
			t.Errorf("integer should be unchanged, got %v", result[2])
		}
	})

	t.Run("handles nested maps in slice", func(t *testing.T) {
		input := []any{
			map[string]any{
				"token": "secret-token",
				"name":  "test",
			},
		}

		result := maskConfigSlice(input)

		nestedMap := result[0].(map[string]any)
		if nestedMap["token"] != "[REDACTED]" {
			t.Errorf("nested token should be redacted, got %v", nestedMap["token"])
		}
		if nestedMap["name"] != "test" {
			t.Errorf("nested name should be unchanged, got %v", nestedMap["name"])
		}
	})

	t.Run("handles nested slices", func(t *testing.T) {
		input := []any{
			[]any{"normal", "sk-proj-abcdefghijklmnopqrstuvwxyz123456"},
		}

		result := maskConfigSlice(input)

		nestedSlice := result[0].([]any)
		if nestedSlice[0] != "normal" {
			t.Errorf("nested normal should be unchanged, got %v", nestedSlice[0])
		}
		if !strings.Contains(nestedSlice[1].(string), "[REDACTED]") {
			t.Errorf("nested secret should be redacted, got %v", nestedSlice[1])
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		result := maskConfigSlice([]any{})
		if len(result) != 0 {
			t.Error("empty slice should return empty slice")
		}
	})
}

func TestMaskSlice(t *testing.T) {
	defer Disable()

	t.Run("masks strings when enabled", func(t *testing.T) {
		Enable()
		input := []interface{}{
			"normal string",
			"ghp_0123456789abcdefghijklmnopqrstuvwxyz",
			123,
		}

		result := maskSlice(input)

		if result[0] != "normal string" {
			t.Errorf("normal string should be unchanged, got %v", result[0])
		}
		if !strings.Contains(result[1].(string), "[REDACTED]") {
			t.Errorf("secret should be redacted, got %v", result[1])
		}
		if result[2] != 123 {
			t.Errorf("integer should be unchanged, got %v", result[2])
		}
	})

	t.Run("handles nested maps", func(t *testing.T) {
		Enable()
		input := []interface{}{
			map[string]interface{}{
				"key": "sk-proj-abcdefghijklmnopqrstuvwxyz123456",
			},
		}

		result := maskSlice(input)

		nestedMap := result[0].(map[string]interface{})
		if !strings.Contains(nestedMap["key"].(string), "[REDACTED]") {
			t.Errorf("nested secret should be redacted, got %v", nestedMap["key"])
		}
	})

	t.Run("handles deeply nested slices", func(t *testing.T) {
		Enable()
		input := []interface{}{
			[]interface{}{
				[]interface{}{
					"ghp_0123456789abcdefghijklmnopqrstuvwxyz",
				},
			},
		}

		result := maskSlice(input)

		nested1 := result[0].([]interface{})
		nested2 := nested1[0].([]interface{})
		if !strings.Contains(nested2[0].(string), "[REDACTED]") {
			t.Errorf("deeply nested secret should be redacted, got %v", nested2[0])
		}
	})
}

func TestMaskedWriter_WriteError(t *testing.T) {
	defer Disable()
	Enable()

	errWriter := &errorWriter{}
	mw := NewMaskedWriter(errWriter)

	_, err := mw.Write([]byte("some data"))
	if err == nil {
		t.Error("Write should propagate underlying writer error")
	}
}

type errorWriter struct{}

func (ew *errorWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("write error")
}

func TestMaskConfigMap_NonStringSensitiveValue(t *testing.T) {
	input := map[string]any{
		"token":  12345, // non-string sensitive value
		"secret": nil,   // nil sensitive value
	}

	result := MaskConfigMap(input)

	if result["token"] != "[REDACTED]" {
		t.Errorf("non-string token should be redacted, got %v", result["token"])
	}
	if result["secret"] != nil {
		t.Errorf("nil secret should remain nil, got %v", result["secret"])
	}
}

func TestMaskConfigMap_SliceWithNonStringValues(t *testing.T) {
	input := map[string]any{
		"items": []any{42, true, nil},
	}

	result := MaskConfigMap(input)

	items := result["items"].([]any)
	if items[0] != 42 {
		t.Errorf("integer should be unchanged, got %v", items[0])
	}
	if items[1] != true {
		t.Errorf("bool should be unchanged, got %v", items[1])
	}
	if items[2] != nil {
		t.Errorf("nil should be unchanged, got %v", items[2])
	}
}

func TestMaskConfigMap_NonSensitiveNonStringValues(t *testing.T) {
	input := map[string]any{
		"count":   42,
		"enabled": true,
	}

	result := MaskConfigMap(input)

	if result["count"] != 42 {
		t.Errorf("count should be unchanged, got %v", result["count"])
	}
	if result["enabled"] != true {
		t.Errorf("enabled should be unchanged, got %v", result["enabled"])
	}
}

func TestMaskMap_DefaultBranch(t *testing.T) {
	defer Disable()
	Enable()
	input := map[string]interface{}{
		"count":   42,
		"enabled": true,
		"ratio":   3.14,
	}
	result := MaskMap(input)
	if result["count"] != 42 {
		t.Errorf("integer should pass through, got %v", result["count"])
	}
	if result["enabled"] != true {
		t.Errorf("bool should pass through, got %v", result["enabled"])
	}
	if result["ratio"] != 3.14 {
		t.Errorf("float should pass through, got %v", result["ratio"])
	}
}

func TestMaskError(t *testing.T) {
	t.Run("masks secrets in error message", func(t *testing.T) {
		err := fmt.Errorf("failed to connect with token ghp_0123456789abcdefghijklmnopqrstuvwxyz")
		result := MaskError(err)

		if strings.Contains(result.Error(), "ghp_") {
			t.Error("error should have token redacted")
		}
		if !strings.Contains(result.Error(), "[REDACTED]") {
			t.Error("error should contain [REDACTED]")
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := MaskError(nil)
		if result != nil {
			t.Error("nil error should return nil")
		}
	})

	t.Run("preserves non-sensitive error message", func(t *testing.T) {
		err := fmt.Errorf("connection timeout")
		result := MaskError(err)

		if result.Error() != "connection timeout" {
			t.Errorf("non-sensitive error should be unchanged, got %v", result.Error())
		}
	})
}
