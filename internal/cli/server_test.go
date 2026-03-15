package cli

import (
	"os"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
)

func TestResolveServerMode(t *testing.T) {
	tests := []struct {
		name     string
		flagVal  string
		envVal   string
		expected config.ServerMode
	}{
		{
			name:     "default when nothing set",
			expected: config.ServerModeEmbedded,
		},
		{
			name:     "flag takes precedence over env",
			flagVal:  "api",
			envVal:   "embedded",
			expected: config.ServerModeAPI,
		},
		{
			name:     "env var when no flag",
			envVal:   "api",
			expected: config.ServerModeAPI,
		},
		{
			name:     "flag embedded",
			flagVal:  "embedded",
			expected: config.ServerModeEmbedded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore
			origFlag := serverMode
			origEnv := os.Getenv("RELICTA_SERVER_MODE")
			defer func() {
				serverMode = origFlag
				if origEnv != "" {
					os.Setenv("RELICTA_SERVER_MODE", origEnv)
				} else {
					os.Unsetenv("RELICTA_SERVER_MODE")
				}
			}()

			serverMode = tt.flagVal
			if tt.envVal != "" {
				os.Setenv("RELICTA_SERVER_MODE", tt.envVal)
			} else {
				os.Unsetenv("RELICTA_SERVER_MODE")
			}

			got := resolveServerMode()
			if got != tt.expected {
				t.Errorf("resolveServerMode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResolveAllowedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		flagVal  string
		envVal   string
		expected []string
	}{
		{
			name:     "nil when nothing set",
			expected: nil,
		},
		{
			name:     "single origin from flag",
			flagVal:  "http://localhost:5173",
			expected: []string{"http://localhost:5173"},
		},
		{
			name:     "multiple origins from flag",
			flagVal:  "http://localhost:5173, https://example.com",
			expected: []string{"http://localhost:5173", "https://example.com"},
		},
		{
			name:     "from env var",
			envVal:   "http://localhost:3000",
			expected: []string{"http://localhost:3000"},
		},
		{
			name:     "flag takes precedence over env",
			flagVal:  "http://a.com",
			envVal:   "http://b.com",
			expected: []string{"http://a.com"},
		},
		{
			name:     "trims whitespace",
			flagVal:  " http://a.com , http://b.com , ",
			expected: []string{"http://a.com", "http://b.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origFlag := serverAllowedOrigins
			origEnv := os.Getenv("RELICTA_ALLOWED_ORIGINS")
			defer func() {
				serverAllowedOrigins = origFlag
				if origEnv != "" {
					os.Setenv("RELICTA_ALLOWED_ORIGINS", origEnv)
				} else {
					os.Unsetenv("RELICTA_ALLOWED_ORIGINS")
				}
			}()

			serverAllowedOrigins = tt.flagVal
			if tt.envVal != "" {
				os.Setenv("RELICTA_ALLOWED_ORIGINS", tt.envVal)
			} else {
				os.Unsetenv("RELICTA_ALLOWED_ORIGINS")
			}

			got := resolveAllowedOrigins()
			if tt.expected == nil {
				if got != nil {
					t.Errorf("resolveAllowedOrigins() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("resolveAllowedOrigins() = %v, want %v", got, tt.expected)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("resolveAllowedOrigins()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}
