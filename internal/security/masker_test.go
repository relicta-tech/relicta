package security

import (
	"bytes"
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
