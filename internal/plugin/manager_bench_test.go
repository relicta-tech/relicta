package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/integration"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/plugin/sandbox"
	"github.com/relicta-tech/relicta/v4/pkg/plugin"
)

func BenchmarkNewManager(b *testing.B) {
	b.ReportAllocs()

	b.Run("empty_config", func(b *testing.B) {
		cfg := &config.Config{}
		for i := 0; i < b.N; i++ {
			_ = NewManager(cfg)
		}
	})

	b.Run("with_plugins", func(b *testing.B) {
		cfg := &config.Config{
			Plugins: []config.PluginConfig{
				{Name: "github", Enabled: ptr(true)},
				{Name: "slack", Enabled: ptr(true)},
				{Name: "jira", Enabled: ptr(true)},
			},
		}
		for i := 0; i < b.N; i++ {
			_ = NewManager(cfg)
		}
	})
}

func BenchmarkValidatePluginName(b *testing.B) {
	b.ReportAllocs()

	b.Run("valid_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validatePluginName("github")
		}
	})

	b.Run("valid_long", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validatePluginName("my-custom-plugin-with-long-name")
		}
	})

	b.Run("valid_with_numbers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validatePluginName("plugin-v2-beta1")
		}
	})

	b.Run("invalid", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validatePluginName("../evil")
		}
	})
}

func BenchmarkCollectPluginsForHook(b *testing.B) {
	b.ReportAllocs()

	// Create manager with mock plugins
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "github", Enabled: ptr(true)},
			{Name: "slack", Enabled: ptr(true)},
			{Name: "jira", Enabled: ptr(true)},
		},
	}
	m := NewManager(cfg)

	// Manually populate plugins map for benchmarking
	m.plugins["github"] = &loadedPlugin{
		name: "github",
		info: plugin.Info{
			Name:    "github",
			Version: "1.0.0",
			Hooks:   []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
		},
	}
	m.plugins["slack"] = &loadedPlugin{
		name: "slack",
		info: plugin.Info{
			Name:    "slack",
			Version: "1.0.0",
			Hooks:   []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess, plugin.HookOnError},
		},
	}
	m.plugins["jira"] = &loadedPlugin{
		name: "jira",
		info: plugin.Info{
			Name:    "jira",
			Version: "1.0.0",
			Hooks:   []plugin.Hook{plugin.HookPostPublish},
		},
	}

	b.Run("PostPublish_3_plugins", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})

	b.Run("OnSuccess_2_plugins", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookOnSuccess)
		}
	})

	b.Run("OnError_1_plugin", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookOnError)
		}
	})

	b.Run("PreInit_0_plugins", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPreInit)
		}
	})
}

func BenchmarkCollectPluginsForHook_Large(b *testing.B) {
	b.ReportAllocs()

	// Create manager with many plugins
	pluginCount := 20
	cfg := &config.Config{
		Plugins: make([]config.PluginConfig, pluginCount),
	}

	for i := 0; i < pluginCount; i++ {
		cfg.Plugins[i] = config.PluginConfig{
			Name:    "plugin-" + string(rune('a'+i)),
			Enabled: ptr(true),
		}
	}

	m := NewManager(cfg)

	// Populate plugins map
	for i := 0; i < pluginCount; i++ {
		name := "plugin-" + string(rune('a'+i))
		hooks := []plugin.Hook{plugin.HookPostPublish}
		if i%2 == 0 {
			hooks = append(hooks, plugin.HookOnSuccess)
		}
		if i%3 == 0 {
			hooks = append(hooks, plugin.HookOnError)
		}

		m.plugins[name] = &loadedPlugin{
			name: name,
			info: plugin.Info{
				Name:    name,
				Version: "1.0.0",
				Hooks:   hooks,
			},
		}
	}

	b.Run("PostPublish_all", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})

	b.Run("OnSuccess_half", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookOnSuccess)
		}
	})

	b.Run("OnError_third", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookOnError)
		}
	})
}

