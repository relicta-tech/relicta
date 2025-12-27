package cli

import (
	"testing"
)

func TestMCPCommand_Exists(t *testing.T) {
	if mcpCmd == nil {
		t.Fatal("mcpCmd is nil")
	}
	if mcpCmd.Use != "mcp" {
		t.Errorf("mcpCmd.Use = %q, want %q", mcpCmd.Use, "mcp")
	}
}

func TestMCPServeCommand_Exists(t *testing.T) {
	if mcpServeCmd == nil {
		t.Fatal("mcpServeCmd is nil")
	}
	if mcpServeCmd.Use != "serve" {
		t.Errorf("mcpServeCmd.Use = %q, want %q", mcpServeCmd.Use, "serve")
	}
}

func TestMCPCommand_HasServeSubcommand(t *testing.T) {
	var hasServe bool
	for _, cmd := range mcpCmd.Commands() {
		if cmd.Use == "serve" {
			hasServe = true
			break
		}
	}
	if !hasServe {
		t.Error("mcpCmd should have 'serve' subcommand")
	}
}
