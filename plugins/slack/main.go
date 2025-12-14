// Package main implements the Slack plugin for Relicta.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&SlackPlugin{})
}
