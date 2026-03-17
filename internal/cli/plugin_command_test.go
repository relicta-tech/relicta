package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/plugin/manager"
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func capturePluginStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	os.Stdout = old
	return buf.String()
}

type stubPluginManager struct {
	listAvailable  func(context.Context, bool) ([]manager.PluginListEntry, error)
	listInstalled  func(context.Context) ([]manager.PluginListEntry, error)
	install        func(context.Context, string) error
	uninstall      func(context.Context, string) error
	enable         func(context.Context, string) error
	disable        func(context.Context, string) error
	getInfo        func(context.Context, string) (*manager.PluginListEntry, error)
	listRegistries func() []manager.RegistryEntry
	addRegistry    func(string, string, int) error
	removeRegistry func(string) error
	enableRegistry func(string, bool) error
	search         func(context.Context, string) ([]manager.PluginInfo, error)
}

func (s *stubPluginManager) ListAvailable(ctx context.Context, force bool) ([]manager.PluginListEntry, error) {
	return s.listAvailable(ctx, force)
}

func (s *stubPluginManager) ListInstalled(ctx context.Context) ([]manager.PluginListEntry, error) {
	return s.listInstalled(ctx)
}

func (s *stubPluginManager) Install(ctx context.Context, name string) error {
	return s.install(ctx, name)
}

func (s *stubPluginManager) Uninstall(ctx context.Context, name string) error {
	return s.uninstall(ctx, name)
}

func (s *stubPluginManager) Enable(ctx context.Context, name string) error {
	return s.enable(ctx, name)
}

func (s *stubPluginManager) Disable(ctx context.Context, name string) error {
	return s.disable(ctx, name)
}

func (s *stubPluginManager) GetPluginInfo(ctx context.Context, name string) (*manager.PluginListEntry, error) {
	return s.getInfo(ctx, name)
}

func (s *stubPluginManager) ListRegistries() []manager.RegistryEntry {
	if s.listRegistries == nil {
		return nil
	}

	return s.listRegistries()
}

func (s *stubPluginManager) AddRegistry(name, url string, priority int) error {
	if s.addRegistry == nil {
		return nil
	}

	return s.addRegistry(name, url, priority)
}

func (s *stubPluginManager) RemoveRegistry(name string) error {
	if s.removeRegistry == nil {
		return nil
	}

	return s.removeRegistry(name)
}

func (s *stubPluginManager) EnableRegistry(name string, enabled bool) error {
	if s.enableRegistry == nil {
		return nil
	}

	return s.enableRegistry(name, enabled)
}

func (s *stubPluginManager) Search(ctx context.Context, query string) ([]manager.PluginInfo, error) {
	if s.search == nil {
		return nil, nil
	}

	return s.search(ctx, query)
}

func TestPluginFormatStatusAndCategory(t *testing.T) {
	if formatStatus("enabled") != "enabled" {
		t.Fatalf("unexpected formatStatus")
	}
	if title := getCategoryTitle("vcs"); !strings.Contains(title, "Version") {
		t.Fatalf("unexpected category title %q", title)
	}
}

func TestGetCategoryTitleBranches(t *testing.T) {
	if got := getCategoryTitle("notification"); got != "Notifications" {
		t.Fatalf("unexpected notification title: %q", got)
	}
	if got := getCategoryTitle("container"); got != "Containers" {
		t.Fatalf("unexpected container title: %q", got)
	}
	if got := getCategoryTitle("custom_tools"); got != "Custom_tools" {
		t.Fatalf("unexpected default title: %q", got)
	}
}

