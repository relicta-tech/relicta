// Package main implements the Docker Hub / Container registry plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&DockerPlugin{})
}
