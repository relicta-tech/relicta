// Package main implements the Homebrew formula publishing plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&HomebrewPlugin{})
}
