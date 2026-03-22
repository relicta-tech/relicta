// Package main is the entry point for the relicta CLI binary.
package main

import (
	"os"

	"github.com/relicta-tech/relicta/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
