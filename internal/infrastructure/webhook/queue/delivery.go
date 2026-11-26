// Package queue provides a file-based persistent webhook delivery queue.
// Deliveries are stored as JSON files on disk, ensuring they survive process
// restarts. Processing is ordered per endpoint with configurable concurrency.
package queue

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// DeliveryStatus represents the current state of a delivery.
type DeliveryStatus string

const (
	// StatusPending indicates the delivery is waiting to be processed.
	StatusPending DeliveryStatus = "pending"
	// StatusInFlight indicates the delivery is currently being attempted.
	StatusInFlight DeliveryStatus = "in_flight"
	// StatusDelivered indicates the delivery completed successfully.
	StatusDelivered DeliveryStatus = "delivered"
	// StatusFailed indicates a delivery attempt failed (may be retried).
	StatusFailed DeliveryStatus = "failed"
	// StatusDeadLetter indicates the delivery exhausted all retries.
	StatusDeadLetter DeliveryStatus = "dead_letter"
)

// Delivery represents a single webhook delivery attempt.
type Delivery struct {
	// ID is a unique identifier for this delivery.
	ID string `json:"id"`
	// EndpointURL is the target webhook URL.
	EndpointURL string `json:"endpoint_url"`
	// EndpointName is a human-readable name for the endpoint.
	EndpointName string `json:"endpoint_name"`
	// Payload is the serialized webhook payload.
	Payload json.RawMessage `json:"payload"`
	// PayloadHash is a SHA-256 hash of the payload for deduplication and logging.
	PayloadHash string `json:"payload_hash"`
	// Status is the current delivery status.
	Status DeliveryStatus `json:"status"`
	// Attempts is the number of delivery attempts made.
	Attempts int `json:"attempts"`
	// MaxAttempts is the maximum number of attempts before dead-lettering.
	MaxAttempts int `json:"max_attempts"`
	// CreatedAt is when the delivery was enqueued.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the delivery was last modified.
	UpdatedAt time.Time `json:"updated_at"`
	// NextAttemptAt is when the next delivery attempt should occur.
	NextAttemptAt time.Time `json:"next_attempt_at"`
	// LastStatusCode is the HTTP status code from the last attempt.
	LastStatusCode int `json:"last_status_code,omitempty"`
	// LastError is the error message from the last failed attempt.
	LastError string `json:"last_error,omitempty"`
	// LastResponseTime is the duration of the last request in milliseconds.
	LastResponseTime int64 `json:"last_response_time_ms,omitempty"`
	// TimeoutMillis is the HTTP request timeout captured when enqueued.
	TimeoutMillis int64 `json:"timeout_ms,omitempty"`
	// RetryDelayMillis is the delay before retrying a failed delivery.
	RetryDelayMillis int64 `json:"retry_delay_ms,omitempty"`
	// Headers are custom headers to include in the request.
	Headers map[string]string `json:"headers,omitempty"`
	// Secret is the HMAC signing secret for this endpoint.
	Secret string `json:"secret,omitempty"`
	// Event is the event name that triggered this delivery.
	Event string `json:"event,omitempty"`
}

// HashPayload computes and sets the PayloadHash from the current Payload.
func (d *Delivery) HashPayload() {
	d.PayloadHash = payloadHash(d.Payload)
}

