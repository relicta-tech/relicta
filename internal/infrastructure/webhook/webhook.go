// Package webhook provides HTTP webhook notifications for release events.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/domain/release"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/webhook/queue"
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

	// inFlight tracks the delivery operations so Close can wait for any operation
	// already started by a concurrent Publish call.
	inFlight      sync.WaitGroup
	queue         *queue.FileQueue
	workerCancel  context.CancelFunc
	workerDone    chan struct{}
	workerStarted bool
}

// NewPersistentPublisher creates a webhook publisher backed by a durable file queue.
// Pending deliveries are retried in the background and survive process restarts.
func NewPersistentPublisher(webhooks []config.WebhookConfig, next release.EventPublisher, q *queue.FileQueue) *Publisher {
	p := NewPublisher(webhooks, next)
	p.queue = q
	return p
}

// Start resumes pending deliveries. It is safe to call more than once.
func (p *Publisher) Start(parent context.Context) {
	if p.queue != nil && p.workerCancel == nil {
		owned, err := p.queue.TryAcquireWorker()
		if err != nil {
			p.logger.Error("failed to acquire webhook worker lock", "error", err)
			return
		}
		if !owned {
			p.logger.Info("webhook queue worker already active in another process")
			return
		}
		if _, err := p.queue.RecoverInFlight(); err != nil {
			p.logger.Error("failed to recover in-flight webhook deliveries", "error", err)
			_ = p.queue.Close()
			return
		}
		ctx, cancel := context.WithCancel(parent)
		p.workerCancel = cancel
		p.workerDone = make(chan struct{})
		p.workerStarted = true
		go p.runQueueWorker(ctx)
	}
}

// NewPublisher creates a new webhook publisher.
// The next parameter is optional - if nil, events are not forwarded.
func NewPublisher(webhooks []config.WebhookConfig, next release.EventPublisher) *Publisher {
	return &Publisher{
		webhooks: webhooks,
		client:   &http.Client{Timeout: 30 * time.Second},
		next:     next,
		logger:   slog.Default().With("component", "webhook_publisher"),
	}
}

// Publish sends events to configured webhook endpoints and returns delivery failures.
// Events are forwarded to the next publisher regardless of webhook success.
func (p *Publisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	if p.queue != nil {
		return p.publishQueued(ctx, events...)
	}
	var deliveries sync.WaitGroup
	deliveryErrors := make(chan error, len(events)*len(p.webhooks))
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
			p.inFlight.Add(1)
			deliveries.Add(1)
			go func() {
				defer deliveries.Done()
				defer p.inFlight.Done()
				if err := p.sendWithRetry(ctx, wh, payload); err != nil {
					deliveryErrors <- fmt.Errorf("webhook %q event %s release %s: %w",
						wh.Name, payload.Event, payload.ReleaseID, err)
				}
			}()
		}
	}
	deliveries.Wait()
	close(deliveryErrors)
	var errs []error
	for err := range deliveryErrors {
		errs = append(errs, err)
	}

	// Forward to the next publisher even when delivery failed, so one unavailable
	// endpoint does not suppress local governance and analytics recording.
	var nextErr error
	if p.next != nil {
		nextErr = p.next.Publish(ctx, events...)
	}
	return errors.Join(append(errs, nextErr)...)
}

func (p *Publisher) publishQueued(ctx context.Context, events ...release.DomainEvent) error {
	var errs []error
	for _, event := range events {
		for i := range p.webhooks {
			wh := &p.webhooks[i]
			if !wh.IsWebhookEnabled() || !p.shouldSendEvent(wh, event.EventName()) {
				continue
			}
			payload := p.buildPayload(event)
			body, err := json.Marshal(payload)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode webhook %q event %s: %w", wh.Name, payload.Event, err))
				continue
			}
			idHash := sha256.Sum256(append(append([]byte(wh.URL), 0), body...))
			attempts := getRetryCount(wh) + 1
			if attempts < 1 {
				attempts = 1
			}
			d := &queue.Delivery{
				ID: hex.EncodeToString(idHash[:]), EndpointURL: wh.URL, EndpointName: wh.Name,
				Payload: body, MaxAttempts: attempts, Headers: cloneHeaders(wh.Headers),
				Secret: wh.Secret, Event: payload.Event,
				TimeoutMillis: getTimeout(wh).Milliseconds(), RetryDelayMillis: getRetryDelay(wh).Milliseconds(),
			}
			if err := p.queue.Enqueue(d); err != nil {
				errs = append(errs, fmt.Errorf("enqueue webhook %q event %s release %s: %w", wh.Name, payload.Event, payload.ReleaseID, err))
			}
		}
	}
	var nextErr error
	if p.next != nil {
		nextErr = p.next.Publish(ctx, events...)
	}
	return errors.Join(append(errs, nextErr)...)
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	clonedHeaders := make(map[string]string, len(headers))
	for key, value := range headers {
		clonedHeaders[key] = value
	}
	return clonedHeaders
}

