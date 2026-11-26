//go:build embed_frontend

package main

import (
	"embed"
	"io/fs"

	"github.com/relicta-tech/relicta/v5/internal/cli"
)

// The Makefile's frontend target copies the production dashboard build here
// before compiling with the embed_frontend build tag.
//
//go:embed frontend
var frontendFiles embed.FS

func init() {
	frontend, err := fs.Sub(frontendFiles, "frontend")
	if err != nil {
		panic("embedded dashboard frontend is missing: " + err.Error())
	}
	cli.SetEmbeddedFrontend(frontend)
}
