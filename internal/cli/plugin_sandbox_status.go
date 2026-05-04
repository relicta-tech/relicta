package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/plugin/sandbox"
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

Displays whether memory / CPU limits are reliably enforced, whether plugin
signatures are verified before load, and any platform-specific caveats
(e.g. Apple Silicon RLIMIT_AS limitations).

Use this before relying on plugin sandboxing as a security boundary.
Plugin loading on best-effort platforms requires --allow-untrusted-plugins.`,
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
