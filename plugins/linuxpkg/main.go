// Package main implements the Linux package repository plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&LinuxPkgPlugin{})
}
