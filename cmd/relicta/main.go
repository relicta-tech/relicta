// Package main is the entry point for the relicta CLI binary.
package main

import (
	"os"
	"runtime/debug"

	"github.com/relicta-tech/relicta/v5/internal/cli"
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
	// `go install module/path/cmd@version` does not apply GoReleaser's linker
	// flags. Prefer the version recorded in the module build metadata when the
	// binary was installed from a tagged module; local source builds remain dev.
	if ver == "" || ver == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			ver = info.Main.Version
		}
	}
	cli.SetVersionInfo(ver, commit, date)
	if err := cli.Execute(); err != nil {
		// cobra runs with SilenceErrors; surface the error ourselves so a
		// non-zero exit is never silent.
		cli.ReportError(err)
		os.Exit(1)
	}
}
