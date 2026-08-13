package cli

// audit.go: one view over both governance records.

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// Governance activity is recorded in two places, and correctly so: a CGP proposal is not a
// release run. A proposal carries an actor, a scope and an intent; a run carries a version, a
// changeset and a state machine. Mapping one onto the other would force each to hold fields
// it has no meaning for.
//
// The cost fell on the reader. Someone asking "what governed this change?" had to run
// `relicta cgp list` for what an agent proposed and `relicta history` for what shipped, and
// join the two by eye. This is that join, done once and in the right place — a read-side
// view, not a merge of the records.
//
// The join key is the version. An ExecutionAuthorization names the version a proposal was
// authorized to release, and a release record names the version that was released, so an
// authorized proposal and the run that carried it out meet there. A proposal that was never
// authorized has no version to join on and is shown on its own, which is the honest
// rendering: it is governance activity that did not become a release.

var auditVersion string

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Show the complete governance record: what was proposed, decided and shipped",
	Long: `Show one timeline over both governance records.

Relicta keeps two: the CGP protocol records that agents and other clients leave
behind (relicta cgp), and the release audit trail of runs that were planned and
published (relicta history). They describe different things, so they are stored
separately — but a reader asking what governed a particular change needs both.

Entries are joined on the version an authorization granted, so an agent's
proposal appears alongside the release that carried it out. Anything that did
not become a release is still listed, on its own.

Examples:
  # Everything, newest first
  relicta audit

  # What governed one version
  relicta audit --version 1.4.0

  # For a tool
  relicta audit --json`,
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().StringVar(&auditVersion, "version", "", "show only the record for one version")
	auditCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "maximum entries to show")
	rootCmd.AddCommand(auditCmd)
}

// auditEntry is one line of the joined timeline.
//
// Either side may be absent: a release with no protocol record (the ordinary case for a
// human-driven release), or a proposal that never reached a release.
type auditEntry struct {
	Version string     `json:"version,omitempty"`
	At      time.Time  `json:"at"`
	Release *auditSide `json:"release,omitempty"`
	Propose *auditSide `json:"proposal,omitempty"`
}

// auditSide is what one record contributes to an entry.
type auditSide struct {
	ID        string  `json:"id"`
	Actor     string  `json:"actor,omitempty"`
	State     string  `json:"state,omitempty"`
	Outcome   string  `json:"outcome,omitempty"`
	Decision  string  `json:"decision,omitempty"`
	RiskScore float64 `json:"risk_score,omitempty"`
}

func runAudit(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	repository := getRepositoryName(ctx)
	if repository == "" {
		return fmt.Errorf("could not identify this repository: run inside a git repository with a remote")
	}

	releases, err := auditReleases(ctx, repository)
	if err != nil {
		return err
	}

	proposals, err := auditProposals(ctx)
	if err != nil {
		return err
	}

	entries := joinGovernanceRecords(releases, proposals)
	if auditVersion != "" {
		entries = filterByVersion(entries, auditVersion)
	}
	if historyLimit > 0 && len(entries) > historyLimit {
		entries = entries[:historyLimit]
	}

	if outputJSON {
		return printJSONOutput(entries)
	}
	printAudit(repository, entries)
	return nil
}

// auditReleases reads the release trail through the same resolver history uses.
func auditReleases(ctx context.Context, repository string) ([]*memory.ReleaseRecord, error) {
	store, err := getMemoryStoreCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open the governance history store: %w", err)
	}

	// A generous window: this command exists to answer questions about the past, and the
	// display limit is applied after the join rather than before it — truncating the
	// inputs would drop the release half of a pair while keeping the proposal half.
	records, err := store.GetReleaseHistory(ctx, repository, 500)
	if err != nil {
		return nil, fmt.Errorf("failed to read release history: %w", err)
	}
	return records, nil
}

// auditProposal is a protocol record with the parts of its handshake that were reached.
type auditProposal struct {
	id      string
	actor   string
	state   string
	version string
	risk    float64
	decided string
	at      time.Time
}

// auditProposals reads the protocol records and resolves each one's handshake.
func auditProposals(ctx context.Context) ([]auditProposal, error) {
	store, err := cgpStore(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := store.ListProposals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list protocol records: %w", err)
	}

	out := make([]auditProposal, 0, len(ids))
	for _, id := range ids {
		proposal, proposalErr := store.GetProposal(ctx, id)
		if proposalErr != nil || proposal == nil {
			// A record that cannot be read is skipped rather than failing the command:
			// one damaged file must not hide the rest of the governance history.
			continue
		}

		entry := auditProposal{
			id:    id,
			actor: qualifiedActor(string(proposal.Actor.Kind), proposal.Actor.ID),
			state: "proposed",
			at:    proposal.Timestamp,
		}

		if decision, decisionErr := store.GetDecision(ctx, id); decisionErr == nil && decision != nil {
			entry.state = "decided"
			entry.decided = string(decision.Decision)
			entry.risk = decision.RiskScore
		}
		// The authorization is what carries the version, and the version is the join key:
		// an unauthorized proposal has nothing to join on, which is why it appears alone.
		if auth, authErr := store.GetAuthorization(ctx, id); authErr == nil && auth != nil {
			entry.state = "authorized"
			entry.version = auth.Version
		}

		out = append(out, entry)
	}
	return out, nil
}