func BenchmarkPluginSupportsHook(b *testing.B) {
	b.ReportAllocs()

	lp := &loadedPlugin{
		name: "test",
		info: plugin.Info{
			Hooks: []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess, plugin.HookOnError},
		},
	}

	cfg := &config.Config{}
	m := NewManager(cfg)

	b.Run("found_first", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.pluginSupportsHook(lp, plugin.HookPostPublish)
		}
	})

	b.Run("found_last", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.pluginSupportsHook(lp, plugin.HookOnError)
		}
	})

	b.Run("not_found", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.pluginSupportsHook(lp, plugin.HookPreInit)
		}
	})
}

func BenchmarkManager_ListPlugins(b *testing.B) {
	b.ReportAllocs()

	cfg := &config.Config{}
	m := NewManager(cfg)

	// Add various plugins
	for i := 0; i < 10; i++ {
		name := "plugin-" + string(rune('a'+i))
		m.plugins[name] = &loadedPlugin{
			name: name,
			info: plugin.Info{
				Name:    name,
				Version: "1.0.0",
			},
		}
	}

	b.Run("list_10_plugins", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.ListPlugins()
		}
	})
}

func BenchmarkJoinErrors(b *testing.B) {
	b.ReportAllocs()

	b.Run("empty", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = joinErrors(nil)
		}
	})

	b.Run("single", func(b *testing.B) {
		errs := []string{"error 1"}
		for i := 0; i < b.N; i++ {
			_ = joinErrors(errs)
		}
	})

	b.Run("multiple", func(b *testing.B) {
		errs := []string{"error 1", "error 2", "error 3", "error 4", "error 5"}
		for i := 0; i < b.N; i++ {
			_ = joinErrors(errs)
		}
	})
}

// Helper function
func ptr[T any](v T) *T {
	return &v
}

// ============================================================================
// gRPC and Sandbox Benchmarks
// ============================================================================

// BenchmarkSandbox_New measures sandbox creation overhead.
// Target: < 1ms for sandbox creation.
func BenchmarkSandbox_New(b *testing.B) {
	b.ReportAllocs()

	b.Run("default_caps", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sandbox.New("test-plugin", nil)
		}
	})

	b.Run("custom_caps_minimal", func(b *testing.B) {
		caps := &config.PluginCapabilities{
			AllowNetwork:    false,
			AllowFilesystem: false,
			AllowEnvRead:    false,
		}
		for i := 0; i < b.N; i++ {
			_ = sandbox.New("test-plugin", caps)
		}
	})

	b.Run("custom_caps_full", func(b *testing.B) {
		caps := &config.PluginCapabilities{
			AllowNetwork:    true,
			AllowFilesystem: true,
			AllowedPaths:    []string{"/tmp", "/var/log", "/home/user/.config"},
			AllowEnvRead:    true,
			AllowedEnvVars:  []string{"HOME", "PATH", "GITHUB_TOKEN", "SLACK_TOKEN"},
			MaxMemoryMB:     1024,
			MaxCPUPercent:   75,
		}
		for i := 0; i < b.N; i++ {
			_ = sandbox.New("test-plugin", caps)
		}
	})
}

// BenchmarkSandbox_PrepareCommand measures command preparation overhead.
// This is critical for plugin initialization latency.
func BenchmarkSandbox_PrepareCommand(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	b.Run("default_caps", func(b *testing.B) {
		sb := sandbox.New("test-plugin", nil)
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("echo", "test")
			_ = sb.PrepareCommand(ctx, cmd)
		}
	})

	b.Run("restricted_env", func(b *testing.B) {
		caps := &config.PluginCapabilities{
			AllowEnvRead:   true,
			AllowedEnvVars: []string{"PATH", "HOME", "GITHUB_TOKEN"},
		}
		sb := sandbox.New("test-plugin", caps)
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("echo", "test")
			_ = sb.PrepareCommand(ctx, cmd)
		}
	})

	b.Run("no_env_read", func(b *testing.B) {
		caps := &config.PluginCapabilities{
			AllowEnvRead: false,
		}
		sb := sandbox.New("test-plugin", caps)
		for i := 0; i < b.N; i++ {
			cmd := exec.Command("echo", "test")
			_ = sb.PrepareCommand(ctx, cmd)
		}
	})
}

