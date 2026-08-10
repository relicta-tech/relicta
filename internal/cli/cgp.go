package cli

// cgp.go: read-only access to the CGP protocol records an agent leaves behind.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/protocol"
)

// The three cgp_* MCP tools record a governance handshake — a proposal, the
// decision it received, and the authorization to execute it. Those records became
// durable so that a decision made by an agent leaves evidence behind.
//
// Evidence only an agent can retrieve is not evidence a person can audit. The
// records were reachable exclusively over MCP, and only by already knowing a
// proposal's ID, which is precisely what someone investigating a release does not
// have. These commands are the reading surface: what did the agents propose here,
// and what was decided.
//
// Read-only on purpose. Proposing and authorizing belong to whoever is making the
// change; a person auditing after the fact should not be able to alter the record
// from the same command they use to read it.

var cgpCmd = &cobra.Command{
	Use:   "cgp",
	Short: "Inspect Change Governance Protocol records",
	Long: `Inspect the CGP records left by AI agents and other protocol clients.

The cgp_propose, cgp_authorize and cgp_status MCP tools record a governance
handshake for each proposed change. These commands read those records, so a
person can audit what was proposed and what was decided without going through
an agent.

Examples:
  # What has been proposed in this repository?
  relicta cgp list

  # The full handshake for one proposal
  relicta cgp status prop_d419e938-a1b`,
}

var cgpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded CGP proposals, newest first",
	RunE:  runCGPList,
}

var cgpStatusCmd = &cobra.Command{
	Use:   "status <proposal-id>",
	Short: "Show the recorded governance state of a proposal",
	Args:  cobra.ExactArgs(1),
	RunE:  runCGPStatus,
}

func init() {
	cgpCmd.AddCommand(cgpListCmd)
	cgpCmd.AddCommand(cgpStatusCmd)
}

// cgpStore opens the protocol store for the current repository.
//
// Anchored to the repository root, not the working directory: the records live
// beside the release runs, and addressing them by cwd would report an empty
// history when run from a subdirectory — the defect that made `relicta cancel`
// claim no release existed (#246).
func cgpStore(ctx context.Context) (*protocol.FileProposalStore, error) {
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}
	defer closeApp(app)

	repoInfo, err := app.GitAdapter().GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the repository: %w", err)
	}
	return protocol.NewFileProposalStore(repoInfo.Path), nil
}

func runCGPList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	store, err := cgpStore(ctx)
	if err != nil {
		return err
	}

	ids, err := store.ListProposals(ctx)
	if err != nil {
		return fmt.Errorf("failed to read CGP records: %w", err)
	}

	type row struct {
		ProposalID string  `json:"proposal_id"`
		State      string  `json:"state"`
		Actor      string  `json:"actor"`
		Summary    string  `json:"summary"`
		Decision   string  `json:"decision,omitempty"`
		RiskScore  float64 `json:"risk_score,omitempty"`
	}

	rows := make([]row, 0, len(ids))
	for _, id := range ids {
		proposal, proposalErr := store.GetProposal(ctx, id)
		if proposalErr != nil {
			// A record that cannot be read is reported, not skipped. Silently
			// dropping it would present a partial history as a complete one.
			printWarning(fmt.Sprintf("could not read proposal %s: %v", id, proposalErr))
			continue
		}

		r := row{
			ProposalID: id,
			State:      "proposed",
			Actor:      fmt.Sprintf("%s:%s", proposal.Actor.Kind, proposal.Actor.ID),
			Summary:    proposal.Intent.Summary,
		}
		if decision, decisionErr := store.GetDecision(ctx, id); decisionErr == nil {
			r.State = "decided"
			r.Decision = decision.Decision
			r.RiskScore = decision.RiskScore
		}
		if _, authErr := store.GetAuthorization(ctx, id); authErr == nil {
			r.State = "authorized"
		}
		rows = append(rows, r)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"proposals": rows,
			"total":     len(rows),
		})
	}

	if len(rows) == 0 {
		fmt.Println("No CGP proposals recorded in this repository.")
		fmt.Println()
		fmt.Println("Records appear here when an agent uses the cgp_propose MCP tool.")
		fmt.Println("Start the server with: relicta mcp serve")
		return nil
	}

	printTitle(fmt.Sprintf("CGP Proposals (%d)", len(rows)))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  PROPOSAL\tSTATE\tRISK\tACTOR\tSUMMARY")
	for _, r := range rows {
		risk := "-"
		if r.Decision != "" {
			risk = fmt.Sprintf("%.0f%%", r.RiskScore*100)
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", r.ProposalID, r.State, risk, r.Actor, r.Summary)
	}
	_ = w.Flush()
	fmt.Println()

	return nil
}

func runCGPStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	proposalID := args[0]

	store, err := cgpStore(ctx)
	if err != nil {
		return err
	}

	proposal, err := store.GetProposal(ctx, proposalID)
	if err != nil {
		// The hint is part of the error rather than printed before it. Printing it
		// first put the suggestion above the problem it responds to, and split the
		// two across stdout and stderr so a redirected run showed one without the
		// other. "Not found" and "you are in the wrong repository" look identical
		// from here, so the hint has to travel with the message.
		return fmt.Errorf("%w — run 'relicta cgp list' to see the proposals recorded "+
			"in this repository", err)
	}

	decision, decisionErr := store.GetDecision(ctx, proposalID)
	auth, authErr := store.GetAuthorization(ctx, proposalID)

	state := "proposed"
	switch {
	case authErr == nil:
		state = "authorized"
	case decisionErr == nil:
		state = "decided"
	}

	if outputJSON {
		out := map[string]any{
			"proposal_id": proposalID,
			"state":       state,
			"proposal":    proposal,
		}
		if decisionErr == nil {
			out["decision"] = decision
		}
		if authErr == nil {
			out["authorization"] = auth
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	printTitle(fmt.Sprintf("Proposal %s", proposalID))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  State:\t%s\n", state)
	fmt.Fprintf(w, "  Actor:\t%s:%s\n", proposal.Actor.Kind, proposal.Actor.ID)
	fmt.Fprintf(w, "  Repository:\t%s\n", proposal.Scope.Repository)
	if proposal.Scope.CommitRange != "" {
		fmt.Fprintf(w, "  Commits:\t%s\n", proposal.Scope.CommitRange)
	}
	fmt.Fprintf(w, "  Summary:\t%s\n", proposal.Intent.Summary)
	fmt.Fprintf(w, "  Proposed at:\t%s\n", proposal.Timestamp.Format("2006-01-02 15:04:05 MST"))
	_ = w.Flush()
	fmt.Println()

	if decisionErr != nil {
		// Said explicitly. A proposal with no decision is a change that was offered
		// and never governed, which is a finding rather than an absence.
		printWarning("No decision recorded for this proposal — it was proposed but never evaluated.")
		return nil
	}

	printTitle("Decision")
	fmt.Println()
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Decision:\t%s\n", decision.Decision)
	fmt.Fprintf(w, "  Risk score:\t%.1f%%\n", decision.RiskScore*100)
	fmt.Fprintf(w, "  Decided at:\t%s\n", decision.Timestamp.Format("2006-01-02 15:04:05 MST"))
	_ = w.Flush()

	if len(decision.Rationale) > 0 {
		fmt.Println()
		fmt.Println("  Rationale:")
		for _, reason := range decision.Rationale {
			fmt.Printf("    - %s\n", reason)
		}
	}
	if len(decision.RequiredActions) > 0 {
		fmt.Println()
		fmt.Println("  Required actions:")
		for _, action := range decision.RequiredActions {
			fmt.Printf("    - %s\n", action.Description)
		}
	}
	fmt.Println()

	if authErr != nil {
		printInfo("Not authorized for execution.")
		return nil
	}

	printTitle("Authorization")
	fmt.Println()
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Authorized at:\t%s\n", auth.Timestamp.Format("2006-01-02 15:04:05 MST"))
	if auth.Version != "" {
		fmt.Fprintf(w, "  Version:\t%s\n", auth.Version)
	}
	fmt.Fprintf(w, "  Decision:\t%s\n", auth.DecisionID)
	_ = w.Flush()
	fmt.Println()

	return nil
}
