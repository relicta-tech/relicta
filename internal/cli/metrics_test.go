// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestMetricsCommand_Configuration(t *testing.T) {
	if metricsCmd == nil {
		t.Fatal("metricsCmd is nil")
	}
	if metricsCmd.Use != "metrics" {
		t.Errorf("metricsCmd.Use = %v, want metrics", metricsCmd.Use)
	}
	if metricsCmd.RunE == nil {
		t.Error("metricsCmd.RunE is nil")
	}
}

func TestMetricsCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"port flag", "port"},
		{"host flag", "host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := metricsCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("metrics command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestMetricsCommand_FlagShorthands(t *testing.T) {
	tests := []struct {
		name          string
		flagName      string
		wantShorthand string
	}{
		{"port shorthand", "port", "p"},
		{"host shorthand", "host", "H"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := metricsCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("%s flag not found", tt.flagName)
			}
			if flag.Shorthand != tt.wantShorthand {
				t.Errorf("%s flag shorthand = %v, want %v", tt.flagName, flag.Shorthand, tt.wantShorthand)
			}
		})
	}
}

func TestMetricsCommand_DefaultValues(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		wantDefault string
	}{
		{"port default", "port", "9090"},
		{"host default", "host", "0.0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := metricsCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("%s flag not found", tt.flagName)
			}
			if flag.DefValue != tt.wantDefault {
				t.Errorf("%s flag default = %v, want %v", tt.flagName, flag.DefValue, tt.wantDefault)
			}
		})
	}
}

func TestMetricsCommand_HasDescription(t *testing.T) {
	if metricsCmd.Short == "" {
		t.Error("metrics command should have a short description")
	}
	if metricsCmd.Long == "" {
		t.Error("metrics command should have a long description")
	}
}

func TestMetricsCommand_IsAddedToRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "metrics" {
			found = true
			break
		}
	}
	if !found {
		t.Error("metrics command should be added to root command")
	}
}

func TestRunMetrics_ContextCanceled(t *testing.T) {
	origPort := metricsPort
	origHost := metricsHost
	defer func() {
		metricsPort = origPort
		metricsHost = origHost
	}()

	metricsPort = 0
	metricsHost = "127.0.0.1"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	if err := runMetrics(cmd, nil); err != nil {
		t.Fatalf("runMetrics error: %v", err)
	}
}