// BenchmarkAdapter_TypeConversion measures the overhead of converting
// between domain types and plugin types - critical for gRPC serialization.
func BenchmarkAdapter_TypeConversion(b *testing.B) {
	b.ReportAllocs()

	// Create test data
	testCommit := func(hash string) *changes.ConventionalCommit {
		return changes.NewConventionalCommit(
			hash,
			changes.CommitTypeFeat,
			"add new feature",
			changes.WithScope("api"),
			changes.WithBody("This is a detailed body\n\nWith multiple paragraphs"),
			changes.WithAuthor("Test User", "test@example.com"),
		)
	}

	b.Run("context_conversion_minimal", func(b *testing.B) {
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("1.2.3"),
			PreviousVersion: version.MustParse("1.2.2"),
			ReleaseType:     changes.ReleaseTypePatch,
			RepositoryOwner: "org",
			RepositoryName:  "repo",
			Branch:          "main",
		}
		for i := 0; i < b.N; i++ {
			_ = toPluginReleaseContext(releaseCtx)
		}
	})

	b.Run("context_conversion_with_changelog", func(b *testing.B) {
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("1.2.3"),
			PreviousVersion: version.MustParse("1.2.2"),
			ReleaseType:     changes.ReleaseTypePatch,
			RepositoryOwner: "org",
			RepositoryName:  "repo",
			Branch:          "main",
			Changelog:       "## v1.2.3\n\n### Features\n- Added new API endpoint\n- Improved performance",
			ReleaseNotes:    "This release includes important bug fixes and improvements.",
		}
		for i := 0; i < b.N; i++ {
			_ = toPluginReleaseContext(releaseCtx)
		}
	})

	b.Run("context_conversion_with_10_commits", func(b *testing.B) {
		cs := changes.NewChangeSet("test", "v1.2.2", "HEAD")
		for i := 0; i < 10; i++ {
			cs.AddCommit(testCommit(fmt.Sprintf("abc%04d", i)))
		}
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("1.2.3"),
			PreviousVersion: version.MustParse("1.2.2"),
			ReleaseType:     changes.ReleaseTypeMinor,
			Changes:         cs,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = toPluginReleaseContext(releaseCtx)
		}
	})

	b.Run("context_conversion_with_100_commits", func(b *testing.B) {
		cs := changes.NewChangeSet("test", "v1.2.2", "HEAD")
		for i := 0; i < 100; i++ {
			cs.AddCommit(testCommit(fmt.Sprintf("abc%04d", i)))
		}
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("1.2.3"),
			PreviousVersion: version.MustParse("1.2.2"),
			ReleaseType:     changes.ReleaseTypeMajor,
			Changes:         cs,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = toPluginReleaseContext(releaseCtx)
		}
	})

	b.Run("context_conversion_with_1000_commits", func(b *testing.B) {
		cs := changes.NewChangeSet("test", "v1.2.2", "HEAD")
		for i := 0; i < 1000; i++ {
			cs.AddCommit(testCommit(fmt.Sprintf("abc%04d", i)))
		}
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("1.2.3"),
			PreviousVersion: version.MustParse("1.2.2"),
			ReleaseType:     changes.ReleaseTypeMajor,
			Changes:         cs,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = toPluginReleaseContext(releaseCtx)
		}
	})
}

