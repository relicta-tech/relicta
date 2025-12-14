// Package main provides the entry point for the GitLab plugin.
package main

import (
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func main() {
	plugin.Serve(&GitLabPlugin{})
}
