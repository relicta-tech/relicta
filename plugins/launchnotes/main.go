// Package main implements the LaunchNotes plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&LaunchNotesPlugin{})
}