// BenchmarkAdapter_ResponseConversion measures response conversion overhead.
func BenchmarkAdapter_ResponseConversion(b *testing.B) {
	b.ReportAllocs()

	b.Run("single_success", func(b *testing.B) {
		responses := []plugin.ExecuteResponse{
			{Success: true, Message: "Done"},
		}
		for i := 0; i < b.N; i++ {
			_ = toIntegrationResponses(responses)
		}
	})

	b.Run("multiple_with_artifacts", func(b *testing.B) {
		responses := []plugin.ExecuteResponse{
			{
				Success: true,
				Message: "GitHub release created",
				Outputs: map[string]any{"release_url": "https://github.com/org/repo/releases/tag/v1.2.3"},
				Artifacts: []plugin.Artifact{
					{Name: "binary-linux", Path: "/dist/app-linux", Type: "binary", Size: 10485760},
					{Name: "binary-darwin", Path: "/dist/app-darwin", Type: "binary", Size: 11534336},
					{Name: "binary-windows", Path: "/dist/app.exe", Type: "binary", Size: 12582912},
				},
			},
			{
				Success: true,
				Message: "Slack notification sent",
				Outputs: map[string]any{"channel": "#releases"},
			},
			{
				Success: true,
				Message: "Jira issue updated",
				Outputs: map[string]any{"issue": "PROJ-123"},
			},
		}
		for i := 0; i < b.N; i++ {
			_ = toIntegrationResponses(responses)
		}
	})

	b.Run("10_plugins_mixed_results", func(b *testing.B) {
		responses := make([]plugin.ExecuteResponse, 10)
		for i := 0; i < 10; i++ {
			responses[i] = plugin.ExecuteResponse{
				Success: i%3 != 0,
				Message: fmt.Sprintf("Plugin %d result", i),
				Outputs: map[string]any{"key": fmt.Sprintf("value%d", i)},
			}
		}
		for i := 0; i < b.N; i++ {
			_ = toIntegrationResponses(responses)
		}
	})
}

// BenchmarkManager_RegistrationScale measures plugin registration at scale.
func BenchmarkManager_RegistrationScale(b *testing.B) {
	b.ReportAllocs()

	createPluginConfigs := func(count int) []config.PluginConfig {
		configs := make([]config.PluginConfig, count)
		for i := 0; i < count; i++ {
			configs[i] = config.PluginConfig{
				Name:    fmt.Sprintf("plugin-%d", i),
				Enabled: ptr(true),
			}
		}
		return configs
	}

	populatePlugins := func(m *Manager, count int) {
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("plugin-%d", i)
			hooks := []plugin.Hook{plugin.HookPostPublish}
			if i%2 == 0 {
				hooks = append(hooks, plugin.HookOnSuccess)
			}
			if i%3 == 0 {
				hooks = append(hooks, plugin.HookOnError)
			}
			if i%5 == 0 {
				hooks = append(hooks, plugin.HookPreInit)
			}
			m.plugins[name] = &loadedPlugin{
				name: name,
				info: plugin.Info{
					Name:    name,
					Version: "1.0.0",
					Hooks:   hooks,
				},
			}
		}
	}

	b.Run("collect_50_plugins", func(b *testing.B) {
		cfg := &config.Config{Plugins: createPluginConfigs(50)}
		m := NewManager(cfg)
		populatePlugins(m, 50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})

	b.Run("collect_100_plugins", func(b *testing.B) {
		cfg := &config.Config{Plugins: createPluginConfigs(100)}
		m := NewManager(cfg)
		populatePlugins(m, 100)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})

	b.Run("list_50_plugins", func(b *testing.B) {
		cfg := &config.Config{Plugins: createPluginConfigs(50)}
		m := NewManager(cfg)
		populatePlugins(m, 50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.ListPlugins()
		}
	})

	b.Run("list_100_plugins", func(b *testing.B) {
		cfg := &config.Config{Plugins: createPluginConfigs(100)}
		m := NewManager(cfg)
		populatePlugins(m, 100)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.ListPlugins()
		}
	})
}

// BenchmarkExecutorAdapter_Creation measures adapter creation overhead.
func BenchmarkExecutorAdapter_Creation(b *testing.B) {
	b.ReportAllocs()

	cfg := &config.Config{}
	m := NewManager(cfg)

	for i := 0; i < b.N; i++ {
		_ = NewExecutorAdapter(m)
	}
}