func payloadHash(payload []byte) string {
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

// FileQueue is a file-based persistent delivery queue.
// Deliveries are stored as individual JSON files in a directory structure:
//
//	<baseDir>/pending/<deliveryID>.json
//	<baseDir>/dead_letter/<deliveryID>.json
//	<baseDir>/delivered/<deliveryID>.json
type FileQueue struct {
	baseDir     string
	mu          sync.RWMutex
	concurrency int
	workerLock  *flock.Flock
	workerOwned bool
}

// QueueOption configures a FileQueue.
type QueueOption func(*FileQueue)

// WithRecovery is retained for source compatibility. In-flight recovery now
// happens only after Publisher.Start acquires the exclusive worker lock.
//
// Deprecated: recovery is managed by the persistent publisher lifecycle.
func WithRecovery() QueueOption {
	return func(*FileQueue) {}
}

// WithoutRecovery is retained for source compatibility. Opening a queue never
// recovers in-flight deliveries; only the locked publisher worker does.
//
// Deprecated: recovery is managed by the persistent publisher lifecycle.
func WithoutRecovery() QueueOption {
	return func(*FileQueue) {}
}

var deliveryIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validateDeliveryID(id string) error {
	if !deliveryIDPattern.MatchString(id) {
		return fmt.Errorf("invalid delivery ID %q: use 1-128 letters, digits, dots, underscores, or hyphens, starting with a letter or digit", id)
	}
	return nil
}

// WithConcurrency sets the maximum number of concurrent deliveries per endpoint.
func WithConcurrency(n int) QueueOption {
	return func(q *FileQueue) {
		if n > 0 {
			q.concurrency = n
		}
	}
}

// NewFileQueue creates a new file-based delivery queue rooted at baseDir.
// The directory structure is created if it does not exist.
func NewFileQueue(baseDir string, opts ...QueueOption) (*FileQueue, error) {
	q := &FileQueue{
		baseDir:     baseDir,
		concurrency: 1,
	}
	for _, opt := range opts {
		opt(q)
	}

	// Create subdirectories for each status
	for _, sub := range []string{"pending", "in_flight", "delivered", "dead_letter"} {
		dir := filepath.Join(baseDir, sub)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("failed to create queue directory %s: %w", dir, err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, fmt.Errorf("failed to secure queue directory %s: %w", dir, err)
		}
	}
	return q, nil
}

// TryAcquireWorker reserves processing and recovery for this process. Other
// queue users may still enqueue and inspect while one worker owns the lock.
func (q *FileQueue) TryAcquireWorker() (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.workerOwned {
		return true, nil
	}
	lock := flock.New(filepath.Join(q.baseDir, "worker.lock"), flock.SetPermissions(0o600))
	owned, err := lock.TryLock()
	if err != nil || !owned {
		return false, err
	}
	q.workerLock = lock
	q.workerOwned = true
	return true, nil
}

// OwnsWorker reports whether this queue instance owns delivery processing.
func (q *FileQueue) OwnsWorker() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.workerOwned
}

// Close releases the worker reservation, if this queue owns it.
func (q *FileQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.workerOwned || q.workerLock == nil {
		return nil
	}
	if err := q.workerLock.Unlock(); err != nil {
		return err
	}
	q.workerLock = nil
	q.workerOwned = false
	return nil
}

// Concurrency returns the configured concurrency level.
func (q *FileQueue) Concurrency() int {
	return q.concurrency
}

// Enqueue adds a delivery to the pending queue.
func (q *FileQueue) Enqueue(d *Delivery) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if err := validateDeliveryID(d.ID); err != nil {
		return err
	}
	for _, status := range []string{"pending", "in_flight", "delivered", "dead_letter"} {
		path := filepath.Join(q.baseDir, status, d.ID+".json")
		existing, err := q.readDeliveryFile(path)
		if err == nil {
			if existing.PayloadHash != "" && existing.PayloadHash != payloadHash(d.Payload) {
				return fmt.Errorf("delivery ID %s already exists with a different payload", d.ID)
			}
			return nil
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("check existing delivery %s: %w", d.ID, err)
		}
	}
	if d.Status == "" {
		d.Status = StatusPending
	}
	d.CreatedAt = time.Now().UTC()
	d.UpdatedAt = d.CreatedAt
	d.HashPayload()

	return q.writeDelivery(d, "pending")
}

