package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/release"
)

func TestCancelCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"reason flag", "reason"},
		{"force flag", "force"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cancelCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("cancel command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestResetCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"force flag", "force"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := resetCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("reset command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestCancelCommand_Configuration(t *testing.T) {
	if cancelCmd == nil {
		t.Fatal("cancelCmd is nil")
	}
	if cancelCmd.Use != "cancel" {
		t.Errorf("cancelCmd.Use = %v, want cancel", cancelCmd.Use)
	}
	if cancelCmd.RunE == nil {
		t.Error("cancelCmd.RunE is nil")
	}
}

func TestResetCommand_Configuration(t *testing.T) {
	if resetCmd == nil {
		t.Fatal("resetCmd is nil")
	}
	if resetCmd.Use != "reset" {
		t.Errorf("resetCmd.Use = %v, want reset", resetCmd.Use)
	}
	if resetCmd.RunE == nil {
		t.Error("resetCmd.RunE is nil")
	}
}

func TestValidateCancelState_Initialized(t *testing.T) {
	rel := release.NewRelease("test-id", "main", "/test/repo")
	// Initialized state should be cancelable
	err := validateCancelState(rel)
	if err != nil {
		t.Errorf("validateCancelState() should allow canceling initialized state, got: %v", err)
	}
}

func TestValidateResetState_NotFailedOrCanceled(t *testing.T) {
	rel := release.NewRelease("test-id", "main", "/test/repo")
	// In initialized state, reset should fail
	err := validateResetState(rel)
	if err == nil {
		t.Error("validateResetState() should return error for initialized state")
	}
}

func TestGetCurrentUser(t *testing.T) {
	user := getCurrentUser()
	// Should return something, even if it's "unknown"
	if user == "" {
		t.Error("getCurrentUser() returned empty string")
	}
}
