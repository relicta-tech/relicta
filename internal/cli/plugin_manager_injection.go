package cli

import (
	"context"

	"github.com/relicta-tech/relicta/internal/plugin/manager"
)

type pluginManager interface {
	ListAvailable(context.Context, bool) ([]manager.PluginListEntry, error)
	ListInstalled(context.Context) ([]manager.PluginListEntry, error)
	Install(context.Context, string) error
	Uninstall(context.Context, string) error
	Enable(context.Context, string) error
	Disable(context.Context, string) error
	GetPluginInfo(context.Context, string) (*manager.PluginListEntry, error)
	ListRegistries() []manager.RegistryEntry
	AddRegistry(name, url string, priority int) error
	RemoveRegistry(name string) error
	EnableRegistry(name string, enabled bool) error
	Search(context.Context, string) ([]manager.PluginInfo, error)
}

var newPluginManager = func() (pluginManager, error) {
	return manager.NewManager()
}