// joinGovernanceRecords pairs releases and proposals on the authorized version.
func joinGovernanceRecords(releases []*memory.ReleaseRecord, proposals []auditProposal) []auditEntry {
	// Claimed by release ID, not by version.
	//
	// Keyed by version, pairing one release suppressed every other release carrying the
	// same version — and a repository routinely has several: a canceled 0.1.0 and the
	// published 0.1.0 that followed it are two records of the same version, and the
	// cancellation disappeared from the timeline the moment a proposal claimed the
	// release. Found by running the command on a repository that had both, not by reading
	// this loop.
	claimed := make(map[string]bool, len(releases))

	// The candidate for each version is the release that actually shipped it. A
	// cancellation carries a version too, and pairing a proposal's authorization with a
	// run that never happened would misreport what the authorization led to.
	byVersion := make(map[string]*memory.ReleaseRecord, len(releases))
	for _, r := range releases {
		if r == nil || r.Version == "" || !r.Outcome.CountsAsRelease() {
			continue
		}
		if _, seen := byVersion[r.Version]; !seen {
			byVersion[r.Version] = r
		}
	}

	entries := make([]auditEntry, 0, len(releases)+len(proposals))

	for _, p := range proposals {
		entry := auditEntry{
			Version: p.version,
			At:      p.at,
			Propose: &auditSide{
				ID:        p.id,
				Actor:     p.actor,
				State:     p.state,
				Decision:  p.decided,
				RiskScore: p.risk,
			},
		}
		if p.version != "" {
			if release, ok := byVersion[p.version]; ok && !claimed[release.ID] {
				claimed[release.ID] = true
				entry.Release = releaseSide(release)
				// The release is the later half, and dating the pair by it puts the
				// entry where a reader looks for "when did this ship".
				entry.At = release.ReleasedAt
			}
		}
		entries = append(entries, entry)
	}

	for _, r := range releases {
		if r == nil || claimed[r.ID] {
			continue
		}
		entries = append(entries, auditEntry{
			Version: r.Version,
			At:      r.ReleasedAt,
			Release: releaseSide(r),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].At.After(entries[j].At) })
	return entries
}

func releaseSide(r *memory.ReleaseRecord) *auditSide {
	return &auditSide{
		ID:        r.ID,
		Actor:     r.Actor.ID,
		Outcome:   string(r.Outcome),
		Decision:  string(r.Decision),
		RiskScore: r.RiskScore,
	}
}

func filterByVersion(entries []auditEntry, version string) []auditEntry {
	// A leading "v" is accepted because that is how the version appears in a tag, and a
	// reader copying one from `git tag` should not get an empty answer for it.
	want := strings.TrimPrefix(version, "v")

	out := make([]auditEntry, 0, len(entries))
	for _, e := range entries {
		if strings.TrimPrefix(e.Version, "v") == want {
			out = append(out, e)
		}
	}
	return out
}

func printAudit(repository string, entries []auditEntry) {
	if len(entries) == 0 {
		if auditVersion != "" {
			fmt.Printf("No governance record for version %s in %s.\n", auditVersion, repository)
			return
		}
		fmt.Printf("No governance record for %s.\n", repository)
		fmt.Println()
		fmt.Println("Releases appear here once one is published; protocol records appear when an")
		fmt.Println("agent uses the cgp_propose MCP tool.")
		return
	}

	printTitle(fmt.Sprintf("Governance Record for %s (%d)", repository, len(entries)))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  WHEN\tVERSION\tRELEASE\tPROPOSAL\tACTOR")
	for _, e := range entries {
		version := e.Version
		if version == "" {
			version = "-"
		}

		release, proposal, actor := "-", "-", "-"
		if e.Release != nil {
			release = fmt.Sprintf("%s %s", getOutcomeSymbol(memory.ReleaseOutcome(e.Release.Outcome)), e.Release.Outcome)
			actor = e.Release.Actor
		}
		if e.Propose != nil {
			proposal = e.Propose.State
			if e.Propose.Decision != "" {
				proposal += " (" + e.Propose.Decision + ")"
			}
			if actor == "-" {
				actor = e.Propose.Actor
			}
		}

		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
			e.At.Format("2006-01-02 15:04"), version, release, proposal, actor)
	}
	_ = w.Flush()
	fmt.Println()

	// Name what a dash means, so a reader does not read "no protocol record" as "the
	// record is missing" when it means the release was driven by a person.
	fmt.Println("A dash means that half of the record does not exist: a release with no")
	fmt.Println("proposal was driven directly, and a proposal with no release was never")
	fmt.Println("authorized or has not shipped yet.")
}