// BenchmarkPluginDiscovery_LargeScale measures discovery performance at scale.
// This simulates having many plugins installed but only some enabled.
func BenchmarkPluginDiscovery_LargeScale(b *testing.B) {
	b.ReportAllocs()

	createMixedPlugins := func(m *Manager, total, enabled int) {
		for i := 0; i < total; i++ {
			name := fmt.Sprintf("plugin-%d", i)
			if i < enabled {
				m.plugins[name] = &loadedPlugin{
					name: name,
					info: plugin.Info{
						Name:    name,
						Version: "1.0.0",
						Hooks:   []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
					},
				}
			}
		}
	}

	b.Run("100_total_20_enabled", func(b *testing.B) {
		cfg := &config.Config{}
		m := NewManager(cfg)
		createMixedPlugins(m, 100, 20)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})

	b.Run("100_total_50_enabled", func(b *testing.B) {
		cfg := &config.Config{}
		m := NewManager(cfg)
		createMixedPlugins(m, 100, 50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})

	b.Run("100_total_100_enabled", func(b *testing.B) {
		cfg := &config.Config{}
		m := NewManager(cfg)
		createMixedPlugins(m, 100, 100)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}
	})
}

// BenchmarkHookExecution_Overhead measures the non-gRPC overhead of hook execution.
// This isolates the infrastructure cost from actual plugin communication.
func BenchmarkHookExecution_Overhead(b *testing.B) {
	b.ReportAllocs()

	cfg := &config.Config{}
	m := NewManager(cfg)

	// Populate with plugins
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("plugin-%d", i)
		m.plugins[name] = &loadedPlugin{
			name: name,
			info: plugin.Info{
				Name:    name,
				Version: "1.0.0",
				Hooks:   []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
			},
		}
	}

	b.Run("collect_and_check_5_plugins", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			plugins := m.collectPluginsForHook(plugin.HookPostPublish)
			// Access the collected plugin info
			for _, p := range plugins {
				_ = p.name // Access plugin name to simulate usage
			}
		}
	})
}

// BenchmarkContextPreparation measures the full context preparation pipeline.
// This is what happens before any gRPC call is made.
func BenchmarkContextPreparation(b *testing.B) {
	b.ReportAllocs()

	testCommit := func(hash string, ctype changes.CommitType) *changes.ConventionalCommit {
		return changes.NewConventionalCommit(
			hash,
			ctype,
			"commit message here",
			changes.WithScope("core"),
			changes.WithBody("Detailed body with information"),
			changes.WithAuthor("Developer", "dev@example.com"),
		)
	}

	b.Run("full_pipeline_small", func(b *testing.B) {
		cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
		for i := 0; i < 10; i++ {
			cs.AddCommit(testCommit(fmt.Sprintf("hash%d", i), changes.CommitTypeFeat))
		}
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("1.1.0"),
			PreviousVersion: version.MustParse("1.0.0"),
			ReleaseType:     changes.ReleaseTypeMinor,
			RepositoryOwner: "org",
			RepositoryName:  "repo",
			Branch:          "main",
			TagName:         "v1.1.0",
			Changes:         cs,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate full preparation: context conversion + sandbox setup
			_ = toPluginReleaseContext(releaseCtx)
			sb := sandbox.New("test-plugin", nil)
			cmd := exec.Command("echo", "test")
			_ = sb.PrepareCommand(context.Background(), cmd)
		}
	})

	b.Run("full_pipeline_large", func(b *testing.B) {
		cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
		commitTypes := []changes.CommitType{
			changes.CommitTypeFeat, changes.CommitTypeFix,
			changes.CommitTypePerf, changes.CommitTypeDocs,
		}
		for i := 0; i < 200; i++ {
			cs.AddCommit(testCommit(fmt.Sprintf("hash%d", i), commitTypes[i%len(commitTypes)]))
		}
		releaseCtx := integration.ReleaseContext{
			Version:         version.MustParse("2.0.0"),
			PreviousVersion: version.MustParse("1.5.0"),
			ReleaseType:     changes.ReleaseTypeMajor,
			RepositoryOwner: "organization",
			RepositoryName:  "repository",
			Branch:          "main",
			TagName:         "v2.0.0",
			Changelog:       "## v2.0.0\n\nMajor release with breaking changes\n\n### Breaking Changes\n- API restructured\n- Config format changed",
			ReleaseNotes:    "Version 2.0.0 is a major release with significant improvements",
			Changes:         cs,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = toPluginReleaseContext(releaseCtx)
			sb := sandbox.New("test-plugin", nil)
			cmd := exec.Command("echo", "test")
			_ = sb.PrepareCommand(context.Background(), cmd)
		}
	})
}

