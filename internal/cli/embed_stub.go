package cli

import "io/fs"

var embeddedFrontend fs.FS // nil when not embedding frontend

// SetEmbeddedFrontend sets the embedded frontend filesystem.
// This is called by the embed.go file when built with the embed_frontend tag.
func SetEmbeddedFrontend(frontend fs.FS) {
	embeddedFrontend = frontend
}
