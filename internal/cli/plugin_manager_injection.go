package cli

import (
	"context"

	"github.com/relicta-tech/relicta/v4/internal/plugin/manager"
)

type pluginManager interface {
	ListAvailable(context.Context, bool) ([]manager.PluginListEntry, error)
	ListInstalled(context.Context) ([]manager.PluginListEntry, error)
	Install(context.Context, string) error
	InstallRequired(context.Context, []string) ([]manager.RequiredResult, error)
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
	mgr, err := manager.NewManager()
	if err != nil {
		return nil, err
	}
	// Apply the configured plugin trust policy (ADR-008). Defaults to
	// permissive when unset so the legacy unsigned registry keeps working;
	// operators opt into signature enforcement via plugin_security.trust_policy.
	if cfg != nil {
		policy := cfg.PluginSecurity.TrustPolicy
		if policy == "" {
			policy = string(manager.TrustPermissive)
		}
		if err := mgr.SetTrustPolicy(policy, cfg.PluginSecurity.TrustKey); err != nil {
			return nil, err
		}
	}
	return mgr, nil
}
