package handlers

import (
	"github.com/relicta-tech/relicta/internal/domain/release"
)

// Context holds dependencies for HTTP handlers.
// This enables dependency injection for handlers.
type Context struct {
	// ReleaseServices provides access to release domain use cases.
	ReleaseServices *release.Services
}

// DefaultContext is the global handler context.
// It is set by the server during initialization.
var DefaultContext *Context

// SetContext sets the default handler context.
func SetContext(ctx *Context) {
	DefaultContext = ctx
}

// GetContext returns the default handler context.
// Returns nil if not initialized.
func GetContext() *Context {
	return DefaultContext
}
