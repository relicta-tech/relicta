package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v5/internal/plugin/sandbox"
)

// pluginSandboxStatusCmd surfaces the sandbox security posture so operators
// can inspect what the runtime actually enforces (vs. what marketing claims).
//
// Output formats:
//   - default: human-readable banner from sandbox.SecurityNotice()
//   - --json:  structured posture for CI / monitoring ingestion
var pluginSandboxStatusCmd = &cobra.Command{
	Use:   "sandbox-status",
	Short: "Show plugin sandbox enforcement posture",
	Long: `Show the current plugin sandbox enforcement posture for this platform.

Displays memory and CPU enforcement limits, plugin signature status, and
platform-specific caveats (e.g. Apple Silicon RLIMIT_AS limitations). Plugin
signatures are not currently verified on any platform.

Use this before relying on plugin sandboxing as a security boundary.
Loading any enabled plugin requires --allow-untrusted-plugins. The flag records
an operator trust decision; it does not verify the publisher or provide a
complete security sandbox.`,
	RunE: runPluginSandboxStatus,
}

func init() {
	pluginCmd.AddCommand(pluginSandboxStatusCmd)
}

func runPluginSandboxStatus(_ *cobra.Command, _ []string) error {
	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sandbox.CurrentPosture())
	}
	fmt.Println(sandbox.SecurityNotice())
	return nil
}
