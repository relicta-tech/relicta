// Package integration provides domain types for plugin integration.
package integration

import "errors"

// Domain errors for plugin integration.
var (
	// ErrInvalidPlugin indicates an invalid plugin.
	ErrInvalidPlugin = errors.New("invalid plugin")

	// ErrPluginNotFound indicates the plugin was not found.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrPluginAlreadyRegistered indicates the plugin is already registered.
	ErrPluginAlreadyRegistered = errors.New("plugin already registered")

	// ErrPluginLoadFailed indicates plugin loading failed.
	ErrPluginLoadFailed = errors.New("plugin load failed")

	// ErrPluginExecutionFailed indicates plugin execution failed.
	ErrPluginExecutionFailed = errors.New("plugin execution failed")

	// ErrPluginTimeout indicates the plugin timed out.
	ErrPluginTimeout = errors.New("plugin timed out")

	// ErrPluginConfigInvalid indicates invalid plugin configuration.
	ErrPluginConfigInvalid = errors.New("plugin configuration invalid")

	// ErrHookNotSupported indicates the hook is not supported by the plugin.
	ErrHookNotSupported = errors.New("hook not supported by plugin")

	// ErrPluginDisabled indicates the plugin is disabled.
	ErrPluginDisabled = errors.New("plugin is disabled")

	// ErrPluginVersionMismatch indicates a version mismatch.
	ErrPluginVersionMismatch = errors.New("plugin version mismatch")
)