// BenchmarkConcurrentPluginOperations measures thread-safety overhead.
func BenchmarkConcurrentPluginOperations(b *testing.B) {
	b.ReportAllocs()

	cfg := &config.Config{}
	m := NewManager(cfg)

	// Populate plugins
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("plugin-%d", i)
		m.plugins[name] = &loadedPlugin{
			name: name,
			info: plugin.Info{
				Name:    name,
				Version: "1.0.0",
				Hooks:   []plugin.Hook{plugin.HookPostPublish},
			},
		}
	}

	b.Run("parallel_collect", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = m.collectPluginsForHook(plugin.HookPostPublish)
			}
		})
	})

	b.Run("parallel_list", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = m.ListPlugins()
			}
		})
	})
}

// BenchmarkPluginLifecycle simulates the full plugin lifecycle without actual gRPC.
// Target: < 200ms total for plugin initialization (without actual binary load).
func BenchmarkPluginLifecycle(b *testing.B) {
	b.ReportAllocs()

	b.Run("lifecycle_simulation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// 1. Create manager
			cfg := &config.Config{
				Plugins: []config.PluginConfig{
					{Name: "github", Enabled: ptr(true)},
					{Name: "slack", Enabled: ptr(true)},
				},
			}
			m := NewManager(cfg)

			// 2. Simulate plugin info registration (normally from gRPC)
			m.plugins["github"] = &loadedPlugin{
				name: "github",
				info: plugin.Info{
					Name:    "github",
					Version: "1.0.0",
					Hooks:   []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
				},
			}
			m.plugins["slack"] = &loadedPlugin{
				name: "slack",
				info: plugin.Info{
					Name:    "slack",
					Version: "1.0.0",
					Hooks:   []plugin.Hook{plugin.HookPostPublish},
				},
			}

			// 3. Create adapter
			adapter := NewExecutorAdapter(m)
			_ = adapter

			// 4. Collect plugins for hook
			plugins := m.collectPluginsForHook(plugin.HookPostPublish)
			_ = plugins

			// 5. Prepare sandbox for each
			for _, p := range plugins {
				sb := sandbox.New(p.name, nil)
				cmd := exec.Command("echo", "test")
				_ = sb.PrepareCommand(context.Background(), cmd)
			}
		}
	})
}

// BenchmarkPluginInitializationTarget validates the <200ms target.
// This measures all infrastructure overhead that contributes to initialization time.
func BenchmarkPluginInitializationTarget(b *testing.B) {
	// This benchmark specifically targets the 200ms initialization budget
	// by measuring cumulative overhead of all pre-gRPC operations.

	b.Run("total_init_overhead_5_plugins", func(b *testing.B) {
		b.ReportAllocs()

		pluginNames := []string{"github", "slack", "jira", "discord", "npm"}

		for i := 0; i < b.N; i++ {
			start := time.Now()

			// Manager creation
			cfg := &config.Config{}
			m := NewManager(cfg)

			// Plugin registration simulation
			for _, name := range pluginNames {
				m.plugins[name] = &loadedPlugin{
					name: name,
					info: plugin.Info{
						Name:    name,
						Version: "1.0.0",
						Hooks:   []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
					},
				}
			}

			// Sandbox preparation for all
			for _, name := range pluginNames {
				sb := sandbox.New(name, nil)
				cmd := exec.Command("echo", "test")
				_ = sb.PrepareCommand(context.Background(), cmd)
			}

			// Verify we're well under 200ms
			elapsed := time.Since(start)
			if elapsed > 200*time.Millisecond {
				b.Errorf("Initialization took %v, exceeds 200ms target", elapsed)
			}
		}
	})
}
