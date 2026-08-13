package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/hubclient"
)

// `relicta hub sync` — the write side of the Hub relationship.
//
// Hub could authenticate a CLI, store events, aggregate releases, compute actor reputation and
// render a dashboard over all of it. Nothing ever sent it anything: /api/v1/sync carried the
// comment "CLI pushes governance events" and had no client, so every reader worked correctly
// over an empty database. This is the push.

var (
	hubSyncLimit  int
	hubSyncDryRun bool
)

var hubSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Push this repository's governance record to Hub",
	Long: `Send local release history to a Relicta Hub.

Reads the governance record this repository already keeps — the same store behind ` + "`relicta history`" + ` —
and pushes it to Hub, which aggregates it into the org-wide release view, DORA metrics and actor
reputation.

Safe to run repeatedly. Event identifiers are derived from the release records rather than
generated, and Hub ignores an identifier it has already stored, so syncing twice converges on the
same rows instead of duplicating them.

Requires a token from ` + "`relicta hub login`" + `.

Examples:
  relicta hub sync
  relicta hub sync --dry-run
  relicta hub sync --limit 50`,
	RunE: runHubSync,
}

func init() {
	hubSyncCmd.Flags().IntVar(&hubSyncLimit, "limit", 200,
		"Maximum number of release records to send")
	hubSyncCmd.Flags().BoolVar(&hubSyncDryRun, "dry-run", false,
		"Show what would be sent without sending it")
	hubCmd.AddCommand(hubSyncCmd)
}

func runHubSync(cmd *cobra.Command, _ []string) error {
	token, err := hubclient.LoadToken()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not authenticated: run `relicta hub login` first")
		}
		return err
	}
	if token.Expired(time.Now()) {
		// Refused before doing any work rather than after: an expired token produces a 401 from
		// Hub that reads like a permissions problem, and the fix is the same either way.
		return fmt.Errorf("the stored Hub token has expired — run `relicta hub login` again")
	}

	// The Hub the token was issued for, unless overridden. A token is only valid for its own
	// Hub, so following the token rather than a flag default is what keeps the credential and
	// the destination from drifting apart.
	hubURL := token.HubURL
	if override, err := resolveHubURLOptional(); err != nil {
		return err
	} else if override != "" && override != hubURL {
		return fmt.Errorf("the stored token was issued by %s, not %s — run `relicta hub login` against %s first",
			token.HubURL, override, override)
	}
	if hubURL == "" {
		return fmt.Errorf("the stored token does not record which Hub issued it — run `relicta hub login` again")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	store, err := getMemoryStoreCtx(ctx)
	if err != nil {
		return fmt.Errorf("failed to open the governance store: %w", err)
	}

	// The governance identity, the same key the local readers use — so a repository is one
	// repository on both sides rather than two records that never join.
	repository := getRepositoryName(ctx)
	if repository == "" {
		return fmt.Errorf("could not identify this repository: run inside a git repository with a remote")
	}

	records, err := store.GetReleaseHistory(ctx, repository, hubSyncLimit)
	if err != nil {
		return fmt.Errorf("failed to read the governance store: %w", err)
	}
	if len(records) == 0 {
		printWarning(fmt.Sprintf("no governance history for %s — run a release first", repository))
		return nil
	}

	events := hubclient.EventsFromReleases(token.OrgID, records)

	if hubSyncDryRun {
		fmt.Printf("Would send %d event(s) from %s to %s:\n\n", len(events), repository, hubURL)
		for _, e := range events {
			fmt.Printf("  %-24s %-18s %s\n", e.Type, e.Data["version"], e.Timestamp.Format(time.RFC3339))
		}
		return nil
	}

	result, err := hubclient.New(hubURL).SyncEvents(ctx, token.Token, events)
	if err != nil {
		return err
	}

	// Rejections are printed individually. "8 of 12 accepted" tells an operator nothing they can
	// act on, and Hub already explains each one.
	if rejected := result.Rejected(); len(rejected) > 0 {
		for _, r := range rejected {
			printWarning(r)
		}
	}

	printSuccess(fmt.Sprintf("Synced %d of %d event(s) from %s to %s",
		result.Accepted, result.Received, repository, hubURL))
	return nil
}

// resolveHubURLOptional returns an explicitly configured Hub URL, or empty when none is set.
//
// Separate from resolveHubURL because sync has a better default than an error: the Hub recorded
// in the token. Reusing the login resolver would make --hub mandatory for a command that already
// knows where to go.
func resolveHubURLOptional() (string, error) {
	if hubURLFlag == "" && os.Getenv("RELICTA_HUB_URL") == "" {
		return "", nil
	}
	return resolveHubURL()
}
