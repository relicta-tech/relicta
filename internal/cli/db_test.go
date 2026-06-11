package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// TestNewDBCommands verifies construction.
func TestNewDBCommands(t *testing.T) {
	called := false
	getCfg := func() config.PersistenceConfig {
		called = true
		return config.PersistenceConfig{Backend: config.BackendFile}
	}

	cmds := NewDBCommands(getCfg)
	if cmds == nil {
		t.Fatal("NewDBCommands() returned nil")
	}

	// Trigger the config provider.
	cmds.getConfig()
	if !called {
		t.Error("getConfig was not called")
	}
}

// TestDBCommands_RunMigrate_NonPostgres verifies early-exit for non-postgres backend.
func TestDBCommands_RunMigrate_NonPostgres(t *testing.T) {
	cmds := NewDBCommands(func() config.PersistenceConfig {
		return config.PersistenceConfig{Backend: config.BackendFile}
	})

	err := cmds.RunMigrate(context.Background())
	if err == nil {
		t.Error("RunMigrate() should fail for non-postgres backend")
	}
}

// TestDBCommands_RunMigrateDown_NonPostgres verifies early-exit for non-postgres backend.
func TestDBCommands_RunMigrateDown_NonPostgres(t *testing.T) {
	cmds := NewDBCommands(func() config.PersistenceConfig {
		return config.PersistenceConfig{Backend: config.BackendFile}
	})

	err := cmds.RunMigrateDown(context.Background())
	if err == nil {
		t.Error("RunMigrateDown() should fail for non-postgres backend")
	}
}

// TestDBCommands_RunStatus_NonPostgres verifies early-exit for non-postgres backend.
func TestDBCommands_RunStatus_NonPostgres(t *testing.T) {
	cmds := NewDBCommands(func() config.PersistenceConfig {
		return config.PersistenceConfig{Backend: config.BackendFile}
	})

	err := cmds.RunStatus(context.Background())
	if err == nil {
		t.Error("RunStatus() should fail for non-postgres backend")
	}
}

// TestDBCommands_RunMigrate_EmptyBackend verifies that empty backend also fails.
func TestDBCommands_RunMigrate_EmptyBackend(t *testing.T) {
	cmds := NewDBCommands(func() config.PersistenceConfig {
		return config.PersistenceConfig{Backend: ""}
	})

	err := cmds.RunMigrate(context.Background())
	if err == nil {
		t.Error("RunMigrate() should fail for empty backend")
	}
}
