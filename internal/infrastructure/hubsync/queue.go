package hubsync

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileQueue is a single-writer, single-reader newline-delimited JSON queue
// stored on disk. Each line is one QueueEntry. Append-only writes give us
// crash safety; rebuild-on-drain compaction reclaims space without locking
// out new writers.
//
// Concurrency model: a single CLI process holds an OS-level advisory lock
// (a sibling .lock file opened with O_EXCL) for the duration of a drain.
// Concurrent CLI invocations targeting the same queue see "queue locked"
// errors and back off — the Hub's idempotent SaveEvent makes accidental
// double-shipment harmless if a stale lock ever leaks.
//
// This is intentionally simple. A SQLite-backed queue would give better
// random-access semantics but adds CGO and migration burden the alpha
// doesn't need.
type FileQueue struct {
	path string
	mu   sync.Mutex
}

// NewFileQueue returns a queue rooted at path. Parent directory is created
// if missing. The queue file itself is created on first append.
func NewFileQueue(path string) (*FileQueue, error) {
	if path == "" {
		return nil, errors.New("hubsync: queue path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create queue dir: %w", err)
	}
	return &FileQueue{path: path}, nil
}

// Enqueue appends an entry. EnqueuedAt is set to now if zero.
func (q *FileQueue) Enqueue(entry QueueEntry) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if entry.EnqueuedAt.IsZero() {
		entry.EnqueuedAt = time.Now().UTC()
	}

	f, err := os.OpenFile(q.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open queue: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		return fmt.Errorf("encode entry: %w", err)
	}
	return nil
}

// Drain reads the queue, hands each entry to fn, and re-writes the queue
// with whatever fn returns as "still pending". A function returning false
// means "stop retrying this entry" (it succeeded or hit a terminal failure).
//
// If fn returns an error AND a non-zero updated entry, that updated entry
// is re-queued with the new attempt count / last_error fields written by
// the caller. Errors from fn do NOT abort the drain — we keep going so a
// single poison entry can't block the rest of the queue.
//
// Returns the count of entries successfully drained (fn returned false).
func (q *FileQueue) Drain(fn func(QueueEntry) (keep bool, updated QueueEntry, err error)) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	f, err := os.Open(q.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // nothing queued yet
		}
		return 0, fmt.Errorf("open queue: %w", err)
	}
	entries, readErr := decodeAllEntries(f)
	_ = f.Close()
	if readErr != nil {
		return 0, readErr
	}

	pending := make([]QueueEntry, 0, len(entries))
	drained := 0
	for _, e := range entries {
		keep, updated, _ := fn(e)
		if keep {
			if updated.ID == "" {
				updated = e // fn didn't replace; re-queue as-is
			}
			pending = append(pending, updated)
		} else {
			drained++
		}
	}

	return drained, q.rewrite(pending)
}

// Len returns the number of entries currently in the queue. Cheap-ish —
// reads the whole file. Suitable for status displays, not hot paths.
func (q *FileQueue) Len() (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	f, err := os.Open(q.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // 4MB max line
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

// rewrite atomically replaces the queue with the given entries. Writes to a
// sibling .tmp file then renames over the target so a crash mid-write can't
// truncate the queue.
func (q *FileQueue) rewrite(entries []QueueEntry) error {
	if len(entries) == 0 {
		// Empty queue — remove the file entirely so Len()=0 returns fast.
		_ = os.Remove(q.path)
		return nil
	}

	tmpPath := q.path + ".tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp: %w", err)
	}

	enc := json.NewEncoder(tmp)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("encode pending: %w", err)
		}
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close tmp: %w", err)
	}
	return os.Rename(tmpPath, q.path)
}

func decodeAllEntries(r io.Reader) ([]QueueEntry, error) {
	out := []QueueEntry{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e QueueEntry
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip corrupt lines rather than aborting — better to ship
			// what we can than fail the entire drain on one bad entry.
			continue
		}
		out = append(out, e)
	}
	return out, scanner.Err()
}
