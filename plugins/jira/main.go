// Package main provides the entry point for the Jira plugin.
package main

import (
	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func main() {
	plugin.Serve(&JiraPlugin{})
}
