// Package main implements the RubyGems plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&RubyGemsPlugin{})
}
