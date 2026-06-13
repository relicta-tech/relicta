// Package main is the entry point for the relicta CLI binary.
package main

import (
	"os"

	"github.com/relicta-tech/relicta/v4/internal/cli"
)

// Populated at build time via -ldflags "-X main.ver=... -X main.commit=... -X main.date=...".
// Without these declarations the linker silently drops the -X flags and
// `relicta version` prints an empty version (issue #135).
var (
	ver    = "dev"
	commit = "none"
	date   = "unknown"
)

func main() {
	cli.SetVersionInfo(ver, commit, date)
	if err := cli.Execute(); err != nil {
		// cobra runs with SilenceErrors; surface the error ourselves so a
		// non-zero exit is never silent.
		cli.ReportError(err)
		os.Exit(1)
	}
}
