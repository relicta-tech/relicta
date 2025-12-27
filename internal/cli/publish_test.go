// Package cli provides the command-line interface for Relicta.
package cli

import (
	"testing"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

func TestPublishCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"skip-approval flag", "skip-approval"},
		{"skip-tag flag", "skip-tag"},
		{"skip-push flag", "skip-push"},
		{"skip-plugins flag", "skip-plugins"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := publishCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("publish command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestPublishCommand_DefaultValues(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		wantDefault string
	}{
		{"skip-approval default", "skip-approval", "false"},
		{"skip-tag default", "skip-tag", "false"},
		{"skip-push default", "skip-push", "false"},
		{"skip-plugins default", "skip-plugins", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := publishCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("%s flag not found", tt.flagName)
			}
			if flag.DefValue != tt.wantDefault {
				t.Errorf("%s flag default = %v, want %v", tt.flagName, flag.DefValue, tt.wantDefault)
			}
		})
	}
}

func TestShouldCreateTag(t *testing.T) {
	// Save original values
	origCfg := cfg
	origSkipTag := publishSkipTag
	defer func() {
		cfg = origCfg
		publishSkipTag = origSkipTag
	}()

	tests := []struct {
		name    string
		skipTag bool
		gitTag  bool
		want    bool
	}{
		{
			name:    "create tag when not skipped and config enabled",
			skipTag: false,
			gitTag:  true,
			want:    true,
		},
		{
			name:    "skip when flag set",
			skipTag: true,
			gitTag:  true,
			want:    false,
		},
		{
			name:    "skip when config disabled",
			skipTag: false,
			gitTag:  false,
			want:    false,
		},
		{
			name:    "skip when both disabled",
			skipTag: true,
			gitTag:  false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipTag = tt.skipTag
			cfg = &config.Config{
				Versioning: config.VersioningConfig{
					GitTag: tt.gitTag,
				},
			}

			got := shouldCreateTag()
			if got != tt.want {
				t.Errorf("shouldCreateTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldPushTag(t *testing.T) {
	// Save original values
	origCfg := cfg
	origSkipPush := publishSkipPush
	defer func() {
		cfg = origCfg
		publishSkipPush = origSkipPush
	}()

	tests := []struct {
		name     string
		skipPush bool
		gitPush  bool
		want     bool
	}{
		{
			name:     "push when not skipped and config enabled",
			skipPush: false,
			gitPush:  true,
			want:     true,
		},
		{
			name:     "skip when flag set",
			skipPush: true,
			gitPush:  true,
			want:     false,
		},
		{
			name:     "skip when config disabled",
			skipPush: false,
			gitPush:  false,
			want:     false,
		},
		{
			name:     "skip when both disabled",
			skipPush: true,
			gitPush:  false,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipPush = tt.skipPush
			cfg = &config.Config{
				Versioning: config.VersioningConfig{
					GitPush: tt.gitPush,
				},
			}

			got := shouldPushTag()
			if got != tt.want {
				t.Errorf("shouldPushTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldRunPlugins(t *testing.T) {
	// Save original values
	origCfg := cfg
	origSkipPlugins := publishSkipPlugins
	defer func() {
		cfg = origCfg
		publishSkipPlugins = origSkipPlugins
	}()

	tests := []struct {
		name        string
		skipPlugins bool
		plugins     []config.PluginConfig
		want        bool
	}{
		{
			name:        "run plugins when not skipped and plugins configured",
			skipPlugins: false,
			plugins: []config.PluginConfig{
				{Name: "github", Enabled: boolPtr(true)},
			},
			want: true,
		},
		{
			name:        "skip when flag set",
			skipPlugins: true,
			plugins: []config.PluginConfig{
				{Name: "github", Enabled: boolPtr(true)},
			},
			want: false,
		},
		{
			name:        "skip when no plugins configured",
			skipPlugins: false,
			plugins:     []config.PluginConfig{},
			want:        false,
		},
		{
			name:        "skip when nil plugins",
			skipPlugins: false,
			plugins:     nil,
			want:        false,
		},
		{
			name:        "skip when both conditions false",
			skipPlugins: true,
			plugins:     nil,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipPlugins = tt.skipPlugins
			cfg = &config.Config{
				Plugins: tt.plugins,
			}

			got := shouldRunPlugins()
			if got != tt.want {
				t.Errorf("shouldRunPlugins() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPublishInput(t *testing.T) {
	// Save original values
	origCfg := cfg
	origDryRun := dryRun
	origSkipTag := publishSkipTag
	origSkipPush := publishSkipPush
	defer func() {
		cfg = origCfg
		dryRun = origDryRun
		publishSkipTag = origSkipTag
		publishSkipPush = origSkipPush
	}()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
			GitTag:    true,
			GitPush:   true,
		},
	}

	rel := release.NewReleaseRunForTest(release.RunID("test-rel-id"), "main", "test-repo")

	tests := []struct {
		name       string
		dryRunMode bool
		skipTag    bool
		skipPush   bool
		wantDryRun bool
		wantTag    bool
		wantPush   bool
	}{
		{
			name:       "normal publish",
			dryRunMode: false,
			skipTag:    false,
			skipPush:   false,
			wantDryRun: false,
			wantTag:    true,
			wantPush:   true,
		},
		{
			name:       "dry run mode",
			dryRunMode: true,
			skipTag:    false,
			skipPush:   false,
			wantDryRun: true,
			wantTag:    true,
			wantPush:   true,
		},
		{
			name:       "skip tag",
			dryRunMode: false,
			skipTag:    true,
			skipPush:   false,
			wantDryRun: false,
			wantTag:    false,
			wantPush:   true,
		},
		{
			name:       "skip push",
			dryRunMode: false,
			skipTag:    false,
			skipPush:   true,
			wantDryRun: false,
			wantTag:    true,
			wantPush:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dryRun = tt.dryRunMode
			publishSkipTag = tt.skipTag
			publishSkipPush = tt.skipPush

			input := buildPublishInput(rel)

			if input.ReleaseID != rel.ID() {
				t.Errorf("buildPublishInput() ReleaseID = %v, want %v", input.ReleaseID, rel.ID())
			}

			if input.DryRun != tt.wantDryRun {
				t.Errorf("buildPublishInput() DryRun = %v, want %v", input.DryRun, tt.wantDryRun)
			}

			if input.CreateTag != tt.wantTag {
				t.Errorf("buildPublishInput() CreateTag = %v, want %v", input.CreateTag, tt.wantTag)
			}

			if input.PushTag != tt.wantPush {
				t.Errorf("buildPublishInput() PushTag = %v, want %v", input.PushTag, tt.wantPush)
			}

			if input.TagPrefix != "v" {
				t.Errorf("buildPublishInput() TagPrefix = %v, want v", input.TagPrefix)
			}

			if input.Remote != "origin" {
				t.Errorf("buildPublishInput() Remote = %v, want origin", input.Remote)
			}
		})
	}
}

func TestOutputPublishResults(t *testing.T) {
	tests := []struct {
		name   string
		output *apprelease.PublishReleaseOutput
	}{
		{
			name: "with tag and release URL",
			output: &apprelease.PublishReleaseOutput{
				TagName:    "v1.0.0",
				ReleaseURL: "https://github.com/test/repo/releases/tag/v1.0.0",
			},
		},
		{
			name: "with tag only",
			output: &apprelease.PublishReleaseOutput{
				TagName:    "v1.0.0",
				ReleaseURL: "",
			},
		},
		{
			name: "with release URL only",
			output: &apprelease.PublishReleaseOutput{
				TagName:    "",
				ReleaseURL: "https://github.com/test/repo/releases/tag/v1.0.0",
			},
		},
		{
			name: "empty output",
			output: &apprelease.PublishReleaseOutput{
				TagName:    "",
				ReleaseURL: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputPublishResults(tt.output)
		})
	}
}

func TestOutputPluginResults(t *testing.T) {
	tests := []struct {
		name    string
		results []apprelease.PluginResult
	}{
		{
			name:    "no results",
			results: []apprelease.PluginResult{},
		},
		{
			name: "single success",
			results: []apprelease.PluginResult{
				{PluginName: "github", Success: true, Message: "Release published"},
			},
		},
		{
			name: "single failure",
			results: []apprelease.PluginResult{
				{PluginName: "npm", Success: false, Message: "Failed to publish"},
			},
		},
		{
			name: "mixed results",
			results: []apprelease.PluginResult{
				{PluginName: "github", Success: true, Message: "Release published"},
				{PluginName: "npm", Success: false, Message: "Failed to publish"},
				{PluginName: "slack", Success: true, Message: "Notification sent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic
			outputPluginResults(tt.results)
		})
	}
}

func TestDisplayPublishActions(t *testing.T) {
	// Save original values
	origCfg := cfg
	origSkipTag := publishSkipTag
	origSkipPush := publishSkipPush
	origSkipPlugins := publishSkipPlugins
	defer func() {
		cfg = origCfg
		publishSkipTag = origSkipTag
		publishSkipPush = origSkipPush
		publishSkipPlugins = origSkipPlugins
	}()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
			GitTag:    true,
			GitPush:   true,
		},
		Plugins: []config.PluginConfig{
			{Name: "github", Enabled: boolPtr(true)},
		},
	}

	tests := []struct {
		name        string
		version     string
		skipTag     bool
		skipPush    bool
		skipPlugins bool
	}{
		{
			name:        "all enabled",
			version:     "1.0.0",
			skipTag:     false,
			skipPush:    false,
			skipPlugins: false,
		},
		{
			name:        "all disabled",
			version:     "2.0.0",
			skipTag:     true,
			skipPush:    true,
			skipPlugins: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishSkipTag = tt.skipTag
			publishSkipPush = tt.skipPush
			publishSkipPlugins = tt.skipPlugins

			// Just verify it doesn't panic
			displayPublishActions(tt.version)
		})
	}
}
