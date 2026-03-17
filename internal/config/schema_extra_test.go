package config

import (
	"testing"
)

// TestChannelDefinitionConfig_NeedsApproval covers all branches.
func TestChannelDefinitionConfig_NeedsApproval(t *testing.T) {
	tests := []struct {
		name            string
		requireApproval *bool
		expected        bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ChannelDefinitionConfig{RequireApproval: tt.requireApproval}
			got := c.NeedsApproval()
			if got != tt.expected {
				t.Errorf("NeedsApproval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestChannelDefinitionConfig_IsAutoApproved covers all branches.
func TestChannelDefinitionConfig_IsAutoApproved(t *testing.T) {
	tests := []struct {
		name        string
		autoApprove *bool
		expected    bool
	}{
		{"nil defaults to false", nil, false},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ChannelDefinitionConfig{AutoApprove: tt.autoApprove}
			got := c.IsAutoApproved()
			if got != tt.expected {
				t.Errorf("IsAutoApproved() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDefaultPersistenceConfig verifies default values.
func TestDefaultPersistenceConfig(t *testing.T) {
	cfg := DefaultPersistenceConfig()

	if cfg.Backend != BackendFile {
		t.Errorf("Backend = %v, want %v", cfg.Backend, BackendFile)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10", cfg.PoolSize)
	}
	if cfg.MigrationMode != MigrationModeManual {
		t.Errorf("MigrationMode = %v, want %v", cfg.MigrationMode, MigrationModeManual)
	}
	if cfg.FilePath == "" {
		t.Error("FilePath should have a default value")
	}
}

// TestPersistenceConfig_Validate covers all branches of Validate.
func TestPersistenceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PersistenceConfig
		wantErr bool
	}{
		{
			name:    "file backend is valid",
			cfg:     PersistenceConfig{Backend: BackendFile},
			wantErr: false,
		},
		{
			name: "postgres valid with all fields",
			cfg: PersistenceConfig{
				Backend:          BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         5,
				MigrationMode:    MigrationModeManual,
			},
			wantErr: false,
		},
		{
			name: "postgres valid with auto migration",
			cfg: PersistenceConfig{
				Backend:          BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         5,
				MigrationMode:    MigrationModeAuto,
			},
			wantErr: false,
		},
		{
			name: "postgres missing connection string",
			cfg: PersistenceConfig{
				Backend:  BackendPostgres,
				PoolSize: 5,
			},
			wantErr: true,
		},
		{
			name: "postgres pool size zero",
			cfg: PersistenceConfig{
				Backend:          BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         0,
			},
			wantErr: true,
		},
		{
			name: "postgres invalid migration mode",
			cfg: PersistenceConfig{
				Backend:          BackendPostgres,
				ConnectionString: "postgres://localhost/db",
				PoolSize:         5,
				MigrationMode:    "invalid",
			},
			wantErr: true,
		},
		{
			name:    "unsupported backend",
			cfg:     PersistenceConfig{Backend: "redis"},
			wantErr: true,
		},
		{
			name:    "empty backend is unsupported",
			cfg:     PersistenceConfig{Backend: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestExpandEnvVars covers the environment variable expansion logic.
func TestExpandEnvVars(t *testing.T) {
	t.Setenv("TEST_DB_HOST", "localhost")
	t.Setenv("TEST_DB_PORT", "5432")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single var expansion",
			input:    "postgres://${TEST_DB_HOST}/db",
			expected: "postgres://localhost/db",
		},
		{
			name:     "multiple var expansions",
			input:    "postgres://${TEST_DB_HOST}:${TEST_DB_PORT}/db",
			expected: "postgres://localhost:5432/db",
		},
		{
			name:     "unset var remains unexpanded",
			input:    "value=${UNSET_VAR_12345}",
			expected: "value=${UNSET_VAR_12345}",
		},
		{
			name:     "no vars",
			input:    "no-variables-here",
			expected: "no-variables-here",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unclosed brace",
			input:    "prefix-${UNCLOSED",
			expected: "prefix-${UNCLOSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandEnvVars(tt.input)
			if got != tt.expected {
				t.Errorf("ExpandEnvVars(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestChannelsConfig_Fields verifies the ChannelsConfig struct fields.
func TestChannelsConfig_Fields(t *testing.T) {
	cfg := ChannelsConfig{
		Enabled: true,
		Default: "stable",
		Definitions: []ChannelDefinitionConfig{
			{
				Name:       "alpha",
				Stability:  20,
				Prerelease: "alpha",
				PromotesTo: []string{"beta"},
			},
		},
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.Default != "stable" {
		t.Errorf("Default = %v, want stable", cfg.Default)
	}
	if len(cfg.Definitions) != 1 {
		t.Errorf("Definitions len = %d, want 1", len(cfg.Definitions))
	}
	if cfg.Definitions[0].Name != "alpha" {
		t.Errorf("Definition name = %v, want alpha", cfg.Definitions[0].Name)
	}
}

// TestCommunicationConfig_Fields verifies the CommunicationConfig struct.
func TestCommunicationConfig_Fields(t *testing.T) {
	cfg := CommunicationConfig{
		DefaultAudience: "engineering",
		Audiences:       nil,
	}

	if cfg.DefaultAudience != "engineering" {
		t.Errorf("DefaultAudience = %v, want engineering", cfg.DefaultAudience)
	}
}
