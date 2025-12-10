// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"testing"
)

func TestApproveCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"yes flag", "yes"},
		{"edit flag", "edit"},
		{"editor flag", "editor"},
		{"interactive flag", "interactive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := approveCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("approve command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestApproveCommand_Configuration(t *testing.T) {
	if approveCmd == nil {
		t.Fatal("approveCmd is nil")
	}
	if approveCmd.Use != "approve" {
		t.Errorf("approveCmd.Use = %v, want approve", approveCmd.Use)
	}
	if approveCmd.RunE == nil {
		t.Error("approveCmd.RunE is nil")
	}
}

func TestAllowedEditors_CommonEditors(t *testing.T) {
	tests := []struct {
		name   string
		editor string
		want   bool
	}{
		{"vim is allowed", "vim", true},
		{"nvim is allowed", "nvim", true},
		{"nano is allowed", "nano", true},
		{"emacs is allowed", "emacs", true},
		{"vi is allowed", "vi", true},
		{"code is allowed", "code", true},
		{"subl is allowed", "subl", true},
		{"gedit is allowed", "gedit", true},
		{"kate is allowed", "kate", true},
		{"micro is allowed", "micro", true},
		{"unknown editor not allowed", "malicious", false},
		{"empty string not allowed", "", false},
		{"command injection not allowed", "vim; rm -rf /", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allowedEditors[tt.editor]
			if got != tt.want {
				t.Errorf("allowedEditors[%q] = %v, want %v", tt.editor, got, tt.want)
			}
		})
	}
}

func TestAllowedEditors_SecurityCheck(t *testing.T) {
	// Test that dangerous commands are NOT in the allowed list
	dangerousInputs := []string{
		"rm",
		"bash",
		"sh",
		"/bin/bash",
		"sudo",
		"su",
		"curl",
		"wget",
		"nc",
		"telnet",
	}

	for _, dangerous := range dangerousInputs {
		t.Run("blocks "+dangerous, func(t *testing.T) {
			if allowedEditors[dangerous] {
				t.Errorf("allowedEditors should NOT allow dangerous command: %s", dangerous)
			}
		})
	}
}

func TestAllowedEditors_Count(t *testing.T) {
	// Verify we have a reasonable number of editors
	count := len(allowedEditors)
	if count < 5 {
		t.Errorf("allowedEditors has only %d editors, expected at least 5", count)
	}
	if count > 20 {
		t.Errorf("allowedEditors has %d editors, this seems excessive", count)
	}
}
