// Command gendocs generates the Markdown CLI reference for relicta under
// docs/cli/ by walking the cobra command tree. Run via `make docs-cli` (or
// `go run ./tools/gendocs`) and commit the result so the reference stays in
// sync with the actual flags and commands.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/relicta-tech/relicta/v4/internal/cli"
)

func main() {
	outDir := "docs/cli"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("create %s: %v", outDir, err)
	}

	root := cli.RootCommand()
	// Drop the per-file "Auto generated ... on <date>" footer so regenerating
	// produces a stable diff (no spurious timestamp churn).
	disableAutoGenTag(root)

	if err := doc.GenMarkdownTree(root, outDir); err != nil {
		log.Fatalf("generate CLI docs: %v", err)
	}
	fmt.Printf("Wrote CLI reference to %s\n", filepath.Clean(outDir))
}

func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, c := range cmd.Commands() {
		disableAutoGenTag(c)
	}
}
