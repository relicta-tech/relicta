package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v5/internal/container"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/webhook"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/webhook/queue"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Inspect and operate the durable webhook delivery queue",
}

var webhookQueueCmd = &cobra.Command{
	Use:   "queue",
	Short: "List pending, in-flight, and dead-lettered webhook deliveries",
	Args:  cobra.NoArgs,
	RunE:  runWebhookQueue,
}

var webhookReplayCmd = &cobra.Command{
	Use:   "replay <delivery-id>",
	Short: "Retry a dead-lettered delivery",
	Args:  cobra.ExactArgs(1),
	RunE:  runWebhookReplay,
}

type webhookQueueRow struct {
	ID            string               `json:"id"`
	Webhook       string               `json:"webhook"`
	Event         string               `json:"event"`
	Status        queue.DeliveryStatus `json:"status"`
	Attempts      int                  `json:"attempts"`
	MaxAttempts   int                  `json:"max_attempts"`
	NextAttemptAt string               `json:"next_attempt_at,omitempty"`
}

func init() {
	webhookCmd.AddCommand(webhookQueueCmd, webhookReplayCmd)
	webhookCmd.GroupID = "ops"
}

func openWebhookQueue(cmd *cobra.Command) (*queue.FileQueue, error) {
	return container.OpenWebhookQueue(cmd.Context(), cfg)
}

func runWebhookQueue(cmd *cobra.Command, _ []string) error {
	q, err := openWebhookQueue(cmd)
	if err != nil {
		return err
	}
	pending, err := q.ListPending("")
	if err != nil {
		return err
	}
	inFlight, err := q.ListInFlight("")
	if err != nil {
		return err
	}
	dead, err := q.ListDeadLetter("")
	if err != nil {
		return err
	}
	rows := make([]webhookQueueRow, 0, len(pending)+len(inFlight)+len(dead))
	for _, group := range [][]*queue.Delivery{pending, inFlight, dead} {
		for _, d := range group {
			next := ""
			if !d.NextAttemptAt.IsZero() {
				next = d.NextAttemptAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			rows = append(rows, webhookQueueRow{
				ID: d.ID, Webhook: d.EndpointName, Event: d.Event, Status: d.Status,
				Attempts: d.Attempts, MaxAttempts: d.MaxAttempts, NextAttemptAt: next,
			})
		}
	}
	if outputJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No pending or dead-lettered webhook deliveries.")
		return err
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tWEBHOOK\tEVENT\tSTATUS\tATTEMPTS\tNEXT ATTEMPT")
	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d/%d\t%s\n",
			row.ID, row.Webhook, row.Event, row.Status, row.Attempts, row.MaxAttempts, row.NextAttemptAt)
	}
	return w.Flush()
}

func runWebhookReplay(cmd *cobra.Command, args []string) error {
	q, err := openWebhookQueue(cmd)
	if err != nil {
		return err
	}
	d, err := q.Get(args[0])
	if err != nil {
		return err
	}
	if d.Status != queue.StatusDeadLetter {
		return fmt.Errorf("delivery %s has status %q; only dead-lettered deliveries can be replayed", d.ID, d.Status)
	}
	configured := false
	if cfg != nil {
		for i := range cfg.Webhooks {
			wh := &cfg.Webhooks[i]
			if wh.URL == d.EndpointURL && wh.Name == d.EndpointName && wh.IsWebhookEnabled() {
				configured = true
				break
			}
		}
	}
	if !configured {
		return fmt.Errorf("webhook %q is not currently configured and enabled; restore its configuration before replay", d.EndpointName)
	}
	if _, err := q.Requeue(d.ID); err != nil {
		return err
	}
	publisher := webhook.NewPersistentPublisher(cfg.Webhooks, nil, q)
	publisher.Start(cmd.Context())
	if err := publisher.Close(); err != nil {
		return fmt.Errorf("replay delivery %s: %w", d.ID, err)
	}
	updated, err := q.Get(d.ID)
	if err != nil {
		return err
	}
	if outputJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(webhookQueueRow{
			ID: updated.ID, Webhook: updated.EndpointName, Event: updated.Event,
			Status: updated.Status, Attempts: updated.Attempts, MaxAttempts: updated.MaxAttempts,
		})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Delivery %s for %s (%s): %s after %d/%d attempts.\n",
		updated.ID, updated.EndpointName, updated.Event, updated.Status, updated.Attempts, updated.MaxAttempts)
	return err
}
