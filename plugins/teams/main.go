// Package main implements the Microsoft Teams plugin for ReleasePilot.
package main

import (
	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func main() {
	plugin.Serve(&TeamsPlugin{})
}