// Dequeue retrieves up to n pending deliveries for the given endpoint,
// moving them to in_flight status. Deliveries are returned in creation order.
func (q *FileQueue) Dequeue(endpointURL string, n int) ([]*Delivery, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pendingDir := filepath.Join(q.baseDir, "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read pending directory: %w", err)
	}

	// Load and filter by endpoint, respecting NextAttemptAt
	now := time.Now().UTC()
	var candidates []*Delivery
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		d, err := q.readDeliveryFile(filepath.Join(pendingDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read pending delivery %s: %w", entry.Name(), err)
		}
		if d.EndpointURL != endpointURL {
			continue
		}
		if d.NextAttemptAt.After(now) {
			continue
		}
		candidates = append(candidates, d)
	}

	// Sort by creation time for FIFO ordering
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})

	if n > len(candidates) {
		n = len(candidates)
	}

	result := candidates[:n]
	for _, d := range result {
		d.Status = StatusInFlight
		d.UpdatedAt = now
		if err := q.moveDelivery(d, "pending", "in_flight"); err != nil {
			return nil, fmt.Errorf("failed to move delivery %s to in_flight: %w", d.ID, err)
		}
	}

	return result, nil
}

// Complete marks a delivery as successfully delivered.
func (q *FileQueue) Complete(d *Delivery) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	d.Status = StatusDelivered
	d.UpdatedAt = time.Now().UTC()
	return q.moveDelivery(d, "in_flight", "delivered")
}

// Fail marks a delivery as failed. If attempts are exhausted, it moves
// to the dead-letter queue. Otherwise, it returns to pending with the
// NextAttemptAt set for retry.
func (q *FileQueue) Fail(d *Delivery, statusCode int, errMsg string, nextAttempt time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	d.Attempts++
	d.LastStatusCode = statusCode
	d.LastError = errMsg
	d.UpdatedAt = time.Now().UTC()

	if d.Attempts >= d.MaxAttempts {
		d.Status = StatusDeadLetter
		return q.moveDelivery(d, "in_flight", "dead_letter")
	}

	d.Status = StatusPending
	d.NextAttemptAt = nextAttempt
	return q.moveDelivery(d, "in_flight", "pending")
}

// ListPending returns all pending deliveries for an endpoint.
func (q *FileQueue) ListPending(endpointURL string) ([]*Delivery, error) {
	return q.listByStatus("pending", endpointURL)
}

// ListDeadLetter returns all dead-lettered deliveries for an endpoint.
func (q *FileQueue) ListDeadLetter(endpointURL string) ([]*Delivery, error) {
	return q.listByStatus("dead_letter", endpointURL)
}

// ListDelivered returns all successfully delivered items for an endpoint.
func (q *FileQueue) ListDelivered(endpointURL string) ([]*Delivery, error) {
	return q.listByStatus("delivered", endpointURL)
}

// ListInFlight returns all deliveries currently marked in-flight.
func (q *FileQueue) ListInFlight(endpointURL string) ([]*Delivery, error) {
	return q.listByStatus("in_flight", endpointURL)
}

// Get retrieves a delivery by ID, searching all status directories.
func (q *FileQueue) Get(id string) (*Delivery, error) {
	if err := validateDeliveryID(id); err != nil {
		return nil, err
	}
	q.mu.RLock()
	defer q.mu.RUnlock()

	for _, status := range []string{"pending", "in_flight", "delivered", "dead_letter"} {
		path := filepath.Join(q.baseDir, status, id+".json")
		d, err := q.readDeliveryFile(path)
		if err == nil {
			return d, nil
		}
	}
	return nil, fmt.Errorf("delivery %s not found", id)
}

// Requeue moves a delivery back to pending for re-delivery.
// This works for dead-lettered or delivered items.
func (q *FileQueue) Requeue(id string) (*Delivery, error) {
	if err := validateDeliveryID(id); err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, fromStatus := range []string{"dead_letter", "delivered", "pending"} {
		path := filepath.Join(q.baseDir, fromStatus, id+".json")
		d, err := q.readDeliveryFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read delivery %s: %w", id, err)
		}
		d.Status = StatusPending
		d.Attempts = 0
		d.LastError = ""
		d.LastStatusCode = 0
		d.NextAttemptAt = time.Time{}
		d.UpdatedAt = time.Now().UTC()
		if err := q.moveDelivery(d, fromStatus, "pending"); err != nil {
			return nil, fmt.Errorf("failed to requeue delivery %s: %w", id, err)
		}
		return d, nil
	}
	return nil, fmt.Errorf("delivery %s not found", id)
}

