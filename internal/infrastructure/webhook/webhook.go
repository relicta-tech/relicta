// Package webhook provides HTTP webhook notifications for release events.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

// getTimeout returns the configured timeout or default.
func getTimeout(c *config.WebhookConfig) time.Duration {
	if c.Timeout == 0 {
		return 10 * time.Second
	}
	return c.Timeout
}

// getRetryCount returns the configured retry count or default.
func getRetryCount(c *config.WebhookConfig) int {
	if c.RetryCount == 0 {
		return 3
	}
	return c.RetryCount
}

// getRetryDelay returns the configured retry delay or default.
func getRetryDelay(c *config.WebhookConfig) time.Duration {
	if c.RetryDelay == 0 {
		return 1 * time.Second
	}
	return c.RetryDelay
}

// WebhookPayload is the JSON payload sent to webhook endpoints.
type WebhookPayload struct {
	// Event is the event name (e.g., "release.published").
	Event string `json:"event"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// ReleaseID is the release aggregate ID.
	ReleaseID string `json:"release_id"`
	// Data contains event-specific data.
	Data map[string]any `json:"data"`
}

// Publisher implements release.EventPublisher and sends events to webhook endpoints.
type Publisher struct {
	webhooks []config.WebhookConfig
	client   *http.Client
	next     release.EventPublisher
	logger   *slog.Logger
}

// NewPublisher creates a new webhook publisher.
// The next parameter is optional - if nil, events are not forwarded.
func NewPublisher(webhooks []config.WebhookConfig, next release.EventPublisher) *Publisher {
	return &Publisher{
		webhooks: webhooks,
		client:   &http.Client{},
		next:     next,
		logger:   slog.Default().With("component", "webhook_publisher"),
	}
}

// Publish sends events to configured webhook endpoints.
// Events are forwarded to the next publisher regardless of webhook success.
func (p *Publisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	for _, event := range events {
		for i := range p.webhooks {
			wh := &p.webhooks[i]
			if !wh.IsWebhookEnabled() {
				continue
			}
			if !p.shouldSendEvent(wh, event.EventName()) {
				continue
			}

			payload := p.buildPayload(event)
			go p.sendWithRetry(ctx, wh, payload)
		}
	}

	// Forward to next publisher
	if p.next != nil {
		return p.next.Publish(ctx, events...)
	}

	return nil
}

// shouldSendEvent checks if the webhook should receive this event.
func (p *Publisher) shouldSendEvent(wh *config.WebhookConfig, eventName string) bool {
	// Empty events list means all events
	if len(wh.Events) == 0 {
		return true
	}

	for _, e := range wh.Events {
		if e == eventName {
			return true
		}
		// Support wildcard patterns like "release.*"
		if strings.HasSuffix(e, ".*") {
			prefix := strings.TrimSuffix(e, "*")
			if strings.HasPrefix(eventName, prefix) {
				return true
			}
		}
	}
	return false
}

// buildPayload creates a WebhookPayload from a domain event.
func (p *Publisher) buildPayload(event release.DomainEvent) *WebhookPayload {
	payload := &WebhookPayload{
		Event:     event.EventName(),
		Timestamp: event.OccurredAt(),
		ReleaseID: string(event.AggregateID()),
		Data:      make(map[string]any),
	}

	// Extract event-specific data
	switch e := event.(type) {
	case release.ReleaseInitializedEvent:
		payload.Data["branch"] = e.Branch
		payload.Data["repository"] = e.Repository

	case release.ReleasePlannedEvent:
		payload.Data["current_version"] = e.CurrentVersion.String()
		payload.Data["next_version"] = e.NextVersion.String()
		payload.Data["release_type"] = e.ReleaseType
		payload.Data["commit_count"] = e.CommitCount

	case release.ReleaseVersionedEvent:
		payload.Data["version"] = e.Version.String()
		payload.Data["tag_name"] = e.TagName

	case release.ReleaseNotesGeneratedEvent:
		payload.Data["changelog_updated"] = e.ChangelogUpdated
		payload.Data["notes_length"] = e.NotesLength

	case release.ReleaseApprovedEvent:
		payload.Data["approved_by"] = e.ApprovedBy

	case release.ReleasePublishingStartedEvent:
		payload.Data["plugins"] = e.Plugins

	case release.ReleasePublishedEvent:
		payload.Data["version"] = e.Version.String()
		payload.Data["tag_name"] = e.TagName
		payload.Data["release_url"] = e.ReleaseURL

	case release.ReleaseFailedEvent:
		payload.Data["reason"] = e.Reason
		payload.Data["failed_at"] = string(e.FailedAt)
		payload.Data["is_recoverable"] = e.IsRecoverable

	case release.ReleaseCanceledEvent:
		payload.Data["reason"] = e.Reason
		payload.Data["canceled_by"] = e.CanceledBy

	case release.PluginExecutedEvent:
		payload.Data["plugin_name"] = e.PluginName
		payload.Data["hook"] = e.Hook
		payload.Data["success"] = e.Success
		payload.Data["message"] = e.Message
		payload.Data["duration_ms"] = e.Duration.Milliseconds()

	case release.ReleaseRetriedEvent:
		payload.Data["previous_state"] = string(e.PreviousState)
		payload.Data["new_state"] = string(e.NewState)
	}

	return payload
}

// sendWithRetry sends a webhook request with retries.
func (p *Publisher) sendWithRetry(ctx context.Context, wh *config.WebhookConfig, payload *WebhookPayload) {
	var lastErr error

	for attempt := 0; attempt <= getRetryCount(wh); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				p.logger.Warn("webhook canceled during retry",
					"webhook", wh.Name,
					"attempt", attempt,
					"error", ctx.Err())
				return
			case <-time.After(getRetryDelay(wh)):
			}
		}

		err := p.send(ctx, wh, payload)
		if err == nil {
			p.logger.Debug("webhook sent successfully",
				"webhook", wh.Name,
				"event", payload.Event,
				"release_id", payload.ReleaseID)
			return
		}

		lastErr = err
		p.logger.Warn("webhook request failed",
			"webhook", wh.Name,
			"attempt", attempt+1,
			"max_attempts", getRetryCount(wh)+1,
			"error", err)
	}

	p.logger.Error("webhook failed after all retries",
		"webhook", wh.Name,
		"event", payload.Event,
		"release_id", payload.ReleaseID,
		"error", lastErr)
}

// send performs a single webhook request.
func (p *Publisher) send(ctx context.Context, wh *config.WebhookConfig, payload *WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, getTimeout(wh))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Relicta-Webhook/1.0")
	req.Header.Set("X-Relicta-Event", payload.Event)
	req.Header.Set("X-Relicta-Delivery", payload.ReleaseID)

	// Add custom headers
	for key, value := range wh.Headers {
		req.Header.Set(key, value)
	}

	// Sign the payload if secret is configured
	if wh.Secret != "" {
		signature := signPayload(body, wh.Secret)
		req.Header.Set("X-Relicta-Signature", "sha256="+signature)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error messages
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// signPayload creates an HMAC-SHA256 signature of the payload.
func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies a webhook signature.
// This is a helper for webhook receivers to validate payloads.
func VerifySignature(payload []byte, signature, secret string) bool {
	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	expected := signPayload(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Ensure Publisher implements release.EventPublisher.
var _ release.EventPublisher = (*Publisher)(nil)