func TestRunPluginListAvailable(t *testing.T) {
	origManager := newPluginManager
	origFlag := pluginListAvailable
	defer func() {
		newPluginManager = origManager
		pluginListAvailable = origFlag
	}()

	pluginListAvailable = true
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			listAvailable: func(ctx context.Context, force bool) ([]manager.PluginListEntry, error) {
				return []manager.PluginListEntry{
					{Info: manager.PluginInfo{Name: "github", Category: "vcs"}},
				}, nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := capturePluginStdout(func() {
		if err := runPluginList(cmd, nil); err != nil {
			t.Fatalf("runPluginList error: %v", err)
		}
	})
	if !strings.Contains(out, "Available Plugins") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunPluginInstall(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	called := false
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			install: func(ctx context.Context, name string) error {
				called = true
				return nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := capturePluginStdout(func() {
		if err := runPluginInstall(cmd, []string{"git"}); err != nil {
			t.Fatalf("runPluginInstall error: %v", err)
		}
	})
	if !called {
		t.Fatal("install not called")
	}
	if !strings.Contains(out, "installed successfully") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunPluginEnableDisable(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	enabled := false
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			enable: func(ctx context.Context, name string) error {
				enabled = true
				return nil
			},
			disable: func(ctx context.Context, name string) error {
				enabled = false
				return nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPluginEnable(cmd, []string{"git"}); err != nil {
		t.Fatalf("runPluginEnable error: %v", err)
	}
	if !enabled {
		t.Fatal("expected enabled")
	}

	if err := runPluginDisable(cmd, []string{"git"}); err != nil {
		t.Fatalf("runPluginDisable error: %v", err)
	}
	if enabled {
		t.Fatal("expected disabled")
	}
}

func TestRunPluginRegistryListOutputsEntries(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			listRegistries: func() []manager.RegistryEntry {
				return []manager.RegistryEntry{
					{Name: manager.OfficialRegistryName, URL: "https://example.com", Enabled: true, Priority: 100},
					{Name: "community", URL: "https://community.example/registry.yaml", Enabled: false, Priority: 50},
				}
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := capturePluginStdout(func() {
		if err := runPluginRegistryList(cmd, nil); err != nil {
			t.Fatalf("runPluginRegistryList error: %v", err)
		}
	})
	if !strings.Contains(out, "Plugin Registries:") {
		t.Fatalf("expected registry header, got %q", out)
	}
	if !strings.Contains(out, "(official)") {
		t.Fatalf("expected official registry mentioned, got %q", out)
	}
}

func TestRunPluginRegistryAddCallsManager(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	called := false
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			addRegistry: func(name, url string, priority int) error {
				called = true
				if name != "custom" {
					t.Fatalf("unexpected name %s", name)
				}
				return nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPluginRegistryAdd(cmd, []string{"custom", "https://example.com", "42"}); err != nil {
		t.Fatalf("runPluginRegistryAdd error: %v", err)
	}
	if !called {
		t.Fatal("add registry not called")
	}
}

func TestRunPluginRegistryRemoveEnableDisable(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	removed := ""
	enabled := ""
	disabled := ""

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			removeRegistry: func(name string) error {
				removed = name
				return nil
			},
			enableRegistry: func(name string, state bool) error {
				if state {
					enabled = name
				} else {
					disabled = name
				}
				return nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPluginRegistryRemove(cmd, []string{"community"}); err != nil {
		t.Fatalf("runPluginRegistryRemove error: %v", err)
	}
	if removed != "community" {
		t.Fatalf("expected removed registry to be community, got %s", removed)
	}

	if err := runPluginRegistryEnable(cmd, []string{"community"}); err != nil {
		t.Fatalf("runPluginRegistryEnable error: %v", err)
	}
	if enabled != "community" {
		t.Fatalf("expected enabled registry to be community, got %s", enabled)
	}

	if err := runPluginRegistryDisable(cmd, []string{"community"}); err != nil {
		t.Fatalf("runPluginRegistryDisable error: %v", err)
	}
	if disabled != "community" {
		t.Fatalf("expected disabled registry to be community, got %s", disabled)
	}

	if err := runPluginRegistryDisable(cmd, []string{manager.OfficialRegistryName}); err == nil {
		t.Fatalf("expected error when disabling official registry")
	}
}

func TestRunPluginSearchFormatsOutput(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			search: func(ctx context.Context, query string) ([]manager.PluginInfo, error) {
				return []manager.PluginInfo{
					{Name: "search-plugin", Version: "1.0.0", Description: "desc", Category: "vcs", Author: "team"},
				}, nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := capturePluginStdout(func() {
		if err := runPluginSearch(cmd, []string{"query"}); err != nil {
			t.Fatalf("runPluginSearch error: %v", err)
		}
	})
	if !strings.Contains(out, "Found 1 plugin(s)") {
		t.Fatalf("unexpected search output: %s", out)
	}
}

func TestRunPluginSearchNoResults(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			search: func(ctx context.Context, query string) ([]manager.PluginInfo, error) {
				return nil, nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := capturePluginStdout(func() {
		if err := runPluginSearch(cmd, []string{"none"}); err != nil {
			t.Fatalf("runPluginSearch error: %v", err)
		}
	})
	if !strings.Contains(out, "No plugins found") {
		t.Fatalf("unexpected search output: %s", out)
	}
}

func TestDisplayInstalledPlugins_ShowsPlugins(t *testing.T) {
	entries := []manager.PluginListEntry{
		{
			Info:      manager.PluginInfo{Name: "repo", Description: "desc"},
			Installed: &manager.InstalledPlugin{Name: "repo"},
			Status:    manager.StatusEnabled,
		},
	}

	out := capturePluginStdout(func() {
		displayInstalledPlugins(entries)
	})
	if !strings.Contains(out, "Installed Plugins") {
		t.Fatalf("expected installed header, got %q", out)
	}
	if !strings.Contains(out, "repo") {
		t.Fatalf("expected plugin name, got %q", out)
	}
}

func TestRunPluginUninstall_CallsManager(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	pluginName := ""
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			uninstall: func(ctx context.Context, name string) error {
				pluginName = name
				return nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPluginUninstall(cmd, []string{"git"}); err != nil {
		t.Fatalf("runPluginUninstall error: %v", err)
	}
	if pluginName != "git" {
		t.Fatalf("expected plugin git, got %s", pluginName)
	}
}

func TestRunPluginInfoDisplaysInstallation(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	called := false
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			getInfo: func(ctx context.Context, name string) (*manager.PluginListEntry, error) {
				called = true
				return &manager.PluginListEntry{
					Info: manager.PluginInfo{
						Name:     name,
						Category: "notification",
						Version:  "1.0",
					},
					Installed: &manager.InstalledPlugin{
						Name:        name,
						Version:     "1.0",
						Enabled:     true,
						InstalledAt: time.Now(),
					},
					Status: manager.StatusEnabled,
				}, nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	out := capturePluginStdout(func() {
		if err := runPluginInfo(cmd, []string{"github"}); err != nil {
			t.Fatalf("runPluginInfo error: %v", err)
		}
	})
	if !called || !strings.Contains(out, "Plugin: github") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunPluginUpdateHandlesNotInstalled(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			getInfo: func(ctx context.Context, name string) (*manager.PluginListEntry, error) {
				return &manager.PluginListEntry{
					Info:   manager.PluginInfo{Name: name},
					Status: manager.StatusInstalled,
				}, nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runPluginUpdate(cmd, []string{"github"})
	if err == nil || !strings.Contains(err.Error(), "is not installed") {
		t.Fatalf("expected not installed error, got %v", err)
	}
}

func TestRunPluginConfigure_NotInstalled(t *testing.T) {
	orig := newPluginManager
	defer func() { newPluginManager = orig }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			getInfo: func(ctx context.Context, name string) (*manager.PluginListEntry, error) {
				return &manager.PluginListEntry{
					Info: manager.PluginInfo{Name: name},
				}, nil
			},
		}, nil
	}

	output := capturePluginStdout(func() {
		if err := runPluginConfigure(&cobra.Command{}, []string{"github"}); err != nil {
			t.Fatalf("runPluginConfigure error: %v", err)
		}
	})

	if !strings.Contains(output, "not installed") {
		t.Fatalf("expected not installed message, got: %s", output)
	}
}

func TestRunPluginConfigure_Disabled(t *testing.T) {
	orig := newPluginManager
	defer func() { newPluginManager = orig }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			getInfo: func(ctx context.Context, name string) (*manager.PluginListEntry, error) {
				return &manager.PluginListEntry{
					Info: manager.PluginInfo{Name: name},
					Installed: &manager.InstalledPlugin{
						Enabled: false,
					},
				}, nil
			},
		}, nil
	}

	output := capturePluginStdout(func() {
		if err := runPluginConfigure(&cobra.Command{}, []string{"slack"}); err != nil {
			t.Fatalf("runPluginConfigure error: %v", err)
		}
	})

	if !strings.Contains(output, "not enabled") {
		t.Fatalf("expected not enabled message, got: %s", output)
	}
}

func TestRunPluginConfigure_ShowsConfig(t *testing.T) {
	orig := newPluginManager
	defer func() { newPluginManager = orig }()

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			getInfo: func(ctx context.Context, name string) (*manager.PluginListEntry, error) {
				return &manager.PluginListEntry{
					Info: manager.PluginInfo{
						Name: name,
						ConfigSchema: map[string]manager.ConfigField{
							"token": {
								Required:    true,
								Description: "API token",
								Env:         "TOKEN",
							},
							"timeout": {
								Required:    false,
								Default:     10,
								Description: "Timeout",
							},
						},
						Hooks: []plugin.Hook{plugin.HookPostPublish},
					},
					Installed: &manager.InstalledPlugin{
						Enabled: true,
					},
				}, nil
			},
		}, nil
	}

	output := capturePluginStdout(func() {
		if err := runPluginConfigure(&cobra.Command{}, []string{"example"}); err != nil {
			t.Fatalf("runPluginConfigure error: %v", err)
		}
	})

	if !strings.Contains(output, "Configuration Guide") {
		t.Fatalf("expected configuration guide output, got: %s", output)
	}
}

func TestRunPluginUpdateUpdatesInstalledPlugin(t *testing.T) {
	origManager := newPluginManager
	defer func() { newPluginManager = origManager }()

	uninstalled := false
	installed := false
	enabled := false

	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			getInfo: func(ctx context.Context, name string) (*manager.PluginListEntry, error) {
				return &manager.PluginListEntry{
					Info: manager.PluginInfo{Name: name, Version: "2.0.0"},
					Installed: &manager.InstalledPlugin{
						Name:    name,
						Version: "1.0.0",
						Enabled: true,
					},
					Status: manager.StatusUpdateAvailable,
				}, nil
			},
			uninstall: func(ctx context.Context, name string) error {
				uninstalled = true
				return nil
			},
			install: func(ctx context.Context, name string) error {
				installed = true
				return nil
			},
			enable: func(ctx context.Context, name string) error {
				enabled = true
				return nil
			},
		}, nil
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runPluginUpdate(cmd, []string{"github"}); err != nil {
		t.Fatalf("runPluginUpdate error: %v", err)
	}
	if !uninstalled || !installed || !enabled {
		t.Fatalf("expected update flow to call uninstall/install/enable: %v %v %v", uninstalled, installed, enabled)
	}
}

func TestRunPluginListNoInstalledPlugins(t *testing.T) {
	orig := newPluginManager
	origAvailable := pluginListAvailable
	defer func() {
		newPluginManager = orig
		pluginListAvailable = origAvailable
	}()

	pluginListAvailable = false
	newPluginManager = func() (pluginManager, error) {
		return &stubPluginManager{
			listInstalled: func(ctx context.Context) ([]manager.PluginListEntry, error) {
				return nil, nil
			},
		}, nil
	}

	output := capturePluginStdout(func() {
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		if err := runPluginList(cmd, nil); err != nil {
			t.Fatalf("runPluginList error: %v", err)
		}
	})

	if !strings.Contains(output, "No plugins installed") {
		t.Fatalf("expected empty installed output, got: %s", output)
	}
}
