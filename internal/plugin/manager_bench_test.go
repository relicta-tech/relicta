package plugin

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/pkg/plugin"
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
