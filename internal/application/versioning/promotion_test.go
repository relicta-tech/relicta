package versioning

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

func TestPromoteReleaseUseCase_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"valid canary to alpha", "canary", "alpha", false},
		{"valid alpha to beta", "alpha", "beta", false},
		{"valid beta to next", "beta", "next", false},
		{"valid next to stable", "next", "stable", false},
		{"invalid stable to canary", "stable", "canary", true},
		{"invalid beta to alpha", "beta", "alpha", true},
		{"unknown source", "nightly", "stable", true},
		{"unknown target", "alpha", "nightly", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			registry := version.NewChannelRegistry()

			// We can only test validation without a full git repo, so use dry run
			uc := NewPromoteReleaseUseCase(nil, registry)

			input := PromoteReleaseInput{
				FromChannel: tt.from,
				ToChannel:   tt.to,
				Version:     "1.0.0-alpha.1",
				DryRun:      true,
			}

			_, err := uc.Execute(context.Background(), input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPromoteReleaseUseCase_VersionCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		from         string
		to           string
		inputVersion string
		wantTarget   string
	}{
		// Promoting to stable strips prerelease
		{"rc to stable", "next", "stable", "1.3.0-rc.2", "1.3.0"},
		{"beta to stable", "beta", "stable", "1.3.0-beta.3", "1.3.0"},
		{"alpha to stable", "alpha", "stable", "1.3.0-alpha.5", "1.3.0"},
		{"canary to stable", "canary", "stable", "1.3.0-canary.1", "1.3.0"},

		// Promoting between prerelease channels
		{"canary to alpha", "canary", "alpha", "1.3.0-canary.5", "1.3.0-alpha.1"},
		{"alpha to beta", "alpha", "beta", "1.3.0-alpha.3", "1.3.0-beta.1"},
		{"beta to next (rc)", "beta", "next", "1.3.0-beta.2", "1.3.0-rc.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			registry := version.NewChannelRegistry()
			uc := NewPromoteReleaseUseCase(nil, registry)

			input := PromoteReleaseInput{
				FromChannel: tt.from,
				ToChannel:   tt.to,
				Version:     tt.inputVersion,
				DryRun:      true,
			}

			output, err := uc.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if output.TargetVersion.String() != tt.wantTarget {
				t.Errorf("TargetVersion = %s, want %s", output.TargetVersion.String(), tt.wantTarget)
			}
			if output.FromChannel != tt.from {
				t.Errorf("FromChannel = %s, want %s", output.FromChannel, tt.from)
			}
			if output.ToChannel != tt.to {
				t.Errorf("ToChannel = %s, want %s", output.ToChannel, tt.to)
			}
		})
	}
}

func TestPromoteReleaseUseCase_NilRegistry(t *testing.T) {
	t.Parallel()

	// Should create default registry when nil is passed
	uc := NewPromoteReleaseUseCase(nil, nil)
	if uc.registry == nil {
		t.Fatal("registry should not be nil after construction with nil")
	}

	// Should still be able to resolve known channels
	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "beta",
		Version:     "1.0.0-alpha.1",
		DryRun:      true,
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output.TargetVersion.String() != "1.0.0-beta.1" {
		t.Errorf("TargetVersion = %s, want 1.0.0-beta.1", output.TargetVersion.String())
	}
}

func TestPromoteReleaseOutput_Fields(t *testing.T) {
	t.Parallel()

	registry := version.NewChannelRegistry()
	uc := NewPromoteReleaseUseCase(nil, registry)

	input := PromoteReleaseInput{
		FromChannel: "alpha",
		ToChannel:   "stable",
		Version:     "2.1.0-alpha.3",
		DryRun:      true,
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.SourceVersion.String() != "2.1.0-alpha.3" {
		t.Errorf("SourceVersion = %s, want 2.1.0-alpha.3", output.SourceVersion.String())
	}
	if output.TargetVersion.String() != "2.1.0" {
		t.Errorf("TargetVersion = %s, want 2.1.0", output.TargetVersion.String())
	}
	if len(output.PromotionPath) != 2 {
		t.Errorf("PromotionPath len = %d, want 2", len(output.PromotionPath))
	}
	if output.PromotedAt.IsZero() {
		t.Error("PromotedAt should not be zero")
	}
}
