// Package main implements the Go Modules plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&GoModPlugin{})
}
