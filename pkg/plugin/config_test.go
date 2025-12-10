package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigParser_GetString(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]any
		field    string
		envVars  []string
		envSetup map[string]string
		want     string
	}{
		{
			name:   "returns string value from config",
			config: map[string]any{"key": "value"},
			field:  "key",
			want:   "value",
		},
		{
			name:   "returns empty for missing field",
			config: map[string]any{},
			field:  "key",
			want:   "",
		},
		{
			name:     "falls back to env var when config empty",
			config:   map[string]any{},
			field:    "key",
			envVars:  []string{"TEST_KEY"},
			envSetup: map[string]string{"TEST_KEY": "env_value"},
			want:     "env_value",
		},
		{
			name:     "prefers config over env var",
			config:   map[string]any{"key": "config_value"},
			field:    "key",
			envVars:  []string{"TEST_KEY"},
			envSetup: map[string]string{"TEST_KEY": "env_value"},
			want:     "config_value",
		},
		{
			name:     "tries multiple env vars in order",
			config:   map[string]any{},
			field:    "key",
			envVars:  []string{"FIRST_KEY", "SECOND_KEY"},
			envSetup: map[string]string{"SECOND_KEY": "second_value"},
			want:     "second_value",
		},
		{
			name:   "returns empty for non-string value",
			config: map[string]any{"key": 123},
			field:  "key",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup env vars
			for k, v := range tt.envSetup {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			p := NewConfigParser(tt.config)
			got := p.GetString(tt.field, tt.envVars...)
			if got != tt.want {
				t.Errorf("GetString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigParser_GetBool(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		field  string
		want   bool
	}{
		{
			name:   "returns true when set",
			config: map[string]any{"key": true},
			field:  "key",
			want:   true,
		},
		{
			name:   "returns false when set to false",
			config: map[string]any{"key": false},
			field:  "key",
			want:   false,
		},
		{
			name:   "returns false for missing field",
			config: map[string]any{},
			field:  "key",
			want:   false,
		},
		{
			name:   "returns false for non-bool value",
			config: map[string]any{"key": "true"},
			field:  "key",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewConfigParser(tt.config)
			got := p.GetBool(tt.field)
			if got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigParser_GetBoolDefault(t *testing.T) {
	p := NewConfigParser(map[string]any{})
	if got := p.GetBoolDefault("missing", true); got != true {
		t.Errorf("GetBoolDefault() = %v, want true", got)
	}

	p = NewConfigParser(map[string]any{"key": false})
	if got := p.GetBoolDefault("key", true); got != false {
		t.Errorf("GetBoolDefault() = %v, want false", got)
	}
}

func TestConfigParser_GetInt(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		field  string
		want   int
	}{
		{
			name:   "returns int from float64 (JSON)",
			config: map[string]any{"key": float64(42)},
			field:  "key",
			want:   42,
		},
		{
			name:   "returns int directly",
			config: map[string]any{"key": 42},
			field:  "key",
			want:   42,
		},
		{
			name:   "returns 0 for missing field",
			config: map[string]any{},
			field:  "key",
			want:   0,
		},
		{
			name:   "returns 0 for non-numeric value",
			config: map[string]any{"key": "42"},
			field:  "key",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewConfigParser(tt.config)
			got := p.GetInt(tt.field)
			if got != tt.want {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigParser_GetStringSlice(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		field  string
		want   []string
	}{
		{
			name:   "returns string slice",
			config: map[string]any{"key": []any{"a", "b", "c"}},
			field:  "key",
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "returns nil for missing field",
			config: map[string]any{},
			field:  "key",
			want:   nil,
		},
		{
			name:   "filters non-string elements",
			config: map[string]any{"key": []any{"a", 123, "b"}},
			field:  "key",
			want:   []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewConfigParser(tt.config)
			got := p.GetStringSlice(tt.field)
			if len(got) != len(tt.want) {
				t.Errorf("GetStringSlice() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetStringSlice()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestConfigParser_GetStringMap(t *testing.T) {
	config := map[string]any{
		"headers": map[string]any{
			"X-Custom": "value",
			"X-Number": 123, // Should be skipped
		},
	}
	p := NewConfigParser(config)
	got := p.GetStringMap("headers")

	if got["X-Custom"] != "value" {
		t.Errorf("GetStringMap()[X-Custom] = %q, want %q", got["X-Custom"], "value")
	}
	if _, ok := got["X-Number"]; ok {
		t.Error("GetStringMap() should not include non-string values")
	}
}

func TestValidationBuilder(t *testing.T) {
	t.Run("empty builder is valid", func(t *testing.T) {
		vb := NewValidationBuilder()
		resp := vb.Build()
		if !resp.Valid {
			t.Error("expected Valid=true for empty builder")
		}
		if len(resp.Errors) != 0 {
			t.Errorf("expected 0 errors, got %d", len(resp.Errors))
		}
	})

	t.Run("AddRequired adds required error", func(t *testing.T) {
		vb := NewValidationBuilder().AddRequired("token")
		resp := vb.Build()
		if resp.Valid {
			t.Error("expected Valid=false")
		}
		if len(resp.Errors) != 1 {
			t.Fatalf("expected 1 error, got %d", len(resp.Errors))
		}
		if resp.Errors[0].Field != "token" {
			t.Errorf("expected field=token, got %s", resp.Errors[0].Field)
		}
		if resp.Errors[0].Code != "required" {
			t.Errorf("expected code=required, got %s", resp.Errors[0].Code)
		}
	})

	t.Run("RequireString validates required string", func(t *testing.T) {
		config := map[string]any{"name": "test"}
		vb := NewValidationBuilder().
			RequireString(config, "name").
			RequireString(config, "missing")

		if !vb.HasErrors() {
			t.Error("expected HasErrors() = true")
		}
		resp := vb.Build()
		if len(resp.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(resp.Errors))
		}
	})

	t.Run("ValidateStringSlice validates array elements", func(t *testing.T) {
		config := map[string]any{
			"valid":   []any{"a", "b"},
			"invalid": []any{"a", 123, "b"},
		}
		vb := NewValidationBuilder().
			ValidateStringSlice(config, "valid").
			ValidateStringSlice(config, "invalid")

		resp := vb.Build()
		if len(resp.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(resp.Errors))
		}
		if resp.Errors[0].Field != "invalid[1]" {
			t.Errorf("expected field=invalid[1], got %s", resp.Errors[0].Field)
		}
	})

	t.Run("ValidateEnum validates allowed values", func(t *testing.T) {
		config := map[string]any{"level": "high"}
		vb := NewValidationBuilder().
			ValidateEnum(config, "level", []string{"low", "medium"})

		resp := vb.Build()
		if len(resp.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(resp.Errors))
		}
		if resp.Errors[0].Code != "enum" {
			t.Errorf("expected code=enum, got %s", resp.Errors[0].Code)
		}
	})

	t.Run("ValidateRegex validates regex patterns", func(t *testing.T) {
		config := map[string]any{
			"valid":   "^[a-z]+$",
			"invalid": "[invalid",
		}
		vb := NewValidationBuilder().
			ValidateRegex(config, "valid").
			ValidateRegex(config, "invalid")

		resp := vb.Build()
		if len(resp.Errors) != 1 {
			t.Errorf("expected 1 error, got %d", len(resp.Errors))
		}
	})
}

func TestURLValidator(t *testing.T) {
	t.Run("validates scheme", func(t *testing.T) {
		v := NewURLValidator("https")
		if err := v.Validate("http://example.com"); err == nil {
			t.Error("expected error for http scheme")
		}
		if err := v.Validate("https://example.com"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validates allowed hosts", func(t *testing.T) {
		v := NewURLValidator("https").WithHosts("hooks.slack.com")
		if err := v.Validate("https://evil.com/webhook"); err == nil {
			t.Error("expected error for disallowed host")
		}
		if err := v.Validate("https://hooks.slack.com/services/xxx"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validates path prefix", func(t *testing.T) {
		v := NewURLValidator("https").WithPathPrefix("/services/")
		if err := v.Validate("https://hooks.slack.com/other/xxx"); err == nil {
			t.Error("expected error for wrong path")
		}
		if err := v.Validate("https://hooks.slack.com/services/xxx"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects empty URL", func(t *testing.T) {
		v := NewURLValidator("https")
		if err := v.Validate(""); err == nil {
			t.Error("expected error for empty URL")
		}
	})
}

func TestValidateAssetPath(t *testing.T) {
	// Create a temp file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Save and change cwd
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	t.Run("accepts valid file path", func(t *testing.T) {
		path, err := ValidateAssetPath("test.txt")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path == "" {
			t.Error("expected non-empty path")
		}
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		_, err := ValidateAssetPath("../../../etc/passwd")
		if err == nil {
			t.Error("expected error for path traversal")
		}
	})

	t.Run("rejects non-existent file", func(t *testing.T) {
		_, err := ValidateAssetPath("nonexistent.txt")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("rejects empty path", func(t *testing.T) {
		_, err := ValidateAssetPath("")
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("rejects directory path", func(t *testing.T) {
		os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
		_, err := ValidateAssetPath("subdir")
		if err == nil {
			t.Error("expected error for directory path")
		}
	})
}

func TestBuildMentionText(t *testing.T) {
	tests := []struct {
		name     string
		mentions []string
		format   MentionFormat
		want     string
	}{
		{
			name:     "empty mentions",
			mentions: nil,
			format:   MentionFormatSlack,
			want:     "",
		},
		{
			name:     "slack format with user ID",
			mentions: []string{"U12345"},
			format:   MentionFormatSlack,
			want:     "<@U12345>",
		},
		{
			name:     "slack format preserves existing format",
			mentions: []string{"<!channel>"},
			format:   MentionFormatSlack,
			want:     "<!channel>",
		},
		{
			name:     "discord format with user ID",
			mentions: []string{"123456789"},
			format:   MentionFormatDiscord,
			want:     "<@123456789>",
		},
		{
			name:     "plain format adds @ prefix",
			mentions: []string{"user1", "@user2"},
			format:   MentionFormatPlain,
			want:     "@user1 @user2",
		},
		{
			name:     "multiple mentions",
			mentions: []string{"U1", "U2", "U3"},
			format:   MentionFormatSlack,
			want:     "<@U1> <@U2> <@U3>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMentionText(tt.mentions, tt.format)
			if got != tt.want {
				t.Errorf("BuildMentionText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewConfigParser_NilConfig(t *testing.T) {
	p := NewConfigParser(nil)
	if p.raw == nil {
		t.Error("expected non-nil raw map")
	}
	if p.GetString("key") != "" {
		t.Error("expected empty string for nil config")
	}
}

func TestConfigParser_Has(t *testing.T) {
	config := map[string]any{
		"present": "value",
		"nil_val": nil,
	}
	p := NewConfigParser(config)

	if !p.Has("present") {
		t.Error("expected Has(present) = true")
	}
	if !p.Has("nil_val") {
		t.Error("expected Has(nil_val) = true (field exists)")
	}
	if p.Has("missing") {
		t.Error("expected Has(missing) = false")
	}
}

func TestConfigParser_Raw(t *testing.T) {
	config := map[string]any{"key": "value"}
	p := NewConfigParser(config)
	raw := p.Raw()
	if raw["key"] != "value" {
		t.Error("expected Raw() to return original config")
	}
}

func TestConfigParser_GetIntDefault(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]any
		field      string
		defaultVal int
		want       int
	}{
		{
			name:       "returns int from float64 (JSON)",
			config:     map[string]any{"key": float64(42)},
			field:      "key",
			defaultVal: 10,
			want:       42,
		},
		{
			name:       "returns int directly",
			config:     map[string]any{"key": 42},
			field:      "key",
			defaultVal: 10,
			want:       42,
		},
		{
			name:       "returns int64",
			config:     map[string]any{"key": int64(42)},
			field:      "key",
			defaultVal: 10,
			want:       42,
		},
		{
			name:       "returns default for missing field",
			config:     map[string]any{},
			field:      "key",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "returns default for non-numeric value",
			config:     map[string]any{"key": "42"},
			field:      "key",
			defaultVal: 10,
			want:       10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewConfigParser(tt.config)
			got := p.GetIntDefault(tt.field, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetIntDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigParser_GetInt_int64(t *testing.T) {
	config := map[string]any{"key": int64(42)}
	p := NewConfigParser(config)
	got := p.GetInt("key")
	if got != 42 {
		t.Errorf("GetInt() = %v, want 42", got)
	}
}

func TestConfigParser_GetFloat(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		field  string
		want   float64
	}{
		{
			name:   "returns float64 value",
			config: map[string]any{"key": 3.14},
			field:  "key",
			want:   3.14,
		},
		{
			name:   "returns 0 for missing field",
			config: map[string]any{},
			field:  "key",
			want:   0,
		},
		{
			name:   "returns 0 for non-float value",
			config: map[string]any{"key": "3.14"},
			field:  "key",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewConfigParser(tt.config)
			got := p.GetFloat(tt.field)
			if got != tt.want {
				t.Errorf("GetFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationBuilder_Errors(t *testing.T) {
	vb := NewValidationBuilder()
	vb.AddError("field1", "message1", "code1")
	vb.AddError("field2", "message2", "code2")

	errors := vb.Errors()
	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
	if errors[0].Field != "field1" {
		t.Errorf("expected first error field=field1, got %s", errors[0].Field)
	}
}

func TestValidationBuilder_RequireStringWithEnv(t *testing.T) {
	t.Run("passes when config has value", func(t *testing.T) {
		config := map[string]any{"token": "secret"}
		vb := NewValidationBuilder().RequireStringWithEnv(config, "token", "TOKEN_ENV")
		if vb.HasErrors() {
			t.Error("expected no errors when config has value")
		}
	})

	t.Run("passes when env var has value", func(t *testing.T) {
		os.Setenv("TEST_TOKEN_123", "secret")
		defer os.Unsetenv("TEST_TOKEN_123")

		config := map[string]any{}
		vb := NewValidationBuilder().RequireStringWithEnv(config, "token", "TEST_TOKEN_123")
		if vb.HasErrors() {
			t.Error("expected no errors when env var has value")
		}
	})

	t.Run("fails when neither config nor env has value", func(t *testing.T) {
		config := map[string]any{}
		vb := NewValidationBuilder().RequireStringWithEnv(config, "token", "NONEXISTENT_ENV_VAR_XYZ")
		if !vb.HasErrors() {
			t.Error("expected error when neither config nor env has value")
		}
	})
}

func TestValidationBuilder_ValidateURL(t *testing.T) {
	t.Run("passes for valid URL", func(t *testing.T) {
		config := map[string]any{"url": "https://example.com/path"}
		vb := NewValidationBuilder().ValidateURL(config, "url")
		if vb.HasErrors() {
			t.Error("expected no errors for valid URL")
		}
	})

	t.Run("skips empty URL", func(t *testing.T) {
		config := map[string]any{}
		vb := NewValidationBuilder().ValidateURL(config, "url")
		if vb.HasErrors() {
			t.Error("expected no errors for missing URL")
		}
	})

	t.Run("fails for invalid URL", func(t *testing.T) {
		config := map[string]any{"url": "://invalid"}
		vb := NewValidationBuilder().ValidateURL(config, "url")
		if !vb.HasErrors() {
			t.Error("expected error for invalid URL")
		}
	})
}

func TestValidateEnum_AllowsValidValue(t *testing.T) {
	config := map[string]any{"level": "medium"}
	vb := NewValidationBuilder().ValidateEnum(config, "level", []string{"low", "medium", "high"})
	if vb.HasErrors() {
		t.Error("expected no errors for valid enum value")
	}
}

func TestValidateEnum_SkipsEmptyValue(t *testing.T) {
	config := map[string]any{}
	vb := NewValidationBuilder().ValidateEnum(config, "level", []string{"low", "medium", "high"})
	if vb.HasErrors() {
		t.Error("expected no errors for missing enum value")
	}
}

func TestBuildMentionText_DiscordSpecialCases(t *testing.T) {
	tests := []struct {
		name     string
		mentions []string
		want     string
	}{
		{
			name:     "discord channel mention",
			mentions: []string{"<#123456789>"},
			want:     "<#123456789>",
		},
		{
			name:     "discord role mention",
			mentions: []string{"<@&123456789>"},
			want:     "<@&123456789>",
		},
		{
			name:     "discord @everyone",
			mentions: []string{"@everyone"},
			want:     "@everyone",
		},
		{
			name:     "discord @here",
			mentions: []string{"@here"},
			want:     "@here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMentionText(tt.mentions, MentionFormatDiscord)
			if got != tt.want {
				t.Errorf("BuildMentionText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMentionText_SlackAtPrefix(t *testing.T) {
	got := BuildMentionText([]string{"@user1"}, MentionFormatSlack)
	if got != "<@user1>" {
		t.Errorf("BuildMentionText(@user1) = %q, want %q", got, "<@user1>")
	}
}
