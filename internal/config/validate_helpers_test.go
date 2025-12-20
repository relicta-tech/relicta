package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatorWorkflowRequiresMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Workflow.AutoCommitChangelog = true
	cfg.Workflow.ChangelogCommitMessage = ""

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error when auto_commit_changelog enabled without message")
	}
	if !strings.Contains(err.Error(), "workflow.changelog_commit_message") {
		t.Fatalf("expected error mentioning workflow.changelog_commit_message, got %v", err)
	}
}

func TestValidatorOutputErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Output.Format = "xml"
	cfg.Output.LogLevel = "verbose"
	cfg.Output.Quiet = true
	cfg.Output.Verbose = true
	missingDir := filepath.Join(t.TempDir(), "missing")
	cfg.Output.LogFile = filepath.Join(missingDir, "relicta.log")

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation errors for output configuration")
	}

	for _, substr := range []string{
		"output.format",
		"output.log_level",
		"output: quiet and verbose",
		"output.log_file",
	} {
		if !strings.Contains(err.Error(), substr) {
			t.Errorf("expected validation error to mention %q, got %q", substr, err)
		}
	}
}

func TestExpandEnvVarVariants(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv("MY_VAR")
	})

	_ = os.Setenv("MY_VAR", "value")
	if got := expandEnvVar("${MY_VAR}"); got != "value" {
		t.Errorf("expected ${MY_VAR} to expand to value, got %q", got)
	}
	if got := expandEnvVar("$MY_VAR"); got != "value" {
		t.Errorf("expected $MY_VAR to expand to value, got %q", got)
	}
	if got := expandEnvVar("${MISSING:-default}"); got != "default" {
		t.Errorf("expected default fallback, got %q", got)
	}
}

func TestConvertToHTTPSURLVariants(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo"},
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "ssh://git@github.com/owner/repo.git"},
		{"custom://example.com/whatever", "custom://example.com/whatever"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := convertToHTTPSURL(tt.input); got != tt.want {
				t.Errorf("convertToHTTPSURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
