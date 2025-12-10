// Package plugin provides the public interface for ReleasePilot plugins.
package plugin

import (
	"os"

	"github.com/hashicorp/go-plugin"
)

const (
	// PluginName is the name used for the plugin map.
	PluginName = "release-pilot-plugin"

	// MagicCookieKey is the key for the plugin handshake.
	MagicCookieKey = "RELEASE_PILOT_PLUGIN"

	// MagicCookieValue is the value for the plugin handshake.
	MagicCookieValue = "release-pilot-v1"
)

// Handshake is the handshake config for plugins.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   MagicCookieKey,
	MagicCookieValue: MagicCookieValue,
}

// PluginMap is the map of plugin implementations.
var PluginMap = map[string]plugin.Plugin{
	PluginName: &GRPCPlugin{},
}

// Serve starts the plugin server with the given plugin implementation.
// This should be called from the plugin's main function.
func Serve(impl Plugin) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			PluginName: &GRPCPlugin{Impl: impl},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

// ServeTest starts the plugin server in test mode.
// This returns a reattach config for the test harness.
func ServeTest(impl Plugin) (*plugin.ReattachConfig, func()) {
	reattach := make(chan *plugin.ReattachConfig, 1)
	closeCh := make(chan struct{})

	go func() {
		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: Handshake,
			Plugins: map[string]plugin.Plugin{
				PluginName: &GRPCPlugin{Impl: impl},
			},
			GRPCServer: plugin.DefaultGRPCServer,
			Test: &plugin.ServeTestConfig{
				ReattachConfigCh: reattach,
				CloseCh:          closeCh,
			},
		})
	}()

	return <-reattach, func() { close(closeCh) }
}

// IsPlugin returns true if the current process is running as a plugin.
func IsPlugin() bool {
	return os.Getenv(MagicCookieKey) == MagicCookieValue
}