func (p *Publisher) runQueueWorker(ctx context.Context) {
	defer close(p.workerDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if _, err := p.processQueue(ctx); err != nil && ctx.Err() == nil {
			p.logger.Error("webhook queue processing failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Publisher) processQueue(ctx context.Context) (int, error) {
	endpoints, err := p.queue.Endpoints()
	if err != nil {
		return 0, err
	}
	var errs []error
	processed := 0
	for _, endpoint := range endpoints {
		wh := p.webhookForURL(endpoint)
		if wh == nil {
			continue
		}
		items, err := p.queue.Dequeue(endpoint, p.queue.Concurrency())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		processed += len(items)
		for _, item := range items {
			item.EndpointName = wh.Name
			item.Headers = cloneHeaders(wh.Headers)
			item.Secret = wh.Secret
			item.TimeoutMillis = getTimeout(wh).Milliseconds()
			item.RetryDelayMillis = getRetryDelay(wh).Milliseconds()
			item.MaxAttempts = getRetryCount(wh) + 1
			if item.MaxAttempts < 1 {
				item.MaxAttempts = 1
			}
			start := time.Now()
			statusCode := 0
			deliveryErr := p.sendQueued(ctx, item, &statusCode)
			item.LastResponseTime = time.Since(start).Milliseconds()
			if deliveryErr == nil {
				if err := p.queue.Complete(item); err != nil {
					errs = append(errs, fmt.Errorf("complete webhook delivery %s: %w", item.ID, err))
				}
				continue
			}
			nextAttempt := time.Now().UTC().Add(time.Duration(item.RetryDelayMillis) * time.Millisecond)
			if err := p.queue.Fail(item, statusCode, deliveryErr.Error(), nextAttempt); err != nil {
				errs = append(errs, fmt.Errorf("record failed webhook delivery %s: %w", item.ID, err))
				continue
			}
			if item.Attempts >= item.MaxAttempts {
				p.logger.Error("webhook delivery moved to dead-letter queue",
					"webhook", item.EndpointName, "event", item.Event,
					"delivery_id", item.ID, "status_code", statusCode)
			} else {
				p.logger.Warn("webhook delivery failed; retry scheduled",
					"webhook", item.EndpointName, "event", item.Event,
					"delivery_id", item.ID, "status_code", statusCode,
					"next_attempt_at", nextAttempt)
			}
		}
	}
	return processed, errors.Join(errs...)
}

func (p *Publisher) webhookForURL(endpoint string) *config.WebhookConfig {
	for i := range p.webhooks {
		wh := &p.webhooks[i]
		if wh.URL == endpoint && wh.IsWebhookEnabled() {
			return wh
		}
	}
	return nil
}

func (p *Publisher) sendQueued(ctx context.Context, d *queue.Delivery, statusCode *int) error {
	var payload WebhookPayload
	if err := json.Unmarshal(d.Payload, &payload); err != nil {
		return fmt.Errorf("decode queued webhook payload: %w", err)
	}
	timeout := time.Duration(d.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.EndpointURL, bytes.NewReader(d.Payload))
	if err != nil {
		return err
	}
	for key, value := range d.Headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Relicta-Webhook/1.0")
	request.Header.Set("X-Relicta-Event", payload.Event)
	request.Header.Set("X-Relicta-Delivery", payload.ReleaseID)
	request.Header.Set("X-Relicta-Delivery-ID", d.PayloadHash)
	if d.Secret != "" {
		request.Header.Set("X-Relicta-Signature", "sha256="+signPayload(d.Payload, d.Secret))
	}
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()
	*statusCode = response.StatusCode
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	if response.StatusCode >= 400 {
		return fmt.Errorf("server returned %d: %s", response.StatusCode, string(responseBody))
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
	case *release.RunCreatedEvent:
		payload.Data["repo_id"] = e.RepoID
		payload.Data["head_sha"] = string(e.HeadSHA)

	case *release.RunPlannedEvent:
		payload.Data["current_version"] = e.VersionCurrent.String()
		payload.Data["next_version"] = e.VersionNext.String()
		payload.Data["bump_kind"] = string(e.BumpKind)
		payload.Data["commit_count"] = e.CommitCount
		payload.Data["risk_score"] = e.RiskScore

	case *release.RunVersionedEvent:
		payload.Data["version"] = e.VersionNext.String()
		payload.Data["tag_name"] = e.TagName
		payload.Data["bump_kind"] = string(e.BumpKind)

	case *release.RunNotesGeneratedEvent:
		payload.Data["notes_length"] = e.NotesLength
		payload.Data["provider"] = e.Provider
		payload.Data["model"] = e.Model

	case *release.RunApprovedEvent:
		payload.Data["approved_by"] = e.ApprovedBy
		payload.Data["auto_approved"] = e.AutoApproved
		payload.Data["plan_hash"] = e.PlanHash

	case *release.RunPublishingStartedEvent:
		payload.Data["steps"] = e.Steps
		payload.Data["plan_hash"] = e.PlanHash

	case *release.RunPublishedEvent:
		payload.Data["version"] = e.Version.String()

	case *release.RunFailedEvent:
		payload.Data["reason"] = e.Reason

	case *release.RunCanceledEvent:
		payload.Data["reason"] = e.Reason
		payload.Data["canceled_by"] = e.By

	case *release.PluginExecutedEvent:
		payload.Data["plugin_name"] = e.PluginName
		payload.Data["hook"] = e.Hook
		payload.Data["success"] = e.Success
		payload.Data["message"] = e.Message
		payload.Data["duration_ms"] = e.Duration.Milliseconds()

	case *release.RunRetriedEvent:
		payload.Data["retried_by"] = e.By

	case *release.StateTransitionedEvent:
		payload.Data["from_state"] = string(e.From)
		payload.Data["to_state"] = string(e.To)
		payload.Data["event"] = e.Event
		payload.Data["actor"] = e.Actor
	}

	return payload
}

// sendWithRetry sends a webhook request with retries.
func (p *Publisher) sendWithRetry(ctx context.Context, wh *config.WebhookConfig, payload *WebhookPayload) error {
	var lastErr error

	for attempt := 0; attempt <= getRetryCount(wh); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				p.logger.Warn("webhook canceled during retry",
					"webhook", wh.Name,
					"attempt", attempt,
					"error", ctx.Err())
				return ctx.Err()
			case <-time.After(getRetryDelay(wh)):
			}
		}

		err := p.send(ctx, wh, payload)
		if err == nil {
			p.logger.Debug("webhook sent successfully",
				"webhook", wh.Name,
				"event", payload.Event,
				"release_id", payload.ReleaseID)
			return nil
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
	return fmt.Errorf("delivery failed after %d attempts: %w", getRetryCount(wh)+1, lastErr)
}

// send performs a single webhook request.
func (p *Publisher) send(ctx context.Context, wh *config.WebhookConfig, payload *WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	deliveryHash := sha256.Sum256(body)

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
	// Stable across retries and repeated publication of the same event, so receivers
	// can deduplicate deliveries without changing the existing release-level header.
	req.Header.Set("X-Relicta-Delivery-ID", hex.EncodeToString(deliveryHash[:]))

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

// Close stops the queue worker and makes a bounded best-effort drain. Pending items remain
// durable if the shutdown deadline expires or an endpoint is unavailable.
func (p *Publisher) Close() error {
	var closeErr error
	if p.workerCancel != nil {
		p.workerCancel()
		<-p.workerDone
	}
	p.inFlight.Wait()
	if p.queue != nil && p.workerStarted && p.queue.OwnsWorker() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for ctx.Err() == nil {
			processed, err := p.processQueue(ctx)
			if err != nil {
				closeErr = fmt.Errorf("drain webhook queue during shutdown: %w", err)
				break
			}
			if processed == 0 {
				endpoints, err := p.queue.Endpoints()
				if err != nil {
					return fmt.Errorf("inspect webhook queue during shutdown: %w", err)
				}
				eligible := false
				for _, endpoint := range endpoints {
					if p.webhookForURL(endpoint) != nil {
						eligible = true
						break
					}
				}
				if !eligible {
					break
				}
				select {
				case <-ctx.Done():
				case <-time.After(100 * time.Millisecond):
				}
			}
		}
	}
	if p.queue != nil {
		return errors.Join(closeErr, p.queue.Close())
	}
	return closeErr
}