// RecoverInFlight moves all in-flight deliveries back to pending.
// This is used during startup to recover deliveries that were being
// processed when the previous instance was interrupted.
func (q *FileQueue) RecoverInFlight() (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	inFlightDir := filepath.Join(q.baseDir, "in_flight")
	entries, err := os.ReadDir(inFlightDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read in_flight directory: %w", err)
	}

	recovered := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(inFlightDir, entry.Name())
		d, err := q.readDeliveryFile(path)
		if err != nil {
			return recovered, fmt.Errorf("failed to read in-flight delivery %s: %w", entry.Name(), err)
		}
		d.Status = StatusPending
		d.UpdatedAt = time.Now().UTC()
		if err := q.moveDelivery(d, "in_flight", "pending"); err != nil {
			return recovered, fmt.Errorf("failed to recover in-flight delivery %s: %w", d.ID, err)
		}
		recovered++
	}

	return recovered, nil
}

// Endpoints returns a deduplicated list of endpoint URLs with pending deliveries.
func (q *FileQueue) Endpoints() ([]string, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	pendingDir := filepath.Join(q.baseDir, "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read pending directory: %w", err)
	}

	seen := make(map[string]struct{})
	var endpoints []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		d, err := q.readDeliveryFile(filepath.Join(pendingDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read pending delivery %s: %w", entry.Name(), err)
		}
		if _, ok := seen[d.EndpointURL]; !ok {
			seen[d.EndpointURL] = struct{}{}
			endpoints = append(endpoints, d.EndpointURL)
		}
	}
	return endpoints, nil
}

// listByStatus lists deliveries in a given status directory, optionally filtered by endpoint.
func (q *FileQueue) listByStatus(status, endpointURL string) ([]*Delivery, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	dir := filepath.Join(q.baseDir, status)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s directory: %w", status, err)
	}

	var result []*Delivery
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		d, err := q.readDeliveryFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s delivery %s: %w", status, entry.Name(), err)
		}
		if endpointURL != "" && d.EndpointURL != endpointURL {
			continue
		}
		result = append(result, d)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

// writeDelivery writes a delivery to a status subdirectory.
func (q *FileQueue) writeDelivery(d *Delivery, status string) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal delivery: %w", err)
	}

	dir := filepath.Join(q.baseDir, status)
	path := filepath.Join(dir, d.ID+".json")
	tmp, err := os.CreateTemp(dir, ".delivery-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary delivery file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to set permissions on delivery file %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write delivery file %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync delivery file %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close delivery file %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to persist delivery file %s: %w", path, err)
	}
	if dirHandle, err := os.Open(dir); err == nil {
		if syncErr := dirHandle.Sync(); syncErr != nil {
			_ = dirHandle.Close()
			return fmt.Errorf("failed to sync delivery directory %s: %w", dir, syncErr)
		}
		if closeErr := dirHandle.Close(); closeErr != nil {
			return fmt.Errorf("failed to close delivery directory %s: %w", dir, closeErr)
		}
	}
	return nil
}

// moveDelivery moves a delivery from one status directory to another.
func (q *FileQueue) moveDelivery(d *Delivery, fromStatus, toStatus string) error {
	// Write to destination first
	if err := q.writeDelivery(d, toStatus); err != nil {
		return err
	}

	// Remove from source
	srcPath := filepath.Join(q.baseDir, fromStatus, d.ID+".json")
	if err := os.Remove(srcPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove source file %s: %w", srcPath, err)
	}
	return nil
}

// readDeliveryFile reads and unmarshals a delivery from a JSON file.
func (q *FileQueue) readDeliveryFile(path string) (*Delivery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var d Delivery
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to unmarshal delivery from %s: %w", path, err)
	}
	if err := validateDeliveryID(d.ID); err != nil {
		return nil, fmt.Errorf("invalid delivery stored in %s: %w", path, err)
	}
	if filepath.Base(path) != d.ID+".json" {
		return nil, fmt.Errorf("delivery ID %q does not match file name %q", d.ID, filepath.Base(path))
	}
	return &d, nil
}
