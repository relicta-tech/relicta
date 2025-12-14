// Package main implements the Microsoft Teams plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&TeamsPlugin{})
}
